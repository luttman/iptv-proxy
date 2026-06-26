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
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/jamesnetherton/m3u"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"

	"github.com/gin-gonic/gin"
)

type cacheMeta struct {
	string
	time.Time
}

// hlsRedirect remembers, for a given HLS token, both the upstream
// redirect location and which proxy user/backend resolved it — the
// chunk follow-up requests (/hls/:token/:chunk) carry no credentials
// of their own.
type hlsRedirect struct {
	url.URL
	resolvedUser
}

// Config represents the server configuration. A single Config serves
// every proxy user; per-user/per-backend identity is resolved per
// request from Store, never stored on Config itself.
type Config struct {
	*config.ProxyConfig

	Store *store.Store

	// Router is exposed so other packages (the admin UI) can mount
	// additional routes on the same Gin engine before Serve is called.
	Router *gin.Engine

	hlsChannelsRedirectURL     map[string]hlsRedirect
	hlsChannelsRedirectURLLock sync.RWMutex

	xtreamM3uCache     map[string]cacheMeta
	xtreamM3uCacheLock sync.RWMutex
}

// NewServer initializes a new server configuration backed by st.
func NewServer(conf *config.ProxyConfig, st *store.Store) (*Config, error) {
	c := &Config{
		ProxyConfig:            conf,
		Store:                  st,
		hlsChannelsRedirectURL: map[string]hlsRedirect{},
		xtreamM3uCache:         map[string]cacheMeta{},
	}

	router := gin.Default()
	router.Use(cors.Default())
	c.routes(router.Group("/"))
	c.Router = router

	return c, nil
}

// Serve the iptv-proxy api
func (c *Config) Serve() error {
	return c.Router.Run(fmt.Sprintf(":%d", c.HostConfig.Port))
}

// marshallInto writes playlist as an M3U file into into, rewriting
// every track's upstream xtream credentials to ru's proxy-facing
// credentials.
func (c *Config) marshallInto(into *os.File, playlist *m3u.Playlist, ru resolvedUser) error {
	filteredTrack := make([]m3u.Track, 0, len(playlist.Tracks))

	into.WriteString("#EXTM3U\n") // nolint: errcheck
	for _, track := range playlist.Tracks {
		var buffer bytes.Buffer

		buffer.WriteString("#EXTINF:")                       // nolint: errcheck
		buffer.WriteString(fmt.Sprintf("%d ", track.Length)) // nolint: errcheck
		for i := range track.Tags {
			if i == len(track.Tags)-1 {
				buffer.WriteString(fmt.Sprintf("%s=%q", track.Tags[i].Name, track.Tags[i].Value)) // nolint: errcheck
				continue
			}
			buffer.WriteString(fmt.Sprintf("%s=%q ", track.Tags[i].Name, track.Tags[i].Value)) // nolint: errcheck
		}

		uri, err := c.replaceURL(track.URI, ru)
		if err != nil {
			slog.Error("track", "name", track.Name, "error", err)
			continue
		}

		into.WriteString(fmt.Sprintf("%s, %s\n%s\n", buffer.String(), track.Name, uri)) // nolint: errcheck

		filteredTrack = append(filteredTrack, track)
	}
	playlist.Tracks = filteredTrack

	return into.Sync()
}

// replaceURL rewrites an upstream xtream track URL so that it points
// back at this proxy using ru's proxy-facing credentials instead of
// the upstream backend's credentials.
func (c *Config) replaceURL(uri string, ru resolvedUser) (string, error) {
	oriURL, err := url.Parse(uri)
	if err != nil {
		return "", err
	}

	protocol := "http"
	if c.HTTPS {
		protocol = "https"
	}

	customEnd := strings.Trim(c.CustomEndpoint, "/")
	if customEnd != "" {
		customEnd = fmt.Sprintf("/%s", customEnd)
	}

	uriPath := oriURL.EscapedPath()
	uriPath = strings.ReplaceAll(uriPath, url.PathEscape(ru.Backend.XtreamUser), url.PathEscape(ru.ProxyUser))
	uriPath = strings.ReplaceAll(uriPath, url.PathEscape(ru.Backend.XtreamPassword), url.PathEscape(ru.ProxyPassword))

	basicAuth := oriURL.User.String()
	if basicAuth != "" {
		basicAuth += "@"
	}

	newURI := fmt.Sprintf(
		"%s://%s%s:%d%s%s",
		protocol,
		basicAuth,
		c.HostConfig.Hostname,
		c.AdvertisedPort,
		customEnd,
		uriPath,
	)

	newURL, err := url.Parse(newURI)
	if err != nil {
		return "", err
	}

	return newURL.String(), nil
}
