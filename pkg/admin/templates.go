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

const styleBlock = `<title>iptv-proxy admin</title>` + themeInit + `
<style>
  :root {
    /* Dark theme (default) */
    --bg: #11141a;
    --panel: #1a1e26;
    --border: #2b313d;
    --text: #e7e9ee;
    --muted: #8d94a3;
    --accent: #7b8cf0;
    --accent-dark: #97a4f3;
    --accent-soft: rgba(123, 140, 240, 0.12);
    --success: #4fd1a5;
    --danger: #ef6478;
    --danger-dark: #f4899a;
    --badge-bg: #232a44;
    --danger-bg: #3a2026;
    --danger-border: #5a2c34;
    --radius: 10px;
  }
  html[data-theme="light"] {
    --bg: #f4f6f9;
    --panel: #ffffff;
    --border: #e3e7ee;
    --text: #1c2230;
    --muted: #6b7280;
    --accent: #4f63d2;
    --accent-dark: #3b4cb0;
    --accent-soft: rgba(79, 99, 210, 0.09);
    --success: #168565;
    --danger: #d3455b;
    --danger-dark: #b8334a;
    --badge-bg: #eef0fb;
    --danger-bg: #fdeef0;
    --danger-border: #f6c9d1;
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
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: radial-gradient(circle at 50% -20%, var(--accent-soft), transparent 38rem), var(--bg);
    color: var(--text);
    margin: 0;
    padding: 0 1.5rem 3rem;
  }
  .wrap { max-width: 960px; margin: 0 auto; }
  header.topbar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 1.25rem 0; margin-bottom: 1.5rem; border-bottom: 1px solid var(--border);
  }
  header.topbar h1 { font-size: 1.15rem; font-weight: 700; margin: 0; letter-spacing: -0.02em; }
  .brand-dot {
    display: inline-block; width: 0.55rem; height: 0.55rem; margin-right: 0.45rem;
    border-radius: 50%; background: var(--success); box-shadow: 0 0 0 4px color-mix(in srgb, var(--success) 14%, transparent);
  }
  header.topbar nav a { color: var(--muted); text-decoration: none; font-size: 0.9rem; margin-right: 1rem; }
  header.topbar nav a:hover { color: var(--text); }
  .login-shell {
    max-width: 360px; margin: 4rem auto 0; padding: 2rem;
    background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius);
  }
  .login-shell h1 { font-size: 1.2rem; text-align: center; margin-top: 0; }
  .login-shell .subtitle { margin: -0.45rem 0 1.5rem; color: var(--muted); font-size: 0.82rem; text-align: center; }
  .stats { display: flex; gap: 1rem; margin-bottom: 1.5rem; flex-wrap: wrap; }
  .stat-card {
    flex: 1; min-width: 160px; background: var(--panel); border: 1px solid var(--border);
    border-radius: var(--radius); padding: 1rem 1.25rem;
    box-shadow: 0 1px 2px rgba(0,0,0,0.08);
  }
  .stat-card .value { font-size: 1.8rem; font-weight: 750; color: var(--accent); line-height: 1; font-variant-numeric: tabular-nums; }
  .stat-card .label { font-size: 0.8rem; color: var(--muted); margin-top: 0.3rem; }
  .panel {
    background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius);
    padding: 1.25rem 1.5rem; margin-bottom: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.08);
  }
  .panel-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.75rem; }
  .panel-header h2 { font-size: 1rem; font-weight: 600; margin: 0; }
  table { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
  .table-scroll { overflow-x: auto; margin: 0 -0.5rem; padding: 0 0.5rem; }
  th, td { text-align: left; padding: 0.55rem 0.5rem; border-bottom: 1px solid var(--border); }
  th { color: var(--muted); font-weight: 600; font-size: 0.78rem; text-transform: uppercase; letter-spacing: 0.03em; }
  tr:last-child td { border-bottom: none; }
  .empty-row td { color: var(--muted); text-align: center; padding: 1.5rem 0; }
  .badge {
    display: inline-block; padding: 0.15rem 0.55rem; border-radius: 999px;
    background: var(--badge-bg); color: var(--accent-dark); font-size: 0.75rem; font-weight: 600;
  }
  .mono { font-family: ui-monospace, Menlo, Consolas, monospace; font-size: 0.82rem; color: var(--muted); }
  .urls { white-space: pre-line; overflow-wrap: anywhere; min-width: 13rem; }
  .health-address { max-width: 20rem; overflow-wrap: anywhere; }
  .status-pill { display: inline-flex; align-items: center; gap: 0.4rem; font-size: 0.78rem; font-weight: 700; }
  .status-pill::before { content: ""; width: 0.55rem; height: 0.55rem; border-radius: 50%; background: currentColor; }
  .status-up { color: var(--success); }
  .status-down { color: var(--danger); }
  .health-history { display: grid; grid-template-columns: repeat(60, minmax(1px, 1fr)); gap: 2px; width: 11rem; height: 1.4rem; }
  .health-bar { min-width: 1px; border-radius: 2px; background: var(--success); opacity: 0.9; }
  .health-bar.down { background: var(--danger); }
  .health-time, .ping, .uptime { font-variant-numeric: tabular-nums; white-space: nowrap; }
  form.inline { display: inline; }
  .actions a, .actions button { margin-right: 0.4rem; }
  .error {
    color: var(--danger); background: var(--danger-bg); border: 1px solid var(--danger-border);
    border-radius: var(--radius); padding: 0.6rem 0.9rem; margin-bottom: 1rem; font-size: 0.88rem;
  }
  label { display: block; margin-top: 0.9rem; font-weight: 600; font-size: 0.85rem; }
  input, select, textarea {
    padding: 0.5rem 0.6rem; width: 100%; max-width: 360px; margin-top: 0.3rem;
    border: 1px solid var(--border); border-radius: 6px; font-size: 0.9rem; background: var(--panel); color: var(--text);
  }
  textarea { min-height: 6rem; resize: vertical; }
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
  .add-link { font-size: 0.88rem; font-weight: 600; color: var(--accent); text-decoration: none; }
  .help { max-width: 360px; margin: 0.35rem 0 0; color: var(--muted); font-size: 0.78rem; line-height: 1.45; text-wrap: pretty; }
  :focus-visible { outline: 3px solid var(--accent-soft); outline-offset: 2px; }
  @media (max-width: 640px) {
    body { padding-inline: 1rem; }
    header.topbar { align-items: flex-start; }
    .stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.6rem; }
    .stat-card { min-width: 0; padding: 0.8rem; }
    .stat-card .value { font-size: 1.45rem; }
    .panel { padding: 1rem; }
    .panel-header { align-items: flex-start; gap: 1rem; }
    th, td { white-space: nowrap; }
    .urls { white-space: pre-line; }
  }
  @media (prefers-reduced-motion: reduce) {
    html, body, .theme-toggle, .btn, .btn-link { transition: none; }
  }
</style>
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
  </div>

  <div class="panel">
    <div class="panel-header">
      <h2>Upstream health</h2>
      <span class="help">Every 5 minutes · latest 60 checks</span>
    </div>
    <div class="table-scroll"><table>
      <tr><th>Backend</th><th>Address</th><th>Status</th><th>Ping</th><th>Uptime</th><th>Recent history</th><th>Checked</th></tr>
      <tbody id="healthBody">
      {{range .BackendHealth}}
      <tr>
        <td><span class="badge">{{.Backend}}</span></td>
        <td class="mono health-address">{{.BaseURL}}</td>
        <td><span class="status-pill {{if .Up}}status-up{{else}}status-down{{end}}">{{if .Up}}Up{{else}}Down{{end}}</span></td>
        <td class="ping">{{if .Up}}{{.LatencyMS}} ms{{else}}&mdash;{{end}}</td>
        <td class="uptime">{{.UptimePercent}}%</td>
        <td><div class="health-history" aria-label="Recent uptime history">{{range .History}}<span class="health-bar {{if not .}}down{{end}}" title="{{if .}}Up{{else}}Down{{end}}"></span>{{end}}</div></td>
        <td><span class="health-time" data-checked="{{.CheckedAtUnix}}">just now</span></td>
      </tr>
      {{else}}
      <tr class="empty-row"><td colspan="7">Waiting for the first health check</td></tr>
      {{end}}
      </tbody>
    </table></div>
  </div>

  <div class="panel">
    <div class="panel-header">
      <h2>Xtream codes</h2>
      <a class="add-link" href="/admin/xtream-codes/new">+ Add xtream code</a>
    </div>
    <div class="table-scroll"><table>
      <tr><th>Name</th><th>Base URLs</th><th>Xtream user</th><th></th></tr>
      {{range .XtreamCodes}}
      <tr>
        <td><span class="badge">{{.Name}}</span></td>
        <td class="mono urls">{{.BaseURL}}</td>
        <td class="mono">{{.XtreamUser}}</td>
        <td class="actions">
          <a class="btn-link" href="/admin/xtream-codes/{{.ID}}/edit">Edit</a>
          <form class="inline" method="post" action="/admin/xtream-codes/{{.ID}}/delete" onsubmit="return confirm('Delete this xtream code?')">
            <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
            <button type="submit" class="btn btn-sm btn-danger">Delete</button>
          </form>
        </td>
      </tr>
      {{else}}
      <tr class="empty-row"><td colspan="4">No xtream codes yet</td></tr>
      {{end}}
    </table></div>
  </div>

  <div class="panel">
    <div class="panel-header">
      <h2>Users</h2>
      <a class="add-link" href="/admin/users/new">+ Add user</a>
    </div>
    <div class="table-scroll"><table>
      <tr><th>Username</th><th>Assigned xtream code</th><th>Max streams</th><th></th></tr>
      {{range .Users}}
      <tr>
        <td>{{.Username}}</td>
        <td><span class="badge">{{.XtreamCodeName}}</span></td>
        <td>{{if .MaxConcurrentStreams}}{{.MaxConcurrentStreams}}{{else}}Unlimited{{end}}</td>
        <td class="actions">
          <a class="btn-link" href="/admin/users/{{.ID}}/edit">Edit</a>
          <form class="inline" method="post" action="/admin/users/{{.ID}}/delete" onsubmit="return confirm('Delete this user?')">
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

function refreshHealth() {
  fetch('/admin/health.json', { credentials: 'same-origin' })
    .then(function (r) { return r.json(); })
    .then(function (data) {
      document.getElementById('healthCount').textContent = data.online + ' / ' + data.total;
      var body = document.getElementById('healthBody');
      if (data.total === 0) {
        body.innerHTML = '<tr class="empty-row"><td colspan="7">Waiting for the first health check</td></tr>';
        return;
      }
      body.innerHTML = data.addresses.map(function (item) {
        var status = item.Up ? 'Up' : 'Down';
        return '<tr><td><span class="badge">' + escapeHtml(item.Backend) + '</span></td>' +
          '<td class="mono health-address">' + escapeHtml(item.BaseURL) + '</td>' +
          '<td><span class="status-pill ' + (item.Up ? 'status-up' : 'status-down') + '">' + status + '</span></td>' +
          '<td class="ping">' + (item.Up ? item.LatencyMS + ' ms' : '&mdash;') + '</td>' +
          '<td class="uptime">' + item.UptimePercent + '%</td>' +
          '<td><div class="health-history" aria-label="Recent uptime history">' + healthHistory(item.History) + '</div></td>' +
          '<td><span class="health-time" data-checked="' + item.CheckedAtUnix + '">just now</span></td></tr>';
      }).join('');
      tickHealthTimes();
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
`

