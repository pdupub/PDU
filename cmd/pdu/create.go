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
	"crypto/ecdsa"
	"fmt"
	"github.com/howeyc/gopass"
	"github.com/mitchellh/go-homedir"
	"github.com/pdupub/go-pdu/common"
	"github.com/pdupub/go-pdu/core"
	"github.com/pdupub/go-pdu/crypto"
	"github.com/pdupub/go-pdu/db"
	"github.com/pdupub/go-pdu/db/bolt"
	"github.com/pdupub/go-pdu/params"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new PDU Universe",
	RunE: func(_ *cobra.Command, args []string) error {
		udb, err := initDB(dataDir)
		if err != nil {
			return err
		}
		fmt.Println("Database initialized successfully", dataDir)

		priKeys, err := unlockRootsKeys(2)
		if err != nil {
			os.RemoveAll(dataDir)
			return err
		}
		fmt.Println("Unlock root key successfully")

		users, err := createRootUsers(priKeys)
		if err != nil {
			os.RemoveAll(dataDir)
			return err
		}
		fmt.Println("Create root users successfully", users[0].Gender(), users[1].Gender())

		fmt.Println("Create universe and space-time successfully")

		if err := udb.Close(); err != nil {
			return err
		}
		fmt.Println("Database closed successfully")
		return nil
	},
}

func createRootUsers(priKeys []*crypto.PrivateKey) (users []*core.User, err error) {
	for _, v := range priKeys {
		for {

			var pubKey crypto.PublicKey
			if v.SigType == crypto.Signature2PublicKey {
				pk := v.PriKey.(*ecdsa.PrivateKey)
				pubKey = crypto.PublicKey{Source: v.Source, SigType: v.SigType, PubKey: pk.PublicKey}
			} else if v.SigType == crypto.MultipleSignatures {
				var pubKeys []interface{}
				pks := v.PriKey.([]*ecdsa.PrivateKey)
				for _, pk := range pks {
					pubKeys = append(pubKeys, pk.PublicKey)
				}
				pubKey = crypto.PublicKey{Source: v.Source, SigType: v.SigType, PubKey: pubKeys}
			} else {
				return users, crypto.ErrSigTypeNotSupport
			}

			var rootName, rootExtra, isSave string
			fmt.Print("name: ")
			fmt.Scan(&rootName)
			fmt.Print("extra: ")
			fmt.Scan(&rootExtra)
			user := core.CreateRootUser(pubKey, rootName, rootExtra)
			fmt.Println("ID", common.Hash2String(user.ID()), "name", user.Name, "extra", user.DOBExtra, "gender", user.Gender())
			fmt.Print("save new user (yes/no): ")
			fmt.Scan(&isSave)
			if strings.ToUpper(isSave) == "YES" || strings.ToUpper(isSave) == "Y" {
				users = append(users, user)
				break
			}
		}

	}
	return users, err
}

func unlockRootsKeys(cnt int) (priKeys []*crypto.PrivateKey, err error) {
	for i := 0; i < cnt; i++ {
		priKey, err := unlockKey()
		if err != nil {
			return priKeys, err
		}
		priKeys = append(priKeys, priKey)
	}
	return priKeys, err
}

func unlockKey() (*crypto.PrivateKey, error) {
	var keyFile string
	fmt.Print("keyfile path: ")
	fmt.Scan(&keyFile)
	keyJson, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	fmt.Print("password: ")
	passwd, err := gopass.GetPasswd()
	if err != nil {
		return nil, err
	}

	return core.DecryptKey(keyJson, string(passwd))
}

func initDB(dataDir string) (db.UDB, error) {
	if dataDir == "" {
		home, _ := homedir.Dir()
		dataDir = path.Join(home, params.DefaultPath)
	}
	err := os.Mkdir(dataDir, os.ModePerm)
	if err != nil {
		return nil, err
	}
	dbFilePath := path.Join(dataDir, "u.db")
	udb, err := bolt.NewDB(dbFilePath)
	if err != nil {
		return nil, err
	}

	bucketName := []byte("universe")
	if err := udb.CreateBucket(bucketName); err != nil {
		return nil, err
	}
	return udb, nil
}

func init() {
	rootCmd.AddCommand(createCmd)
}
