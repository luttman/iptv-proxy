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
	"strings"
	"time"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

const upstreamProbeTimeout = 2 * time.Second

type probeResult struct {
	baseURL string
	delay   time.Duration
}

// selectUpstream chooses the reachable service with the quickest TCP
// connection. A TCP probe checks the port the IPTV service actually uses,
// unlike ICMP ping, which many hosts block.
func selectUpstream(ctx context.Context, backend store.XtreamCode) (store.XtreamCode, error) {
	addresses := strings.Fields(backend.BaseURL)
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
		go probeUpstream(probeCtx, address, results)
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

func probeUpstream(ctx context.Context, baseURL string, results chan<- probeResult) {
	baseURL = strings.TrimRight(baseURL, "/")
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		results <- probeResult{}
		return
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
		results <- probeResult{}
		return
	}
	conn.Close() // nolint: errcheck
	results <- probeResult{baseURL: baseURL, delay: time.Since(started)}
}