const xtreamCodeFormPage = styleBlock + themeToggle + `
<div class="wrap">
  <header class="topbar"><h1>iptv-proxy admin</h1><nav><a href="/admin">&larr; Dashboard</a></nav></header>
  <div class="panel" style="max-width:480px;">
    <h2>{{if .ID}}Edit{{else}}New{{end}} xtream code</h2>
    {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
    <form method="post" action="{{.Action}}">
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <label for="name">Name</label>
      <input id="name" type="text" name="name" value="{{.Name}}" autocomplete="off" required autofocus>
      <label for="base_url">Base URLs</label>
      <textarea id="base_url" name="base_url" aria-describedby="base-url-help" placeholder="http://primary.example.tv:8080&#10;http://backup.example.tv:8080" required>{{.BaseURL}}</textarea>
      <p class="help" id="base-url-help">Enter one address per line. The proxy automatically uses the fastest reachable address.</p>
      <label for="xtream_user">Xtream username</label>
      <input id="xtream_user" type="text" name="xtream_user" value="{{.XtreamUser}}" autocomplete="off" required>
      <label for="xtream_password">Xtream password</label>
      <input id="xtream_password" type="password" name="xtream_password" value="{{.XtreamPassword}}" autocomplete="new-password" required>
      <button type="submit" class="btn">Save</button>
    </form>
  </div>
</div>
`

