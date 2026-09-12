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
	"net"
	"testing"
	"time"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

func TestSelectUpstreamSkipsUnreachableAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close() // nolint: errcheck

	backend, err := selectUpstream(context.Background(), store.XtreamCode{
		BaseURL: "http://127.0.0.1:1\nhttp://" + listener.Addr().String(),
	})
	if err != nil {
		t.Fatalf("selectUpstream() error: %v", err)
	}
	want := "http://" + listener.Addr().String()
	if backend.BaseURL != want {
		t.Errorf("selectUpstream() = %q, want %q", backend.BaseURL, want)
	}
}

func TestSelectUpstreamKeepsSingleAddress(t *testing.T) {
	backend, err := selectUpstream(context.Background(), store.XtreamCode{BaseURL: "http://example.com/"})
	if err != nil {
		t.Fatalf("selectUpstream() error: %v", err)
	}
	if backend.BaseURL != "http://example.com" {
		t.Errorf("selectUpstream() = %q", backend.BaseURL)
	}
}

func TestRecordHealthKeepsRecentUptime(t *testing.T) {
	c := &Config{upstreamHealth: map[string]UpstreamHealth{}}
	for i := 0; i < recentHealthSamples+10; i++ {
		result := probeResult{delay: 12 * time.Millisecond}
		if i%2 == 0 {
			result.baseURL = "http://example.com"
		}
		c.recordHealth("provider", "http://example.com", result)
	}

	health := c.BackendHealth()
	if len(health) != 1 || len(health[0].History) != recentHealthSamples {
		t.Fatalf("BackendHealth() = %+v, want one row with %d samples", health, recentHealthSamples)
	}
	if health[0].UptimePercent != 50 {
		t.Errorf("UptimePercent = %d, want 50", health[0].UptimePercent)
	}
}
