# Survey Plane Finder

Detects aircraft flying aerial survey patterns (grid/parallel line flights) using live ADS-B data from [adsb.lol](https://adsb.lol), and publishes detections to a real-time web map.

## How it works

The detector polls the adsb.lol API every 3 seconds for aircraft positions within a configurable radius. For each aircraft, it builds a track and analyzes it for survey-like flight patterns — specifically, parallel flight segments at consistent altitude flying in opposite directions.

Two detection methods run in parallel:
- **Grid-based**: Accumulates miles flown per heading/altitude bin and looks for significant mileage in opposite directions
- **Exhaustive**: Identifies straight-line segments and checks for parallel pairs within distance/altitude thresholds

Detections are uploaded as GeoJSON to a Cloudflare R2 bucket, where a static frontend renders them on a MapLibre GL JS map.

## Architecture

```
adsb.lol API → Go detector (runs at home) → Cloudflare R2 (JSON files) → Static frontend (Cloudflare Pages)
```

- **Live data** (`live/current.json`): uploaded every 60 seconds, contains currently-active survey aircraft
- **Daily archive** (`archive/YYYY-MM-DD.json`): uploaded every 5 minutes, accumulates all detections for the day
- **Date index** (`index.json`): lists available archive dates for the frontend date picker

## Running the detector

### With Docker

```bash
docker run -d \
  -e R2_ENDPOINT=https://<account>.r2.cloudflarestorage.com \
  -e R2_BUCKET_NAME=survey-planes \
  -e R2_ACCESS_KEY_ID=<key> \
  -e R2_SECRET_ACCESS_KEY=<secret> \
  ghcr.io/iandees/survey-plane-finder:latest
```

### From source

```bash
go build -o survey-plane-finder .
R2_ENDPOINT=https://... R2_BUCKET_NAME=... R2_ACCESS_KEY_ID=... R2_SECRET_ACCESS_KEY=... ./survey-plane-finder
```

### Without R2 (local only)

If no `R2_ENDPOINT` is set, the detector runs in local-only mode — logging detections to stdout and writing `aircraft_tracks.json` locally.

```bash
go run .
```

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `R2_ENDPOINT` | No | Cloudflare R2 S3-compatible endpoint URL |
| `R2_BUCKET_NAME` | If R2 | R2 bucket name |
| `R2_ACCESS_KEY_ID` | If R2 | R2 API token access key |
| `R2_SECRET_ACCESS_KEY` | If R2 | R2 API token secret key |

## Frontend

The `web/` directory contains a static site that reads from the R2 bucket and renders detections on a map. It can be hosted on Cloudflare Pages or any static hosting.

For local development with sample data:

```bash
cd web && python3 -m http.server 8080
```

## Detection parameters

Tunable constants in `main.go`:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `surveyMinAltitude` | 1,000 ft | Ignore aircraft below this |
| `surveyMaxAltitude` | 20,000 ft | Ignore aircraft above this |
| `minDistanceOnParallelPath` | 20 mi | Minimum segment length to count |
| `maxDistanceBetweenParallelTracksMiles` | 3.0 mi | Max distance between parallel segments |
| `maxAltitudeDiff` | 200 ft | Max altitude difference between parallel segments |
| `minParallelPaths` | 2 | Minimum parallel pairs to flag |
