package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

type outboundProxy struct {
	sync.RWMutex
	settings store.ProxySettings
}

// TestOutboundProxy checks the entered proxy without saving or enabling it.
func (c *Config) TestOutboundProxy(ctx context.Context, s store.ProxySettings) (string, error) {
	return testOutboundProxy(ctx, s, "https://api.ipify.org?format=json")
}

func testOutboundProxy(ctx context.Context, s store.ProxySettings, target string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s.Enabled = true
	s.URL = strings.TrimSpace(s.URL)
	p := &outboundProxy{settings: s}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", errors.New("invalid test destination")
	}
	u, err := p.proxy(req)
	if err != nil {
		return "", errors.New("enter a valid HTTP or SOCKS5 proxy URL")
	}
	if u == nil {
		return "", errors.New("enter a proxy URL before testing")
	}
	client := newUpstreamHTTPClient()
	transport := client.Transport.(*http.Transport)
	transport.Proxy = http.ProxyURL(u)
	defer client.CloseIdleConnections()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("proxy test failed: check the address, credentials, and connectivity (10-second timeout)")
	}
	defer resp.Body.Close() // nolint: errcheck
	if resp.StatusCode == http.StatusProxyAuthRequired {
		return "", errors.New("proxy authentication failed; check the username and password")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("proxy test returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result); err != nil || net.ParseIP(result.IP) == nil {
		return "", errors.New("proxy test returned an invalid IP response")
	}
	return fmt.Sprintf("Proxy works over %s. Exit IP: %s (%d ms). Settings were not changed.", strings.ToUpper(req.URL.Scheme), result.IP, time.Since(start).Milliseconds()), nil
}

func validateProxyURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("invalid proxy URL")
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, errors.New("proxy URL must start with http://, https://, or socks5://")
	}
	if u.Hostname() == "" || u.Opaque != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("proxy URL must contain a host and optional port, without a path, query, or fragment")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return nil, errors.New("proxy port must be between 1 and 65535")
		}
	}
	return u, nil
}

func (p *outboundProxy) proxy(_ *http.Request) (*url.URL, error) {
	p.RLock()
	s := p.settings
	p.RUnlock()
	if !s.Enabled {
		return nil, nil
	}
	return validateProxyURL(s.URL)
}

func (c *Config) ProxySettings() store.ProxySettings {
	c.outboundProxy.RLock()
	defer c.outboundProxy.RUnlock()
	s := c.outboundProxy.settings
	if s.URL == "" {
		s.Enabled = false
	}
	return s
}

// SetProxySettings saves first, then applies to new requests. Existing streams
// keep their connection; idle connections are discarded.
func (c *Config) SetProxySettings(s store.ProxySettings) error {
	s.URL = strings.TrimSpace(s.URL)
	if s.URL != "" {
		if _, err := validateProxyURL(s.URL); err != nil {
			return err
		}
	} else if s.Enabled {
		return errors.New("enter a proxy URL before enabling it")
	}
	c.outboundProxy.Lock()
	defer c.outboundProxy.Unlock()
	if err := c.Store.SaveProxySettings(s); err != nil {
		return err
	}
	c.outboundProxy.settings = s
	c.httpClient.CloseIdleConnections()
	return nil
}
