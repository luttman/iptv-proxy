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

// ErrCredentialInUse is returned when deleting a credential still
// assigned to one or more users.
var ErrCredentialInUse = errors.New("credential is still assigned to one or more users")

// CreateCredential adds a new username/password pair under a
// provider. name is an optional human-friendly label.
func (s *Store) CreateCredential(xtreamCodeID int64, name, username, password string) (XtreamCredential, error) {
	encUser, err := encryptValue(s.key, username)
	if err != nil {
		return XtreamCredential{}, fmt.Errorf("create credential: %w", err)
	}
	encPass, err := encryptValue(s.key, password)
	if err != nil {
		return XtreamCredential{}, fmt.Errorf("create credential: %w", err)
	}

	res, err := s.db.Exec(
		`INSERT INTO xtream_credentials (xtream_code_id, name, xtream_user, xtream_password) VALUES (?, ?, ?, ?)`,
		xtreamCodeID, name, encUser, encPass,
	)
	if err != nil {
		return XtreamCredential{}, fmt.Errorf("create credential: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return XtreamCredential{}, fmt.Errorf("create credential: %w", err)
	}

	return s.GetCredential(id)
}

// UpdateCredential changes a credential's name/username/password. An
// empty password leaves the existing one unchanged (so editing a
// credential doesn't require re-typing a secret you're not changing).
func (s *Store) UpdateCredential(id int64, name, username, password string) (XtreamCredential, error) {
	if password == "" {
		existing, err := s.GetCredential(id)
		if err != nil {
			return XtreamCredential{}, fmt.Errorf("update credential: %w", err)
		}
		password = existing.XtreamPassword
	}

	encUser, err := encryptValue(s.key, username)
	if err != nil {
		return XtreamCredential{}, fmt.Errorf("update credential: %w", err)
	}
	encPass, err := encryptValue(s.key, password)
	if err != nil {
		return XtreamCredential{}, fmt.Errorf("update credential: %w", err)
	}

	if _, err := s.db.Exec(`UPDATE xtream_credentials SET name = ?, xtream_user = ?, xtream_password = ? WHERE id = ?`, name, encUser, encPass, id); err != nil {
		return XtreamCredential{}, fmt.Errorf("update credential: %w", err)
	}

	return s.GetCredential(id)
}

// DeleteCredential removes a credential. It fails if a user is still
// assigned to it.
func (s *Store) DeleteCredential(id int64) error {
	var inUse int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE credential_id = ?`, id).Scan(&inUse); err != nil {
		return fmt.Errorf("check credential usage: %w", err)
	}
	if inUse > 0 {
		return ErrCredentialInUse
	}

	if _, err := s.db.Exec(`DELETE FROM xtream_credentials WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}

	return nil
}

// GetCredential looks up a credential by id.
func (s *Store) GetCredential(id int64) (XtreamCredential, error) {
	row := s.db.QueryRow(`SELECT id, xtream_code_id, name, xtream_user, xtream_password, created_at FROM xtream_credentials WHERE id = ?`, id)
	return s.scanCredential(row)
}

// ListCredentials returns every credential under a provider, oldest
// first.
func (s *Store) ListCredentials(xtreamCodeID int64) ([]XtreamCredential, error) {
	rows, err := s.db.Query(
		`SELECT id, xtream_code_id, name, xtream_user, xtream_password, created_at FROM xtream_credentials WHERE xtream_code_id = ? ORDER BY id`,
		xtreamCodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close() // nolint: errcheck

	var credentials []XtreamCredential
	for rows.Next() {
		c, err := s.scanCredential(rows)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, c)
	}

	return credentials, rows.Err()
}

func (s *Store) scanCredential(row rowScanner) (XtreamCredential, error) {
	var c XtreamCredential
	err := row.Scan(&c.ID, &c.XtreamCodeID, &c.Name, &c.XtreamUser, &c.XtreamPassword, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return XtreamCredential{}, ErrNotFound
	}
	if err != nil {
		return XtreamCredential{}, fmt.Errorf("scan credential: %w", err)
	}

	if c.XtreamUser, err = decryptValue(s.key, c.XtreamUser); err != nil {
		return XtreamCredential{}, err
	}
	if c.XtreamPassword, err = decryptValue(s.key, c.XtreamPassword); err != nil {
		return XtreamCredential{}, err
	}

	return c, nil
}
