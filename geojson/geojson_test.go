package geojson

import (
	"encoding/json"
	"survey-plane-finder/model"
	"testing"
	"time"
)

func TestBuildLiveFeature(t *testing.T) {
	now := time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC)
	track := &model.AircraftTrack{
		Hex:    "a1b2c3",
		Flight: "N12345",
		Points: []model.TrackPoint{
			{Lat: 44.95, Lon: -93.21, Alt: 3500, Speed: 120, Track: 90, Timestamp: now.Add(-10 * time.Minute)},
			{Lat: 44.96, Lon: -93.18, Alt: 3500, Speed: 120, Track: 90, Timestamp: now.Add(-5 * time.Minute)},
			{Lat: 44.97, Lon: -93.15, Alt: 3500, Speed: 120, Track: 270, Timestamp: now},
		},
		Flagged:  true,
		LastSeen: now,
	}

	feature := BuildLiveFeature(track)

	if feature.Geometry.Type != "LineString" {
		t.Errorf("expected LineString, got %s", feature.Geometry.Type)
	}
	if len(feature.Geometry.Coordinates) != 3 {
		t.Errorf("expected 3 coordinates, got %d", len(feature.Geometry.Coordinates))
	}
	if feature.Geometry.Coordinates[0][0] != -93.21 {
		t.Errorf("expected lon -93.21, got %f", feature.Geometry.Coordinates[0][0])
	}

	props := feature.Properties
	if props["icao"] != "a1b2c3" {
		t.Errorf("expected icao a1b2c3, got %v", props["icao"])
	}
	if props["callsign"] != "N12345" {
		t.Errorf("expected callsign N12345, got %v", props["callsign"])
	}
	if props["altitude_ft"] != 3500 {
		t.Errorf("expected altitude 3500, got %v", props["altitude_ft"])
	}
	if props["speed_kts"] != 120.0 {
		t.Errorf("expected speed 120, got %v", props["speed_kts"])
	}

	bounds, ok := props["survey_bounds"].([]float64)
	if !ok {
		t.Fatalf("expected survey_bounds to be []float64, got %T", props["survey_bounds"])
	}
	if len(bounds) != 4 {
		t.Errorf("expected 4 bounds values, got %d", len(bounds))
	}

	if _, ok := props["globe_url"]; !ok {
		t.Error("expected globe_url property")
	}

	data, err := json.Marshal(feature)
	if err != nil {
		t.Fatalf("failed to marshal feature: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestBuildLiveCollection(t *testing.T) {
	now := time.Now()
	tracks := map[string]*model.AircraftTrack{
		"a1b2c3": {
			Hex:    "a1b2c3",
			Flight: "N12345",
			Points: []model.TrackPoint{
				{Lat: 44.95, Lon: -93.21, Alt: 3500, Timestamp: now},
				{Lat: 44.96, Lon: -93.18, Alt: 3500, Timestamp: now},
			},
			Flagged:  true,
			LastSeen: now,
		},
		"d4e5f6": {
			Hex:    "d4e5f6",
			Points: []model.TrackPoint{
				{Lat: 44.90, Lon: -93.10, Alt: 2000, Timestamp: now},
			},
			Flagged: false,
		},
	}

	collection := BuildLiveCollection(tracks, nil)

	if collection.Type != "FeatureCollection" {
		t.Errorf("expected FeatureCollection, got %s", collection.Type)
	}
	if len(collection.Features) != 1 {
		t.Errorf("expected 1 feature (only flagged with 2+ points), got %d", len(collection.Features))
	}
}
