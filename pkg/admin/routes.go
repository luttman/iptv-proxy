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
	"fmt"
	"time"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/ratelimit"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/server"
)

// loginAttemptLimit and loginAttemptWindow bound brute-forcing of the
// single admin account: 5 wrong passwords from one IP within 5
// minutes locks that IP out of /admin/login until the window rolls
// over.
const (
	loginAttemptLimit  = 5
	loginAttemptWindow = 5 * time.Minute
)

type admin struct {
	srv           *server.Config
	creds         Credentials
	signer        *sessionSigner
	secureCookies bool
	loginLimiter  *ratelimit.Limiter
}

// Register mounts the admin management UI at /admin on srv's Gin
// router. It must be called before srv.Serve().
func Register(srv *server.Config, creds Credentials) error {
	signer, err := newSessionSigner()
	if err != nil {
		return fmt.Errorf("register admin UI: %w", err)
	}

	a := &admin{
		srv: srv, creds: creds, signer: signer, secureCookies: srv.HTTPS,
		loginLimiter: ratelimit.New(loginAttemptLimit, loginAttemptWindow),
	}

	r := srv.Router.Group("/admin")
	r.GET("/login", a.loginPage)
	r.POST("/login", a.login)

	authed := r.Group("", a.requireSession)
	authed.POST("/logout", a.csrfProtect, a.logout)
	authed.GET("", a.dashboard)
	authed.GET("/health.json", a.healthJSON)
	authed.GET("/xtream-codes/new", a.xtreamCodeNewForm)
	authed.POST("/xtream-codes/new", a.csrfProtect, a.xtreamCodeCreate)
	authed.GET("/xtream-codes/:id/edit", a.xtreamCodeEditForm)
	authed.POST("/xtream-codes/:id/edit", a.csrfProtect, a.xtreamCodeUpdate)
	authed.POST("/xtream-codes/:id/delete", a.csrfProtect, a.xtreamCodeDelete)
	authed.POST("/xtream-codes/:id/addresses", a.csrfProtect, a.addressesBulkAdd)
	authed.POST("/xtream-codes/:id/addresses/:addressId/toggle", a.csrfProtect, a.addressToggle)
	authed.POST("/xtream-codes/:id/addresses/:addressId/delete", a.csrfProtect, a.addressDelete)
	authed.POST("/xtream-codes/:id/credentials", a.csrfProtect, a.credentialCreate)
	authed.POST("/xtream-codes/:id/credentials/:credId/delete", a.csrfProtect, a.credentialDelete)
	authed.GET("/users/new", a.userNewForm)
	authed.POST("/users/new", a.csrfProtect, a.userCreate)
	authed.GET("/users/:id/edit", a.userEditForm)
	authed.POST("/users/:id/edit", a.csrfProtect, a.userUpdate)
	authed.POST("/users/:id/delete", a.csrfProtect, a.userDelete)

	return nil
}
