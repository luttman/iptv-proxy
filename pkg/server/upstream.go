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
	healthCheckInterval  = 5 * time.Minute
	recentHealthSamples  = 60
	uptimeRetention      = 7 * 24 * time.Hour

	// switchLatencyMarginMS is how much faster a candidate address
	// must be than the currently selected one before failover bothers
	// switching, to avoid flapping between addresses with near-
	// identical latency.
	switchLatencyMarginMS = 50
)

type probeResult struct {
	baseURL string
	delay   time.Duration
}

// UpstreamHealth is the current and recent status of one address. A
// ping only tests whether the host is reachable, not which account
// is behind it, so an address shared by several xtream-code backends
// gets one row listing all of them rather than one row each.
type UpstreamHealth struct {
	Backends         []string
	BaseURL          string
	Up               bool
	LatencyMS        int64
	CheckedAtUnix    int64
	History          []bool
	Uptime24hPercent int
	Uptime24hKnown   bool
	Uptime7dPercent  int
	Uptime7dKnown    bool
}

// selectUpstream chooses an address for backend, preferring the
// lowest-latency address recent monitoring found healthy over
// probing the network on every call. exclude, if non-empty, is
// skipped (used to fail over away from an address that just failed
// mid-request). Falls back to a one-off live probe only when there
// is no monitoring data yet to go on (e.g. right after startup).
func (c *Config) selectUpstream(ctx context.Context, backend store.ResolvedBackend, exclude string) (store.ResolvedBackend, error) {
	addresses := upstreamAddresses(backend)
	if exclude != "" {
		filtered := addresses[:0]
		for _, a := range addresses {
			if a != exclude {
				filtered = append(filtered, a)
			}
		}
		addresses = filtered
	}

	if len(addresses) == 0 {
		return backend, errors.New("backend has no other reachable base URL")
	}
	if len(addresses) == 1 {
		backend.BaseURL = strings.TrimRight(addresses[0], "/")
		return backend, nil
	}

	if best, ok := c.bestKnownAddress(backend.Name, addresses); ok {
		backend.BaseURL = best
		return backend, nil
	}

	return probeAllAndPickFastest(ctx, backend, addresses)
}

// bestKnownAddress picks the lowest-latency address among those
// recent monitoring found "up", falling back to the previously
// selected address when a faster candidate isn't meaningfully faster
// (avoids unnecessary switching). ok is false when there's no
// monitoring data at all for any candidate address yet.
func (c *Config) bestKnownAddress(backendName string, addresses []string) (string, bool) {
	type candidate struct {
		addr    string
		latency int64
	}

	c.upstreamHealthLock.RLock()
	var healthy []candidate
	knownAny := false
	for _, addr := range addresses {
		health, seen := c.upstreamHealth[addr]
		if !seen {
			continue
		}
		knownAny = true
		if health.Up {
			healthy = append(healthy, candidate{addr, health.LatencyMS})
		}
	}
	c.upstreamHealthLock.RUnlock()

	if !knownAny || len(healthy) == 0 {
		return "", false
	}

	sort.Slice(healthy, func(i, j int) bool { return healthy[i].latency < healthy[j].latency })
	best := healthy[0]

	c.currentAddressLock.Lock()
	defer c.currentAddressLock.Unlock()

	current := c.currentAddress[backendName]
	for _, cand := range healthy {
		if cand.addr != current {
			continue
		}
		if cand.latency <= best.latency+switchLatencyMarginMS {
			return current, true
		}
		break
	}

	if c.currentAddress == nil {
		c.currentAddress = map[string]string{}
	}
	c.currentAddress[backendName] = best.addr
	return best.addr, true
}

