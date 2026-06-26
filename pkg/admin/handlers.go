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

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

func (a *admin) loginPage(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(ctx.Writer, "login", gin.H{}) // nolint: errcheck
}

func (a *admin) login(ctx *gin.Context) {
	ip := ctx.ClientIP()
	if a.loginLimiter.Blocked(ip) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.Status(http.StatusTooManyRequests)
		templates.ExecuteTemplate(ctx.Writer, "login", gin.H{"Error": "Too many failed attempts. Try again later."}) // nolint: errcheck
		return
	}

	username := ctx.PostForm("username")
	password := ctx.PostForm("password")

	if !checkCredentials(a.creds, username, password) {
		a.loginLimiter.RecordFailure(ip)
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.Status(http.StatusUnauthorized)
		templates.ExecuteTemplate(ctx.Writer, "login", gin.H{"Error": "Invalid username or password"}) // nolint: errcheck
		return
	}

	a.loginLimiter.RecordSuccess(ip)
	a.issueSession(ctx)
	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) logout(ctx *gin.Context) {
	a.clearSession(ctx)
	ctx.Redirect(http.StatusFound, "/admin/login")
}

type dashboardUserRow struct {
	store.User
	XtreamCodeName string
}

// streamRow mirrors server.ActiveStream with a Unix-timestamp field
// the dashboard's JS can use directly (both for the initial
// server-rendered page and the polled JSON endpoint).
type streamRow struct {
	ID            int64
	ProxyUser     string
	Backend       string
	Path          string
	StartedAtUnix int64
}

func (a *admin) streamRows() []streamRow {
	active := a.srv.ActiveStreams()
	rows := make([]streamRow, 0, len(active))
	for _, s := range active {
		rows = append(rows, streamRow{
			ID:            s.ID,
			ProxyUser:     s.ProxyUser,
			Backend:       s.Backend,
			Path:          s.Path,
			StartedAtUnix: s.StartedAt.Unix(),
		})
	}

	return rows
}

func (a *admin) streamsJSON(ctx *gin.Context) {
	rows := a.streamRows()
	ctx.JSON(http.StatusOK, gin.H{"count": len(rows), "streams": rows})
}

func (a *admin) streamStop(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	if !a.srv.StopStream(id) {
		ctx.String(http.StatusNotFound, "stream not found (it may have already ended)")
		return
	}

	ctx.Status(http.StatusNoContent)
}

func (a *admin) dashboard(ctx *gin.Context) {
	codes, err := a.srv.Store.ListXtreamCodes()
	if err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	names := make(map[int64]string, len(codes))
	for _, c := range codes {
		names[c.ID] = c.Name
	}

	users, err := a.srv.Store.ListUsers()
	if err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	rows := make([]dashboardUserRow, 0, len(users))
	for _, u := range users {
		rows = append(rows, dashboardUserRow{User: u, XtreamCodeName: names[u.XtreamCodeID]})
	}

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(ctx.Writer, "dashboard", gin.H{ // nolint: errcheck
		"XtreamCodes":   codes,
		"Users":         rows,
		"ActiveStreams": a.streamRows(),
		"CSRFToken":     a.csrfToken(ctx),
	})
}

func (a *admin) xtreamCodeNewForm(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(ctx.Writer, "xtreamCodeForm", gin.H{ // nolint: errcheck
		"Action":    "/admin/xtream-codes/new",
		"CSRFToken": a.csrfToken(ctx),
	})
}

