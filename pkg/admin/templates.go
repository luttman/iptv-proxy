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

const styleBlock = themeInit + `
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
    --danger: #d3455b;
    --danger-dark: #b8334a;
    --badge-bg: #eef0fb;
    --danger-bg: #fdeef0;
    --danger-border: #f6c9d1;
  }
  * { box-sizing: border-box; }
  html, body { transition: background-color 0.15s ease; }
  .theme-toggle {
    position: fixed; top: 1rem; right: 1.5rem; z-index: 10;
    background: var(--panel); border: 1px solid var(--border); border-radius: 999px;
    width: 2.2rem; height: 2.2rem; cursor: pointer; font-size: 1rem; line-height: 1;
  }
  .theme-toggle:hover { border-color: var(--accent); }
  .theme-toggle .icon-moon { display: none; }
  html[data-theme="light"] .theme-toggle .icon-sun { display: none; }
  html[data-theme="light"] .theme-toggle .icon-moon { display: inline; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: var(--bg);
    color: var(--text);
    margin: 0;
    padding: 0 1.5rem 3rem;
  }
  .wrap { max-width: 960px; margin: 0 auto; }
  header.topbar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 1.25rem 0; margin-bottom: 1.5rem; border-bottom: 1px solid var(--border);
  }
  header.topbar h1 { font-size: 1.15rem; font-weight: 600; margin: 0; }
  header.topbar nav a { color: var(--muted); text-decoration: none; font-size: 0.9rem; margin-right: 1rem; }
  header.topbar nav a:hover { color: var(--text); }
  .login-shell {
    max-width: 360px; margin: 4rem auto 0; padding: 2rem;
    background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius);
  }
  .login-shell h1 { font-size: 1.2rem; text-align: center; margin-top: 0; }
  .stats { display: flex; gap: 1rem; margin-bottom: 1.5rem; flex-wrap: wrap; }
  .stat-card {
    flex: 1; min-width: 160px; background: var(--panel); border: 1px solid var(--border);
    border-radius: var(--radius); padding: 1rem 1.25rem;
  }
  .stat-card .value { font-size: 1.8rem; font-weight: 700; color: var(--accent); line-height: 1; }
  .stat-card .label { font-size: 0.8rem; color: var(--muted); margin-top: 0.3rem; }
  .panel {
    background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius);
    padding: 1.25rem 1.5rem; margin-bottom: 1.5rem;
  }
  .panel-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.75rem; }
  .panel-header h2 { font-size: 1rem; font-weight: 600; margin: 0; }
  table { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
  th, td { text-align: left; padding: 0.55rem 0.5rem; border-bottom: 1px solid var(--border); }
  th { color: var(--muted); font-weight: 600; font-size: 0.78rem; text-transform: uppercase; letter-spacing: 0.03em; }
  tr:last-child td { border-bottom: none; }
  .empty-row td { color: var(--muted); text-align: center; padding: 1.5rem 0; }
  .badge {
    display: inline-block; padding: 0.15rem 0.55rem; border-radius: 999px;
    background: var(--badge-bg); color: var(--accent-dark); font-size: 0.75rem; font-weight: 600;
  }
  .mono { font-family: ui-monospace, Menlo, Consolas, monospace; font-size: 0.82rem; color: var(--muted); }
  form.inline { display: inline; }
  .actions a, .actions button { margin-right: 0.4rem; }
  .error {
    color: var(--danger); background: var(--danger-bg); border: 1px solid var(--danger-border);
    border-radius: var(--radius); padding: 0.6rem 0.9rem; margin-bottom: 1rem; font-size: 0.88rem;
  }
  label { display: block; margin-top: 0.9rem; font-weight: 600; font-size: 0.85rem; }
  input, select {
    padding: 0.5rem 0.6rem; width: 100%; max-width: 360px; margin-top: 0.3rem;
    border: 1px solid var(--border); border-radius: 6px; font-size: 0.9rem; background: var(--panel); color: var(--text);
  }
  input:focus, select:focus { outline: none; border-color: var(--accent); }
  .btn {
    display: inline-block; margin-top: 1.25rem; padding: 0.55rem 1.1rem; cursor: pointer;
    border: none; border-radius: 6px; font-size: 0.88rem; font-weight: 600;
    background: var(--accent); color: white;
  }
  .btn:hover { background: var(--accent-dark); }
  .btn-sm { margin-top: 0; padding: 0.3rem 0.7rem; font-size: 0.8rem; }
  .btn-danger { background: var(--danger); }
  .btn-danger:hover { background: var(--danger-dark); }
  .btn-link {
    display: inline-block; padding: 0.3rem 0.7rem; font-size: 0.8rem; font-weight: 600;
    color: var(--accent); text-decoration: none; border: 1px solid var(--border); border-radius: 6px;
  }
  .btn-link:hover { background: var(--badge-bg); }
  .add-link { font-size: 0.88rem; font-weight: 600; color: var(--accent); text-decoration: none; }
</style>
`

