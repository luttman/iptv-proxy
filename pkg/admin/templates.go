/*
 * Iptv-Proxy is a project to proxyfie an m3u file and to proxyfie an Xtream iptv service (client API).
 * Copyright (C) 2020  Pierre-Emmanuel Jacquier
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package admin

import "html/template"

// themeInit runs before any styling is parsed so the saved theme
// (or the dark default) applies with no flash of the wrong theme.
const themeInit = `<script>(function(){var t=localStorage.getItem('iptvproxy-theme')||'dark';document.documentElement.setAttribute('data-theme',t);})();</script>`

const themeToggle = `
<button class="theme-toggle" type="button" title="Toggle light/dark theme" onclick="(function(){var h=document.documentElement;var next=h.getAttribute('data-theme')==='light'?'dark':'light';localStorage.setItem('iptvproxy-theme',next);h.setAttribute('data-theme',next);})()">
  <span class="icon-sun">&#9728;&#65039;</span><span class="icon-moon">&#127769;</span>
</button>
`

// styleBlock is the shared visual system: a "broadcast console" look
// (near-black, a signal-green live indicator, an indigo accent for
// interactive elements) built for scanning many rows/cards at once —
// this admin manages a fleet of upstream providers, not a handful.
const styleBlock = `<meta name="viewport" content="width=device-width, initial-scale=1">
<title>iptv-proxy admin</title>` + themeInit + `
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@500;600;700&family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
<style>
  :root {
    /* Dark theme (default) */
    --bg: #0a0d12;
    --panel: #12151d;
    --panel-2: #171b26;
    --border: #232838;
    --text: #e7e9f0;
    --muted: #7d8598;
    --accent: #6c8cff;
    --accent-dark: #8ba0ff;
    --accent-dim: rgba(108, 140, 255, 0.14);
    --signal: #35e0ab;
    --signal-dim: rgba(53, 224, 171, 0.16);
    --danger: #ff5c72;
    --danger-dark: #ff7c8f;
    --danger-dim: rgba(255, 92, 114, 0.14);
    --danger-border: #5a2c34;
    --warn: #ffb454;
    --badge-bg: #1c2338;
    --radius: 12px;
    --radius-sm: 8px;
    --font-display: 'Space Grotesk', ui-sans-serif, system-ui, sans-serif;
    --font-body: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    --font-mono: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  }
  html[data-theme="light"] {
    --bg: #f3f4f8;
    --panel: #ffffff;
    --panel-2: #f6f7fb;
    --border: #e2e5ee;
    --text: #171a23;
    --muted: #667085;
    --accent: #4a5fe0;
    --accent-dark: #3848b8;
    --accent-dim: rgba(74, 95, 224, 0.10);
    --signal: #0f9d72;
    --signal-dim: rgba(15, 157, 114, 0.12);
    --danger: #d3405a;
    --danger-dark: #b12f47;
    --danger-dim: rgba(211, 64, 90, 0.10);
    --danger-border: #f6c9d1;
    --warn: #b9760a;
    --badge-bg: #eef0fb;
  }
  * { box-sizing: border-box; }
  html { -webkit-font-smoothing: antialiased; -moz-osx-font-smoothing: grayscale; }
  html, body { transition: background-color 0.15s ease, color 0.15s ease; }
  .theme-toggle {
    position: fixed; top: 1rem; right: 1.5rem; z-index: 10;
    background: var(--panel); border: 1px solid var(--border); border-radius: 999px;
    width: 2.75rem; height: 2.75rem; cursor: pointer; font-size: 1rem; line-height: 1;
    transition: border-color 0.15s ease, transform 0.15s ease;
  }
  .theme-toggle:hover { border-color: var(--accent); }
  .theme-toggle:active { transform: scale(0.96); }
  .theme-toggle .icon-moon { display: none; }
  html[data-theme="light"] .theme-toggle .icon-sun { display: none; }
  html[data-theme="light"] .theme-toggle .icon-moon { display: inline; }
  body {
    font-family: var(--font-body);
    background: radial-gradient(circle at 50% -20%, var(--accent-dim), transparent 38rem), var(--bg);
    color: var(--text);
    margin: 0;
    padding: 0 1.5rem 3rem;
  }
  h1, h2, h3 { font-family: var(--font-display); letter-spacing: -0.01em; }
  .wrap { max-width: 1080px; margin: 0 auto; }
  header.topbar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 1.25rem 0; margin-bottom: 1.5rem; border-bottom: 1px solid var(--border);
  }
  header.topbar h1 { font-size: 1.2rem; font-weight: 700; margin: 0; }
  .brand-dot {
    display: inline-block; width: 0.6rem; height: 0.6rem; margin-right: 0.5rem;
    border-radius: 50%; background: var(--signal);
    animation: pulse 2.4s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { box-shadow: 0 0 0 3px var(--signal-dim); }
    50% { box-shadow: 0 0 0 7px var(--signal-dim); }
  }
  header.topbar nav a { color: var(--muted); text-decoration: none; font-size: 0.9rem; margin-right: 1rem; }
  header.topbar nav a:hover { color: var(--text); }
  .login-shell {
    max-width: 360px; margin: 4rem auto 0; padding: 2rem;
    background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius);
  }
  .login-shell h1 { font-size: 1.3rem; text-align: center; margin-top: 0; }
  .login-shell .subtitle { margin: -0.45rem 0 1.5rem; color: var(--muted); font-size: 0.82rem; text-align: center; font-family: var(--font-body); }
  .stats { display: flex; gap: 0.85rem; margin-bottom: 1.5rem; flex-wrap: wrap; }
  .stat-card {
    flex: 1; min-width: 145px; background: var(--panel); border: 1px solid var(--border);
    border-radius: var(--radius); padding: 0.9rem 1.1rem;
    box-shadow: 0 1px 2px rgba(0,0,0,0.08);
  }
  .stat-card .value { font-family: var(--font-display); font-size: 1.65rem; font-weight: 700; color: var(--accent); line-height: 1; font-variant-numeric: tabular-nums; }
  .stat-card .label { font-size: 0.78rem; color: var(--muted); margin-top: 0.3rem; }
  .panel {
    background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius);
    padding: 1.25rem 1.5rem; margin-bottom: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.08);
  }
  .panel-header { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; margin-bottom: 0.75rem; flex-wrap: wrap; }
  .panel-header h2 { font-size: 1.05rem; font-weight: 600; margin: 0; }
  .panel-header h3 { font-size: 0.92rem; font-weight: 600; margin: 1.1rem 0 0.4rem; }
  .panel-tools { display: flex; align-items: center; gap: 0.75rem; }
  .filter-input {
    width: 13rem; max-width: 100%; margin: 0; padding: 0.4rem 0.7rem;
    border: 1px solid var(--border); border-radius: var(--radius-sm); font-size: 0.82rem;
    background: var(--panel-2); color: var(--text);
  }
  table { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
  .table-scroll { overflow-x: auto; margin: 0 -0.5rem; padding: 0 0.5rem; }
  .table-scroll.scroll-bounded { max-height: 23rem; overflow-y: auto; }
  th, td { text-align: left; padding: 0.55rem 0.5rem; border-bottom: 1px solid var(--border); }
  th { color: var(--muted); font-weight: 600; font-size: 0.72rem; text-transform: uppercase; letter-spacing: 0.04em; }
  tr:last-child td { border-bottom: none; }
  .empty-row td { color: var(--muted); text-align: center; padding: 1.5rem 0; }
  .badge {
    display: inline-block; padding: 0.15rem 0.55rem; border-radius: 999px;
    background: var(--badge-bg); color: var(--accent-dark); font-size: 0.75rem; font-weight: 600;
  }
  .mono { font-family: var(--font-mono); font-size: 0.82rem; color: var(--muted); }
  .urls { white-space: pre-line; overflow-wrap: anywhere; min-width: 13rem; }
  .health-address { max-width: 20rem; overflow-wrap: anywhere; }
  .status-pill { display: inline-flex; align-items: center; gap: 0.4rem; font-size: 0.78rem; font-weight: 700; }
  .status-pill::before { content: ""; width: 0.55rem; height: 0.55rem; border-radius: 50%; background: currentColor; }
  .status-up { color: var(--signal); }
  .status-up::before { animation: pulse-dot 2.4s ease-in-out infinite; }
  @keyframes pulse-dot {
    0%, 100% { box-shadow: 0 0 0 2px var(--signal-dim); }
    50% { box-shadow: 0 0 0 5px var(--signal-dim); }
  }
  .status-down { color: var(--danger); }
  .health-history { display: grid; grid-template-columns: repeat(60, minmax(1px, 1fr)); gap: 2px; width: 11rem; height: 1.4rem; }
  .health-bar { min-width: 1px; border-radius: 2px; background: var(--signal); opacity: 0.9; }
  .health-bar.down { background: var(--danger); }
  .health-time, .ping, .uptime { font-variant-numeric: tabular-nums; white-space: nowrap; }
  form.inline { display: inline; }
  .actions a, .actions button { margin-right: 0.4rem; }
  .error {
    color: var(--danger); background: var(--danger-dim); border: 1px solid var(--danger-border);
    border-radius: var(--radius); padding: 0.6rem 0.9rem; margin-bottom: 1rem; font-size: 0.88rem;
  }
  label { display: block; margin-top: 0.9rem; font-weight: 600; font-size: 0.85rem; }
  input, select, textarea {
    padding: 0.5rem 0.6rem; width: 100%; max-width: 360px; margin-top: 0.3rem;
    border: 1px solid var(--border); border-radius: 6px; font-size: 0.9rem; background: var(--panel-2); color: var(--text);
    font-family: var(--font-body);
  }
  textarea { min-height: 6rem; resize: vertical; font-family: var(--font-mono); }
  input:focus, select:focus, textarea:focus { outline: none; border-color: var(--accent); }
  .btn {
    display: inline-flex; align-items: center; justify-content: center; min-height: 2.5rem;
    margin-top: 1.25rem; padding: 0.55rem 1.1rem; cursor: pointer;
    border: none; border-radius: 6px; font-size: 0.88rem; font-weight: 600;
    background: var(--accent); color: white;
    transition: background-color 0.15s ease, transform 0.15s ease;
  }
  .btn:hover { background: var(--accent-dark); }
  .btn:active, .btn-link:active { transform: scale(0.97); }
  .btn-sm { margin-top: 0; min-height: 2.25rem; padding: 0.3rem 0.7rem; font-size: 0.8rem; }
  .btn-danger { background: var(--danger); }
  .btn-danger:hover { background: var(--danger-dark); }
  .btn-link {
    display: inline-flex; align-items: center; justify-content: center; min-height: 2.25rem;
    padding: 0.3rem 0.7rem; font-size: 0.8rem; font-weight: 600;
    color: var(--accent); text-decoration: none; border: 1px solid var(--border); border-radius: 6px;
    transition: background-color 0.15s ease, transform 0.15s ease;
  }
  .btn-link:hover { background: var(--badge-bg); }
  .add-link { font-size: 0.88rem; font-weight: 600; color: var(--accent); text-decoration: none; white-space: nowrap; }
  .help { max-width: 360px; margin: 0.35rem 0 0; color: var(--muted); font-size: 0.78rem; line-height: 1.45; text-wrap: pretty; }
  .link-btn { background: none; border: none; cursor: pointer; padding: 0; }
  dialog { border: none; border-radius: var(--radius); padding: 1.5rem; background: var(--panel); color: var(--text); max-width: 420px; width: 90%; max-height: 85vh; overflow-y: auto; box-shadow: 0 12px 40px rgba(0,0,0,0.35); }
  dialog.dialog-wide { max-width: 720px; }
  dialog h3 { margin-bottom: 0.4rem; }
  dialog::backdrop { background: rgba(0,0,0,0.55); }
  dialog h2 { margin-top: 0; }
  .dialog-actions { display: flex; align-items: center; justify-content: flex-end; gap: 0.6rem; margin-top: 1.25rem; }
  .dialog-actions .btn { margin-top: 0; }
  :focus-visible { outline: 3px solid var(--accent-dim); outline-offset: 2px; }

  /* Provider cards: a 20-upstream fleet reads better as a scannable
     grid than as one giant table row stuffed with nested lists. */
  .code-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 0.85rem; }
  .code-card {
    background: var(--panel-2); border: 1px solid var(--border); border-radius: var(--radius);
    padding: 1rem 1.1rem; display: flex; flex-direction: column; gap: 0.5rem;
  }
  .code-card-head { display: flex; align-items: baseline; justify-content: space-between; gap: 0.5rem; }
  .code-name { font-family: var(--font-display); font-weight: 600; font-size: 1rem; overflow-wrap: anywhere; }
  .code-health { font-size: 0.74rem; font-weight: 700; white-space: nowrap; font-variant-numeric: tabular-nums; }
  .code-health.all-up { color: var(--signal); }
  .code-health.some-down { color: var(--danger); }
  .code-health.unknown { color: var(--muted); }
  .code-meta { display: flex; flex-wrap: wrap; gap: 0.5rem 0.9rem; font-size: 0.78rem; color: var(--muted); }
  .code-expiry.status-down { font-weight: 700; }
  .code-actions { display: flex; gap: 0.5rem; margin-top: auto; padding-top: 0.35rem; }
  .code-actions form { flex: 0 0 auto; }

  @media (max-width: 640px) {
    body { padding-inline: 1rem; }
    header.topbar { align-items: flex-start; }
    .stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.6rem; }
    .stat-card { min-width: 0; padding: 0.8rem; }
    .stat-card .value { font-size: 1.3rem; }
    .panel { padding: 1rem; }
    .panel-header { align-items: flex-start; }
    .filter-input { width: 100%; }
    th, td { white-space: nowrap; }
    .urls { white-space: pre-line; }
    .code-grid { grid-template-columns: 1fr; }
  }
  @media (prefers-reduced-motion: reduce) {
    html, body, .theme-toggle, .btn, .btn-link { transition: none; }
    .brand-dot, .status-up::before { animation: none; }
  }
