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
	s, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { s.Close() }) // nolint: errcheck

	return s
}

// newXtreamCode is the one-shot test convenience the old single-
// address/single-credential CreateXtreamCode used to be: create the
// provider, add its addresses, and add one credential.
func newXtreamCode(t *testing.T, s *Store, name, baseURL, user, pass string) (XtreamCode, XtreamCredential) {
	t.Helper()

	xc, err := s.CreateXtreamCode(name)
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if err := s.AddAddresses(xc.ID, baseURL); err != nil {
		t.Fatalf("AddAddresses() error: %v", err)
	}
	cred, err := s.CreateCredential(xc.ID, user, pass)
	if err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}

	return xc, cred
}

func TestXtreamCodeCRUD(t *testing.T) {
	s := newTestStore(t)

	xc, _ := newXtreamCode(t, s, "provider-a", "http://a.example.com", "auser", "apass")
	if xc.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := s.GetXtreamCode(xc.ID)
	if err != nil {
		t.Fatalf("GetXtreamCode() error: %v", err)
	}
	if got.Name != "provider-a" {
		t.Errorf("GetXtreamCode() = %+v, want name to match", got)
	}

	addrs, err := s.EnabledAddressesString(xc.ID)
	if err != nil {
		t.Fatalf("EnabledAddressesString() error: %v", err)
	}
	if addrs != "http://a.example.com" {
		t.Errorf("EnabledAddressesString() = %q, want %q", addrs, "http://a.example.com")
	}

	updated, err := s.UpdateXtreamCodeName(xc.ID, "provider-a-renamed")
	if err != nil {
		t.Fatalf("UpdateXtreamCodeName() error: %v", err)
	}
	if updated.Name != "provider-a-renamed" {
		t.Errorf("UpdateXtreamCodeName() name = %q, want %q", updated.Name, "provider-a-renamed")
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

	xc, cred := newXtreamCode(t, s, "provider-a", "http://a.example.com", "auser", "apass")

	if _, err := s.CreateUser("alice", "hunter2", cred.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	if err := s.DeleteXtreamCode(xc.ID); !errors.Is(err, ErrXtreamCodeInUse) {
		t.Errorf("DeleteXtreamCode() err = %v, want ErrXtreamCodeInUse", err)
	}
}

func TestAddressToggleAndDelete(t *testing.T) {
	s := newTestStore(t)

	xc, err := s.CreateXtreamCode("provider-a")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if err := s.AddAddresses(xc.ID, "http://a.example.com\nhttp://b.example.com"); err != nil {
		t.Fatalf("AddAddresses() error: %v", err)
	}

	addresses, err := s.ListAddresses(xc.ID)
	if err != nil {
		t.Fatalf("ListAddresses() error: %v", err)
	}
	if len(addresses) != 2 {
		t.Fatalf("ListAddresses() len = %d, want 2", len(addresses))
	}
	for _, a := range addresses {
		if !a.Enabled {
			t.Errorf("address %+v should be enabled by default", a)
		}
	}

	if err := s.SetAddressEnabled(addresses[0].ID, false); err != nil {
		t.Fatalf("SetAddressEnabled() error: %v", err)
	}

	enabled, err := s.EnabledAddressesString(xc.ID)
	if err != nil {
		t.Fatalf("EnabledAddressesString() error: %v", err)
	}
	if enabled != addresses[1].Address {
		t.Errorf("EnabledAddressesString() = %q, want only %q", enabled, addresses[1].Address)
	}

	if err := s.DeleteAddress(addresses[1].ID); err != nil {
		t.Fatalf("DeleteAddress() error: %v", err)
	}
	remaining, err := s.ListAddresses(xc.ID)
	if err != nil {
		t.Fatalf("ListAddresses() error: %v", err)
	}
	if len(remaining) != 1 {
		t.Errorf("ListAddresses() after delete len = %d, want 1", len(remaining))
	}
}

func TestCredentialCRUDAndInUse(t *testing.T) {
	s := newTestStore(t)

	xc, err := s.CreateXtreamCode("provider-a")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}

	cred, err := s.CreateCredential(xc.ID, "user1", "pass1")
	if err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}

	got, err := s.GetCredential(cred.ID)
	if err != nil || got.XtreamUser != "user1" || got.XtreamPassword != "pass1" {
		t.Errorf("GetCredential() = %+v, err %v, want user1/pass1", got, err)
	}

	if _, err := s.CreateCredential(xc.ID, "user2", "pass2"); err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}
	list, err := s.ListCredentials(xc.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListCredentials() = %+v, err %v, want 2 credentials", list, err)
	}

	if _, err := s.CreateUser("alice", "hunter2", cred.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}
	if err := s.DeleteCredential(cred.ID); !errors.Is(err, ErrCredentialInUse) {
		t.Errorf("DeleteCredential() err = %v, want ErrCredentialInUse", err)
	}
}

