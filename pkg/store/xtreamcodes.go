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
)

// ErrNotFound is returned when a lookup by id finds no row.
var ErrNotFound = errors.New("not found")

// ErrXtreamCodeInUse is returned when deleting an xtream code that
// still has users assigned to it.
var ErrXtreamCodeInUse = errors.New("xtream code is still assigned to one or more users")

// CreateXtreamCode inserts a new xtream-code backend.
func (s *Store) CreateXtreamCode(name, baseURL, xtreamUser, xtreamPassword string) (XtreamCode, error) {
	res, err := s.db.Exec(
		`INSERT INTO xtream_codes (name, base_url, xtream_user, xtream_password) VALUES (?, ?, ?, ?)`,
		name, baseURL, xtreamUser, xtreamPassword,
	)
	if err != nil {
		return XtreamCode{}, fmt.Errorf("create xtream code: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return XtreamCode{}, fmt.Errorf("create xtream code: %w", err)
	}

	return s.GetXtreamCode(id)
}

// UpdateXtreamCode updates an existing xtream-code backend.
func (s *Store) UpdateXtreamCode(id int64, name, baseURL, xtreamUser, xtreamPassword string) (XtreamCode, error) {
	_, err := s.db.Exec(
		`UPDATE xtream_codes SET name = ?, base_url = ?, xtream_user = ?, xtream_password = ? WHERE id = ?`,
		name, baseURL, xtreamUser, xtreamPassword, id,
	)
	if err != nil {
		return XtreamCode{}, fmt.Errorf("update xtream code: %w", err)
	}

	return s.GetXtreamCode(id)
}

// DeleteXtreamCode removes an xtream-code backend. It fails if any
// user is still assigned to it.
func (s *Store) DeleteXtreamCode(id int64) error {
	var inUse int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE xtream_code_id = ?`, id).Scan(&inUse); err != nil {
		return fmt.Errorf("check xtream code usage: %w", err)
	}
	if inUse > 0 {
		return ErrXtreamCodeInUse
	}

	if _, err := s.db.Exec(`DELETE FROM xtream_codes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete xtream code: %w", err)
	}

	return nil
}

// GetXtreamCode looks up an xtream-code backend by id.
func (s *Store) GetXtreamCode(id int64) (XtreamCode, error) {
	row := s.db.QueryRow(
		`SELECT id, name, base_url, xtream_user, xtream_password, created_at FROM xtream_codes WHERE id = ?`,
		id,
	)

	return scanXtreamCode(row)
}

// ListXtreamCodes returns all xtream-code backends ordered by name.
func (s *Store) ListXtreamCodes() ([]XtreamCode, error) {
	rows, err := s.db.Query(`SELECT id, name, base_url, xtream_user, xtream_password, created_at FROM xtream_codes ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list xtream codes: %w", err)
	}
	defer rows.Close()

	var codes []XtreamCode
	for rows.Next() {
		xc, err := scanXtreamCode(rows)
		if err != nil {
			return nil, err
		}
		codes = append(codes, xc)
	}

	return codes, rows.Err()
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanXtreamCode(row rowScanner) (XtreamCode, error) {
	var xc XtreamCode
	err := row.Scan(&xc.ID, &xc.Name, &xc.BaseURL, &xc.XtreamUser, &xc.XtreamPassword, &xc.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return XtreamCode{}, ErrNotFound
	}
	if err != nil {
		return XtreamCode{}, fmt.Errorf("scan xtream code: %w", err)
	}

	return xc, nil
}
