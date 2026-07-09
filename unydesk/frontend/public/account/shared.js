export function formatDateTime(value) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString();
}

export function relativeTime(value) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  const diff = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

export function escapeHTML(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

export function hostOSIcon(host) {
  const osName = String(host && host.os ? host.os : "").toLowerCase();
  if (osName.includes("windows")) return "fa-brands fa-windows";
  if (osName.includes("darwin") || osName.includes("mac") || osName.includes("osx")) return "fa-brands fa-apple";
  if (osName.includes("linux") || osName.includes("alpine")) return "fa-brands fa-linux";
  return "fa-solid fa-desktop";
}

export function hostPlatformLabel(host) {
  const os = hostOSLabel(host && host.os);
  const arch = hostArchLabel(host && host.arch);
  if (os === "?" && arch === "?") return "?";
  if (os === "?") return arch;
  if (arch === "?") return os;
  return `${os} ${arch}`;
}

export function hostOSLabel(value) {
  const raw = String(value || "").trim();
  const osName = raw.toLowerCase();
  if (!osName) return "?";
  const linuxDistro = linuxDistributionLabel(osName);
  if (linuxDistro) return linuxDistro;
  if (osName.includes("windows")) return "Windows";
  if (osName.includes("darwin") || osName.includes("mac") || osName.includes("osx")) return "macOS";
  if (osName.includes("linux")) return "Linux";
  return raw;
}

export function linuxDistributionLabel(source) {
  const distributions = [
    [/ubuntu/, "Ubuntu Linux"],
    [/debian/, "Debian Linux"],
    [/fedora/, "Fedora Linux"],
    [/\brhel\b/, "Red Hat Enterprise Linux"],
    [/red\s*hat/, "Red Hat Enterprise Linux"],
    [/centos/, "CentOS Linux"],
    [/rocky/, "Rocky Linux"],
    [/almalinux/, "AlmaLinux"],
    [/opensuse/, "openSUSE Linux"],
    [/\bsuse\b/, "SUSE Linux"],
    [/\barch\b/, "Arch Linux"],
    [/manjaro/, "Manjaro Linux"],
    [/alpine/, "Alpine Linux"],
    [/gentoo/, "Gentoo Linux"],
    [/linux\s*mint/, "Linux Mint"],
    [/\bmint\b/, "Linux Mint"],
    [/pop[!_\s-]*os/, "Pop!_OS"],
  ];
  const match = distributions.find(([pattern]) => pattern.test(source));
  return match ? match[1] : "";
}

export function hostArchLabel(value) {
  const arch = String(value || "").trim().toLowerCase();
  if (!arch) return "?";
  if (["amd64", "x86_64", "x64"].includes(arch)) return "64-bits";
  if (["386", "i386", "i686", "x86"].includes(arch)) return "32-bits";
  if (["arm64", "aarch64"].includes(arch)) return "ARM64";
  return value;
}

export function hostRoleLabel(host) {
  return String(host && host.role ? host.role : "host").trim().toLowerCase() === "client" ? "Client" : "Host";
}

export function hostAccessEnabled(host) {
  return !host || host.access_enabled !== false;
}

export function hostReadyForSession(host) {
  const status = String(host && host.status ? host.status : "").trim().toLowerCase();
  return status === "online" && hostAccessEnabled(host);
}

export function hostStatusLabel(host) {
  const status = String(host && host.status ? host.status : "unknown").trim().toLowerCase();
  if (!hostAccessEnabled(host)) return status === "online" ? "Paused" : "Access paused";
  if (status === "online") return "Online";
  if (status === "paused") return "Paused";
  if (status === "offline") return "Offline";
  return status ? `${status[0].toUpperCase()}${status.slice(1)}` : "Unknown";
}

export function hostStatusClass(host) {
  const status = hostStatusLabel(host).trim().toLowerCase();
  if (status === "online") return "is-online";
  if (status === "paused" || status.includes("paused")) return "is-paused";
  return "is-offline";
}

export function hostLastSeenLabel(host) {
  const value = host && host.last_seen_at;
  if (!value) return "No heartbeat yet";
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getUTCFullYear() < 2020) return "No heartbeat yet";
  return `Last seen ${relativeTime(value)}`;
}

export function hostAvailabilityLine(host) {
  if (hostReadyForSession(host)) return "Ready for remote sessions";
  if (!hostAccessEnabled(host)) return `Access paused · ${hostLastSeenLabel(host)}`;
  const status = String(host && host.status ? host.status : "").trim().toLowerCase();
  if (status === "paused") return `Paused by host · ${hostLastSeenLabel(host)}`;
  if (status === "online") return "Online, waiting for access approval";
  return hostLastSeenLabel(host);
}

export function userInitials(user) {
  const avatar = user && typeof user.avatar === "string" ? user.avatar.trim() : "";
  if (avatar) return avatar.slice(0, 2).toUpperCase();
  const source = (user && (user.display_name || user.email)) || "UnyDesk";
  return source
    .split(/[\s@._-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0].toUpperCase())
    .join("") || "U";
}
