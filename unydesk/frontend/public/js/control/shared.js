export const SCREEN_WIRE_HEADER_BYTES = 36;
export const SCREEN_WIRE_CODEC_WEBP = 2;
export const SCREEN_WIRE_CODEC_PNG = 3;
export const SCREEN_WIRE_CODEC_RGBA = 4;
export const SCREEN_WIRE_KIND_KEYFRAME = 1;
export const SCREEN_WIRE_KIND_PATCH = 2;
export const SCREEN_CHUNK_MAGIC = "USDT";
export const SCREEN_CHUNK_HEADER_BYTES = 20;
export const SCREEN_CHUNK_MAX_ASSEMBLIES = 3;
export const SCREEN_CHUNK_ASSEMBLY_TIMEOUT_MS = 350;
export const SESSION_SOCKET_RECONNECT_BASE_MS = 1000;
export const SESSION_SOCKET_RECONNECT_MAX_MS = 30000;
export const SESSION_SOCKET_RECONNECT_MAX_ATTEMPTS = 8;
export const SESSION_SOCKET_HIDDEN_RECONNECT_MS = 60000;
export const VIDEO_PLAYBACK_TIMEOUT_MS = 4200;
export const RTC_STATS_INTERVAL_MS = 1500;
export const ICE_OFFER_GATHER_TIMEOUT_MS = 800;
export const RTC_VIDEO_STALL_TIMEOUT_MS = 4500;
export const STALE_ANSWER_RECOVERY_MS = 5000;
export const STALE_ANSWER_MAX_RECOVERIES = 2;
export const ANSWER_WATCHDOG_MS = 2200;
export const ANSWER_WATCHDOG_MAX_ATTEMPTS = 4;
export const CODEC_PROMOTION_DELAY_MS = 8000;
export const CODEC_PROMOTION_RECHECK_MS = 3000;
export const CODEC_PROMOTION_MIN_DECODED_FRAMES = 90;
export const VIDEO_FRAME_LOG_INTERVAL_MS = 2000;
export const SCREEN_FALLBACK_RETRY_MS = 800;
export const SCREEN_FALLBACK_MAX_ATTEMPTS = 5;
export const REMOTE_CURSOR_ARROW = 'url("data:image/svg+xml,%3Csvg xmlns=\'http://www.w3.org/2000/svg\' width=\'20\' height=\'20\' viewBox=\'0 0 20 20\'%3E%3Cpath d=\'M3 2l10 10H8.6l2.5 5.1-1.8.9-2.5-5.1L4 15.6V2z\' fill=\'%23000000\'/%3E%3Cpath d=\'M3.6 3.3v10.85l2.83-2.81h.39l2.34 4.8.78-.39-2.34-4.8v-.54h3.54L3.6 3.3z\' fill=\'%23ffffff\' fill-opacity=\'.18\'/%3E%3C/svg%3E") 3 2, auto';

export function nowMs() {
  return window.performance && typeof window.performance.now === "function" ? window.performance.now() : Date.now();
}

export function elapsedMs(startedAt) {
  if (!startedAt) return 0;
  return Math.max(0, Math.round(nowMs() - startedAt));
}

export function formatBytes(value) {
  const bytes = Number(value || 0);
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

export function arrayBufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let index = 0; index < bytes.length; index += 8192) {
    const chunk = bytes.subarray(index, index + 8192);
    binary += String.fromCharCode.apply(null, Array.from(chunk));
  }
  return window.btoa(binary);
}

export function normalizeIceServerURLs(urls) {
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

export function normalizeIceServers(servers) {
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

export function describeIceServers(servers) {
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

export function codecNameForMime(mimeType) {
  const value = String(mimeType || "").toLowerCase();
  if (value.includes("h265") || value.includes("hevc")) return "h265";
  if (value.includes("av1")) return "av1";
  if (value.includes("h264")) return "h264";
  if (value.includes("vp9")) return "vp9";
  if (value.includes("vp8")) return "vp8";
  return "";
}

export function videoCodecOrderFromSDP(sdp) {
  const lines = String(sdp || "").split(/\r?\n/);
  const payloadOrder = [];
  const payloadCodecs = new Map();
  let inVideo = false;
  lines.forEach((line) => {
    if (line.startsWith("m=")) {
      inVideo = line.startsWith("m=video");
      if (inVideo) {
        line.split(/\s+/).slice(3).forEach((payload) => payloadOrder.push(payload));
      }
      return;
    }
    if (!inVideo) return;
    const match = line.match(/^a=rtpmap:(\d+)\s+([^/]+)/i);
    if (match) {
      const codec = codecNameForMime(`video/${match[2]}`);
      if (codec) payloadCodecs.set(match[1], codec);
    }
  });
  const ordered = [];
  const seen = new Set();
  payloadOrder.forEach((payload) => {
    const codec = payloadCodecs.get(payload);
    if (!codec || seen.has(codec)) return;
    seen.add(codec);
    ordered.push(codec);
  });
  return ordered;
}

export function describeVideoCodecsFromSDP(sdp) {
  const orderedCodecs = videoCodecOrderFromSDP(sdp);
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
    .join(", ") || (orderedCodecs.length ? orderedCodecs.join(", ") : "none");
}

export function waitForICEGatheringComplete(peerConnection, timeoutMs = ICE_OFFER_GATHER_TIMEOUT_MS) {
  if (!peerConnection || peerConnection.iceGatheringState === "complete") {
    return Promise.resolve("complete");
  }
  return new Promise((resolve) => {
    let settled = false;
    let timer = null;
    const finish = (reason) => {
      if (settled) return;
      settled = true;
      if (timer) window.clearTimeout(timer);
      peerConnection.removeEventListener("icegatheringstatechange", onChange);
      resolve(reason);
    };
    const onChange = () => {
      if (peerConnection.iceGatheringState === "complete") {
        finish("complete");
      }
    };
    peerConnection.addEventListener("icegatheringstatechange", onChange);
    timer = window.setTimeout(() => finish("timeout"), Math.max(50, timeoutMs));
  });
}

export function codecPreferenceRank(codecName, codec) {
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

export function uniqueCodecNames(codecs) {
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

export function countSDPCandidates(sdp) {
  return String(sdp || "").split(/\r?\n/).filter((line) => line.startsWith("a=candidate:")).length;
}

export function describeICECandidate(candidate) {
  const raw = String((candidate && candidate.candidate) || "");
  const type = raw.match(/\btyp\s+(\S+)/i);
  const protocol = raw.match(/\s(udp|tcp)\s/i);
  return `${type ? type[1] : "unknown"} ${protocol ? protocol[1].toLowerCase() : ""}`.trim();
}
