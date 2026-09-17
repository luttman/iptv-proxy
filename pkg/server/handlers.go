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
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func (c *Config) stream(ctx *gin.Context, oriURL *url.URL, ru resolvedUser) {
	// A cancelable child of the request context, so a stream can be
	// forcibly torn down from the admin UI (some IPTV players don't
	// cleanly close the old connection when switching channels,
	// leaving an orphaned stream that would otherwise never end).
	reqCtx, cancel := context.WithCancel(ctx.Request.Context())
	defer cancel()

	resp, err := c.doUpstreamRequest(reqCtx, ctx, oriURL)
	if err != nil || resp.StatusCode >= http.StatusInternalServerError {
		// Nothing has been written to the client yet, so it's still
		// safe to fail over to another address: connect failures,
		// timeouts before a response, and server errors are transient
		// upstream problems, not something a different address would
		// also return (invalid credentials, a missing channel, and
		// connection-limit responses come back as 4xx and are left
		// alone here).
		if resp != nil {
			resp.Body.Close() // nolint: errcheck
		}
		if altURL, ok := c.failoverURL(reqCtx, oriURL, ru); ok {
			if altResp, altErr := c.doUpstreamRequest(reqCtx, ctx, altURL); altErr == nil {
				resp, err = altResp, nil
			}
		}
	}
	if err != nil {
		ctx.AbortWithError(http.StatusBadGateway, err) // nolint: errcheck
		return
	}
	defer resp.Body.Close() // nolint: errcheck

	streamID := c.streams.start(ru, ctx.Request.URL.Path, cancel)
	defer c.streams.end(streamID)

	mergeHttpHeader(ctx.Writer.Header(), resp.Header)
	ctx.Status(resp.StatusCode)
	ctx.Stream(func(w io.Writer) bool {
		io.Copy(&countingWriter{w: w, counter: &c.bandwidthBytes}, resp.Body) // nolint: errcheck
		return false
	})
}

func (c *Config) doUpstreamRequest(reqCtx context.Context, ctx *gin.Context, oriURL *url.URL) (*http.Response, error) {
	req, err := http.NewRequestWithContext(reqCtx, "GET", oriURL.String(), nil)
	if err != nil {
		return nil, err
	}
	mergeHttpHeader(req.Header, ctx.Request.Header)

	return c.httpClient.Do(req)
}

// failoverURL rebuilds oriURL against another of ru.Backend's
// addresses (the same account and channel path, just a different
// host), for retrying a request whose upstream just failed.
func (c *Config) failoverURL(ctx context.Context, oriURL *url.URL, ru resolvedUser) (*url.URL, bool) {
	failed := strings.TrimRight(oriURL.Scheme+"://"+oriURL.Host, "/")

	// ru.Backend.BaseURL was already narrowed to a single chosen
	// address at auth time; re-fetch the backend to get back its full
	// list of addresses to fail over among.
	backend, err := c.Store.GetXtreamCode(ru.Backend.ID)
	if err != nil {
		return nil, false
	}

	alt, err := c.selectUpstream(ctx, backend, failed)
	if err != nil {
		return nil, false
	}

	altBase, err := url.Parse(alt.BaseURL)
	if err != nil {
		return nil, false
	}

	newURL := *oriURL
	newURL.Scheme = altBase.Scheme
	newURL.Host = altBase.Host
	return &newURL, true
}

func (c *Config) xtreamStream(ctx *gin.Context, oriURL *url.URL, ru resolvedUser) {
	id := ctx.Param("id")
	if strings.HasSuffix(id, ".m3u8") {
		c.hlsXtreamStream(ctx, oriURL, ru)
		return
	}

	c.stream(ctx, oriURL, ru)
}

type values []string

func (vs values) contains(s string) bool {
	for _, v := range vs {
		if v == s {
			return true
		}
	}

	return false
}

func mergeHttpHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			if values(dst.Values(k)).contains(v) {
				continue
			}
			dst.Add(k, v)
		}
	}
}

// authRequest handle auth credentials
type authRequest struct {
	Username string `form:"username" binding:"required"`
	Password string `form:"password" binding:"required"`
}

func (c *Config) authenticate(ctx *gin.Context) {
	ip := ctx.ClientIP()
	if c.authLimiter.Blocked(ip) {
		ctx.AbortWithStatus(http.StatusTooManyRequests)
		return
	}

	var authReq authRequest
	if err := ctx.Bind(&authReq); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, err) // nolint: errcheck
		return
	}

	user, xc, err := c.Store.Authenticate(authReq.Username, authReq.Password)
	if err != nil {
		c.authLimiter.RecordFailure(ip)
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.authLimiter.RecordSuccess(ip)
	xc, err = c.selectUpstream(ctx.Request.Context(), xc, "")
	if err != nil {
		ctx.AbortWithError(http.StatusBadGateway, err) // nolint: errcheck
		return
	}

	setResolvedUser(ctx, resolvedUser{ProxyUser: user.Username, ProxyPassword: authReq.Password, Backend: xc, MaxConcurrentStreams: user.MaxConcurrentStreams})
}

func (c *Config) appAuthenticate(ctx *gin.Context) {
	ip := ctx.ClientIP()
	if c.authLimiter.Blocked(ip) {
		ctx.AbortWithStatus(http.StatusTooManyRequests)
		return
	}

	contents, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}

	q, err := url.ParseQuery(string(contents))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}
	if len(q["username"]) == 0 || len(q["password"]) == 0 {
		ctx.AbortWithError(http.StatusBadRequest, fmt.Errorf("bad body url query parameters")) // nolint: errcheck
		return
	}
	slog.Info("app auth", "client", ctx.ClientIP())

	username, password := q["username"][0], q["password"][0]
	user, xc, err := c.Store.Authenticate(username, password)
	if err != nil {
		c.authLimiter.RecordFailure(ip)
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.authLimiter.RecordSuccess(ip)
	xc, err = c.selectUpstream(ctx.Request.Context(), xc, "")
	if err != nil {
		ctx.AbortWithError(http.StatusBadGateway, err) // nolint: errcheck
		return
	}
	setResolvedUser(ctx, resolvedUser{ProxyUser: user.Username, ProxyPassword: password, Backend: xc, MaxConcurrentStreams: user.MaxConcurrentStreams})

	ctx.Request.Body = io.NopCloser(bytes.NewReader(contents))
}
