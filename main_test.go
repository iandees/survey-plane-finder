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

func TestDeferredGridAllocation(t *testing.T) {
	// Create a track with fewer than minTrackPoints
	track := &model.AircraftTrack{
		Hex:    "abc123",
		Points: make([]model.TrackPoint, 0),
	}

	// Add points below threshold
	for i := 0; i < minTrackPoints-1; i++ {
		track.Points = append(track.Points, model.TrackPoint{
			Lat: 44.95 + float64(i)*0.01, Lon: -93.21, Alt: 3500, Track: 90, Speed: 120,
		})
	}

	// Grid should still be nil
	if track.Grid != nil {
		t.Error("Grid should be nil before reaching minTrackPoints")
	}

	// Add one more point to reach threshold, then backfill
	track.Points = append(track.Points, model.TrackPoint{
		Lat: 44.95 + float64(minTrackPoints)*0.01, Lon: -93.21, Alt: 3500, Track: 90, Speed: 120,
	})
	backfillGrid(track)

	if track.Grid == nil {
		t.Error("Grid should be allocated after backfill")
	}
	if len(track.Grid) == 0 {
		t.Error("Grid should have entries after backfill")
	}
}
