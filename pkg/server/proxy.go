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

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

type outboundProxy struct {
	sync.RWMutex
	settings store.ProxySettings
}

// TestOutboundProxy checks the selected proxy without saving or enabling it.
// Environment mode tests HTTPS when available, otherwise HTTP.
func (c *Config) TestOutboundProxy(ctx context.Context, s store.ProxySettings) (string, error) {
	s.Enabled = true
	p := &outboundProxy{settings: s}
	target := "https://api.ipify.org?format=json"
	if s.UseEnvironment {
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		u, err := p.proxy(req)
		if err != nil {
			return "", errors.New("invalid environment proxy configuration")
		}
		if u == nil {
			target = "http://api.ipify.org?format=json"
		}
	}
	return testOutboundProxy(ctx, s, target)
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
		return "", errors.New("no proxy selected for the test destination; check the URL, environment variables, and NO_PROXY")
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

func (p *outboundProxy) proxy(req *http.Request) (*url.URL, error) {
	p.RLock()
	s := p.settings
	p.RUnlock()
	if !s.Enabled {
		return nil, nil
	}
	if s.UseEnvironment {
		return http.ProxyFromEnvironment(req)
	}
	return validateProxyURL(s.URL)
}

func (c *Config) ProxySettings() store.ProxySettings {
	c.outboundProxy.RLock()
	defer c.outboundProxy.RUnlock()
	s := c.outboundProxy.settings
	if s.UseEnvironment && !config.HasEnvironmentProxy() {
		s.Enabled = false
	}
	return s
}

// SetProxySettings saves first, then applies to new requests. Existing streams
// keep their connection; idle connections are discarded.
func (c *Config) SetProxySettings(s store.ProxySettings) error {
	if s.UseEnvironment && !config.HasEnvironmentProxy() {
		s.Enabled = false
	}
	s.URL = strings.TrimSpace(s.URL)
	if s.URL != "" {
		if _, err := validateProxyURL(s.URL); err != nil {
			return err
		}
	} else if !s.UseEnvironment && s.Enabled {
		return errors.New("enter a proxy URL before enabling a custom proxy")
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
