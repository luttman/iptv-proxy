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

// Package admin provides a small server-rendered web UI for managing
// proxy users and the xtream-code backends they're assigned to.
package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const sessionCookieName = "iptvproxy_admin_session"
const csrfCookieName = "iptvproxy_admin_csrf"
const sessionTTL = 24 * time.Hour

// Credentials are the single admin login, supplied at process
// startup (flags/env), not stored in the database.
type Credentials struct {
	Username string
	Password string
}

// sessionSecret is generated once per process start. Restarting the
// process invalidates all existing admin sessions — acceptable for a
// single-admin tool, and avoids the complexity of persisting a secret.
type sessionSigner struct {
	secret []byte
}

func newSessionSigner() (*sessionSigner, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate session secret: %w", err)
	}

	return &sessionSigner{secret: secret}, nil
}

func (s *sessionSigner) sign(expiry int64) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(strconv.FormatInt(expiry, 10))) // nolint: errcheck
	sig := mac.Sum(nil)

	return strconv.FormatInt(expiry, 10) + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (s *sessionSigner) verify(token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}

	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expiry {
		return false
	}

	want := s.sign(expiry)
	return subtle.ConstantTimeCompare([]byte(token), []byte(want)) == 1
}

func (a *admin) issueSession(ctx *gin.Context) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	expiry := time.Now().Add(sessionTTL).Unix()
	ctx.SetCookie(sessionCookieName, a.signer.sign(expiry), int(sessionTTL.Seconds()), "/admin", "", a.secureCookies, true)
	a.issueCSRFToken(ctx)
}

func (a *admin) clearSession(ctx *gin.Context) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(sessionCookieName, "", -1, "/admin", "", a.secureCookies, true)
	ctx.SetCookie(csrfCookieName, "", -1, "/admin", "", a.secureCookies, true)
}

func (a *admin) requireSession(ctx *gin.Context) {
	cookie, err := ctx.Cookie(sessionCookieName)
	if err != nil || !a.signer.verify(cookie) {
		if strings.Contains(ctx.GetHeader("Accept"), "application/json") {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Session expired. Reload the dashboard and sign in again."})
			return
		}
		ctx.Redirect(http.StatusFound, "/admin/login")
		ctx.Abort()
		return
	}

	// Defensive: guarantee a CSRF cookie exists for any authenticated
	// page render, even if it was somehow cleared without the session
	// cookie also being cleared.
	if _, err := ctx.Cookie(csrfCookieName); err != nil {
		a.issueCSRFToken(ctx)
	}
}

// issueCSRFToken sets a fresh random CSRF token cookie and returns
// its value. Used with the double-submit-cookie pattern: forms embed
// this same value as a hidden field, and csrfProtect verifies the two
// match on state-changing requests. The cookie doesn't need to be
// readable by JS since the server (which already holds the request's
// cookies) is the one rendering the hidden field.
func (a *admin) issueCSRFToken(ctx *gin.Context) string {
	raw := make([]byte, 32)
	rand.Read(raw) // nolint: errcheck

	token := base64.RawURLEncoding.EncodeToString(raw)
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(csrfCookieName, token, int(sessionTTL.Seconds()), "/admin", "", a.secureCookies, true)

	return token
}

// csrfToken returns the current request's CSRF token, for embedding
// into a hidden form field. requireSession guarantees the cookie is
// present on every authenticated page render.
func (a *admin) csrfToken(ctx *gin.Context) string {
	token, _ := ctx.Cookie(csrfCookieName)
	return token
}

// csrfProtect rejects state-changing requests whose csrf_token form
// field doesn't match the csrf cookie (double-submit cookie pattern).
func (a *admin) csrfProtect(ctx *gin.Context) {
	cookie, err := ctx.Cookie(csrfCookieName)
	if err != nil || cookie == "" {
		ctx.String(http.StatusForbidden, "missing CSRF cookie")
		ctx.Abort()
		return
	}

	form := ctx.PostForm("csrf_token")
	if form == "" || subtle.ConstantTimeCompare([]byte(form), []byte(cookie)) != 1 {
		ctx.String(http.StatusForbidden, "invalid CSRF token")
		ctx.Abort()
		return
	}
}

func checkCredentials(want Credentials, username, password string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(want.Username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(want.Password)) == 1

	return userOK && passOK
}
