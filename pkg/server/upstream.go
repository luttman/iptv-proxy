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
	"context"
	"errors"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

const (
	upstreamProbeTimeout = 2 * time.Second
	healthCheckInterval  = 30 * time.Second
	recentHealthSamples  = 60
)

type probeResult struct {
	baseURL string
	delay   time.Duration
}

// UpstreamHealth is the current and recent status of one backend address.
type UpstreamHealth struct {
	Backend       string
	BaseURL       string
	Up            bool
	LatencyMS     int64
	UptimePercent int
	CheckedAtUnix int64
	History       []bool
}

// selectUpstream chooses the reachable service with the quickest TCP
// connection. A TCP probe checks the port the IPTV service actually uses,
// unlike ICMP ping, which many hosts block.
func selectUpstream(ctx context.Context, backend store.XtreamCode) (store.XtreamCode, error) {
	addresses := upstreamAddresses(backend)
	if len(addresses) == 1 {
		backend.BaseURL = strings.TrimRight(addresses[0], "/")
		return backend, nil
	}
	if len(addresses) == 0 {
		return backend, errors.New("backend has no base URL")
	}

	probeCtx, cancel := context.WithTimeout(ctx, upstreamProbeTimeout)
	defer cancel()

	results := make(chan probeResult, len(addresses))
	for _, address := range addresses {
		go func(address string) { results <- probeAddress(probeCtx, address) }(address)
	}

	var best probeResult
	for range addresses {
		select {
		case result := <-results:
			if result.baseURL != "" && (best.baseURL == "" || result.delay < best.delay) {
				best = result
			}
		case <-probeCtx.Done():
			if best.baseURL == "" {
				return backend, errors.New("no backend address is reachable")
			}
			backend.BaseURL = best.baseURL
			return backend, nil
		}
	}

	if best.baseURL == "" {
		return backend, errors.New("no backend address is reachable")
	}
	backend.BaseURL = best.baseURL
	return backend, nil
}

func upstreamAddresses(backend store.XtreamCode) []string {
	addresses := strings.Fields(backend.BaseURL)
	for i := range addresses {
		addresses[i] = strings.TrimRight(addresses[i], "/")
	}
	return addresses
}

func probeAddress(ctx context.Context, baseURL string) probeResult {
	baseURL = strings.TrimRight(baseURL, "/")
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return probeResult{}
	}

	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	started := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		return probeResult{}
	}
	conn.Close() // nolint: errcheck
	return probeResult{baseURL: baseURL, delay: time.Since(started)}
}

func (c *Config) monitorUpstreams(ctx context.Context) {
	c.checkUpstreams(ctx)
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkUpstreams(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (c *Config) checkUpstreams(ctx context.Context) {
	backends, err := c.Store.ListXtreamCodes()
	if err != nil {
		return
	}

	type target struct{ backend, address string }
	var targets []target
	for _, backend := range backends {
		for _, address := range upstreamAddresses(backend) {
			targets = append(targets, target{backend: backend.Name, address: address})
		}
	}

	type healthResult struct {
		target
		probeResult
	}
	results := make(chan healthResult, len(targets))
	probeCtx, cancel := context.WithTimeout(ctx, upstreamProbeTimeout)
	defer cancel()

	for _, item := range targets {
		go func(item target) {
			results <- healthResult{item, probeAddress(probeCtx, item.address)}
		}(item)
	}

	seen := make(map[string]bool, len(targets))
	for range targets {
		result := <-results
		seen[result.backend+"\x00"+result.address] = true
		c.recordHealth(result.backend, result.address, result.probeResult)
	}

	c.upstreamHealthLock.Lock()
	for key := range c.upstreamHealth {
		if !seen[key] {
			delete(c.upstreamHealth, key)
		}
	}
	c.upstreamHealthLock.Unlock()
}

func (c *Config) recordHealth(backend, address string, result probeResult) {
	key := backend + "\x00" + address
	c.upstreamHealthLock.Lock()
	defer c.upstreamHealthLock.Unlock()

	health := c.upstreamHealth[key]
	health.Backend = backend
	health.BaseURL = address
	health.Up = result.baseURL != ""
	health.LatencyMS = result.delay.Milliseconds()
	if health.Up && health.LatencyMS == 0 {
		health.LatencyMS = 1
	}
	health.CheckedAtUnix = time.Now().Unix()
	health.History = append(health.History, health.Up)
	if len(health.History) > recentHealthSamples {
		health.History = health.History[len(health.History)-recentHealthSamples:]
	}

	up := 0
	for _, sample := range health.History {
		if sample {
			up++
		}
	}
	health.UptimePercent = up * 100 / len(health.History)
	c.upstreamHealth[key] = health
}

// BackendHealth returns a stable snapshot for the admin UI.
func (c *Config) BackendHealth() []UpstreamHealth {
	c.upstreamHealthLock.RLock()
	rows := make([]UpstreamHealth, 0, len(c.upstreamHealth))
	for _, health := range c.upstreamHealth {
		health.History = append([]bool(nil), health.History...)
		rows = append(rows, health)
	}
	c.upstreamHealthLock.RUnlock()

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Backend == rows[j].Backend {
			if rows[i].Up != rows[j].Up {
				return rows[i].Up
			}
			if rows[i].Up && rows[i].LatencyMS != rows[j].LatencyMS {
				return rows[i].LatencyMS < rows[j].LatencyMS
			}
			return rows[i].BaseURL < rows[j].BaseURL
		}
		return rows[i].Backend < rows[j].Backend
	})
	return rows
}
