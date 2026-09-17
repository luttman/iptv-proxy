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
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/server"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

// formatBandwidth converts raw byte counters into the units people
// actually read (Mbps, GB), so the template stays free of math.
func formatBandwidth(stats server.BandwidthStats) gin.H {
	current := "—"
	if stats.CurrentKnown {
		current = fmt.Sprintf("%.1f Mbps", stats.CurrentBytesPerSec*8/1e6)
	}

	peak := "—"
	if stats.PeakKnown {
		peak = fmt.Sprintf("%.1f Mbps", stats.PeakBytesPerSec*8/1e6)
	}

	return gin.H{
		"Current":    current,
		"Peak":       peak,
		"PeakAtUnix": stats.PeakAtUnix,
		"Last24hGB":  fmt.Sprintf("%.2f", float64(stats.Last24hBytes)/1e9),
		"Last7dGB":   fmt.Sprintf("%.2f", float64(stats.Last7dBytes)/1e9),
	}
}

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

// codeWithCredentials is a provider alongside its credentials, used
// wherever the admin UI needs to let someone pick "which provider,
// which username" (the user form's grouped dropdown).
type codeWithCredentials struct {
	store.XtreamCode
	Credentials []store.XtreamCredential
}

func (a *admin) listCodesWithCredentials() ([]codeWithCredentials, error) {
	codes, err := a.srv.Store.ListXtreamCodes()
	if err != nil {
		return nil, err
	}

	result := make([]codeWithCredentials, 0, len(codes))
	for _, c := range codes {
		credentials, err := a.srv.Store.ListCredentials(c.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, codeWithCredentials{XtreamCode: c, Credentials: credentials})
	}

	return result, nil
}

type dashboardUserRow struct {
	store.User
	XtreamCodeName  string
	CredentialLabel string
}

func (a *admin) healthJSON(ctx *gin.Context) {
	health := a.srv.BackendHealth()
	online := 0
	for _, item := range health {
		if item.Up {
			online++
		}
	}
	ctx.JSON(http.StatusOK, gin.H{
		"online":    online,
		"total":     len(health),
		"addresses": health,
		"bandwidth": formatBandwidth(a.srv.BandwidthStats()),
	})
}

func (a *admin) dashboard(ctx *gin.Context) {
	codes, err := a.listCodesWithCredentials()
	if err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	type credentialInfo struct{ codeName, label string }
	credentials := map[int64]credentialInfo{}
	for _, c := range codes {
		for _, cred := range c.Credentials {
			credentials[cred.ID] = credentialInfo{codeName: c.Name, label: cred.Label()}
		}
	}

	addressesByCode := map[int64][]store.XtreamAddress{}
	for _, c := range codes {
		addrs, err := a.srv.Store.ListAddresses(c.ID)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "%s", err)
			return
		}
		addressesByCode[c.ID] = addrs
	}

	users, err := a.srv.Store.ListUsers()
	if err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	rows := make([]dashboardUserRow, 0, len(users))
	for _, u := range users {
		info := credentials[u.CredentialID]
		rows = append(rows, dashboardUserRow{User: u, XtreamCodeName: info.codeName, CredentialLabel: info.label})
	}
	health := a.srv.BackendHealth()
	online := 0
	for _, item := range health {
		if item.Up {
			online++
		}
	}

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(ctx.Writer, "dashboard", gin.H{ // nolint: errcheck
		"XtreamCodes":          codes,
		"AddressesByCode":      addressesByCode,
		"Users":                rows,
		"BackendHealth":        health,
		"OnlineBackends":       online,
		"SubscriptionExpiries": a.srv.SubscriptionExpiries(),
		"Bandwidth":            formatBandwidth(a.srv.BandwidthStats()),
		"CSRFToken":            a.csrfToken(ctx),
	})
}

// xtreamCodeCreate creates a provider from the "bulk add" dialog: a
// name, one or more addresses pasted at once, and its first
// credential. Further addresses/credentials are managed afterwards
// from the provider's Manage popup on the dashboard.
func (a *admin) xtreamCodeCreate(ctx *gin.Context) {
	xc, err := a.srv.Store.CreateXtreamCode(ctx.PostForm("name"))
	if err == nil {
		err = a.srv.Store.AddAddresses(xc.ID, ctx.PostForm("base_url"))
	}
	if err == nil {
		_, err = a.srv.Store.CreateCredential(xc.ID, ctx.PostForm("credential_name"), ctx.PostForm("xtream_user"), ctx.PostForm("xtream_password"))
	}
	if err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) xtreamCodeUpdate(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	if _, err := a.srv.Store.UpdateXtreamCodeName(id, ctx.PostForm("name")); err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
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
			ctx.String(http.StatusConflict, "cannot delete: one or more users are still assigned to a credential on this xtream code")
			return
		}
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) addressesBulkAdd(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	if err := a.srv.Store.AddAddresses(id, ctx.PostForm("addresses")); err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) addressToggle(ctx *gin.Context) {
	addrID, err := strconv.ParseInt(ctx.Param("addressId"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid address id")
		return
	}

	if err := a.srv.Store.SetAddressEnabled(addrID, ctx.PostForm("enabled") == "1"); err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) addressDelete(ctx *gin.Context) {
	addrID, err := strconv.ParseInt(ctx.Param("addressId"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid address id")
		return
	}

	if err := a.srv.Store.DeleteAddress(addrID); err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) credentialCreate(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	if _, err := a.srv.Store.CreateCredential(id, ctx.PostForm("name"), ctx.PostForm("xtream_user"), ctx.PostForm("xtream_password")); err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) credentialUpdate(ctx *gin.Context) {
	credID, err := strconv.ParseInt(ctx.Param("credId"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid credential id")
		return
	}

	if _, err := a.srv.Store.UpdateCredential(credID, ctx.PostForm("name"), ctx.PostForm("xtream_user"), ctx.PostForm("xtream_password")); err != nil {
		ctx.String(http.StatusInternalServerError, "%s", err)
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) credentialDelete(ctx *gin.Context) {
	credID, err := strconv.ParseInt(ctx.Param("credId"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid credential id")
		return
	}

	if err := a.srv.Store.DeleteCredential(credID); err != nil {
		if errors.Is(err, store.ErrCredentialInUse) {
			ctx.String(http.StatusConflict, "cannot delete: one or more users are still assigned to this credential")
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

func (a *admin) userCreate(ctx *gin.Context) {
	maxStreams := maxStreamsFromForm(ctx)
	credentialID, err := strconv.ParseInt(ctx.PostForm("credential_id"), 10, 64)
	if err == nil {
		_, err = a.srv.Store.CreateUser(ctx.PostForm("username"), ctx.PostForm("password"), credentialID, maxStreams)
	}
	if err != nil {
		ctx.String(http.StatusBadRequest, "%s", err)
		return
	}

	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) userUpdate(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusBadRequest, "invalid id")
		return
	}

	maxStreams := maxStreamsFromForm(ctx)
	credentialID, err := strconv.ParseInt(ctx.PostForm("credential_id"), 10, 64)
	if err == nil {
		_, err = a.srv.Store.UpdateUser(id, ctx.PostForm("username"), ctx.PostForm("password"), credentialID, maxStreams)
	}
	if err != nil {
		ctx.String(http.StatusBadRequest, "%s", err)
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
