package geojson

import (
	"fmt"
	"math"
	"strings"
	"survey-plane-finder/model"
	"time"
)

type Coordinate [2]float64

type Geometry struct {
	Type        string       `json:"type"`
	Coordinates []Coordinate `json:"coordinates"`
}

type Feature struct {
	Type       string                 `json:"type"`
	Geometry   Geometry               `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type FeatureCollection struct {
	Type        string    `json:"type"`
	GeneratedAt string    `json:"generated_at"`
	Features    []Feature `json:"features"`
}

func BuildLiveFeature(track *model.AircraftTrack) Feature {
	coords := make([]Coordinate, 0, len(track.Points))
	var minLon, maxLon, minLat, maxLat float64
	minLon, minLat = 180, 90
	maxLon, maxLat = -180, -90

	for _, p := range track.Points {
		coords = append(coords, Coordinate{p.Lon, p.Lat})
		minLon = math.Min(minLon, p.Lon)
		maxLon = math.Max(maxLon, p.Lon)
		minLat = math.Min(minLat, p.Lat)
		maxLat = math.Max(maxLat, p.Lat)
	}

	lastPoint := track.Points[len(track.Points)-1]
	firstPoint := track.Points[0]

	props := map[string]interface{}{
		"icao":             track.Hex,
		"callsign":         strings.TrimSpace(track.Flight),
		"first_seen":       firstPoint.Timestamp.UTC().Format(time.RFC3339),
		"last_seen":        lastPoint.Timestamp.UTC().Format(time.RFC3339),
		"altitude_ft":      lastPoint.Alt,
		"speed_kts":        lastPoint.Speed,
		"detection_method":  track.DetectionMethod,
		"survey_bounds":    []float64{minLon, minLat, maxLon, maxLat},
		"globe_url":        fmt.Sprintf("https://globe.adsb.lol/?icao=%s", track.Hex),
		"registration_url": fmt.Sprintf("https://globe.adsbexchange.com/?icao=%s", track.Hex),
		"flightaware_url":  buildFlightAwareURL(track.Flight),
	}

	return Feature{
		Type:       "Feature",
		Geometry:   Geometry{Type: "LineString", Coordinates: coords},
		Properties: props,
	}
}

func BuildLiveCollection(tracks map[string]*model.AircraftTrack) FeatureCollection {
	features := make([]Feature, 0)
	for _, track := range tracks {
		if !track.Flagged || len(track.Points) < 2 {
			continue
		}
		features = append(features, BuildLiveFeature(track))
	}

	return FeatureCollection{
		Type:        "FeatureCollection",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Features:    features,
	}
}

func buildFlightAwareURL(flight string) string {
	callsign := strings.TrimSpace(flight)
	if callsign == "" {
		return ""
	}
	return fmt.Sprintf("https://flightaware.com/live/flight/%s", callsign)
}
