document.addEventListener('alpine:init', () => {
  Alpine.data('unydeskLanding', () => ({
    locale: 'en',
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
    localHostRuntime: {
      ...(window.unydeskLocalHostBridge && window.unydeskLocalHostBridge.defaultRuntime
        ? window.unydeskLocalHostBridge.defaultRuntime()
        : {
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
          }),
    },
    browserLocalIPv4: '',
    standaloneTarget: '',
    standaloneHostPassword: '',
    standaloneStatus: 'Enter a host ID and the password currently shown on the host.',
    standaloneBusy: false,
    hostPasswordBusy: false,
    hostClaimBusy: false,
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
    localHostPollTimer: null,
    localHostProbePromise: null,
    localHostRoutePromise: null,
    localHostLastRouteAt: 0,
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
      icon: 'fa-solid fa-download',
      disabled: false,
    },
    downloads: [
      { title: 'Host for Linux 64-bits', description: 'Static binary for major x86_64 Linux distributions.', href: '/download/host/linux-amd64', archiveHref: '/download/host/linux-amd64.zip', secondary: false, icon: 'fa-brands fa-linux' },
      { title: 'Host for Linux ARM64', description: 'Same host flow for ARM targets and lightweight edge nodes.', href: '/download/host/linux-arm64', archiveHref: '/download/host/linux-arm64.zip', secondary: false, icon: 'fa-brands fa-linux' },
      { title: 'Host for Android', description: 'Android endpoint detected. Native APK is not published yet; use the standalone web client for now.', href: '#android-host', archiveHref: '', secondary: true, disabled: true, cta: 'Coming soon', icon: 'fa-brands fa-android' },
      { title: 'Host for Windows 64-bits', description: 'Standard 64-bit Windows host binary for desktop and server editions.', href: '/download/host/windows-amd64', archiveHref: '/download/host/windows-amd64.zip', secondary: true, icon: 'fa-brands fa-windows' },
      { title: 'Host for Windows ARM64', description: 'Windows on ARM build for newer ARM laptops and tablets.', href: '/download/host/windows-arm64', archiveHref: '/download/host/windows-arm64.zip', secondary: true, icon: 'fa-brands fa-windows' },
      { title: 'Host for macOS Intel', description: 'Darwin build for Intel-based Mac systems.', href: '/download/host/macos-amd64', archiveHref: '/download/host/macos-amd64.zip', secondary: true, icon: 'fa-brands fa-apple' },
      { title: 'Host for macOS Apple Silicon', description: 'Darwin build for Apple Silicon systems.', href: '/download/host/macos-arm64', archiveHref: '/download/host/macos-arm64.zip', secondary: true, icon: 'fa-brands fa-apple' },
    ],

    async boot() {
      this.locale = window.unydeskI18n && window.unydeskI18n.getLocale ? window.unydeskI18n.getLocale() : 'en';
      window.addEventListener('unydesk:localechange', (event) => {
        this.locale = event.detail && event.detail.locale ? event.detail.locale : 'en';
      });
      document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape') this.closeDownloads();
        if (event.key === 'Escape') this.closeLogin();
        if (event.key === 'Escape') this.closeServiceStatus();
      });
      document.addEventListener('visibilitychange', () => {
        if (document.hidden) {
          this.stopLocalHostPolling();
          return;
        }
        this.startLocalHostPolling(0);
      });
      this.resolveDownloadChoice();
      await Promise.allSettled([
        this.loadSession(),
        this.ensureBrowserIdentity(),
        this.loadInfo(),
      ]);
      this.applyPairedDownloads();
      this.detectBrowserLocalIPv4();
      this.resolveDownloadChoice();
      this.startLocalHostPolling(150);
    },

    async fetchWithTimeout(url, options = {}, timeoutMs = 4500) {
      if (!window.AbortController) return fetch(url, options);
      const controller = new AbortController();
      const timeout = window.setTimeout(() => controller.abort(), timeoutMs);
      try {
        return await fetch(url, { ...options, signal: controller.signal });
      } finally {
        window.clearTimeout(timeout);
      }
    },

    browserTokenStorageKey() {
      return 'unydesk.browser.token';
    },

    tr(value) {
      const locale = this.locale;
      void locale;
      if (!window.unydeskI18n || !window.unydeskI18n.t) return value;
      return window.unydeskI18n.t(value, value);
    },

    async loadInfo() {
      try {
        const response = await this.fetchWithTimeout('/api/v1/info');
        this.captureCSRF(response);
        if (!response.ok) throw new Error('info fetch failed');
        this.info = await response.json();
      } catch (error) {
      }
    },

    async loadSession() {
      try {
        const response = await this.fetchWithTimeout('/api/v1/auth/session');
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
        const response = await this.fetchWithTimeout(`/api/v1/browser/identity?token=${encodeURIComponent(token)}`);
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

    async discoverLocalHostRuntime() {
      if (this.localHostProbePromise) return this.localHostProbePromise;
      this.localHostProbePromise = (async () => {
        try {
          const bridge = window.unydeskLocalHostBridge;
          if (!bridge || !bridge.discover) throw new Error('local host bridge unavailable');
          this.localHostRuntime = await bridge.discover(this.fetchWithTimeout.bind(this), this.localHostRuntime);
          if (!`${this.standaloneHostPassword || ''}`.trim() && this.localHostRuntime.access_password) {
            this.standaloneHostPassword = this.localHostRuntime.access_password;
          }
          await this.bootstrapLocalHostRouteIfNeeded();
          return true;
        } catch (_error) {
          const bridge = window.unydeskLocalHostBridge;
          this.localHostRuntime = bridge && bridge.defaultRuntime ? bridge.defaultRuntime() : {
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
          return false;
        }
      })();

      try {
        return await this.localHostProbePromise;
      } finally {
        this.localHostProbePromise = null;
      }
    },

    async bootstrapLocalHostRouteIfNeeded() {
      if (!this.localHostRuntime.available) return false;
      if (`${this.localHostRuntime.server_url || ''}`.trim()) return false;
      if (this.localHostRoutePromise) return this.localHostRoutePromise;
      const now = Date.now();
      if (now - this.localHostLastRouteAt < 15000) return false;
      this.localHostLastRouteAt = now;
      this.localHostRoutePromise = (async () => {
        try {
          const bridge = window.unydeskLocalHostBridge;
          if (!bridge || !bridge.bootstrap) return false;
          const serverRoute = `${this.info.server_url || ''}`.trim() || window.location.origin || '';
          this.localHostRuntime = await bridge.bootstrap(this.fetchWithTimeout.bind(this), this.localHostRuntime, {
            domain: window.location.hostname || '',
            server_url: serverRoute,
            install_id: this.localHostRuntime.install_id,
            public_id: this.localHostRuntime.public_id,
          });
          return true;
        } catch (_error) {
          return false;
        } finally {
          this.localHostRoutePromise = null;
        }
      })();
      return this.localHostRoutePromise;
    },

    startLocalHostPolling(delayMs = 0) {
      if (document.hidden || this.localHostPollTimer) return;
      this.localHostPollTimer = window.setTimeout(async () => {
        this.localHostPollTimer = null;
        const available = await this.discoverLocalHostRuntime();
        this.startLocalHostPolling(available ? 3000 : 15000);
      }, Math.max(0, Number(delayMs) || 0));
    },

    stopLocalHostPolling() {
      if (!this.localHostPollTimer) return;
      window.clearTimeout(this.localHostPollTimer);
      this.localHostPollTimer = null;
    },

    async detectBrowserLocalIPv4() {
      const ip = await this.readBrowserLocalIPv4();
      if (ip) this.browserLocalIPv4 = ip;
    },

    async readBrowserLocalIPv4() {
      const PeerConnection = window.RTCPeerConnection || window.webkitRTCPeerConnection || window.mozRTCPeerConnection;
      if (!PeerConnection) return '';

      const candidates = new Set();
      const addCandidate = (candidate) => {
        const text = `${candidate || ''}`;
        for (const match of text.matchAll(/\b(?:\d{1,3}\.){3}\d{1,3}\b/g)) {
          const ip = match[0];
          if (this.isPrivateIPv4(ip)) candidates.add(ip);
        }
      };

      let pc;
      try {
        pc = new PeerConnection({ iceServers: [] });
        pc.createDataChannel('uny-local-ip');
        pc.addEventListener('icecandidate', (event) => {
          if (event.candidate) addCandidate(event.candidate.candidate);
        });

        const offer = await pc.createOffer();
        await pc.setLocalDescription(offer);
        if (offer && offer.sdp) addCandidate(offer.sdp);

        await new Promise((resolve) => {
          const done = () => resolve();
          pc.addEventListener('icegatheringstatechange', () => {
            if (pc.iceGatheringState === 'complete') done();
          });
          window.setTimeout(done, 1200);
        });

        if (pc.localDescription && pc.localDescription.sdp) addCandidate(pc.localDescription.sdp);
        if (pc.getStats) {
          const stats = await pc.getStats();
          stats.forEach((stat) => {
            if (stat.type !== 'local-candidate') return;
            addCandidate(stat.address || stat.ip || stat.relatedAddress || '');
          });
        }
      } catch (_error) {
      } finally {
        if (pc) pc.close();
      }

      return this.preferredPrivateIPv4([...candidates]);
    },

    preferredPrivateIPv4(ips) {
      return ips
        .map((ip) => ({ ip, score: this.privateIPv4Score(ip) }))
        .filter((entry) => entry.score > 0)
        .sort((a, b) => b.score - a.score || a.ip.localeCompare(b.ip))[0]?.ip || '';
    },

    isPrivateIPv4(ip) {
      return this.privateIPv4Score(ip) > 0;
    },

    privateIPv4Score(ip) {
      const parts = `${ip || ''}`.split('.').map((part) => Number(part));
      if (parts.length !== 4 || parts.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) return 0;
      const [a, b] = parts;
      if (a === 192 && b === 168) return 40;
      if (a === 10) return 30;
      if (a === 172 && b >= 16 && b <= 31) return 20;
      return 0;
    },

    displayedPublicID() {
      return this.localHostRuntime.available && this.localHostRuntime.public_id
        ? this.localHostRuntime.public_id
        : this.browserIdentity.public_id;
    },

    hostPasswordDisplay() {
      if (this.localHostRuntime.available) {
        return this.localHostRuntime.access_password || 'Unavailable';
      }
      return 'Client not installed on this device';
    },

    hostPasswordNote() {
      if (this.localHostRuntime.available && this.localHostRuntime.provisioned) {
        return 'Detected locally from the installed host client.';
      }
      if (this.localHostRuntime.available && this.auth.authenticated) {
        return 'Local host detected. Claim it once to bind this machine to your workspace.';
      }
      if (this.localHostRuntime.available) {
        return 'Sign in to claim this local host and keep it attached to your workspace.';
      }
      return 'The public landing page only shows the host password when the local host client is installed on this device.';
    },

    showLocalHostClaimAction() {
      return this.localHostRuntime.available && !this.localHostRuntime.provisioned;
    },

    localHostClaimCTA() {
      if (this.hostClaimBusy) return 'Linking...';
      if (this.auth.authenticated) return 'Claim local host';
      return 'Sign in to claim';
    },

    async rotateLocalHostPassword() {
      if (!this.localHostRuntime.available || this.hostPasswordBusy) return;
      this.hostPasswordBusy = true;
      try {
        const bridge = window.unydeskLocalHostBridge;
        if (!bridge || !bridge.rotatePassword) throw new Error('local host bridge unavailable');
        this.localHostRuntime = await bridge.rotatePassword(this.fetchWithTimeout.bind(this), this.localHostRuntime);
        this.standaloneHostPassword = this.localHostRuntime.access_password;
        this.standaloneStatus = 'Host password rotated locally.';
      } catch (_error) {
        this.standaloneStatus = 'Unable to rotate the local host password right now.';
      } finally {
        this.hostPasswordBusy = false;
      }
    },

    async claimLocalHost() {
      if (!this.localHostRuntime.available || this.hostClaimBusy) return;
      if (!this.auth.authenticated) {
        this.openLogin();
        return;
      }
      this.hostClaimBusy = true;
      this.standaloneStatus = 'Linking the local host to your workspace...';
      try {
        const csrfReady = await this.ensureCSRFToken();
        if (!csrfReady) {
          this.standaloneStatus = 'Security token unavailable. Refresh the page and try again.';
          return;
        }
        const claimResponse = await fetch('/api/v1/bootstrap/claim', {
          method: 'POST',
          credentials: 'same-origin',
          headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': this.csrfToken,
          },
          body: JSON.stringify({
            install_id: this.localHostRuntime.install_id,
            public_id: this.localHostRuntime.public_id,
            hostname: this.localHostRuntime.hostname,
            version: this.localHostRuntime.version,
          }),
        });
        this.captureCSRF(claimResponse);
        const claimData = await claimResponse.json();
        if (!claimResponse.ok) {
          this.standaloneStatus = claimData.error || 'Unable to claim the local host right now.';
          return;
        }

        const bridge = window.unydeskLocalHostBridge;
        if (!bridge || !bridge.bootstrap) throw new Error('local host bridge unavailable');
        this.localHostRuntime = await bridge.bootstrap(this.fetchWithTimeout.bind(this), this.localHostRuntime, {
          domain: claimData.domain,
          server_url: claimData.server_url,
          install_id: claimData.install_id || this.localHostRuntime.install_id,
          public_id: claimData.public_id || this.localHostRuntime.public_id,
          credential: claimData.credential,
          credential_type: claimData.credential_type,
        });
        this.standaloneStatus = 'Local host linked to your workspace.';
        await this.discoverLocalHostRuntime();
      } catch (_error) {
        this.standaloneStatus = 'Unable to claim the local host right now.';
      } finally {
        this.hostClaimBusy = false;
      }
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
      if (!`${this.standaloneHostPassword || ''}`.trim()) {
        this.standaloneStatus = 'Enter the host password first.';
        return;
      }

      this.standaloneBusy = true;
      this.standaloneStatus = 'Authenticating host access...';
      try {
        const csrfReady = await this.ensureCSRFToken();
        if (!csrfReady) {
          this.standaloneStatus = 'Security token unavailable. Refresh the page and try again.';
          return;
        }
        const response = await fetch('/api/v1/standalone/session', {
          method: 'POST',
          credentials: 'same-origin',
          headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': this.csrfToken,
          },
          body: JSON.stringify({
            target,
            viewer: this.browserIdentity.public_id,
            viewer_label: this.viewerLabel(),
            password: this.standaloneHostPassword,
          }),
        });
        this.captureCSRF(response);
        const data = await response.json();
        if (!response.ok) {
          this.standaloneStatus = data.error || 'Unable to open a standalone session right now.';
          return;
        }

        const sessionID = String(data.id || data.session?.id || '').trim();
        const viewerToken = String(data.viewer_token || '').trim();
        if (!sessionID || !viewerToken) {
          this.standaloneStatus = 'Unable to provision a direct host session right now.';
          return;
        }

        const url = new URL('/connect/', window.location.origin);
        url.searchParams.set('session', sessionID);
        url.hash = new URLSearchParams({ standalone: viewerToken }).toString();

        const opened = window.open(url.toString(), '_blank', 'noopener');
        this.standaloneStatus = opened
          ? `Direct session ${sessionID} opened in a new tab.`
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

    viewerLabel() {
      const localRuntimeHostname = `${this.localHostRuntime && this.localHostRuntime.hostname ? this.localHostRuntime.hostname : ''}`.trim();
      if (localRuntimeHostname) return localRuntimeHostname;
      const browserHostname = `${this.browserIdentity && this.browserIdentity.hostname ? this.browserIdentity.hostname : ''}`.trim();
      if (browserHostname) return browserHostname;
      return '';
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

    directAddressHost() {
      const host = this.directAddress();
      if (!host) return '';
      if (host.startsWith('[')) {
        const end = host.indexOf(']');
        return end > 0 ? host.slice(1, end) : host;
      }
      return host.split(':')[0] || host;
    },

    hostIP() {
      if (this.info.host_ipv4) return this.info.host_ipv4;
      const host = this.directAddressHost();
      if (!host) return '127.0.0.1';
      return host;
    },

    hostIPDisplay(limit = 18) {
      return this.truncateAddress(this.hostIP(), limit);
    },

    clientIP() {
      return this.browserLocalIPv4 || this.info.client_ipv4 || 'Detecting...';
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
      if (!href || String(href).startsWith('#')) return href || '#';
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
      return [...this.hosts].sort((a, b) => this.compareHostsStable(a, b));
    },

    compareHostsStable(a, b) {
      const leftRegistered = this.hostRegisteredAtTime(a);
      const rightRegistered = this.hostRegisteredAtTime(b);
      if (leftRegistered !== rightRegistered) return leftRegistered - rightRegistered;
      return this.hostStableSortKey(a).localeCompare(this.hostStableSortKey(b));
    },

    hostRegisteredAtTime(host) {
      const value = new Date((host && host.registered_at) || '').getTime();
      return Number.isFinite(value) ? value : Number.MAX_SAFE_INTEGER;
    },

    hostStableSortKey(host) {
      return [
        host && host.hostname,
        host && host.name,
        host && host.public_id,
        host && host.install_id,
        host && host.id,
      ].map((part) => String(part || '').trim().toLowerCase()).join('\u0000');
    },

    hostMeta(host) {
      return `${this.hostPlatformLabel(host)} · ${host.version || '?'}`;
    },

    hostPlatformLabel(host) {
      const os = this.hostOSLabel(host && host.os);
      const arch = this.hostArchLabel(host && host.arch);
      if (os === '?' && arch === '?') return '?';
      if (os === '?') return arch;
      if (arch === '?') return os;
      return `${os} ${arch}`;
    },

    hostOSLabel(value) {
      const raw = String(value || '').trim();
      const osName = raw.toLowerCase();
      if (!osName) return '?';
      const distro = this.detectLinuxDistribution(osName);
      if (distro.confident) return distro.name.includes('Linux') ? distro.name : `${distro.name} Linux`;
      if (osName.includes('windows')) return 'Windows';
      if (osName.includes('darwin') || osName.includes('mac') || osName.includes('osx')) return 'macOS';
      if (osName.includes('linux')) return 'Linux';
      return raw;
    },

    hostArchLabel(value) {
      const arch = String(value || '').trim().toLowerCase();
      if (!arch) return '?';
      if (['amd64', 'x86_64', 'x64'].includes(arch)) return '64-bits';
      if (['386', 'i386', 'i686', 'x86'].includes(arch)) return '32-bits';
      if (['arm64', 'aarch64'].includes(arch)) return 'ARM64';
      return value;
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
          const hints = await navigator.userAgentData.getHighEntropyValues(['architecture', 'bitness', 'platform', 'platformVersion']);
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
      const clientSource = `${navigator.userAgent || ''} ${navigator.platform || ''} ${hints.platformVersion || ''} ${arch} ${bitness}`.toLowerCase();

      if (platform.includes('android') || clientSource.includes('android')) {
        return this.androidChoice(this.androidArchLabel(clientSource));
      }
      if (platform.includes('windows')) {
        if (arch.includes('arm')) return this.choice('/download/host/windows-arm64', 'Recommended host: Windows ARM64', 'Download Windows ARM64', 'Detected: Windows on ARM.', 'fa-brands fa-windows');
        return this.choice('/download/host/windows-amd64', 'Recommended host: Windows 64-bits', 'Download Windows 64-bits', 'Detected: Windows 64-bits.', 'fa-brands fa-windows');
      }
      if (platform.includes('mac')) {
        if (arch.includes('arm')) return this.choice('/download/host/macos-arm64', 'Recommended host: macOS Apple Silicon', 'Download macOS Apple Silicon', 'Detected: macOS Apple Silicon.', 'fa-brands fa-apple');
        return this.choice('/download/host/macos-amd64', 'Recommended host: macOS Intel', 'Download macOS Intel', 'Detected: macOS Intel.', 'fa-brands fa-apple');
      }
      if (platform.includes('linux')) {
        const distro = this.detectLinuxDistribution(clientSource);
        if (arch.includes('arm')) return this.linuxChoice('/download/host/linux-arm64', 'ARM64', distro);
        if (arch.includes('x86') || arch.includes('amd') || bitness === '64') return this.linuxChoice('/download/host/linux-amd64', '64-bits', distro);
      }
      return null;
    },

    detectFromUserAgent() {
      const source = `${navigator.userAgent || ''} ${navigator.platform || ''}`.toLowerCase();

      if (source.includes('android')) {
        return this.androidChoice(this.androidArchLabel(source));
      }
      if (source.includes('windows')) {
        if (source.includes('arm') || source.includes('aarch64')) return this.choice('/download/host/windows-arm64', 'Recommended host: Windows ARM64', 'Download Windows ARM64', 'Detected: Windows on ARM.', 'fa-brands fa-windows');
        return this.choice('/download/host/windows-amd64', 'Recommended host: Windows 64-bits', 'Download Windows 64-bits', 'Detected: Windows 64-bits.', 'fa-brands fa-windows');
      }
      if (source.includes('mac') || source.includes('darwin')) {
        if (source.includes('arm') || source.includes('apple') || source.includes('aarch64')) return this.choice('/download/host/macos-arm64', 'Recommended host: macOS Apple Silicon', 'Download macOS Apple Silicon', 'Detected: macOS Apple Silicon.', 'fa-brands fa-apple');
        return this.choice('/download/host/macos-amd64', 'Recommended host: macOS Intel', 'Download macOS Intel', 'Detected: macOS Intel.', 'fa-brands fa-apple');
      }
      if (source.includes('linux') || source.includes('x11')) {
        const distro = this.detectLinuxDistribution(source);
        if (source.includes('arm') || source.includes('aarch64')) return this.linuxChoice('/download/host/linux-arm64', 'ARM64', distro);
        return this.linuxChoice('/download/host/linux-amd64', '64-bits', distro);
      }
      return this.choice('/download/host/linux-amd64', 'Recommended host: Linux 64-bits', 'Download Linux 64-bits', 'Detection was unclear. This is the fallback choice.', 'fa-brands fa-linux');
    },

    linuxChoice(href, archLabel, distro) {
      const distroSuffix = distro.confident ? ` (${distro.name})` : '';
      const meta = distro.confident
        ? `Detected: ${distro.name} Linux ${archLabel}.`
        : `Detected: Linux ${archLabel}. Distribution not exposed by this browser.`;
      return this.choice(href, `Recommended host: Linux ${archLabel}${distroSuffix}`, `Download Linux ${archLabel}`, meta, 'fa-brands fa-linux');
    },

    androidChoice(archLabel = '') {
      const suffix = archLabel ? ` ${archLabel}` : '';
      return this.choice(
        '#android-host',
        `Android detected${suffix}`,
        'Android host coming soon',
        `Detected: Android${suffix}. Native host APK is not published yet; use the standalone web client from this browser for now.`,
        'fa-brands fa-android',
        { disabled: true }
      );
    },

    androidArchLabel(source) {
      const text = String(source || '').toLowerCase();
      if (text.includes('arm64') || text.includes('aarch64')) return 'ARM64';
      if (text.includes('arm')) return 'ARM';
      if (text.includes('x86_64') || text.includes('x64') || text.includes('amd64') || text.includes('64')) return '64-bits';
      return '';
    },

    detectLinuxDistribution(source) {
      const text = `${source || ''}`.toLowerCase();
      const distributions = [
        [/ubuntu/, 'Ubuntu'],
        [/debian/, 'Debian'],
        [/fedora/, 'Fedora'],
        [/\brhel\b/, 'Red Hat Enterprise Linux'],
        [/red\s*hat/, 'Red Hat Enterprise Linux'],
        [/centos/, 'CentOS'],
        [/rocky/, 'Rocky Linux'],
        [/almalinux/, 'AlmaLinux'],
        [/opensuse/, 'openSUSE'],
        [/\bsuse\b/, 'SUSE Linux'],
        [/\barch\b/, 'Arch Linux'],
        [/manjaro/, 'Manjaro'],
        [/alpine/, 'Alpine Linux'],
        [/gentoo/, 'Gentoo'],
        [/linux\s*mint/, 'Linux Mint'],
        [/\bmint\b/, 'Linux Mint'],
        [/pop[!_\s-]*os/, 'Pop!_OS'],
      ];
      const match = distributions.find(([pattern]) => pattern.test(text));
      return match ? { name: match[1], confident: true } : { name: 'Linux', confident: false };
    },

    choice(href, label, cta, meta, icon = 'fa-solid fa-download', options = {}) {
      return { href: this.pairedDownloadHref(href), label, cta, meta, icon, disabled: !!options.disabled };
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

    async ensureCSRFToken() {
      if (`${this.csrfToken || ''}`.trim()) return true;
      try {
        const response = await this.fetchWithTimeout('/api/v1/auth/session');
        this.captureCSRF(response);
      } catch (_error) {
      }
      return !!`${this.csrfToken || ''}`.trim();
    },

    async submitAuth() {
      this.authError = '';
      try {
        const csrfReady = await this.ensureCSRFToken();
        if (!csrfReady) {
          this.authError = 'Security token unavailable. Refresh the page and try again.';
          return;
        }
        const response = await fetch('/api/v1/auth/login', {
          method: 'POST',
          credentials: 'same-origin',
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
