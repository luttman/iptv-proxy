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

package admin

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// newLoggedInClient returns an http.Client with a valid admin session
// and the CSRF token needed to POST with it.
func newLoggedInClient(t *testing.T, baseURL string) (*http.Client, string) {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	client := &http.Client{Jar: jar}
	login(t, client, baseURL)

	token := cookieValue(t, jar, baseURL+"/admin", csrfCookieName)
	return client, token
}

func TestXtreamCodeAddresses_BulkAddThenToggle(t *testing.T) {
	srv, ts := newTestAdminServer(t)
	client, token := newLoggedInClient(t, ts.URL)

	resp, err := client.PostForm(ts.URL+"/admin/xtream-codes/new", url.Values{
		"name":            {"provider-a"},
		"base_url":        {"http://a.example.com\nhttp://b.example.com"},
		"xtream_user":     {"xu"},
		"xtream_password": {"xp"},
		"csrf_token":      {token},
	})
	if err != nil {
		t.Fatalf("POST new error: %v", err)
	}
	resp.Body.Close() // nolint: errcheck

	codes, err := srv.Store.ListXtreamCodes()
	if err != nil || len(codes) != 1 {
		t.Fatalf("ListXtreamCodes() = %+v, err %v, want one code", codes, err)
	}
	xcID := codes[0].ID

	// Bulk-add two more addresses on top of the two from creation.
	resp, err = client.PostForm(fmt.Sprintf("%s/admin/xtream-codes/%d/addresses", ts.URL, xcID), url.Values{
		"addresses":  {"http://c.example.com\nhttp://d.example.com"},
		"csrf_token": {token},
	})
	if err != nil {
		t.Fatalf("POST addresses error: %v", err)
	}
	resp.Body.Close() // nolint: errcheck

	addresses, err := srv.Store.ListAddresses(xcID)
	if err != nil || len(addresses) != 4 {
		t.Fatalf("ListAddresses() = %+v, err %v, want 4 addresses", addresses, err)
	}
	for _, a := range addresses {
		if !a.Enabled {
			t.Errorf("address %+v should start enabled", a)
		}
	}

	// Toggle the first address off.
	resp, err = client.PostForm(fmt.Sprintf("%s/admin/xtream-codes/%d/addresses/%d/toggle", ts.URL, xcID, addresses[0].ID), url.Values{
		"enabled":    {"0"},
		"csrf_token": {token},
	})
	if err != nil {
		t.Fatalf("POST toggle error: %v", err)
	}
	resp.Body.Close() // nolint: errcheck

	enabled, err := srv.Store.EnabledAddressesString(xcID)
	if err != nil {
		t.Fatalf("EnabledAddressesString() error: %v", err)
	}
	if enabled == "" {
		t.Fatal("EnabledAddressesString() is empty, want the three still-enabled addresses")
	}
	enabledFields := strings.Fields(enabled)
	for _, want := range []string{addresses[1].Address, addresses[2].Address, addresses[3].Address} {
		found := false
		for _, f := range enabledFields {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("EnabledAddressesString() = %q, missing %q", enabled, want)
		}
	}
	for _, f := range enabledFields {
		if f == addresses[0].Address {
			t.Errorf("EnabledAddressesString() = %q, should not include the disabled address %q", enabled, addresses[0].Address)
		}
	}

	// Delete one of the remaining addresses outright.
	resp, err = client.PostForm(fmt.Sprintf("%s/admin/xtream-codes/%d/addresses/%d/delete", ts.URL, xcID, addresses[1].ID), url.Values{
		"csrf_token": {token},
	})
	if err != nil {
		t.Fatalf("POST address delete error: %v", err)
	}
	resp.Body.Close() // nolint: errcheck

	remaining, err := srv.Store.ListAddresses(xcID)
	if err != nil || len(remaining) != 3 {
		t.Fatalf("ListAddresses() after delete = %+v, err %v, want 3", remaining, err)
	}
}

