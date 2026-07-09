package main

import (
	"fmt"
	"html/template"
	"net/http"
)

func handleLocalHostUIPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, localHostUIHTML)
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
          publishes the current access password on <strong>this machine only</strong>, and keeps the remote browser dashboard in the <strong>Client</strong> role.
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
            <div class="identity-label">Session Password</div>
            <div class="identity-code" id="access-password">Loading…</div>
            <p class="identity-note" id="heartbeat-note">Generated locally and shared only from this host browser.</p>
            <div class="btn-row">
              <button class="btn btn-secondary" id="copy-password" type="button">Copy password</button>
              <button class="btn btn-secondary" id="rotate-password" type="button">Generate new password</button>
            </div>
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
            <span class="panel-row-label">Server Route</span>
            <div class="code" id="server-url">—</div>
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

      <div class="panel-head" id="provision-panel" style="margin-top:18px;">
        <strong>Provision Host</strong>
        <p>Sign in once to store a local provisioning token instead of keeping your account password inside the host.</p>
      </div>

      <div class="panel-grid" id="provision-fields" style="margin-bottom:14px;">
        <label class="field">
          <span class="panel-row-label">Account email</span>
          <input class="auth-input" id="provision-email" type="email" autocomplete="username email" placeholder="admin@example.com">
        </label>
        <label class="field">
          <span class="panel-row-label">Account password</span>
          <div class="password-wrap">
            <input class="auth-input" id="provision-password" type="password" autocomplete="current-password" placeholder="Current account password">
            <button class="btn btn-secondary password-eye" id="toggle-provision-password" type="button" aria-label="Show password" title="Show password">Eye</button>
          </div>
        </label>
      </div>

      <div class="btn-row" style="margin-bottom:8px;">
        <button class="btn btn-primary" id="provision-submit" type="button">Sign in and provision</button>
      </div>
      <div class="support" id="provision-status">Provisioning stores a bearer token locally and avoids persisting the account password.</div>

        <div class="btn-row">
          <a class="btn btn-primary" id="account-link" href="#" target="_blank" rel="noopener">Open account</a>
          <button class="btn btn-secondary" id="access-toggle" type="button">Pause host access</button>
          <a class="btn btn-secondary" id="local-link" href="#" target="_blank" rel="noopener">Refresh local UI</a>
        </div>
      </aside>
    </section>

    <footer class="footer">
      <span>Host role runs locally. Client role lives in the browser dashboard.</span>
      <span>Same install identity, same password surfaced locally, same lightweight broker model.</span>
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
      setText('access-password', status.access_password || 'Unavailable');
      setText('role-name', status.role || 'Host');
      setText('access-state', status.access_state || (status.access_enabled ? 'Enabled' : 'Paused'));
      setText('admin-state', status.admin ? 'Administrator' : 'Standard user');
      setText('connection-state', status.connection_state || 'Unknown');
      setText('install-id', status.install_id || 'Unavailable');
      setText('server-url', status.server_url || 'Unavailable');
      setText('connection-note', status.connection_note || 'No detail available.');
      setText('heartbeat-note', status.last_heartbeat_at
        ? 'Last heartbeat: ' + new Date(status.last_heartbeat_at).toLocaleString() + ' · Share this password only with the intended client.'
        : 'Generated locally and shared only from this host browser.');
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
        : (status.provisioned
            ? 'This install identity is provisioned locally and waiting to reconnect.'
            : 'This install identity stays stable even before the host reconnects.'));

      const provisionPanel = document.getElementById('provision-panel');
      const provisionFields = document.getElementById('provision-fields');
      const provisionSubmit = document.getElementById('provision-submit');
      const provisionStatus = document.getElementById('provision-status');
      const showProvisioning = !!status.server_url && !status.provisioned;
      if (provisionPanel) provisionPanel.style.display = showProvisioning ? '' : 'none';
      if (provisionFields) provisionFields.style.display = showProvisioning ? '' : 'none';
      if (provisionSubmit) provisionSubmit.style.display = showProvisioning ? '' : 'none';
      if (provisionStatus && !showProvisioning) {
        provisionStatus.textContent = 'Bootstrap token already loaded locally. This host can wait silently for remote access requests.';
        provisionStatus.style.color = '#1f7a38';
      }
      if (provisionStatus) {
        provisionStatus.style.display = showProvisioning ? '' : '';
      }

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

    async function rotatePassword() {
      const response = await fetch('/api/password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      if (!response.ok) throw new Error('password rotate failed');
      return response.json();
    }

    async function provisionHost(email, password) {
      const response = await fetch('/api/provision', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });
      if (!response.ok) {
        const message = await response.text();
        throw new Error(message || 'Host provisioning failed');
      }
      return response.json();
    }

    async function copyPassword() {
      const text = (document.getElementById('access-password')?.textContent || '').trim();
      if (!text || text === 'Unavailable' || text === 'Loading…') return;
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(text);
        return;
      }
      const input = document.createElement('textarea');
      input.value = text;
      input.setAttribute('readonly', '');
      input.style.position = 'absolute';
      input.style.left = '-9999px';
      document.body.appendChild(input);
      input.select();
      document.execCommand('copy');
      document.body.removeChild(input);
    }

    async function refresh() {
      try {
        applyStatus(await loadStatus());
      } catch (_error) {}
    }

    function setProvisionStatus(message, ok) {
      const node = document.getElementById('provision-status');
      if (!node) return;
      node.textContent = message;
      node.style.color = ok ? '#1f7a38' : '';
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

    document.getElementById('copy-password').addEventListener('click', async (event) => {
      const button = event.currentTarget;
      button.disabled = true;
      try {
        await copyPassword();
      } catch (_error) {
      } finally {
        window.setTimeout(() => {
          button.disabled = false;
        }, 180);
      }
    });

    document.getElementById('rotate-password').addEventListener('click', async (event) => {
      const button = event.currentTarget;
      button.disabled = true;
      try {
        applyStatus(await rotatePassword());
      } catch (_error) {
      } finally {
        button.disabled = false;
      }
    });

    document.getElementById('toggle-provision-password').addEventListener('click', () => {
      const input = document.getElementById('provision-password');
      const button = document.getElementById('toggle-provision-password');
      if (!input || !button) return;
      const show = input.type === 'password';
      input.type = show ? 'text' : 'password';
      button.textContent = show ? 'Hide' : 'Eye';
      button.setAttribute('aria-label', show ? 'Hide password' : 'Show password');
      button.setAttribute('title', show ? 'Hide password' : 'Show password');
    });

    document.getElementById('provision-submit').addEventListener('click', async (event) => {
      const button = event.currentTarget;
      const email = (document.getElementById('provision-email')?.value || '').trim();
      const password = (document.getElementById('provision-password')?.value || '').trim();
      if (!email || !password) {
        setProvisionStatus('Enter the account email and password first.', false);
        return;
      }
      button.disabled = true;
      setProvisionStatus('Provisioning host access...', false);
      try {
        applyStatus(await provisionHost(email, password));
        document.getElementById('provision-password').value = '';
        setProvisionStatus('Provisioning token stored locally. The host can now connect without persisting the account password.', true);
      } catch (error) {
        setProvisionStatus((error && error.message) || 'Host provisioning failed.', false);
      } finally {
        button.disabled = false;
      }
    });

    refresh();
    window.setInterval(refresh, 1000);
  </script>
</body>
</html>`)
