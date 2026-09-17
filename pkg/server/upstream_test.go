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
	"sync/atomic"
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

	c := newTestConfig(t)
	backend, err := c.selectUpstream(context.Background(), store.XtreamCode{
		BaseURL: "http://127.0.0.1:1\nhttp://" + listener.Addr().String(),
	}, "")
	if err != nil {
		t.Fatalf("selectUpstream() error: %v", err)
	}
	want := "http://" + listener.Addr().String()
	if backend.BaseURL != want {
		t.Errorf("selectUpstream() = %q, want %q", backend.BaseURL, want)
	}
}

func TestSelectUpstreamKeepsSingleAddress(t *testing.T) {
	c := newTestConfig(t)
	backend, err := c.selectUpstream(context.Background(), store.XtreamCode{BaseURL: "http://example.com/"}, "")
	if err != nil {
		t.Fatalf("selectUpstream() error: %v", err)
	}
	if backend.BaseURL != "http://example.com" {
		t.Errorf("selectUpstream() = %q", backend.BaseURL)
	}
}

func TestSelectUpstreamUsesMonitoringResultsWithoutProbing(t *testing.T) {
	c := newTestConfig(t)
	backend := store.XtreamCode{Name: "acme", BaseURL: "http://slow.invalid http://fast.invalid"}

	// Neither address is actually reachable; if selectUpstream fell
	// back to probing it would fail. Seed monitoring data instead.
	c.upstreamHealth["http://slow.invalid"] = UpstreamHealth{Up: true, LatencyMS: 200}
	c.upstreamHealth["http://fast.invalid"] = UpstreamHealth{Up: true, LatencyMS: 10}

	got, err := c.selectUpstream(context.Background(), backend, "")
	if err != nil {
		t.Fatalf("selectUpstream() error: %v", err)
	}
	if got.BaseURL != "http://fast.invalid" {
		t.Errorf("selectUpstream() = %q, want the lowest-latency known-healthy address", got.BaseURL)
	}
}

func TestSelectUpstreamKeepsCurrentWhenDifferenceIsSmall(t *testing.T) {
	c := newTestConfig(t)
	backend := store.XtreamCode{Name: "acme", BaseURL: "http://a.invalid http://b.invalid"}

	c.upstreamHealth["http://a.invalid"] = UpstreamHealth{Up: true, LatencyMS: 40}
	c.upstreamHealth["http://b.invalid"] = UpstreamHealth{Up: true, LatencyMS: 20}
	c.currentAddress = map[string]string{"acme": "http://a.invalid"}

	got, err := c.selectUpstream(context.Background(), backend, "")
	if err != nil {
		t.Fatalf("selectUpstream() error: %v", err)
	}
	if got.BaseURL != "http://a.invalid" {
		t.Errorf("selectUpstream() = %q, want to keep current address within the switch margin", got.BaseURL)
	}
}

func TestSelectUpstreamSwitchesWhenDifferenceIsLarge(t *testing.T) {
	c := newTestConfig(t)
	backend := store.XtreamCode{Name: "acme", BaseURL: "http://a.invalid http://b.invalid"}

	c.upstreamHealth["http://a.invalid"] = UpstreamHealth{Up: true, LatencyMS: 500}
	c.upstreamHealth["http://b.invalid"] = UpstreamHealth{Up: true, LatencyMS: 20}
	c.currentAddress = map[string]string{"acme": "http://a.invalid"}

	got, err := c.selectUpstream(context.Background(), backend, "")
	if err != nil {
		t.Fatalf("selectUpstream() error: %v", err)
	}
	if got.BaseURL != "http://b.invalid" {
		t.Errorf("selectUpstream() = %q, want to switch to the much faster address", got.BaseURL)
	}
}

func TestSelectUpstreamExcludesFailedAddress(t *testing.T) {
	c := newTestConfig(t)
	backend := store.XtreamCode{Name: "acme", BaseURL: "http://a.invalid http://b.invalid"}

	c.upstreamHealth["http://a.invalid"] = UpstreamHealth{Up: true, LatencyMS: 5}
	c.upstreamHealth["http://b.invalid"] = UpstreamHealth{Up: true, LatencyMS: 100}

	got, err := c.selectUpstream(context.Background(), backend, "http://a.invalid")
	if err != nil {
		t.Fatalf("selectUpstream() error: %v", err)
	}
	if got.BaseURL != "http://b.invalid" {
		t.Errorf("selectUpstream() = %q, want the other backup address", got.BaseURL)
	}
}

func TestSelectUpstreamStaleDataStillUsed(t *testing.T) {
	c := newTestConfig(t)
	backend := store.XtreamCode{Name: "acme", BaseURL: "http://a.invalid http://b.invalid"}

	// Data from well outside the health-check interval is still the
	// best information available and should still be honored, rather
	// than triggering a live probe on every request.
	c.upstreamHealth["http://a.invalid"] = UpstreamHealth{Up: true, LatencyMS: 20, CheckedAtUnix: time.Now().Add(-2 * time.Hour).Unix()}
	c.upstreamHealth["http://b.invalid"] = UpstreamHealth{Up: false, LatencyMS: 0, CheckedAtUnix: time.Now().Add(-2 * time.Hour).Unix()}

	got, err := c.selectUpstream(context.Background(), backend, "")
	if err != nil {
		t.Fatalf("selectUpstream() error: %v", err)
	}
	if got.BaseURL != "http://a.invalid" {
		t.Errorf("selectUpstream() = %q, want the only known-healthy (if stale) address", got.BaseURL)
	}
}

