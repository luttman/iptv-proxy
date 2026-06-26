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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	xtreamapi "github.com/pierre-emmanuelJ/iptv-proxy/pkg/xtream-proxy"
)

func (c *Config) xtreamPlayerAPIGET(ctx *gin.Context) {
	c.xtreamPlayerAPI(ctx, ctx.Request.URL.Query())
}

func (c *Config) xtreamPlayerAPIPOST(ctx *gin.Context) {
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

	c.xtreamPlayerAPI(ctx, q)
}

func (c *Config) xtreamPlayerAPI(ctx *gin.Context, q url.Values) {
	ru, ok := resolvedUserFromCtx(ctx)
	if !ok {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var action string
	if len(q["action"]) > 0 {
		action = q["action"][0]
	}

	client, err := xtreamapi.New(ctx.Request.Context(), ru.Backend.XtreamUser, ru.Backend.XtreamPassword, ru.Backend.BaseURL, ctx.Request.UserAgent())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}

	resp, httpcode, err := client.Action(ru.ProxyUser, ru.ProxyPassword, c.HostConfig, c.HTTPS, c.AdvertisedPort, action, q)
	if err != nil {
		ctx.AbortWithError(httpcode, err) // nolint: errcheck
		return
	}

	slog.Info("xtream action", "client", ctx.ClientIP(), "action", action)

	ctx.JSON(http.StatusOK, resp)
}

// xtreamXMLTV proxies xmltv.php directly, rather than going through
// xtreamapi.New (which performs a separate player_api.php login
// first). Two reasons: (1) the vendored go.xtream-codes library
// appends a bogus "&action=xmltv.php" to the request when fetching
// XMLTV through that path, which some providers reject outright with
// a connection reset; (2) the extra login burns a connection slot on
// providers with strict concurrent-connection limits, which can fail
// EPG fetches while a stream is already active.
func (c *Config) xtreamXMLTV(ctx *gin.Context) {
	ru, ok := resolvedUserFromCtx(ctx)
	if !ok {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	rawURL := fmt.Sprintf(
		"%s/xmltv.php?username=%s&password=%s",
		ru.Backend.BaseURL,
		url.QueryEscape(ru.Backend.XtreamUser),
		url.QueryEscape(ru.Backend.XtreamPassword),
	)

	req, err := http.NewRequestWithContext(ctx.Request.Context(), "GET", rawURL, nil)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}
	req.Header.Set("User-Agent", ctx.Request.UserAgent())

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}

	ctx.Data(resp.StatusCode, "application/xml", body)
}