func TestUserCRUDAndAuthenticate(t *testing.T) {
	s := newTestStore(t)

	xc, cred := newXtreamCode(t, s, "provider-a", "http://a.example.com", "auser", "apass")

	u, err := s.CreateUser("alice", "hunter2", cred.ID, 2)
	if err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}
	if u.PasswordHash == "hunter2" {
		t.Error("expected password to be hashed, not stored in plaintext")
	}
	if u.MaxConcurrentStreams != 2 {
		t.Errorf("MaxConcurrentStreams = %d, want 2", u.MaxConcurrentStreams)
	}

	gotUser, gotBackend, err := s.Authenticate("alice", "hunter2")
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}
	if gotUser.Username != "alice" {
		t.Errorf("Authenticate() user = %q, want %q", gotUser.Username, "alice")
	}
	if gotBackend.ID != xc.ID {
		t.Errorf("Authenticate() resolved xtream code ID = %d, want %d", gotBackend.ID, xc.ID)
	}
	if gotBackend.XtreamUser != "auser" || gotBackend.XtreamPassword != "apass" {
		t.Errorf("Authenticate() resolved credential = %+v, want auser/apass", gotBackend)
	}
	if gotBackend.BaseURL != "http://a.example.com" {
		t.Errorf("Authenticate() resolved BaseURL = %q, want %q", gotBackend.BaseURL, "http://a.example.com")
	}

	if _, _, err := s.Authenticate("alice", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Authenticate() with wrong password: err = %v, want ErrInvalidCredentials", err)
	}

	if _, _, err := s.Authenticate("nobody", "hunter2"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Authenticate() with unknown user: err = %v, want ErrInvalidCredentials", err)
	}

	updated, err := s.UpdateUser(u.ID, "alice2", "", cred.ID, 3)
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

func TestUsersCanShareOneCredentialOrHaveTheirOwn(t *testing.T) {
	s := newTestStore(t)

	xc, err := s.CreateXtreamCode("provider-a")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if err := s.AddAddresses(xc.ID, "http://a.example.com"); err != nil {
		t.Fatalf("AddAddresses() error: %v", err)
	}
	credA, err := s.CreateCredential(xc.ID, "userA", "passA")
	if err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}
	credB, err := s.CreateCredential(xc.ID, "userB", "passB")
	if err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}

	if _, err := s.CreateUser("alice", "hunter2", credA.ID, 0); err != nil {
		t.Fatalf("CreateUser(alice) error: %v", err)
	}
	if _, err := s.CreateUser("bob", "hunter3", credB.ID, 0); err != nil {
		t.Fatalf("CreateUser(bob) error: %v", err)
	}

	_, aliceBackend, err := s.Authenticate("alice", "hunter2")
	if err != nil {
		t.Fatalf("Authenticate(alice) error: %v", err)
	}
	_, bobBackend, err := s.Authenticate("bob", "hunter3")
	if err != nil {
		t.Fatalf("Authenticate(bob) error: %v", err)
	}

	if aliceBackend.XtreamUser != "userA" || bobBackend.XtreamUser != "userB" {
		t.Errorf("each user should resolve to its own credential: alice=%q bob=%q", aliceBackend.XtreamUser, bobBackend.XtreamUser)
	}
	if aliceBackend.Name != bobBackend.Name {
		t.Errorf("both users share the same provider: alice=%q bob=%q", aliceBackend.Name, bobBackend.Name)
	}
}
