const SurveyMap = (() => {
  let map = null;
  let mapLoaded = false;
  let onAircraftClick = null;
  let pendingTracks = null;
  let pendingWatchBounds = null;

  function init(containerId, opts = {}) {
    map = new maplibregl.Map({
      container: containerId,
      style: {
        version: 8,
        sources: {
          'osm-tiles': {
            type: 'raster',
            tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
            tileSize: 256,
            attribution: '© OpenStreetMap contributors',
          },
        },
        layers: [{
          id: 'osm-layer',
          type: 'raster',
          source: 'osm-tiles',
          paint: { 'raster-saturation': -0.8, 'raster-brightness-max': 0.4 },
        }],
      },
      center: opts.center || [-93.204, 44.956],
      zoom: opts.zoom || 7,
    });

    map.on('load', () => {
      // Watch area bounding box
      map.addSource('watch-bounds', {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] },
      });

      map.addLayer({
        id: 'watch-bounds-line',
        type: 'line',
        source: 'watch-bounds',
        paint: {
          'line-color': '#ff8800',
          'line-width': 2,
          'line-dasharray': [6, 4],
          'line-opacity': 0.8,
        },
      });

      // Aircraft tracks
      map.addSource('tracks', {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] },
      });

      map.addLayer({
        id: 'track-lines',
        type: 'line',
        source: 'tracks',
        paint: {
          'line-color': ['get', 'color'],
          'line-width': 2,
          'line-opacity': 0.8,
        },
      });

      map.on('click', 'track-lines', (e) => {
        if (e.features.length > 0 && onAircraftClick) {
          onAircraftClick(e.features[0].properties.icao);
        }
      });

      map.on('mouseenter', 'track-lines', () => {
        map.getCanvas().style.cursor = 'pointer';
      });
      map.on('mouseleave', 'track-lines', () => {
        map.getCanvas().style.cursor = '';
      });

      mapLoaded = true;

      // Replay any calls that arrived before map loaded
      if (pendingWatchBounds) {
        setWatchBounds(pendingWatchBounds);
        pendingWatchBounds = null;
      }
      if (pendingTracks) {
        updateTracks(pendingTracks);
        pendingTracks = null;
      }
    });

    return map;
  }

  function updateTracks(featureCollection) {
    if (!mapLoaded) { pendingTracks = featureCollection; return; }

    const features = (featureCollection.features || []).map(f => ({
      ...f,
      properties: {
        ...f.properties,
        color: Colors.forHex(f.properties.icao),
      },
    }));

    map.getSource('tracks').setData({
      type: 'FeatureCollection',
      features,
    });
  }

  function zoomToBounds(bounds) {
    if (!map || !bounds || bounds.length !== 4) return;
    map.fitBounds([[bounds[0], bounds[1]], [bounds[2], bounds[3]]], {
      padding: 50,
      duration: 1000,
    });
  }

  function setClickHandler(handler) {
    onAircraftClick = handler;
  }

  function setWatchBounds(bounds) {
    console.log('setWatchBounds called', { bounds, mapLoaded });
    if (!bounds || bounds.length !== 4) return;
    if (!mapLoaded) { pendingWatchBounds = bounds; return; }
    const [west, south, east, north] = bounds;
    console.log('Drawing watch bounds', { west, south, east, north });
    map.getSource('watch-bounds').setData({
      type: 'FeatureCollection',
      features: [{
        type: 'Feature',
        geometry: {
          type: 'LineString',
          coordinates: [
            [west, south], [east, south], [east, north], [west, north], [west, south],
          ],
        },
        properties: {},
      }],
    });
  }

  function getMap() { return map; }

  return { init, updateTracks, zoomToBounds, setWatchBounds, setClickHandler, getMap };
})();
