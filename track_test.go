package main

import (
	"encoding/json"
	"math"
	"os"
	"survey-plane-finder/model"
	"testing"
)

func TestSurveyPatternDetection(t *testing.T) {
	testCases := []struct {
		name     string
		fileName string
		expected bool
	}{
		{"a0d65a-yes", "test_tracks/positive_examples/a0d65a.json", true},
		{"a212bb-yes", "test_tracks/positive_examples/a212bb.json", true},
		{"a345cd-yes", "test_tracks/positive_examples/a345cd.json", true},
		{"a45add-yes", "test_tracks/positive_examples/a45add.json", true},
		{"a598d6-yes", "test_tracks/positive_examples/a598d6.json", true},
		{"a65f5e-yes", "test_tracks/positive_examples/a65f5e.json", true},
		{"a68b84-yes", "test_tracks/positive_examples/a68b84.json", true},
		{"a6aa5f-yes", "test_tracks/positive_examples/a6aa5f.json", true},
		{"a9a2b4-yes", "test_tracks/positive_examples/a9a2b4.json", true},
		{"aa8e3e-yes", "test_tracks/positive_examples/aa8e3e.json", true},
		{"aae3ba-yes", "test_tracks/positive_examples/aae3ba.json", true},
		{"a12396-no", "test_tracks/negative_examples/a12396.json", false},
		{"a41ffc-no", "test_tracks/negative_examples/a41ffc.json", false},
		{"a565bb-no", "test_tracks/negative_examples/a565bb.json", false},
		{"a6bb6d-no", "test_tracks/negative_examples/a6bb6d.json", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load the test track file
			data, err := os.ReadFile(tc.fileName)
			if err != nil {
				t.Fatalf("Failed to read test file %s: %v", tc.fileName, err)
			}

			// Parse the track
			var track model.AircraftTrack
			if err := json.Unmarshal(data, &track); err != nil {
				t.Fatalf("Failed to parse track data: %v", err)
			}

			// Basic track info
			t.Logf("Track %s: %d points", track.Hex, len(track.Points))

			// First test grid-based detection
			gridResult := detectSurveyPatternWithGrid(&track)

			// Log grid analysis
			t.Log("=== Grid Analysis ===")
			numHeadingBins := 360 / gridBucketDegrees
			for key, miles := range track.Grid {
				trackIdx := key[0]
				altIdx := key[1]
				if miles >= minDistanceOnParallelPath {
					oppositeTrackIdx := (trackIdx + (numHeadingBins / 2)) % numHeadingBins
					t.Logf("Significant track: heading=%d°, alt=%d ft, distance=%.1f miles",
						trackIdx*gridBucketDegrees,
						altIdx*gridBucketFeet,
						miles)
					oppositeKey := [2]int{oppositeTrackIdx, altIdx}
					t.Logf("  Opposite heading=%d° has distance=%.1f miles",
						oppositeTrackIdx*gridBucketDegrees,
						track.Grid[oppositeKey])

					for _, altOffset := range []int{-1, 1} {
						checkAltIdx := altIdx + altOffset
						if checkAltIdx >= 0 {
							adjKey := [2]int{oppositeTrackIdx, checkAltIdx}
							t.Logf("  Adjacent altitude bin %d ft has distance=%.1f miles",
								checkAltIdx*gridBucketFeet,
								track.Grid[adjKey])
						}
					}
				}
			}

			// Now test the exhaustive method
			t.Log("=== Exhaustive Analysis ===")
			segments := findStraightLineSegments(track.Points)
			t.Logf("Found %d straight line segments", len(segments))

			// Log details of each segment
			for i, seg := range segments {
				startPt := track.Points[seg.StartIdx]
				endPt := track.Points[seg.EndIdx]
				distance := distanceInMiles(
					startPt.Lat, startPt.Lon,
					endPt.Lat, endPt.Lon,
				)
				t.Logf("Segment %d: heading=%.1f°, distance=%.1f miles, alt=%d ft",
					i, seg.Heading, distance, startPt.Alt)
			}

			// Check for parallel segments
			if len(segments) >= 2 {
				t.Log("Parallel segment analysis:")
				parallelCount := 0
				for i := 0; i < len(segments); i++ {
					for j := i + 1; j < len(segments); j++ {
						// Calculate heading difference
						headingDiff := math.Abs(angleDifference(segments[i].Heading, segments[j].Heading))
						headingMatch := math.Abs(180-headingDiff) < parallelThreshold

						// Calculate segment lengths
						s1StartPoint := track.Points[segments[i].StartIdx]
						s1EndPoint := track.Points[segments[i].EndIdx]
						s2StartPoint := track.Points[segments[j].StartIdx]
						s2EndPoint := track.Points[segments[j].EndIdx]

						s1Length := distanceInMiles(s1StartPoint.Lat, s1StartPoint.Lon,
							s1EndPoint.Lat, s1EndPoint.Lon)
						s2Length := distanceInMiles(s2StartPoint.Lat, s2StartPoint.Lon,
							s2EndPoint.Lat, s2EndPoint.Lon)

						// Calculate midpoints and distance between segments
						s1MidLat := (s1StartPoint.Lat + s1EndPoint.Lat) / 2.0
						s1MidLon := (s1StartPoint.Lon + s1EndPoint.Lon) / 2.0
						s2MidLat := (s2StartPoint.Lat + s2EndPoint.Lat) / 2.0
						s2MidLon := (s2StartPoint.Lon + s2EndPoint.Lon) / 2.0

						distance := distanceInMiles(s1MidLat, s1MidLon, s2MidLat, s2MidLon)

						// Calculate altitude difference
						s1AvgAlt := (s1StartPoint.Alt + s1EndPoint.Alt) / 2
						s2AvgAlt := (s2StartPoint.Alt + s2EndPoint.Alt) / 2
						altitudeDiff := math.Abs(float64(s1AvgAlt - s2AvgAlt))

						isParallel := areSegmentsParallel(segments[i], segments[j], track.Points)
						t.Logf("Segments %d & %d: opposite=%v, len1=%.1f, len2=%.1f, dist=%.1f, altDiff=%.1f => parallel=%v",
							i, j, headingMatch, s1Length, s2Length, distance, altitudeDiff, isParallel)

						if isParallel {
							parallelCount++
						}
					}
				}
				t.Logf("Found %d parallel segment pairs", parallelCount)
			}

			exhaustiveResult := detectSurveyPatternExhaustive(&track)

			// Final results
			t.Logf("Grid detection: %v", gridResult)
			t.Logf("Exhaustive detection: %v", exhaustiveResult)

			// Check if either detection method worked
			combinedResult := gridResult || exhaustiveResult
			if combinedResult != tc.expected {
				t.Errorf("Expected detection=%v, got=%v (grid=%v, exhaustive=%v)",
					tc.expected, combinedResult, gridResult, exhaustiveResult)
			}
		})
	}
}
