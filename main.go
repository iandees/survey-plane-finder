package main

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"survey-plane-finder/archive"
	"survey-plane-finder/bincraft"
	"survey-plane-finder/geojson"
	"survey-plane-finder/model"
	"survey-plane-finder/r2"
	"time"
)

// Global map to store aircraft tracks
var aircraftTracks = make(map[string]*model.AircraftTrack)

// Configuration parameters
const (
	// Wait this long before removing aircraft tracks after not seeing them
	trackTimeout = 5 * time.Minute
	// Maximum altitude for survey aircraft in feet
	surveyMaxAltitude = 20000
	// Minimum altitude for survey aircraft in feet
	surveyMinAltitude = 1000
	// Maximum degrees of difference to consider tracks parallel
	parallelThreshold = 5
	// Minimum track points needed before analysis
	minTrackPoints = 10
	// Minimum number of parallel paths to flag as survey
	minParallelPaths = 2
	// Minimum distance (in miles) on each parallel path to consider it a survey path
	minDistanceOnParallelPath = 20
	// Time between API polls
	pollInterval = 3 * time.Second
	// Maximum altitude difference for parallel tracks in feet
	maxAltitudeDiff = 200
	// Maximum distance between parallel tracks in miles
	maxDistanceBetweenParallelTracksMiles = 3.0
	// Number of degrees each grid bucket covers
	gridBucketDegrees = 5
	// Number of feet of altitude each grid bucket covers
	gridBucketFeet = 100

	// Heatmap image generation interval
	heatmapGenerationInterval = 30 * time.Second
	// Cell size in pixels for the heatmap
	cellSize = 10
	// Maximum color intensity for the heatmap
	maxColorIntensity = 255
	// Minimum miles for max intensity
	minMilesForMaxIntensity = 5.0
	// Directory to store heatmap images
	heatmapDirectory = "heatmaps"
)

// fetchAircraftData retrieves aircraft data from adsb.lol binCraft API
func fetchAircraftData(south, north, west, east float64) ([]model.Aircraft, error) {
	url := fmt.Sprintf("https://adsb.lol/re-api/?binCraft&zstd&box=%f,%f,%f,%f",
		south, north, west, east)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "survey-plane-finder (https://github.com/iandees/survey-plane-finder)")
	req.Header.Set("Referer", "https://adsb.lol/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	bcResp, err := bincraft.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("decode binCraft: %w", err)
	}

	aircraft := make([]model.Aircraft, 0, len(bcResp.Aircraft))
	for _, ac := range bcResp.Aircraft {
		if !ac.HasPosition {
			continue
		}
		a := model.Aircraft{
			Hex:    ac.Hex,
			Flight: ac.Callsign,
			Lat:    ac.Lat,
			Lon:    ac.Lon,
			Track:  ac.Track,
		}
		if ac.HasAltBaro {
			a.Alt = float64(ac.AltBaro)
		}
		if ac.HasGS {
			a.GS = ac.GS
		}
		aircraft = append(aircraft, a)
	}

	return aircraft, nil
}

func updateAircraftTracks(aircraft []model.Aircraft) {
	now := time.Now()

	for _, a := range aircraft {
		if a.Hex == "" || a.Lat == 0 || a.Lon == 0 {
			continue // Skip aircraft with incomplete data
		}

		altitude := a.Altitude()

		// Ignore aircraft on the ground or too high
		if altitude < surveyMinAltitude || altitude > surveyMaxAltitude {
			continue
		}

		track, exists := aircraftTracks[a.Hex]
		if !exists {
			track = &model.AircraftTrack{
				Hex:      a.Hex,
				Flight:   a.Flight,
				Points:   make([]model.TrackPoint, 0),
				Grid:     initTrackingGrid(),
				LastSeen: now,
			}
			aircraftTracks[a.Hex] = track
		}

		// Update flight number if available
		if a.Flight != "" && track.Flight == "" {
			track.Flight = a.Flight
		}

		// Add new point to track
		track.Points = append(track.Points, model.TrackPoint{
			Lat:       a.Lat,
			Lon:       a.Lon,
			Alt:       altitude,
			Track:     a.Track,
			Speed:     a.GS,
			Timestamp: now,
		})

		// Calculate distance from last point and record that distance spent in the grid based on the previous point's track and altitude.
		if len(track.Points) > 1 {
			thisPoint := track.Points[len(track.Points)-1]
			lastPoint := track.Points[len(track.Points)-2]
			updateGrid(track, thisPoint, lastPoint)
		}

		track.LastSeen = now

		// Analyze track for survey pattern if aircraft is below max altitude
		// Skip analysis for ground aircraft
		if len(track.Points) >= minTrackPoints && !track.Flagged {
			if detectSurveyPatternExhaustive(track) {
				track.Flagged = true
				track.DetectionMethod = "exhaustive"
				flightID := track.Hex
				if track.Flight != "" {
					flightID = fmt.Sprintf("%s (%s)", track.Flight, track.Hex)
				}
				log.Printf("SURVEY AIRCRAFT DETECTED: %s - https://globe.adsb.lol/?icao=%s",
					flightID, track.Hex)
			}
		}
	}

	// Clean up old tracks
	cleanupOldTracks(now)
}

