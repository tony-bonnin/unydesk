document.addEventListener('alpine:init', () => {
  Alpine.data('unydeskLanding', () => ({
    csrfToken: '',
    info: {
      name: 'UnyDesk',
      version: 'Loading…',
      public_id: 'Loading…',
      host_ipv4: '',
      client_ipv4: '',
      host_heartbeat_seconds: 35,
    },
    browserIdentity: {
      id: '',
      public_id: 'Loading…',
      token: '',
    },
    standaloneTarget: '',
    standalonePassword: '',
    standaloneStatus: 'No account required. Open a standalone client in a new tab.',
    standaloneBusy: false,
    standaloneRegenActive: false,
    auth: {
      authenticated: false,
      user: null,
    },
    loginOpen: false,
    authError: '',
    authForm: {
      email: '',
      password: '',
    },
    hosts: [],
    downloadsOpen: false,
    serviceStatusOpen: false,
    serviceStatus: {
      title: 'Service Status',
      subtitle: 'Inspect API and health endpoint responses without leaving the page.',
      meta: 'Ready.',
      body: 'Choose an endpoint to inspect.',
      ok: true,
    },
    copiedField: '',
    copyTimer: null,
    checksumsText: 'Checksums will appear here on demand.',
    downloadChoice: {
      href: '#',
      cta: 'Preparing suggestion…',
      label: 'Detecting system',
      meta: 'Waiting for browser detection.',
    },
    downloads: [
      { title: 'Host for Linux amd64', description: 'Stripped static binary for Alpine-friendly deployments.', href: '/download/host/linux-amd64', archiveHref: '/download/host/linux-amd64.zip', secondary: false },
      { title: 'Host for Linux arm64', description: 'Same host flow for ARM targets and lightweight edge nodes.', href: '/download/host/linux-arm64', archiveHref: '/download/host/linux-arm64.zip', secondary: false },
      { title: 'Host for Windows amd64', description: 'Standard 64-bit Windows host binary for desktop and server editions.', href: '/download/host/windows-amd64', archiveHref: '/download/host/windows-amd64.zip', secondary: true },
      { title: 'Host for Windows arm64', description: 'Windows on ARM build for newer ARM laptops and tablets.', href: '/download/host/windows-arm64', archiveHref: '/download/host/windows-arm64.zip', secondary: true },
      { title: 'Host for macOS Intel', description: 'Darwin build for Intel-based Mac systems.', href: '/download/host/macos-amd64', archiveHref: '/download/host/macos-amd64.zip', secondary: true },
      { title: 'Host for macOS Apple Silicon', description: 'Darwin build for Apple Silicon systems.', href: '/download/host/macos-arm64', archiveHref: '/download/host/macos-arm64.zip', secondary: true },
    ],

    async boot() {
      document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape') this.closeDownloads();
        if (event.key === 'Escape') this.closeLogin();
        if (event.key === 'Escape') this.closeServiceStatus();
      });
      await this.loadSession();
      await this.ensureBrowserIdentity();
      this.ensureStandalonePassword();
      this.applyPairedDownloads();
      await this.loadInfo();
      this.resolveDownloadChoice();
    },

    browserTokenStorageKey() {
      return 'unydesk.browser.token';
    },

    standalonePasswordStorageKey() {
      return 'unydesk.standalone.password';
    },

    async loadInfo() {
      try {
        const response = await fetch('/api/v1/info');
        this.captureCSRF(response);
        if (!response.ok) throw new Error('info fetch failed');
        this.info = await response.json();
      } catch (error) {
      }
    },

    async loadSession() {
      try {
        const response = await fetch('/api/v1/auth/session');
        this.captureCSRF(response);
        if (!response.ok) throw new Error('session fetch failed');
        const data = await response.json();
        this.auth = {
          authenticated: !!data.authenticated,
          user: data.user || null,
        };
      } catch (error) {
        this.auth = { authenticated: false, user: null };
      }
    },

    async ensureBrowserIdentity() {
      let token = window.localStorage.getItem(this.browserTokenStorageKey()) || '';
      if (!token && window.crypto && window.crypto.getRandomValues) {
        const raw = new Uint8Array(24);
        window.crypto.getRandomValues(raw);
        token = Array.from(raw, (value) => value.toString(16).padStart(2, '0')).join('');
      }
      try {
        const response = await fetch(`/api/v1/browser/identity?token=${encodeURIComponent(token)}`);
        this.captureCSRF(response);
        if (!response.ok) throw new Error('browser identity fetch failed');
        const data = await response.json();
        this.browserIdentity = data;
        if (data.token) {
          window.localStorage.setItem(this.browserTokenStorageKey(), data.token);
        }
      } catch (error) {
      }
    },

    generateStandalonePassword() {
      const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
      const size = 18;
      if (window.crypto && window.crypto.getRandomValues) {
        const raw = new Uint8Array(size);
        window.crypto.getRandomValues(raw);
        return Array.from(raw, (value) => alphabet[value % alphabet.length]).join('');
      }
      return Math.random().toString(36).slice(2, 20).toUpperCase();
    },

    ensureStandalonePassword() {
      const current = window.sessionStorage.getItem(this.standalonePasswordStorageKey()) || '';
      if (current) {
        this.standalonePassword = current;
        return;
      }
      this.standalonePassword = this.generateStandalonePassword();
      window.sessionStorage.setItem(this.standalonePasswordStorageKey(), this.standalonePassword);
    },

    regenerateStandalonePassword() {
      this.standaloneRegenActive = true;
      this.standalonePassword = this.generateStandalonePassword();
      window.sessionStorage.setItem(this.standalonePasswordStorageKey(), this.standalonePassword);
      this.standaloneStatus = 'Ephemeral client password regenerated.';
      window.setTimeout(() => {
        this.standaloneRegenActive = false;
      }, 650);
    },

    async openStandaloneClient() {
      const target = `${this.standaloneTarget || ''}`.trim();
      if (!target) {
        this.standaloneStatus = 'Enter a target host ID or hostname first.';
        return;
      }
      if (!this.browserIdentity.public_id || this.browserIdentity.public_id === 'Loading…') {
        this.standaloneStatus = 'Client identity is still loading.';
        return;
      }
      if (!this.standalonePassword) {
        this.ensureStandalonePassword();
      }

      this.standaloneBusy = true;
      this.standaloneStatus = 'Preparing standalone session...';
      try {
        const response = await fetch('/api/v1/standalone/session', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': this.csrfToken,
          },
          body: JSON.stringify({
            target,
            viewer: this.browserIdentity.public_id,
            password: this.standalonePassword,
          }),
        });
        this.captureCSRF(response);
        const data = await response.json();
        if (!response.ok) {
          this.standaloneStatus = data.error || 'Unable to open a standalone session right now.';
          return;
        }

        const url = new URL('/connect/', window.location.origin);
        url.searchParams.set('session', data.id);
        url.hash = new URLSearchParams({ standalone: this.standalonePassword }).toString();

        const opened = window.open(url.toString(), '_blank', 'noopener');
        this.standaloneStatus = opened
          ? `Standalone session ${data.id} opened in a new tab.`
          : 'Popup blocked. Opening standalone client in this tab instead.';
        if (!opened) {
          window.location.assign(url.toString());
        }
      } catch (error) {
        this.standaloneStatus = 'Unable to open a standalone session right now.';
      } finally {
        this.standaloneBusy = false;
      }
    },

    async loadHosts() {
      try {
        const response = await fetch('/api/v1/hosts');
        this.captureCSRF(response);
        if (!response.ok) throw new Error('host fetch failed');
        const data = await response.json();
        this.hosts = data.hosts || [];
      } catch (error) {
        this.hosts = [];
      }
    },

    directAddress() {
      return window.location.host || '127.0.0.1:8890';
    },

    hostIP() {
      if (this.info.host_ipv4) return this.info.host_ipv4;
      const host = this.directAddress();
      if (!host) return '127.0.0.1';
      if (host.startsWith('[')) {
        const end = host.indexOf(']');
        return end > 0 ? host.slice(1, end) : host;
      }
      return host.split(':')[0] || host;
    },

    hostIPDisplay(limit = 18) {
      return this.truncateAddress(this.hostIP(), limit);
    },

    clientIP() {
      return this.info.client_ipv4 || this.hostIP();
    },

    clientIPDisplay(limit = 18) {
      return this.truncateAddress(this.clientIP(), limit);
    },

    directAddressDisplay(limit = 28) {
      return this.truncateAddress(this.directAddress(), limit);
    },

    directServerURL() {
      return window.location.origin || 'http://127.0.0.1:8890';
    },

    truncateMiddle(value, limit = 28) {
      const text = `${value || ''}`;
      const separator = '...';
      if (!text || text.length <= limit) return text;

      const tail = Math.min(10, Math.max(6, Math.floor(limit / 3)));
      const head = Math.max(8, limit - tail - separator.length);
      if (head + tail + separator.length >= text.length) return text;

      return `${text.slice(0, head)}${separator}${text.slice(-tail)}`;
    },

    truncateAddress(value, limit = 22) {
      const text = `${value || ''}`.trim();
      if (!text || text.length <= limit) return text;
      if (limit <= 3) return text.slice(0, limit);
      return `${text.slice(0, limit - 3)}...`;
    },

    exampleCommand() {
      const installID = this.browserIdentity.install_id || '';
      if (!installID) return `unydesk-host --server ${this.directServerURL()} --no-pause`;
      return `unydesk-host --server ${this.directServerURL()} --install-id ${installID} --no-pause`;
    },

    pairedDownloadHref(href) {
      try {
        const url = new URL(href, window.location.origin);
        if (this.browserIdentity.install_id) {
          url.searchParams.set('install_id', this.browserIdentity.install_id);
        }
        return `${url.pathname}${url.search}`;
      } catch (error) {
        return href;
      }
    },

    applyPairedDownloads() {
      this.downloads = this.downloads.map((item) => ({
        ...item,
        href: this.pairedDownloadHref(item.href),
      }));
    },

    downloadTitle() {
      return this.downloadChoice.label;
    },

    hostSummary() {
      return `${this.onlineCount()} online / ${this.hosts.length} total`;
    },

    onlineCount() {
      return this.hosts.filter((host) => host.status === 'online').length;
    },

    sortedHosts() {
      return [...this.hosts].sort((a, b) => new Date(b.last_seen_at).getTime() - new Date(a.last_seen_at).getTime());
    },

    hostMeta(host) {
      return `${host.os || '?'}\/${host.arch || '?'} · ${host.version || '?'}`;
    },

    hostStatus(host) {
      return `${host.status === 'online' ? 'Online' : 'Offline'} · ${this.relativeLastSeen(host.last_seen_at)}`;
    },

    relativeLastSeen(value) {
      if (!value) return 'never';
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) return 'unknown';
      const diff = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
      if (diff < 2) return 'just now';
      if (diff < 60) return `${diff}s ago`;
      if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
      return `${Math.floor(diff / 3600)}h ago`;
    },

    async loadChecksums() {
      this.checksumsText = 'Loading checksums...';
      try {
        const response = await fetch('/download/host/checksums');
        this.captureCSRF(response);
        if (!response.ok) throw new Error('checksum fetch failed');
        const text = await response.text();
        this.checksumsText = text.trim() || 'No checksums available.';
      } catch (error) {
        this.checksumsText = 'Unable to load checksums right now.';
      }
    },

    async resolveDownloadChoice() {
      let choice = null;
      try {
        if (navigator.userAgentData && navigator.userAgentData.getHighEntropyValues) {
          const hints = await navigator.userAgentData.getHighEntropyValues(['architecture', 'bitness', 'platform']);
          choice = this.detectFromClientHints(hints);
        }
      } catch (error) {
      }
      if (!choice) {
        choice = this.detectFromUserAgent();
      }
      this.downloadChoice = choice;
    },

    detectFromClientHints(hints) {
      const platform = (hints.platform || '').toLowerCase();
      const arch = (hints.architecture || '').toLowerCase();
      const bitness = (hints.bitness || '').toLowerCase();

      if (platform.includes('windows')) {
        if (arch.includes('arm')) return this.choice('/download/host/windows-arm64', 'Recommended host: Windows arm64', 'Download Windows arm64', 'Detected: Windows on ARM.');
        return this.choice('/download/host/windows-amd64', 'Recommended host: Windows amd64', 'Download Windows amd64', 'Detected: Windows desktop.');
      }
      if (platform.includes('mac')) {
        if (arch.includes('arm')) return this.choice('/download/host/macos-arm64', 'Recommended host: macOS Apple Silicon', 'Download macOS Apple Silicon', 'Detected: macOS Apple Silicon.');
        return this.choice('/download/host/macos-amd64', 'Recommended host: macOS Intel', 'Download macOS Intel', 'Detected: macOS Intel.');
      }
      if (platform.includes('linux')) {
        if (arch.includes('arm')) return this.choice('/download/host/linux-arm64', 'Recommended host: Linux arm64', 'Download Linux arm64', 'Detected: Linux on ARM.');
        if (arch.includes('x86') || arch.includes('amd') || bitness === '64') return this.choice('/download/host/linux-amd64', 'Recommended host: Linux amd64', 'Download Linux amd64', 'Detected: Linux x86_64.');
      }
      return null;
    },

    detectFromUserAgent() {
      const source = `${navigator.userAgent || ''} ${navigator.platform || ''}`.toLowerCase();

      if (source.includes('windows')) {
        if (source.includes('arm') || source.includes('aarch64')) return this.choice('/download/host/windows-arm64', 'Recommended host: Windows arm64', 'Download Windows arm64', 'Detected: Windows on ARM.');
        return this.choice('/download/host/windows-amd64', 'Recommended host: Windows amd64', 'Download Windows amd64', 'Detected: Windows desktop.');
      }
      if (source.includes('mac') || source.includes('darwin')) {
        if (source.includes('arm') || source.includes('apple') || source.includes('aarch64')) return this.choice('/download/host/macos-arm64', 'Recommended host: macOS Apple Silicon', 'Download macOS Apple Silicon', 'Detected: macOS Apple Silicon.');
        return this.choice('/download/host/macos-amd64', 'Recommended host: macOS Intel', 'Download macOS Intel', 'Detected: macOS Intel.');
      }
      if (source.includes('linux') || source.includes('x11')) {
        if (source.includes('arm') || source.includes('aarch64')) return this.choice('/download/host/linux-arm64', 'Recommended host: Linux arm64', 'Download Linux arm64', 'Detected: Linux on ARM.');
        return this.choice('/download/host/linux-amd64', 'Recommended host: Linux amd64', 'Download Linux amd64', 'Detected: Linux desktop.');
      }
      return this.choice('/download/host/linux-amd64', 'Recommended host: Linux amd64', 'Download Linux amd64', 'Detection was unclear. This is the fallback choice.');
    },

    choice(href, label, cta, meta) {
      return { href: this.pairedDownloadHref(href), label, cta, meta };
    },

    openLogin() {
      this.authError = '';
      this.loginOpen = true;
    },

    closeLogin() {
      this.loginOpen = false;
    },

    openDownloads() {
      this.downloadsOpen = true;
    },

    closeDownloads() {
      this.downloadsOpen = false;
    },

    closeServiceStatus() {
      this.serviceStatusOpen = false;
    },

    async openServiceStatus(kind) {
      const endpoint = kind === 'health' ? '/healthz' : '/api/v1/info';
      this.serviceStatusOpen = true;
      this.serviceStatus = {
        title: kind === 'health' ? 'Health Check' : 'API Info',
        subtitle: kind === 'health'
          ? 'Live response from the service health endpoint.'
          : 'Live response from the API information endpoint.',
        meta: `Loading ${endpoint}...`,
        body: 'Loading...',
        ok: true,
      };

      try {
        const response = await fetch(endpoint);
        this.captureCSRF(response);
        const text = await response.text();
        let parsed = null;
        try {
          parsed = JSON.parse(text);
        } catch (_error) {}

        this.serviceStatus = {
          title: kind === 'health' ? 'Health Check' : 'API Info',
          subtitle: kind === 'health'
            ? 'Live response from the service health endpoint.'
            : 'Live response from the API information endpoint.',
          meta: `HTTP ${response.status} ${response.statusText || ''}`.trim(),
          body: parsed ? JSON.stringify(parsed, null, 2) : (text.trim() || 'No response body.'),
          ok: response.ok,
        };
      } catch (_error) {
        this.serviceStatus = {
          title: kind === 'health' ? 'Health Check' : 'API Info',
          subtitle: kind === 'health'
            ? 'Live response from the service health endpoint.'
            : 'Live response from the API information endpoint.',
          meta: `Unable to reach ${endpoint}`,
          body: 'The request failed before a response was received.',
          ok: false,
        };
      }
    },

    captureCSRF(response) {
      const token = response.headers.get('X-CSRF-Token');
      if (token) this.csrfToken = token;
    },

    async submitAuth() {
      this.authError = '';
      try {
        const response = await fetch('/api/v1/auth/login', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': this.csrfToken,
          },
          body: JSON.stringify({
            email: this.authForm.email,
            password: this.authForm.password,
          }),
        });
        this.captureCSRF(response);
        const data = await response.json();
        if (!response.ok) {
          this.authError = data.error || 'Authentication failed.';
          return;
        }
        this.auth = { authenticated: true, user: data.user || null };
        this.authForm.password = '';
        this.closeLogin();
        window.location.assign('/account');
      } catch (error) {
        this.authError = 'Authentication failed.';
      }
    },

    fallbackCopy(value) {
      const input = document.createElement('textarea');
      input.value = value || '';
      input.setAttribute('readonly', 'readonly');
      input.style.position = 'absolute';
      input.style.left = '-9999px';
      document.body.appendChild(input);
      input.select();
      const copied = document.execCommand('copy');
      document.body.removeChild(input);
      if (!copied) throw new Error('copy failed');
    },

    async copy(value, field) {
      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(value || '');
        } else {
          this.fallbackCopy(value);
        }
        this.copiedField = field || '';
        if (this.copyTimer) window.clearTimeout(this.copyTimer);
        this.copyTimer = window.setTimeout(() => {
          this.copiedField = '';
          this.copyTimer = null;
        }, 1800);
      } catch (error) {
      }
    },
  }));
});
