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
	"github.com/gin-gonic/gin"
)

func (c *Config) routes(r *gin.RouterGroup) {
	r = r.Group(c.CustomEndpoint)

	c.xtreamRoutes(r)
}

func (c *Config) xtreamRoutes(r *gin.RouterGroup) {
	getphp := gin.HandlerFunc(c.xtreamGet)
	if c.XtreamGenerateApiGet {
		getphp = c.xtreamApiGet
	}
	r.GET("/get.php", c.authenticate, getphp)
	r.POST("/get.php", c.authenticate, getphp)
	r.GET("/apiget", c.authenticate, c.xtreamApiGet)
	r.GET("/player_api.php", c.authenticate, c.xtreamPlayerAPIGET)
	r.POST("/player_api.php", c.appAuthenticate, c.xtreamPlayerAPIPOST)
	r.GET("/xmltv.php", c.authenticate, c.xtreamXMLTV)
	r.GET("/:proxyUser/:proxyPass/:id", c.xtreamStreamHandler)
	r.GET("/live/:proxyUser/:proxyPass/:id", c.xtreamStreamLive)
	r.GET("/timeshift/:proxyUser/:proxyPass/:duration/:start/:id", c.xtreamStreamTimeshift)
	r.GET("/movie/:proxyUser/:proxyPass/:id", c.xtreamStreamMovie)
	r.GET("/series/:proxyUser/:proxyPass/:id", c.xtreamStreamSeries)
	r.GET("/hlsr/:token/:proxyUser/:proxyPass/:channel/:hash/:chunk", c.xtreamHlsrStream)
	r.GET("/hls/:token/:chunk", c.xtreamHlsStream)
	r.GET("/play/:proxyUser/:proxyPass/:token/:type", c.xtreamStreamPlay)
}
