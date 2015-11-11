// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// recorder is a reverse proxy to record requests and output tests.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/vdobler/ht/recorder"
)

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Missing target\n")
		os.Exit(9)
	}

	remote, err := url.Parse(args[0])
	if err != nil {
		panic(err)
	}

	registerTestDumpers()

	err = recorder.StartReverseProxy(":8080", remote)
	if err != nil {
		panic(err)
	}
}

func registerTestDumpers() {
	// http.HandleFunc("/DUMP", dumpEvents)
	log.Println("Endpoint /DUMP registered")
}
