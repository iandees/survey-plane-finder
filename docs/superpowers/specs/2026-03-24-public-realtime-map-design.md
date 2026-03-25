# Survey Plane Finder — Public Real-Time Map

## Overview

Transform the existing Go-based survey plane detector from a CLI tool into a publicly accessible web application that shows survey aircraft on a real-time map. The target audience is the general public and journalists who want to understand what aerial surveys are happening in their area.

## Architecture

**Go Detector (runs at home) → Cloudflare R2 (JSON files) → Static Frontend (Cloudflare Pages)**

The Go detector continues polling ADSB.lol every 3 seconds and running survey pattern detection. Instead of only logging to stdout and writing a local GeoJSON file, it also uploads JSON files to a Cloudflare R2 bucket on a schedule. The frontend is a static HTML/JS site hosted on Cloudflare Pages that fetches these JSON files directly from the R2 public bucket. There is no API server or cloud database.

### Why R2 + Static Frontend

- Zero cloud compute cost — R2 has free egress, Pages is free-tier
- No server to maintain in the cloud — JSON files ARE the API
- GeoJSON is consumed natively by MapLibre, no conversion layer needed
- Easy to scale to multiple detector regions — each writes to its own path in the bucket

## R2 Bucket File Layout

```
survey-planes/
├── live/
│   └── current.json          # Overwritten every ~60s
│                               # Currently-active survey aircraft + tracks from last ~30 min
│
├── archive/
│   ├── 2026-03-24.json       # Today's detections so far (updated every ~5 min)
│   ├── 2026-03-23.json       # Yesterday
│   └── ...
│
└── index.json                # List of available archive dates (updated when new day starts)
```

### Upload Cadence

