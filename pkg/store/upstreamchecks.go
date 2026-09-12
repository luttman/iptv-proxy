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
	"fmt"
	"time"
)

// RecordUpstreamCheck persists one health-check result for a
// backend/address pair, keyed by the backend's stable ID so history
// survives the backend being renamed.
func (s *Store) RecordUpstreamCheck(xtreamCodeID int64, address string, up bool, latencyMS int64, checkedAt time.Time) error {
	upInt := 0
	if up {
		upInt = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO upstream_checks (xtream_code_id, address, checked_at, up, latency_ms) VALUES (?, ?, ?, ?, ?)`,
		xtreamCodeID, address, checkedAt.UTC(), upInt, latencyMS,
	)
	if err != nil {
		return fmt.Errorf("record upstream check: %w", err)
	}

	return nil
}

// PruneUpstreamChecks deletes recorded checks older than before,
// keeping the table bounded to roughly the retention window.
func (s *Store) PruneUpstreamChecks(before time.Time) error {
	if _, err := s.db.Exec(`DELETE FROM upstream_checks WHERE checked_at < ?`, before.UTC()); err != nil {
		return fmt.Errorf("prune upstream checks: %w", err)
	}

	return nil
}

// UpstreamUptime reports the percentage of "up" checks recorded for
// a backend/address since the given time. ok is false when there are
// no recorded checks in the window at all, meaning coverage is
// unknown (e.g. the proxy itself was down) rather than 0%.
func (s *Store) UpstreamUptime(xtreamCodeID int64, address string, since time.Time) (percent int, ok bool, err error) {
	var total, up int
	row := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(up), 0) FROM upstream_checks WHERE xtream_code_id = ? AND address = ? AND checked_at >= ?`,
		xtreamCodeID, address, since.UTC(),
	)
	if err := row.Scan(&total, &up); err != nil {
		return 0, false, fmt.Errorf("upstream uptime: %w", err)
	}
	if total == 0 {
		return 0, false, nil
	}

	return up * 100 / total, true, nil
}

// RecentUpstreamChecks returns up to limit of the most recent checks
// for a backend/address, oldest first, for rendering history bars
// that survive a restart.
func (s *Store) RecentUpstreamChecks(xtreamCodeID int64, address string, limit int) ([]bool, error) {
	rows, err := s.db.Query(
		`SELECT up FROM upstream_checks WHERE xtream_code_id = ? AND address = ? ORDER BY checked_at DESC LIMIT ?`,
		xtreamCodeID, address, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent upstream checks: %w", err)
	}
	defer rows.Close() // nolint: errcheck

	var reversed []bool
	for rows.Next() {
		var up int
		if err := rows.Scan(&up); err != nil {
			return nil, fmt.Errorf("recent upstream checks: %w", err)
		}
		reversed = append(reversed, up != 0)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recent upstream checks: %w", err)
	}

	history := make([]bool, len(reversed))
	for i, v := range reversed {
		history[len(reversed)-1-i] = v
	}

	return history, nil
}
