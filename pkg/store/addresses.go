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
	"strings"
)

// AddAddresses bulk-inserts one row per non-empty whitespace/newline
// separated address (the "paste several URLs at once" flow), each
// enabled by default.
func (s *Store) AddAddresses(xtreamCodeID int64, rawAddresses string) error {
	for _, addr := range strings.Fields(rawAddresses) {
		addr = strings.TrimRight(addr, "/")
		if addr == "" {
			continue
		}
		if _, err := s.db.Exec(`INSERT INTO xtream_addresses (xtream_code_id, address, enabled) VALUES (?, ?, 1)`, xtreamCodeID, addr); err != nil {
			return fmt.Errorf("add address: %w", err)
		}
	}

	return nil
}

// SetAddressEnabled toggles whether an address is considered for
// health-checking and failover, without losing its recorded history.
func (s *Store) SetAddressEnabled(id int64, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	if _, err := s.db.Exec(`UPDATE xtream_addresses SET enabled = ? WHERE id = ?`, enabledInt, id); err != nil {
		return fmt.Errorf("set address enabled: %w", err)
	}

	return nil
}

// DeleteAddress permanently removes one address.
func (s *Store) DeleteAddress(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM xtream_addresses WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete address: %w", err)
	}

	return nil
}

// ListAddresses returns every address under a provider, in the order
// they were added.
func (s *Store) ListAddresses(xtreamCodeID int64) ([]XtreamAddress, error) {
	rows, err := s.db.Query(`SELECT id, xtream_code_id, address, enabled FROM xtream_addresses WHERE xtream_code_id = ? ORDER BY id`, xtreamCodeID)
	if err != nil {
		return nil, fmt.Errorf("list addresses: %w", err)
	}
	defer rows.Close() // nolint: errcheck

	var addresses []XtreamAddress
	for rows.Next() {
		var a XtreamAddress
		var enabled int
		if err := rows.Scan(&a.ID, &a.XtreamCodeID, &a.Address, &enabled); err != nil {
			return nil, fmt.Errorf("list addresses: %w", err)
		}
		a.Enabled = enabled != 0
		addresses = append(addresses, a)
	}

	return addresses, rows.Err()
}

// EnabledAddressesString returns every enabled address under a
// provider, space-joined the way the rest of the proxy expects a
// multi-address backend's BaseURL to look.
func (s *Store) EnabledAddressesString(xtreamCodeID int64) (string, error) {
	rows, err := s.db.Query(`SELECT address FROM xtream_addresses WHERE xtream_code_id = ? AND enabled = 1 ORDER BY id`, xtreamCodeID)
	if err != nil {
		return "", fmt.Errorf("enabled addresses: %w", err)
	}
	defer rows.Close() // nolint: errcheck

	var addresses []string
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			return "", fmt.Errorf("enabled addresses: %w", err)
		}
		addresses = append(addresses, addr)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("enabled addresses: %w", err)
	}

	return strings.Join(addresses, " "), nil
}
