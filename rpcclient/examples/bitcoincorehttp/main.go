// Copyright (c) 2014-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcd/rpcclient"
)

func main() {
	// Connect to local bitcoin core RPC server using HTTP POST mode.

	//	btcdHomeDir := btcutil.AppDataDir("btcd", false)
	btcdHomeDir := "D:\\data\\Btcd"
	certs, err := os.ReadFile(filepath.Join(btcdHomeDir, "rpc.cert"))
	if err != nil {
		log.Fatal(err)
	}

	connCfg := &rpcclient.ConnConfig{
		Host:         "localhost:48334",
		User:         "q17AIoqBJSEhW7djqjn0nTsZcz4=",
		Pass:         "nnlkAZn58bqsyYwVtHIajZ16cj8=",
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		//		DisableTLS:   true, // Bitcoin core does not provide TLS by default
		Certificates: certs,
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Printf("Connect to rpc server failed: %v", err)
		log.Fatal(err)
	}
	defer client.Shutdown()

	// Get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		log.Printf("GetBlockCount from rpc server failed: %v", err)
		log.Fatal(err)
	}
	log.Printf("Block count: %d", blockCount)
}
