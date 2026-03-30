package main

import (
	"survey-plane-finder/model"
	"testing"
)

func TestSparseGridUpdate(t *testing.T) {
	track := &model.AircraftTrack{
		Hex:  "abc123",
		Grid: make(map[[2]int]float64),
		Points: []model.TrackPoint{
			{Lat: 44.95, Lon: -93.21, Alt: 3500, Track: 90, Speed: 120},
			{Lat: 44.95, Lon: -93.10, Alt: 3500, Track: 90, Speed: 120},
		},
	}

	updateGrid(track, track.Points[0], track.Points[1])

	// Heading 90° / 5° per bucket = bucket 18, altitude 3500 / 100 = bucket 35
	key := [2]int{18, 35}
	if val, ok := track.Grid[key]; !ok || val <= 0 {
		t.Errorf("Expected non-zero value at grid key %v, got %v (exists=%v)", key, val, ok)
	}

	// Verify sparsity — only one cell should be populated
	if len(track.Grid) != 1 {
		t.Errorf("Expected 1 grid entry, got %d", len(track.Grid))
	}
}
