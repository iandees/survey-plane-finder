package main

import (
	"survey-plane-finder/model"
	"testing"
	"time"
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

func TestPruneOldPointsUnflagged(t *testing.T) {
	now := time.Now()
	track := &model.AircraftTrack{
		Hex:     "abc123",
		Flagged: false,
		Points: []model.TrackPoint{
			{Lat: 44.95, Lon: -93.21, Alt: 3500, Timestamp: now.Add(-45 * time.Minute)},
			{Lat: 44.96, Lon: -93.20, Alt: 3500, Timestamp: now.Add(-35 * time.Minute)},
			{Lat: 44.97, Lon: -93.19, Alt: 3500, Timestamp: now.Add(-20 * time.Minute)},
			{Lat: 44.98, Lon: -93.18, Alt: 3500, Timestamp: now.Add(-5 * time.Minute)},
		},
	}

	pruneTrackPoints(track, now)

	// Points older than 30 min should be removed
	if len(track.Points) != 2 {
		t.Errorf("Expected 2 points after pruning, got %d", len(track.Points))
	}
	// Oldest remaining should be the -20 min point
	if track.Points[0].Lat != 44.97 {
		t.Errorf("Expected oldest remaining point at lat 44.97, got %.2f", track.Points[0].Lat)
	}
}

func TestPruneOldPointsFlagged(t *testing.T) {
	now := time.Now()
	track := &model.AircraftTrack{
		Hex:     "abc123",
		Flagged: true,
		Points:  make([]model.TrackPoint, 0),
	}

	// Add points every 5 seconds from 45 min ago to now
	for i := 0; i < 540; i++ {
		track.Points = append(track.Points, model.TrackPoint{
			Lat:       44.95 + float64(i)*0.0001,
			Lon:       -93.21,
			Alt:       3500,
			Timestamp: now.Add(-45*time.Minute + time.Duration(i)*5*time.Second),
		})
	}

	originalCount := len(track.Points)
	pruneTrackPoints(track, now)

	// Should have fewer points than original (old ones downsampled)
	if len(track.Points) >= originalCount {
		t.Errorf("Expected fewer points after downsampling, got %d (was %d)", len(track.Points), originalCount)
	}
	// But should still have some old points (not all removed)
	if track.Points[0].Timestamp.After(now.Add(-30 * time.Minute)) {
		t.Error("Flagged tracks should retain (downsampled) old points")
	}
	// Recent points (last 30 min) should be untouched
	recentCount := 0
	for _, p := range track.Points {
		if !p.Timestamp.Before(now.Add(-30 * time.Minute)) {
			recentCount++
		}
	}
	// 30 min * 60s / 5s = 360 recent points
	if recentCount != 360 {
		t.Errorf("Expected 360 recent points untouched, got %d", recentCount)
	}
}