func detectSurveyPatternWithGrid(track *model.AircraftTrack) bool {
	// Check if we have enough data in the grid
	if len(track.Points) < minTrackPoints {
		return false
	}

	// Look for opposite tracks with similar altitudes and significant distance
	numHeadingBins := len(track.Grid)

	for trackIdx := 0; trackIdx < numHeadingBins; trackIdx++ {
		// Calculate the opposite direction index (180 degrees away)
		oppositeTrackIdx := (trackIdx + (numHeadingBins / 2)) % numHeadingBins

		// Check each altitude
		for altIdx := 0; altIdx < len(track.Grid[trackIdx]); altIdx++ {
			// Check if we have at least minDistanceOnParallelPath miles in this direction
			if track.Grid[trackIdx][altIdx] < minDistanceOnParallelPath {
				continue
			}

			// Check if the opposite direction has sufficient miles at the same altitude
			if track.Grid[oppositeTrackIdx][altIdx] >= minDistanceOnParallelPath {
				return true
			}

			// Also check adjacent altitude bins (to account for slight altitude changes)
			for altOffset := -1; altOffset <= 1; altOffset += 2 { // Just check -1 and +1
				checkAltIdx := altIdx + altOffset
				if checkAltIdx >= 0 && checkAltIdx < len(track.Grid[oppositeTrackIdx]) {
					if track.Grid[oppositeTrackIdx][checkAltIdx] >= minDistanceOnParallelPath {
						return true
					}
				}
			}
		}
	}

	return false
}

func initTrackingGrid() [][]float64 {
	grid := make([][]float64, 360/gridBucketDegrees)
	for i := range grid {
		grid[i] = make([]float64, surveyMaxAltitude/gridBucketFeet)
	}
	return grid
}

func updateGrid(track *model.AircraftTrack, oldPoint model.TrackPoint, newPoint model.TrackPoint) {
	altitudeCell := int(math.Floor(float64(oldPoint.Alt) / gridBucketFeet))   // 1000 ft cells
	trackCell := int(math.Floor(float64(oldPoint.Track) / gridBucketDegrees)) // 5 degree cells

	// Ensure we don't go out of bounds
	if trackCell < 0 || trackCell >= len(track.Grid) {
		return
	}
	if altitudeCell < 0 || altitudeCell >= len(track.Grid[0]) {
		return
	}

	track.Grid[trackCell][altitudeCell] += distanceInMiles(oldPoint.Lat, oldPoint.Lon, newPoint.Lat, newPoint.Lon)
}

// cleanupOldTracks removes tracks that haven't been updated recently
func cleanupOldTracks(now time.Time) {
	for hex, track := range aircraftTracks {
		if now.Sub(track.LastSeen) > trackTimeout {
			delete(aircraftTracks, hex)
		}
	}
}

// detectSurveyPatternExhaustive analyzes an aircraft track to determine if it's flying a survey pattern
func detectSurveyPatternExhaustive(track *model.AircraftTrack) bool {
	// Skip if not enough points
	if len(track.Points) < minTrackPoints {
		return false
	}

	// Check if we have enough straight line segments
	segments := findStraightLineSegments(track.Points)
	if len(segments) < minParallelPaths {
		return false
	}

	// Check for parallel segments
	parallelCount := 0
	for i := 0; i < len(segments); i++ {
		for j := i + 1; j < len(segments); j++ {
			if areSegmentsParallel(segments[i], segments[j], track.Points) {
				parallelCount++
				if parallelCount >= minParallelPaths-1 {
					return true
				}
			}
		}
	}

	return false
}

