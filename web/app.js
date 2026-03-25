(async function() {
  // Use sample data for local dev, real R2 URL for production
  const R2_BASE_URL = window.location.hostname === 'localhost' ? './sample' : 'https://YOUR_R2_PUBLIC_URL';

  // State
  let mode = 'live';
  let currentDate = null;
  let lastData = null;

  // Init data module
  Data.configure(R2_BASE_URL, (data) => {
    lastData = data;
    if (mode === 'live') {
      SurveyMap.updateTracks(data);
      Panel.renderList(data);
      updateStats(data);
    }
  });

  // Init map
  SurveyMap.init('map');
  SurveyMap.setClickHandler((icao) => {
    Panel.showDetail(icao);
    const feature = findFeature(icao);
    if (feature) SurveyMap.zoomToBounds(feature.properties.survey_bounds);
  });

  // Init panel
  Panel.setSelectHandler((icao) => {
    const feature = findFeature(icao);
    if (feature) SurveyMap.zoomToBounds(feature.properties.survey_bounds);
  });

  // Date navigation
  const dateDisplay = document.getElementById('date-display');
  const dateNext = document.getElementById('date-next');
  const datePrev = document.getElementById('date-prev');
  const liveBtn = document.getElementById('live-btn');

  dateDisplay.addEventListener('click', () => switchToLive());
  liveBtn.addEventListener('click', () => switchToLive());
  datePrev.addEventListener('click', () => navigateDate(-1));
  dateNext.addEventListener('click', () => navigateDate(1));

  // Tab switching
  document.getElementById('panel-tabs').addEventListener('click', (e) => {
    if (e.target.classList.contains('panel-tab')) {
      document.querySelectorAll('.panel-tab').forEach(t => t.classList.remove('active'));
      e.target.classList.add('active');
    }
  });

  // URL hash routing
  function parseHash() {
    const hash = window.location.hash.slice(1);
    const params = new URLSearchParams(hash);
    const date = params.get('date');
    const icao = params.get('icao');

    if (date) {
      switchToHistory(date);
      if (icao) setTimeout(() => Panel.showDetail(icao), 500);
    } else {
      switchToLive();
    }
  }

  function updateHash() {
    if (mode === 'live') {
      const icao = Panel.getSelectedIcao();
      window.location.hash = icao ? `icao=${icao}` : '';
    } else {
      const icao = Panel.getSelectedIcao();
      let hash = `date=${currentDate}`;
      if (icao) hash += `&icao=${icao}`;
      window.location.hash = hash;
    }
  }

  async function switchToLive() {
    mode = 'live';
    currentDate = null;
    dateDisplay.textContent = '● Live';
    dateDisplay.style.background = 'var(--accent)';
    liveBtn.style.display = 'none';
    document.getElementById('panel-tabs').style.display = '';
    Panel.hideDetail();
    Data.startPolling(60000);
    updateHash();
  }

  async function switchToHistory(date) {
    mode = 'history';
    currentDate = date;
    Data.stopPolling();
    dateDisplay.textContent = formatDateDisplay(date);
    dateDisplay.style.background = '#333';
    liveBtn.style.display = '';
    document.getElementById('panel-tabs').style.display = 'none';
    Panel.hideDetail();

    try {
      const data = await Data.fetchArchive(date);
      lastData = data;
      SurveyMap.updateTracks(data);
      Panel.renderList(data);
      updateStats(data);
    } catch (e) {
      console.warn('Failed to load archive:', e);
    }
    updateHash();
  }

  function navigateDate(offset) {
    const dates = Data.getAvailableDates();
    if (mode === 'live') {
      if (dates.length > 0) switchToHistory(dates[dates.length - 1]);
      return;
    }
    const idx = dates.indexOf(currentDate);
    const newIdx = idx + offset;
    if (newIdx >= 0 && newIdx < dates.length) {
      switchToHistory(dates[newIdx]);
    } else if (newIdx >= dates.length) {
      switchToLive();
    }
  }

  function updateStats(data) {
    const features = data.features || [];
    const active = features.filter(f => f.properties.active !== false).length;
    document.getElementById('active-count').innerHTML = `<strong>${active}</strong> active`;
    document.getElementById('today-count').innerHTML = `<strong>${features.length}</strong> today`;
  }

  function findFeature(icao) {
    if (!lastData) return null;
    return lastData.features.find(f => f.properties.icao === icao);
  }

  function formatDateDisplay(dateStr) {
    const d = new Date(dateStr + 'T12:00:00');
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
  }

  // Start
  await Data.fetchIndex();
  parseHash();
  window.addEventListener('hashchange', parseHash);
})();