const userFormPage = styleBlock + themeToggle + `
<div class="wrap">
  <header class="topbar"><h1>iptv-proxy admin</h1><nav><a href="/admin">&larr; Dashboard</a></nav></header>
  <div class="panel" style="max-width:480px;">
    <h2>{{if .ID}}Edit{{else}}New{{end}} user</h2>
    {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
    <form method="post" action="{{.Action}}">
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <label for="username">Username</label>
      <input id="username" type="text" name="username" value="{{.Username}}" autocomplete="off" required autofocus>
      <label for="password">Password{{if .ID}} (leave blank to keep current){{end}}</label>
      <input id="password" type="password" name="password" autocomplete="new-password" {{if not .ID}}required{{end}}>
      <label for="xtream_code_id">Xtream code</label>
      <select id="xtream_code_id" name="xtream_code_id" required>
        {{range .XtreamCodes}}
        <option value="{{.ID}}" {{if eq .ID $.XtreamCodeID}}selected{{end}}>{{.Name}}</option>
        {{end}}
      </select>
      <label for="max_concurrent_streams">Max concurrent streams</label>
      <input id="max_concurrent_streams" type="number" name="max_concurrent_streams" value="{{.MaxConcurrentStreams}}" min="0" aria-describedby="stream-limit-help" required>
      <p class="help" id="stream-limit-help">Use 0 for unlimited streams.</p>
      <button type="submit" class="btn">Save</button>
    </form>
  </div>
</div>
`

var templates = template.Must(template.New("root").Parse(""))

func init() {
	template.Must(templates.New("login").Parse(loginPage))
	template.Must(templates.New("dashboard").Parse(dashboardPage))
	template.Must(templates.New("xtreamCodeForm").Parse(xtreamCodeFormPage))
	template.Must(templates.New("userForm").Parse(userFormPage))
}
