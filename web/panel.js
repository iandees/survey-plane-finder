const Panel = (() => {
  let currentData = null;
  let selectedIcao = null;
  let onSelect = null;

  function setSelectHandler(handler) {
    onSelect = handler;
  }

  function renderList(featureCollection) {
    currentData = featureCollection;
    const list = document.getElementById('aircraft-list');
    const features = featureCollection.features || [];

    if (features.length === 0) {
      list.innerHTML = '<div style="padding:1rem;color:#666;font-size:0.8rem;">No survey aircraft detected</div>';
      return;
    }

    const sorted = [...features].sort((a, b) => {
      const aActive = a.properties.active !== false;
      const bActive = b.properties.active !== false;
      if (aActive !== bActive) return bActive - aActive;
      return new Date(b.properties.last_seen) - new Date(a.properties.last_seen);
    });

    list.innerHTML = sorted.map(f => {
      const p = f.properties;
      const isActive = p.active !== false;
      const color = Colors.forHex(p.icao);
      const callsign = p.callsign || p.icao;
      const alt = p.altitude_ft ? `${p.altitude_ft.toLocaleString()}ft` : '';
      const duration = p.duration_min ? formatDuration(p.duration_min) : timeSince(p.first_seen);
      const paths = p.parallel_paths ? `${p.parallel_paths} paths` : '';
      const detail = [alt, duration, paths].filter(Boolean).join(' · ');

      return `
        <div class="aircraft-item ${isActive ? '' : 'completed'}" data-icao="${p.icao}">
          <div class="row">
            <span><span class="dot" style="background:${color}"></span><span class="callsign">${callsign}</span></span>
            <span class="badge ${isActive ? 'badge-active' : 'badge-completed'}">${isActive ? '● ACTIVE' : 'COMPLETED'}</span>
          </div>
          <div class="row" style="margin-top:0.3rem">
            <span class="detail">${detail}</span>
          </div>
        </div>
      `;
    }).join('');

    list.querySelectorAll('.aircraft-item').forEach(el => {
      el.addEventListener('click', () => {
        const icao = el.dataset.icao;
        showDetail(icao);
        if (onSelect) onSelect(icao);
      });
    });
  }

  function showDetail(icao) {
    if (!currentData) return;
    const feature = currentData.features.find(f => f.properties.icao === icao);
    if (!feature) return;

    selectedIcao = icao;
    const p = feature.properties;
    const isActive = p.active !== false;
    const callsign = p.callsign || p.icao;
    const duration = p.duration_min ? formatDuration(p.duration_min) : timeSince(p.first_seen);

    const detailView = document.getElementById('detail-view');
    const aircraftList = document.getElementById('aircraft-list');

    detailView.innerHTML = `
      <div class="detail-header">
        <div>
          <h3>${callsign}</h3>
          <span class="icao">ICAO: ${p.icao.toUpperCase()}</span>
        </div>
        <div style="display:flex;align-items:center;gap:0.5rem">
          <span class="badge ${isActive ? 'badge-active' : 'badge-completed'}">${isActive ? '● ACTIVE' : 'COMPLETED'}</span>
          <button class="close-btn" id="detail-close">×</button>
        </div>
      </div>

      <div class="stat-grid">
        <div class="stat-box"><div class="label">Altitude</div><div class="value">${p.altitude_ft ? p.altitude_ft.toLocaleString() : '—'} <small>ft</small></div></div>
        <div class="stat-box"><div class="label">Speed</div><div class="value">${p.speed_kts ? Math.round(p.speed_kts) : '—'} <small>kts</small></div></div>
        <div class="stat-box"><div class="label">Duration</div><div class="value">${duration}</div></div>
        <div class="stat-box"><div class="label">Parallel Paths</div><div class="value">${p.parallel_paths || '—'}</div></div>
        <div class="stat-box"><div class="label">Track Distance</div><div class="value">${p.track_miles ? p.track_miles : '—'} <small>mi</small></div></div>
        <div class="stat-box"><div class="label">First Seen</div><div class="value" style="font-size:0.75rem">${formatTime(p.first_seen)}</div></div>
      </div>

      <div class="section-label">External Links</div>
      <ul class="link-list">
        ${p.globe_url ? `<li><a href="${p.globe_url}" target="_blank">Track on ADSB.lol <span>→</span></a></li>` : ''}
        ${p.registration_url ? `<li><a href="${p.registration_url}" target="_blank">ADS-B Exchange <span>→</span></a></li>` : ''}
        ${p.flightaware_url ? `<li><a href="${p.flightaware_url}" target="_blank">FlightAware <span>→</span></a></li>` : ''}
      </ul>
    `;

    aircraftList.style.display = 'none';
    detailView.style.display = 'block';

    document.getElementById('detail-close').addEventListener('click', hideDetail);
  }

  function hideDetail() {
    selectedIcao = null;
    document.getElementById('aircraft-list').style.display = 'block';
    document.getElementById('detail-view').style.display = 'none';
  }

  function getSelectedIcao() { return selectedIcao; }

  function formatDuration(minutes) {
    if (minutes < 60) return `${minutes}min`;
    const h = Math.floor(minutes / 60);
    const m = minutes % 60;
    return m > 0 ? `${h}h ${m}m` : `${h}h`;
  }

  function timeSince(isoString) {
    if (!isoString) return '—';
    const mins = Math.round((Date.now() - new Date(isoString).getTime()) / 60000);
    return formatDuration(mins);
  }

  function formatTime(isoString) {
    if (!isoString) return '—';
    return new Date(isoString).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }

  return { setSelectHandler, renderList, showDetail, hideDetail, getSelectedIcao };
})();