const loginPage = styleBlock + themeToggle + `
<div class="login-shell">
  <h1>iptv-proxy admin</h1>
  {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
  <form method="post" action="/admin/login">
    <label>Username</label>
    <input type="text" name="username" required autofocus>
    <label>Password</label>
    <input type="password" name="password" required>
    <button type="submit" class="btn" style="width:100%">Log in</button>
  </form>
</div>
`

const dashboardPage = styleBlock + themeToggle + `
<div class="wrap">
  <header class="topbar">
    <h1>iptv-proxy admin</h1>
    <nav>
      <form class="inline" method="post" action="/admin/logout">
        <button type="submit" class="btn-link" style="background:none;border:none;cursor:pointer;">Log out</button>
      </form>
    </nav>
  </header>

  <div class="stats">
    <div class="stat-card">
      <div class="value" id="streamCount">{{len .ActiveStreams}}</div>
      <div class="label">Active streams</div>
    </div>
    <div class="stat-card">
      <div class="value">{{len .XtreamCodes}}</div>
      <div class="label">Xtream codes</div>
    </div>
    <div class="stat-card">
      <div class="value">{{len .Users}}</div>
      <div class="label">Users</div>
    </div>
  </div>

  <div class="panel">
    <div class="panel-header"><h2>Active streams</h2></div>
    <table id="streamsTable">
      <tr><th>User</th><th>Backend</th><th>Path</th><th>Duration</th></tr>
      <tbody id="streamsBody">
      {{range .ActiveStreams}}
      <tr>
        <td>{{.ProxyUser}}</td>
        <td>{{.Backend}}</td>
        <td class="mono">{{.Path}}</td>
        <td data-started="{{.StartedAtUnix}}" class="duration">0s</td>
      </tr>
      {{else}}
      <tr class="empty-row"><td colspan="4">No active streams</td></tr>
      {{end}}
      </tbody>
    </table>
  </div>

  <div class="panel">
    <div class="panel-header">
      <h2>Xtream codes</h2>
      <a class="add-link" href="/admin/xtream-codes/new">+ Add xtream code</a>
    </div>
    <table>
      <tr><th>Name</th><th>Base URL</th><th>Xtream user</th><th></th></tr>
      {{range .XtreamCodes}}
      <tr>
        <td><span class="badge">{{.Name}}</span></td>
        <td class="mono">{{.BaseURL}}</td>
        <td class="mono">{{.XtreamUser}}</td>
        <td class="actions">
          <a class="btn-link" href="/admin/xtream-codes/{{.ID}}/edit">Edit</a>
          <form class="inline" method="post" action="/admin/xtream-codes/{{.ID}}/delete">
            <button type="submit" class="btn btn-sm btn-danger">Delete</button>
          </form>
        </td>
      </tr>
      {{else}}
      <tr class="empty-row"><td colspan="4">No xtream codes yet</td></tr>
      {{end}}
    </table>
  </div>

  <div class="panel">
    <div class="panel-header">
      <h2>Users</h2>
      <a class="add-link" href="/admin/users/new">+ Add user</a>
    </div>
    <table>
      <tr><th>Username</th><th>Assigned xtream code</th><th></th></tr>
      {{range .Users}}
      <tr>
        <td>{{.Username}}</td>
        <td><span class="badge">{{.XtreamCodeName}}</span></td>
        <td class="actions">
          <a class="btn-link" href="/admin/users/{{.ID}}/edit">Edit</a>
          <form class="inline" method="post" action="/admin/users/{{.ID}}/delete">
            <button type="submit" class="btn btn-sm btn-danger">Delete</button>
          </form>
        </td>
      </tr>
      {{else}}
      <tr class="empty-row"><td colspan="3">No users yet</td></tr>
      {{end}}
    </table>
  </div>
</div>

<script>
function fmtDuration(seconds) {
  seconds = Math.max(0, Math.floor(seconds));
  var h = Math.floor(seconds / 3600);
  var m = Math.floor((seconds % 3600) / 60);
  var s = seconds % 60;
  var parts = [];
  if (h > 0) parts.push(h + 'h');
  if (h > 0 || m > 0) parts.push(m + 'm');
  parts.push(s + 's');
  return parts.join(' ');
}

function tickDurations() {
  document.querySelectorAll('#streamsBody .duration').forEach(function (cell) {
    var started = parseInt(cell.getAttribute('data-started'), 10);
    if (!started) return;
    cell.textContent = fmtDuration(Date.now() / 1000 - started);
  });
}

function refreshStreams() {
  fetch('/admin/streams.json', { credentials: 'same-origin' })
    .then(function (r) { return r.json(); })
    .then(function (data) {
      document.getElementById('streamCount').textContent = data.count;
      var body = document.getElementById('streamsBody');
      if (data.count === 0) {
        body.innerHTML = '<tr class="empty-row"><td colspan="4">No active streams</td></tr>';
        return;
      }
      var rows = data.streams.map(function (s) {
        return '<tr><td>' + escapeHtml(s.ProxyUser) + '</td><td>' + escapeHtml(s.Backend) +
          '</td><td class="mono">' + escapeHtml(s.Path) + '</td>' +
          '<td data-started="' + s.StartedAtUnix + '" class="duration">0s</td></tr>';
      });
      body.innerHTML = rows.join('');
      tickDurations();
    })
    .catch(function () {});
}

function escapeHtml(s) {
  var div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

tickDurations();
setInterval(tickDurations, 1000);
setInterval(refreshStreams, 5000);
</script>
`