</style>
`

// ajaxFormsScript intercepts every form with class "ajax-form" so
// submitting it (or opening/closing a "data-open-dialog"/
// "data-close-dialog" popup) never navigates the browser: it POSTs in
// the background, then re-fetches the current page and swaps in the
// refreshed .wrap content, so newly rendered rows/dialog triggers get
// their listeners re-bound too.
const ajaxFormsScript = `
<script>
function bindAjaxForms() {
  document.querySelectorAll('.ajax-form').forEach(function (form) {
    if (form.dataset.bound) return;
    form.dataset.bound = '1';
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var confirmMsg = form.getAttribute('data-confirm');
      if (confirmMsg && !confirm(confirmMsg)) return;
      fetch(form.action, { method: 'POST', body: new FormData(form), credentials: 'same-origin' })
        .then(function (r) {
          if (!r.ok) return r.text().then(function (t) { throw new Error(t || ('status ' + r.status)); });
          // Only leaf popups (add/edit forms) close on success; a row
          // action living directly inside a bigger dialog (e.g. an
          // address toggle inside "Manage") must not take that whole
          // dialog down with it.
          if (form.hasAttribute('data-close-dialog-on-success')) {
            var dialog = form.closest('dialog');
            if (dialog) dialog.close();
          }
          softRefreshWrap();
        })
        .catch(function (err) { alert('Error: ' + err.message); });
    });
  });
}