- `live/current.json`: every ~60 seconds
- `archive/YYYY-MM-DD.json`: every ~5 minutes (today's file), immutable for past days
- `index.json`: once when a new day starts

### Retention

Archive files are kept indefinitely. A busy day is ~50-100KB of GeoJSON, so storage costs are negligible.

## Data Model

Both live and archive files use GeoJSON FeatureCollection format. Each Feature is one survey aircraft with its track as a LineString.

### live/current.json

```json
{
  "type": "FeatureCollection",
  "generated_at": "2026-03-24T14:32:00Z",
  "features": [
    {
      "type": "Feature",
      "geometry": {
        "type": "LineString",
        "coordinates": [[-93.21, 44.95], [-93.18, 44.96], "..."]
      },
      "properties": {
        "icao": "a1b2c3",
        "callsign": "N12345",
        "first_seen": "2026-03-24T13:45:00Z",
        "last_seen": "2026-03-24T14:31:50Z",
        "altitude_ft": 3500,
        "speed_kts": 120,
        "detection_method": "grid",  // "grid" or "exhaustive"
        "survey_bounds": [-93.4, 44.8, -93.0, 45.1],
        "globe_url": "https://globe.adsb.lol/?icao=a1b2c3",
        "registration_url": "https://registry.faa.gov/..."
      }
    }
  ]
}
```

### archive/YYYY-MM-DD.json

Same format as live, with additional summary fields on each feature:

```json
{
  "properties": {
    "active": false,
    "duration_min": 153,
    "track_miles": 245.1,
    "parallel_paths": 14,
    "parallel_path_spacing_mi": 0.4
  }
}
```

### Track Simplification

For archive files with long tracks, apply Douglas-Peucker simplification to keep file sizes small while preserving the visual shape of survey patterns.

## Frontend

### Technology

- **MapLibre GL JS** — open-source, free, renders GeoJSON natively
- **Vanilla JS** — no framework needed for this scope
- **Hosted on Cloudflare Pages** — free tier, automatic HTTPS, global CDN

### Layout: Side Panel

Full-screen map with a persistent right-side panel (280px wide).

**Top bar** contains:
- Logo/title: "Survey Plane Finder"
- Live stats: count of active aircraft, count of today's detections
- Date picker: left/right arrows to step through days, click date for calendar, "Live" button to return to real-time view

**Map area** (left):
- MapLibre GL JS with a dark basemap (e.g., MapTiler free tier or Protomaps self-hosted tiles)
- Survey aircraft tracks rendered as colored LineStrings
- Each aircraft gets a unique color for distinguishability
- Active aircraft show a pulsing dot at the head of the track
- Clicking a track opens the detail view and zooms to the survey area

**Side panel** (right, 280px):
- **Live mode tabs**: "Live" (currently active aircraft) and "Today" (includes completed)
- **History mode**: shows all detections for the selected date
- Each aircraft item shows: color dot, callsign, altitude, duration, path count
- Completed aircraft shown dimmed
- Clicking an item zooms the map to `survey_bounds` and opens the detail view

### Detail View

When an aircraft is selected, the detail view slides over the list in the side panel:

- **Header**: callsign, ICAO hex, active/completed badge, close button
- **Stat grid** (2 columns): altitude, speed, duration, parallel paths, track distance, time range
- **Survey type** (stretch goal): icon, label, confidence level, and a human-readable explanation of why this classification was chosen (e.g., "Typical LiDAR surveys operate at low altitude (1,000-4,000 ft) with tight parallel spacing")
- **External links**: ADSB.lol track, FAA registration, FlightAware, ADS-B Exchange — all open in new tabs

### URL State

Date and selected aircraft encoded in the URL hash for shareable links:
- `#date=2026-03-22&icao=a1b2c3`
- Live view is the default (no hash or `#live`)

### Data Loading

- **Live mode**: polls `live/current.json` every 60 seconds
- **History mode**: fetches `archive/YYYY-MM-DD.json` once per day viewed, cached in memory
- **Date index**: fetches `index.json` on page load to know which archive dates are available

## Changes to the Go Detector

### New Functionality

1. **R2 upload module**: uses the S3-compatible API (R2 is S3-compatible) to PUT JSON files to the bucket on schedule
2. **Archive accumulator**: maintains a list of all detections seen today (including aircraft that have timed out), serialized as GeoJSON for the daily archive file
3. **Track simplification**: Douglas-Peucker simplification for archive tracks to reduce file size
4. **External link generation**: compute FAA registration URL, FlightAware URL, ADS-B Exchange URL from ICAO hex and callsign
5. **Capture ground speed**: the ADSB.lol API provides `gs` (ground speed in knots) which the detector currently ignores. Start storing it on each TrackPoint so it's available for the frontend and for survey type classification.

### Configuration

New config values (environment variables or config file):
- `R2_BUCKET_NAME`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_ENDPOINT` — R2 credentials
- `UPLOAD_LIVE_INTERVAL` — how often to upload current.json (default: 60s)
- `UPLOAD_ARCHIVE_INTERVAL` — how often to upload daily archive (default: 5m)

### What Stays the Same

- ADSB.lol polling logic, track management, and survey detection algorithms are unchanged
- `saved_tracks.json` local persistence continues to work as before
- stdout logging continues for local monitoring

## Stretch Goal: Survey Type Classification

### Approach

A heuristic classifier that uses altitude, speed, track spacing, and pattern geometry to estimate what kind of survey is being conducted.

### Survey Types

| Type | Typical Altitude | Typical Speed | Pattern Characteristics |
|------|-----------------|---------------|------------------------|
| LiDAR Mapping | 1,000-4,000 ft AGL | 80-130 kts | Tight parallel spacing (< 0.5 mi), very consistent altitude |
| Aerial Photography | 3,000-10,000 ft AGL | 100-160 kts | Moderate spacing, consistent altitude, often grid pattern |
| Pipeline/Powerline Inspection | 500-2,000 ft AGL | 60-100 kts | Linear corridor, not parallel grid — follows infrastructure |
| Environmental Survey | 500-5,000 ft AGL | 80-140 kts | Large grid, wider spacing, may vary altitude |

### Output

Each classification includes:
- `survey_type`: string enum (`lidar`, `photography`, `pipeline_inspection`, `environmental`, `unknown`)
- `survey_type_confidence`: `high`, `medium`, `low`
- `survey_type_explanation`: human-readable string explaining why this type was selected, e.g., "Low altitude (2,100 ft) with tight parallel spacing (0.3 mi) is typical of LiDAR mapping surveys"

### Implementation Notes

- This is a stretch goal — the initial release can ship with `survey_type: "unknown"` on all detections
- The heuristic can be refined over time as more survey patterns are observed and manually classified
- Pipeline/powerline inspection may not trigger the current parallel-path detection at all, since it follows a corridor rather than a grid — this may require a separate detection method later

## Region Expansion

The initial deployment covers Minneapolis (250 mi radius). The architecture supports expansion:

- Run additional detector instances covering other regions
- Each instance uploads to the same R2 bucket (or region-specific prefixes)
- The frontend loads all data from the bucket regardless of source region
- `index.json` could be extended to include region metadata

No changes to the frontend are needed for global expansion — it just renders whatever GeoJSON it finds.

## Testing Strategy

- **Go detector**: existing test suite for detection algorithms is unaffected. New tests needed for R2 upload logic (mock the S3 client) and archive accumulation.
- **Frontend**: manual testing against sample JSON files. Create a few representative `current.json` and archive files for development.
- **Integration**: run detector locally, confirm files appear in R2, confirm frontend renders them correctly.
