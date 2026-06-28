(async () => {
  const SCREEN_WIRE_HEADER_BYTES = 36;
  const SCREEN_WIRE_CODEC_WEBP = 2;
  const SCREEN_WIRE_CODEC_PNG = 3;
  const SCREEN_WIRE_CODEC_RGBA = 4;
  const SCREEN_WIRE_KIND_KEYFRAME = 1;
  const SCREEN_WIRE_KIND_PATCH = 2;
  const SCREEN_CHUNK_MAGIC = "USDT";
  const SCREEN_CHUNK_HEADER_BYTES = 20;
  const SCREEN_CHUNK_MAX_ASSEMBLIES = 6;
  const SCREEN_CHUNK_ASSEMBLY_TIMEOUT_MS = 1000;
  const REMOTE_CURSOR_ARROW = 'url("data:image/svg+xml,%3Csvg xmlns=\'http://www.w3.org/2000/svg\' width=\'20\' height=\'20\' viewBox=\'0 0 20 20\'%3E%3Cpath d=\'M3 2l10 10H8.6l2.5 5.1-1.8.9-2.5-5.1L4 15.6V2z\' fill=\'%23000000\'/%3E%3Cpath d=\'M3.6 3.3v10.85l2.83-2.81h.39l2.34 4.8.78-.39-2.34-4.8v-.54h3.54L3.6 3.3z\' fill=\'%23ffffff\' fill-opacity=\'.18\'/%3E%3C/svg%3E") 3 2, auto';

  const state = {
    csrfToken: "",
    sessionID: "",
    standaloneMode: false,
    standaloneToken: "",
    sessionSocket: null,
    sessionSocketTimer: null,
    sessionSocketConnected: false,
    peerConnection: null,
    peerConnectionState: "idle",
    inputChannel: null,
    auxChannel: null,
    screenChannel: null,
    localOfferPosted: false,
    remoteAnswerApplied: false,
    postedViewerCandidates: new Set(),
    appliedHostCandidates: new Set(),
    currentScreenObjectURL: "",
    screenVideoStream: null,
    screenVideoReady: false,
    screenVideoPlaybackLogged: false,
    hasRenderedScreenFrame: false,
    screenDecodeInFlight: false,
    pendingScreenBuffer: null,
    screenCanvasContext: null,
    pendingPointerMove: null,
    pointerMoveFrameRequested: false,
    expandedView: false,
    lastScreenMetaAt: 0,
    lastScreenError: "",
    activePointerID: null,
    activePointerButton: 0,
    pointerPressed: false,
    lastPointerX: null,
    lastPointerY: null,
    currentScreenRevision: 0,
    viewerTransportLog: [],
    browserIdentity: null,
    sessionEventCount: 0,
    outboundTransfer: null,
    lastDispatchFeedback: "",
    lastFabricCaps: "",
    remoteCursorKind: "default",
    screenChunkAssemblies: new Map(),
    screenLastCompletedFrameID: 0,
    screenLastSeenFrameID: 0,
  };

  const sessionFeedbackWrapEl = document.getElementById("session-feedback-wrap");
  const sessionFeedbackEl = document.getElementById("session-feedback");
  const sessionPageTitleEl = document.getElementById("session-page-title");
  const sessionPageSubtitleEl = document.getElementById("session-page-subtitle");
  const sessionFullscreenBtn = document.getElementById("session-fullscreen-btn");
  const sessionRefreshBtn = document.getElementById("session-refresh-btn");
  const sessionCloseBtn = document.getElementById("session-close-btn");
  const sessionSendPingBtn = document.getElementById("session-send-ping-btn");
  const sessionScreenStageEl = document.getElementById("session-screen-stage");
  const sessionDetailIDEl = document.getElementById("session-detail-id");
  const sessionDetailStatusEl = document.getElementById("session-detail-status");
  const sessionDetailTargetEl = document.getElementById("session-detail-target");
  const sessionDetailViewerEl = document.getElementById("session-detail-viewer");
  const sessionDetailCreatedEl = document.getElementById("session-detail-created");
  const sessionDetailUpdatedEl = document.getElementById("session-detail-updated");
  const sessionDetailRouteEl = document.getElementById("session-detail-route");
  const sessionDetailDispatchEl = document.getElementById("session-detail-dispatch");
  const sessionDetailOfferEl = document.getElementById("session-detail-offer");
  const sessionDetailAnswerEl = document.getElementById("session-detail-answer");
  const sessionDetailViewerTransportEl = document.getElementById("session-detail-viewer-transport");
  const sessionDetailCandidatesEl = document.getElementById("session-detail-candidates");
  const sessionDetailDeliveredAtEl = document.getElementById("session-detail-delivered-at");
  const sessionDetailDeliveriesEl = document.getElementById("session-detail-deliveries");
  const sessionDetailHostAckEl = document.getElementById("session-detail-host-ack");
  const sessionOfferPreviewEl = document.getElementById("session-offer-preview");
  const sessionAnswerPreviewEl = document.getElementById("session-answer-preview");
  const sessionTransportLogEl = document.getElementById("session-transport-log");
  const sessionScreenMetaEl = document.getElementById("session-screen-meta");
  const sessionScreenVideoEl = document.getElementById("session-screen-video");
  const sessionScreenCanvasEl = document.getElementById("session-screen-canvas");
  const sessionScreenImageEl = document.getElementById("session-screen-image");
  const sessionScreenEmptyEl = document.getElementById("session-screen-empty");
  const sessionClipboardTextEl = document.getElementById("session-clipboard-text");
  const sessionClipboardSendBtn = document.getElementById("session-clipboard-send-btn");
  const sessionClipboardReadBtn = document.getElementById("session-clipboard-read-btn");
  const sessionClipboardStatusEl = document.getElementById("session-clipboard-status");
  const sessionFileInputEl = document.getElementById("session-file-input");
  const sessionFileMetaEl = document.getElementById("session-file-meta");
  const sessionFileSendBtn = document.getElementById("session-file-send-btn");
  const sessionFileCancelBtn = document.getElementById("session-file-cancel-btn");
  const sessionFileStatusEl = document.getElementById("session-file-status");

  function captureCSRF(response) {
    const token = response.headers.get("X-CSRF-Token");
    if (token) state.csrfToken = token;
  }

  function setSessionFeedback(kind, message) {
    sessionFeedbackEl.textContent = message;
    sessionFeedbackWrapEl.classList.remove("hidden");
    sessionFeedbackEl.classList.remove("is-success", "is-error");
    sessionFeedbackEl.classList.add(kind === "success" ? "is-success" : "is-error");
  }

  function clearSessionFeedback() {
    sessionFeedbackEl.textContent = "";
    sessionFeedbackEl.classList.remove("is-success", "is-error");
    sessionFeedbackWrapEl.classList.add("hidden");
  }

  function formatDateTime(value) {
    if (!value) return "—";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "—";
    return date.toLocaleString();
  }

  function appendTransportLog(message) {
    const stamp = new Date().toLocaleTimeString();
    state.viewerTransportLog.push(`[${stamp}] ${message}`);
    state.viewerTransportLog = state.viewerTransportLog.slice(-80);
    sessionTransportLogEl.value = state.viewerTransportLog.join("\n");
    sessionTransportLogEl.scrollTop = sessionTransportLogEl.scrollHeight;
  }

  function formatBytes(value) {
    const bytes = Number(value || 0);
    if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }

  function setClipboardStatus(message) {
    sessionClipboardStatusEl.textContent = message;
  }

  function setFileStatus(message) {
    sessionFileStatusEl.textContent = message;
  }

  function cssCursorForRemoteKind(kind) {
    switch (String(kind || "").trim()) {
      case "pointer":
        return "pointer";
      case "text":
        return "text";
      case "wait":
        return "wait";
      case "progress":
        return "progress";
      case "crosshair":
        return "crosshair";
      case "move":
        return "move";
      case "help":
        return "help";
      case "not-allowed":
        return "not-allowed";
      case "ew-resize":
        return "ew-resize";
      case "ns-resize":
        return "ns-resize";
      case "nwse-resize":
        return "nwse-resize";
      case "nesw-resize":
        return "nesw-resize";
      default:
        return REMOTE_CURSOR_ARROW;
    }
  }

  function applyRemoteCursor(kind) {
    const nextKind = String(kind || "default").trim() || "default";
    if (nextKind === state.remoteCursorKind) return;
    state.remoteCursorKind = nextKind;
    sessionScreenStageEl.style.cursor = cssCursorForRemoteKind(nextKind);
  }

  function updateSelectedFileMeta() {
    const file = sessionFileInputEl.files && sessionFileInputEl.files[0];
    if (!file) {
      sessionFileMetaEl.textContent = "No file selected yet.";
      return;
    }
    sessionFileMetaEl.textContent = `${file.name} · ${formatBytes(file.size)}`;
  }

  function randomTransferID() {
    const part = () => Math.random().toString(16).slice(2, 10);
    return `${Date.now().toString(16)}-${part()}-${part()}`;
  }

  function arrayBufferToBase64(buffer) {
    const bytes = new Uint8Array(buffer);
    let binary = "";
    for (let index = 0; index < bytes.length; index += 8192) {
      const chunk = bytes.subarray(index, index + 8192);
      binary += String.fromCharCode.apply(null, Array.from(chunk));
    }
    return window.btoa(binary);
  }

  function sessionSocketURL() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = new URL(`${protocol}//${window.location.host}/api/v1/sessions/${encodeURIComponent(state.sessionID)}/ws`);
    if (state.standaloneToken) {
      url.searchParams.set("standalone_token", state.standaloneToken);
    }
    return url.toString();
  }

  function browserIdentityRequestURL() {
    let token = window.localStorage.getItem("unydesk.browser.token") || "";
    if (!token && window.crypto && window.crypto.getRandomValues) {
      const raw = new Uint8Array(24);
      window.crypto.getRandomValues(raw);
      token = Array.from(raw, (value) => value.toString(16).padStart(2, "0")).join("");
    }
    const params = new URLSearchParams();
    params.set("token", token);
    return `/api/v1/browser/identity?${params.toString()}`;
  }

  function standaloneTokenStorageKey() {
    return `unydesk.standalone.token.${state.sessionID || "pending"}`;
  }

  function sessionRequestHeaders(extra = {}) {
    const headers = { ...extra };
    if (state.standaloneToken) {
      headers["X-UnyDesk-Standalone-Token"] = state.standaloneToken;
    }
    return headers;
  }

  function renderSessionDetails(session) {
    sessionPageTitleEl.textContent = `${state.standaloneMode ? "Standalone Client" : "Host Control"} · ${session.id || "—"}`;
    sessionPageSubtitleEl.textContent = `Target ${session.target || "—"} · Viewer ${session.viewer || "—"}`;
    sessionDetailIDEl.textContent = session.id || "—";
    sessionDetailStatusEl.textContent = session.status || "—";
    sessionDetailTargetEl.textContent = session.target || "—";
    sessionDetailViewerEl.textContent = session.viewer || "—";
    sessionDetailCreatedEl.textContent = formatDateTime(session.created_at);
    sessionDetailUpdatedEl.textContent = formatDateTime(session.updated_at);
    sessionDetailRouteEl.textContent = session.routed_hostname || session.routed_host_public_id || session.routed_host_id || "Direct / unresolved";
    sessionDetailDispatchEl.textContent = session.dispatch_state || "Not queued";
    sessionDetailOfferEl.textContent = state.localOfferPosted ? "Offer posted" : "Preparing";
    sessionDetailAnswerEl.textContent = state.remoteAnswerApplied ? "Answer applied" : "Waiting";
    sessionDetailViewerTransportEl.textContent = describeTransportState();
    sessionDetailCandidatesEl.textContent = `${state.appliedHostCandidates.size} host ICE · ${state.postedViewerCandidates.size} viewer ICE`;
    sessionDetailDeliveredAtEl.textContent = formatDateTime(session.last_dispatch_at);
    sessionDetailDeliveriesEl.textContent = String(session.dispatch_count || 0);
    sessionDetailHostAckEl.textContent = formatDateTime(session.last_host_ack_at);
    sessionOfferPreviewEl.value = "Viewer creates a WebRTC offer, host posts the answer, and ICE candidates trickle through the broker API.";
    sessionAnswerPreviewEl.value = "Screen video uses WebRTC H.264 when available; input, clipboard, and file transfer use WebRTC data channels with broker fallback.";
    sessionCloseBtn.disabled = session.status === "closed";
    renderScreenPreview(session);
  }

  function describeTransportState() {
    const states = [];
    if (state.sessionSocketConnected) states.push("signaling");
    if (state.peerConnection) states.push(`pc:${state.peerConnectionState}`);
    if (state.inputChannel && state.inputChannel.readyState === "open") states.push("input");
    if (state.auxChannel && state.auxChannel.readyState === "open") states.push("aux");
    if (isScreenVideoActive()) states.push("h264");
    if (state.screenChannel && state.screenChannel.readyState === "open") states.push("screen");
    return states.length > 0 ? states.join(" · ") : "Idle";
  }

  function renderFullscreenState() {
    document.body.classList.toggle("account-control-expanded", state.expandedView);
    sessionScreenStageEl.classList.toggle("is-fullscreen", state.expandedView);
    if (state.expandedView) {
      sessionScreenStageEl.focus();
    }
    const icon = sessionFullscreenBtn.querySelector("i");
    const label = sessionFullscreenBtn.querySelector("span");
    if (icon) icon.className = state.expandedView ? "fa-solid fa-compress" : "fa-solid fa-expand";
    if (label) label.textContent = state.expandedView ? "Exit full page" : "Full page";
  }

  function isScreenVideoActive() {
    return Boolean(state.screenVideoReady && sessionScreenVideoEl && sessionScreenVideoEl.srcObject);
  }

  function showScreenVideo(prefix = "H.264 video") {
    if (!sessionScreenVideoEl || !sessionScreenVideoEl.srcObject) return;
    state.screenVideoReady = true;
    if (!state.screenVideoPlaybackLogged) {
      state.screenVideoPlaybackLogged = true;
      appendTransportLog("H.264 video playing");
    }
    sessionScreenVideoEl.classList.remove("hidden");
    sessionScreenCanvasEl.classList.add("hidden");
    sessionScreenImageEl.classList.add("hidden");
    sessionScreenEmptyEl.classList.add("hidden");
    updateScreenMeta(prefix, sessionScreenVideoEl.videoWidth || 0, sessionScreenVideoEl.videoHeight || 0);
  }

  function resetScreenVideo() {
    state.screenVideoReady = false;
    state.screenVideoPlaybackLogged = false;
    state.screenVideoStream = null;
    if (!sessionScreenVideoEl) return;
    try {
      sessionScreenVideoEl.pause();
    } catch (_error) {}
    sessionScreenVideoEl.srcObject = null;
    sessionScreenVideoEl.classList.add("hidden");
  }

  function renderScreenPreview(session) {
    if (isScreenVideoActive()) {
      sessionScreenVideoEl.classList.remove("hidden");
      sessionScreenCanvasEl.classList.add("hidden");
      sessionScreenImageEl.classList.add("hidden");
      sessionScreenEmptyEl.classList.add("hidden");
      return;
    }
    const hasImage = Boolean(state.hasRenderedScreenFrame || state.currentScreenObjectURL || session.screen_data_url);
    if (hasImage) {
      if (state.hasRenderedScreenFrame) {
        sessionScreenVideoEl.classList.add("hidden");
        sessionScreenCanvasEl.classList.remove("hidden");
        sessionScreenImageEl.classList.add("hidden");
      } else if (!state.currentScreenObjectURL && session.screen_data_url && sessionScreenImageEl.src !== session.screen_data_url) {
        sessionScreenImageEl.src = session.screen_data_url;
        sessionScreenImageEl.classList.remove("hidden");
        sessionScreenVideoEl.classList.add("hidden");
        sessionScreenCanvasEl.classList.add("hidden");
      }
      sessionScreenEmptyEl.classList.add("hidden");
      return;
    }
    sessionScreenVideoEl.classList.add("hidden");
    sessionScreenCanvasEl.classList.add("hidden");
    sessionScreenImageEl.removeAttribute("src");
    sessionScreenImageEl.classList.add("hidden");
    sessionScreenEmptyEl.classList.remove("hidden");
    sessionScreenEmptyEl.textContent = state.screenChannel && state.screenChannel.readyState === "open"
      ? (session.screen_capture_error || "Waiting for the first host frame.")
      : "Waiting for the peer screen stream.";
    sessionScreenMetaEl.textContent = state.screenChannel && state.screenChannel.readyState === "open"
      ? "Peer screen channel connected."
      : "Connecting peer screen channel...";
  }

  async function fetchSession() {
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(state.sessionID)}`, {
      headers: sessionRequestHeaders(),
    });
    captureCSRF(response);
    if (!response.ok) throw new Error("session fetch failed");
    return response.json();
  }

  async function postSessionJSON(path, payload) {
    const response = await fetch(path, {
      method: "POST",
      headers: sessionRequestHeaders({
        "Content-Type": "application/json",
        "X-CSRF-Token": state.csrfToken,
      }),
      body: JSON.stringify(payload),
    });
    captureCSRF(response);
    if (!response.ok) {
      const data = await response.json().catch(() => ({ error: "request failed" }));
      throw new Error(data.error || "request failed");
    }
    return response.json().catch(() => null);
  }

  function maybeRenderDispatchFeedback(session) {
    const dispatchState = String(session.dispatch_state || "");
    if (!dispatchState || dispatchState === state.lastDispatchFeedback) return;
    if (dispatchState === "rejected") {
      state.lastDispatchFeedback = dispatchState;
      setSessionFeedback("error", "The host denied this remote access request.");
      appendTransportLog("host denied the remote access request");
      return;
    }
    if (dispatchState === "busy") {
      state.lastDispatchFeedback = dispatchState;
      setSessionFeedback("error", "The host is not accepting remote access right now.");
      appendTransportLog("host is not accepting remote access right now");
      return;
    }
    if (dispatchState === "accepted") {
      state.lastDispatchFeedback = dispatchState;
      clearSessionFeedback();
    }
  }

  async function handleSessionUpdate(session, withFeedback = false) {
    renderSessionDetails(session);
    maybeRenderDispatchFeedback(session);
    await ensureRealtimeSession(session);
    await applyRemoteSignaling(session);
    if (withFeedback) clearSessionFeedback();
  }

  function scheduleSessionSocketReconnect() {
    if (state.sessionSocketTimer || !state.sessionID) return;
    state.sessionSocketTimer = window.setTimeout(() => {
      state.sessionSocketTimer = null;
      openSessionSocket();
    }, 1000);
  }

  function closeSessionSocket() {
    if (state.sessionSocketTimer) {
      window.clearTimeout(state.sessionSocketTimer);
      state.sessionSocketTimer = null;
    }
    if (!state.sessionSocket) return;
    try {
      state.sessionSocket.close();
    } catch (_error) {}
    state.sessionSocket = null;
    state.sessionSocketConnected = false;
  }

  function openSessionSocket() {
    closeSessionSocket();
    if (!state.sessionID) return;
    const socket = new WebSocket(sessionSocketURL());
    state.sessionSocket = socket;
    socket.addEventListener("open", () => {
      state.sessionSocketConnected = true;
      appendTransportLog("session signaling websocket connected");
      setClipboardStatus("Waiting for WebRTC data channels...");
      setFileStatus("Waiting for WebRTC data channels...");
    });
    socket.addEventListener("message", (event) => {
      try {
        const payload = JSON.parse(String(event.data || "{}"));
        if (payload.type === "session" && payload.session) {
          void handleSessionUpdate(payload.session);
          return;
        }
        if (payload.type === "error" && payload.error) {
          appendTransportLog(`server error: ${payload.error}`);
        }
      } catch (_error) {}
    });
    socket.addEventListener("close", () => {
      state.sessionSocketConnected = false;
      appendTransportLog("session signaling websocket disconnected");
      if (state.sessionSocket === socket) state.sessionSocket = null;
      scheduleSessionSocketReconnect();
    });
    socket.addEventListener("error", () => {
      state.sessionSocketConnected = false;
      try {
        socket.close();
      } catch (_error) {}
    });
  }

  async function ensureRealtimeSession(session) {
    if (state.peerConnection || !session || session.status === "closed") {
      if (session && session.status === "closed") closeRealtimeSession();
      return;
    }
    const peerConnection = new RTCPeerConnection({ iceServers: [] });
    state.peerConnection = peerConnection;
    state.peerConnectionState = peerConnection.connectionState || "new";
    resetScreenChunkAssemblies();
    appendTransportLog("creating WebRTC peer connection");

    peerConnection.addEventListener("connectionstatechange", () => {
      state.peerConnectionState = peerConnection.connectionState || "unknown";
      appendTransportLog(`peer connection state: ${state.peerConnectionState}`);
      renderSessionDetails(session);
      if (state.peerConnectionState === "connected") {
        setClipboardStatus("WebRTC connected. Clipboard sync is ready.");
        if (!state.outboundTransfer) {
          setFileStatus("WebRTC connected. Files are delivered to the host Downloads folder when available.");
        }
      }
      if (["failed", "closed", "disconnected"].includes(state.peerConnectionState)) {
        applyRemoteCursor("default");
      }
    });

    peerConnection.addEventListener("iceconnectionstatechange", () => {
      appendTransportLog(`ICE connection state: ${peerConnection.iceConnectionState || "unknown"}`);
    });

    peerConnection.addEventListener("icegatheringstatechange", () => {
      appendTransportLog(`ICE gathering state: ${peerConnection.iceGatheringState || "unknown"}`);
    });

    peerConnection.addEventListener("icecandidate", (event) => {
      if (!event.candidate) {
        appendTransportLog("viewer ICE gathering complete");
        return;
      }
      appendTransportLog(`viewer ICE candidate: ${event.candidate.type || "unknown"} ${event.candidate.protocol || ""}`);
      const raw = JSON.stringify(event.candidate.toJSON());
      if (state.postedViewerCandidates.has(raw)) return;
      state.postedViewerCandidates.add(raw);
      void postSessionJSON(`/api/v1/sessions/${encodeURIComponent(state.sessionID)}/candidates`, {
        candidate: raw,
        source: "viewer",
      }).then(() => {
        renderSessionDetails(session);
      }).catch((error) => {
        appendTransportLog(`viewer ICE candidate post failed: ${error.message}`);
      });
    });

    peerConnection.addEventListener("track", (event) => {
      if (!event.track || event.track.kind !== "video") {
        appendTransportLog(`received unexpected media track ${event.track ? event.track.kind : "unknown"}`);
        return;
      }
      const stream = event.streams && event.streams[0] ? event.streams[0] : new MediaStream([event.track]);
      state.screenVideoStream = stream;
      state.screenVideoReady = false;
      state.screenVideoPlaybackLogged = false;
      sessionScreenVideoEl.srcObject = stream;
      sessionScreenMetaEl.textContent = "H.264 screen track negotiated; waiting for first video frame.";
      appendTransportLog(`received H.264 video track ${event.track.id || "screen"}`);
      event.track.addEventListener("unmute", () => appendTransportLog("H.264 video track unmuted"));
      event.track.addEventListener("mute", () => appendTransportLog("H.264 video track muted"));
      event.track.addEventListener("ended", () => {
        appendTransportLog("H.264 video track ended");
        resetScreenVideo();
      });
      void sessionScreenVideoEl.play().catch((error) => {
        appendTransportLog(`H.264 video autoplay waiting: ${error.message}`);
      });
      renderSessionDetails(session);
    });

    peerConnection.addEventListener("datachannel", (event) => {
      const channel = event.channel;
      if (channel.label !== "screen") {
        appendTransportLog(`received unexpected data channel ${channel.label}`);
        return;
      }
      state.screenChannel = channel;
      channel.binaryType = "arraybuffer";
      channel.addEventListener("open", () => {
        const mode = channel.ordered ? "reliable ordered" : "realtime unordered";
        appendTransportLog(`screen data channel open (${mode})`);
        sessionScreenMetaEl.textContent = "Peer screen channel connected.";
        renderSessionDetails(session);
      });
      channel.addEventListener("close", () => {
        appendTransportLog("screen data channel closed");
        renderSessionDetails(session);
      });
      channel.addEventListener("message", (messageEvent) => {
        if (!(messageEvent.data instanceof ArrayBuffer)) return;
        const reassembled = absorbScreenChunk(messageEvent.data);
        if (!reassembled) return;
        queueScreenBuffer(reassembled);
      });
    });

    peerConnection.addTransceiver("video", { direction: "recvonly" });
    const inputChannel = peerConnection.createDataChannel("input");
    const auxChannel = peerConnection.createDataChannel("aux");
    state.inputChannel = inputChannel;
    state.auxChannel = auxChannel;

    for (const [label, channel] of [["input", inputChannel], ["aux", auxChannel]]) {
      channel.addEventListener("open", () => {
        appendTransportLog(`${label} data channel open`);
        renderSessionDetails(session);
      });
      channel.addEventListener("close", () => {
        appendTransportLog(`${label} data channel closed`);
        renderSessionDetails(session);
      });
    }

    auxChannel.addEventListener("message", (event) => {
      try {
        const payload = JSON.parse(String(event.data || "{}"));
        handleSessionEvent(payload);
      } catch (_error) {}
    });

    const offer = await peerConnection.createOffer();
    await peerConnection.setLocalDescription(offer);
    await postSessionJSON(`/api/v1/sessions/${encodeURIComponent(state.sessionID)}/offer`, {
      sdp: JSON.stringify(peerConnection.localDescription),
    });
    state.localOfferPosted = true;
    appendTransportLog("posted WebRTC offer");
    renderSessionDetails(session);
  }

  async function applyRemoteSignaling(session) {
    const pc = state.peerConnection;
    if (!pc || !session) return;
    if (!state.remoteAnswerApplied && session.answer_sdp) {
      const answer = JSON.parse(session.answer_sdp);
      await pc.setRemoteDescription(answer);
      state.remoteAnswerApplied = true;
      appendTransportLog(`applied remote answer (${countSDPCandidates(answer.sdp)} candidates)`);
      window.setTimeout(() => {
        void fetchSession()
          .then((nextSession) => applyRemoteSignaling(nextSession))
          .catch((error) => appendTransportLog(`post-answer signaling refresh failed: ${error.message}`));
      }, 500);
      renderSessionDetails(session);
    }
    if (!state.remoteAnswerApplied) return;
    for (const rawCandidate of session.host_ice_candidates || []) {
      if (!rawCandidate || state.appliedHostCandidates.has(rawCandidate)) continue;
      state.appliedHostCandidates.add(rawCandidate);
      try {
        const candidate = JSON.parse(rawCandidate);
        await pc.addIceCandidate(candidate);
        appendTransportLog(`applied host ICE candidate: ${describeICECandidate(candidate)}`);
      } catch (error) {
        appendTransportLog(`host ICE candidate add failed: ${error.message}`);
      }
    }
    renderSessionDetails(session);
  }

  function closeRealtimeSession() {
    if (state.peerConnection) {
      try {
        state.peerConnection.close();
      } catch (_error) {}
    }
    state.peerConnection = null;
    state.peerConnectionState = "closed";
    state.inputChannel = null;
    state.auxChannel = null;
    state.screenChannel = null;
    state.localOfferPosted = false;
    state.remoteAnswerApplied = false;
    state.postedViewerCandidates = new Set();
    state.appliedHostCandidates = new Set();
    resetScreenVideo();
    resetScreenChunkAssemblies();
  }

  function resetScreenChunkAssemblies() {
    state.screenChunkAssemblies = new Map();
    state.screenLastCompletedFrameID = 0;
    state.screenLastSeenFrameID = 0;
  }

  function handleSessionEvent(eventPayload) {
    if (!eventPayload || typeof eventPayload !== "object") return;
    state.sessionEventCount += 1;
    switch (String(eventPayload.type || "")) {
      case "pong":
        appendTransportLog("received pong");
        return;
      case "cursor":
        applyRemoteCursor(eventPayload.cursor);
        return;
      case "screen_error":
        state.lastScreenError = String(eventPayload.error || "");
        appendTransportLog(`screen error: ${state.lastScreenError}`);
        if (/h\.?264/i.test(state.lastScreenError)) {
          resetScreenVideo();
        }
        sessionScreenMetaEl.textContent = `Peer screen channel · ${state.lastScreenError}`;
        return;
      case "screen_status": {
        const transport = String(eventPayload.transport || "");
        if (transport !== "h264") return;
        const profile = String(eventPayload.profile || "adaptive");
        const fps = Number(eventPayload.fps || 0);
        const crf = Number(eventPayload.crf || 0);
        const maxEdge = Number(eventPayload.max_edge || 0);
        const ageMs = Number(eventPayload.age_ms || 0);
        const maxrate = Number(eventPayload.maxrate || 0);
        const lossPct = Number(eventPayload.loss_pct || 0);
        const jitterMs = Number(eventPayload.jitter_ms || 0);
        const nacks = Number(eventPayload.nacks || 0);
        const plis = Number(eventPayload.plis || 0);
        const captureProvider = String(eventPayload.capture_provider || "");
        const captureStatus = String(eventPayload.capture_status || "");
        const captureCaps = String(eventPayload.capture_caps || "");
        const encoderProvider = String(eventPayload.encoder_provider || "");
        const encoderStatus = String(eventPayload.encoder_status || "");
        const encoderCaps = String(eventPayload.encoder_caps || "");
        const transportPrimary = String(eventPayload.transport_primary || "");
        const transportStatus = String(eventPayload.transport_status || "");
        const transportCaps = String(eventPayload.transport_caps || "");
        const action = String(eventPayload.action || "profile");
        const statusSuffix = (status) => (status && status !== "active" ? `/${status}` : "");
        const networkDetails = `${lossPct ? ` · loss ${lossPct.toFixed(1)}%` : ""}${jitterMs ? ` · jitter ${Math.round(jitterMs)}ms` : ""}${nacks ? ` · NACK ${nacks}` : ""}${plis ? ` · PLI ${plis}` : ""}`;
        const fabricDetails = `${transportPrimary ? ` · ${transportPrimary}${statusSuffix(transportStatus)}` : ""}${encoderProvider ? ` · ${encoderProvider}${statusSuffix(encoderStatus)}` : ""}${captureProvider ? ` · ${captureProvider}${statusSuffix(captureStatus)}` : ""}`;
        const details = `${profile}${fps ? ` · ${fps}fps` : ""}${crf ? ` · CRF ${crf}` : ""}${maxEdge ? ` · edge ${maxEdge}` : ""}${maxrate ? ` · ${maxrate}kbps` : ""}${ageMs ? ` · ${ageMs}ms` : ""}${networkDetails}${fabricDetails}`;
        const fabricCaps = [captureCaps, encoderCaps, transportCaps].filter(Boolean).join(" | ");
        if (fabricCaps && fabricCaps !== state.lastFabricCaps) {
          state.lastFabricCaps = fabricCaps;
          appendTransportLog(`Remote fabric caps: ${fabricCaps}`);
        }
        appendTransportLog(`H.264 ${action}: ${details}`);
        sessionScreenMetaEl.textContent = `H.264 adaptive profile · ${details}`;
        return;
      }
      case "clipboard":
        if (eventPayload.status === "ok" && eventPayload.action === "get") {
          sessionClipboardTextEl.value = String(eventPayload.text || "");
          setClipboardStatus(`Host clipboard loaded · ${sessionClipboardTextEl.value.length} characters.`);
          appendTransportLog("received clipboard contents");
          return;
        }
        if (eventPayload.status === "ok" && eventPayload.action === "set") {
          setClipboardStatus(`Clipboard pushed to host · ${eventPayload.length || 0} characters.`);
          appendTransportLog("host clipboard updated");
          return;
        }
        setClipboardStatus(String(eventPayload.error || "Clipboard request failed."));
        appendTransportLog(`clipboard error: ${eventPayload.error || "request failed"}`);
        return;
      case "file_transfer": {
        const stage = String(eventPayload.stage || "progress");
        const transferID = String(eventPayload.transfer_id || "");
        const name = String(eventPayload.name || "file");
        if (stage === "started") {
          setFileStatus(`Host is preparing ${name} in ${eventPayload.destination || "its transfer folder"}.`);
          appendTransportLog(`host accepted file transfer ${name}`);
          return;
        }
        if (stage === "progress") {
          setFileStatus(`Host received ${formatBytes(eventPayload.bytes_received)} of ${formatBytes(eventPayload.total_bytes)} for ${name}.`);
          return;
        }
        if (stage === "completed") {
          if (state.outboundTransfer && state.outboundTransfer.id === transferID) {
            state.outboundTransfer = null;
            sessionFileCancelBtn.disabled = true;
          }
          setFileStatus(`Saved ${name} to ${eventPayload.path || "the host transfer folder"}.`);
          appendTransportLog(`file transfer completed: ${name}`);
          return;
        }
        if (stage === "error") {
          if (state.outboundTransfer && state.outboundTransfer.id === transferID) {
            state.outboundTransfer = null;
            sessionFileCancelBtn.disabled = true;
          }
          setFileStatus(`Transfer failed for ${name || "the selected file"} · ${eventPayload.error || "unknown error"}`);
          appendTransportLog(`file transfer error: ${eventPayload.error || "unknown error"}`);
          return;
        }
        return;
      }
      case "input_error":
        appendTransportLog(`${eventPayload.control || "input"} failed: ${eventPayload.error || "unknown error"}`);
        return;
      case "unsupported":
        appendTransportLog(`host does not support ${eventPayload.control || "this action"} yet`);
        return;
      default:
        appendTransportLog(`host event: ${String(eventPayload.type || "unknown")}`);
    }
  }

  function updateScreenImageFromBlob(blob) {
    const objectURL = URL.createObjectURL(blob);
    if (state.currentScreenObjectURL) {
      URL.revokeObjectURL(state.currentScreenObjectURL);
    }
    state.currentScreenObjectURL = objectURL;
    state.hasRenderedScreenFrame = false;
    sessionScreenImageEl.src = objectURL;
    sessionScreenVideoEl.classList.add("hidden");
    sessionScreenCanvasEl.classList.add("hidden");
    sessionScreenImageEl.classList.remove("hidden");
    sessionScreenEmptyEl.classList.add("hidden");
    updateScreenMeta("Peer frame");
  }

  function updateScreenMeta(prefix, width = 0, height = 0) {
    const now = Date.now();
    if (now - state.lastScreenMetaAt < 180) return;
    state.lastScreenMetaAt = now;
    const stamp = new Date(now).toLocaleTimeString();
    const sizePrefix = width > 0 && height > 0 ? `${width}x${height} · ` : "";
    sessionScreenMetaEl.textContent = `${sizePrefix}${prefix} · Updated ${stamp}`;
  }

  function sniffImageMime(bytes) {
    if (!bytes || bytes.length < 12) return "";
    if (bytes[0] === 0xff && bytes[1] === 0xd8) return "image/jpeg";
    if (
      bytes[0] === 0x89 && bytes[1] === 0x50 && bytes[2] === 0x4e && bytes[3] === 0x47 &&
      bytes[4] === 0x0d && bytes[5] === 0x0a && bytes[6] === 0x1a && bytes[7] === 0x0a
    ) {
      return "image/png";
    }
    if (
      bytes[0] === 0x52 && bytes[1] === 0x49 && bytes[2] === 0x46 && bytes[3] === 0x46 &&
      bytes[8] === 0x57 && bytes[9] === 0x45 && bytes[10] === 0x42 && bytes[11] === 0x50
    ) {
      return "image/webp";
    }
    return "";
  }

  function decodeScreenWirePacket(buffer) {
    if (!(buffer instanceof ArrayBuffer)) return null;
    const bytes = new Uint8Array(buffer);
    if (
      bytes.length >= SCREEN_WIRE_HEADER_BYTES &&
      bytes[0] === 0x55 && bytes[1] === 0x53 && bytes[2] === 0x43 && bytes[3] === 0x52
    ) {
      const view = new DataView(buffer);
      const codec = view.getUint8(5);
      const kind = view.getUint8(6);
      const x = view.getUint32(8);
      const y = view.getUint32(12);
      const width = view.getUint32(16);
      const height = view.getUint32(20);
      const fullWidth = view.getUint32(24);
      const fullHeight = view.getUint32(28);
      const payloadLength = view.getUint32(32);
      if (SCREEN_WIRE_HEADER_BYTES + payloadLength > bytes.length) return null;
      return {
        codec,
        mimeType: codec === SCREEN_WIRE_CODEC_WEBP ? "image/webp" : (codec === SCREEN_WIRE_CODEC_PNG ? "image/png" : (codec === SCREEN_WIRE_CODEC_RGBA ? "application/x-unydesk-rgba" : "image/jpeg")),
        kind,
        x,
        y,
        width,
        height,
        fullWidth,
        fullHeight,
        payload: bytes.slice(SCREEN_WIRE_HEADER_BYTES, SCREEN_WIRE_HEADER_BYTES + payloadLength),
      };
    }
    const mimeType = sniffImageMime(bytes);
    if (!mimeType) return null;
    return {
      codec: 0,
      mimeType,
      kind: SCREEN_WIRE_KIND_KEYFRAME,
      x: 0,
      y: 0,
      width: 0,
      height: 0,
      fullWidth: 0,
      fullHeight: 0,
      payload: bytes,
    };
  }

  function drawScreenBitmap(bitmap, packet) {
    const width = packet.width || bitmap.width || sessionScreenCanvasEl.width || 1;
    const height = packet.height || bitmap.height || sessionScreenCanvasEl.height || 1;
    const fullWidth = packet.fullWidth || bitmap.width || width;
    const fullHeight = packet.fullHeight || bitmap.height || height;
    if (packet.kind === SCREEN_WIRE_KIND_PATCH && !state.hasRenderedScreenFrame) {
      return false;
    }
    if (sessionScreenCanvasEl.width !== fullWidth || sessionScreenCanvasEl.height !== fullHeight) {
      sessionScreenCanvasEl.width = fullWidth;
      sessionScreenCanvasEl.height = fullHeight;
      state.hasRenderedScreenFrame = false;
    }
    if (!state.screenCanvasContext) {
      state.screenCanvasContext = sessionScreenCanvasEl.getContext("2d", { alpha: false, desynchronized: true });
    }
    if (!state.screenCanvasContext) return false;
    state.screenCanvasContext.imageSmoothingEnabled = true;
    state.screenCanvasContext.imageSmoothingQuality = "high";
    if (packet.kind === SCREEN_WIRE_KIND_KEYFRAME || !state.hasRenderedScreenFrame) {
      state.screenCanvasContext.clearRect(0, 0, fullWidth, fullHeight);
      state.screenCanvasContext.drawImage(bitmap, 0, 0, width, height);
    } else {
      state.screenCanvasContext.drawImage(bitmap, packet.x, packet.y, width, height);
    }
    state.hasRenderedScreenFrame = true;
    sessionScreenVideoEl.classList.add("hidden");
    sessionScreenCanvasEl.classList.remove("hidden");
    sessionScreenImageEl.classList.add("hidden");
    sessionScreenEmptyEl.classList.add("hidden");
    updateScreenMeta("Peer frame", fullWidth, fullHeight);
    return true;
  }

  function drawRawScreenPacket(packet) {
    const width = packet.width || 0;
    const height = packet.height || 0;
    const fullWidth = packet.fullWidth || width;
    const fullHeight = packet.fullHeight || height;
    if (width <= 0 || height <= 0 || fullWidth <= 0 || fullHeight <= 0) return false;
    if (packet.payload.length < width * height * 4) return false;
    if (packet.kind === SCREEN_WIRE_KIND_PATCH && !state.hasRenderedScreenFrame) {
      return false;
    }
    if (sessionScreenCanvasEl.width !== fullWidth || sessionScreenCanvasEl.height !== fullHeight) {
      sessionScreenCanvasEl.width = fullWidth;
      sessionScreenCanvasEl.height = fullHeight;
      state.hasRenderedScreenFrame = false;
    }
    if (!state.screenCanvasContext) {
      state.screenCanvasContext = sessionScreenCanvasEl.getContext("2d", { alpha: false, desynchronized: true });
    }
    if (!state.screenCanvasContext) return false;
    const pixels = new Uint8ClampedArray(packet.payload.buffer, packet.payload.byteOffset, width * height * 4);
    const imageData = new ImageData(pixels, width, height);
    if (packet.kind === SCREEN_WIRE_KIND_KEYFRAME || !state.hasRenderedScreenFrame) {
      state.screenCanvasContext.clearRect(0, 0, fullWidth, fullHeight);
      state.screenCanvasContext.putImageData(imageData, 0, 0);
    } else {
      state.screenCanvasContext.putImageData(imageData, packet.x, packet.y);
    }
    state.hasRenderedScreenFrame = true;
    sessionScreenVideoEl.classList.add("hidden");
    sessionScreenCanvasEl.classList.remove("hidden");
    sessionScreenImageEl.classList.add("hidden");
    sessionScreenEmptyEl.classList.add("hidden");
    updateScreenMeta("Peer frame", fullWidth, fullHeight);
    return true;
  }

  async function decodeAndRenderScreenBuffer(buffer) {
    const packet = decodeScreenWirePacket(buffer);
    if (!packet) return;
    if (packet.codec === SCREEN_WIRE_CODEC_RGBA) {
      drawRawScreenPacket(packet);
      return;
    }
    const blob = new Blob([packet.payload], { type: packet.mimeType });
    if (window.createImageBitmap) {
      const bitmap = await window.createImageBitmap(blob);
      try {
        if (!drawScreenBitmap(bitmap, packet)) updateScreenImageFromBlob(blob);
      } finally {
        if (bitmap && typeof bitmap.close === "function") bitmap.close();
      }
      return;
    }
    updateScreenImageFromBlob(blob);
  }

  function queueScreenBuffer(buffer) {
    if (isScreenVideoActive()) return;
    if (state.screenDecodeInFlight) {
      state.pendingScreenBuffer = buffer;
      return;
    }
    state.screenDecodeInFlight = true;
    void decodeAndRenderScreenBuffer(buffer)
      .catch(() => {
        const packet = decodeScreenWirePacket(buffer);
        if (packet) updateScreenImageFromBlob(new Blob([packet.payload], { type: packet.mimeType }));
      })
      .finally(() => {
        state.screenDecodeInFlight = false;
        if (state.pendingScreenBuffer) {
          const nextBuffer = state.pendingScreenBuffer;
          state.pendingScreenBuffer = null;
          queueScreenBuffer(nextBuffer);
        }
      });
  }

  function absorbScreenChunk(buffer) {
    const bytes = new Uint8Array(buffer);
    if (bytes.length < SCREEN_CHUNK_HEADER_BYTES) return null;
    const magic = String.fromCharCode(bytes[0], bytes[1], bytes[2], bytes[3]);
    if (magic !== SCREEN_CHUNK_MAGIC) return buffer;
    const view = new DataView(buffer);
    const frameID = view.getUint32(8);
    const chunkIndex = view.getUint32(12);
    const totalChunks = view.getUint32(16);
    if (!Number.isFinite(frameID) || frameID <= state.screenLastCompletedFrameID) return null;
    if (totalChunks <= 0 || totalChunks > 4096 || chunkIndex >= totalChunks) return null;

    const now = window.performance ? window.performance.now() : Date.now();
    if (frameID > state.screenLastSeenFrameID) state.screenLastSeenFrameID = frameID;

    let assembly = state.screenChunkAssemblies.get(frameID);
    if (!assembly) {
      assembly = {
        totalChunks,
        parts: new Array(totalChunks),
        received: 0,
        totalSize: 0,
        updatedAt: now,
      };
      state.screenChunkAssemblies.set(frameID, assembly);
    }
    if (assembly.totalChunks !== totalChunks) {
      state.screenChunkAssemblies.delete(frameID);
      return null;
    }

    if (!(assembly.parts[chunkIndex] instanceof Uint8Array)) {
      const part = bytes.slice(SCREEN_CHUNK_HEADER_BYTES);
      assembly.parts[chunkIndex] = part;
      assembly.received += 1;
      assembly.totalSize += part.length;
      assembly.updatedAt = now;
    }
    pruneScreenChunkAssemblies(now);
    if (assembly.received < assembly.totalChunks) return null;

    const totalSize = assembly.totalSize;
    const joined = new Uint8Array(totalSize);
    let offset = 0;
    for (const part of assembly.parts) {
      if (!(part instanceof Uint8Array)) return null;
      joined.set(part, offset);
      offset += part.length;
    }
    state.screenLastCompletedFrameID = Math.max(state.screenLastCompletedFrameID, frameID);
    for (const id of Array.from(state.screenChunkAssemblies.keys())) {
      if (id <= frameID) state.screenChunkAssemblies.delete(id);
    }
    return joined.buffer;
  }

  function pruneScreenChunkAssemblies(now) {
    const staleBefore = now - SCREEN_CHUNK_ASSEMBLY_TIMEOUT_MS;
    for (const [frameID, assembly] of state.screenChunkAssemblies) {
      if (assembly.updatedAt < staleBefore || frameID + SCREEN_CHUNK_MAX_ASSEMBLIES < state.screenLastSeenFrameID) {
        state.screenChunkAssemblies.delete(frameID);
      }
    }
    while (state.screenChunkAssemblies.size > SCREEN_CHUNK_MAX_ASSEMBLIES) {
      const oldestFrameID = Math.min(...state.screenChunkAssemblies.keys());
      state.screenChunkAssemblies.delete(oldestFrameID);
    }
  }

  function getChannel(label) {
    switch (label) {
      case "input":
        return state.inputChannel;
      case "aux":
      default:
        return state.auxChannel;
    }
  }

  function sendControlViaSessionSocket(payload, channelLabel, options) {
    if (!state.sessionSocket || state.sessionSocket.readyState !== WebSocket.OPEN) return false;
    state.sessionSocket.send(JSON.stringify({
      type: "control",
      payload,
    }));
    if (options.log !== false) appendTransportLog(`sent ${payload.type} via signaling fallback (${channelLabel})`);
    return true;
  }

  function sendViewerControlMessage(payload, options = {}) {
    const channelLabel = options.channel || "aux";
    const channel = getChannel(channelLabel);
    if (!channel || channel.readyState !== "open") {
      if (sendControlViaSessionSocket(payload, channelLabel, options)) return true;
      setSessionFeedback("error", "Control channel is not connected yet.");
      return false;
    }
    channel.send(JSON.stringify(payload));
    if (options.log !== false) appendTransportLog(`sent ${payload.type} on ${channelLabel}`);
    return true;
  }

  function countSDPCandidates(sdp) {
    return String(sdp || "").split(/\r?\n/).filter((line) => line.startsWith("a=candidate:")).length;
  }

  function describeICECandidate(candidate) {
    const raw = String((candidate && candidate.candidate) || "");
    const type = raw.match(/\btyp\s+(\S+)/i);
    const protocol = raw.match(/\s(udp|tcp)\s/i);
    return `${type ? type[1] : "unknown"} ${protocol ? protocol[1].toLowerCase() : ""}`.trim();
  }

  function queuePointerMove(x, y) {
    state.pendingPointerMove = { x, y };
    if (state.pointerMoveFrameRequested) return;
    state.pointerMoveFrameRequested = true;
    window.requestAnimationFrame(() => {
      state.pointerMoveFrameRequested = false;
      const move = state.pendingPointerMove;
      state.pendingPointerMove = null;
      if (!move) return;
      sendViewerControlMessage({ type: "mouse_move", session_id: state.sessionID, x: move.x, y: move.y }, { log: false, channel: "input" });
    });
  }

  function requestHostClipboard() {
    if (sendViewerControlMessage({ type: "clipboard_get", session_id: state.sessionID }, { channel: "aux" })) {
      setClipboardStatus("Requesting the current host clipboard...");
    }
  }

  function sendClipboardToHost() {
    const text = sessionClipboardTextEl.value || "";
    if (sendViewerControlMessage({ type: "clipboard_set", session_id: state.sessionID, text }, { channel: "aux" })) {
      setClipboardStatus(`Sending ${text.length} characters to the host clipboard...`);
    }
  }

  async function sendSelectedFileToHost() {
    const file = sessionFileInputEl.files && sessionFileInputEl.files[0];
    if (!file) {
      setFileStatus("Choose a file before starting a transfer.");
      return;
    }
    if (state.outboundTransfer && state.outboundTransfer.active) {
      setFileStatus("A file transfer is already running.");
      return;
    }

    const transferID = randomTransferID();
    state.outboundTransfer = {
      id: transferID,
      active: true,
      cancelled: false,
      name: file.name,
      totalBytes: file.size,
      bytesSent: 0,
    };
    sessionFileCancelBtn.disabled = false;
    setFileStatus(`Starting transfer for ${file.name}...`);
    if (!sendViewerControlMessage({
      type: "file_begin",
      session_id: state.sessionID,
      transfer_id: transferID,
      name: file.name,
      size: file.size,
      last_modified: file.lastModified,
    }, { log: false, channel: "aux" })) {
      state.outboundTransfer = null;
      sessionFileCancelBtn.disabled = true;
      return;
    }

    const chunkSize = 48 * 1024;
    try {
      for (let offset = 0, chunkIndex = 0; offset < file.size; offset += chunkSize, chunkIndex += 1) {
        if (!state.outboundTransfer || state.outboundTransfer.id !== transferID || state.outboundTransfer.cancelled) {
          setFileStatus(`Transfer cancelled for ${file.name}.`);
          return;
        }
        const buffer = await file.slice(offset, Math.min(file.size, offset + chunkSize)).arrayBuffer();
        if (!sendViewerControlMessage({
          type: "file_chunk",
          session_id: state.sessionID,
          transfer_id: transferID,
          index: chunkIndex,
          data: arrayBufferToBase64(buffer),
        }, { log: false, channel: "aux" })) {
          state.outboundTransfer = null;
          sessionFileCancelBtn.disabled = true;
          return;
        }
        state.outboundTransfer.bytesSent = Math.min(file.size, offset + buffer.byteLength);
        setFileStatus(`Sending ${file.name} · ${formatBytes(state.outboundTransfer.bytesSent)} of ${formatBytes(file.size)} uploaded from the viewer.`);
        if (chunkIndex % 4 === 3) {
          await new Promise((resolve) => window.setTimeout(resolve, 0));
        }
      }

      if (!state.outboundTransfer || state.outboundTransfer.cancelled) {
        setFileStatus(`Transfer cancelled for ${file.name}.`);
        return;
      }
      if (sendViewerControlMessage({
        type: "file_complete",
        session_id: state.sessionID,
        transfer_id: transferID,
      }, { log: false, channel: "aux" })) {
        setFileStatus(`Upload finished for ${file.name}. Waiting for host confirmation...`);
        return;
      }
      state.outboundTransfer = null;
      sessionFileCancelBtn.disabled = true;
    } catch (_error) {
      state.outboundTransfer = null;
      sessionFileCancelBtn.disabled = true;
      setFileStatus(`Unable to send ${file.name} right now.`);
    }
  }

  function cancelSelectedFileTransfer() {
    if (!state.outboundTransfer || !state.outboundTransfer.active) return;
    state.outboundTransfer.cancelled = true;
    sendViewerControlMessage({
      type: "file_cancel",
      session_id: state.sessionID,
      transfer_id: state.outboundTransfer.id,
    }, { log: false, channel: "aux" });
    sessionFileCancelBtn.disabled = true;
    setFileStatus(`Transfer cancelled for ${state.outboundTransfer.name}.`);
    state.outboundTransfer = null;
  }

  async function toggleFullscreen() {
    state.expandedView = !state.expandedView;
    renderFullscreenState();
  }

  async function closeCurrentSession() {
    clearSessionFeedback();
    const response = await fetch(`/api/v1/sessions/${encodeURIComponent(state.sessionID)}/close`, {
      method: "POST",
      headers: sessionRequestHeaders({ "X-CSRF-Token": state.csrfToken }),
    });
    captureCSRF(response);
    const data = await response.json();
    if (!response.ok) {
      setSessionFeedback("error", data.error || "Unable to close this session.");
      return;
    }
    closeRealtimeSession();
    renderSessionDetails(data);
    setSessionFeedback("success", `Session ${data.id} closed.`);
  }

  function pointerPositionFromScreen(event) {
    const rect = isScreenVideoActive()
      ? sessionScreenVideoEl.getBoundingClientRect()
      : (state.hasRenderedScreenFrame
      ? sessionScreenCanvasEl.getBoundingClientRect()
      : (sessionScreenImageEl.classList.contains("hidden")
        ? sessionScreenStageEl.getBoundingClientRect()
        : sessionScreenImageEl.getBoundingClientRect()));
    const x = Math.max(0, Math.min(1, (event.clientX - rect.left) / Math.max(1, rect.width)));
    const y = Math.max(0, Math.min(1, (event.clientY - rect.top) / Math.max(1, rect.height)));
    state.lastPointerX = Number(x.toFixed(4));
    state.lastPointerY = Number(y.toFixed(4));
    return { x: state.lastPointerX, y: state.lastPointerY };
  }

  function releasePointerState(sendMouseUp = false) {
    if (sendMouseUp && state.pointerPressed && state.lastPointerX !== null && state.lastPointerY !== null) {
      sendViewerControlMessage({
        type: "mouse_up",
        session_id: state.sessionID,
        button: state.activePointerButton,
        x: state.lastPointerX,
        y: state.lastPointerY,
      }, { channel: "input" });
    }
    state.pointerPressed = false;
    state.activePointerID = null;
    state.activePointerButton = 0;
    sessionScreenStageEl.classList.remove("is-pressing");
  }

  function bindScreenControlSurface() {
    sessionScreenStageEl.addEventListener("mouseenter", () => sessionScreenStageEl.focus());
    sessionScreenStageEl.addEventListener("pointerdown", (event) => {
      sessionScreenStageEl.focus();
      state.activePointerID = event.pointerId;
      state.activePointerButton = event.button;
      state.pointerPressed = true;
      sessionScreenStageEl.classList.add("is-pressing");
      try {
        sessionScreenStageEl.setPointerCapture(event.pointerId);
      } catch (_error) {}
      const { x, y } = pointerPositionFromScreen(event);
      sendViewerControlMessage({ type: "mouse_move", session_id: state.sessionID, x, y }, { log: false, channel: "input" });
      sendViewerControlMessage({ type: "mouse_down", session_id: state.sessionID, button: event.button, x, y }, { channel: "input" });
      event.preventDefault();
    });
    sessionScreenStageEl.addEventListener("pointermove", (event) => {
      const { x, y } = pointerPositionFromScreen(event);
      queuePointerMove(x, y);
    });

    const releasePointer = (event) => {
      if (!state.pointerPressed) return;
      if (state.activePointerID !== null && event.pointerId !== undefined && state.activePointerID !== event.pointerId) return;
      const { x, y } = pointerPositionFromScreen(event);
      sendViewerControlMessage({ type: "mouse_move", session_id: state.sessionID, x, y }, { log: false, channel: "input" });
      sendViewerControlMessage({ type: "mouse_up", session_id: state.sessionID, button: state.activePointerButton, x, y }, { channel: "input" });
      releasePointerState(false);
      if (event.pointerId !== undefined) {
        try {
          sessionScreenStageEl.releasePointerCapture(event.pointerId);
        } catch (_error) {}
      }
      event.preventDefault();
    };

    sessionScreenStageEl.addEventListener("pointerup", releasePointer);
    sessionScreenStageEl.addEventListener("pointercancel", releasePointer);
    sessionScreenStageEl.addEventListener("contextmenu", (event) => event.preventDefault());
    sessionScreenStageEl.addEventListener("wheel", (event) => {
      if (event.deltaY === 0) return;
      const { x, y } = pointerPositionFromScreen(event);
      const deltaY = event.deltaY > 0 ? 1 : -1;
      sendViewerControlMessage({ type: "mouse_wheel", session_id: state.sessionID, x, y, delta_y: deltaY }, { log: false, channel: "input" });
      event.preventDefault();
    }, { passive: false });
    sessionScreenStageEl.addEventListener("keydown", (event) => {
      if (!event.repeat) {
        sendViewerControlMessage({ type: "key_down", session_id: state.sessionID, key: event.key, code: event.code }, { channel: "input" });
      }
      event.preventDefault();
    });
    sessionScreenStageEl.addEventListener("keyup", (event) => {
      sendViewerControlMessage({ type: "key_up", session_id: state.sessionID, key: event.key, code: event.code }, { channel: "input" });
      event.preventDefault();
    });
    window.addEventListener("blur", () => releasePointerState(true));
  }

  async function ensureBrowserIdentity() {
    const response = await fetch(browserIdentityRequestURL());
    captureCSRF(response);
    if (!response.ok) throw new Error("browser identity fetch failed");
    const data = await response.json();
    state.browserIdentity = data;
    if (data.token) window.localStorage.setItem("unydesk.browser.token", data.token);
  }

  function readSessionID() {
    const url = new URL(window.location.href);
    return String(url.searchParams.get("session") || "").trim();
  }

  function readStandaloneToken() {
    const url = new URL(window.location.href);
    const hash = String(url.hash || "").replace(/^#/, "");
    let token = "";
    if (hash) {
      const params = new URLSearchParams(hash);
      token = String(params.get("standalone") || "").trim();
      if (token) {
        window.sessionStorage.setItem(standaloneTokenStorageKey(), token);
        params.delete("standalone");
        const cleanHash = params.toString();
        const cleanURL = `${url.pathname}${url.search}${cleanHash ? `#${cleanHash}` : ""}`;
        window.history.replaceState({}, document.title, cleanURL);
      }
    }
    if (!token) token = String(window.sessionStorage.getItem(standaloneTokenStorageKey()) || "").trim();
    state.standaloneToken = token;
  }

  sessionFullscreenBtn.addEventListener("click", () => void toggleFullscreen());
  sessionSendPingBtn.addEventListener("click", () => {
    sendViewerControlMessage({ type: "ping", session_id: state.sessionID, sent_at: new Date().toISOString() }, { channel: "aux" });
  });
  sessionClipboardSendBtn.addEventListener("click", sendClipboardToHost);
  sessionClipboardReadBtn.addEventListener("click", requestHostClipboard);
  sessionFileInputEl.addEventListener("change", updateSelectedFileMeta);
  sessionFileSendBtn.addEventListener("click", () => void sendSelectedFileToHost());
  sessionFileCancelBtn.addEventListener("click", cancelSelectedFileTransfer);
  sessionRefreshBtn.addEventListener("click", async () => {
    try {
      await handleSessionUpdate(await fetchSession(), true);
    } catch (_error) {
      setSessionFeedback("error", "Unable to refresh session details right now.");
    }
  });
  sessionCloseBtn.addEventListener("click", closeCurrentSession);
  bindScreenControlSurface();
  window.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && state.expandedView) {
      state.expandedView = false;
      renderFullscreenState();
    }
  });
  window.addEventListener("beforeunload", () => {
    closeSessionSocket();
    closeRealtimeSession();
    if (state.currentScreenObjectURL) {
      URL.revokeObjectURL(state.currentScreenObjectURL);
      state.currentScreenObjectURL = "";
    }
  });

  sessionScreenVideoEl.addEventListener("loadeddata", () => showScreenVideo("H.264 video"));
  sessionScreenVideoEl.addEventListener("playing", () => showScreenVideo("H.264 video"));
  sessionScreenVideoEl.addEventListener("resize", () => {
    if (isScreenVideoActive()) {
      updateScreenMeta("H.264 video", sessionScreenVideoEl.videoWidth || 0, sessionScreenVideoEl.videoHeight || 0);
    }
  });

  sessionScreenImageEl.addEventListener("load", () => {
    const width = sessionScreenImageEl.naturalWidth || sessionScreenImageEl.width;
    const height = sessionScreenImageEl.naturalHeight || sessionScreenImageEl.height;
    updateScreenMeta("Peer frame", width, height);
  });

  try {
    state.sessionID = readSessionID();
    if (!state.sessionID) throw new Error("Missing session id");
    state.standaloneMode = window.location.pathname.startsWith("/connect");
    readStandaloneToken();
    if (state.standaloneMode && !state.standaloneToken) {
      throw new Error("Missing standalone token");
    }
    updateSelectedFileMeta();
    setClipboardStatus("Clipboard sync is ready when the WebRTC aux channel is connected.");
    setFileStatus("Files are delivered to the host Downloads folder when the WebRTC aux channel is connected.");
    await ensureBrowserIdentity();
    await handleSessionUpdate(await fetchSession(), false);
    renderFullscreenState();
    openSessionSocket();
  } catch (error) {
    if (error instanceof Error && error.message === "Missing standalone token") {
      setSessionFeedback("error", "Missing standalone client password for this session.");
      return;
    }
    setSessionFeedback("error", "Unable to open this control session.");
  }
})();