// LineSegment represents a straight line segment in the track
type LineSegment struct {
	StartIdx int
	EndIdx   int
	Heading  float64
}

// findStraightLineSegments identifies straight line segments in a track
func findStraightLineSegments(points []model.TrackPoint) []LineSegment {
	const (
		minSegmentPoints = 5
		maxHeadingDev    = 5.0
	)

	var segments []LineSegment
	start := 0

	for i := 1; i < len(points); i++ {
		// If heading changes significantly or we're at the end
		if i == len(points)-1 || math.Abs(angleDifference(points[i].Track, points[start].Track)) > maxHeadingDev {
			// If segment is long enough, add it
			if i-start >= minSegmentPoints {
				segments = append(segments, LineSegment{
					StartIdx: start,
					EndIdx:   i,
					Heading:  points[start].Track,
				})
			}
			start = i
		}
	}

	return segments
}

// distanceInMiles calculates the distance between two points in miles
func distanceInMiles(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMiles = 3958.8 // Earth's radius in miles

	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	// Haversine formula
	dlat := lat2Rad - lat1Rad
	dlon := lon2Rad - lon1Rad
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dlon/2)*math.Sin(dlon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadiusMiles * c

	return distance
}

// areSegmentsParallel determines if two line segments are approximately parallel,
// in opposite directions, within 3 miles of each other, at similar altitudes,
// and at least 20 miles long
func areSegmentsParallel(s1, s2 LineSegment, points []model.TrackPoint) bool {
	// For survey patterns, we want segments running in opposite directions (180° apart)
	headingDiff := math.Abs(angleDifference(s1.Heading, s2.Heading))

	// Check if headings are approximately 180° apart (within the parallelThreshold)
	headingMatch := math.Abs(180-headingDiff) < parallelThreshold

	if !headingMatch {
		return false
	}

	// Calculate segment lengths and check if both are at least 20 miles long
	s1StartPoint := points[s1.StartIdx]
	s1EndPoint := points[s1.EndIdx]
	s2StartPoint := points[s2.StartIdx]
	s2EndPoint := points[s2.EndIdx]

	// Calculate length of segment 1
	s1Length := distanceInMiles(s1StartPoint.Lat, s1StartPoint.Lon, s1EndPoint.Lat, s1EndPoint.Lon)
	if s1Length < minDistanceOnParallelPath {
		return false
	}

	// Calculate length of segment 2
	s2Length := distanceInMiles(s2StartPoint.Lat, s2StartPoint.Lon, s2EndPoint.Lat, s2EndPoint.Lon)
	if s2Length < minDistanceOnParallelPath {
		return false
	}

	// Check if segments are within close to one another
	s1MidLat := (s1StartPoint.Lat + s1EndPoint.Lat) / 2.0
	s1MidLon := (s1StartPoint.Lon + s1EndPoint.Lon) / 2.0
	s2MidLat := (s2StartPoint.Lat + s2EndPoint.Lat) / 2.0
	s2MidLon := (s2StartPoint.Lon + s2EndPoint.Lon) / 2.0

	distance := distanceInMiles(s1MidLat, s1MidLon, s2MidLat, s2MidLon)

	if distance > maxDistanceBetweenParallelTracksMiles {
		return false
	}

	// Check if segments are at similar altitudes
	s1AvgAlt := (s1StartPoint.Alt + s1EndPoint.Alt) / 2.0
	s2AvgAlt := (s2StartPoint.Alt + s2EndPoint.Alt) / 2.0

	altitudeDiff := math.Abs(float64(s1AvgAlt - s2AvgAlt))

	return altitudeDiff <= maxAltitudeDiff
}

// angleDifference calculates the smallest difference between two angles
func angleDifference(a, b float64) float64 {
	diff := math.Abs(a - b)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff
}

