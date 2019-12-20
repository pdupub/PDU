// Copyright 2019 The PDU Authors
// This file is part of the PDU library.
//
// The PDU library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The PDU library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the PDU library. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"github.com/mitchellh/go-homedir"
	"github.com/pdupub/go-pdu/common/log"
	"github.com/pdupub/go-pdu/db"
	"github.com/pdupub/go-pdu/db/bolt"
	"github.com/pdupub/go-pdu/params"
	"github.com/spf13/cobra"
	"os"
	"path"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new PDU Universe",
	RunE: func(_ *cobra.Command, args []string) error {
		log.Info("create ...")
		var udb db.UDB
		home, _ := homedir.Dir()
		dbDirPath := path.Join(home, params.DefaultPath)
		os.Mkdir(dbDirPath, os.ModePerm)
		dbFilePath := path.Join(dbDirPath, "u.db")
		udb, err := bolt.NewDB(dbFilePath)
		if err != nil {
			log.Error(err)
			return err
		}
		udb.CreateBucket([]byte("universe"))
		udb.Close()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}