func TestCredentials_AddAndDeleteProtected(t *testing.T) {
	srv, ts := newTestAdminServer(t)
	client, token := newLoggedInClient(t, ts.URL)

	resp, err := client.PostForm(ts.URL+"/admin/xtream-codes/new", url.Values{
		"name":            {"provider-a"},
		"base_url":        {"http://a.example.com"},
		"xtream_user":     {"first"},
		"xtream_password": {"firstpass"},
		"csrf_token":      {token},
	})
	if err != nil {
		t.Fatalf("POST new error: %v", err)
	}
	resp.Body.Close() // nolint: errcheck

	codes, err := srv.Store.ListXtreamCodes()
	if err != nil || len(codes) != 1 {
		t.Fatalf("ListXtreamCodes() = %+v, err %v, want one code", codes, err)
	}
	xcID := codes[0].ID

	// Add a second credential under the same provider.
	resp, err = client.PostForm(fmt.Sprintf("%s/admin/xtream-codes/%d/credentials", ts.URL, xcID), url.Values{
		"xtream_user":     {"second"},
		"xtream_password": {"secondpass"},
		"csrf_token":      {token},
	})
	if err != nil {
		t.Fatalf("POST credentials error: %v", err)
	}
	resp.Body.Close() // nolint: errcheck

	credentials, err := srv.Store.ListCredentials(xcID)
	if err != nil || len(credentials) != 2 {
		t.Fatalf("ListCredentials() = %+v, err %v, want 2 credentials", credentials, err)
	}

	// Assign a user to the second credential, then try to delete it.
	if _, err := srv.Store.CreateUser("alice", "hunter2", credentials[1].ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	resp, err = client.PostForm(fmt.Sprintf("%s/admin/xtream-codes/%d/credentials/%d/delete", ts.URL, xcID, credentials[1].ID), url.Values{
		"csrf_token": {token},
	})
	if err != nil {
		t.Fatalf("POST credential delete error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want %d (credential in use)", resp.StatusCode, http.StatusConflict)
	}

	// The unassigned credential can still be deleted.
	resp2, err := client.PostForm(fmt.Sprintf("%s/admin/xtream-codes/%d/credentials/%d/delete", ts.URL, xcID, credentials[0].ID), url.Values{
		"csrf_token": {token},
	})
	if err != nil {
		t.Fatalf("POST credential delete error: %v", err)
	}
	resp2.Body.Close() // nolint: errcheck

	remaining, err := srv.Store.ListCredentials(xcID)
	if err != nil || len(remaining) != 1 {
		t.Fatalf("ListCredentials() after delete = %+v, err %v, want 1", remaining, err)
	}
}

func TestUserForm_PicksSpecificCredentialUnderProvider(t *testing.T) {
	srv, ts := newTestAdminServer(t)
	client, token := newLoggedInClient(t, ts.URL)

	xc, err := srv.Store.CreateXtreamCode("provider-a")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if err := srv.Store.AddAddresses(xc.ID, "http://a.example.com"); err != nil {
		t.Fatalf("AddAddresses() error: %v", err)
	}
	credA, err := srv.Store.CreateCredential(xc.ID, "userA", "passA")
	if err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}
	credB, err := srv.Store.CreateCredential(xc.ID, "userB", "passB")
	if err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}

	resp, err := client.PostForm(ts.URL+"/admin/users/new", url.Values{
		"username":               {"alice"},
		"password":               {"hunter2"},
		"credential_id":          {strconv.FormatInt(credB.ID, 10)},
		"max_concurrent_streams": {"1"},
		"csrf_token":             {token},
	})
	if err != nil {
		t.Fatalf("POST users/new error: %v", err)
	}
	resp.Body.Close() // nolint: errcheck

	users, err := srv.Store.ListUsers()
	if err != nil || len(users) != 1 {
		t.Fatalf("ListUsers() = %+v, err %v, want 1 user", users, err)
	}
	if users[0].CredentialID != credB.ID {
		t.Errorf("CredentialID = %d, want %d (userB, not userA=%d)", users[0].CredentialID, credB.ID, credA.ID)
	}

	_, backend, err := srv.Store.Authenticate("alice", "hunter2")
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}
	if backend.XtreamUser != "userB" {
		t.Errorf("resolved XtreamUser = %q, want %q", backend.XtreamUser, "userB")
	}
}