// outputTracksAsGeoJSON returns a GeoJSON FeatureCollection of all aircraft tracks
func outputTracksAsGeoJSON() string {
	type GeoJSONPoint [2]float64

	type GeoJSONLineString struct {
		Type        string         `json:"type"`
		Coordinates []GeoJSONPoint `json:"coordinates"`
	}

	type GeoJSONFeature struct {
		Type       string                 `json:"type"`
		Geometry   GeoJSONLineString      `json:"geometry"`
		Properties map[string]interface{} `json:"properties"`
	}

	type GeoJSONFeatureCollection struct {
		Type     string           `json:"type"`
		Features []GeoJSONFeature `json:"features"`
	}

	// Create feature collection
	fc := GeoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: make([]GeoJSONFeature, 0),
	}

	// Add each aircraft track as a feature
	for _, track := range aircraftTracks {
		if len(track.Points) < 2 {
			continue
		}

		// Create coordinates array
		coords := make([]GeoJSONPoint, 0, len(track.Points))
		for _, point := range track.Points {
			coords = append(coords, GeoJSONPoint{point.Lon, point.Lat}) // GeoJSON uses [lon, lat] order
		}

		// Create feature
		feature := GeoJSONFeature{
			Type: "Feature",
			Geometry: GeoJSONLineString{
				Type:        "LineString",
				Coordinates: coords,
			},
			Properties: map[string]interface{}{
				"icao":           track.Hex,
				"flight":         track.Flight,
				"flagged":        track.Flagged,
				"altitude":       track.Points[len(track.Points)-1].Alt,
				"stroke-width":   2,
				"stroke-opacity": 1,
				"stroke":         calculateColor(track),
			},
		}

		fc.Features = append(fc.Features, feature)
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(fc)
	if err != nil {
		return "{\"type\":\"FeatureCollection\",\"features\":[]}"
	}

	return string(jsonBytes)
}

// calculateColor determines the color for the track based on its hex code
func calculateColor(track *model.AircraftTrack) string {
	// If this is a flagged survey aircraft, always use red
	if track.Flagged {
		return "#FF0000"
	}

	// Generate a stable color from the hex code
	hexValue := track.Hex

	h := 0
	for i := 0; i < len(hexValue); i++ {
		h = 31*h + int(hexValue[i])
	}

	// Generate RGB values between 0-255, avoiding colors that are too dark
	r := 50 + (h % 206)
	g := 50 + ((h >> 8) % 206)
	b := 50 + ((h >> 16) % 206)

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// generateHeatmapImages creates heatmap visualizations for all aircraft tracks
func writeHeatmapImages() {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(heatmapDirectory, 0755); err != nil {
		log.Printf("Error creating heatmap directory: %v", err)
		return
	}

	// Generate heatmap for each aircraft
	for hex, track := range aircraftTracks {
		if len(track.Points) < minTrackPoints {
			continue // Skip tracks with too few points
		}

		go generateAircraftHeatmap(hex, track)
	}
}

// dataDir is the directory for persistent data files. Set via DATA_DIR env var.
var dataDir = "."

func dataPath(filename string) string {
	return filepath.Join(dataDir, filename)
}

func saveAircraftTracks() error {
	f, err := os.Create(dataPath("saved_tracks.gob"))
	if err != nil {
		return fmt.Errorf("error creating tracks file: %v", err)
	}
	defer f.Close()

	if err := gob.NewEncoder(f).Encode(aircraftTracks); err != nil {
		return fmt.Errorf("error encoding tracks: %v", err)
	}
	return nil
}

func loadAircraftTracks() error {
	f, err := os.Open(dataPath("saved_tracks.gob"))
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No saved tracks file found, starting fresh")
			return nil
		}
		return fmt.Errorf("error reading saved tracks: %v", err)
	}
	defer f.Close()

	return gob.NewDecoder(f).Decode(&aircraftTracks)
}

