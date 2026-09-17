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
	"log/slog"
	"time"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
	xtreamapi "github.com/pierre-emmanuelJ/iptv-proxy/pkg/xtream-proxy"
)

// subscriptionCheckInterval is deliberately much longer than the
// upstream TCP health check: fetching expiry requires a full xtream
// login, which burns a connection slot on providers with strict
// concurrent-connection limits (the same reason xtreamXMLTV avoids an
// extra login per request).
const (
	subscriptionCheckInterval = 6 * time.Hour
	subscriptionCheckTimeout  = 10 * time.Second
)

// SubscriptionExpiry is a backend's upstream subscription status, as
// reported by its own login response. CheckedAtUnix is zero until the
// first successful check.
type SubscriptionExpiry struct {
	HasExpiry     bool
	ExpiresAtUnix int64
	DaysLeft      int
	CheckedAtUnix int64
}

func (c *Config) monitorSubscriptions(ctx context.Context) {
	c.checkSubscriptions(ctx)
	ticker := time.NewTicker(subscriptionCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkSubscriptions(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (c *Config) checkSubscriptions(ctx context.Context) {
	backends, err := c.Store.ListXtreamCodes()
	if err != nil {
		return
	}

	for _, backend := range backends {
		// Each credential is its own account on the provider's panel and
		// can carry its own expiry, so every credential is checked and
		// tracked separately rather than assuming one represents them all.
		credentials, err := c.Store.ListCredentials(backend.ID)
		if err != nil || len(credentials) == 0 {
			continue
		}

		addresses, err := c.Store.EnabledAddressesString(backend.ID)
		if err != nil {
			slog.Warn("subscription check failed", "backend", backend.Name, "error", err)
			continue
		}

		for _, cred := range credentials {
			unresolved := store.ResolvedBackend{
				ID:             backend.ID,
				Name:           backend.Name,
				BaseURL:        addresses,
				XtreamUser:     cred.XtreamUser,
				XtreamPassword: cred.XtreamPassword,
			}

			// Addresses may still hold several failover candidates; narrow
			// to the one failover would actually use before logging in.
			resolved, err := c.selectUpstream(ctx, unresolved, "")
			if err != nil {
				slog.Warn("subscription check failed", "backend", backend.Name, "credential", cred.ID, "error", err)
				continue
			}

			expiry, err := fetchSubscriptionExpiry(ctx, resolved)
			if err != nil {
				slog.Warn("subscription check failed", "backend", backend.Name, "credential", cred.ID, "error", err)
				continue
			}

			c.subscriptionExpiryLock.Lock()
			c.subscriptionExpiry[cred.ID] = expiry
			c.subscriptionExpiryLock.Unlock()
		}
	}
}

func fetchSubscriptionExpiry(ctx context.Context, backend store.ResolvedBackend) (SubscriptionExpiry, error) {
	loginCtx, cancel := context.WithTimeout(ctx, subscriptionCheckTimeout)
	defer cancel()

	cli, err := xtreamapi.New(loginCtx, backend.XtreamUser, backend.XtreamPassword, backend.BaseURL, "iptv-proxy")
	if err != nil {
		return SubscriptionExpiry{}, err
	}

	expiry := SubscriptionExpiry{CheckedAtUnix: time.Now().Unix()}
	if cli.UserInfo.ExpDate != nil {
		expiry.HasExpiry = true
		expiry.ExpiresAtUnix = cli.UserInfo.ExpDate.Unix()
		expiry.DaysLeft = int(time.Until(cli.UserInfo.ExpDate.Time) / (24 * time.Hour))
	}

	return expiry, nil
}

// SubscriptionExpiries returns a snapshot of the latest known
// subscription status per credential, keyed by credential ID. A
// credential not yet present (CheckedAtUnix == 0) hasn't been checked yet.
func (c *Config) SubscriptionExpiries() map[int64]SubscriptionExpiry {
	c.subscriptionExpiryLock.RLock()
	defer c.subscriptionExpiryLock.RUnlock()

	rows := make(map[int64]SubscriptionExpiry, len(c.subscriptionExpiry))
	for id, e := range c.subscriptionExpiry {
		rows[id] = e
	}
	return rows
}
