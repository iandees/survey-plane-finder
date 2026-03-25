package geojson

import (
	"testing"
)

func TestSimplifyKeepsEndpoints(t *testing.T) {
	coords := []Coordinate{
		{-93.21, 44.95},
		{-93.20, 44.955},
		{-93.19, 44.96},
	}

	result := SimplifyTrack(coords, 1.0)
	if len(result) != 2 {
		t.Errorf("expected 2 points (endpoints only), got %d", len(result))
	}
}

func TestSimplifyPreservesSharpTurns(t *testing.T) {
	coords := []Coordinate{
		{-93.0, 45.0},
		{-93.0, 45.1},
		{-93.0, 45.2},
		{-92.9, 45.2},
		{-92.8, 45.2},
	}

	result := SimplifyTrack(coords, 0.001)
	if len(result) < 3 {
		t.Errorf("expected at least 3 points (preserving corner), got %d", len(result))
	}
}

func TestSimplifyNoOpWithFewPoints(t *testing.T) {
	coords := []Coordinate{
		{-93.0, 45.0},
		{-93.1, 45.1},
	}

	result := SimplifyTrack(coords, 0.001)
	if len(result) != 2 {
		t.Errorf("expected 2 points unchanged, got %d", len(result))
	}
}
