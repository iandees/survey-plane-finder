package archive

import (
	"survey-plane-finder/model"
	"testing"
	"time"
)

func TestAddDetection(t *testing.T) {
	a := New()
	now := time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC)

	track := &model.AircraftTrack{
		Hex:    "a1b2c3",
		Flight: "N12345",
		Points: []model.TrackPoint{
			{Lat: 44.95, Lon: -93.21, Alt: 3500, Speed: 120, Timestamp: now.Add(-30 * time.Minute)},
			{Lat: 44.96, Lon: -93.18, Alt: 3500, Speed: 120, Timestamp: now},
		},
		Flagged:  true,
		LastSeen: now,
	}

	a.AddOrUpdate(track)

	detections := a.Detections()
	if len(detections) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(detections))
	}
	if detections[0].Hex != "a1b2c3" {
		t.Errorf("expected hex a1b2c3, got %s", detections[0].Hex)
	}
}

func TestUpdateExistingDetection(t *testing.T) {
	a := New()
	now := time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC)

	track := &model.AircraftTrack{
		Hex:    "a1b2c3",
		Flight: "N12345",
		Points: []model.TrackPoint{
			{Lat: 44.95, Lon: -93.21, Alt: 3500, Speed: 120, Timestamp: now.Add(-30 * time.Minute)},
			{Lat: 44.96, Lon: -93.18, Alt: 3500, Speed: 120, Timestamp: now},
		},
		Flagged:  true,
		LastSeen: now,
	}
	a.AddOrUpdate(track)

	track.Points = append(track.Points, model.TrackPoint{
		Lat: 44.97, Lon: -93.15, Alt: 3500, Speed: 120, Timestamp: now.Add(5 * time.Minute),
	})
	track.LastSeen = now.Add(5 * time.Minute)
	a.AddOrUpdate(track)

	detections := a.Detections()
	if len(detections) != 1 {
		t.Fatalf("expected still 1 detection, got %d", len(detections))
	}
	if len(detections[0].Points) != 3 {
		t.Errorf("expected 3 points after update, got %d", len(detections[0].Points))
	}
}

func TestBuildArchiveCollection(t *testing.T) {
	a := New()
	now := time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC)

	track := &model.AircraftTrack{
		Hex:    "a1b2c3",
		Flight: "N12345",
		Points: []model.TrackPoint{
			{Lat: 44.95, Lon: -93.21, Alt: 3500, Speed: 120, Timestamp: now.Add(-30 * time.Minute)},
			{Lat: 44.96, Lon: -93.18, Alt: 3500, Speed: 120, Timestamp: now},
		},
		Flagged:  true,
		LastSeen: now,
	}
	a.AddOrUpdate(track)

	collection := a.BuildCollection("2026-03-24")
	if collection.Type != "FeatureCollection" {
		t.Errorf("expected FeatureCollection, got %s", collection.Type)
	}
	if len(collection.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(collection.Features))
	}

	props := collection.Features[0].Properties
	if _, ok := props["duration_min"]; !ok {
		t.Error("expected duration_min property in archive feature")
	}
	if _, ok := props["active"]; !ok {
		t.Error("expected active property in archive feature")
	}
}

func TestDayRollover(t *testing.T) {
	a := New()
	now := time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC)

	track := &model.AircraftTrack{
		Hex: "a1b2c3",
		Points: []model.TrackPoint{
			{Lat: 44.95, Lon: -93.21, Alt: 3500, Timestamp: now},
			{Lat: 44.96, Lon: -93.18, Alt: 3500, Timestamp: now},
		},
		Flagged:  true,
		LastSeen: now,
	}
	a.AddOrUpdate(track)

	a.ResetForNewDay()
	if len(a.Detections()) != 0 {
		t.Errorf("expected 0 detections after rollover, got %d", len(a.Detections()))
	}
}