func (a *admin) xtreamCodeCreate(ctx *gin.Context) {
	_, err := a.srv.Store.CreateXtreamCode(
		ctx.PostForm("name"),
		ctx.PostForm("base_url"),
		ctx.PostForm("xtream_user"),
		ctx.PostForm("xtream_password"),
	)
	if err != nil {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		templates.ExecuteTemplate(ctx.Writer, "xtreamCodeForm", gin.H{ // nolint: errcheck
			"Action": "/admin/xtream-codes/new",
			"Error":  err.Error(), "CSRFToken": a.csrfToken(ctx),
			"Name": ctx.PostForm("name"), "BaseURL": ctx.PostForm("base_url"),
			"XtreamUser": ctx.PostForm("xtream_user"), "XtreamPassword": ctx.PostForm("xtream_password"),
		})
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) xtreamCodeEditForm(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	xc, err := a.srv.Store.GetXtreamCode(id)
	if err != nil {
		ctx.String(http.StatusNotFound, "%s", err)
		return
	}

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(ctx.Writer, "xtreamCodeForm", gin.H{ // nolint: errcheck
		"Action": "/admin/xtream-codes/" + ctx.Param("id") + "/edit", "CSRFToken": a.csrfToken(ctx),
		"ID": xc.ID, "Name": xc.Name, "BaseURL": xc.BaseURL,
		"XtreamUser": xc.XtreamUser, "XtreamPassword": xc.XtreamPassword,
	})
}

func (a *admin) xtreamCodeUpdate(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	_, err = a.srv.Store.UpdateXtreamCode(
		id,
		ctx.PostForm("name"),
		ctx.PostForm("base_url"),
		ctx.PostForm("xtream_user"),
		ctx.PostForm("xtream_password"),
	)
	if err != nil {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		templates.ExecuteTemplate(ctx.Writer, "xtreamCodeForm", gin.H{ // nolint: errcheck
			"Action": "/admin/xtream-codes/" + ctx.Param("id") + "/edit", "CSRFToken": a.csrfToken(ctx),
			"ID": id, "Error": err.Error(),
			"Name": ctx.PostForm("name"), "BaseURL": ctx.PostForm("base_url"),
			"XtreamUser": ctx.PostForm("xtream_user"), "XtreamPassword": ctx.PostForm("xtream_password"),
		})
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) xtreamCodeDelete(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	if err := a.srv.Store.DeleteXtreamCode(id); err != nil {
		if errors.Is(err, store.ErrXtreamCodeInUse) {
			ctx.String(http.StatusConflict, "cannot delete: one or more users are still assigned to this xtream code")
			return
		}
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

// maxStreamsFromForm parses the max_concurrent_streams field,
// defaulting to 1 (matching the database column default) if it's
// missing or invalid rather than silently allowing unlimited streams.
func maxStreamsFromForm(ctx *gin.Context) int {
	n, err := strconv.Atoi(ctx.PostForm("max_concurrent_streams"))
	if err != nil || n < 0 {
		return 1
	}

	return n
}

func (a *admin) userNewForm(ctx *gin.Context) {
	codes, err := a.srv.Store.ListXtreamCodes()
	if err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(ctx.Writer, "userForm", gin.H{ // nolint: errcheck
		"Action":               "/admin/users/new",
		"XtreamCodes":          codes,
		"CSRFToken":            a.csrfToken(ctx),
		"MaxConcurrentStreams": 1,
	})
}

func (a *admin) userCreate(ctx *gin.Context) {
	codes, err := a.srv.Store.ListXtreamCodes()
	if err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	maxStreams := maxStreamsFromForm(ctx)
	xtreamCodeID, err := strconv.ParseInt(ctx.PostForm("xtream_code_id"), 10, 64)
	if err == nil {
		_, err = a.srv.Store.CreateUser(ctx.PostForm("username"), ctx.PostForm("password"), xtreamCodeID, maxStreams)
	}
	if err != nil {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		templates.ExecuteTemplate(ctx.Writer, "userForm", gin.H{ // nolint: errcheck
			"Action": "/admin/users/new", "Error": err.Error(), "CSRFToken": a.csrfToken(ctx),
			"Username": ctx.PostForm("username"), "XtreamCodes": codes, "XtreamCodeID": xtreamCodeID,
			"MaxConcurrentStreams": maxStreams,
		})
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) userEditForm(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	u, err := a.srv.Store.GetUser(id)
	if err != nil {
		ctx.String(http.StatusNotFound, "%s", err)
		return
	}

	codes, err := a.srv.Store.ListXtreamCodes()
	if err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(ctx.Writer, "userForm", gin.H{ // nolint: errcheck
		"Action": "/admin/users/" + ctx.Param("id") + "/edit", "CSRFToken": a.csrfToken(ctx),
		"ID": u.ID, "Username": u.Username, "XtreamCodeID": u.XtreamCodeID,
		"XtreamCodes":          codes,
		"MaxConcurrentStreams": u.MaxConcurrentStreams,
	})
}

func (a *admin) userUpdate(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	codes, err := a.srv.Store.ListXtreamCodes()
	if err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	maxStreams := maxStreamsFromForm(ctx)
	xtreamCodeID, err := strconv.ParseInt(ctx.PostForm("xtream_code_id"), 10, 64)
	if err == nil {
		_, err = a.srv.Store.UpdateUser(id, ctx.PostForm("username"), ctx.PostForm("password"), xtreamCodeID, maxStreams)
	}
	if err != nil {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		templates.ExecuteTemplate(ctx.Writer, "userForm", gin.H{ // nolint: errcheck
			"Action": "/admin/users/" + ctx.Param("id") + "/edit", "Error": err.Error(), "CSRFToken": a.csrfToken(ctx),
			"ID": id, "Username": ctx.PostForm("username"), "XtreamCodes": codes, "XtreamCodeID": xtreamCodeID,
			"MaxConcurrentStreams": maxStreams,
		})
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) userDelete(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	if err := a.srv.Store.DeleteUser(id); err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}
