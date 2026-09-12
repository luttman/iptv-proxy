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
	"io"
	"log"
	"os"
	"time"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var encryptCmd = &cobra.Command{
	Use:   "encrypt-credentials",
	Short: "Encrypt existing plaintext xtream-code credentials in the database with the configured encryption key",
	Run: func(cmd *cobra.Command, args []string) {
		key, err := store.LoadEncryptionKey(viper.GetString("encryption-key-file"), viper.GetString("encryption-key"))
		if err != nil {
			log.Fatal(err)
		}
		if key == nil {
			log.Fatal("--encryption-key-file or ENCRYPTION_KEY is required to encrypt credentials")
		}

		dbPath := viper.GetString("db-path")
		backupPath := fmt.Sprintf("%s.bak-%d", dbPath, time.Now().Unix())
		if err := copyFile(dbPath, backupPath); err != nil {
			log.Fatalf("backup database before migrating: %v", err)
		}
		fmt.Println("Backed up database to", backupPath)

		// Open without a key: the database is still plaintext at this
		// point, so nothing needs decrypting yet.
		st, err := store.Open(dbPath, nil)
		if err != nil {
			log.Fatal(err)
		}
		defer st.Close() // nolint: errcheck

		if err := st.EncryptExistingCredentials(key); err != nil {
			log.Fatalf("encrypt credentials (database restored from %s): %v", backupPath, err)
		}

		fmt.Println("Encrypted upstream credentials. Keep", backupPath, "and the encryption key safe: both are needed to recover the database.")
	},
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() // nolint: errcheck

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer out.Close() // nolint: errcheck

	_, err = io.Copy(out, in)
	return err
}

func init() {
	rootCmd.AddCommand(encryptCmd)
}