func generateAircraftHeatmap(hex string, track *model.AircraftTrack) {
	// Find maximum value in grid for normalization
	maxValue := 0.1 // Avoid division by zero

	for trackIdx := range track.Grid {
		for altIdx := range track.Grid[trackIdx] {
			if track.Grid[trackIdx][altIdx] > maxValue {
				maxValue = track.Grid[trackIdx][altIdx]
			}
		}
	}

	// If no data, skip
	if maxValue == 0.1 {
		return
	}

	// Cap maximum for better visualization
	if maxValue > minMilesForMaxIntensity {
		maxValue = minMilesForMaxIntensity
	}

	// Create image (heading on X axis, altitude on Y axis)
	width := len(track.Grid) * cellSize
	height := 0
	if len(track.Grid) > 0 {
		height = len(track.Grid[0]) * cellSize
	}

	if width == 0 || height == 0 {
		return // Skip if dimensions are invalid
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with background color (black)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{0, 0, 0, 255})
		}
	}

	// Draw each cell with intensity based on miles flown
	for trackIdx := range track.Grid {
		for altIdx := range track.Grid[trackIdx] {
			value := track.Grid[trackIdx][altIdx]
			if value <= 0 {
				continue
			}

			// Normalize value and create color (use logarithmic scale for better visibility)
			// This helps make smaller values more visible
			normalizedValue := int(math.Log1p(value/maxValue*9.0) / math.Log(10) * maxColorIntensity)
			if normalizedValue > maxColorIntensity {
				normalizedValue = maxColorIntensity
			}

			cellColor := color.RGBA{
				R: uint8(normalizedValue),
				G: uint8(normalizedValue),
				B: uint8(normalizedValue),
				A: 255,
			}

			// If this is a survey aircraft, highlight opposite pairs
			if track.Flagged {
				oppositeHeadingIdx := (trackIdx + (len(track.Grid) / 2)) % len(track.Grid)
				if track.Grid[oppositeHeadingIdx][altIdx] > 0 {
					// Use red for flagged survey patterns
					cellColor.R = 255
					cellColor.G = 0
					cellColor.B = 0
				}
			}

			// Fill the cell
			startX := trackIdx * cellSize
			startY := (len(track.Grid[0]) - 1 - altIdx) * cellSize // Invert Y axis so higher altitudes are at the top
			for x := startX; x < startX+cellSize; x++ {
				for y := startY; y < startY+cellSize; y++ {
					if x >= 0 && x < width && y >= 0 && y < height {
						img.Set(x, y, cellColor)
					}
				}
			}
		}
	}

	// Add labels (optional - would require more complex image processing)

	// Save the image
	filename := fmt.Sprintf("%s.png", hex)

	filePath := filepath.Join(heatmapDirectory, filename)
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("Error creating heatmap file: %v", err)
		return
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		log.Printf("Error encoding heatmap image: %v", err)
	}
}

func writeTracksToFile(file string) {
	os.WriteFile(file, []byte(outputTracksAsGeoJSON()), 0644)
}

