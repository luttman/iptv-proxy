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
	"strings"
)

// ErrNotFound is returned when a lookup by id finds no row.
var ErrNotFound = errors.New("not found")

// ErrXtreamCodeInUse is returned when deleting an xtream code that
// still has a credential assigned to one or more users.
var ErrXtreamCodeInUse = errors.New("xtream code is still assigned to one or more users")

// CreateXtreamCode inserts a new named provider, with no addresses or
// credentials yet (add those with AddAddresses/CreateCredential).
func (s *Store) CreateXtreamCode(name string) (XtreamCode, error) {
	// base_url/xtream_user/xtream_password stay in the schema so
	// existing rows (and their NOT NULL constraints) don't need a risky
	// migration; new rows just leave them empty, since addresses and
	// credentials now live in their own tables.
	res, err := s.db.Exec(
		`INSERT INTO xtream_codes (name, base_url, xtream_user, xtream_password) VALUES (?, '', '', '')`,
		name,
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

// UpdateXtreamCodeName renames an existing provider.
func (s *Store) UpdateXtreamCodeName(id int64, name string) (XtreamCode, error) {
	if _, err := s.db.Exec(`UPDATE xtream_codes SET name = ? WHERE id = ?`, name, id); err != nil {
		return XtreamCode{}, fmt.Errorf("update xtream code: %w", err)
	}

	return s.GetXtreamCode(id)
}

// DeleteXtreamCode removes a provider along with its addresses and
// credentials. It fails if any of its credentials is still assigned
// to a user.
func (s *Store) DeleteXtreamCode(id int64) error {
	var inUse int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM users u JOIN xtream_credentials c ON c.id = u.credential_id WHERE c.xtream_code_id = ?`,
		id,
	).Scan(&inUse); err != nil {
		return fmt.Errorf("check xtream code usage: %w", err)
	}
	if inUse > 0 {
		return ErrXtreamCodeInUse
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("delete xtream code: %w", err)
	}
	defer tx.Rollback() // nolint: errcheck

	for _, stmt := range []string{
		`DELETE FROM xtream_addresses WHERE xtream_code_id = ?`,
		`DELETE FROM xtream_credentials WHERE xtream_code_id = ?`,
		`DELETE FROM xtream_codes WHERE id = ?`,
	} {
		if _, err := tx.Exec(stmt, id); err != nil {
			return fmt.Errorf("delete xtream code: %w", err)
		}
	}

	return tx.Commit()
}

// GetXtreamCode looks up a provider by id.
func (s *Store) GetXtreamCode(id int64) (XtreamCode, error) {
	row := s.db.QueryRow(`SELECT id, name, created_at FROM xtream_codes WHERE id = ?`, id)
	return scanXtreamCode(row)
}

// ListXtreamCodes returns all providers ordered by name.
func (s *Store) ListXtreamCodes() ([]XtreamCode, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM xtream_codes ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list xtream codes: %w", err)
	}
	defer rows.Close() // nolint: errcheck

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

// EncryptExistingCredentials re-encrypts every xtream credential with
// key inside a single transaction, skipping rows already encrypted.
// Callers should back up the database file before calling this (see
// cmd/encrypt.go).
func (s *Store) EncryptExistingCredentials(key []byte) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("encrypt credentials: %w", err)
	}
	defer tx.Rollback() // nolint: errcheck

	rows, err := tx.Query(`SELECT id, xtream_user, xtream_password FROM xtream_credentials`)
	if err != nil {
		return fmt.Errorf("encrypt credentials: %w", err)
	}

	type plainRow struct {
		id         int64
		user, pass string
	}
	var pending []plainRow
	for rows.Next() {
		var r plainRow
		if err := rows.Scan(&r.id, &r.user, &r.pass); err != nil {
			rows.Close() // nolint: errcheck
			return fmt.Errorf("encrypt credentials: %w", err)
		}
		if strings.HasPrefix(r.user, encPrefix) && strings.HasPrefix(r.pass, encPrefix) {
			continue
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close() // nolint: errcheck
		return fmt.Errorf("encrypt credentials: %w", err)
	}
	rows.Close() // nolint: errcheck

	for _, r := range pending {
		encUser, err := encryptValue(key, r.user)
		if err != nil {
			return fmt.Errorf("encrypt credentials: %w", err)
		}
		encPass, err := encryptValue(key, r.pass)
		if err != nil {
			return fmt.Errorf("encrypt credentials: %w", err)
		}
		if _, err := tx.Exec(`UPDATE xtream_credentials SET xtream_user = ?, xtream_password = ? WHERE id = ?`, encUser, encPass, r.id); err != nil {
			return fmt.Errorf("encrypt credentials: %w", err)
		}
	}

	// backfillAddressesAndCredentials copies xtream_codes' legacy
	// columns into xtream_credentials verbatim, so a plaintext copy can
	// still be sitting there even once xtream_credentials is encrypted.
	// Those columns are unused for anything else now, so clear them.
	if _, err := tx.Exec(`UPDATE xtream_codes SET xtream_user = '', xtream_password = '' WHERE xtream_user != '' OR xtream_password != ''`); err != nil {
		return fmt.Errorf("encrypt credentials: %w", err)
	}

	return tx.Commit()
}

func scanXtreamCode(row rowScanner) (XtreamCode, error) {
	var xc XtreamCode
	err := row.Scan(&xc.ID, &xc.Name, &xc.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return XtreamCode{}, ErrNotFound
	}
	if err != nil {
		return XtreamCode{}, fmt.Errorf("scan xtream code: %w", err)
	}

	return xc, nil
}
