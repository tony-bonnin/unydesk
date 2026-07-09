(() => {
  const LOOPBACK_BASE_URL = 'http://127.0.0.1:39091';

  function trim(value) {
    return `${value || ''}`.trim();
  }

  function defaultRuntime() {
    return {
      available: false,
      hostname: '',
      version: '',
      server_url: '',
      install_id: '',
      public_id: '',
      access_password: '',
      local_ui_url: '',
      connected: false,
      provisioned: false,
      connection_state: '',
      connection_note: '',
    };
  }

  function normalizeSnapshot(data, fallback = {}) {
    const base = { ...defaultRuntime(), ...fallback };
    const publicID = trim(data.public_id || base.public_id);
    const accessPassword = trim(data.access_password || base.access_password);
    const localUIURL = trim(data.local_ui_url || base.local_ui_url || LOOPBACK_BASE_URL);
    const installID = trim(data.install_id || base.install_id);
    const serverURL = trim(data.server_url || base.server_url);
    return {
      available: true,
      hostname: trim(data.hostname || base.hostname),
      version: trim(data.version || base.version),
      server_url: serverURL,
      install_id: installID,
      public_id: publicID,
      access_password: accessPassword,
      local_ui_url: localUIURL,
      connected: !!data.connected,
      provisioned: !!data.provisioned,
      connection_state: trim(data.connection_state || base.connection_state),
      connection_note: trim(data.connection_note || base.connection_note),
    };
  }

  async function discover(fetchWithTimeout, fallbackRuntime = {}) {
    try {
      const statusResponse = await fetchWithTimeout(`${LOOPBACK_BASE_URL}/api/status`, {
        cache: 'no-store',
        headers: { Accept: 'application/json' },
      }, 350);
      if (statusResponse.ok) {
        return normalizeSnapshot(await statusResponse.json(), fallbackRuntime);
      }
    } catch (_error) {
    }

    const discoveryResponse = await fetchWithTimeout(`${LOOPBACK_BASE_URL}/api/discovery`, {
      cache: 'no-store',
      headers: { Accept: 'application/json' },
    }, 350);
    if (!discoveryResponse.ok) throw new Error('local host discovery failed');
    return normalizeSnapshot(await discoveryResponse.json(), fallbackRuntime);
  }

  async function rotatePassword(fetchWithTimeout, runtime) {
    const baseURL = trim(runtime.local_ui_url || LOOPBACK_BASE_URL).replace(/\/$/, '');
    const response = await fetchWithTimeout(`${baseURL}/api/password`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    }, 2000);
    if (!response.ok) throw new Error('password rotation failed');
    return normalizeSnapshot(await response.json(), runtime);
  }

  async function bootstrap(fetchWithTimeout, runtime, payload) {
    const baseURL = trim(runtime.local_ui_url || LOOPBACK_BASE_URL).replace(/\/$/, '');
    const response = await fetchWithTimeout(`${baseURL}/api/bootstrap`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload || {}),
    }, 2500);
    if (!response.ok) throw new Error('local bootstrap failed');
    return normalizeSnapshot(await response.json(), runtime);
  }

  window.unydeskLocalHostBridge = {
    LOOPBACK_BASE_URL,
    defaultRuntime,
    discover,
    rotatePassword,
    bootstrap,
  };
})();
