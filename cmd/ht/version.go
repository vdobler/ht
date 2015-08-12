// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/vdobler/ht/ht"
)

var cmdVersion = &Command{
	Run:         runVersion,
	Usage:       "version",
	Description: "print version information",
	Help: `
Version prints version information about ht.
	`,
}

var (
	version = "0.2beta"
)

func runVersion(cmd *Command, suites []*ht.Suite) {
	fmt.Printf("ht version %s\n", version)
}
