//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os/exec"
	"sync"
)

var startLocalHostUIOnce sync.Once

func startLocalHostUI(ctx context.Context, autoOpen bool) {
	startLocalHostUIOnce.Do(func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return
		}

		baseURL := "http://" + listener.Addr().String()
		localHostUI.setLocalUIURL(baseURL)

		mux := http.NewServeMux()
		mux.HandleFunc("/", handleLocalHostUIPage)
		mux.HandleFunc("/api/status", handleLocalHostUIStatus)
		mux.HandleFunc("/api/access", handleLocalHostUIAccess)

		server := &http.Server{Handler: mux}
		go func() {
			<-ctx.Done()
			_ = server.Shutdown(context.Background())
		}()
		go func() {
			_ = server.Serve(listener)
		}()

		if autoOpen {
			go func() {
				_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", baseURL).Start()
			}()
		}
	})
}

func handleLocalHostUIStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(localHostUI.snapshot())
}

func handleLocalHostUIPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, localHostUIHTML)
}

func handleLocalHostUIAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	setHostAccessEnabled(payload.Enabled)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(localHostUI.snapshot())
}

var localHostUIHTML = template.HTML(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>UnyDesk Host</title>
  <style>
    :root {
      --ink: #1d2142;
      --muted: #6a7196;
      --brand: #3539c2;
      --brand-deep: #1f23b5;
      --panel: #ffffff;
      --panel-soft: #f7f8ff;
      --line: rgba(34, 41, 102, 0.08);
      --bg:
        radial-gradient(circle at top left, rgba(63, 71, 191, 0.16), transparent 28%),
        linear-gradient(180deg, #f5f7ff, #edf1ff);
    }

    * { box-sizing: border-box; }

    body {
      margin: 0;
      min-height: 100vh;
      font-family: "Segoe UI", ui-sans-serif, system-ui, sans-serif;
      color: var(--ink);
      background: var(--bg);
    }

    .shell {
      width: min(1280px, calc(100vw - 40px));
      margin: 24px auto;
      padding: 40px;
      border: 1px solid var(--line);
      border-radius: 20px;
      background: rgba(255, 255, 255, 0.92);
      box-shadow: 0 24px 70px rgba(63, 71, 191, 0.12);
      backdrop-filter: blur(10px);
    }

    .topbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 24px;
      padding-bottom: 28px;
      border-bottom: 1px solid var(--line);
    }

    .brand {
      display: flex;
      align-items: center;
      gap: 16px;
      color: var(--brand);
    }

    .brand-mark {
      width: 64px;
      height: 64px;
      border-radius: 18px;
      display: grid;
      place-items: center;
      background: linear-gradient(180deg, #eef1ff, #e3e7ff);
      box-shadow: inset 0 0 0 1px rgba(53, 57, 194, 0.08);
      font-size: 1.9rem;
    }

    .brand strong {
      display: block;
      font-size: 3rem;
      line-height: 1;
      letter-spacing: -0.04em;
    }

    .hero {
      display: grid;
      grid-template-columns: minmax(0, 1.35fr) minmax(360px, 0.95fr);
      gap: 24px;
      margin-top: 28px;
    }

    .eyebrow {
      display: inline-block;
      margin: 0 0 16px;
      padding: 0.35rem 1rem;
      border-radius: 4px;
      background: #eef1ff;
      color: #4b4bb7;
      font-size: 0.72rem;
      font-weight: 700;
      letter-spacing: .12em;
      text-transform: uppercase;
    }

    h1 {
      margin: 0 0 12px;
      font-size: clamp(3rem, 6vw, 4.6rem);
      line-height: 0.94;
      letter-spacing: -0.06em;
    }

    .lede {
      margin: 0;
      max-width: 760px;
      color: var(--muted);
      font-size: 1.15rem;
      line-height: 1.7;
    }

    .stats-strip {
      margin-top: 28px;
      padding: 18px 22px;
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
      gap: 18px;
      border: 1px solid var(--line);
      border-radius: 16px;
      background: var(--panel);
      box-shadow: inset 0 1px 0 rgba(255,255,255,0.75);
    }

    .stat-kicker {
      display: block;
      margin-bottom: 8px;
      color: #a0a4bc;
      text-transform: uppercase;
      font-size: 0.78rem;
      letter-spacing: 0.08em;
      font-weight: 700;
    }

    .stat-value {
      display: block;
      font-size: 1.15rem;
      font-weight: 700;
      color: var(--ink);
    }

    .stat-value.brand {
      color: var(--brand-deep);
      font: 900 1.35rem/1.35 ui-monospace, SFMono-Regular, Menlo, monospace;
    }

    .connect-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 18px;
      margin-top: 24px;
    }

    .identity,
    .panel-soft {
      padding: 26px;
      border: 1px solid var(--line);
      border-radius: 18px;
      background: linear-gradient(180deg, #ffffff, #f9faff);
      box-shadow: 0 18px 45px rgba(63, 71, 191, 0.08);
    }

    .identity-label {
      margin-bottom: 18px;
      color: #6c7397;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      font-size: 0.92rem;
      font-weight: 700;
    }

    .identity-code,
    .code {
      color: var(--brand-deep);
      font: 900 1.65rem/1.25 ui-monospace, SFMono-Regular, Menlo, monospace;
      word-break: break-word;
    }

    .identity-note,
    .panel-soft p,
    .support {
      margin: 0;
      color: var(--muted);
      font-size: 0.95rem;
      line-height: 1.6;
    }

    .panel-soft {
      display: grid;
      gap: 18px;
      align-content: start;
      background: linear-gradient(180deg, #f9fbff, #f2f5ff);
    }

    .panel-head {
      display: grid;
      gap: 6px;
    }

    .panel-head strong {
      font-size: 1.45rem;
      line-height: 1.1;
    }

    .panel-grid {
      display: grid;
      gap: 12px;
    }

    .panel-row {
      margin-top: 24px;
      padding: 1rem 1.66rem 1rem;
      border-radius: 2px;
      background: #f7f8ff;
    }

    .panel-row-label {
      display: block;
      margin-bottom: 8px;
      color: #6c7397;
      font-size: 0.8rem;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      font-weight: 700;
    }

    .btn-row {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      margin-top: 10px;
    }

    a.btn,
    button.btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      min-width: 170px;
      padding: 0.95rem 1.35rem;
      border-radius: 10px;
      border: none;
      cursor: pointer;
      text-decoration: none;
      font-family: inherit;
      font-size: 1rem;
      font-weight: 700;
      transition: transform .18s ease, box-shadow .18s ease;
    }

    a.btn:hover,
    button.btn:hover {
      transform: translateY(-1px);
    }

    .btn-primary {
      color: #ffffff;
      background: linear-gradient(180deg, #4740d8, #2c22bf);
      box-shadow: 0 14px 30px rgba(52, 57, 194, 0.26);
    }

    .btn-secondary {
      color: var(--ink);
      background: #ffffff;
      box-shadow: inset 0 0 0 1px rgba(34, 41, 102, 0.08);
    }

    .footer {
      margin-top: 28px;
      display: flex;
      justify-content: space-between;
      gap: 16px;
      color: #9a9fb7;
      font-size: 0.92rem;
    }

    @media (max-width: 980px) {
      .shell {
        width: min(100vw - 24px, 980px);
        padding: 24px;
      }

      .topbar,
      .hero,
      .connect-grid,
      .stats-strip,
      .footer {
        grid-template-columns: 1fr;
        display: grid;
      }

      .brand strong {
        font-size: 2.35rem;
      }
    }
  </style>
</head>
<body>
  <main class="shell">
    <header class="topbar">
      <div class="brand">
        <div class="brand-mark">🖥️</div>
        <strong>UnyDesk Host</strong>
      </div>
    </header>

    <section class="hero">
      <div>
        <div class="eyebrow"># TRINITY Labs Host Companion</div>
        <h1>Ready Presence</h1>
        <p class="lede">
          A local companion surface for the outbound Windows host. The installed binary is the <strong>Host</strong> role,
          while your browser dashboard acts as the <strong>Client</strong> role and can initiate sessions only while host access stays enabled.
        </p>

        <div class="stats-strip">
          <div>
            <span class="stat-kicker">Access ID</span>
            <span class="stat-value brand" id="public-id">Loading…</span>
          </div>
          <div>
            <span class="stat-kicker">Role</span>
            <span class="stat-value" id="role-name">Host</span>
          </div>
          <div>
            <span class="stat-kicker">Client Access</span>
            <span class="stat-value" id="access-state">Enabled</span>
          </div>
          <div>
            <span class="stat-kicker">Windows Rights</span>
            <span class="stat-value" id="admin-state">Checking…</span>
          </div>
          <div>
            <span class="stat-kicker">Runtime</span>
            <span class="stat-value" id="connection-state">Starting</span>
          </div>
        </div>

        <div class="connect-grid">
          <div class="identity">
            <div class="identity-label">Install ID</div>
            <div class="identity-code" id="install-id">Loading…</div>
            <p class="identity-note" id="install-note">This install identity is the central pairing key propagated into the host binary.</p>
          </div>
          <div class="identity">
            <div class="identity-label">Server</div>
            <div class="identity-code" id="server-url">Loading…</div>
            <p class="identity-note" id="heartbeat-note">Waiting for the first heartbeat.</p>
          </div>
        </div>
      </div>

      <aside class="panel-soft">
        <div class="panel-head">
          <strong>Host Runtime</strong>
          <p id="connection-note">Preparing host runtime.</p>
        </div>

        <div class="panel-grid">
          <div class="panel-row">
            <span class="panel-row-label">Host ID</span>
            <div class="code" id="host-id">—</div>
          </div>
          <div class="panel-row">
            <span class="panel-row-label">Host profile</span>
            <div class="code" id="host-profile">—</div>
          </div>
          <div class="panel-row">
            <span class="panel-row-label">Last session</span>
            <div class="code" id="last-session">No session dispatched yet.</div>
          </div>
          <div class="panel-row">
            <span class="panel-row-label">Last error</span>
            <div class="support" id="last-error">No runtime error reported.</div>
          </div>
        </div>

        <div class="btn-row">
          <a class="btn btn-primary" id="account-link" href="#" target="_blank" rel="noopener">Open account</a>
          <button class="btn btn-secondary" id="access-toggle" type="button">Pause host access</button>
          <a class="btn btn-secondary" id="local-link" href="#" target="_blank" rel="noopener">Refresh local UI</a>
        </div>
      </aside>
    </section>

    <footer class="footer">
      <span>Host role runs locally. Client role lives in the browser dashboard.</span>
      <span>Same install identity, same host pairing, same browser-first model.</span>
    </footer>
  </main>

  <script>
    async function loadStatus() {
      const response = await fetch('/api/status', { cache: 'no-store' });
      if (!response.ok) throw new Error('status fetch failed');
      return response.json();
    }

    function setText(id, value) {
      const node = document.getElementById(id);
      if (!node) return;
      node.textContent = value || '—';
    }

    function applyStatus(status) {
      setText('public-id', status.public_id || 'Unavailable');
      setText('role-name', status.role || 'Host');
      setText('access-state', status.access_state || (status.access_enabled ? 'Enabled' : 'Paused'));
      setText('admin-state', status.admin ? 'Administrator' : 'Standard user');
      setText('connection-state', status.connection_state || 'Unknown');
      setText('install-id', status.install_id || 'Unavailable');
      setText('server-url', status.server_url || 'Unavailable');
      setText('connection-note', status.connection_note || 'No detail available.');
      setText('heartbeat-note', status.last_heartbeat_at
        ? 'Last heartbeat: ' + new Date(status.last_heartbeat_at).toLocaleString()
        : 'Waiting for the first heartbeat.');
      setText('host-id', status.host_id || 'Waiting for registration');
      setText('host-profile', (status.hostname || 'unknown-host') + ' · ' + (status.version || '—'));
      setText('last-session', status.last_session_id
        ? status.last_session_id + (status.last_session_meta ? ' · ' + status.last_session_meta : '')
        : 'No session dispatched yet.');
      setText('last-error', status.last_error || 'No runtime error reported.');
      setText('install-note', status.connected
        ? (status.access_enabled
            ? 'This install identity is active and currently exposed as a live Host role for Client sessions.'
            : 'This install identity is active, but the Host role is paused for new Client sessions.')
        : 'This install identity stays stable even before the host reconnects.');

      const accountLink = document.getElementById('account-link');
      if (accountLink && status.account_url) {
        accountLink.href = status.account_url;
      }

      const localLink = document.getElementById('local-link');
      if (localLink && status.local_ui_url) {
        localLink.href = status.local_ui_url;
      }

      const accessToggle = document.getElementById('access-toggle');
      if (accessToggle) {
        accessToggle.textContent = status.access_enabled ? 'Pause host access' : 'Enable host access';
        accessToggle.dataset.nextState = status.access_enabled ? '0' : '1';
      }
    }

    async function setAccessEnabled(enabled) {
      const response = await fetch('/api/access', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: !!enabled }),
      });
      if (!response.ok) throw new Error('access update failed');
      return response.json();
    }

    async function refresh() {
      try {
        applyStatus(await loadStatus());
      } catch (_error) {}
    }

    document.getElementById('access-toggle').addEventListener('click', async (event) => {
      const button = event.currentTarget;
      const enabled = button.dataset.nextState === '1';
      button.disabled = true;
      try {
        applyStatus(await setAccessEnabled(enabled));
      } catch (_error) {
      } finally {
        button.disabled = false;
      }
    });

    refresh();
    window.setInterval(refresh, 1000);
  </script>
</body>
</html>`)
