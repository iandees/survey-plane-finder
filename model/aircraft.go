package model

import "time"

// Aircraft represents a single aircraft with position data
type Aircraft struct {
	Hex       string      `json:"hex"`
	Flight    string      `json:"flight"`
	Alt       interface{} `json:"alt_baro"` // Can be int or string "ground"
	Lat       float64     `json:"lat"`
	Lon       float64     `json:"lon"`
	Track     float64     `json:"track"`
	GS        float64     `json:"gs"`
	Timestamp int64       `json:"last_seen"`
}

// Altitude returns the altitude as an integer, or 0 if aircraft is on ground
func (a *Aircraft) Altitude() int {
	switch v := a.Alt.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if v == "ground" {
			return 0
		}
	}
	return 0 // Default to 0 for unknown values
}

// ADSBResponse represents the response from the ADSB.lol API
type ADSBResponse struct {
	Aircraft []Aircraft `json:"ac"`
}

// TrackPoint represents a single point in an aircraft's track
type TrackPoint struct {
	Lat       float64
	Lon       float64
	Alt       int
	Track     float64
	Speed     float64
	Timestamp time.Time
}

// AircraftTrack stores historical track information for an aircraft
type AircraftTrack struct {
	Hex             string
	Flight          string
	Points          []TrackPoint
	Flagged         bool
	DetectionMethod string // "grid" or "exhaustive"
	LastSeen        time.Time
	// Miles spent in each [direction][altitude] cell
	Grid [][]float64
}
