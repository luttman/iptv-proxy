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

const styleBlock = `
<style>
  body { font-family: system-ui, sans-serif; max-width: 900px; margin: 2rem auto; color: #1a1a1a; }
  h1 { font-size: 1.4rem; }
  h2 { font-size: 1.1rem; margin-top: 2rem; }
  table { width: 100%; border-collapse: collapse; margin-bottom: 1rem; }
  th, td { text-align: left; padding: 0.4rem 0.6rem; border-bottom: 1px solid #ddd; }
  form.inline { display: inline; }
  .error { color: #b00020; }
  .actions a, .actions button { margin-right: 0.5rem; }
  label { display: block; margin-top: 0.8rem; font-weight: 600; }
  input, select { padding: 0.4rem; width: 100%; max-width: 320px; margin-top: 0.2rem; }
  button { margin-top: 1rem; padding: 0.5rem 1rem; cursor: pointer; }
  nav { margin-bottom: 1.5rem; }
  nav a { margin-right: 1rem; }
</style>
`

const loginPage = styleBlock + `
<h1>iptv-proxy admin</h1>
{{if .Error}}<p class="error">{{.Error}}</p>{{end}}
<form method="post" action="/admin/login">
  <label>Username</label>
  <input type="text" name="username" required>
  <label>Password</label>
  <input type="password" name="password" required>
  <button type="submit">Log in</button>
</form>
`

const dashboardPage = styleBlock + `
<nav><a href="/admin">Dashboard</a> | <form class="inline" method="post" action="/admin/logout"><button type="submit">Log out</button></form></nav>
<h1>iptv-proxy admin</h1>

<h2>Xtream codes</h2>
<table>
  <tr><th>Name</th><th>Base URL</th><th>Xtream user</th><th></th></tr>
  {{range .XtreamCodes}}
  <tr>
    <td>{{.Name}}</td>
    <td>{{.BaseURL}}</td>
    <td>{{.XtreamUser}}</td>
    <td class="actions">
      <a href="/admin/xtream-codes/{{.ID}}/edit">Edit</a>
      <form class="inline" method="post" action="/admin/xtream-codes/{{.ID}}/delete">
        <button type="submit">Delete</button>
      </form>
    </td>
  </tr>
  {{end}}
</table>
<a href="/admin/xtream-codes/new">+ Add xtream code</a>

<h2>Users</h2>
<table>
  <tr><th>Username</th><th>Assigned xtream code</th><th></th></tr>
  {{range .Users}}
  <tr>
    <td>{{.Username}}</td>
    <td>{{.XtreamCodeName}}</td>
    <td class="actions">
      <a href="/admin/users/{{.ID}}/edit">Edit</a>
      <form class="inline" method="post" action="/admin/users/{{.ID}}/delete">
        <button type="submit">Delete</button>
      </form>
    </td>
  </tr>
  {{end}}
</table>
<a href="/admin/users/new">+ Add user</a>
`

const xtreamCodeFormPage = styleBlock + `
<nav><a href="/admin">Dashboard</a></nav>
<h1>{{if .ID}}Edit{{else}}New{{end}} xtream code</h1>
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
  <button type="submit">Save</button>
</form>
`

const userFormPage = styleBlock + `
<nav><a href="/admin">Dashboard</a></nav>
<h1>{{if .ID}}Edit{{else}}New{{end}} user</h1>
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
  <button type="submit">Save</button>
</form>
`

var templates = template.Must(template.New("root").Parse(""))

func init() {
	template.Must(templates.New("login").Parse(loginPage))
	template.Must(templates.New("dashboard").Parse(dashboardPage))
	template.Must(templates.New("xtreamCodeForm").Parse(xtreamCodeFormPage))
	template.Must(templates.New("userForm").Parse(userFormPage))
}
