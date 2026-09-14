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
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/jamesnetherton/m3u"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/ratelimit"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"

	"github.com/gin-gonic/gin"
)

// shutdownGracePeriod bounds how long Serve waits for in-flight
// requests (including active streams) to finish after a SIGTERM/
// SIGINT before forcibly closing remaining connections.
const shutdownGracePeriod = 30 * time.Second

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

	streams *streamTracker

	authLimiter *ratelimit.Limiter

	upstreamHealth     map[string]UpstreamHealth
	upstreamHealthLock sync.RWMutex

	// currentAddress remembers, per backend, the address failover last
	// picked, so selection can favor stability over chasing marginally
	// lower latency (see switchLatencyMarginMS).
	currentAddress     map[string]string
	currentAddressLock sync.Mutex

	subscriptionExpiry     map[int64]SubscriptionExpiry
	subscriptionExpiryLock sync.RWMutex

	// httpClient is shared across every proxied stream/API request so
	// upstream connections get pooled and reused instead of paying a
	// fresh TCP/TLS handshake on every channel switch. Its Transport
	// bounds connection setup and time-to-first-byte (DialContext,
	// ResponseHeaderTimeout) without capping how long an established
	// stream may run — a stream's *body* read has no deadline here, so
	// long-running video doesn't get cut off by these timeouts.
	httpClient *http.Client
}

// NewServer initializes a new server configuration backed by st.
func NewServer(conf *config.ProxyConfig, st *store.Store) (*Config, error) {
	c := &Config{
		ProxyConfig:            conf,
		Store:                  st,
		hlsChannelsRedirectURL: map[string]hlsRedirect{},
		xtreamM3uCache:         map[string]cacheMeta{},
		streams:                newStreamTracker(),
		authLimiter:            ratelimit.New(authAttemptLimit, authAttemptWindow),
		upstreamHealth:         map[string]UpstreamHealth{},
		subscriptionExpiry:     map[int64]SubscriptionExpiry{},
		httpClient:             newUpstreamHTTPClient(),
	}

	router := gin.Default()
	router.Use(cors.Default())
	c.routes(router.Group("/"))
	c.Router = router

	return c, nil
}

func newUpstreamHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ResponseHeaderTimeout: 15 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
		},
	}
}

// Serve the iptv-proxy api. Server-side timeouts here are deliberately
// limited to header reads and idle keep-alive connections — not
// ReadTimeout/WriteTimeout, which would bound the *entire* connection
// lifetime including response body writes and would cut off long
// streams.
//
// SIGTERM/SIGINT (e.g. a container stop or Ctrl-C) trigger a graceful
// shutdown: stop accepting new connections, but give in-flight
// requests — including active streams — up to shutdownGracePeriod to
// finish on their own before forcibly closing them.
func (c *Config) Serve() error {
	healthCtx, stopHealthChecks := context.WithCancel(context.Background())
	defer stopHealthChecks()
	go c.monitorUpstreams(healthCtx)
	go c.monitorSubscriptions(healthCtx)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", c.HostConfig.Port),
		Handler:           c.Router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		slog.Info("shutting down", "signal", sig.String(), "grace_period", shutdownGracePeriod)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return <-errCh
	}
}

// marshallInto writes playlist as an M3U file into into, rewriting
// every track's upstream xtream credentials to ru's proxy-facing
// credentials.
func (c *Config) marshallInto(into *os.File, playlist *m3u.Playlist, ru resolvedUser) error {
	filteredTrack := make([]m3u.Track, 0, len(playlist.Tracks))

	into.WriteString("#EXTM3U\n") // nolint: errcheck
	for _, track := range playlist.Tracks {
		var buffer bytes.Buffer

		buffer.WriteString("#EXTINF:")            // nolint: errcheck
		fmt.Fprintf(&buffer, "%d ", track.Length) // nolint: errcheck
		for i := range track.Tags {
			if i == len(track.Tags)-1 {
				fmt.Fprintf(&buffer, "%s=%q", track.Tags[i].Name, track.Tags[i].Value) // nolint: errcheck
				continue
			}
			fmt.Fprintf(&buffer, "%s=%q ", track.Tags[i].Name, track.Tags[i].Value) // nolint: errcheck
		}

		uri, err := c.replaceURL(track.URI, ru)
		if err != nil {
			slog.Error("track", "name", track.Name, "error", err)
			continue
		}

		fmt.Fprintf(into, "%s, %s\n%s\n", buffer.String(), track.Name, uri) // nolint: errcheck

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