func main() {
	log.Println("Starting ADSB survey aircraft detector")

	if dir := os.Getenv("DATA_DIR"); dir != "" {
		dataDir = dir
		log.Printf("Using data directory: %s", dataDir)
	}

	// Default bounding box (Minneapolis area, ~250mi radius)
	south := 40.0
	north := 50.0
	west := -98.0
	east := -88.0
	trackFile := dataPath("aircraft_tracks.json")

	// Try to load saved tracks
	if err := loadAircraftTracks(); err != nil {
		log.Printf("Error loading aircraft tracks: %v", err)
	} else {
		log.Printf("Loaded %d aircraft tracks from disk", len(aircraftTracks))
	}

	// R2 upload configuration (optional — runs without R2 if not configured)
	var r2Client *r2.Client
	var todayArchive *archive.Archive
	var archiveDates []string

	r2Endpoint := os.Getenv("R2_ENDPOINT")
	if r2Endpoint != "" {
		client, err := r2.NewClient(r2.Config{
			Bucket:          os.Getenv("R2_BUCKET_NAME"),
			AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
			Endpoint:        r2Endpoint,
		})
		if err != nil {
			log.Fatalf("Failed to create R2 client: %v", err)
		}
		r2Client = client
		todayArchive = archive.New()

		// Load existing index from R2 so we don't overwrite it
		var existingIndex struct {
			Dates []string `json:"dates"`
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := r2Client.DownloadJSON(ctx, "index.json", &existingIndex); err != nil {
			log.Printf("Could not load existing index.json: %v", err)
		}
		cancel()
		archiveDates = existingIndex.Dates
		log.Printf("R2 upload enabled (loaded %d existing archive dates)", len(archiveDates))

		// Load existing archive for today so we don't overwrite it
		today := time.Now().Format("2006-01-02")
		var existingArchive geojson.FeatureCollection
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		if err := r2Client.DownloadJSON(ctx2, fmt.Sprintf("archive/%s.json", today), &existingArchive); err != nil {
			log.Printf("Could not load existing archive for %s: %v", today, err)
		} else if len(existingArchive.Features) > 0 {
			todayArchive.LoadExisting(existingArchive)
			log.Printf("Loaded %d existing detections for %s", len(existingArchive.Features), today)
		}
		cancel2()
	} else {
		log.Println("R2 not configured, running in local-only mode")
	}

	// Ensure we save data when shutting down
	defer func() {
		err := saveAircraftTracks()
		if err != nil {
			log.Printf("Error saving aircraft tracks on shutdown: %v", err)
		} else {
			log.Printf("Saved %d aircraft tracks to disk on shutdown", len(aircraftTracks))
		}
	}()

	pollTicker := time.NewTicker(pollInterval)
	trackFileTicker := time.NewTicker(heatmapGenerationInterval)

	var liveUploadTicker, archiveUploadTicker *time.Ticker
	if r2Client != nil {
		liveUploadTicker = time.NewTicker(60 * time.Second)
		archiveUploadTicker = time.NewTicker(5 * time.Minute)
	} else {
		liveUploadTicker = time.NewTicker(time.Hour)
		liveUploadTicker.Stop()
		archiveUploadTicker = time.NewTicker(time.Hour)
		archiveUploadTicker.Stop()
	}

	currentDate := time.Now().Format("2006-01-02")

	for {
		select {
		case <-pollTicker.C:
			aircraft, err := fetchAircraftData(south, north, west, east)
			if err != nil {
				log.Printf("Error fetching data: %v", err)
			}

			//log.Printf("Fetched data for %d aircraft", len(aircraft))
			updateAircraftTracks(aircraft)
		case <-trackFileTicker.C:
			writeTracksToFile(trackFile)
			announceMatches()
			//writeHeatmapImages()

			if err := saveAircraftTracks(); err != nil {
				log.Printf("Error saving aircraft tracks: %v", err)
			} else {
				log.Printf("Saved %d aircraft tracks to disk", len(aircraftTracks))
			}
		case <-liveUploadTicker.C:
			if r2Client != nil {
				collection := geojson.BuildLiveCollection(aircraftTracks)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				err := r2Client.UploadJSON(ctx, "live/current.json", collection)
				cancel()
				if err != nil {
					log.Printf("Error uploading live data: %v", err)
				} else {
					log.Printf("Uploaded live data: %d aircraft", len(collection.Features))
				}

				// Update archive with all currently-flagged aircraft
				for _, track := range aircraftTracks {
					if track.Flagged {
						todayArchive.AddOrUpdate(track)
					}
				}
			}
		case <-archiveUploadTicker.C:
			if r2Client != nil {
				today := time.Now().Format("2006-01-02")
				if today != currentDate {
					todayArchive.ResetForNewDay()
					currentDate = today
				}

				// Ensure current date is in archiveDates
				if len(archiveDates) == 0 || archiveDates[len(archiveDates)-1] != currentDate {
					archiveDates = append(archiveDates, currentDate)
				}

				// Upload index.json with all known dates
				idxCtx, idxCancel := context.WithTimeout(context.Background(), 10*time.Second)
				indexData := map[string]interface{}{"dates": archiveDates}
				err := r2Client.UploadJSON(idxCtx, "index.json", indexData)
				idxCancel()
				if err != nil {
					log.Printf("Error uploading index.json: %v", err)
				}

				collection := todayArchive.BuildCollection(currentDate)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				err = r2Client.UploadJSON(ctx, fmt.Sprintf("archive/%s.json", currentDate), collection)
				cancel()
				if err != nil {
					log.Printf("Error uploading archive: %v", err)
				} else {
					log.Printf("Uploaded archive for %s: %d detections", currentDate, len(collection.Features))
				}
			}
		}
	}
}

func announceMatches() {
	// Log a URL that includes the icao for all flagged aircraft as a comma-separated list
	icaoMatches := []string{}
	for _, track := range aircraftTracks {
		if track.Flagged {
			icaoMatches = append(icaoMatches, track.Hex)
		}
	}

	if len(icaoMatches) > 0 {
		sort.Slice(icaoMatches, func(i, j int) bool { return icaoMatches[i] < icaoMatches[j] })
		icaoList := strings.Join(icaoMatches, ",")
		log.Printf("%d flagged aircraft detected: https://globe.adsb.lol/?icao=%s", len(icaoMatches), icaoList)
	}
}
