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
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

func (c *Config) xtreamStreamHandler(ctx *gin.Context) {
	ru, ok := c.resolveFromPath(ctx)
	if !ok {
		return
	}

	id := ctx.Param("id")
	rpURL, err := url.Parse(fmt.Sprintf("%s/%s/%s/%s", ru.Backend.BaseURL, ru.Backend.XtreamUser, ru.Backend.XtreamPassword, id))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}

	c.xtreamStream(ctx, rpURL, ru)
}

func (c *Config) xtreamStreamLive(ctx *gin.Context) {
	ru, ok := c.resolveFromPath(ctx)
	if !ok {
		return
	}

	id := ctx.Param("id")
	rpURL, err := url.Parse(fmt.Sprintf("%s/live/%s/%s/%s", ru.Backend.BaseURL, ru.Backend.XtreamUser, ru.Backend.XtreamPassword, id))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}

	c.xtreamStream(ctx, rpURL, ru)
}

func (c *Config) xtreamStreamPlay(ctx *gin.Context) {
	ru, ok := c.resolveFromPath(ctx)
	if !ok {
		return
	}

	token := ctx.Param("token")
	t := ctx.Param("type")
	rpURL, err := url.Parse(fmt.Sprintf("%s/play/%s/%s", ru.Backend.BaseURL, token, t))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}

	c.xtreamStream(ctx, rpURL, ru)
}

func (c *Config) xtreamStreamTimeshift(ctx *gin.Context) {
	ru, ok := c.resolveFromPath(ctx)
	if !ok {
		return
	}

	duration := ctx.Param("duration")
	start := ctx.Param("start")
	id := ctx.Param("id")
	rpURL, err := url.Parse(fmt.Sprintf("%s/timeshift/%s/%s/%s/%s/%s", ru.Backend.BaseURL, ru.Backend.XtreamUser, ru.Backend.XtreamPassword, duration, start, id))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}

	c.stream(ctx, rpURL)
}

func (c *Config) xtreamStreamMovie(ctx *gin.Context) {
	ru, ok := c.resolveFromPath(ctx)
	if !ok {
		return
	}

	id := ctx.Param("id")
	rpURL, err := url.Parse(fmt.Sprintf("%s/movie/%s/%s/%s", ru.Backend.BaseURL, ru.Backend.XtreamUser, ru.Backend.XtreamPassword, id))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}

	c.xtreamStream(ctx, rpURL, ru)
}

func (c *Config) xtreamStreamSeries(ctx *gin.Context) {
	ru, ok := c.resolveFromPath(ctx)
	if !ok {
		return
	}

	id := ctx.Param("id")
	rpURL, err := url.Parse(fmt.Sprintf("%s/series/%s/%s/%s", ru.Backend.BaseURL, ru.Backend.XtreamUser, ru.Backend.XtreamPassword, id))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err) // nolint: errcheck
		return
	}

	c.xtreamStream(ctx, rpURL, ru)
}
