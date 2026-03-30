package model

import (
	"encoding/json"
	"time"
)

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
	// Miles spent in each [heading_bin, altitude_bin] cell (sparse)
	Grid map[[2]int]float64
}

// unmarshalHelper is used for JSON deserialization, supporting both sparse and dense grid formats.
type unmarshalHelper struct {
	Hex             string          `json:"Hex"`
	Flight          string          `json:"Flight"`
	Points          []TrackPoint    `json:"Points"`
	Flagged         bool            `json:"Flagged"`
	DetectionMethod string          `json:"DetectionMethod"`
	LastSeen        time.Time       `json:"LastSeen"`
	Grid            json.RawMessage `json:"Grid"`
}

func (t *AircraftTrack) UnmarshalJSON(data []byte) error {
	var h unmarshalHelper
	if err := json.Unmarshal(data, &h); err != nil {
		return err
	}

	t.Hex = h.Hex
	t.Flight = h.Flight
	t.Points = h.Points
	t.Flagged = h.Flagged
	t.DetectionMethod = h.DetectionMethod
	t.LastSeen = h.LastSeen

	if len(h.Grid) == 0 || string(h.Grid) == "null" {
		return nil
	}

	// Try sparse format first (map with string keys like "18,35")
	var sparse map[[2]int]float64
	if err := json.Unmarshal(h.Grid, &sparse); err == nil {
		t.Grid = sparse
		return nil
	}

	// Fall back to dense format ([][]float64)
	var dense [][]float64
	if err := json.Unmarshal(h.Grid, &dense); err != nil {
		return err
	}

	t.Grid = make(map[[2]int]float64)
	for i, row := range dense {
		for j, val := range row {
			if val > 0 {
				t.Grid[[2]int{i, j}] = val
			}
		}
	}
	return nil
}
