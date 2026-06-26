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

package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

// authAttemptLimit and authAttemptWindow bound credential brute-
// forcing against proxy users. Only failed attempts count, so a
// legitimate user's player hitting auth-protected endpoints
// repeatedly throughout the day is never throttled.
const (
	authAttemptLimit  = 20
	authAttemptWindow = 5 * time.Minute
)

// resolvedUser is the per-request identity established by
// authentication: which proxy user made the request, and which
// upstream xtream-code backend they're assigned to.
type resolvedUser struct {
	ProxyUser            string
	ProxyPassword        string
	Backend              store.XtreamCode
	MaxConcurrentStreams int
}

const resolvedUserKey = "iptv-proxy.resolvedUser"

func setResolvedUser(ctx *gin.Context, ru resolvedUser) {
	ctx.Set(resolvedUserKey, ru)
}

// resolvedUserFromCtx retrieves the resolvedUser stashed by the
// authenticate/appAuthenticate middleware.
func resolvedUserFromCtx(ctx *gin.Context) (resolvedUser, bool) {
	v, ok := ctx.Get(resolvedUserKey)
	if !ok {
		return resolvedUser{}, false
	}

	ru, ok := v.(resolvedUser)
	return ru, ok
}

// resolveFromPath authenticates a request that carries its
// credentials as :proxyUser/:proxyPass path parameters (the xtream
// stream endpoints, which have no middleware of their own since
// routes are compiled once at startup and can't embed per-user
// literal credentials). It aborts the request with 401 on failure.
func (c *Config) resolveFromPath(ctx *gin.Context) (resolvedUser, bool) {
	ip := ctx.ClientIP()
	if c.authLimiter.Blocked(ip) {
		ctx.AbortWithStatus(http.StatusTooManyRequests)
		return resolvedUser{}, false
	}

	proxyUser := ctx.Param("proxyUser")
	proxyPass := ctx.Param("proxyPass")

	user, xc, err := c.Store.Authenticate(proxyUser, proxyPass)
	if err != nil {
		c.authLimiter.RecordFailure(ip)
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return resolvedUser{}, false
	}
	c.authLimiter.RecordSuccess(ip)

	return resolvedUser{ProxyUser: user.Username, ProxyPassword: proxyPass, Backend: xc, MaxConcurrentStreams: user.MaxConcurrentStreams}, true
}
