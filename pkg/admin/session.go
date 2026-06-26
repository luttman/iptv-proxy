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
	expiry := time.Now().Add(sessionTTL).Unix()
	ctx.SetCookie(sessionCookieName, a.signer.sign(expiry), int(sessionTTL.Seconds()), "/admin", "", false, true)
}

func (a *admin) clearSession(ctx *gin.Context) {
	ctx.SetCookie(sessionCookieName, "", -1, "/admin", "", false, true)
}

func (a *admin) requireSession(ctx *gin.Context) {
	cookie, err := ctx.Cookie(sessionCookieName)
	if err != nil || !a.signer.verify(cookie) {
		ctx.Redirect(http.StatusFound, "/admin/login")
		ctx.Abort()
		return
	}
}

func checkCredentials(want Credentials, username, password string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(want.Username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(want.Password)) == 1

	return userOK && passOK
}
