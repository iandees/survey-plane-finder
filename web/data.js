const Data = (() => {
  let baseUrl = '';
  let archiveCache = {};
  let indexDates = [];
  let pollTimer = null;
  let onUpdate = null;

  function configure(r2BaseUrl, updateCallback) {
    baseUrl = r2BaseUrl.replace(/\/$/, '');
    onUpdate = updateCallback;
  }

  async function fetchJSON(path) {
    const resp = await fetch(`${baseUrl}/${path}`);
    if (!resp.ok) throw new Error(`Fetch failed: ${resp.status} ${path}`);
    return resp.json();
  }

  async function fetchIndex() {
    try {
      const data = await fetchJSON('index.json');
      indexDates = data.dates || [];
    } catch (e) {
      console.warn('Could not load index.json:', e);
      indexDates = [];
    }
    return indexDates;
  }

  async function fetchLive() {
    const data = await fetchJSON('live/current.json');
    if (onUpdate) onUpdate(data);
    return data;
  }

  async function fetchArchive(date) {
    if (archiveCache[date]) {
      return archiveCache[date];
    }
    const data = await fetchJSON(`archive/${date}.json`);
    archiveCache[date] = data;
    return data;
  }

  function startPolling(intervalMs = 60000) {
    stopPolling();
    fetchLive().catch(e => console.warn('Live fetch error:', e));
    pollTimer = setInterval(() => {
      fetchLive().catch(e => console.warn('Live fetch error:', e));
    }, intervalMs);
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  function getAvailableDates() {
    return indexDates;
  }

  return { configure, fetchIndex, fetchLive, fetchArchive, startPolling, stopPolling, getAvailableDates };
})();
