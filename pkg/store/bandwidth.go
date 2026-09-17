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
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RecordBandwidthSample persists the number of bytes proxied during
// one sample interval.
func (s *Store) RecordBandwidthSample(sampledAt time.Time, bytes int64) error {
	if _, err := s.db.Exec(`INSERT INTO bandwidth_samples (sampled_at, bytes) VALUES (?, ?)`, sampledAt.UTC(), bytes); err != nil {
		return fmt.Errorf("record bandwidth sample: %w", err)
	}

	return nil
}

// PruneBandwidthSamples deletes samples older than before.
func (s *Store) PruneBandwidthSamples(before time.Time) error {
	if _, err := s.db.Exec(`DELETE FROM bandwidth_samples WHERE sampled_at < ?`, before.UTC()); err != nil {
		return fmt.Errorf("prune bandwidth samples: %w", err)
	}

	return nil
}

// BandwidthSince sums the bytes proxied since the given time.
func (s *Store) BandwidthSince(since time.Time) (int64, error) {
	var total int64
	row := s.db.QueryRow(`SELECT COALESCE(SUM(bytes), 0) FROM bandwidth_samples WHERE sampled_at >= ?`, since.UTC())
	if err := row.Scan(&total); err != nil {
		return 0, fmt.Errorf("bandwidth since: %w", err)
	}

	return total, nil
}

// PeakBandwidthSample returns the single busiest sample recorded
// since the given time. ok is false when there are no samples yet.
func (s *Store) PeakBandwidthSample(since time.Time) (bytes int64, sampledAt time.Time, ok bool, err error) {
	row := s.db.QueryRow(
		`SELECT bytes, sampled_at FROM bandwidth_samples WHERE sampled_at >= ? ORDER BY bytes DESC LIMIT 1`,
		since.UTC(),
	)
	if err := row.Scan(&bytes, &sampledAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, time.Time{}, false, nil
		}
		return 0, time.Time{}, false, fmt.Errorf("peak bandwidth sample: %w", err)
	}

	return bytes, sampledAt, true, nil
}

// LatestBandwidthSample returns the most recently recorded sample.
// ok is false when there are no samples yet.
func (s *Store) LatestBandwidthSample() (bytes int64, sampledAt time.Time, ok bool, err error) {
	row := s.db.QueryRow(`SELECT bytes, sampled_at FROM bandwidth_samples ORDER BY sampled_at DESC LIMIT 1`)
	if err := row.Scan(&bytes, &sampledAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, time.Time{}, false, nil
		}
		return 0, time.Time{}, false, fmt.Errorf("latest bandwidth sample: %w", err)
	}

	return bytes, sampledAt, true, nil
}
