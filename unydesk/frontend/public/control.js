(async () => {
  const CONTROL_DIAGNOSTIC_BUILD = "20260629-h264-first-stall-guard";
  const SCREEN_WIRE_HEADER_BYTES = 36;
  const SCREEN_WIRE_CODEC_WEBP = 2;
  const SCREEN_WIRE_CODEC_PNG = 3;
  const SCREEN_WIRE_CODEC_RGBA = 4;
  const SCREEN_WIRE_KIND_KEYFRAME = 1;
  const SCREEN_WIRE_KIND_PATCH = 2;
  const SCREEN_CHUNK_MAGIC = "USDT";
  const SCREEN_CHUNK_HEADER_BYTES = 20;
  const SCREEN_CHUNK_MAX_ASSEMBLIES = 3;
  const SCREEN_CHUNK_ASSEMBLY_TIMEOUT_MS = 350;
  const SESSION_SOCKET_RECONNECT_BASE_MS = 1000;
  const SESSION_SOCKET_RECONNECT_MAX_MS = 30000;
  const SESSION_SOCKET_RECONNECT_MAX_ATTEMPTS = 8;
  const SESSION_SOCKET_HIDDEN_RECONNECT_MS = 60000;
  const VIDEO_PLAYBACK_TIMEOUT_MS = 1800;
  const RTC_STATS_INTERVAL_MS = 1500;
  const RTC_VIDEO_STALL_TIMEOUT_MS = 4500;
  const VIDEO_FRAME_LOG_INTERVAL_MS = 2000;
  const SCREEN_FALLBACK_RETRY_MS = 800;
  const SCREEN_FALLBACK_MAX_ATTEMPTS = 5;
  const REMOTE_CURSOR_ARROW = 'url("data:image/svg+xml,%3Csvg xmlns=\'http://www.w3.org/2000/svg\' width=\'20\' height=\'20\' viewBox=\'0 0 20 20\'%3E%3Cpath d=\'M3 2l10 10H8.6l2.5 5.1-1.8.9-2.5-5.1L4 15.6V2z\' fill=\'%23000000\'/%3E%3Cpath d=\'M3.6 3.3v10.85l2.83-2.81h.39l2.34 4.8.78-.39-2.34-4.8v-.54h3.54L3.6 3.3z\' fill=\'%23ffffff\' fill-opacity=\'.18\'/%3E%3C/svg%3E") 3 2, auto';

  const state = {
    csrfToken: "",
    sessionID: "",
    standaloneMode: false,
    standaloneToken: "",
    sessionSocket: null,
    sessionSocketTimer: null,
    sessionSocketConnected: false,
    sessionSocketReconnectAttempts: 0,
    sessionSocketStopped: false,
    iceServers: [],
    runtimeFeatures: {},
    preferredVideoCodecs: ["h264", "h265", "av1"],
    disabledRealtimeCodecs: new Set(),
    activeRealtimeCodecOrder: [],
    realtimeRetrying: false,
    peerConnection: null,
    realtimeStarting: false,
    realtimeStartedAt: 0,
    realtimeOfferCreatedAt: 0,
    realtimeOfferPostedAt: 0,
    realtimeAnswerAppliedAt: 0,
    realtimeTrackReceivedAt: 0,
    currentOfferSDP: "",
    lastIgnoredAnswerKey: "",
    realtimeStatsTimer: null,
    realtimeLastStats: null,
    realtimeHostStatusSeen: false,
    realtimeHostStatusWatchdog: null,
    realtimeTrackUnavailable: false,
    realtimeVideoStallStartedAt: 0,
    realtimeVideoStallRecoveryStarted: false,
    videoFrameProbeActive: false,
    videoFrameProbeToken: 0,
    videoFrameStats: null,
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
    videoPlaybackTimer: null,
    screenFallbackRequested: false,
    screenFallbackAttempts: 0,
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
    screenFallbackLastStatsAt: 0,
    screenFallbackFrames: 0,
    screenFallbackBytes: 0,
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
    state.viewerTransportLog = state.viewerTransportLog.slice(-200);
    sessionTransportLogEl.value = state.viewerTransportLog.join("\n");
    sessionTransportLogEl.scrollTop = sessionTransportLogEl.scrollHeight;
  }

  function nowMs() {
    return window.performance && typeof window.performance.now === "function" ? window.performance.now() : Date.now();
  }

  function elapsedMs(startedAt) {
    if (!startedAt) return 0;
    return Math.max(0, Math.round(nowMs() - startedAt));
  }

  async function timedStep(label, fn) {
    const startedAt = nowMs();
    try {
      const result = await fn();
      appendTransportLog(`${label} completed in ${Math.round(nowMs() - startedAt)}ms`);
      return result;
    } catch (error) {
      appendTransportLog(`${label} failed after ${Math.round(nowMs() - startedAt)}ms: ${error.message}`);
      throw error;
    }
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

  function normalizeIceServerURLs(urls) {
    if (typeof urls === "string") {
      const value = urls.trim();
      return value ? [value] : [];
    }
    if (!Array.isArray(urls)) return [];
    const seen = new Set();
    const normalized = [];
    urls.forEach((url) => {
      const value = String(url || "").trim();
      if (!value || seen.has(value)) return;
      seen.add(value);
      normalized.push(value);
    });
    return normalized;
  }

  function normalizeIceServers(servers) {
    if (!Array.isArray(servers)) return [];
    return servers.map((server) => {
      const urls = normalizeIceServerURLs(server && server.urls);
      if (urls.length === 0) return null;
      const normalized = { urls };
      const username = String((server && server.username) || "").trim();
      const credential = String((server && server.credential) || "").trim();
      const credentialType = String((server && (server.credentialType || server.credential_type)) || "").trim();
      if (username) normalized.username = username;
      if (credential) normalized.credential = credential;
      if (credentialType) normalized.credentialType = credentialType;
      return normalized;
    }).filter(Boolean);
  }

  function describeIceServers(servers) {
    if (!servers || servers.length === 0) {
      return "ICE servers not configured; direct host candidates only";
    }
    let stunURLs = 0;
    let turnURLs = 0;
    servers.forEach((server) => {
      normalizeIceServerURLs(server.urls).forEach((url) => {
        const value = url.toLowerCase();
        if (value.startsWith("turn:") || value.startsWith("turns:")) turnURLs += 1;
        if (value.startsWith("stun:") || value.startsWith("stuns:")) stunURLs += 1;
      });
    });
    return `ICE servers configured: ${servers.length} server(s), ${turnURLs} TURN URL(s), ${stunURLs} STUN URL(s)`;
  }

  async function loadRuntimeInfo() {
    try {
      const response = await fetch("/api/v1/info", {
        headers: sessionRequestHeaders(),
      });
      captureCSRF(response);
      if (!response.ok) throw new Error(`runtime info failed (${response.status})`);
      const data = await response.json();
      const servers = data.ice_servers || (data.webrtc && data.webrtc.ice_servers) || [];
      const features = data.features || {};
      const preferred = features.preferred_video_codecs || (data.webrtc && data.webrtc.preferred_video_codecs) || [];
      state.iceServers = normalizeIceServers(servers);
      state.runtimeFeatures = features;
      state.preferredVideoCodecs = normalizePreferredVideoCodecs(preferred);
      appendTransportLog(`diagnostic build: ${CONTROL_DIAGNOSTIC_BUILD}`);
      appendTransportLog(describeIceServers(state.iceServers));
      appendTransportLog(describeRuntimeFeatures());
    } catch (error) {
      state.iceServers = [];
      state.runtimeFeatures = {};
      state.preferredVideoCodecs = ["h264", "h265", "av1"];
      appendTransportLog(`ICE config unavailable: ${error.message}`);
    }
  }

  function normalizePreferredVideoCodecs(codecs) {
    const values = Array.isArray(codecs) && codecs.length > 0 ? codecs : ["h264", "h265", "av1"];
    const normalized = [];
    const seen = new Set();
    values.forEach((raw) => {
      let codec = String(raw || "").trim().toLowerCase().replace("video/", "");
      if (codec === "hevc" || codec === "h.265") codec = "h265";
      if (codec === "h.264") codec = "h264";
      if (!["h265", "av1", "h264", "vp9", "vp8"].includes(codec) || seen.has(codec)) return;
      seen.add(codec);
      normalized.push(codec);
    });
    return normalized.length > 0 ? normalized : ["h264", "h265", "av1"];
  }

  function describeRuntimeFeatures() {
    const features = state.runtimeFeatures || {};
    const enabled = [];
    if (features.h265) enabled.push("H.265");
    if (features.av1) enabled.push("AV1");
    if (features.h264) enabled.push("H.264");
    if (features.quic) enabled.push("QUIC-reserved");
    return `runtime features: ${enabled.length ? enabled.join(", ") : "default"} · codec preference: ${state.preferredVideoCodecs.join(" > ")}`;
  }

  function codecNameForMime(mimeType) {
    const value = String(mimeType || "").toLowerCase();
    if (value.includes("h265") || value.includes("hevc")) return "h265";
    if (value.includes("av1")) return "av1";
    if (value.includes("h264")) return "h264";
    if (value.includes("vp9")) return "vp9";
    if (value.includes("vp8")) return "vp8";
    return "";
  }

  function applyVideoCodecPreferences(transceiver) {
    if (!transceiver || typeof transceiver.setCodecPreferences !== "function") return;
    if (!window.RTCRtpReceiver || typeof window.RTCRtpReceiver.getCapabilities !== "function") return;
    const capabilities = window.RTCRtpReceiver.getCapabilities("video");
    const codecs = capabilities && Array.isArray(capabilities.codecs) ? capabilities.codecs : [];
    if (codecs.length === 0) return;
    const preferred = [];
    const selected = new Set();
    state.preferredVideoCodecs.forEach((codecName) => {
      if (state.disabledRealtimeCodecs.has(codecName)) return;
      const rankedCodecs = codecs
        .map((codec, index) => ({ codec, index }))
        .filter((entry) => codecNameForMime(entry.codec.mimeType) === codecName)
        .sort((left, right) => codecPreferenceRank(codecName, left.codec) - codecPreferenceRank(codecName, right.codec));
      rankedCodecs.forEach(({ codec, index }) => {
        if (selected.has(index)) return;
        preferred.push(codec);
        selected.add(index);
      });
    });
    if (preferred.length === 0) return;
    const remaining = codecs.filter((codec, index) => {
      if (selected.has(index)) return false;
      const codecName = codecNameForMime(codec.mimeType);
      return !codecName || !state.disabledRealtimeCodecs.has(codecName);
    });
    try {
      transceiver.setCodecPreferences([...preferred, ...remaining]);
      state.activeRealtimeCodecOrder = uniqueCodecNames(preferred).map((name) => name.toLowerCase().replace(".", ""));
      appendTransportLog(`video codec preference applied: ${uniqueCodecNames(preferred).join(" > ")}`);
    } catch (error) {
      appendTransportLog(`video codec preference ignored: ${error.message}`);
    }
  }

  function applyRealtimePlaybackHints(receiver) {
    if (!receiver) return;
    const hints = [
      ["playoutDelayHint", 0],
      ["jitterBufferTarget", 0],
    ];
    hints.forEach(([property, value]) => {
      if (!(property in receiver)) return;
      try {
        receiver[property] = value;
        appendTransportLog(`realtime playback hint applied: ${property}=0`);
      } catch (_error) {}
    });
  }

  function describeVideoCodecsFromSDP(sdp) {
    const lines = String(sdp || "").split(/\r?\n/);
    const payloads = new Set();
    let inVideo = false;
    lines.forEach((line) => {
      if (line.startsWith("m=")) {
        inVideo = line.startsWith("m=video");
        if (inVideo) {
          line.split(/\s+/).slice(3).forEach((payload) => payloads.add(payload));
        }
        return;
      }
      if (!inVideo) return;
      const match = line.match(/^a=rtpmap:(\d+)\s+([^/]+)/i);
      if (match && payloads.has(match[1])) {
        payloads.add(`${match[1]}:${match[2].toUpperCase()}`);
      }
    });
    return Array.from(payloads)
      .map((value) => String(value))
      .filter((value) => value.includes(":"))
      .slice(0, 12)
      .join(", ") || "none";
  }

  function startRealtimeStatsMonitor(peerConnection) {
    stopRealtimeStatsMonitor();
    state.realtimeLastStats = null;
    state.realtimeVideoStallStartedAt = 0;
    state.realtimeVideoStallRecoveryStarted = false;
    const tick = async () => {
      if (!peerConnection || state.peerConnection !== peerConnection || peerConnection.connectionState === "closed") return;
      try {
        const report = await peerConnection.getStats();
        appendTransportLog(describeRTCStats(report));
      } catch (error) {
        appendTransportLog(`RTC stats unavailable: ${error.message}`);
      }
    };
    state.realtimeStatsTimer = window.setInterval(tick, RTC_STATS_INTERVAL_MS);
    window.setTimeout(tick, 300);
  }

  function stopRealtimeStatsMonitor() {
    if (state.realtimeStatsTimer) {
      window.clearInterval(state.realtimeStatsTimer);
      state.realtimeStatsTimer = null;
    }
    state.realtimeLastStats = null;
  }

  function scheduleHostStatusWatchdog() {
    clearHostStatusWatchdog();
    state.realtimeHostStatusSeen = false;
    state.realtimeHostStatusWatchdog = window.setTimeout(() => {
      state.realtimeHostStatusWatchdog = null;
      if (state.realtimeHostStatusSeen || !state.peerConnection || state.peerConnection.connectionState !== "connected") return;
      appendTransportLog("host RTP diagnostics missing after WebRTC connect; reload page and retélécharge/restart the latest host binary if this line persists");
    }, 4500);
  }

  function clearHostStatusWatchdog() {
    if (!state.realtimeHostStatusWatchdog) return;
    window.clearTimeout(state.realtimeHostStatusWatchdog);
    state.realtimeHostStatusWatchdog = null;
  }

  function describeRTCStats(report) {
    let inbound = null;
    let selectedPair = null;
    const codecs = new Map();
    const candidates = new Map();
    report.forEach((stat) => {
      if (stat.type === "codec") codecs.set(stat.id, stat);
      if (stat.type === "local-candidate" || stat.type === "remote-candidate") candidates.set(stat.id, stat);
      if (stat.type === "inbound-rtp" && (stat.kind === "video" || stat.mediaType === "video") && !stat.isRemote) {
        if (!inbound || Number(stat.timestamp || 0) > Number(inbound.timestamp || 0)) inbound = stat;
      }
      if (stat.type === "candidate-pair" && (stat.selected || (stat.nominated && stat.state === "succeeded"))) {
        selectedPair = stat;
      }
    });

    const pairText = selectedPair ? describeCandidatePair(selectedPair, candidates) : "pair=pending";
    if (!inbound) {
      state.realtimeLastStats = { inbound: null, at: nowMs() };
      return `RTC stats: no inbound video RTP yet · ${pairText}`;
    }

    const previous = state.realtimeLastStats && state.realtimeLastStats.inbound;
    const elapsedSeconds = previous && inbound.timestamp && previous.timestamp
      ? Math.max(0.001, (inbound.timestamp - previous.timestamp) / 1000)
      : 0;
    const bytesDelta = previous ? Math.max(0, Number(inbound.bytesReceived || 0) - Number(previous.bytesReceived || 0)) : 0;
    const framesDelta = previous ? Math.max(0, Number(inbound.framesDecoded || 0) - Number(previous.framesDecoded || 0)) : 0;
    const bitrateKbps = elapsedSeconds ? (bytesDelta * 8) / 1000 / elapsedSeconds : 0;
    const decodedFPS = elapsedSeconds ? framesDelta / elapsedSeconds : 0;
    const jitterMs = Number(inbound.jitter || 0) * 1000;
    const codec = codecs.get(inbound.codecId);
    const codecText = codec ? `${String(codec.mimeType || "video").replace("video/", "").toUpperCase()} ${String(codec.sdpFmtpLine || "").slice(0, 80)}`.trim() : "codec=unknown";
    const rttMs = selectedPair && Number.isFinite(Number(selectedPair.currentRoundTripTime)) ? Number(selectedPair.currentRoundTripTime) * 1000 : 0;
    const size = inbound.frameWidth && inbound.frameHeight ? `${inbound.frameWidth}x${inbound.frameHeight}` : "size=?";
    const freezes = Number(inbound.freezeCount || 0);
    const freezeSeconds = Number(inbound.totalFreezesDuration || 0);
    observeRealtimeVideoHealth({ bytesDelta, framesDelta, inbound, selectedPair, codecText });
    state.realtimeLastStats = { inbound: { ...inbound }, at: nowMs() };
    return `RTC stats: ${size} recv=${bitrateKbps.toFixed(0)}kbps decoded=${decodedFPS.toFixed(1)}fps frames=${Number(inbound.framesDecoded || 0)} dropped=${Number(inbound.framesDropped || 0)} lost=${Number(inbound.packetsLost || 0)} jitter=${jitterMs.toFixed(0)}ms rtt=${rttMs.toFixed(0)}ms pli=${Number(inbound.pliCount || 0)} nack=${Number(inbound.nackCount || 0)} freeze=${freezes}/${freezeSeconds.toFixed(1)}s · ${codecText} · ${pairText}`;
  }

  function observeRealtimeVideoHealth({ bytesDelta, framesDelta, inbound, selectedPair, codecText }) {
    if (!state.screenVideoReady || state.realtimeRetrying || state.screenFallbackRequested) {
      state.realtimeVideoStallStartedAt = 0;
      return;
    }
    const decodedFrames = Number(inbound && inbound.framesDecoded || 0);
    if (decodedFrames <= 0) {
      state.realtimeVideoStallStartedAt = 0;
      return;
    }
    const iceConnected = state.peerConnection && ["connected", "completed"].includes(String(state.peerConnection.iceConnectionState || ""));
    const pairSucceeded = !selectedPair || selectedPair.state === "succeeded" || selectedPair.selected || selectedPair.nominated;
    const stalled = iceConnected && pairSucceeded && Number(bytesDelta || 0) <= 0 && Number(framesDelta || 0) <= 0;
    if (!stalled) {
      state.realtimeVideoStallStartedAt = 0;
      return;
    }
    if (!state.realtimeVideoStallStartedAt) {
      state.realtimeVideoStallStartedAt = nowMs();
      return;
    }
    if (state.realtimeVideoStallRecoveryStarted || nowMs() - state.realtimeVideoStallStartedAt < RTC_VIDEO_STALL_TIMEOUT_MS) return;
    state.realtimeVideoStallRecoveryStarted = true;
    const codec = state.activeRealtimeCodecOrder[0] || "";
    appendTransportLog(`realtime video stalled for ${Math.round(nowMs() - state.realtimeVideoStallStartedAt)}ms · ${codecText}`);
    if (codec === "h265" && retryRealtimeWithoutCodec("h265", "H.265 stalled after first frames")) return;
    resetScreenVideo();
    sessionScreenMetaEl.textContent = "Realtime video stalled; requesting peer frame fallback.";
    requestScreenFallback("realtime video stalled after first frames");
  }

  function describeCandidatePair(pair, candidates) {
    const local = candidates.get(pair.localCandidateId) || {};
    const remote = candidates.get(pair.remoteCandidateId) || {};
    const localText = `${local.candidateType || "local"}/${local.protocol || "?"}`;
    const remoteText = `${remote.candidateType || "remote"}/${remote.protocol || "?"}`;
    const available = Number(pair.availableIncomingBitrate || pair.availableOutgoingBitrate || 0);
    return `pair=${localText}->${remoteText}${available ? ` avail=${(available / 1000).toFixed(0)}kbps` : ""}`;
  }

  function startVideoFrameProbe() {
    stopVideoFrameProbe();
    if (!sessionScreenVideoEl || typeof sessionScreenVideoEl.requestVideoFrameCallback !== "function") {
      appendTransportLog("video frame probe unavailable in this browser");
      return;
    }
    state.videoFrameProbeActive = true;
    state.videoFrameProbeToken += 1;
    const token = state.videoFrameProbeToken;
    state.videoFrameStats = {
      startedAt: nowMs(),
      lastLogAt: nowMs(),
      firstFrameLogged: false,
      lastPresentedFrames: 0,
    };
    const probe = (_now, metadata) => {
      if (!state.videoFrameProbeActive || token !== state.videoFrameProbeToken || !sessionScreenVideoEl.srcObject) return;
      const stats = state.videoFrameStats || {};
      const presented = Number(metadata.presentedFrames || 0);
      if (!stats.firstFrameLogged) {
        stats.firstFrameLogged = true;
        appendTransportLog(`first decoded video frame after offer=${elapsedMs(state.realtimeOfferPostedAt)}ms answer=${elapsedMs(state.realtimeAnswerAppliedAt)}ms track=${elapsedMs(state.realtimeTrackReceivedAt)}ms size=${sessionScreenVideoEl.videoWidth || 0}x${sessionScreenVideoEl.videoHeight || 0}`);
      }
      const elapsed = nowMs() - Number(stats.lastLogAt || 0);
      if (elapsed >= VIDEO_FRAME_LOG_INTERVAL_MS) {
        const deltaFrames = Math.max(0, presented - Number(stats.lastPresentedFrames || 0));
        const fps = elapsed ? deltaFrames * 1000 / elapsed : 0;
        appendTransportLog(`video frames: rendered=${presented} fps=${fps.toFixed(1)} size=${sessionScreenVideoEl.videoWidth || 0}x${sessionScreenVideoEl.videoHeight || 0} media=${Number(metadata.mediaTime || 0).toFixed(2)}s processing=${Number(metadata.processingDuration || 0).toFixed(3)}s`);
        stats.lastLogAt = nowMs();
        stats.lastPresentedFrames = presented;
      }
      state.videoFrameStats = stats;
      sessionScreenVideoEl.requestVideoFrameCallback(probe);
    };
    sessionScreenVideoEl.requestVideoFrameCallback(probe);
  }

  function stopVideoFrameProbe() {
    state.videoFrameProbeActive = false;
    state.videoFrameProbeToken += 1;
    state.videoFrameStats = null;
  }

  function codecPreferenceRank(codecName, codec) {
    if (codecName !== "h264") return 100;
    const fmtp = String((codec && codec.sdpFmtpLine) || "").toLowerCase();
    const packetizationScore = fmtp.includes("packetization-mode=1") ? 0 : 100;
    const profileMatch = fmtp.match(/profile-level-id=([0-9a-f]+)/);
    const profile = profileMatch ? profileMatch[1] : "";
    const profileRank = {
      "42e01f": 0,
      "42001f": 10,
      "4d001f": 20,
      "64001f": 30,
    };
    return packetizationScore + (profileRank[profile] ?? 90);
  }

  function uniqueCodecNames(codecs) {
    const names = [];
    const seen = new Set();
    codecs.forEach((codec) => {
      const name = codecNameForMime(codec.mimeType).toUpperCase();
      if (!name || seen.has(name)) return;
      seen.add(name);
      names.push(name);
    });
    return names;
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
    sessionAnswerPreviewEl.value = "Screen video uses WebRTC realtime codecs (H.265 preferred, H.264 stable fallback, AV1 optional) when available; input, clipboard, and file transfer use WebRTC data channels with broker fallback.";
    sessionCloseBtn.disabled = session.status === "closed";
    renderScreenPreview(session);
  }

  function describeTransportState() {
    const states = [];
    if (state.sessionSocketConnected) states.push("signaling");
    if (state.peerConnection) states.push(`pc:${state.peerConnectionState}`);
    if (state.inputChannel && state.inputChannel.readyState === "open") states.push("input");
    if (state.auxChannel && state.auxChannel.readyState === "open") states.push("aux");
    if (isScreenVideoActive()) states.push("video");
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

  function clearVideoPlaybackWatchdog() {
    if (!state.videoPlaybackTimer) return;
    window.clearTimeout(state.videoPlaybackTimer);
    state.videoPlaybackTimer = null;
  }

  function requestScreenFallback(reason, attempt = 0) {
    if (!state.sessionID) return;
    if (attempt === 0) {
      if (state.screenFallbackRequested) return;
      state.screenFallbackRequested = true;
      state.screenFallbackAttempts = 0;
      appendTransportLog(`requesting peer frame fallback: ${reason}`);
    }
    state.screenFallbackAttempts = Math.max(state.screenFallbackAttempts, attempt + 1);
    const sent = sendViewerControlMessage({
      type: "screen_fallback_request",
      session_id: state.sessionID,
      reason,
    }, { channel: "aux", log: false, silent: true, peerOnly: true });
    if (sent) {
      appendTransportLog("peer frame fallback requested");
      return;
    }
    if (attempt + 1 >= SCREEN_FALLBACK_MAX_ATTEMPTS) {
      appendTransportLog("peer frame fallback request could not be delivered yet");
      return;
    }
    window.setTimeout(() => requestScreenFallback(reason, attempt + 1), SCREEN_FALLBACK_RETRY_MS);
  }

  function scheduleVideoPlaybackWatchdog() {
    clearVideoPlaybackWatchdog();
    if (state.realtimeTrackUnavailable) {
      requestScreenFallback("realtime video track unavailable");
      return;
    }
    state.screenFallbackRequested = false;
    state.screenFallbackAttempts = 0;
    state.videoPlaybackTimer = window.setTimeout(() => {
      state.videoPlaybackTimer = null;
      if (isScreenVideoActive()) return;
      appendTransportLog("realtime video track did not produce a decoded frame in time");
      const inbound = state.realtimeLastStats && state.realtimeLastStats.inbound;
      if (inbound) {
        appendTransportLog(`decode timeout stats: bytes=${Number(inbound.bytesReceived || 0)} packets=${Number(inbound.packetsReceived || 0)} framesDecoded=${Number(inbound.framesDecoded || 0)} keyFrames=${Number(inbound.keyFramesDecoded || 0)} lost=${Number(inbound.packetsLost || 0)} jitter=${(Number(inbound.jitter || 0) * 1000).toFixed(0)}ms`);
      } else {
        appendTransportLog("decode timeout stats: no inbound RTP stats yet");
      }
      if (retryRealtimeWithoutCodec("h265", "H.265 decode timeout")) return;
      resetScreenVideo();
      sessionScreenMetaEl.textContent = "Realtime video decode timeout; requesting peer frame fallback.";
      requestScreenFallback("realtime video playback timeout");
    }, realtimePlaybackTimeoutMs());
  }

  function realtimePlaybackTimeoutMs() {
    const codec = state.activeRealtimeCodecOrder[0] || "";
    if (codec === "h265") return 1400;
    if (codec === "h264") return VIDEO_PLAYBACK_TIMEOUT_MS;
    return 2200;
  }

  function shouldRetryRealtimeWithoutCodec(codecName) {
    const codec = String(codecName || "").toLowerCase();
    if (!codec || state.disabledRealtimeCodecs.has(codec)) return false;
    if (!state.activeRealtimeCodecOrder.length) return false;
    return state.activeRealtimeCodecOrder[0] === codec;
  }

  function retryRealtimeWithoutCodec(codecName, reason) {
    const codec = String(codecName || "").toLowerCase();
    if (!shouldRetryRealtimeWithoutCodec(codec) || state.realtimeRetrying || !state.sessionID) return false;
    state.realtimeRetrying = true;
    state.disabledRealtimeCodecs.add(codec);
    appendTransportLog(`${reason}; retrying realtime video without ${codec.toUpperCase()}`);
    sessionScreenMetaEl.textContent = `${reason}; retrying realtime video with fallback codec.`;
    closeRealtimeSession({ preserveDisabledCodecs: true });
    window.setTimeout(() => {
      void fetchSession()
        .then((session) => ensureRealtimeSession(session))
        .catch((error) => {
          appendTransportLog(`realtime codec retry failed: ${error.message}`);
          requestScreenFallback(`${reason}; codec retry failed`);
        })
        .finally(() => {
          state.realtimeRetrying = false;
        });
    }, 250);
    return true;
  }

  function showScreenVideo(prefix = "Realtime video") {
    if (!sessionScreenVideoEl || !sessionScreenVideoEl.srcObject) return;
    clearVideoPlaybackWatchdog();
    state.screenVideoReady = true;
    if (!state.screenVideoPlaybackLogged) {
      state.screenVideoPlaybackLogged = true;
      appendTransportLog(`realtime video playing after offer=${elapsedMs(state.realtimeOfferPostedAt)}ms answer=${elapsedMs(state.realtimeAnswerAppliedAt)}ms track=${elapsedMs(state.realtimeTrackReceivedAt)}ms`);
    }
    sessionScreenVideoEl.classList.remove("hidden");
    sessionScreenCanvasEl.classList.add("hidden");
    sessionScreenImageEl.classList.add("hidden");
    sessionScreenEmptyEl.classList.add("hidden");
    updateScreenMeta(prefix, sessionScreenVideoEl.videoWidth || 0, sessionScreenVideoEl.videoHeight || 0);
  }

  function resetScreenVideo() {
    stopVideoFrameProbe();
    clearVideoPlaybackWatchdog();
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
    if (!response.ok) {
      const data = await response.json().catch(() => ({}));
      const error = new Error(data.error || "session fetch failed");
      error.status = response.status;
      throw error;
    }
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
    if (!session || session.status === "closed") {
      stopSessionSocketReconnect("session is closed");
      closeRealtimeSession();
      if (session) renderSessionDetails(session);
      return;
    }
    state.sessionSocketReconnectAttempts = 0;
    renderSessionDetails(session);
    maybeRenderDispatchFeedback(session);
    await ensureRealtimeSession(session);
    await applyRemoteSignaling(session);
    if (withFeedback) clearSessionFeedback();
  }

  function stopSessionSocketReconnect(reason = "") {
    if (state.sessionSocketStopped) return;
    state.sessionSocketStopped = true;
    closeSessionSocket({ stopReconnect: true });
    if (reason) appendTransportLog(`stopped signaling reconnect: ${reason}`);
  }

  function scheduleSessionSocketReconnect() {
    if (state.sessionSocketTimer || !state.sessionID || state.sessionSocketStopped) return;
    if (!document.hidden) {
      state.sessionSocketReconnectAttempts += 1;
      if (state.sessionSocketReconnectAttempts > SESSION_SOCKET_RECONNECT_MAX_ATTEMPTS) {
        stopSessionSocketReconnect("too many reconnect attempts");
        setSessionFeedback("error", "Session signaling stopped after repeated reconnect attempts.");
        return;
      }
    }
    const retryDelay = document.hidden
      ? SESSION_SOCKET_HIDDEN_RECONNECT_MS
      : Math.min(
        SESSION_SOCKET_RECONNECT_MAX_MS,
        SESSION_SOCKET_RECONNECT_BASE_MS * (2 ** Math.max(0, state.sessionSocketReconnectAttempts - 1)),
      );
    state.sessionSocketTimer = window.setTimeout(() => {
      state.sessionSocketTimer = null;
      openSessionSocket();
    }, retryDelay);
  }

  function closeSessionSocket(options = {}) {
    if (options.stopReconnect) {
      state.sessionSocketStopped = true;
    }
    if (state.sessionSocketTimer) {
      window.clearTimeout(state.sessionSocketTimer);
      state.sessionSocketTimer = null;
    }
    if (!state.sessionSocket) return;
    const socket = state.sessionSocket;
    state.sessionSocket = null;
    try {
      socket.close();
    } catch (_error) {}
    state.sessionSocketConnected = false;
  }

  function openSessionSocket() {
    closeSessionSocket();
    if (!state.sessionID || state.sessionSocketStopped) return;
    if (document.hidden) {
      scheduleSessionSocketReconnect();
      return;
    }
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
        if (payload.type === "event" && payload.event) {
          const sessionEvent = typeof payload.event === "string"
            ? JSON.parse(payload.event)
            : payload.event;
          handleSessionEvent(sessionEvent);
          return;
        }
        if (payload.type === "error" && payload.error) {
          appendTransportLog(`server error: ${payload.error}`);
          if (/session not found|closed/i.test(String(payload.error))) {
            stopSessionSocketReconnect(payload.error);
          }
        }
      } catch (_error) {}
    });
    socket.addEventListener("close", () => {
      if (state.sessionSocket !== socket) return;
      state.sessionSocketConnected = false;
      appendTransportLog("session signaling websocket disconnected");
      state.sessionSocket = null;
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
    if (state.peerConnection || state.realtimeStarting || !session || session.status === "closed") {
      if (session && session.status === "closed") closeRealtimeSession();
      return;
    }
    state.realtimeStarting = true;
    state.realtimeStartedAt = nowMs();
    state.realtimeOfferCreatedAt = 0;
    state.realtimeOfferPostedAt = 0;
    state.realtimeAnswerAppliedAt = 0;
    state.realtimeTrackReceivedAt = 0;
    const peerConfig = { iceServers: state.iceServers };
    if (state.iceServers.length > 0) {
      peerConfig.iceCandidatePoolSize = 2;
    }
    const peerConnection = new RTCPeerConnection(peerConfig);
    state.peerConnection = peerConnection;
    state.peerConnectionState = peerConnection.connectionState || "new";
    resetScreenChunkAssemblies();
    appendTransportLog(`creating WebRTC peer connection (${describeIceServers(state.iceServers)}) session=${state.sessionID}`);
    startRealtimeStatsMonitor(peerConnection);

    peerConnection.addEventListener("connectionstatechange", () => {
      state.peerConnectionState = peerConnection.connectionState || "unknown";
      appendTransportLog(`peer connection state: ${state.peerConnectionState} after start=${elapsedMs(state.realtimeStartedAt)}ms offer=${elapsedMs(state.realtimeOfferPostedAt)}ms answer=${elapsedMs(state.realtimeAnswerAppliedAt)}ms`);
      renderSessionDetails(session);
      if (state.peerConnectionState === "connected") {
        setClipboardStatus("WebRTC connected. Clipboard sync is ready.");
        if (!state.outboundTransfer) {
          setFileStatus("WebRTC connected. Files are delivered to the host Downloads folder when available.");
        }
        scheduleHostStatusWatchdog();
      }
      if (["failed", "closed", "disconnected"].includes(state.peerConnectionState)) {
        applyRemoteCursor("default");
      }
    });

    peerConnection.addEventListener("iceconnectionstatechange", () => {
      appendTransportLog(`ICE connection state: ${peerConnection.iceConnectionState || "unknown"} after ${elapsedMs(state.realtimeStartedAt)}ms`);
    });

    peerConnection.addEventListener("icegatheringstatechange", () => {
      appendTransportLog(`ICE gathering state: ${peerConnection.iceGatheringState || "unknown"} after ${elapsedMs(state.realtimeStartedAt)}ms`);
    });

    peerConnection.addEventListener("icecandidate", (event) => {
      if (!event.candidate) {
        appendTransportLog("viewer ICE gathering complete");
        return;
      }
      appendTransportLog(`viewer ICE candidate: ${event.candidate.type || "unknown"} ${event.candidate.protocol || ""} addr=${event.candidate.address || "?"} port=${event.candidate.port || "?"}`);
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
      state.realtimeTrackReceivedAt = nowMs();
      applyRealtimePlaybackHints(event.receiver);
      if (state.realtimeTrackUnavailable) {
        appendTransportLog("ignoring empty realtime video track because host encoder is unavailable");
        requestScreenFallback("realtime video track unavailable");
        renderSessionDetails(session);
        return;
      }
      const stream = event.streams && event.streams[0] ? event.streams[0] : new MediaStream([event.track]);
      state.screenVideoStream = stream;
      state.screenVideoReady = false;
      state.screenVideoPlaybackLogged = false;
      sessionScreenVideoEl.srcObject = stream;
      sessionScreenMetaEl.textContent = "Realtime screen video track negotiated; waiting for first video frame.";
      appendTransportLog(`received realtime video track ${event.track.id || "screen"} after answer=${elapsedMs(state.realtimeAnswerAppliedAt)}ms offer=${elapsedMs(state.realtimeOfferPostedAt)}ms streams=${event.streams ? event.streams.length : 0}`);
      scheduleVideoPlaybackWatchdog();
      startVideoFrameProbe();
      event.track.addEventListener("unmute", () => appendTransportLog("realtime video track unmuted"));
      event.track.addEventListener("mute", () => appendTransportLog("realtime video track muted"));
      event.track.addEventListener("ended", () => {
        appendTransportLog("realtime video track ended");
        resetScreenVideo();
      });
      void sessionScreenVideoEl.play().catch((error) => {
        appendTransportLog(`realtime video autoplay waiting: ${error.message}`);
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
        if (typeof messageEvent.data === "string") {
          try {
            handleSessionEvent(JSON.parse(messageEvent.data));
          } catch (_error) {}
          return;
        }
        if (!(messageEvent.data instanceof ArrayBuffer)) return;
        const reassembled = absorbScreenChunk(messageEvent.data);
        if (!reassembled) return;
        queueScreenBuffer(reassembled);
      });
    });

    const videoTransceiver = peerConnection.addTransceiver("video", { direction: "recvonly" });
    applyVideoCodecPreferences(videoTransceiver);
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

    try {
      const offer = await timedStep("create WebRTC offer", () => peerConnection.createOffer());
      state.realtimeOfferCreatedAt = nowMs();
      appendTransportLog(`local offer video codecs: ${describeVideoCodecsFromSDP(offer.sdp)}`);
      await timedStep("set local WebRTC offer", () => peerConnection.setLocalDescription(offer));
      state.currentOfferSDP = JSON.stringify(peerConnection.localDescription);
      await timedStep("post WebRTC offer", () => postSessionJSON(`/api/v1/sessions/${encodeURIComponent(state.sessionID)}/offer`, {
        sdp: state.currentOfferSDP,
      }));
    } catch (error) {
      state.realtimeStarting = false;
      closeRealtimeSession();
      throw error;
    }
    state.localOfferPosted = true;
    state.realtimeOfferPostedAt = nowMs();
    appendTransportLog(`posted WebRTC offer after ${elapsedMs(state.realtimeStartedAt)}ms`);
    renderSessionDetails(session);
    state.realtimeStarting = false;
  }

  async function applyRemoteSignaling(session) {
    const pc = state.peerConnection;
    if (!pc || !session) return;
    if (!state.remoteAnswerApplied && session.answer_sdp) {
      if (!remoteAnswerMatchesCurrentOffer(session)) {
        const key = `${String(session.offer_sdp || "").slice(0, 80)}:${String(session.answer_sdp || "").slice(0, 80)}`;
        if (key !== state.lastIgnoredAnswerKey) {
          state.lastIgnoredAnswerKey = key;
          appendTransportLog("ignored stale remote answer: offer revision mismatch");
        }
        return;
      }
      const answer = JSON.parse(session.answer_sdp);
      appendTransportLog(`remote answer video codecs: ${describeVideoCodecsFromSDP(answer.sdp)}`);
      await timedStep("set remote WebRTC answer", () => pc.setRemoteDescription(answer));
      state.remoteAnswerApplied = true;
      state.realtimeAnswerAppliedAt = nowMs();
      appendTransportLog(`applied remote answer (${countSDPCandidates(answer.sdp)} candidates) after offer=${elapsedMs(state.realtimeOfferPostedAt)}ms`);
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
        await timedStep(`add host ICE candidate ${describeICECandidate(candidate)}`, () => pc.addIceCandidate(candidate));
        appendTransportLog(`applied host ICE candidate: ${describeICECandidate(candidate)}`);
      } catch (error) {
        appendTransportLog(`host ICE candidate add failed: ${error.message}`);
      }
    }
    renderSessionDetails(session);
  }

  function remoteAnswerMatchesCurrentOffer(session) {
    if (!state.localOfferPosted || !state.currentOfferSDP) return false;
    return String(session.offer_sdp || "") === state.currentOfferSDP;
  }

  function closeRealtimeSession(options = {}) {
    if (state.peerConnection) {
      try {
        state.peerConnection.close();
      } catch (_error) {}
    }
    stopRealtimeStatsMonitor();
    stopVideoFrameProbe();
    clearHostStatusWatchdog();
    state.peerConnection = null;
    state.realtimeStarting = false;
    state.realtimeStartedAt = 0;
    state.realtimeOfferCreatedAt = 0;
    state.realtimeOfferPostedAt = 0;
    state.realtimeAnswerAppliedAt = 0;
    state.realtimeTrackReceivedAt = 0;
    state.realtimeTrackUnavailable = false;
    state.currentOfferSDP = "";
    state.lastIgnoredAnswerKey = "";
    state.realtimeHostStatusSeen = false;
    state.peerConnectionState = "closed";
    state.inputChannel = null;
    state.auxChannel = null;
    state.screenChannel = null;
    state.localOfferPosted = false;
    state.remoteAnswerApplied = false;
    state.activeRealtimeCodecOrder = [];
    state.postedViewerCandidates = new Set();
    state.appliedHostCandidates = new Set();
    state.screenFallbackRequested = false;
    state.screenFallbackAttempts = 0;
    if (!options.preserveDisabledCodecs) {
      state.disabledRealtimeCodecs = new Set();
    }
    clearVideoPlaybackWatchdog();
    resetScreenVideo();
    resetScreenChunkAssemblies();
  }

  function resetScreenChunkAssemblies() {
    state.screenChunkAssemblies = new Map();
    state.screenLastCompletedFrameID = 0;
    state.screenLastSeenFrameID = 0;
    state.screenFallbackLastStatsAt = 0;
    state.screenFallbackFrames = 0;
    state.screenFallbackBytes = 0;
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
        if (/h\.?264|av1|video|encoder|decode/i.test(state.lastScreenError)) {
          resetScreenVideo();
        }
        sessionScreenMetaEl.textContent = `Peer screen channel · ${state.lastScreenError}`;
        return;
      case "screen_status": {
        const transport = String(eventPayload.transport || "video").toLowerCase();
        if (!["h265", "h264", "av1", "video"].includes(transport)) return;
        const profile = String(eventPayload.profile || "adaptive");
        const fps = Number(eventPayload.fps || 0);
        const crf = Number(eventPayload.crf || 0);
        const maxEdge = Number(eventPayload.max_edge || 0);
        const ageMs = Number(eventPayload.age_ms || 0);
        const maxrate = Number(eventPayload.maxrate || 0);
        const sentFPS = Number(eventPayload.sent_fps || 0);
        const sentKbps = Number(eventPayload.sent_kbps || 0);
        const sentFrames = Number(eventPayload.sent_frames || 0);
        const rawDrops = Number(eventPayload.raw_drops || 0);
        const encodedDrops = Number(eventPayload.encoded_drops || 0);
        const queueMs = Number(eventPayload.queue_ms || 0);
        const maxQueueMs = Number(eventPayload.max_queue_ms || 0);
        const rtpPackets = Number(eventPayload.rtp_packets || 0);
        const rtpDeltaPackets = Number(eventPayload.rtp_delta_packets || 0);
        const rtpKbps = Number(eventPayload.rtp_kbps || 0);
        const rtpDiscardedPackets = Number(eventPayload.rtp_discarded_packets || 0);
        const pairPackets = Number(eventPayload.pair_packets || 0);
        const pairState = String(eventPayload.pair_state || "");
        const phase = String(eventPayload.phase || "");
        const trigger = String(eventPayload.trigger || "");
        const encoder = String(eventPayload.encoder || "");
        const ffmpeg = String(eventPayload.ffmpeg || "");
        const width = Number(eventPayload.width || 0);
        const height = Number(eventPayload.height || 0);
        const error = String(eventPayload.error || "");
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
        const videoPresent = typeof eventPayload.video_present === "boolean" ? eventPayload.video_present : null;
        const trackPresent = typeof eventPayload.track_present === "boolean" ? eventPayload.track_present : null;
        const streamPresent = typeof eventPayload.stream_present === "boolean" ? eventPayload.stream_present : null;
        const action = String(eventPayload.action || "profile");
        if (["host-rtp", "rtp-starting", "rtp-start-blocked", "track-unavailable", "fallback"].includes(action)) {
          state.realtimeHostStatusSeen = true;
          clearHostStatusWatchdog();
        }
        if (action === "track-unavailable") {
          state.realtimeTrackUnavailable = true;
          ["h265", "h264", "av1"].forEach((codec) => state.disabledRealtimeCodecs.add(codec));
          clearVideoPlaybackWatchdog();
          resetScreenVideo();
          requestScreenFallback(error || "realtime video track unavailable");
        }
        const statusSuffix = (status) => (status && status !== "active" ? `/${status}` : "");
        const hostMetrics = `${sentFPS ? ` · host ${sentFPS.toFixed(1)}fps` : ""}${sentKbps ? ` · ${Math.round(sentKbps)}kbps` : ""}${sentFrames ? ` · ${sentFrames} frames` : ""}${queueMs ? ` · queue ${Math.round(queueMs)}ms` : ""}${maxQueueMs ? ` · maxQ ${Math.round(maxQueueMs)}ms` : ""}${rawDrops ? ` · rawDrop ${rawDrops}` : ""}${encodedDrops ? ` · encDrop ${encodedDrops}` : ""}`;
        const hostRTP = `${action === "host-rtp" || rtpPackets ? ` · RTP ${rtpPackets}pkts` : ""}${rtpDeltaPackets ? ` · +${rtpDeltaPackets}pkts` : ""}${rtpKbps ? ` · ${Math.round(rtpKbps)}kbps RTP` : ""}${rtpDiscardedPackets ? ` · RTPdrop ${rtpDiscardedPackets}` : ""}${pairPackets ? ` · pair ${pairPackets}pkts` : ""}${pairState && pairState !== "unknown" ? ` · ${pairState}` : ""}`;
        const encoderDetails = `${phase ? ` · ${phase}` : ""}${trigger ? ` · ${trigger}` : ""}${encoder ? ` · ${encoder}` : ""}${ffmpeg ? ` · ${ffmpeg}` : ""}${width && height ? ` · ${width}x${height}` : ""}${error ? ` · ${error}` : ""}`;
        const networkDetails = `${lossPct ? ` · loss ${lossPct.toFixed(1)}%` : ""}${jitterMs ? ` · jitter ${Math.round(jitterMs)}ms` : ""}${nacks ? ` · NACK ${nacks}` : ""}${plis ? ` · PLI ${plis}` : ""}`;
        const fabricDetails = `${transportPrimary ? ` · ${transportPrimary}${statusSuffix(transportStatus)}` : ""}${encoderProvider ? ` · ${encoderProvider}${statusSuffix(encoderStatus)}` : ""}${captureProvider ? ` · ${captureProvider}${statusSuffix(captureStatus)}` : ""}`;
        const readinessDetails = `${videoPresent === null ? "" : ` · video=${videoPresent ? "yes" : "no"}`}${trackPresent === null ? "" : ` · track=${trackPresent ? "yes" : "no"}`}${streamPresent === null ? "" : ` · stream=${streamPresent ? "yes" : "no"}`}`;
        const details = `${profile}${fps ? ` · target ${fps}fps` : ""}${crf ? ` · CRF ${crf}` : ""}${maxEdge ? ` · edge ${maxEdge}` : ""}${maxrate ? ` · cap ${maxrate}kbps` : ""}${ageMs ? ` · ${ageMs}ms` : ""}${encoderDetails}${readinessDetails}${hostMetrics}${hostRTP}${networkDetails}${fabricDetails}`;
        const fabricCaps = [captureCaps, encoderCaps, transportCaps].filter(Boolean).join(" | ");
        if (fabricCaps && fabricCaps !== state.lastFabricCaps) {
          state.lastFabricCaps = fabricCaps;
          appendTransportLog(`Remote fabric caps: ${fabricCaps}`);
        }
        const codecLabel = transport === "h265" ? "H.265" : transport === "av1" ? "AV1" : transport === "h264" ? "H.264" : "Realtime video";
        appendTransportLog(`${codecLabel} ${action}: ${details}`);
        sessionScreenMetaEl.textContent = `${codecLabel} adaptive profile · ${details}`;
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
    const startedAt = nowMs();
    const packet = decodeScreenWirePacket(buffer);
    if (!packet) return;
    const codecLabel = packet.codec === SCREEN_WIRE_CODEC_RGBA ? "rgba" : packet.mimeType.replace("image/", "");
    if (packet.codec === SCREEN_WIRE_CODEC_RGBA) {
      drawRawScreenPacket(packet);
      logScreenFallbackStats(buffer, codecLabel, nowMs() - startedAt);
      return;
    }
    const blob = new Blob([packet.payload], { type: packet.mimeType });
    if (window.createImageBitmap) {
      const bitmap = await window.createImageBitmap(blob);
      try {
        if (!drawScreenBitmap(bitmap, packet)) updateScreenImageFromBlob(blob);
        logScreenFallbackStats(buffer, codecLabel, nowMs() - startedAt);
      } finally {
        if (bitmap && typeof bitmap.close === "function") bitmap.close();
      }
      return;
    }
    updateScreenImageFromBlob(blob);
    logScreenFallbackStats(buffer, codecLabel, nowMs() - startedAt);
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

  function logScreenFallbackStats(buffer, codecLabel, decodeMs) {
    const now = nowMs();
    state.screenFallbackFrames += 1;
    state.screenFallbackBytes += buffer instanceof ArrayBuffer ? buffer.byteLength : 0;
    if (!state.screenFallbackLastStatsAt) {
      state.screenFallbackLastStatsAt = now;
      return;
    }
    const elapsed = now - state.screenFallbackLastStatsAt;
    if (elapsed < 2000) return;
    const fps = state.screenFallbackFrames * 1000 / elapsed;
    const kbps = state.screenFallbackBytes * 8 / elapsed;
    appendTransportLog(`peer frame stats: ${codecLabel} fps=${fps.toFixed(1)} throughput=${kbps.toFixed(0)}kbps decode=${Math.round(decodeMs)}ms queue=${state.screenChunkAssemblies.size}`);
    state.screenFallbackFrames = 0;
    state.screenFallbackBytes = 0;
    state.screenFallbackLastStatsAt = now;
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
      if (options.peerOnly) return false;
      if (sendControlViaSessionSocket(payload, channelLabel, options)) return true;
      if (!options.silent) setSessionFeedback("error", "Control channel is not connected yet.");
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
    stopSessionSocketReconnect("session closed by viewer");
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
    closeSessionSocket({ stopReconnect: true });
    closeRealtimeSession();
    if (state.currentScreenObjectURL) {
      URL.revokeObjectURL(state.currentScreenObjectURL);
      state.currentScreenObjectURL = "";
    }
  });
  document.addEventListener("visibilitychange", () => {
    if (document.hidden || state.sessionSocketStopped || !state.sessionID || state.sessionSocket) return;
    state.sessionSocketReconnectAttempts = 0;
    openSessionSocket();
  });

  sessionScreenVideoEl.addEventListener("loadeddata", () => showScreenVideo("Realtime video"));
  sessionScreenVideoEl.addEventListener("playing", () => showScreenVideo("Realtime video"));
  sessionScreenVideoEl.addEventListener("error", () => {
    appendTransportLog("realtime video element reported a decode/playback error");
    if (retryRealtimeWithoutCodec("h265", "H.265 video element error")) return;
    resetScreenVideo();
    requestScreenFallback("realtime video element error");
  });
  sessionScreenVideoEl.addEventListener("resize", () => {
    if (isScreenVideoActive()) {
      updateScreenMeta("Realtime video", sessionScreenVideoEl.videoWidth || 0, sessionScreenVideoEl.videoHeight || 0);
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
    await loadRuntimeInfo();
    await ensureBrowserIdentity();
    try {
      await handleSessionUpdate(await fetchSession(), false);
    } catch (error) {
      if (error && error.status === 404) {
        stopSessionSocketReconnect("session not found");
      }
      throw error;
    }
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