func TestSelectUpstreamCancellation(t *testing.T) {
	c := newTestConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// No monitoring data at all forces the bootstrap probe fallback,
	// which must respect a canceled context instead of hanging.
	_, err := c.selectUpstream(ctx, store.XtreamCode{BaseURL: "http://a.invalid http://b.invalid"}, "")
	if err == nil {
		t.Error("selectUpstream() with canceled context: want error, got nil")
	}
}

func TestCheckUpstreamsDoesNotDuplicateProbesForSharedAddress(t *testing.T) {
	var connections int32
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close() // nolint: errcheck

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&connections, 1)
			conn.Close() // nolint: errcheck
		}
	}()

	c := newTestConfig(t)
	addr := "http://" + listener.Addr().String()

	// Two xtream codes sharing the same address: a ping only tests
	// whether the host is reachable, not which account is behind it,
	// so it should only be probed once per cycle.
	if _, err := c.Store.CreateXtreamCode("provider-a", addr, "u1", "p1"); err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if _, err := c.Store.CreateXtreamCode("provider-b", addr, "u2", "p2"); err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}

	c.checkUpstreams(context.Background())

	if got := atomic.LoadInt32(&connections); got != 1 {
		t.Errorf("connections to shared address = %d, want 1", got)
	}

	health := c.BackendHealth()
	if len(health) != 1 {
		t.Fatalf("BackendHealth() = %+v, want a single row for the shared address", health)
	}
	if !health[0].Up {
		t.Errorf("BackendHealth() row %+v, want Up", health[0])
	}
	if len(health[0].Backends) != 2 {
		t.Errorf("BackendHealth() row Backends = %v, want both backend names listed", health[0].Backends)
	}
}

func TestRecordHealthPersistsToStore(t *testing.T) {
	c := newTestConfig(t)
	xc, err := c.Store.CreateXtreamCode("provider-a", "http://example.com", "u", "p")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}

	now := time.Now()
	refs := []backendRef{{id: xc.ID, name: "provider-a"}}
	c.recordHealth(refs, "http://example.com", probeResult{baseURL: "http://example.com", delay: 12 * time.Millisecond}, now)
	c.recordHealth(refs, "http://example.com", probeResult{}, now.Add(time.Minute))

	pct, ok, err := c.Store.UpstreamUptime(xc.ID, "http://example.com", now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("UpstreamUptime() error: %v", err)
	}
	if !ok || pct != 50 {
		t.Errorf("UpstreamUptime() = %d, %v, want 50, true", pct, ok)
	}

	health := c.BackendHealth()
	if len(health) != 1 {
		t.Fatalf("BackendHealth() = %+v, want one row", health)
	}
	if len(health[0].History) != 2 {
		t.Errorf("History = %+v, want 2 entries", health[0].History)
	}
	if !health[0].Uptime24hKnown || health[0].Uptime24hPercent != 50 {
		t.Errorf("Uptime24h = %d known=%v, want 50 true", health[0].Uptime24hPercent, health[0].Uptime24hKnown)
	}
}

func TestUpstreamUptimeUnknownWithoutChecks(t *testing.T) {
	c := newTestConfig(t)
	xc, err := c.Store.CreateXtreamCode("provider-a", "http://example.com", "u", "p")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}

	_, ok, err := c.Store.UpstreamUptime(xc.ID, "http://example.com", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("UpstreamUptime() error: %v", err)
	}
	if ok {
		t.Error("UpstreamUptime() with no recorded checks should be unknown, not 0%")
	}
}

func TestPruneUpstreamChecksRemovesOldSamples(t *testing.T) {
	c := newTestConfig(t)
	xc, err := c.Store.CreateXtreamCode("provider-a", "http://example.com", "u", "p")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}

	old := time.Now().Add(-10 * 24 * time.Hour)
	if err := c.Store.RecordUpstreamCheck(xc.ID, "http://example.com", true, 5, old); err != nil {
		t.Fatalf("RecordUpstreamCheck() error: %v", err)
	}
	if err := c.Store.RecordUpstreamCheck(xc.ID, "http://example.com", true, 5, time.Now()); err != nil {
		t.Fatalf("RecordUpstreamCheck() error: %v", err)
	}

	if err := c.Store.PruneUpstreamChecks(time.Now().Add(-uptimeRetention)); err != nil {
		t.Fatalf("PruneUpstreamChecks() error: %v", err)
	}

	history, err := c.Store.RecentUpstreamChecks(xc.ID, "http://example.com", 10)
	if err != nil {
		t.Fatalf("RecentUpstreamChecks() error: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("RecentUpstreamChecks() = %+v, want 1 entry after pruning", history)
	}
}
