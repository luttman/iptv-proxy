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

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

func xtreamLoginServer(t *testing.T, expUnix string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expField := "null"
		if expUnix != "" {
			expField = `"` + expUnix + `"`
		}
		_, _ = fmt.Fprintf(w, `{
			"user_info": {"username":"u","password":"p","auth":1,"status":"Active","exp_date":%s,"is_trial":"0","active_cons":"0","created_at":"1000","max_connections":"1","allowed_output_formats":["ts"]},
			"server_info": {"url":"x","port":"80","https_port":"443","server_protocol":"http","rtmp_port":"25462","timezone":"UTC","timestamp_now":1000,"time_now":"2024-01-01 00:00:00"}
		}`, expField)
	}))
}

func TestFetchSubscriptionExpiryWithExpiry(t *testing.T) {
	expiresAt := time.Now().Add(72 * time.Hour)
	upstream := xtreamLoginServer(t, fmt.Sprintf("%d", expiresAt.Unix()))
	defer upstream.Close()

	backend := store.ResolvedBackend{Name: "provider-a", BaseURL: upstream.URL, XtreamUser: "u", XtreamPassword: "p"}
	expiry, err := fetchSubscriptionExpiry(context.Background(), backend)
	if err != nil {
		t.Fatalf("fetchSubscriptionExpiry() error: %v", err)
	}
	if !expiry.HasExpiry {
		t.Fatal("HasExpiry = false, want true")
	}
	if expiry.DaysLeft != 2 && expiry.DaysLeft != 3 {
		t.Errorf("DaysLeft = %d, want ~3", expiry.DaysLeft)
	}
	if expiry.CheckedAtUnix == 0 {
		t.Error("CheckedAtUnix not set")
	}
}

func TestFetchSubscriptionExpiryUnlimited(t *testing.T) {
	upstream := xtreamLoginServer(t, "")
	defer upstream.Close()

	backend := store.ResolvedBackend{Name: "provider-a", BaseURL: upstream.URL, XtreamUser: "u", XtreamPassword: "p"}
	expiry, err := fetchSubscriptionExpiry(context.Background(), backend)
	if err != nil {
		t.Fatalf("fetchSubscriptionExpiry() error: %v", err)
	}
	if expiry.HasExpiry {
		t.Error("HasExpiry = true, want false for a plan with no exp_date")
	}
}

func TestFetchSubscriptionExpiryAlreadyExpired(t *testing.T) {
	expiredAt := time.Now().Add(-48 * time.Hour)
	upstream := xtreamLoginServer(t, fmt.Sprintf("%d", expiredAt.Unix()))
	defer upstream.Close()

	backend := store.ResolvedBackend{Name: "provider-a", BaseURL: upstream.URL, XtreamUser: "u", XtreamPassword: "p"}
	expiry, err := fetchSubscriptionExpiry(context.Background(), backend)
	if err != nil {
		t.Fatalf("fetchSubscriptionExpiry() error: %v", err)
	}
	if !expiry.HasExpiry || expiry.DaysLeft >= 0 {
		t.Errorf("expiry = %+v, want HasExpiry=true and a negative DaysLeft", expiry)
	}
}

func TestCheckSubscriptionsNarrowsMultiAddressBackend(t *testing.T) {
	upstream := xtreamLoginServer(t, fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()))
	defer upstream.Close()

	c := newTestConfig(t)
	c.subscriptionExpiry = map[int64]SubscriptionExpiry{}

	// A backend with a failover address list (space/newline separated)
	// used to be passed to the xtream client as-is, which url.Parse
	// rejects outright ("invalid control character in URL").
	xc, err := c.Store.CreateXtreamCode("provider-a")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if err := c.Store.AddAddresses(xc.ID, upstream.URL+"\r\nhttp://unreachable.invalid"); err != nil {
		t.Fatalf("AddAddresses() error: %v", err)
	}
	cred, err := c.Store.CreateCredential(xc.ID, "", "u", "p")
	if err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}

	c.checkSubscriptions(context.Background())

	got, ok := c.SubscriptionExpiries()[cred.ID]
	if !ok || !got.HasExpiry {
		t.Errorf("SubscriptionExpiries()[%d] = %+v, ok=%v, want a resolved expiry", cred.ID, got, ok)
	}
}

func TestCheckSubscriptionsPopulatesSnapshot(t *testing.T) {
	upstream := xtreamLoginServer(t, fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()))
	defer upstream.Close()

	c := newTestConfig(t)
	c.subscriptionExpiry = map[int64]SubscriptionExpiry{}

	xc, err := c.Store.CreateXtreamCode("provider-a")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if err := c.Store.AddAddresses(xc.ID, upstream.URL); err != nil {
		t.Fatalf("AddAddresses() error: %v", err)
	}
	cred, err := c.Store.CreateCredential(xc.ID, "", "u", "p")
	if err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}

	c.checkSubscriptions(context.Background())

	snap := c.SubscriptionExpiries()
	got, ok := snap[cred.ID]
	if !ok {
		t.Fatal("SubscriptionExpiries() missing entry for credential")
	}
	if !got.HasExpiry {
		t.Errorf("HasExpiry = false, want true")
	}
}

func TestCheckSubscriptionsTracksEachCredentialSeparately(t *testing.T) {
	soon := xtreamLoginServer(t, fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()))
	defer soon.Close()

	c := newTestConfig(t)
	c.subscriptionExpiry = map[int64]SubscriptionExpiry{}

	xc, err := c.Store.CreateXtreamCode("provider-a")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	// Both credentials share the same address list; each one's own
	// login response determines its own expiry.
	if err := c.Store.AddAddresses(xc.ID, soon.URL); err != nil {
		t.Fatalf("AddAddresses() error: %v", err)
	}
	credA, err := c.Store.CreateCredential(xc.ID, "A", "u", "p")
	if err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}
	credB, err := c.Store.CreateCredential(xc.ID, "B", "u", "p")
	if err != nil {
		t.Fatalf("CreateCredential() error: %v", err)
	}

	c.checkSubscriptions(context.Background())

	snap := c.SubscriptionExpiries()
	if _, ok := snap[credA.ID]; !ok {
		t.Errorf("SubscriptionExpiries() missing entry for credential A (%d)", credA.ID)
	}
	if _, ok := snap[credB.ID]; !ok {
		t.Errorf("SubscriptionExpiries() missing entry for credential B (%d)", credB.ID)
	}
}
