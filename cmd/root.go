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

package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/admin"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/server"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "iptv-proxy",
	Short: "Reverse proxy on xtream codes server api, with multi-user/multi-backend admin UI",
	Run: func(cmd *cobra.Command, args []string) {
		adminUser := viper.GetString("admin-user")
		adminPassword := viper.GetString("admin-password")
		if adminUser == "" || adminPassword == "" {
			log.Fatal("--admin-user and --admin-password (or ADMIN_USER/ADMIN_PASSWORD) are required")
		}

		key, err := store.LoadEncryptionKey(viper.GetString("encryption-key-file"), viper.GetString("encryption-key"))
		if err != nil {
			log.Fatal(err)
		}

		st, err := store.Open(viper.GetString("db-path"), key)
		if err != nil {
			log.Fatal(err)
		}

		conf := &config.ProxyConfig{
			HostConfig: &config.HostConfiguration{
				Hostname: viper.GetString("hostname"),
				Port:     viper.GetInt("port"),
			},
			M3UCacheExpiration:   viper.GetInt("m3u-cache-expiration"),
			AdvertisedPort:       viper.GetInt("advertised-port"),
			HTTPS:                viper.GetBool("https"),
			M3UFileName:          viper.GetString("m3u-file-name"),
			CustomEndpoint:       viper.GetString("custom-endpoint"),
			XtreamGenerateApiGet: viper.GetBool("xtream-api-get"),
		}

		if conf.AdvertisedPort == 0 {
			conf.AdvertisedPort = conf.HostConfig.Port
		}

		srv, err := server.NewServer(conf, st)
		if err != nil {
			log.Fatal(err)
		}

		if err := admin.Register(srv, admin.Credentials{Username: adminUser, Password: adminPassword}); err != nil {
			log.Fatal(err)
		}

		if e := srv.Serve(); e != nil {
			log.Fatal(e)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "iptv-proxy-config", "C", "Config file (default is $HOME/.iptv-proxy.yaml)")
	rootCmd.Flags().StringP("db-path", "", "./iptv-proxy.db", "Path to the SQLite database file storing users and xtream-code backends")
	rootCmd.Flags().StringP("encryption-key-file", "", "", "Path to a mounted secret file holding the 32-byte AES-256 key used to encrypt upstream credentials at rest (or set ENCRYPTION_KEY)")
	rootCmd.Flags().StringP("admin-user", "", "", "Admin username for the /admin management UI (required)")
	rootCmd.Flags().StringP("admin-password", "", "", "Admin password for the /admin management UI (required)")
	rootCmd.Flags().StringP("m3u-file-name", "", "iptv.m3u", `Name of the proxified m3u file e.g "http://proxy.com/iptv.m3u"`)
	rootCmd.Flags().StringP("custom-endpoint", "", "", `Custom endpoint "http://proxy.com/<custom-endpoint>/iptv.m3u"`)
	rootCmd.Flags().Int("port", 8080, "Iptv-proxy listening port")
	rootCmd.Flags().Int("advertised-port", 0, "Port to expose the IPTV file and xtream (by default, it's taking value from port) useful to put behind a reverse proxy")
	rootCmd.Flags().String("hostname", "", "Hostname or IP to expose the IPTVs endpoints")
	rootCmd.Flags().BoolP("https", "", false, "Activate https for urls proxy")
	rootCmd.Flags().Int("m3u-cache-expiration", 1, "M3U cache expiration in hour")
	rootCmd.Flags().BoolP("xtream-api-get", "", false, "Generate get.php from xtream API instead of get.php original endpoint")

	if e := viper.BindPFlags(rootCmd.Flags()); e != nil {
		log.Fatal("error binding PFlags to viper")
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".iptv-proxy" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName(".iptv-proxy")
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