const xtreamCodeFormPage = styleBlock + themeToggle + `
<div class="wrap">
  <header class="topbar"><h1>iptv-proxy admin</h1><nav><a href="/admin">&larr; Dashboard</a></nav></header>
  <div class="panel" style="max-width:480px;">
    <h2>{{if .ID}}Edit{{else}}New{{end}} xtream code</h2>
    {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
    <form method="post" action="{{.Action}}">
      <label>Name</label>
      <input type="text" name="name" value="{{.Name}}" required>
      <label>Base URL</label>
      <input type="text" name="base_url" value="{{.BaseURL}}" placeholder="http://example.tv:8080" required>
      <label>Xtream username</label>
      <input type="text" name="xtream_user" value="{{.XtreamUser}}" required>
      <label>Xtream password</label>
      <input type="text" name="xtream_password" value="{{.XtreamPassword}}" required>
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
      <label>Username</label>
      <input type="text" name="username" value="{{.Username}}" required>
      <label>Password{{if .ID}} (leave blank to keep current){{end}}</label>
      <input type="password" name="password" {{if not .ID}}required{{end}}>
      <label>Xtream code</label>
      <select name="xtream_code_id" required>
        {{range .XtreamCodes}}
        <option value="{{.ID}}" {{if eq .ID $.XtreamCodeID}}selected{{end}}>{{.Name}}</option>
        {{end}}
      </select>
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
