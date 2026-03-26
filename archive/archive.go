package archive

import (
	"math"
	"survey-plane-finder/geojson"
	"survey-plane-finder/model"
	"sync"
	"time"
)

type Archive struct {
	mu              sync.RWMutex
	detections      map[string]*model.AircraftTrack
	priorFeatures   []geojson.Feature // features loaded from a previous archive file
	priorICAOs      map[string]bool   // ICAOs from prior features, to avoid duplicates
}

func New() *Archive {
	return &Archive{
		detections:    make(map[string]*model.AircraftTrack),
		priorICAOs:    make(map[string]bool),
	}
}

// LoadExisting seeds the archive with features from a previously-uploaded archive file.
// These features are preserved in output unless a new detection with the same ICAO arrives.
func (a *Archive) LoadExisting(fc geojson.FeatureCollection) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.priorFeatures = fc.Features
	a.priorICAOs = make(map[string]bool)
	for _, f := range fc.Features {
		if icao, ok := f.Properties["icao"].(string); ok {
			a.priorICAOs[icao] = true
		}
	}
}

func (a *Archive) AddOrUpdate(track *model.AircraftTrack) {
	a.mu.Lock()
	defer a.mu.Unlock()

	points := make([]model.TrackPoint, len(track.Points))
	copy(points, track.Points)

	a.detections[track.Hex] = &model.AircraftTrack{
		Hex:             track.Hex,
		Flight:          track.Flight,
		Points:          points,
		Flagged:         track.Flagged,
		DetectionMethod: track.DetectionMethod,
		LastSeen:        track.LastSeen,
		Grid:            track.Grid,
	}
}

func (a *Archive) Detections() []*model.AircraftTrack {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]*model.AircraftTrack, 0, len(a.detections))
	for _, d := range a.detections {
		result = append(result, d)
	}
	return result
}

func (a *Archive) BuildCollection(date string) geojson.FeatureCollection {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Start with new detections (these take priority over prior features)
	newICAOs := make(map[string]bool)
	features := make([]geojson.Feature, 0, len(a.detections)+len(a.priorFeatures))
	for _, track := range a.detections {
		if len(track.Points) < 2 {
			continue
		}
		feature := geojson.BuildLiveFeature(track)

		firstPoint := track.Points[0]
		lastPoint := track.Points[len(track.Points)-1]
		duration := lastPoint.Timestamp.Sub(firstPoint.Timestamp)
		feature.Properties["duration_min"] = int(math.Round(duration.Minutes()))
		feature.Properties["track_miles"] = calculateTrackMiles(track.Points)
		feature.Properties["active"] = time.Since(track.LastSeen) < 5*time.Minute

		feature.Geometry.Coordinates = geojson.SimplifyTrack(feature.Geometry.Coordinates, 0.0001)

		newICAOs[track.Hex] = true
		features = append(features, feature)
	}

	// Add prior features that haven't been superseded by new detections
	for _, f := range a.priorFeatures {
		if icao, ok := f.Properties["icao"].(string); ok {
			if !newICAOs[icao] {
				features = append(features, f)
			}
		}
	}

	return geojson.FeatureCollection{
		Type:        "FeatureCollection",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Features:    features,
	}
}

func (a *Archive) ResetForNewDay() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.detections = make(map[string]*model.AircraftTrack)
	a.priorFeatures = nil
	a.priorICAOs = make(map[string]bool)
}

func calculateTrackMiles(points []model.TrackPoint) float64 {
	total := 0.0
	for i := 1; i < len(points); i++ {
		total += haversineDistance(points[i-1].Lat, points[i-1].Lon, points[i].Lat, points[i].Lon)
	}
	return math.Round(total*10) / 10
}

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMiles = 3958.8
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	dlat := lat2Rad - lat1Rad
	dlon := lon2Rad - lon1Rad
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dlon/2)*math.Sin(dlon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMiles * c
}