function bindDialogButtons() {
  document.querySelectorAll('[data-open-dialog]').forEach(function (btn) {
    if (btn.dataset.bound) return;
    btn.dataset.bound = '1';
    btn.addEventListener('click', function () {
      if (btn.hasAttribute('data-fill-form')) fillDialogForm(btn);
      document.getElementById(btn.getAttribute('data-open-dialog')).showModal();
    });
  });
  document.querySelectorAll('[data-close-dialog]').forEach(function (btn) {
    if (btn.dataset.bound) return;
    btn.dataset.bound = '1';
    btn.addEventListener('click', function () { btn.closest('dialog').close(); });
  });
}

// bindManageButtons wires up "Manage" buttons to fetch that one
// provider's address/credential admin UI on demand, rather than every
// provider's full manage markup being baked into every page load —
// the difference between one page load and twenty at scale.
function bindManageButtons() {
  document.querySelectorAll('[data-manage]').forEach(function (btn) {
    if (btn.dataset.bound) return;
    btn.dataset.bound = '1';
    btn.addEventListener('click', function () { openManageDialog(btn.getAttribute('data-manage')); });
  });
}

function openManageDialog(id) {
  fetch('/admin/xtream-codes/' + id + '/manage', { credentials: 'same-origin' })
    .then(function (r) {
      if (!r.ok) throw new Error('status ' + r.status);
      return r.text();
    })
    .then(function (html) {
      var dialog = document.getElementById('manageDialog');
      dialog.innerHTML = html;
      dialog.dataset.providerId = id;
      bindAjaxForms();
      bindDialogButtons();
      dialog.showModal();
    })
    .catch(function () { alert('Could not load this provider.'); });
}