// probeAllAndPickFastest is the bootstrap fallback used only before
// any monitoring data exists for a backend's addresses.
func probeAllAndPickFastest(ctx context.Context, backend store.ResolvedBackend, addresses []string) (store.ResolvedBackend, error) {
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

func upstreamAddresses(backend store.ResolvedBackend) []string {
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

type backendRef struct {
	id   int64
	name string
}

func (c *Config) checkUpstreams(ctx context.Context) {
	backends, err := c.Store.ListXtreamCodes()
	if err != nil {
		return
	}

	addressBackends := map[string][]backendRef{}
	for _, backend := range backends {
		addresses, err := c.Store.ListAddresses(backend.ID)
		if err != nil {
			continue
		}
		for _, addr := range addresses {
			if !addr.Enabled {
				continue
			}
			addressBackends[addr.Address] = append(addressBackends[addr.Address], backendRef{backend.ID, backend.Name})
		}
	}

	// Probe each distinct address once: it only tests whether the host
	// is reachable, not which backend's account is behind it, so two
	// xtream codes pointing at the same address don't need separate
	// probes to get the same answer.
	type addressResult struct {
		address string
		probeResult
	}
	probeCtx, cancel := context.WithTimeout(ctx, upstreamProbeTimeout)
	defer cancel()

	results := make(chan addressResult, len(addressBackends))
	for address := range addressBackends {
		go func(address string) { results <- addressResult{address, probeAddress(probeCtx, address)} }(address)
	}

	now := time.Now()
	seen := make(map[string]bool, len(addressBackends))
	for range addressBackends {
		r := <-results
		seen[r.address] = true
		c.recordHealth(addressBackends[r.address], r.address, r.probeResult, now)
	}

	c.upstreamHealthLock.Lock()
	for address := range c.upstreamHealth {
		if !seen[address] {
			delete(c.upstreamHealth, address)
		}
	}
	c.upstreamHealthLock.Unlock()

	c.Store.PruneUpstreamChecks(now.Add(-uptimeRetention)) // nolint: errcheck
}

func (c *Config) recordHealth(backends []backendRef, address string, result probeResult, checkedAt time.Time) {
	up := result.baseURL != ""
	latencyMS := result.delay.Milliseconds()
	if up && latencyMS == 0 {
		latencyMS = 1
	}

	names := make([]string, len(backends))
	for i, b := range backends {
		names[i] = b.name
	}
	sort.Strings(names)

	c.upstreamHealthLock.Lock()
	health := c.upstreamHealth[address]
	health.Backends = names
	health.BaseURL = address
	health.Up = up
	health.LatencyMS = latencyMS
	health.CheckedAtUnix = checkedAt.Unix()
	c.upstreamHealth[address] = health
	c.upstreamHealthLock.Unlock()

	for _, b := range backends {
		c.Store.RecordUpstreamCheck(b.id, address, up, latencyMS, checkedAt) // nolint: errcheck
	}
}

// BackendHealth returns a stable snapshot for the admin UI, combining
// the latest in-memory check with persisted 24h/7d uptime and recent
// history so both survive a restart.
func (c *Config) BackendHealth() []UpstreamHealth {
	codes, err := c.Store.ListXtreamCodes()
	if err != nil {
		return nil
	}
	idByName := make(map[string]int64, len(codes))
	for _, xc := range codes {
		idByName[xc.Name] = xc.ID
	}

	c.upstreamHealthLock.RLock()
	rows := make([]UpstreamHealth, 0, len(c.upstreamHealth))
	for _, health := range c.upstreamHealth {
		rows = append(rows, health)
	}
	c.upstreamHealthLock.RUnlock()

	now := time.Now()
	for i := range rows {
		// History/uptime are persisted per backend ID (so they survive
		// a backend rename), but every backend sharing this address
		// has been recording identical rows since the dedup above, so
		// any one of them (the lowest ID, roughly the oldest / most
		// complete history) represents the address as a whole.
		var id int64
		found := false
		for _, name := range rows[i].Backends {
			backendID, ok := idByName[name]
			if !ok {
				continue
			}
			if !found || backendID < id {
				id, found = backendID, true
			}
		}
		if !found {
			continue
		}

		if history, err := c.Store.RecentUpstreamChecks(id, rows[i].BaseURL, recentHealthSamples); err == nil {
			rows[i].History = history
		}
		if pct, known, err := c.Store.UpstreamUptime(id, rows[i].BaseURL, now.Add(-24*time.Hour)); err == nil {
			rows[i].Uptime24hPercent, rows[i].Uptime24hKnown = pct, known
		}
		if pct, known, err := c.Store.UpstreamUptime(id, rows[i].BaseURL, now.Add(-uptimeRetention)); err == nil {
			rows[i].Uptime7dPercent, rows[i].Uptime7dKnown = pct, known
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].BaseURL < rows[j].BaseURL })
	return rows
}
