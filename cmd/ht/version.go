// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
)

var cmdVersion = &Command{
	RunArgs:     runVersion,
	Usage:       "version",
	Description: "print version information",
	Flag:        flag.NewFlagSet("version", flag.ContinueOnError),
	Help: `
Version prints version information about ht.
	`,
}

var (
	version = "3.0.0-rc2"
)

func runVersion(cmd *Command, _ []string) {
	fmt.Printf("ht version %s\n", version)
}