// fillDialogForm copies a trigger button's data-field-* attributes
// into the matching named inputs of the form named by data-fill-form,
// and sets the form's action from data-action — used to reuse one
// "Edit" dialog/form for whichever row's Edit button was clicked.
function fillDialogForm(btn) {
  var form = document.getElementById(btn.getAttribute('data-fill-form'));
  var dialog = form.closest('dialog');
  if (btn.hasAttribute('data-action')) form.action = btn.getAttribute('data-action');
  // Reset first so reusing one dialog/form for both "add" and "edit"
  // (across different rows) never leaks a previous row's values —
  // including passwords, which are never prefilled from a data
  // attribute anyway, so "blank means keep current" holds.
  form.reset();
  for (var i = 0; i < btn.attributes.length; i++) {
    var attr = btn.attributes[i];
    if (attr.name.indexOf('data-field-') !== 0) continue;
    var field = form.querySelector('[name="' + attr.name.slice('data-field-'.length) + '"]');
    if (field) field.value = attr.value;
  }
  if (dialog) {
    var titleEl = dialog.querySelector('h2');
    if (titleEl && btn.hasAttribute('data-title')) titleEl.textContent = btn.getAttribute('data-title');
  }
  var passwordField = form.querySelector('input[type="password"]');
  if (passwordField) passwordField.required = btn.getAttribute('data-password-required') === '1';
  var hint = form.querySelector('[data-password-hint]');
  if (hint) hint.textContent = btn.getAttribute('data-password-hint') || 'Password';
}

function softRefreshWrap() {
  // The Manage dialog, if open, sits inside .wrap and would otherwise
  // be silently destroyed and recreated empty/closed by the innerHTML
  // swap below — right as its own content is what just changed.
  // Reopening it re-fetches fresh content anyway, so there's no
  // staleness risk in doing this unconditionally.
  var md = document.getElementById('manageDialog');
  var reopenManageId = (md && md.open) ? md.dataset.providerId : null;

  fetch(location.href, { credentials: 'same-origin' })
    .then(function (r) { return r.text(); })
    .then(function (html) {
      var doc = new DOMParser().parseFromString(html, 'text/html');
      var fresh = doc.querySelector('.wrap');
      if (fresh) document.querySelector('.wrap').innerHTML = fresh.innerHTML;
      bindAjaxForms();
      bindDialogButtons();
      bindManageButtons();
      if (reopenManageId) openManageDialog(reopenManageId);
    })
    .catch(function () {});
}

