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
	"sync/atomic"
	"testing"
	"time"
)

func TestCountingWriterAddsBytesWritten(t *testing.T) {
	var counter atomic.Int64
	var buf bytes.Buffer
	cw := &countingWriter{w: &buf, counter: &counter}

	if _, err := cw.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := cw.Write([]byte("!!")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if got := counter.Load(); got != 7 {
		t.Errorf("counter = %d, want 7", got)
	}
	if buf.String() != "hello!!" {
		t.Errorf("underlying writer got %q, want %q", buf.String(), "hello!!")
	}
}

func TestBandwidthStatsAggregatesSamples(t *testing.T) {
	c := newTestConfig(t)

	now := time.Now()
	if err := c.Store.RecordBandwidthSample(now, 3_000_000); err != nil {
		t.Fatalf("RecordBandwidthSample() error: %v", err)
	}
	if err := c.Store.RecordBandwidthSample(now.Add(-time.Hour), 1_000_000); err != nil {
		t.Fatalf("RecordBandwidthSample() error: %v", err)
	}

	stats := c.BandwidthStats()

	if !stats.CurrentKnown {
		t.Error("CurrentKnown = false, want true for a just-recorded sample")
	}
	wantBps := float64(3_000_000) / bandwidthSampleInterval.Seconds()
	if stats.CurrentBytesPerSec != wantBps {
		t.Errorf("CurrentBytesPerSec = %v, want %v", stats.CurrentBytesPerSec, wantBps)
	}
	if stats.Last24hBytes != 4_000_000 {
		t.Errorf("Last24hBytes = %d, want 4000000", stats.Last24hBytes)
	}
	if !stats.PeakKnown || stats.PeakBytesPerSec != wantBps {
		t.Errorf("Peak = %v known=%v, want %v true", stats.PeakBytesPerSec, stats.PeakKnown, wantBps)
	}
}

func TestBandwidthStatsCurrentUnknownWhenStale(t *testing.T) {
	c := newTestConfig(t)

	if err := c.Store.RecordBandwidthSample(time.Now().Add(-time.Hour), 1_000_000); err != nil {
		t.Fatalf("RecordBandwidthSample() error: %v", err)
	}

	stats := c.BandwidthStats()
	if stats.CurrentKnown {
		t.Error("CurrentKnown = true, want false for a stale (1h old) sample")
	}
}

func TestSampleBandwidthRecordsDeltaSinceLastSample(t *testing.T) {
	c := newTestConfig(t)
	c.bandwidthBytes.Store(1000)

	lastTotal := c.sampleBandwidth(0)
	if lastTotal != 1000 {
		t.Errorf("sampleBandwidth() returned %d, want 1000", lastTotal)
	}

	c.bandwidthBytes.Add(500)
	lastTotal = c.sampleBandwidth(lastTotal)
	if lastTotal != 1500 {
		t.Errorf("sampleBandwidth() returned %d, want 1500", lastTotal)
	}

	bytes, _, ok, err := c.Store.LatestBandwidthSample()
	if err != nil || !ok {
		t.Fatalf("LatestBandwidthSample() error=%v ok=%v", err, ok)
	}
	if bytes != 500 {
		t.Errorf("second sample recorded %d, want the 500-byte delta, not the running total", bytes)
	}
}
