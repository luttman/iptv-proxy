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

package store

import (
	"testing"
	"time"
)

func TestBandwidthSamplesSumAndPeak(t *testing.T) {
	s := newTestStore(t)

	now := time.Now()
	samples := []struct {
		at    time.Time
		bytes int64
	}{
		{now.Add(-6 * 24 * time.Hour), 100},
		{now.Add(-2 * time.Hour), 500},
		{now.Add(-time.Minute), 200},
	}
	for _, sample := range samples {
		if err := s.RecordBandwidthSample(sample.at, sample.bytes); err != nil {
			t.Fatalf("RecordBandwidthSample() error: %v", err)
		}
	}

	total24h, err := s.BandwidthSince(now.Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("BandwidthSince() error: %v", err)
	}
	if total24h != 700 {
		t.Errorf("BandwidthSince(24h) = %d, want 700", total24h)
	}

	total7d, err := s.BandwidthSince(now.Add(-7 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("BandwidthSince() error: %v", err)
	}
	if total7d != 800 {
		t.Errorf("BandwidthSince(7d) = %d, want 800", total7d)
	}

	peakBytes, _, ok, err := s.PeakBandwidthSample(now.Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("PeakBandwidthSample() error: %v", err)
	}
	if !ok || peakBytes != 500 {
		t.Errorf("PeakBandwidthSample(24h) = %d, ok=%v, want 500, true", peakBytes, ok)
	}

	latestBytes, _, ok, err := s.LatestBandwidthSample()
	if err != nil {
		t.Fatalf("LatestBandwidthSample() error: %v", err)
	}
	if !ok || latestBytes != 200 {
		t.Errorf("LatestBandwidthSample() = %d, ok=%v, want 200, true", latestBytes, ok)
	}
}

func TestBandwidthSamplesUnknownWithoutData(t *testing.T) {
	s := newTestStore(t)

	total, err := s.BandwidthSince(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("BandwidthSince() error: %v", err)
	}
	if total != 0 {
		t.Errorf("BandwidthSince() with no samples = %d, want 0", total)
	}

	if _, _, ok, err := s.PeakBandwidthSample(time.Now().Add(-time.Hour)); err != nil || ok {
		t.Errorf("PeakBandwidthSample() with no samples: ok=%v err=%v, want false, nil", ok, err)
	}
	if _, _, ok, err := s.LatestBandwidthSample(); err != nil || ok {
		t.Errorf("LatestBandwidthSample() with no samples: ok=%v err=%v, want false, nil", ok, err)
	}
}

func TestPruneBandwidthSamplesRemovesOldOnes(t *testing.T) {
	s := newTestStore(t)

	now := time.Now()
	if err := s.RecordBandwidthSample(now.Add(-10*24*time.Hour), 100); err != nil {
		t.Fatalf("RecordBandwidthSample() error: %v", err)
	}
	if err := s.RecordBandwidthSample(now, 50); err != nil {
		t.Fatalf("RecordBandwidthSample() error: %v", err)
	}

	if err := s.PruneBandwidthSamples(now.Add(-7 * 24 * time.Hour)); err != nil {
		t.Fatalf("PruneBandwidthSamples() error: %v", err)
	}

	total, err := s.BandwidthSince(now.Add(-30 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("BandwidthSince() error: %v", err)
	}
	if total != 50 {
		t.Errorf("BandwidthSince() after prune = %d, want 50 (old sample should be gone)", total)
	}
}
