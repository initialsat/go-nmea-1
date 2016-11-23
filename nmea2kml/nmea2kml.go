package main

import (
	"flag"
	"io"
	"log"
	"math"
	"os"

	"text/template"

	"github.com/dustin/go-nmea"
)

const kmlHeader = `<?xml version="1.0" encoding="UTF-8"?>
<kml xmlns="http://www.opengis.net/kml/2.2"
          xmlns:gx="http://www.google.com/kml/ext/2.2">

<Folder>`
const kmlPoint = `<!-- {{ .D }} -->
<Placemark>
  <TimeStamp>{{.TS}}</TimeStamp>
  <Point><coordinates>{{.Lon}},{{.Lat}}</coordinates></Point>
  <gx:flyToMode>smooth</gx:flyToMode>
</Placemark>
`
const kmlFooter = `</Folder></kml>`

const tsFormat = "2006-01-02T15:04:05Z"

var (
	minDist = flag.Int("minDist", 1000, "minimum distance (meters) between points")

	tmpl = template.Must(template.New("").Parse(kmlPoint))
)

type errRememberer struct {
	w   io.WriteCloser
	err error
}

func (e errRememberer) Write(b []byte) (int, error) {
	if e.err != nil {
		return 0, e.err
	}

	var n int
	n, e.err = e.w.Write(b)

	return n, e.err
}

func (e errRememberer) Close() error {
	if e.err != nil {
		return e.err
	}
	return e.w.Close()
}

type kmlWriter struct {
	w          errRememberer
	plat, plon float64
}

func (k *kmlWriter) HandleRMC(m nmea.RMC) {
	Δλ := distance(m.Longitude, m.Latitude, k.plon, k.plat)
	if Δλ > float64(*minDist) {
		tmpl.Execute(k.w, struct {
			Lon, Lat float64
			TS       string
			D        float64
		}{m.Longitude, m.Latitude, m.Timestamp.Format(tsFormat), Δλ})
		k.plat = m.Latitude
		k.plon = m.Longitude
	}
}

func (k kmlWriter) Init() error {
	k.w.Write([]byte(kmlHeader))
	return k.w.err
}

func (k kmlWriter) Close() error {
	k.w.Write([]byte(kmlFooter))
	return k.w.Close()
}

func d2r(d float64) float64 {
	return d * math.Pi / 180.0
}

func distance(lon1, lat1, lon2, lat2 float64) float64 {
	φ1 := d2r(lat1)
	φ2 := d2r(lat2)
	Δφ := d2r(lat2 - lat1)
	Δλ := d2r(lon2 - lon1)

	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*
			math.Sin(Δλ/2)*math.Sin(Δλ/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	rv := 6371000 * c
	log.Printf("Distance from %v,%v to %v,%v is %v", lon1, lat1, lon2, lat2, rv)
	return rv
}

func main() {
	flag.Parse()
	h := &kmlWriter{w: errRememberer{w: os.Stdout}}
	h.Init()
	err := nmea.Process(os.Stdin, h, func(s string, err error) error {
		if s != "" && err != nil {
			log.Printf("On %q: %v", s, err)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error processing stuff: %v", err)
	}
	h.Close()
}