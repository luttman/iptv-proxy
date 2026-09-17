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
	"io"
	"sync/atomic"
	"time"
)

// bandwidthSampleInterval is how often the running byte counter is
// snapshotted and persisted. Short enough for "current throughput" to
// feel live, long enough not to write a row per request.
const (
	bandwidthSampleInterval = 30 * time.Second
	bandwidthRetention      = 7 * 24 * time.Hour
)

// countingWriter adds every byte written through it to a shared
// counter, so proxied stream bodies (the vast majority of traffic)
// can be measured without touching each stream's own logic.
type countingWriter struct {
	w       io.Writer
	counter *atomic.Int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.counter.Add(int64(n))
	return n, err
}

func (c *Config) monitorBandwidth(ctx context.Context) {
	ticker := time.NewTicker(bandwidthSampleInterval)
	defer ticker.Stop()

	var lastTotal int64
	for {
		select {
		case <-ticker.C:
			lastTotal = c.sampleBandwidth(lastTotal)
		case <-ctx.Done():
			return
		}
	}
}

// sampleBandwidth records the bytes proxied since lastTotal and
// returns the new running total to pass as lastTotal next time.
func (c *Config) sampleBandwidth(lastTotal int64) int64 {
	total := c.bandwidthBytes.Load()
	delta := total - lastTotal

	now := time.Now()
	c.Store.RecordBandwidthSample(now, delta)                   // nolint: errcheck
	c.Store.PruneBandwidthSamples(now.Add(-bandwidthRetention)) // nolint: errcheck

	return total
}

// BandwidthStats is a snapshot of proxied traffic for the admin UI.
type BandwidthStats struct {
	CurrentBytesPerSec float64
	CurrentKnown       bool
	Last24hBytes       int64
	Last7dBytes        int64
	PeakBytesPerSec    float64
	PeakAtUnix         int64
	PeakKnown          bool
}

// BandwidthStats returns current/24h/7d/peak throughput figures
// computed from persisted samples, so they survive a restart.
func (c *Config) BandwidthStats() BandwidthStats {
	var stats BandwidthStats
	now := time.Now()

	if bytes, sampledAt, ok, err := c.Store.LatestBandwidthSample(); err == nil && ok {
		stats.CurrentBytesPerSec = float64(bytes) / bandwidthSampleInterval.Seconds()
		// A sample older than two intervals means nothing has been
		// recorded recently (e.g. the process just started); don't
		// present a stale number as "current".
		stats.CurrentKnown = now.Sub(sampledAt) < 2*bandwidthSampleInterval
	}
	if total, err := c.Store.BandwidthSince(now.Add(-24 * time.Hour)); err == nil {
		stats.Last24hBytes = total
	}
	if total, err := c.Store.BandwidthSince(now.Add(-bandwidthRetention)); err == nil {
		stats.Last7dBytes = total
	}
	if bytes, sampledAt, ok, err := c.Store.PeakBandwidthSample(now.Add(-bandwidthRetention)); err == nil && ok {
		stats.PeakBytesPerSec = float64(bytes) / bandwidthSampleInterval.Seconds()
		stats.PeakAtUnix = sampledAt.Unix()
		stats.PeakKnown = true
	}

	return stats
}
