(async () => {
  const state = {
    csrfToken: '',
    sessionID: '',
    sessionPollTimer: null,
    viewerPeerConnection: null,
    viewerDataChannel: null,
    viewerTransportState: 'Idle',
    viewerTransportLog: [],
    remoteCandidatesSeen: [],
    browserIdentity: null,
  };

  const sessionFeedbackWrapEl = document.getElementById('session-feedback-wrap');
  const sessionFeedbackEl = document.getElementById('session-feedback');
  const sessionPageTitleEl = document.getElementById('session-page-title');
  const sessionPageSubtitleEl = document.getElementById('session-page-subtitle');
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

  function captureCSRF(response) {
    const token = response.headers.get('X-CSRF-Token');
    if (token) state.csrfToken = token;
  }

  function setSessionFeedback(kind, message) {
    sessionFeedbackEl.textContent = message;
    sessionFeedbackWrapEl.classList.remove('hidden');
    sessionFeedbackEl.classList.remove('is-success', 'is-error');
    sessionFeedbackEl.classList.add(kind === 'success' ? 'is-success' : 'is-error');
  }

  function clearSessionFeedback() {
    sessionFeedbackEl.textContent = '';
    sessionFeedbackEl.classList.remove('is-success', 'is-error');
    sessionFeedbackWrapEl.classList.add('hidden');
  }

  function formatDateTime(value) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '—';
    return date.toLocaleString();
  }

  function appendTransportLog(message) {
    const stamp = new Date().toLocaleTimeString();
    state.viewerTransportLog.push(`[${stamp}] ${message}`);
    state.viewerTransportLog = state.viewerTransportLog.slice(-50);
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
    sessionPageTitleEl.textContent = `Host Control · ${session.id || '—'}`;
    sessionPageSubtitleEl.textContent = `Target ${session.target || '—'} · Viewer ${session.viewer || '—'}`;
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

  async function fetchSession() {
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(state.sessionID)}`);
    captureCSRF(response);
    if (!response.ok) throw new Error('session fetch failed');
    return response.json();
  }

  async function postSessionOffer(sdp) {
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(state.sessionID)}/offer`, {
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

  async function postSessionCandidate(candidate) {
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(state.sessionID)}/candidates`, {
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

  async function applyRemoteAnswerIfPresent(session) {
    if (!state.viewerPeerConnection || !session.answer_sdp) return;
    const pc = state.viewerPeerConnection;
    if (pc.remoteDescription && pc.remoteDescription.type === 'answer') return;
    await pc.setRemoteDescription({ type: 'answer', sdp: session.answer_sdp });
    state.viewerTransportState = 'Answer applied';
    appendTransportLog('remote answer applied');
  }

  async function applyRemoteCandidatesIfPresent(session) {
    if (!state.viewerPeerConnection || !Array.isArray(session.host_ice_candidates)) return;
    for (const candidate of session.host_ice_candidates) {
      if (!candidate || state.remoteCandidatesSeen.includes(candidate)) continue;
      state.remoteCandidatesSeen.push(candidate);
      try {
        await state.viewerPeerConnection.addIceCandidate({ candidate });
      } catch (_error) {}
    }
  }

  async function refreshSessionDetails(withFeedback = true) {
    const session = await fetchSession();
    await applyRemoteAnswerIfPresent(session);
    await applyRemoteCandidatesIfPresent(session);
    renderSessionDetails(session);
    if (withFeedback) clearSessionFeedback();
  }

  function startSessionPolling() {
    stopSessionPolling();
    state.sessionPollTimer = window.setInterval(() => {
      if (!state.sessionID) return;
      void refreshSessionDetails(false);
    }, 750);
  }

  function stopSessionPolling() {
    if (!state.sessionPollTimer) return;
    window.clearInterval(state.sessionPollTimer);
    state.sessionPollTimer = null;
  }

  function sendViewerControlMessage(payload) {
    if (!state.viewerDataChannel || state.viewerDataChannel.readyState !== 'open') {
      setSessionFeedback('error', 'Viewer data channel is not open yet.');
      return;
    }
    state.viewerDataChannel.send(JSON.stringify(payload));
    appendTransportLog(`sent ${payload.type}`);
  }

  async function startViewerSignaling() {
    if (!window.RTCPeerConnection) {
      setSessionFeedback('error', 'WebRTC is not available in this browser.');
      return;
    }
    resetViewerTransport();
    clearSessionFeedback();
    sessionStartSignalingBtn.disabled = true;
    try {
      const pc = new RTCPeerConnection({
        iceServers: [{ urls: ['stun:stun.l.google.com:19302'] }],
      });
      state.viewerPeerConnection = pc;
      state.viewerTransportState = 'Starting';
      renderSessionDetails(await fetchSession());

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
          session_id: state.sessionID,
        });
        sendViewerControlMessage({
          type: 'ping',
          session_id: state.sessionID,
          sent_at: new Date().toISOString(),
        });
        void refreshSessionDetails(false);
      };
      dc.onclose = () => {
        state.viewerTransportState = 'Data channel closed';
        sessionSendPingBtn.disabled = true;
        appendTransportLog('data channel closed');
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
        if (!event.candidate) return;
        appendTransportLog('viewer ICE candidate gathered');
        void postSessionCandidate(event.candidate.candidate).catch(() => {});
      };

      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      state.viewerTransportState = 'Offer created';
      const updatedSession = await postSessionOffer(offer.sdp || '');
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
        renderSessionDetails(await fetchSession());
      } catch (_error) {}
    }
  }

  async function closeCurrentSession() {
    clearSessionFeedback();
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(state.sessionID)}/close`, {
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
  }

  function bindRemotePad() {
    sessionRemotePadEl.addEventListener('mouseenter', () => sessionRemotePadEl.focus());
    sessionRemotePadEl.addEventListener('mousemove', (event) => {
      const rect = sessionRemotePadEl.getBoundingClientRect();
      const x = Math.max(0, Math.min(1, (event.clientX - rect.left) / Math.max(1, rect.width)));
      const y = Math.max(0, Math.min(1, (event.clientY - rect.top) / Math.max(1, rect.height)));
      sendViewerControlMessage({ type: 'mouse_move', session_id: state.sessionID, x: Number(x.toFixed(4)), y: Number(y.toFixed(4)) });
    });
    sessionRemotePadEl.addEventListener('click', (event) => {
      const rect = sessionRemotePadEl.getBoundingClientRect();
      const x = Math.max(0, Math.min(1, (event.clientX - rect.left) / Math.max(1, rect.width)));
      const y = Math.max(0, Math.min(1, (event.clientY - rect.top) / Math.max(1, rect.height)));
      sendViewerControlMessage({ type: 'mouse_click', session_id: state.sessionID, button: event.button, x: Number(x.toFixed(4)), y: Number(y.toFixed(4)) });
    });
    sessionRemotePadEl.addEventListener('keydown', (event) => {
      if (!event.repeat) {
        sendViewerControlMessage({ type: 'key_down', session_id: state.sessionID, key: event.key, code: event.code });
      }
      event.preventDefault();
    });
    sessionRemotePadEl.addEventListener('keyup', (event) => {
      sendViewerControlMessage({ type: 'key_up', session_id: state.sessionID, key: event.key, code: event.code });
      event.preventDefault();
    });
  }

  async function ensureBrowserIdentity() {
    let token = window.localStorage.getItem('unydesk.browser.token') || '';
    if (!token && window.crypto && window.crypto.getRandomValues) {
      const raw = new Uint8Array(24);
      window.crypto.getRandomValues(raw);
      token = Array.from(raw, (value) => value.toString(16).padStart(2, '0')).join('');
    }
    const response = await fetch(`/api/v1/browser/identity?token=${encodeURIComponent(token)}`);
    captureCSRF(response);
    if (!response.ok) throw new Error('browser identity fetch failed');
    const data = await response.json();
    state.browserIdentity = data;
    if (data.token) window.localStorage.setItem('unydesk.browser.token', data.token);
  }

  function readSessionID() {
    const url = new URL(window.location.href);
    return String(url.searchParams.get('session') || '').trim();
  }

  sessionStartSignalingBtn.addEventListener('click', startViewerSignaling);
  sessionSendPingBtn.addEventListener('click', () => {
    sendViewerControlMessage({ type: 'ping', session_id: state.sessionID, sent_at: new Date().toISOString() });
  });
  sessionRefreshBtn.addEventListener('click', async () => {
    try {
      await refreshSessionDetails();
    } catch (_error) {
      setSessionFeedback('error', 'Unable to refresh session details right now.');
    }
  });
  sessionCloseBtn.addEventListener('click', closeCurrentSession);
  bindRemotePad();

  try {
    state.sessionID = readSessionID();
    if (!state.sessionID) throw new Error('Missing session id');
    await ensureBrowserIdentity();
    await refreshSessionDetails(false);
    startSessionPolling();
  } catch (_error) {
    setSessionFeedback('error', 'Unable to open this control session.');
  }
})();
