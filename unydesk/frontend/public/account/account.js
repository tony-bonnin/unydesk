(async () => {
  const state = {
    csrfToken: '',
    browserIdentity: null,
    user: null,
    hosts: [],
    sessions: [],
    currentSection: 'overview',
    mobileMenuOpen: false,
    settingsModalOpen: false,
    sessionModalOpen: false,
    currentSessionID: '',
    sessionPollTimer: null,
    viewerPeerConnection: null,
    viewerDataChannel: null,
    viewerTransportState: 'Idle',
    viewerTransportLog: [],
    remoteCandidatesSeen: [],
    creatingSession: false,
    settingsTab: 'profile',
    preferences: {
      theme: 'system',
      defaultSection: 'overview',
    },
  };

  const sectionTitles = {
    overview: 'Overview',
    connections: 'New Connection',
    hosts: 'Hosts',
    sessions: 'Sessions',
  };

  const preferenceStorageKey = 'unydesk.account.preferences';

  const statusEl = document.getElementById('account-status');
  const nameEl = document.getElementById('account-name');
  const emailEl = document.getElementById('account-email');
  const browserIDEl = document.getElementById('account-browser-id');
  const hostsOnlineEl = document.getElementById('hosts-online-count');
  const hostsTotalEl = document.getElementById('hosts-total-count');
  const hostsOfflineEl = document.getElementById('hosts-offline-count');
  const sessionsActiveEl = document.getElementById('sessions-active-count');
  const sessionsTotalEl = document.getElementById('sessions-total-count');
  const sidebarUserNameEl = document.getElementById('sidebar-user-name');
  const sidebarUserEmailEl = document.getElementById('sidebar-user-email');
  const sidebarAvatarEl = document.getElementById('sidebar-avatar');
  const mobileUserNameEl = document.getElementById('mobile-user-name');
  const mobileUserEmailEl = document.getElementById('mobile-user-email');
  const mobileAvatarEl = document.getElementById('mobile-avatar');
  const workspaceTitleEl = document.getElementById('workspace-title');
  const globalFeedbackWrapEl = document.getElementById('global-feedback-wrap');
  const globalFeedbackEl = document.getElementById('global-feedback');
  const sessionFeedbackWrapEl = document.getElementById('session-feedback-wrap');
  const sessionFeedbackEl = document.getElementById('session-feedback');
  const settingsFeedbackWrapEl = document.getElementById('settings-feedback-wrap');
  const settingsFeedbackEl = document.getElementById('settings-feedback');
  const settingsModalBackdropEl = document.getElementById('settings-modal-backdrop');
  const settingsModalCloseEl = document.getElementById('settings-modal-close');
  const settingsModalNameEl = document.getElementById('settings-modal-name');
  const settingsModalEmailEl = document.getElementById('settings-modal-email');
  const settingsModalAvatarEl = document.getElementById('settings-modal-avatar');
  const sessionModalBackdropEl = document.getElementById('session-modal-backdrop');
  const sessionModalCloseEl = document.getElementById('session-modal-close');
  const sessionModalTitleEl = document.getElementById('session-modal-title');
  const sessionModalSubtitleEl = document.getElementById('session-modal-subtitle');
  const sessionRefreshBtn = document.getElementById('session-refresh-btn');
  const sessionCloseBtn = document.getElementById('session-close-btn');
  const sessionStartSignalingBtn = document.getElementById('session-start-signaling-btn');
  const sessionSendPingBtn = document.getElementById('session-send-ping-btn');
  const sessionRemotePadEl = document.getElementById('session-remote-pad');
  const sessionDetailIDEl = document.getElementById('session-detail-id');
  const sessionDetailStatusEl = document.getElementById('session-detail-status');
  const sessionDetailTargetEl = document.getElementById('session-detail-target');
  const sessionDetailViewerEl = document.getElementById('session-detail-viewer');
  const sessionDetailCreatedEl = document.getElementById('session-detail-created');
  const sessionDetailUpdatedEl = document.getElementById('session-detail-updated');
  const sessionDetailRouteEl = document.getElementById('session-detail-route');
  const sessionDetailDispatchEl = document.getElementById('session-detail-dispatch');
  const sessionDetailOfferEl = document.getElementById('session-detail-offer');
  const sessionDetailAnswerEl = document.getElementById('session-detail-answer');
  const sessionDetailViewerTransportEl = document.getElementById('session-detail-viewer-transport');
  const sessionDetailCandidatesEl = document.getElementById('session-detail-candidates');
  const sessionDetailDeliveredAtEl = document.getElementById('session-detail-delivered-at');
  const sessionDetailDeliveriesEl = document.getElementById('session-detail-deliveries');
  const sessionDetailHostAckEl = document.getElementById('session-detail-host-ack');
  const sessionOfferPreviewEl = document.getElementById('session-offer-preview');
  const sessionAnswerPreviewEl = document.getElementById('session-answer-preview');
  const sessionTransportLogEl = document.getElementById('session-transport-log');
  const settingsTabButtons = Array.from(document.querySelectorAll('.settings-modal-tab'));
  const settingsPanels = Array.from(document.querySelectorAll('.settings-tab-panel'));
  const overviewHostsGridEl = document.getElementById('overview-hosts-grid');
  const hostsTableBody = document.getElementById('hosts-table-body');
  const sessionsTableBody = document.getElementById('sessions-table-body');
  const sessionTargetEl = document.getElementById('session-target');
  const sessionViewerEl = document.getElementById('session-viewer');
  const sessionCreateBtn = document.getElementById('session-create-btn');
  const profileAvatarEl = document.getElementById('profile-avatar');
  const profileDisplayNameEl = document.getElementById('profile-display-name');
  const profileEmailEl = document.getElementById('profile-email');
  const profileSaveBtn = document.getElementById('profile-save-btn');
  const preferencesThemeEl = document.getElementById('pref-theme');
  const preferencesDefaultSectionEl = document.getElementById('pref-default-section');
  const preferencesSaveBtn = document.getElementById('preferences-save-btn');
  const passwordCurrentEl = document.getElementById('password-current');
  const passwordNewEl = document.getElementById('password-new');
  const passwordChangeBtn = document.getElementById('password-change-btn');
  const logoutBtn = document.getElementById('logout-btn');
  const mobileLogoutBtn = document.getElementById('mobile-logout-btn');
  const mobileOpenBtn = document.getElementById('mobile-open');
  const mobileCloseBtn = document.getElementById('mobile-close');
  const mobileOverlayEl = document.getElementById('mobile-overlay');
  const mobileMenuEl = document.getElementById('mobile-menu');
  const sidebarUserTriggerEl = document.getElementById('sidebar-user-trigger');
  const mobileUserTriggerEl = document.getElementById('mobile-user-trigger');
  const sidebarSettingsTriggerEl = document.getElementById('sidebar-settings-trigger');
  const mobileSettingsTriggerEl = document.getElementById('mobile-settings-trigger');
  const navItems = Array.from(document.querySelectorAll('.account-nav-item[data-section]'));
  const panelItems = Array.from(document.querySelectorAll('.account-panel[data-panel]'));

  function sessionControlURL(sessionID) {
    return `/account/control/?session=${encodeURIComponent(sessionID)}`;
  }

  function openSessionTab(sessionID) {
    if (!sessionID) return;
    const url = sessionControlURL(sessionID);
    const opened = window.open(url, '_blank', 'noopener');
    if (!opened) {
      window.location.assign(url);
    }
  }

  function captureCSRF(response) {
    const token = response.headers.get('X-CSRF-Token');
    if (token) state.csrfToken = token;
  }

  function browserTokenStorageKey() {
    return 'unydesk.browser.token';
  }

  function browserIdentityRequestURL() {
    let token = window.localStorage.getItem(browserTokenStorageKey()) || '';
    if (!token && window.crypto && window.crypto.getRandomValues) {
      const raw = new Uint8Array(24);
      window.crypto.getRandomValues(raw);
      token = Array.from(raw, (value) => value.toString(16).padStart(2, '0')).join('');
    }

    const params = new URLSearchParams();
    params.set('token', token);

    return `/api/v1/browser/identity?${params.toString()}`;
  }

  function setFeedback(target, kind, message) {
    if (!target && globalFeedbackEl && globalFeedbackWrapEl) {
      globalFeedbackEl.textContent = message;
      globalFeedbackWrapEl.classList.remove('hidden');
      globalFeedbackEl.classList.remove('is-success', 'is-error');
      globalFeedbackEl.classList.add(kind === 'success' ? 'is-success' : 'is-error');
      globalFeedbackWrapEl.scrollIntoView({ behavior: 'smooth', block: 'start' });
      return;
    }
    target.textContent = message;
    target.classList.remove('hidden', 'is-success', 'is-error');
    target.classList.add(kind === 'success' ? 'is-success' : 'is-error');
  }

  function clearFeedback(target) {
    if (!target && globalFeedbackEl && globalFeedbackWrapEl) {
      globalFeedbackEl.textContent = '';
      globalFeedbackEl.classList.remove('is-success', 'is-error');
      globalFeedbackWrapEl.classList.add('hidden');
      return;
    }
    target.textContent = '';
    target.classList.add('hidden');
    target.classList.remove('is-success', 'is-error');
  }

  function setSettingsFeedback(kind, message) {
    setFeedback(settingsFeedbackEl, kind, message);
    if (settingsFeedbackWrapEl) settingsFeedbackWrapEl.classList.remove('hidden');
    if (settingsFeedbackWrapEl) settingsFeedbackWrapEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  function clearSettingsFeedback() {
    clearFeedback(settingsFeedbackEl);
    if (settingsFeedbackWrapEl) settingsFeedbackWrapEl.classList.add('hidden');
  }

  function setSessionFeedback(kind, message) {
    setFeedback(sessionFeedbackEl, kind, message);
    if (sessionFeedbackWrapEl) sessionFeedbackWrapEl.classList.remove('hidden');
  }

  function clearSessionFeedback() {
    clearFeedback(sessionFeedbackEl);
    if (sessionFeedbackWrapEl) sessionFeedbackWrapEl.classList.add('hidden');
  }

  function formatDateTime(value) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '—';
    return date.toLocaleString();
  }

  function relativeTime(value) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '—';
    const diff = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
    if (diff < 60) return `${diff}s ago`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return `${Math.floor(diff / 86400)}d ago`;
  }

  function escapeHTML(value) {
    return String(value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function hostOSIcon(host) {
    const osName = String(host && host.os ? host.os : '').toLowerCase();
    if (osName.includes('windows')) return 'fa-brands fa-windows';
    if (osName.includes('darwin') || osName.includes('mac') || osName.includes('osx')) return 'fa-brands fa-apple';
    if (osName.includes('linux') || osName.includes('alpine')) return 'fa-brands fa-linux';
    return 'fa-solid fa-desktop';
  }

  function hostRoleLabel(host) {
    return String(host && host.role ? host.role : 'host').trim().toLowerCase() === 'client' ? 'Client' : 'Host';
  }

  function hostStatusClass(host) {
    const status = String(host && host.status ? host.status : '').trim().toLowerCase();
    if (status === 'online') return 'is-online';
    if (status === 'paused') return 'is-paused';
    return 'is-offline';
  }

  function userInitials(user) {
    const avatar = user && typeof user.avatar === 'string' ? user.avatar.trim() : '';
    if (avatar) return avatar.slice(0, 2).toUpperCase();
    const source = (user && (user.display_name || user.email)) || 'UnyDesk';
    return source
      .split(/[\s@._-]+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0].toUpperCase())
      .join('') || 'U';
  }

  function setCurrentSection(section) {
    state.currentSection = sectionTitles[section] ? section : state.preferences.defaultSection || 'overview';
    workspaceTitleEl.textContent = sectionTitles[state.currentSection];
    navItems.forEach((item) => {
      item.classList.toggle('is-active', item.dataset.section === state.currentSection);
    });
    panelItems.forEach((panel) => {
      panel.classList.toggle('is-active', panel.dataset.panel === state.currentSection);
    });
    if (window.location.hash !== `#${state.currentSection}`) {
      history.replaceState(null, '', `#${state.currentSection}`);
    }
  }

  function setSettingsTab(tab) {
    state.settingsTab = ['profile', 'preferences', 'password'].includes(tab) ? tab : 'profile';
    settingsTabButtons.forEach((button) => {
      button.classList.toggle('is-active', button.dataset.settingsTab === state.settingsTab);
    });
    settingsPanels.forEach((panel) => {
      panel.classList.toggle('is-active', panel.dataset.settingsPanel === state.settingsTab);
    });
  }

  function openSettingsModal(tab = 'profile') {
    setSettingsTab(tab);
    clearSettingsFeedback();
    settingsModalBackdropEl.classList.add('open');
    state.settingsModalOpen = true;
    document.body.classList.add('account-menu-open');
  }

  function closeSettingsModal() {
    settingsModalBackdropEl.classList.remove('open');
    state.settingsModalOpen = false;
    if (!state.mobileMenuOpen) {
      document.body.classList.remove('account-menu-open');
    }
  }

  function startSessionPolling() {
    stopSessionPolling();
    state.sessionPollTimer = window.setInterval(() => {
      if (!state.currentSessionID || !state.sessionModalOpen) return;
      void refreshSessionDetails(false);
    }, 250);
  }

  function stopSessionPolling() {
    if (!state.sessionPollTimer) return;
    window.clearInterval(state.sessionPollTimer);
    state.sessionPollTimer = null;
  }

  function appendTransportLog(message) {
    const stamp = new Date().toLocaleTimeString();
    state.viewerTransportLog.push(`[${stamp}] ${message}`);
    state.viewerTransportLog = state.viewerTransportLog.slice(-40);
    sessionTransportLogEl.value = state.viewerTransportLog.join('\n');
    sessionTransportLogEl.scrollTop = sessionTransportLogEl.scrollHeight;
  }

  function resetViewerTransport() {
    if (state.viewerDataChannel) {
      try { state.viewerDataChannel.close(); } catch (_error) {}
    }
    if (state.viewerPeerConnection) {
      try { state.viewerPeerConnection.close(); } catch (_error) {}
    }
    state.viewerPeerConnection = null;
    state.viewerDataChannel = null;
    state.viewerTransportState = 'Idle';
    state.viewerTransportLog = [];
    sessionTransportLogEl.value = '';
    state.remoteCandidatesSeen = [];
    sessionSendPingBtn.disabled = true;
  }

  function renderSessionDetails(session) {
    if (!session) return;
    sessionModalTitleEl.textContent = `Session Control · ${session.id || '—'}`;
    sessionModalSubtitleEl.textContent = `Target ${session.target || '—'} · Viewer ${session.viewer || '—'}`;
    sessionDetailIDEl.textContent = session.id || '—';
    sessionDetailStatusEl.textContent = session.status || '—';
    sessionDetailTargetEl.textContent = session.target || '—';
    sessionDetailViewerEl.textContent = session.viewer || '—';
    sessionDetailCreatedEl.textContent = formatDateTime(session.created_at);
    sessionDetailUpdatedEl.textContent = formatDateTime(session.updated_at);
    sessionDetailRouteEl.textContent = session.routed_hostname || session.routed_host_public_id || session.routed_host_id || 'Direct / unresolved';
    sessionDetailDispatchEl.textContent = session.dispatch_state || 'Not queued';
    sessionDetailOfferEl.textContent = session.offer_sdp ? 'Ready' : 'Waiting';
    sessionDetailAnswerEl.textContent = session.answer_sdp ? 'Ready' : 'Waiting';
    sessionDetailViewerTransportEl.textContent = state.viewerTransportState;
    sessionDetailCandidatesEl.textContent = String((session.viewer_ice_candidates || []).length + (session.host_ice_candidates || []).length);
    sessionDetailDeliveredAtEl.textContent = formatDateTime(session.last_dispatch_at);
    sessionDetailDeliveriesEl.textContent = String(session.dispatch_count || 0);
    sessionDetailHostAckEl.textContent = formatDateTime(session.last_host_ack_at);
    sessionOfferPreviewEl.value = session.offer_sdp || '';
    sessionAnswerPreviewEl.value = session.answer_sdp || '';
    sessionCloseBtn.disabled = session.status === 'closed';
  }

  function sendViewerControlMessage(payload) {
    if (!state.viewerDataChannel || state.viewerDataChannel.readyState !== 'open') {
      setSessionFeedback('error', 'Viewer data channel is not open yet.');
      return;
    }
    state.viewerDataChannel.send(JSON.stringify(payload));
    appendTransportLog(`sent ${payload.type}`);
  }

  function bindRemotePad() {
    if (!sessionRemotePadEl) return;

    sessionRemotePadEl.addEventListener('mouseenter', () => {
      sessionRemotePadEl.focus();
    });

    sessionRemotePadEl.addEventListener('mousemove', (event) => {
      const rect = sessionRemotePadEl.getBoundingClientRect();
      const x = Math.max(0, Math.min(1, (event.clientX - rect.left) / Math.max(1, rect.width)));
      const y = Math.max(0, Math.min(1, (event.clientY - rect.top) / Math.max(1, rect.height)));
      sendViewerControlMessage({
        type: 'mouse_move',
        session_id: state.currentSessionID,
        x: Number(x.toFixed(4)),
        y: Number(y.toFixed(4)),
      });
    });

    sessionRemotePadEl.addEventListener('click', (event) => {
      const rect = sessionRemotePadEl.getBoundingClientRect();
      const x = Math.max(0, Math.min(1, (event.clientX - rect.left) / Math.max(1, rect.width)));
      const y = Math.max(0, Math.min(1, (event.clientY - rect.top) / Math.max(1, rect.height)));
      sendViewerControlMessage({
        type: 'mouse_click',
        session_id: state.currentSessionID,
        button: event.button,
        x: Number(x.toFixed(4)),
        y: Number(y.toFixed(4)),
      });
    });

    sessionRemotePadEl.addEventListener('keydown', (event) => {
      if (event.repeat) return;
      sendViewerControlMessage({
        type: 'key_down',
        session_id: state.currentSessionID,
        key: event.key,
        code: event.code,
      });
      event.preventDefault();
    });

    sessionRemotePadEl.addEventListener('keyup', (event) => {
      sendViewerControlMessage({
        type: 'key_up',
        session_id: state.currentSessionID,
        key: event.key,
        code: event.code,
      });
      event.preventDefault();
    });
  }

  async function postSessionCandidate(sessionID, candidate) {
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(sessionID)}/candidates`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': state.csrfToken,
      },
      body: JSON.stringify({ candidate, source: 'viewer' }),
    });
    captureCSRF(response);
    if (!response.ok) throw new Error('candidate post failed');
  }

  async function postSessionOffer(sessionID, sdp) {
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(sessionID)}/offer`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': state.csrfToken,
      },
      body: JSON.stringify({ sdp }),
    });
    captureCSRF(response);
    if (!response.ok) throw new Error('offer post failed');
    return response.json();
  }

  async function applyRemoteAnswerIfPresent(session) {
    if (!state.viewerPeerConnection || !session || !session.answer_sdp) return;
    const pc = state.viewerPeerConnection;
    if (pc.remoteDescription && pc.remoteDescription.type === 'answer') return;
    await pc.setRemoteDescription({ type: 'answer', sdp: session.answer_sdp });
    state.viewerTransportState = 'Answer applied';
    appendTransportLog('remote answer applied');
  }

  async function applyRemoteCandidatesIfPresent(session) {
    if (!state.viewerPeerConnection || !session || !Array.isArray(session.host_ice_candidates)) return;
    for (const candidate of session.host_ice_candidates) {
      if (!candidate || state.remoteCandidatesSeen.includes(candidate)) continue;
      state.remoteCandidatesSeen.push(candidate);
      try {
        await state.viewerPeerConnection.addIceCandidate({ candidate });
      } catch (_error) {}
    }
  }

  async function startViewerSignaling() {
    if (!state.currentSessionID) return;
    if (!window.RTCPeerConnection) {
      setSessionFeedback('error', 'WebRTC is not available in this browser.');
      return;
    }
    resetViewerTransport();
    clearSessionFeedback();
    sessionStartSignalingBtn.disabled = true;
    try {
      const pc = new RTCPeerConnection({
        iceServers: [],
        iceCandidatePoolSize: 1,
      });
      state.viewerPeerConnection = pc;
      state.viewerTransportState = 'Starting';
      renderSessionDetails(await fetchSession(state.currentSessionID));

      const dc = pc.createDataChannel('unydesk-control');
      state.viewerDataChannel = dc;
      sessionSendPingBtn.disabled = true;

      dc.onopen = () => {
        state.viewerTransportState = 'Data channel open';
        sessionSendPingBtn.disabled = false;
        appendTransportLog('data channel open');
        sendViewerControlMessage({
          type: 'hello',
          viewer: state.browserIdentity ? state.browserIdentity.public_id : '',
          session_id: state.currentSessionID,
        });
        sendViewerControlMessage({
          type: 'ping',
          session_id: state.currentSessionID,
          sent_at: new Date().toISOString(),
        });
        void refreshSessionDetails(false);
      };
      dc.onclose = () => {
        state.viewerTransportState = 'Data channel closed';
        sessionSendPingBtn.disabled = true;
        appendTransportLog('data channel closed');
        void refreshSessionDetails(false);
      };
      dc.onmessage = (event) => {
        let label = 'message';
        try {
          const data = JSON.parse(String(event.data || '{}'));
          label = data.type || label;
        } catch (_error) {}
        appendTransportLog(`received ${label}`);
      };

      pc.onconnectionstatechange = () => {
        state.viewerTransportState = `Peer ${pc.connectionState}`;
        appendTransportLog(`peer ${pc.connectionState}`);
        void refreshSessionDetails(false);
      };

      pc.onicecandidate = (event) => {
        if (!event.candidate || !state.currentSessionID) return;
        appendTransportLog('viewer ICE candidate gathered');
        void postSessionCandidate(state.currentSessionID, event.candidate.candidate).catch(() => {});
      };

      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      state.viewerTransportState = 'Offer created';
      const updatedSession = await postSessionOffer(state.currentSessionID, offer.sdp || '');
      renderSessionDetails(updatedSession);
      await applyRemoteAnswerIfPresent(updatedSession);
      await applyRemoteCandidatesIfPresent(updatedSession);
      setSessionFeedback('success', 'Viewer offer published. Waiting for host answer.');
    } catch (_error) {
      resetViewerTransport();
      setSessionFeedback('error', 'Unable to start viewer signaling.');
    } finally {
      sessionStartSignalingBtn.disabled = false;
      try {
        const session = await fetchSession(state.currentSessionID);
        renderSessionDetails(session);
      } catch (_error) {}
    }
  }

  async function fetchSession(sessionID) {
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(sessionID)}`);
    captureCSRF(response);
    if (!response.ok) throw new Error('session fetch failed');
    return response.json();
  }

  async function refreshSessionDetails(withFeedback = true) {
    if (!state.currentSessionID) return;
    const session = await fetchSession(state.currentSessionID);
    await applyRemoteAnswerIfPresent(session);
    await applyRemoteCandidatesIfPresent(session);
    renderSessionDetails(session);
    if (withFeedback) {
      clearSessionFeedback();
    }
  }

  async function openSessionModal(sessionID, options = {}) {
    if (!sessionID) return;
    state.currentSessionID = sessionID;
    clearSessionFeedback();
    sessionModalBackdropEl.classList.add('open');
    state.sessionModalOpen = true;
    document.body.classList.add('account-menu-open');
    try {
      await refreshSessionDetails(false);
      if (options.message) setSessionFeedback('success', options.message);
      startSessionPolling();
    } catch (_error) {
      setSessionFeedback('error', 'Unable to load session details right now.');
    }
  }

  function closeSessionModal() {
    resetViewerTransport();
    sessionModalBackdropEl.classList.remove('open');
    state.sessionModalOpen = false;
    state.currentSessionID = '';
    stopSessionPolling();
    clearSessionFeedback();
    if (!state.mobileMenuOpen && !state.settingsModalOpen) {
      document.body.classList.remove('account-menu-open');
    }
  }

  function loadPreferences() {
    try {
      const raw = window.localStorage.getItem(preferenceStorageKey);
      if (!raw) return;
      const saved = JSON.parse(raw);
      if (saved && typeof saved === 'object') {
        if (saved.theme && ['system', 'light', 'dark'].includes(saved.theme)) {
          state.preferences.theme = saved.theme;
        }
        if (saved.defaultSection && sectionTitles[saved.defaultSection]) {
          state.preferences.defaultSection = saved.defaultSection;
        }
      }
    } catch (_error) {}
  }

  function persistPreferences() {
    window.localStorage.setItem(preferenceStorageKey, JSON.stringify(state.preferences));
  }

  function applyTheme(theme) {
    const root = document.documentElement;
    if (theme === 'dark') {
      root.setAttribute('data-theme', 'dark');
      return;
    }
    if (theme === 'light') {
      root.removeAttribute('data-theme');
      return;
    }
    const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
    if (prefersDark) root.setAttribute('data-theme', 'dark');
    else root.removeAttribute('data-theme');
  }

  function renderPreferences() {
    preferencesThemeEl.value = state.preferences.theme;
    preferencesDefaultSectionEl.value = state.preferences.defaultSection;
    applyTheme(state.preferences.theme);
  }

  function openMobileMenu() {
    state.mobileMenuOpen = true;
    mobileOverlayEl.classList.add('is-open');
    mobileMenuEl.classList.add('is-open');
    mobileMenuEl.setAttribute('aria-hidden', 'false');
    document.body.classList.add('account-menu-open');
  }

  function closeMobileMenu() {
    state.mobileMenuOpen = false;
    mobileOverlayEl.classList.remove('is-open');
    mobileMenuEl.classList.remove('is-open');
    mobileMenuEl.setAttribute('aria-hidden', 'true');
    if (!state.settingsModalOpen) {
      document.body.classList.remove('account-menu-open');
    }
  }

  function bindNavigation() {
    navItems.forEach((item) => {
      item.addEventListener('click', () => {
        setCurrentSection(item.dataset.section);
        closeMobileMenu();
      });
    });
  }

  function bindSettingsTabs() {
    settingsTabButtons.forEach((button) => {
      button.addEventListener('click', () => setSettingsTab(button.dataset.settingsTab));
    });
  }

  async function ensureBrowserIdentity() {
    const response = await fetch(browserIdentityRequestURL());
    captureCSRF(response);
    if (!response.ok) throw new Error('browser identity fetch failed');
    state.browserIdentity = await response.json();
    if (state.browserIdentity && state.browserIdentity.token) {
      window.localStorage.setItem(browserTokenStorageKey(), state.browserIdentity.token);
    }
    browserIDEl.textContent = state.browserIdentity.public_id || 'Unavailable';
    browserIDEl.title = 'Reserved local client identity for this browser';
    if (!sessionViewerEl.value) {
      sessionViewerEl.value = state.browserIdentity.public_id || '';
    }
  }

  async function loadSession() {
    const response = await fetch('/api/v1/auth/session');
    captureCSRF(response);
    if (!response.ok) throw new Error('session fetch failed');
    const data = await response.json();
    if (!data.authenticated || !data.user) {
      window.location.assign('/');
      return false;
    }
    state.user = data.user;
    renderSession();
    return true;
  }

  function renderSession() {
    const name = state.user.display_name || 'Authenticated user';
    const email = state.user.email || '—';
    const initials = userInitials(state.user);
    statusEl.textContent = 'Authenticated';
    nameEl.textContent = name;
    emailEl.textContent = email;
    sidebarUserNameEl.textContent = name;
    sidebarUserEmailEl.textContent = email;
    mobileUserNameEl.textContent = name;
    mobileUserEmailEl.textContent = email;
    sidebarAvatarEl.textContent = initials;
    mobileAvatarEl.textContent = initials;
    settingsModalNameEl.textContent = name;
    settingsModalEmailEl.textContent = email;
    settingsModalAvatarEl.textContent = initials;
    profileAvatarEl.value = state.user.avatar || initials;
    profileDisplayNameEl.value = name;
    profileEmailEl.value = email === '—' ? '' : email;
  }

  async function loadHosts() {
    const response = await fetch('/api/v1/hosts');
    captureCSRF(response);
    if (!response.ok) throw new Error('host fetch failed');
    const data = await response.json();
    state.hosts = data.hosts || [];
    renderHosts();
  }

  function renderHosts() {
    const onlineCount = state.hosts.filter((host) => host.status === 'online').length;
    const totalCount = state.hosts.length;
    hostsOnlineEl.textContent = String(onlineCount);
    hostsTotalEl.textContent = String(totalCount);
    hostsOfflineEl.textContent = String(Math.max(0, totalCount - onlineCount));
    renderOverviewHosts();
    if (!state.hosts.length) {
      hostsTableBody.innerHTML = '<tr><td colspan="5">No hosts registered yet.</td></tr>';
      return;
    }
    hostsTableBody.innerHTML = state.hosts.map((host) => `
      <tr>
        <td class="host-table-cell">
          <div class="host-name-stack">
            <strong>${escapeHTML(host.hostname || host.name || 'unknown host')}</strong>
            <span class="host-secondary-id">${escapeHTML(`${hostRoleLabel(host)} role`)}</span>
          </div>
        </td>
        <td>${escapeHTML(host.public_id || '—')}</td>
        <td>${escapeHTML(`${host.os || '?'} / ${host.arch || '?'}`)}</td>
        <td><span class="table-status ${hostStatusClass(host)}">${escapeHTML(host.status || 'unknown')}</span></td>
        <td>
          <div class="host-table-actions">
            <button
              class="btn btn-secondary host-control-btn"
              type="button"
              data-target="${escapeHTML(host.public_id || host.id || '')}"
              ${host.status === 'online' ? '' : 'disabled'}
            >
              <i class="fa-solid fa-tv"></i>
              <span>Control</span>
            </button>
          </div>
        </td>
      </tr>
    `).join('');
    bindHostControlActions();
  }

  function renderOverviewHosts() {
    const onlineHosts = state.hosts.filter((host) => host.status === 'online');
    if (!onlineHosts.length) {
      overviewHostsGridEl.innerHTML = '<div class="overview-host-empty">No connected hosts right now.</div>';
      return;
    }
    overviewHostsGridEl.innerHTML = onlineHosts.map((host) => `
      <article class="overview-host-card">
        <div class="overview-host-card-head">
          <strong class="overview-host-title">
            <i class="${escapeHTML(hostOSIcon(host))} overview-host-os"></i>
            <span class="overview-host-title-text">
              <span>${escapeHTML(host.hostname || host.name || 'unknown host')}</span>
              <span class="host-secondary-id">${escapeHTML(host.public_id || '—')}</span>
            </span>
          </strong>
          <span class="table-status ${hostStatusClass(host)}">${escapeHTML(host.status || 'unknown')}</span>
        </div>
        <div class="overview-host-card-meta">
          <span>${escapeHTML(`${hostRoleLabel(host)} role · ${host.os || '?'} / ${host.arch || '?'}`)}</span>
        </div>
        <div class="overview-host-actions">
          <button
            class="btn btn-secondary overview-host-action"
            type="button"
            data-target="${escapeHTML(host.public_id || host.id || '')}"
          >
            <i class="fa-solid fa-tv"></i>
            <span>Control host</span>
          </button>
        </div>
      </article>
    `).join('');
    bindHostControlActions();
  }

  function bindHostControlActions() {
    document.querySelectorAll('[data-target]').forEach((button) => {
      if (button.dataset.controlBound === '1') return;
      button.dataset.controlBound = '1';
      button.addEventListener('click', async () => {
        const target = String(button.dataset.target || '').trim();
        if (!target) return;
        sessionTargetEl.value = target;
        if (!sessionViewerEl.value && state.browserIdentity && state.browserIdentity.public_id) {
          sessionViewerEl.value = state.browserIdentity.public_id;
        }
        await createSession({ switchToSessions: false, preserveTarget: true });
      });
    });
  }

  async function loadSessions() {
    const response = await fetch('/api/v1/sessions');
    captureCSRF(response);
    if (!response.ok) throw new Error('session list fetch failed');
    const data = await response.json();
    state.sessions = data.sessions || [];
    renderSessions();
  }

  function renderSessions() {
    const activeCount = state.sessions.filter((session) => session.status === 'active').length;
    sessionsActiveEl.textContent = String(activeCount);
    sessionsTotalEl.textContent = String(state.sessions.length);
    if (!state.sessions.length) {
      sessionsTableBody.innerHTML = '<tr><td colspan="6">No sessions created yet.</td></tr>';
      return;
    }
    sessionsTableBody.innerHTML = [...state.sessions]
      .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
      .map((session) => `
        <tr>
          <td><strong>${escapeHTML(session.id || '—')}</strong></td>
          <td>${escapeHTML(session.target || '—')}</td>
          <td>${escapeHTML(session.viewer || '—')}</td>
          <td><span class="table-status">${escapeHTML(session.status || 'unknown')}</span></td>
          <td>${escapeHTML(relativeTime(session.updated_at))}</td>
          <td>
            <button
              class="btn btn-secondary session-open-btn"
              type="button"
              data-session-open="${escapeHTML(session.id || '')}"
            >
              <i class="fa-solid fa-tower-broadcast"></i>
              <span>Open</span>
            </button>
          </td>
        </tr>
      `).join('');
    bindSessionActions();
  }

  function bindSessionActions() {
    document.querySelectorAll('[data-session-open]').forEach((button) => {
      if (button.dataset.sessionBound === '1') return;
      button.dataset.sessionBound = '1';
      button.addEventListener('click', async () => {
        const sessionID = String(button.dataset.sessionOpen || '').trim();
        if (!sessionID) return;
        openSessionTab(sessionID);
      });
    });
  }

  async function createSession(options = {}) {
    const switchToSessions = options.switchToSessions !== false;
    const preserveTarget = options.preserveTarget === true;
    if (state.creatingSession) return;
    clearFeedback(null);
    const target = sessionTargetEl.value.trim();
    const viewer = sessionViewerEl.value.trim();
    if (!target || !viewer) {
      setFeedback(null, 'error', 'Target and viewer are required.');
      return;
    }
    state.creatingSession = true;
    sessionCreateBtn.disabled = true;
    try {
      const response = await fetch('/api/v1/sessions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': state.csrfToken,
        },
        body: JSON.stringify({ target, viewer }),
      });
      captureCSRF(response);
      const data = await response.json();
      if (!response.ok) {
        setFeedback(null, 'error', data.error || 'Unable to create session.');
        return;
      }
      if (!preserveTarget) {
        sessionTargetEl.value = '';
      }
      setFeedback(null, 'success', `Session ${data.id} is ready.`);
      await loadSessions();
      if (switchToSessions) {
        setCurrentSection('sessions');
      }
      openSessionTab(data.id);
    } finally {
      state.creatingSession = false;
      sessionCreateBtn.disabled = false;
    }
  }

  async function closeCurrentSession() {
    if (!state.currentSessionID) return;
    clearSessionFeedback();
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(state.currentSessionID)}/close`, {
      method: 'POST',
      headers: { 'X-CSRF-Token': state.csrfToken },
    });
    captureCSRF(response);
    const data = await response.json();
    if (!response.ok) {
      setSessionFeedback('error', data.error || 'Unable to close this session.');
      return;
    }
    renderSessionDetails(data);
    setSessionFeedback('success', `Session ${data.id} closed.`);
    await loadSessions();
  }

  async function changePassword() {
    clearSettingsFeedback();
    const currentPassword = passwordCurrentEl.value;
    const newPassword = passwordNewEl.value;
    if (!currentPassword || !newPassword) {
      setSettingsFeedback('error', 'Current and new password are required.');
      return;
    }
    const response = await fetch('/api/v1/auth/password', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': state.csrfToken,
      },
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    });
    captureCSRF(response);
    const data = await response.json();
    if (!response.ok) {
      setSettingsFeedback('error', data.error || 'Unable to update password.');
      return;
    }
    passwordCurrentEl.value = '';
    passwordNewEl.value = '';
    setSettingsFeedback('success', 'Password updated successfully.');
  }

  async function saveProfile() {
    clearSettingsFeedback();
    const displayName = profileDisplayNameEl.value.trim();
    const email = profileEmailEl.value.trim();
    const avatar = profileAvatarEl.value.trim().toUpperCase();
    if (!displayName || !email) {
      setSettingsFeedback('error', 'Display name and email are required.');
      return;
    }
    const response = await fetch('/api/v1/profile', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': state.csrfToken,
      },
      body: JSON.stringify({ display_name: displayName, email, avatar }),
    });
    captureCSRF(response);
    const data = await response.json();
    if (!response.ok) {
      setSettingsFeedback('error', data.error || 'Unable to save profile.');
      return;
    }
    state.user = data.user || state.user;
    renderSession();
    setSettingsFeedback('success', 'Profile updated successfully.');
  }

  function savePreferences() {
    clearSettingsFeedback();
    state.preferences.theme = preferencesThemeEl.value;
    state.preferences.defaultSection = preferencesDefaultSectionEl.value;
    persistPreferences();
    renderPreferences();
    setSettingsFeedback('success', 'Preferences saved in this browser.');
  }

  async function logout() {
    await fetch('/api/v1/auth/logout', {
      method: 'POST',
      headers: { 'X-CSRF-Token': state.csrfToken },
    }).then(captureCSRF).catch(() => {});
    window.location.assign('/');
  }

  sessionCreateBtn.addEventListener('click', createSession);
  profileSaveBtn.addEventListener('click', saveProfile);
  preferencesSaveBtn.addEventListener('click', savePreferences);
  passwordChangeBtn.addEventListener('click', changePassword);
  logoutBtn.addEventListener('click', logout);
  mobileLogoutBtn.addEventListener('click', logout);
  mobileOpenBtn.addEventListener('click', openMobileMenu);
  mobileCloseBtn.addEventListener('click', closeMobileMenu);
  mobileOverlayEl.addEventListener('click', closeMobileMenu);
  sidebarUserTriggerEl.addEventListener('click', () => openSettingsModal('profile'));
  mobileUserTriggerEl.addEventListener('click', () => {
    openSettingsModal('profile');
    closeMobileMenu();
  });
  sidebarSettingsTriggerEl.addEventListener('click', () => openSettingsModal('profile'));
  mobileSettingsTriggerEl.addEventListener('click', () => {
    openSettingsModal('profile');
    closeMobileMenu();
  });
  settingsModalCloseEl.addEventListener('click', closeSettingsModal);
  sessionModalCloseEl.addEventListener('click', closeSessionModal);
  sessionRefreshBtn.addEventListener('click', async () => {
    try {
      await refreshSessionDetails();
    } catch (_error) {
      setSessionFeedback('error', 'Unable to refresh session details right now.');
    }
  });
  sessionStartSignalingBtn.addEventListener('click', startViewerSignaling);
  sessionSendPingBtn.addEventListener('click', () => {
    sendViewerControlMessage({
      type: 'ping',
      session_id: state.currentSessionID,
      sent_at: new Date().toISOString(),
    });
  });
  sessionCloseBtn.addEventListener('click', closeCurrentSession);
  settingsModalBackdropEl.addEventListener('click', (event) => {
    if (event.target === settingsModalBackdropEl) closeSettingsModal();
  });
  sessionModalBackdropEl.addEventListener('click', (event) => {
    if (event.target === sessionModalBackdropEl) closeSessionModal();
  });
  window.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && state.mobileMenuOpen) closeMobileMenu();
    if (event.key === 'Escape' && state.settingsModalOpen) closeSettingsModal();
    if (event.key === 'Escape' && state.sessionModalOpen) closeSessionModal();
  });

  bindNavigation();
  bindRemotePad();
  bindSettingsTabs();
  loadPreferences();
  renderPreferences();

  try {
    await ensureBrowserIdentity();
    const authenticated = await loadSession();
    if (!authenticated) return;
    const initialSection = (window.location.hash || '').replace(/^#/, '');
    setCurrentSection(initialSection || state.preferences.defaultSection || 'overview');
    await Promise.all([loadHosts(), loadSessions()]);
    window.setInterval(() => loadHosts().catch(() => {}), 2000);
    window.setInterval(() => loadSessions().catch(() => {}), 2000);
  } catch (_error) {
    statusEl.textContent = 'Dashboard unavailable';
  }
})();
