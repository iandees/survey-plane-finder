const SurveyMap = (() => {
  let map = null;
  let onAircraftClick = null;

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
    });

    return map;
  }

  function updateTracks(featureCollection) {
    if (!map || !map.getSource('tracks')) return;

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

  function getMap() { return map; }

  return { init, updateTracks, zoomToBounds, setClickHandler, getMap };
})();
