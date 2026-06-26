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

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is returned by Authenticate when the
// username doesn't exist or the password doesn't match.
var ErrInvalidCredentials = errors.New("invalid username or password")

// CreateUser inserts a new proxy user assigned to the given xtream code.
func (s *Store) CreateUser(username, password string, xtreamCodeID int64) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, xtream_code_id) VALUES (?, ?, ?)`,
		username, string(hash), xtreamCodeID,
	)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}

	return s.GetUser(id)
}

// UpdateUser updates a user's xtream-code assignment and, if password
// is non-empty, its password too.
func (s *Store) UpdateUser(id int64, username, password string, xtreamCodeID int64) (User, error) {
	if password == "" {
		_, err := s.db.Exec(
			`UPDATE users SET username = ?, xtream_code_id = ? WHERE id = ?`,
			username, xtreamCodeID, id,
		)
		if err != nil {
			return User{}, fmt.Errorf("update user: %w", err)
		}

		return s.GetUser(id)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	_, err = s.db.Exec(
		`UPDATE users SET username = ?, password_hash = ?, xtream_code_id = ? WHERE id = ?`,
		username, string(hash), xtreamCodeID, id,
	)
	if err != nil {
		return User{}, fmt.Errorf("update user: %w", err)
	}

	return s.GetUser(id)
}

// DeleteUser removes a proxy user.
func (s *Store) DeleteUser(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}

// GetUser looks up a user by id.
func (s *Store) GetUser(id int64) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, xtream_code_id, created_at FROM users WHERE id = ?`,
		id,
	)

	return scanUser(row)
}

// ListUsers returns all users ordered by username.
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, xtream_code_id, created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

// Authenticate verifies a username/password pair and, on success,
// returns the user along with the xtream-code backend it's assigned
// to.
func (s *Store) Authenticate(username, password string) (User, XtreamCode, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, xtream_code_id, created_at FROM users WHERE username = ?`,
		username,
	)

	user, err := scanUser(row)
	if errors.Is(err, ErrNotFound) {
		return User{}, XtreamCode{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, XtreamCode{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return User{}, XtreamCode{}, ErrInvalidCredentials
	}

	xc, err := s.GetXtreamCode(user.XtreamCodeID)
	if err != nil {
		return User{}, XtreamCode{}, fmt.Errorf("resolve xtream code for user %q: %w", username, err)
	}

	return user, xc, nil
}

func scanUser(row rowScanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.XtreamCodeID, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}

	return u, nil
}