bindAjaxForms();
bindDialogButtons();
bindManageButtons();
</script>
`

const loginPage = styleBlock + themeToggle + `
<div class="login-shell">
  <h1><span class="brand-dot" aria-hidden="true"></span>iptv-proxy</h1>
  <p class="subtitle">Admin control panel</p>
  {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
  <form method="post" action="/admin/login">
    <label for="login-username">Username</label>
    <input id="login-username" type="text" name="username" autocomplete="username" required autofocus>
    <label for="login-password">Password</label>
    <input id="login-password" type="password" name="password" autocomplete="current-password" required>
    <button type="submit" class="btn" style="width:100%">Log in</button>
  </form>
</div>
`

const dashboardPage = styleBlock + themeToggle + `
<div class="wrap">
  <header class="topbar">
    <h1><span class="brand-dot" aria-hidden="true"></span>iptv-proxy</h1>
    <nav>
      <form class="inline" method="post" action="/admin/logout">
        <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
        <button type="submit" class="btn-link" style="background:none;border:none;cursor:pointer;">Log out</button>
      </form>
    </nav>
  </header>

  <div class="stats">
    <div class="stat-card">
      <div class="value">{{len .XtreamCodes}}</div>
      <div class="label">Xtream codes</div>
    </div>
    <div class="stat-card">
      <div class="value">{{len .Users}}</div>
      <div class="label">Users</div>
    </div>
    <div class="stat-card">
      <div class="value" id="healthCount">{{.OnlineBackends}} / {{len .BackendHealth}}</div>
      <div class="label">Addresses online</div>
    </div>
    <div class="stat-card">
      <div class="value" id="bwCurrent">{{.Bandwidth.Current}}</div>
      <div class="label">Current throughput</div>
    </div>
    <div class="stat-card">
      <div class="value" id="bw24h">{{.Bandwidth.Last24hGB}} GB</div>
      <div class="label">Last 24h</div>
    </div>
    <div class="stat-card">
      <div class="value" id="bw7d">{{.Bandwidth.Last7dGB}} GB</div>
      <div class="label">Last 7d</div>
    </div>
    <div class="stat-card">
      <div class="value" id="bwPeak">{{.Bandwidth.Peak}}</div>
      <div class="label">Peak (7d)</div>
    </div>
  </div>

  <div class="panel">
    <div class="panel-header">
      <h2>Upstream health</h2>
      <div class="panel-tools">
        <input type="search" class="filter-input" id="healthFilter" placeholder="Filter backend or address…" oninput="filterHealthRows()">
        <span class="help">Every 5 min · latest 60 checks</span>
      </div>
    </div>
    <div class="table-scroll scroll-bounded"><table>
      <tr><th>Backend</th><th>Address</th><th>Status</th><th>Ping</th><th>24h</th><th>7d</th><th>Recent history</th><th>Checked</th></tr>
      <tbody id="healthBody">
      {{range .BackendHealth}}
      <tr>
        <td>{{range .Backends}}<span class="badge">{{.}}</span> {{end}}</td>
        <td class="mono health-address">{{.BaseURL}}</td>
        <td><span class="status-pill {{if .Up}}status-up{{else}}status-down{{end}}">{{if .Up}}Up{{else}}Down{{end}}</span></td>
        <td class="ping">{{if .Up}}{{.LatencyMS}} ms{{else}}&mdash;{{end}}</td>
        <td class="uptime">{{if .Uptime24hKnown}}{{.Uptime24hPercent}}%{{else}}unknown{{end}}</td>
        <td class="uptime">{{if .Uptime7dKnown}}{{.Uptime7dPercent}}%{{else}}unknown{{end}}</td>
        <td><div class="health-history" aria-label="Recent uptime history">{{range .History}}<span class="health-bar {{if not .}}down{{end}}" title="{{if .}}Up{{else}}Down{{end}}"></span>{{end}}</div></td>
        <td><span class="health-time" data-checked="{{.CheckedAtUnix}}">just now</span></td>
      </tr>
      {{else}}
      <tr class="empty-row"><td colspan="8">Waiting for the first health check</td></tr>
      {{end}}
      </tbody>
    </table></div>
  </div>

  <div class="panel">
    <div class="panel-header">
      <h2>Xtream codes</h2>
      <div class="panel-tools">
        <input type="search" class="filter-input" id="codeFilter" placeholder="Filter by name…" oninput="filterCodeCards()">
        <button type="button" class="add-link link-btn" data-open-dialog="newXtreamCodeDialog">+ Add xtream code</button>
      </div>
    </div>
    <div class="code-grid" id="codeGrid">
      {{range .XtreamCodes}}
      {{$exp := index $.SubscriptionExpiries .ID}}
      {{$addrs := index $.AddressesByCode .ID}}
      {{$h := index $.CodeHealth .Name}}
      <div class="code-card" data-name="{{.Name}}">
        <div class="code-card-head">
          <span class="code-name">{{.Name}}</span>
          {{if eq $h.Total 0}}<span class="code-health unknown">no data</span>
          {{else if eq $h.Up $h.Total}}<span class="code-health all-up">{{$h.Up}}/{{$h.Total}} up</span>
          {{else}}<span class="code-health some-down">{{$h.Up}}/{{$h.Total}} up</span>
          {{end}}
        </div>
        <div class="code-meta">
          <span>{{len $addrs}} address{{if ne (len $addrs) 1}}es{{end}}</span>
          <span>{{len .Credentials}} credential{{if ne (len .Credentials) 1}}s{{end}}</span>
          <span class="code-expiry{{if and $exp.HasExpiry (le $exp.DaysLeft 0)}} status-down{{end}}">
            {{if eq $exp.CheckedAtUnix 0}}expiry unknown
            {{else if not $exp.HasExpiry}}unlimited
            {{else if le $exp.DaysLeft 0}}expired
            {{else}}{{$exp.DaysLeft}}d left
            {{end}}
          </span>
        </div>
        <div class="code-actions">
          <button type="button" class="btn-link btn-sm" data-manage="{{.ID}}">Manage</button>
          <form class="inline ajax-form" method="post" action="/admin/xtream-codes/{{.ID}}/delete" data-confirm="Delete this xtream code?">
            <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
            <button type="submit" class="btn btn-sm btn-danger">Delete</button>
          </form>
        </div>
      </div>
      {{else}}
      <p class="help">No xtream codes yet.</p>
      {{end}}
    </div>
  </div>

  <div class="panel">
    <div class="panel-header">
      <h2>Users</h2>
      <button type="button" class="add-link link-btn" data-open-dialog="userFormDialog" data-fill-form="userFormDialogForm"
        data-action="/admin/users/new" data-title="New user" data-password-required="1" data-password-hint="Password">+ Add user</button>
    </div>
    <div class="table-scroll"><table>
      <tr><th>Username</th><th>Assigned credential</th><th>Max streams</th><th></th></tr>
      {{range .Users}}
      <tr>
        <td>{{.Username}}</td>
        <td><span class="badge">{{.XtreamCodeName}}</span> <span class="mono">{{.CredentialLabel}}</span></td>
        <td>{{if .MaxConcurrentStreams}}{{.MaxConcurrentStreams}}{{else}}Unlimited{{end}}</td>
        <td class="actions">
          <button type="button" class="btn-link"
            data-open-dialog="userFormDialog" data-fill-form="userFormDialogForm"
            data-action="/admin/users/{{.ID}}/edit" data-title="Edit user"
            data-field-username="{{.Username}}" data-field-credential_id="{{.CredentialID}}" data-field-max_concurrent_streams="{{.MaxConcurrentStreams}}"
            data-password-required="0" data-password-hint="Password (leave blank to keep current)">Edit</button>
          <form class="inline ajax-form" method="post" action="/admin/users/{{.ID}}/delete" data-confirm="Delete this user?">
            <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
            <button type="submit" class="btn btn-sm btn-danger">Delete</button>
          </form>
        </td>
      </tr>
      {{else}}
      <tr class="empty-row"><td colspan="4">No users yet</td></tr>
      {{end}}
    </table></div>
  </div>

  <!-- Inside .wrap (not siblings after it) so a soft-refresh after any
       action re-renders their server-side content too — e.g. the
       credential <select> below must pick up a just-added xtream
       code/credential immediately, not the page's stale initial load. -->
  <dialog id="manageDialog" class="dialog-wide"></dialog>

  <dialog id="addAddressDialog">
    <h2>Add addresses</h2>
    <form method="post" id="addAddressDialogForm" class="ajax-form" data-close-dialog-on-success>
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <label for="addresses">Addresses</label>
      <textarea id="addresses" name="addresses" placeholder="http://backup2.example.tv:8080"></textarea>
      <p class="help">One address per line. Paste several at once; each becomes its own toggleable row.</p>
      <div class="dialog-actions">
        <button type="button" class="btn-link" data-close-dialog>Cancel</button>
        <button type="submit" class="btn">Add</button>
      </div>
    </form>
  </dialog>

  <dialog id="addCredentialDialog">
    <h2>Add credential</h2>
    <form method="post" id="addCredentialDialogForm" class="ajax-form" data-close-dialog-on-success>
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <label for="add_cred_name">Name (optional)</label>
      <input id="add_cred_name" type="text" name="name" autocomplete="off" placeholder="e.g. Mom's account">
      <label for="add_cred_user">Xtream username</label>
      <input id="add_cred_user" type="text" name="xtream_user" autocomplete="off" required>
      <label for="add_cred_pass">Xtream password</label>
      <input id="add_cred_pass" type="password" name="xtream_password" autocomplete="new-password" required>
      <div class="dialog-actions">
        <button type="button" class="btn-link" data-close-dialog>Cancel</button>
        <button type="submit" class="btn">Add credential</button>
      </div>
    </form>
  </dialog>

  <dialog id="editCredentialDialog">
    <h2>Edit credential</h2>
    <form method="post" id="editCredentialForm" class="ajax-form" data-close-dialog-on-success>
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <label for="edit_cred_name">Name (optional)</label>
      <input id="edit_cred_name" type="text" name="name" autocomplete="off" placeholder="e.g. Mom's account">
      <label for="edit_cred_user">Xtream username</label>
      <input id="edit_cred_user" type="text" name="xtream_user" autocomplete="off" required>
      <label for="edit_cred_pass">Xtream password (leave blank to keep current)</label>
      <input id="edit_cred_pass" type="password" name="xtream_password" autocomplete="new-password">
      <div class="dialog-actions">
        <button type="button" class="btn-link" data-close-dialog>Cancel</button>
        <button type="submit" class="btn">Save</button>
      </div>
    </form>
  </dialog>

  <dialog id="newXtreamCodeDialog">
    <h2>New xtream code</h2>
    <form method="post" action="/admin/xtream-codes/new" class="ajax-form" data-close-dialog-on-success>
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <label for="new_xc_name">Name</label>
      <input id="new_xc_name" type="text" name="name" autocomplete="off" required autofocus>
      <label for="new_xc_base_url">Base URLs</label>
      <textarea id="new_xc_base_url" name="base_url" placeholder="http://primary.example.tv:8080&#10;http://backup.example.tv:8080" required></textarea>
      <p class="help">Enter one address per line. Toggle individual addresses on/off, or add more, from the provider's Manage popup afterwards.</p>
      <label for="new_xc_cred_name">Credential name (optional)</label>
      <input id="new_xc_cred_name" type="text" name="credential_name" autocomplete="off" placeholder="e.g. Mom's account">
      <label for="new_xc_user">Xtream username</label>
      <input id="new_xc_user" type="text" name="xtream_user" autocomplete="off" required>
      <label for="new_xc_pass">Xtream password</label>
      <input id="new_xc_pass" type="password" name="xtream_password" autocomplete="new-password" required>
      <p class="help">You can add more username/password pairs from the provider's Manage popup afterwards.</p>
      <div class="dialog-actions">
        <button type="button" class="btn-link" data-close-dialog>Cancel</button>
        <button type="submit" class="btn">Save</button>
      </div>
    </form>
  </dialog>

  <dialog id="userFormDialog">
    <h2>New user</h2>
    <form method="post" id="userFormDialogForm" class="ajax-form" data-close-dialog-on-success>
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <label for="uf_username">Username</label>
      <input id="uf_username" type="text" name="username" autocomplete="off" required autofocus>
      <label for="uf_password" data-password-hint>Password</label>
      <input id="uf_password" type="password" name="password" autocomplete="new-password">
      <label for="uf_credential_id">Xtream code / credential</label>
      <select id="uf_credential_id" name="credential_id" required>
        {{range .XtreamCodes}}
        {{if .Credentials}}
        <optgroup label="{{.Name}}">
          {{range .Credentials}}
          <option value="{{.ID}}">{{.Label}}</option>
          {{end}}
        </optgroup>
        {{end}}
        {{end}}
      </select>
      <label for="uf_max_streams">Max concurrent streams</label>
      <input id="uf_max_streams" type="number" name="max_concurrent_streams" min="0" value="1" required>
      <p class="help">Use 0 for unlimited streams.</p>
      <div class="dialog-actions">
        <button type="button" class="btn-link" data-close-dialog>Cancel</button>
        <button type="submit" class="btn">Save</button>
      </div>
    </form>
  </dialog>
</div>

<script>
function fmtAgo(unix) {
  var seconds = Math.max(0, Math.floor(Date.now() / 1000 - unix));
  if (seconds < 60) return seconds + 's ago';
  if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
  return Math.floor(seconds / 3600) + 'h ago';
}

function tickHealthTimes() {
  document.querySelectorAll('.health-time').forEach(function (cell) {
    cell.textContent = fmtAgo(parseInt(cell.getAttribute('data-checked'), 10));
  });
}

function healthHistory(history) {
  return history.map(function (up) {
    return '<span class="health-bar' + (up ? '' : ' down') + '" title="' + (up ? 'Up' : 'Down') + '"></span>';
  }).join('');
}

function filterHealthRows() {
  var q = document.getElementById('healthFilter').value.toLowerCase();
  document.querySelectorAll('#healthBody tr').forEach(function (tr) {
    if (tr.classList.contains('empty-row')) return;
    tr.style.display = tr.textContent.toLowerCase().indexOf(q) === -1 ? 'none' : '';
  });
}

function filterCodeCards() {
  var q = document.getElementById('codeFilter').value.toLowerCase();
  document.querySelectorAll('#codeGrid .code-card').forEach(function (card) {
    card.style.display = (card.dataset.name || '').toLowerCase().indexOf(q) === -1 ? 'none' : '';
  });
}

function refreshHealth() {
  fetch('/admin/health.json', { credentials: 'same-origin' })
    .then(function (r) { return r.json(); })
    .then(function (data) {
      document.getElementById('healthCount').textContent = data.online + ' / ' + data.total;
      var body = document.getElementById('healthBody');
      if (data.total === 0) {
        body.innerHTML = '<tr class="empty-row"><td colspan="8">Waiting for the first health check</td></tr>';
        return;
      }
      body.innerHTML = data.addresses.map(function (item) {
        var status = item.Up ? 'Up' : 'Down';
        var backends = (item.Backends || []).map(function (b) { return '<span class="badge">' + escapeHtml(b) + '</span>'; }).join(' ');
        return '<tr><td>' + backends + '</td>' +
          '<td class="mono health-address">' + escapeHtml(item.BaseURL) + '</td>' +
          '<td><span class="status-pill ' + (item.Up ? 'status-up' : 'status-down') + '">' + status + '</span></td>' +
          '<td class="ping">' + (item.Up ? item.LatencyMS + ' ms' : '&mdash;') + '</td>' +
          '<td class="uptime">' + (item.Uptime24hKnown ? item.Uptime24hPercent + '%' : 'unknown') + '</td>' +
          '<td class="uptime">' + (item.Uptime7dKnown ? item.Uptime7dPercent + '%' : 'unknown') + '</td>' +
          '<td><div class="health-history" aria-label="Recent uptime history">' + healthHistory(item.History) + '</div></td>' +
          '<td><span class="health-time" data-checked="' + item.CheckedAtUnix + '">just now</span></td></tr>';
      }).join('');
      tickHealthTimes();
      filterHealthRows();

      var bw = data.bandwidth || {};
      document.getElementById('bwCurrent').textContent = bw.Current || '—';
      document.getElementById('bw24h').textContent = (bw.Last24hGB || '0.00') + ' GB';
      document.getElementById('bw7d').textContent = (bw.Last7dGB || '0.00') + ' GB';
      document.getElementById('bwPeak').textContent = bw.Peak || '—';
    })
    .catch(function () {});
}

function escapeHtml(s) {
  var div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

tickHealthTimes();
setInterval(tickHealthTimes, 1000);
setInterval(refreshHealth, 10000);
</script>
` + ajaxFormsScript

// manageFragmentPage is the inner content of the (single, shared)
// "Manage" dialog, fetched on demand for one provider at a time —
// with 20+ providers, baking every one of these into the dashboard's
// initial HTML would mean rendering 20 address/credential tables no
// one is looking at yet.
const manageFragmentPage = `
<h2>Manage {{.Name}}</h2>

<h3>Rename</h3>
<form method="post" action="/admin/xtream-codes/{{.ID}}/edit" class="ajax-form">
  <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
  <label for="manage_name">Name</label>
  <input id="manage_name" type="text" name="name" value="{{.Name}}" autocomplete="off" required>
  <button type="submit" class="btn">Save</button>
</form>

<div class="panel-header">
  <h3>Addresses</h3>
  <button type="button" class="add-link link-btn"
    data-open-dialog="addAddressDialog" data-fill-form="addAddressDialogForm"
    data-action="/admin/xtream-codes/{{.ID}}/addresses" data-title="Add addresses to {{.Name}}">+ Add addresses</button>
</div>
<div class="table-scroll"><table>
  <tr><th>Address</th><th>Status</th><th></th></tr>
  {{range .Addresses}}
  <tr>
    <td class="mono">{{.Address}}</td>
    <td>
      <form class="inline ajax-form" method="post" action="/admin/xtream-codes/{{$.ID}}/addresses/{{.ID}}/toggle">
        <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
        <input type="hidden" name="enabled" value="{{if .Enabled}}0{{else}}1{{end}}">
        <button type="submit" class="btn-sm {{if .Enabled}}btn{{else}}btn-link{{end}}">{{if .Enabled}}Enabled{{else}}Disabled{{end}}</button>
      </form>
    </td>
    <td class="actions">
      <form class="inline ajax-form" method="post" action="/admin/xtream-codes/{{$.ID}}/addresses/{{.ID}}/delete" data-confirm="Remove this address?">
        <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
        <button type="submit" class="btn btn-sm btn-danger">Delete</button>
      </form>
    </td>
  </tr>
  {{else}}
  <tr class="empty-row"><td colspan="3">No addresses yet</td></tr>
  {{end}}
</table></div>

<div class="panel-header">
  <h3>Credentials</h3>
  <button type="button" class="add-link link-btn"
    data-open-dialog="addCredentialDialog" data-fill-form="addCredentialDialogForm"
    data-action="/admin/xtream-codes/{{.ID}}/credentials" data-title="Add credential to {{.Name}}">+ Add credential</button>
</div>
<div class="table-scroll"><table>
  <tr><th>Name</th><th>Username</th><th></th></tr>
  {{range .Credentials}}
  <tr>
    <td>{{if .Name}}{{.Name}}{{else}}&mdash;{{end}}</td>
    <td class="mono">{{.XtreamUser}}</td>
    <td class="actions">
      <button type="button" class="btn-link btn-sm"
        data-open-dialog="editCredentialDialog" data-fill-form="editCredentialForm"
        data-action="/admin/xtream-codes/{{$.ID}}/credentials/{{.ID}}/edit" data-title="Edit credential"
        data-field-name="{{.Name}}" data-field-xtream_user="{{.XtreamUser}}">Edit</button>
      <form class="inline ajax-form" method="post" action="/admin/xtream-codes/{{$.ID}}/credentials/{{.ID}}/delete" data-confirm="Delete this credential?">
        <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
        <button type="submit" class="btn btn-sm btn-danger">Delete</button>
      </form>
    </td>
  </tr>
  {{else}}
  <tr class="empty-row"><td colspan="3">No credentials yet</td></tr>
  {{end}}
</table></div>

<div class="dialog-actions">
  <button type="button" class="btn-link" data-close-dialog>Close</button>
</div>
`

var templates = template.Must(template.New("root").Parse(""))

func init() {
	template.Must(templates.New("login").Parse(loginPage))
	template.Must(templates.New("dashboard").Parse(dashboardPage))
	template.Must(templates.New("manageFragment").Parse(manageFragmentPage))
}
