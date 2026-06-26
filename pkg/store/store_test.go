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
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	return s
}

func TestXtreamCodeCRUD(t *testing.T) {
	s := newTestStore(t)

	xc, err := s.CreateXtreamCode("provider-a", "http://a.example.com", "auser", "apass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if xc.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := s.GetXtreamCode(xc.ID)
	if err != nil {
		t.Fatalf("GetXtreamCode() error: %v", err)
	}
	if got.Name != "provider-a" || got.BaseURL != "http://a.example.com" {
		t.Errorf("GetXtreamCode() = %+v, want name/baseURL to match", got)
	}

	updated, err := s.UpdateXtreamCode(xc.ID, "provider-a-renamed", "http://a2.example.com", "auser2", "apass2")
	if err != nil {
		t.Fatalf("UpdateXtreamCode() error: %v", err)
	}
	if updated.Name != "provider-a-renamed" {
		t.Errorf("UpdateXtreamCode() name = %q, want %q", updated.Name, "provider-a-renamed")
	}

	list, err := s.ListXtreamCodes()
	if err != nil {
		t.Fatalf("ListXtreamCodes() error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListXtreamCodes() len = %d, want 1", len(list))
	}

	if err := s.DeleteXtreamCode(xc.ID); err != nil {
		t.Fatalf("DeleteXtreamCode() error: %v", err)
	}

	if _, err := s.GetXtreamCode(xc.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetXtreamCode() after delete: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteXtreamCodeInUse(t *testing.T) {
	s := newTestStore(t)

	xc, err := s.CreateXtreamCode("provider-a", "http://a.example.com", "auser", "apass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}

	if _, err := s.CreateUser("alice", "hunter2", xc.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	if err := s.DeleteXtreamCode(xc.ID); !errors.Is(err, ErrXtreamCodeInUse) {
		t.Errorf("DeleteXtreamCode() err = %v, want ErrXtreamCodeInUse", err)
	}
}

func TestUserCRUDAndAuthenticate(t *testing.T) {
	s := newTestStore(t)

	xc, err := s.CreateXtreamCode("provider-a", "http://a.example.com", "auser", "apass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}

	u, err := s.CreateUser("alice", "hunter2", xc.ID, 2)
	if err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}
	if u.PasswordHash == "hunter2" {
		t.Error("expected password to be hashed, not stored in plaintext")
	}
	if u.MaxConcurrentStreams != 2 {
		t.Errorf("MaxConcurrentStreams = %d, want 2", u.MaxConcurrentStreams)
	}

	gotUser, gotXC, err := s.Authenticate("alice", "hunter2")
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}
	if gotUser.Username != "alice" {
		t.Errorf("Authenticate() user = %q, want %q", gotUser.Username, "alice")
	}
	if gotXC.ID != xc.ID {
		t.Errorf("Authenticate() resolved xtream code ID = %d, want %d", gotXC.ID, xc.ID)
	}

	if _, _, err := s.Authenticate("alice", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Authenticate() with wrong password: err = %v, want ErrInvalidCredentials", err)
	}

	if _, _, err := s.Authenticate("nobody", "hunter2"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Authenticate() with unknown user: err = %v, want ErrInvalidCredentials", err)
	}

	updated, err := s.UpdateUser(u.ID, "alice2", "", xc.ID, 3)
	if err != nil {
		t.Fatalf("UpdateUser() error: %v", err)
	}
	if updated.Username != "alice2" {
		t.Errorf("UpdateUser() username = %q, want %q", updated.Username, "alice2")
	}
	if updated.PasswordHash != u.PasswordHash {
		t.Error("UpdateUser() with empty password should not change password hash")
	}
	if updated.MaxConcurrentStreams != 3 {
		t.Errorf("UpdateUser() MaxConcurrentStreams = %d, want 3", updated.MaxConcurrentStreams)
	}

	list, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers() error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListUsers() len = %d, want 1", len(list))
	}

	if err := s.DeleteUser(u.ID); err != nil {
		t.Fatalf("DeleteUser() error: %v", err)
	}

	if _, err := s.GetUser(u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetUser() after delete: err = %v, want ErrNotFound", err)
	}
}
