import {
  escapeHTML,
  formatDateTime,
  hostAvailabilityLine,
  hostOSIcon,
  hostPlatformLabel,
  hostReadyForSession,
  hostRoleLabel,
  hostStatusClass,
  hostStatusLabel,
  relativeTime,
  userInitials,
} from "/account/shared.js";

(async () => {
  const state = {
    csrfToken: '',
    browserIdentity: null,
    localHostRuntime: window.unydeskLocalHostBridge && window.unydeskLocalHostBridge.defaultRuntime
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
        },
    user: null,
    hosts: [],
    sessions: [],
    currentSection: 'overview',
    mobileMenuOpen: false,
    settingsModalOpen: false,
    sessionModalOpen: false,
    approvalWaitModalOpen: false,
    currentSessionID: '',
    sessionPollTimer: null,
    localHostPollTimer: null,
    localHostLastClaimAt: 0,
    sessionPollInFlight: false,
    viewerTransportState: 'Dedicated control page',
    viewerTransportLog: [],
    lastSessionLogKey: '',
    creatingSession: false,
    settingsTab: 'profile',
    preferences: {
      theme: 'system',
      defaultSection: 'overview',
    },
    localHostProbePromise: null,
    localHostClaimPromise: null,
    approvalWaitSessionID: '',
    approvalWaitTarget: '',
  };

  const sectionTitles = {
    overview: 'Overview',
    connections: 'Connect',
    hosts: 'Machines',
    sessions: 'Sessions',
  };
  const sessionsCollectionURL = '/api/v1/sessions/';

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
  const approvalWaitBackdropEl = document.getElementById('approval-wait-backdrop');
  const approvalWaitCloseEl = document.getElementById('approval-wait-close');
  const approvalWaitTitleEl = document.getElementById('approval-wait-title');
  const approvalWaitSubtitleEl = document.getElementById('approval-wait-subtitle');
  const approvalWaitMessageEl = document.getElementById('approval-wait-message');
  const approvalWaitTargetEl = document.getElementById('approval-wait-target');
  const approvalWaitSessionEl = document.getElementById('approval-wait-session');
  const approvalWaitFeedbackWrapEl = document.getElementById('approval-wait-feedback-wrap');
  const approvalWaitFeedbackEl = document.getElementById('approval-wait-feedback');
  const approvalWaitRepromptBtn = document.getElementById('approval-wait-reprompt-btn');
  const approvalWaitDismissBtn = document.getElementById('approval-wait-dismiss-btn');
  const sessionModalBackdropEl = document.getElementById('session-modal-backdrop');
  const sessionModalCloseEl = document.getElementById('session-modal-close');
  const sessionModalTitleEl = document.getElementById('session-modal-title');
  const sessionModalSubtitleEl = document.getElementById('session-modal-subtitle');
  const sessionRefreshBtn = document.getElementById('session-refresh-btn');
  const sessionRepromptBtn = document.getElementById('session-reprompt-btn');
  const sessionCloseBtn = document.getElementById('session-close-btn');
  const sessionOpenControlBtn = document.getElementById('session-open-control-btn');
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
  const sessionLogToggleEl = document.getElementById('session-log-toggle');
  const sessionLogBodyEl = document.getElementById('session-log-body');
  const settingsTabButtons = Array.from(document.querySelectorAll('.settings-modal-tab'));
  const settingsPanels = Array.from(document.querySelectorAll('.settings-tab-panel'));
  const overviewHostsGridEl = document.getElementById('overview-hosts-grid');
  const hostsTableBody = document.getElementById('hosts-table-body');
  const sessionsTableBody = document.getElementById('sessions-table-body');
  const sessionTargetEl = document.getElementById('session-target');
  const sessionPasswordEl = document.getElementById('session-password');
  const sessionConnectStatusEl = document.getElementById('session-connect-status');
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

  function t(value, fallback = value, params) {
    if (window.unydeskI18n && typeof window.unydeskI18n.t === 'function') {
      return window.unydeskI18n.t(value, fallback, params);
    }
    return fallback;
  }

  function translateFragment(root) {
    if (!root) return;
    if (window.unydeskI18n && typeof window.unydeskI18n.apply === 'function') {
      window.unydeskI18n.apply(root);
    }
  }

  function openSessionTab(sessionID) {
    if (!sessionID) return;
    const url = sessionControlURL(sessionID);
    const target = `unydesk-control-${sessionID.replace(/[^a-z0-9_-]/gi, '')}`;
    const opened = window.open(url, target);
    if (!opened) {
      window.location.assign(url);
      return;
    }
    opened.focus();
  }

  function sessionApprovalReady(session) {
    if (!session) return false;
    return session.status === 'active' || session.dispatch_state === 'accepted';
  }

  function sessionApprovalRejected(session) {
    if (!session) return false;
    return session.status === 'closed' || session.dispatch_state === 'rejected';
  }

  function sessionAwaitingApproval(session) {
    if (!session) return false;
    return !sessionApprovalReady(session) && !sessionApprovalRejected(session);
  }

  function beginApprovalWait(sessionID, target = '', message = '') {
    state.approvalWaitSessionID = String(sessionID || '').trim();
    state.approvalWaitTarget = String(target || '').trim();
    setCurrentSection('overview');
    return openApprovalWaitModal(state.approvalWaitSessionID, state.approvalWaitTarget, message || t('Waiting for host approval.'));
  }

  function clearApprovalWait(sessionID = '') {
    const current = String(state.approvalWaitSessionID || '').trim();
    const target = String(sessionID || '').trim();
    if (!target || current === target) {
      state.approvalWaitSessionID = '';
      state.approvalWaitTarget = '';
    }
  }

  function setApprovalWaitFeedback(kind, message) {
    setFeedback(approvalWaitFeedbackEl, kind, message);
    if (approvalWaitFeedbackWrapEl) approvalWaitFeedbackWrapEl.classList.remove('hidden');
  }

  function clearApprovalWaitFeedback() {
    clearFeedback(approvalWaitFeedbackEl);
    if (approvalWaitFeedbackWrapEl) approvalWaitFeedbackWrapEl.classList.add('hidden');
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

  async function fetchWithTimeout(url, options = {}, timeoutMs = 4500) {
    if (!window.AbortController) return fetch(url, options);
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), timeoutMs);
    try {
      return await fetch(url, { ...options, signal: controller.signal });
    } finally {
      window.clearTimeout(timeout);
    }
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

  function sessionOverlayOpen() {
    return state.sessionModalOpen || state.approvalWaitModalOpen;
  }

  async function openApprovalWaitModal(sessionID, target = '', message = '') {
    if (!sessionID) return;
    state.currentSessionID = sessionID;
    clearApprovalWaitFeedback();
    approvalWaitTitleEl.textContent = t('Waiting for approval');
    approvalWaitSubtitleEl.textContent = t('The host must authorize this connection before live control opens.');
    approvalWaitMessageEl.textContent = message || t('Keep this window open while the host decides, or relaunch the request if the host popup was closed.');
    approvalWaitTargetEl.textContent = target || '—';
    approvalWaitSessionEl.textContent = sessionID || '—';
    approvalWaitBackdropEl.classList.add('open');
    state.approvalWaitModalOpen = true;
    document.body.classList.add('account-menu-open');
    startSessionPolling();
    try {
      const session = await fetchSession(sessionID);
      if (sessionApprovalReady(session)) {
        clearApprovalWait(sessionID);
        closeApprovalWaitModal({ preserveTracking: false });
        openSessionTab(sessionID);
        return;
      }
      if (sessionApprovalRejected(session)) {
        setApprovalWaitFeedback('error', t('Host approval was refused or the request expired.'));
      }
    } catch (_error) {
      setApprovalWaitFeedback('error', t('Unable to load session details right now.'));
    }
  }

  function closeApprovalWaitModal(options = {}) {
    approvalWaitBackdropEl.classList.remove('open');
    state.approvalWaitModalOpen = false;
    clearApprovalWaitFeedback();
    if (!options.preserveTracking) {
      clearApprovalWait(state.currentSessionID);
      if (!state.sessionModalOpen) {
        state.currentSessionID = '';
      }
    }
    if (!sessionOverlayOpen() && !state.mobileMenuOpen && !state.settingsModalOpen) {
      document.body.classList.remove('account-menu-open');
    }
    if (!sessionOverlayOpen()) {
      stopSessionPolling();
    }
  }

  function startSessionPolling() {
    stopSessionPolling();
    state.sessionPollTimer = window.setInterval(async () => {
      if (!state.currentSessionID || !sessionOverlayOpen() || document.hidden || state.sessionPollInFlight) return;
      state.sessionPollInFlight = true;
      try {
        await refreshSessionDetails(false);
      } catch (_error) {
      } finally {
        state.sessionPollInFlight = false;
      }
    }, 1500);
  }

  function stopSessionPolling() {
    if (!state.sessionPollTimer) return;
    window.clearInterval(state.sessionPollTimer);
    state.sessionPollTimer = null;
    state.sessionPollInFlight = false;
  }

  function appendTransportLog(message) {
    const stamp = new Date().toLocaleTimeString();
    state.viewerTransportLog.push(`[${stamp}] ${message}`);
    state.viewerTransportLog = state.viewerTransportLog.slice(-40);
    renderTransportLog();
  }

  function renderTransportLog() {
    if (!sessionTransportLogEl) return;
    const enabled = !sessionLogToggleEl || sessionLogToggleEl.checked;
    if (sessionLogBodyEl) sessionLogBodyEl.classList.toggle('hidden', !enabled);
    if (!enabled) return;
    sessionTransportLogEl.value = state.viewerTransportLog.join('\n');
    sessionTransportLogEl.scrollTop = sessionTransportLogEl.scrollHeight;
  }

  function resetViewerTransport() {
    state.viewerTransportState = 'Dedicated control page';
    state.viewerTransportLog = [];
    renderTransportLog();
  }

  function renderSessionDetails(session) {
    if (!session) return;
    const ready = sessionApprovalReady(session);
    const waiting = sessionAwaitingApproval(session);
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
    sessionOpenControlBtn.disabled = !ready;
    if (sessionRepromptBtn) {
      sessionRepromptBtn.disabled = !waiting;
      sessionRepromptBtn.classList.toggle('hidden', !waiting);
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
    renderSessionDetails(session);
    if (state.approvalWaitSessionID && state.approvalWaitSessionID === String(session.id || '').trim()) {
      if (sessionApprovalReady(session)) {
        clearApprovalWait(session.id);
        closeApprovalWaitModal({ preserveTracking: false });
        openSessionTab(session.id);
        return;
      }
      if (sessionApprovalRejected(session)) {
        setApprovalWaitFeedback('error', t('Host approval was refused or the request expired.'));
      }
    }
    const logKey = `${session.id || ''}:${session.status || 'unknown'}:${session.dispatch_state || 'none'}:${session.updated_at || ''}`;
    if (logKey !== state.lastSessionLogKey) {
      state.lastSessionLogKey = logKey;
      appendTransportLog(`session state · status=${session.status || 'unknown'} · dispatch=${session.dispatch_state || 'none'}`);
    }
    if (withFeedback) {
      clearSessionFeedback();
    }
  }

  async function openSessionModal(sessionID, options = {}) {
    if (!sessionID) return;
    state.currentSessionID = sessionID;
    state.lastSessionLogKey = '';
    clearSessionFeedback();
    sessionModalBackdropEl.classList.add('open');
    state.sessionModalOpen = true;
    document.body.classList.add('account-menu-open');
    try {
      resetViewerTransport();
      await refreshSessionDetails(false);
      appendTransportLog(`session opened · ${sessionID}`);
      if (options.message) setSessionFeedback('success', options.message);
      startSessionPolling();
    } catch (_error) {
      setSessionFeedback('error', 'Unable to load session details right now.');
    }
  }

  function closeSessionModal() {
    appendTransportLog(`session closed · ${state.currentSessionID || 'unknown'}`);
    resetViewerTransport();
    state.lastSessionLogKey = '';
    sessionModalBackdropEl.classList.remove('open');
    state.sessionModalOpen = false;
    state.currentSessionID = '';
    stopSessionPolling();
    clearSessionFeedback();
    if (!sessionOverlayOpen() && !state.mobileMenuOpen && !state.settingsModalOpen) {
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
  }

  async function discoverLocalHostRuntime() {
    if (state.localHostProbePromise) return state.localHostProbePromise;
    state.localHostProbePromise = (async () => {
      try {
        const bridge = window.unydeskLocalHostBridge;
        if (!bridge || !bridge.discover) throw new Error('local host bridge unavailable');
        state.localHostRuntime = await bridge.discover(fetchWithTimeout, state.localHostRuntime);
        return true;
      } catch (_error) {
        const bridge = window.unydeskLocalHostBridge;
        state.localHostRuntime = bridge && bridge.defaultRuntime
          ? bridge.defaultRuntime()
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
            };
        return false;
      }
    })();

    try {
      return await state.localHostProbePromise;
    } finally {
      state.localHostProbePromise = null;
    }
  }

  async function claimLocalHostSilently() {
    if (state.localHostClaimPromise) return state.localHostClaimPromise;
    if (!state.user || !state.localHostRuntime.available) return false;
    const now = Date.now();
    if (state.localHostRuntime.connected) return false;
    if (now - state.localHostLastClaimAt < 15000) return false;
    state.localHostLastClaimAt = now;
    state.localHostClaimPromise = (async () => {
      try {
        const response = await fetch('/api/v1/bootstrap/claim', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': state.csrfToken,
          },
          body: JSON.stringify({
            install_id: state.localHostRuntime.install_id,
            public_id: state.localHostRuntime.public_id,
            hostname: state.localHostRuntime.hostname,
            version: state.localHostRuntime.version,
          }),
        });
        captureCSRF(response);
        const data = await response.json().catch(() => ({}));
        if (!response.ok) return false;
        const bridge = window.unydeskLocalHostBridge;
        if (!bridge || !bridge.bootstrap) return false;
        state.localHostRuntime = await bridge.bootstrap(fetchWithTimeout, state.localHostRuntime, {
          domain: data.domain,
          server_url: data.server_url,
          install_id: data.install_id || state.localHostRuntime.install_id,
          public_id: data.public_id || state.localHostRuntime.public_id,
          credential: data.credential,
          credential_type: data.credential_type,
        });
        await discoverLocalHostRuntime();
        if (state.localHostRuntime.provisioned) {
          await loadHosts().catch(() => {});
        }
        return !!state.localHostRuntime.provisioned;
      } catch (_error) {
        return false;
      } finally {
        state.localHostClaimPromise = null;
      }
    })();
    return state.localHostClaimPromise;
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
    const response = await fetch('/api/v1/admin/hosts');
    captureCSRF(response);
    if (!response.ok) throw new Error('host fetch failed');
    const data = await response.json();
    state.hosts = sortHostsStable(data.hosts || []);
    renderHosts();
    updateConnectStatus();
  }

  function renderHosts() {
    const hosts = sortHostsStable(state.hosts);
    const readyCount = hosts.filter(hostReadyForSession).length;
    const totalCount = hosts.length;
    hostsOnlineEl.textContent = String(readyCount);
    hostsTotalEl.textContent = String(totalCount);
    hostsOfflineEl.textContent = String(Math.max(0, totalCount - readyCount));
    renderOverviewHosts(hosts);
    if (!hosts.length) {
      hostsTableBody.innerHTML = '<tr><td colspan="5">No registered machines yet.</td></tr>';
      translateFragment(hostsTableBody);
      return;
    }
    hostsTableBody.innerHTML = hosts.map((host) => `
      <tr>
        <td class="host-table-cell">
          <div class="host-name-stack">
            <strong>${escapeHTML(host.hostname || host.name || 'unknown host')}</strong>
            <span class="host-secondary-id">${escapeHTML(`${hostRoleLabel(host)} role · ${hostAvailabilityLine(host)}${host.trusted ? ` · ${t('trusted access')}` : ` · ${t('first access uses host password')}`}`)}</span>
          </div>
        </td>
        <td>${escapeHTML(host.public_id || '—')}</td>
        <td>${escapeHTML(hostPlatformLabel(host))}</td>
        <td><span class="table-status ${hostStatusClass(host)}">${escapeHTML(hostStatusLabel(host))}</span></td>
        <td>
          <div class="host-table-actions">
            <button
            class="btn btn-secondary host-control-btn"
            type="button"
            data-target="${escapeHTML(host.public_id || host.id || '')}"
            ${hostReadyForSession(host) ? '' : 'disabled'}
          >
            <i class="fa-solid fa-tv"></i>
            <span>${hostReadyForSession(host) ? (host.trusted ? t('Connect now') : t('Authenticate once')) : t('Unavailable')}</span>
          </button>
          ${host.trusted ? `
          <button
            class="btn btn-secondary host-untrust-btn"
            type="button"
            data-target="${escapeHTML(host.id || host.public_id || '')}"
          >
            <i class="fa-solid fa-user-xmark"></i>
            <span>${t('Forget')}</span>
          </button>
          ` : ''}
          </div>
        </td>
      </tr>
    `).join('');
    bindHostControlActions();
    bindHostTrustActions();
    translateFragment(hostsTableBody);
  }

  function sortHostsStable(hosts) {
    return [...hosts].sort(compareHostsStable);
  }

  function compareHostsStable(a, b) {
    const leftRegistered = hostRegisteredAtTime(a);
    const rightRegistered = hostRegisteredAtTime(b);
    if (leftRegistered !== rightRegistered) return leftRegistered - rightRegistered;
    return hostStableSortKey(a).localeCompare(hostStableSortKey(b));
  }

  function hostRegisteredAtTime(host) {
    const value = new Date((host && host.registered_at) || '').getTime();
    return Number.isFinite(value) ? value : Number.MAX_SAFE_INTEGER;
  }

  function hostStableSortKey(host) {
    return [
      host && host.hostname,
      host && host.name,
      host && host.public_id,
      host && host.install_id,
      host && host.id,
    ].map((part) => String(part || '').trim().toLowerCase()).join('\u0000');
  }

  function renderOverviewHosts(hosts = state.hosts) {
    const registeredHosts = sortHostsStable(hosts);
    if (!registeredHosts.length) {
      overviewHostsGridEl.innerHTML = '<div class="overview-host-empty">No registered machines yet.</div>';
      translateFragment(overviewHostsGridEl);
      return;
    }
    overviewHostsGridEl.innerHTML = registeredHosts.map((host) => `
      <article class="overview-host-card ${hostReadyForSession(host) ? 'is-ready' : 'is-unavailable'}">
        <div class="overview-host-card-head">
          <strong class="overview-host-title">
            <i class="${escapeHTML(hostOSIcon(host))} overview-host-os"></i>
            <span class="overview-host-title-text">
              <span>${escapeHTML(host.hostname || host.name || 'unknown host')}</span>
              <span class="host-secondary-id">${escapeHTML(host.public_id || '—')}</span>
            </span>
          </strong>
          <span class="table-status ${hostStatusClass(host)}">${escapeHTML(hostStatusLabel(host))}</span>
        </div>
        <div class="overview-host-card-meta">
          <span>${escapeHTML(`${hostRoleLabel(host)} role · ${hostPlatformLabel(host)}`)}</span>
          <span>${escapeHTML(`${hostAvailabilityLine(host)}${host.trusted ? ` · ${t('trusted access')}` : ` · ${t('first access needs the host password')}`}`)}</span>
        </div>
        <div class="overview-host-actions">
          <button
            class="btn btn-secondary overview-host-action"
            type="button"
            data-target="${escapeHTML(host.public_id || host.id || '')}"
            ${hostReadyForSession(host) ? '' : 'disabled'}
          >
            <i class="fa-solid fa-tv"></i>
            <span>${hostReadyForSession(host) ? (host.trusted ? t('Connect now') : t('Authenticate once')) : t('Not available')}</span>
          </button>
          ${host.trusted ? `
          <button
            class="btn btn-secondary host-untrust-btn"
            type="button"
            data-target="${escapeHTML(host.id || host.public_id || '')}"
          >
            <i class="fa-solid fa-user-xmark"></i>
            <span>${t('Forget')}</span>
          </button>
          ` : ''}
        </div>
      </article>
    `).join('');
    bindHostControlActions();
    bindHostTrustActions();
    translateFragment(overviewHostsGridEl);
  }

  function normalizeTargetLookup(value) {
    return String(value || '').trim().toLowerCase();
  }

  function findKnownHostByTarget(target) {
    const needle = normalizeTargetLookup(target);
    if (!needle) return null;
    return state.hosts.find((host) => {
      return [
        host && host.id,
        host && host.public_id,
        host && host.hostname,
        host && host.name,
      ].some((value) => normalizeTargetLookup(value) === needle);
    }) || null;
  }

  function normalizeSessionKeyPart(value) {
    return String(value || '').trim().toLowerCase();
  }

  function sessionBelongsToCurrentViewer(session) {
    const currentViewer = normalizeSessionKeyPart(state.browserIdentity && state.browserIdentity.public_id);
    if (!currentViewer) return false;
    return normalizeSessionKeyPart(session && session.viewer) === currentViewer;
  }

  function sessionMatchesTarget(session, target) {
    const needle = normalizeTargetLookup(target);
    if (!needle || !session) return false;
    return [
      session.target,
      session.routed_host_public_id,
      session.routed_hostname,
      session.routed_host_id,
    ].some((value) => normalizeTargetLookup(value) === needle);
  }

  function sessionReusableRank(session) {
    if (!session) return -1;
    if (session.status === 'active' || session.dispatch_state === 'accepted') return 4;
    if (session.status === 'offered') return 3;
    if (session.dispatch_state === 'delivered' || session.dispatch_state === 'queued' || session.dispatch_state === 'busy') return 2;
    if (session.status === 'pending') return 1;
    return -1;
  }

  function compareSessionFreshness(a, b) {
    const left = new Date((a && a.updated_at) || (a && a.created_at) || '').getTime();
    const right = new Date((b && b.updated_at) || (b && b.created_at) || '').getTime();
    const safeLeft = Number.isFinite(left) ? left : 0;
    const safeRight = Number.isFinite(right) ? right : 0;
    return safeRight - safeLeft;
  }

  function findReusableSession(target) {
    return [...state.sessions]
      .filter((session) => session && session.status !== 'closed')
      .filter((session) => sessionBelongsToCurrentViewer(session))
      .filter((session) => sessionMatchesTarget(session, target))
      .sort((left, right) => {
        const rankDelta = sessionReusableRank(right) - sessionReusableRank(left);
        if (rankDelta !== 0) return rankDelta;
        return compareSessionFreshness(left, right);
      })[0] || null;
  }

  function updateConnectStatus() {
    if (!sessionConnectStatusEl) return;
    const selectedHost = findKnownHostByTarget(sessionTargetEl.value);
    if (selectedHost && selectedHost.trusted) {
      sessionConnectStatusEl.textContent = t('Trusted machine detected. You can reconnect without retyping the host password.');
      sessionPasswordEl.placeholder = t('Optional for this trusted machine');
      if (document.activeElement !== sessionPasswordEl) {
        sessionPasswordEl.value = '';
      }
      return;
    }
    if (selectedHost) {
      sessionConnectStatusEl.textContent = t('First approved connection will remember this host automatically for your account.');
      sessionPasswordEl.placeholder = t('Password currently shown on the host browser');
      return;
    }
    sessionConnectStatusEl.textContent = t('The first authenticated connection remembers this host for your account automatically.');
    sessionPasswordEl.placeholder = t('Password currently shown on the host browser');
  }

  function bindHostControlActions() {
    document.querySelectorAll('.host-control-btn[data-target], .overview-host-action[data-target]').forEach((button) => {
      if (button.dataset.controlBound === '1') return;
      button.dataset.controlBound = '1';
      button.addEventListener('click', async () => {
        const target = String(button.dataset.target || '').trim();
        if (!target) return;
        sessionTargetEl.value = target;
        updateConnectStatus();
        setCurrentSection('connections');
        const selectedHost = findKnownHostByTarget(target);
        if ((!selectedHost || !selectedHost.trusted) && !sessionPasswordEl.value.trim()) {
          setFeedback(null, 'success', t('Target host selected. Enter the current host password to continue.'));
          sessionPasswordEl.focus();
          return;
        }
        await createSession({ switchToSessions: false, preserveTarget: true });
      });
    });
  }

  function bindHostTrustActions() {
    document.querySelectorAll('.host-untrust-btn[data-target]').forEach((button) => {
      if (button.dataset.untrustBound === '1') return;
      button.dataset.untrustBound = '1';
      button.addEventListener('click', async () => {
        const target = String(button.dataset.target || '').trim();
        if (!target) return;
        await untrustHost(target);
      });
    });
  }

  async function untrustHost(target) {
    clearFeedback(null);
    const response = await fetch('/api/v1/admin/hosts/untrust', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': state.csrfToken,
      },
      body: JSON.stringify({ target }),
    });
    captureCSRF(response);
    const data = await response.json().catch(() => ({}));
    if (!response.ok) {
      setFeedback(null, 'error', data.error || t('Unable to forget this trusted machine.'));
      return;
    }
    setFeedback(null, 'success', t('Trusted machine forgotten. The host password will be required again.'));
    await loadHosts();
  }

  async function loadSessions() {
    const response = await fetch(sessionsCollectionURL);
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
      translateFragment(sessionsTableBody);
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
    translateFragment(sessionsTableBody);
  }

  function bindSessionActions() {
    document.querySelectorAll('[data-session-open]').forEach((button) => {
      if (button.dataset.sessionBound === '1') return;
      button.dataset.sessionBound = '1';
      button.addEventListener('click', async () => {
        const sessionID = String(button.dataset.sessionOpen || '').trim();
        if (!sessionID) return;
        await openSessionWhenReady(sessionID);
      });
    });
  }

  async function openSessionWhenReady(sessionID) {
    if (!sessionID) return;
    const session = await fetchSession(sessionID);
    if (sessionApprovalReady(session)) {
      clearApprovalWait(sessionID);
      openSessionTab(sessionID);
      return;
    }
    if (sessionApprovalRejected(session)) {
      clearApprovalWait(sessionID);
      setFeedback(null, 'error', t('Host approval was refused or the request expired.'));
      return;
    }
    await beginApprovalWait(sessionID, session.target || '', t('Waiting for host approval.'));
  }

  async function resumeExistingSession(session, target) {
    if (!session || !session.id) return false;
    const sessionID = String(session.id || '').trim();
    if (!sessionID) return false;
    if (sessionApprovalReady(session)) {
      clearApprovalWait(sessionID);
      openSessionTab(sessionID);
      return true;
    }
    if (sessionApprovalRejected(session)) {
      return false;
    }
    const message = session.dispatch_state === 'busy'
      ? t('The host is already handling this request. Keeping the current approval flow open.')
      : t('Reusing the current connection request for this host.');
    await beginApprovalWait(sessionID, target || session.target || '', message);
    return true;
  }

  async function createSession(options = {}) {
    const preserveTarget = options.preserveTarget === true;
    if (state.creatingSession) return;
    clearFeedback(null);
    const target = sessionTargetEl.value.trim();
    if (target && state.approvalWaitSessionID && state.approvalWaitTarget === target) {
      const existingSessionID = state.approvalWaitSessionID;
      await openSessionWhenReady(existingSessionID);
      if (state.approvalWaitSessionID === existingSessionID && state.approvalWaitModalOpen) {
        await repromptSession(existingSessionID, { feedbackTarget: 'approval' });
      }
      return;
    }
    const viewer = state.browserIdentity && state.browserIdentity.public_id
      ? String(state.browserIdentity.public_id).trim()
      : '';
    const selectedHost = findKnownHostByTarget(target);
    const password = selectedHost && selectedHost.trusted ? '' : sessionPasswordEl.value.trim();
    if (!target) {
      setFeedback(null, 'error', t('Host target is required.'));
      return;
    }
    if (!viewer) {
      setFeedback(null, 'error', t('This browser identity is not ready yet.'));
      return;
    }
    const reusableSession = findReusableSession(target);
    if (await resumeExistingSession(reusableSession, target)) {
      return;
    }
    if (!password && (!selectedHost || !selectedHost.trusted)) {
      setFeedback(null, 'error', t('Enter the host password for the first approved connection.'));
      return;
    }
    state.creatingSession = true;
    sessionCreateBtn.disabled = true;
    try {
      const response = await fetch('/api/v1/admin/hosts/connect', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': state.csrfToken,
        },
        body: JSON.stringify({ target, viewer, viewer_label: viewerLabel(), password }),
      });
      captureCSRF(response);
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        setFeedback(null, 'error', data.error || 'Unable to create session.');
        return;
      }
      const sessionID = String(data.id || data.session?.id || '').trim();
      if (!sessionID) {
        console.error('Unexpected session creation response', data);
        setFeedback(null, 'error', data.error || 'Session creation returned no session id.');
        return;
      }
      if (!preserveTarget) {
        sessionTargetEl.value = '';
      }
      sessionPasswordEl.value = '';
      updateConnectStatus();
      await Promise.all([loadHosts(), loadSessions()]);
      const message = data.used_trusted_access
        ? t('Waiting for host approval. Trusted access is ready once the host accepts.')
        : t('Waiting for host approval. This host will be trusted for your account after the first accepted connection.');
      await beginApprovalWait(sessionID, target, message);
    } finally {
      state.creatingSession = false;
      sessionCreateBtn.disabled = false;
    }
  }

  async function repromptSession(sessionID, options = {}) {
    const feedbackTarget = ['session', 'approval'].includes(options.feedbackTarget) ? options.feedbackTarget : 'global';
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(sessionID)}/reprompt`, {
      method: 'POST',
      headers: {
        'X-CSRF-Token': state.csrfToken,
      },
    });
    captureCSRF(response);
    const data = await response.json().catch(() => ({}));
    if (!response.ok) {
      const message = data.error || t('Unable to re-open the approval request right now.');
      if (feedbackTarget === 'session') {
        setSessionFeedback('error', message);
      } else if (feedbackTarget === 'approval') {
        setApprovalWaitFeedback('error', message);
      } else {
        setFeedback(null, 'error', message);
      }
      return false;
    }
    const message = t('Approval request sent to the host again.');
    if (feedbackTarget === 'session') {
      setSessionFeedback('success', message);
    } else if (feedbackTarget === 'approval') {
      setApprovalWaitFeedback('success', message);
    } else {
      setFeedback(null, 'success', message);
    }
    return true;
  }

  function viewerLabel() {
    const localRuntimeHostname = `${state.localHostRuntime && state.localHostRuntime.hostname ? state.localHostRuntime.hostname : ''}`.trim();
    if (localRuntimeHostname) return localRuntimeHostname;
    const browserHostname = `${state.browserIdentity && state.browserIdentity.hostname ? state.browserIdentity.hostname : ''}`.trim();
    if (browserHostname) return browserHostname;
    return '';
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
    clearApprovalWait(state.currentSessionID);
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

  function scheduleDashboardRefresh(loader, delay) {
    let inFlight = false;
    return window.setInterval(async () => {
      if (document.hidden || inFlight) return;
      inFlight = true;
      try {
        await loader();
      } catch (_error) {
      } finally {
        inFlight = false;
      }
    }, delay);
  }

  function stopLocalHostPolling() {
    if (state.localHostPollTimer) {
      window.clearInterval(state.localHostPollTimer);
      state.localHostPollTimer = null;
    }
  }

  function startLocalHostPolling(delay = 2500) {
    stopLocalHostPolling();
    const tick = async () => {
      if (document.hidden) return;
      const available = await discoverLocalHostRuntime();
      if (!available || !state.user) return;
      if (!state.localHostRuntime.connected) {
        const claimed = await claimLocalHostSilently();
        if (claimed) {
          await loadHosts().catch(() => {});
        }
      } else if (state.localHostRuntime.connected) {
        await loadHosts().catch(() => {});
      }
    };
    window.setTimeout(() => {
      void tick();
    }, Math.max(0, delay));
    state.localHostPollTimer = window.setInterval(() => {
      void tick();
    }, 4000);
  }

  sessionCreateBtn.addEventListener('click', createSession);
  if (sessionLogToggleEl) sessionLogToggleEl.addEventListener('change', renderTransportLog);
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
  approvalWaitCloseEl.addEventListener('click', () => closeApprovalWaitModal({ preserveTracking: true }));
  approvalWaitDismissBtn.addEventListener('click', () => closeApprovalWaitModal({ preserveTracking: true }));
  approvalWaitRepromptBtn.addEventListener('click', async () => {
    if (!state.approvalWaitSessionID) return;
    await repromptSession(state.approvalWaitSessionID, { feedbackTarget: 'approval' });
  });
  sessionRefreshBtn.addEventListener('click', async () => {
    try {
      await refreshSessionDetails();
    } catch (_error) {
      setSessionFeedback('error', 'Unable to refresh session details right now.');
    }
  });
  sessionOpenControlBtn.addEventListener('click', () => {
    if (!state.currentSessionID) return;
    openSessionTab(state.currentSessionID);
  });
  if (sessionRepromptBtn) {
    sessionRepromptBtn.addEventListener('click', async () => {
      if (!state.currentSessionID) return;
      await repromptSession(state.currentSessionID, { feedbackTarget: 'session' });
    });
  }
  sessionCloseBtn.addEventListener('click', closeCurrentSession);
  sessionTargetEl.addEventListener('input', updateConnectStatus);
  settingsModalBackdropEl.addEventListener('click', (event) => {
    if (event.target === settingsModalBackdropEl) closeSettingsModal();
  });
  approvalWaitBackdropEl.addEventListener('click', (event) => {
    if (event.target === approvalWaitBackdropEl) closeApprovalWaitModal({ preserveTracking: true });
  });
  sessionModalBackdropEl.addEventListener('click', (event) => {
    if (event.target === sessionModalBackdropEl) closeSessionModal();
  });
  window.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && state.mobileMenuOpen) closeMobileMenu();
    if (event.key === 'Escape' && state.settingsModalOpen) closeSettingsModal();
    if (event.key === 'Escape' && state.approvalWaitModalOpen) closeApprovalWaitModal({ preserveTracking: true });
    if (event.key === 'Escape' && state.sessionModalOpen) closeSessionModal();
  });
  window.addEventListener('unydesk:localechange', updateConnectStatus);
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
      stopLocalHostPolling();
      return;
    }
    startLocalHostPolling(250);
  });

  bindNavigation();
  bindSettingsTabs();
  loadPreferences();
  renderPreferences();

  try {
    await ensureBrowserIdentity();
    const authenticated = await loadSession();
    if (!authenticated) return;
    await discoverLocalHostRuntime();
    if (state.localHostRuntime.available && !state.localHostRuntime.connected) {
      await claimLocalHostSilently();
    }
    const initialSection = (window.location.hash || '').replace(/^#/, '');
    setCurrentSection(initialSection || state.preferences.defaultSection || 'overview');
    await Promise.all([loadHosts(), loadSessions()]);
    scheduleDashboardRefresh(loadHosts, 5000);
    scheduleDashboardRefresh(loadSessions, 7000);
    startLocalHostPolling(500);
  } catch (_error) {
    statusEl.textContent = 'Dashboard unavailable';
  }
})();
