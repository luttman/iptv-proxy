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

package xtreamproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	xtream "github.com/tellytv/go.xtream-codes"
)

const (
	getLiveCategories   = "get_live_categories"
	getLiveStreams      = "get_live_streams"
	getVodCategories    = "get_vod_categories"
	getVodStreams       = "get_vod_streams"
	getVodInfo          = "get_vod_info"
	getSeriesCategories = "get_series_categories"
	getSeries           = "get_series"
	getSerieInfo        = "get_series_info"
	getShortEPG         = "get_short_epg"
	getSimpleDataTable  = "get_simple_data_table"
)

// Client represent an xtream client
type Client struct {
	*xtream.XtreamClient
}

// New new xtream client
func New(ctx context.Context, user, password, baseURL, userAgent string) (*Client, error) {
	cli, err := xtream.NewClientWithUserAgent(ctx, user, password, baseURL, userAgent)
	if err != nil {
		return nil, err
	}

	return &Client{cli}, nil
}

// NewWithHTTP injects the server's client before the initial login. The
// upstream constructor logs in using http.DefaultClient before accepting context.
func NewWithHTTP(ctx context.Context, user, password, baseURL, userAgent string, httpClient *http.Client) (*Client, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	c := &Client{&xtream.XtreamClient{
		Username: user, Password: password, BaseURL: strings.TrimRight(baseURL, "/"),
		UserAgent: userAgent, Context: ctx, HTTP: httpClient,
	}}
	var auth xtream.AuthenticationResponse
	if err := c.requestJSON("", nil, &auth); err != nil {
		return nil, err
	}
	c.UserInfo, c.ServerInfo = auth.UserInfo, auth.ServerInfo
	return c, nil
}

func (c *Client) requestJSON(action string, params url.Values, result interface{}) error {
	u, err := url.Parse(c.BaseURL + "/player_api.php")
	if err != nil {
		return fmt.Errorf("invalid provider URL")
	}
	q := u.Query()
	q.Set("username", c.Username)
	q.Set("password", c.Password)
	if action != "" {
		q.Set("action", action)
	}
	for key, values := range params {
		q[key] = values
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(c.Context, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() // nolint: errcheck
	if resp.StatusCode >= 400 {
		return fmt.Errorf("provider returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(result)
}

// The upstream stream methods write to a private cache initialized only by
// its default-client constructor. This app only needs the returned stream list.
func (c *Client) GetLiveStreams(categoryID string) ([]xtream.Stream, error) {
	var streams []xtream.Stream
	err := c.requestJSON(getLiveStreams, url.Values{"category_id": {categoryID}}, &streams)
	return streams, err
}

func (c *Client) GetVideoOnDemandStreams(categoryID string) ([]xtream.Stream, error) {
	var streams []xtream.Stream
	err := c.requestJSON(getVodStreams, url.Values{"category_id": {categoryID}}, &streams)
	return streams, err
}

type login struct {
	UserInfo   xtream.UserInfo   `json:"user_info"`
	ServerInfo xtream.ServerInfo `json:"server_info"`
}

// Login xtream login
func (c *Client) login(proxyUser, proxyPassword, proxyURL string, proxyPort int, protocol string) (login, error) {
	req := login{
		UserInfo: xtream.UserInfo{
			Username:             proxyUser,
			Password:             proxyPassword,
			Message:              c.UserInfo.Message,
			Auth:                 c.UserInfo.Auth,
			Status:               c.UserInfo.Status,
			ExpDate:              c.UserInfo.ExpDate,
			IsTrial:              c.UserInfo.IsTrial,
			ActiveConnections:    c.UserInfo.ActiveConnections,
			CreatedAt:            c.UserInfo.CreatedAt,
			MaxConnections:       c.UserInfo.MaxConnections,
			AllowedOutputFormats: c.UserInfo.AllowedOutputFormats,
		},
		ServerInfo: xtream.ServerInfo{
			URL:          proxyURL,
			Port:         xtream.FlexInt(proxyPort),
			HTTPSPort:    xtream.FlexInt(proxyPort),
			Protocol:     protocol,
			RTMPPort:     xtream.FlexInt(proxyPort),
			Timezone:     c.ServerInfo.Timezone,
			TimestampNow: c.ServerInfo.TimestampNow,
			TimeNow:      c.ServerInfo.TimeNow,
		},
	}

	return req, nil
}

// Action execute an xtream action.
func (c *Client) Action(proxyUser, proxyPassword string, hostConfig *config.HostConfiguration, https bool, advertisedPort int, action string, q url.Values) (respBody interface{}, httpcode int, err error) {
	protocol := "http"
	if https {
		protocol = "https"
	}

	switch action {
	case getLiveCategories:
		respBody, err = c.GetLiveCategories()
	case getLiveStreams:
		categoryID := ""
		if len(q["category_id"]) > 0 {
			categoryID = q["category_id"][0]
		}
		respBody, err = c.GetLiveStreams(categoryID)
	case getVodCategories:
		respBody, err = c.GetVideoOnDemandCategories()
	case getVodStreams:
		categoryID := ""
		if len(q["category_id"]) > 0 {
			categoryID = q["category_id"][0]
		}
		respBody, err = c.GetVideoOnDemandStreams(categoryID)
	case getVodInfo:
		httpcode, err = validateParams(q, "vod_id")
		if err != nil {
			return
		}
		respBody, err = c.GetVideoOnDemandInfo(q["vod_id"][0])
	case getSeriesCategories:
		respBody, err = c.GetSeriesCategories()
	case getSeries:
		categoryID := ""
		if len(q["category_id"]) > 0 {
			categoryID = q["category_id"][0]
		}
		respBody, err = c.GetSeries(categoryID)
	case getSerieInfo:
		httpcode, err = validateParams(q, "series_id")
		if err != nil {
			return
		}
		respBody, err = c.GetSeriesInfo(q["series_id"][0])
	case getShortEPG:
		limit := 0

		httpcode, err = validateParams(q, "stream_id")
		if err != nil {
			return
		}
		if len(q["limit"]) > 0 {
			limit, err = strconv.Atoi(q["limit"][0])
			if err != nil {
				httpcode = http.StatusInternalServerError
				return
			}
		}
		respBody, err = c.GetShortEPG(q["stream_id"][0], limit)
	case getSimpleDataTable:
		httpcode, err = validateParams(q, "stream_id")
		if err != nil {
			return
		}
		respBody, err = c.GetEPG(q["stream_id"][0])
	default:
		respBody, err = c.login(proxyUser, proxyPassword, protocol+"://"+hostConfig.Hostname, advertisedPort, protocol)
	}

	return
}

func validateParams(u url.Values, params ...string) (int, error) {
	for _, p := range params {
		if len(u[p]) < 1 {
			return http.StatusBadRequest, fmt.Errorf("missing %q", p)
		}

	}

	return 0, nil
}
