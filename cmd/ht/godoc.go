// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
)

var cmdDoc = &Command{
	RunArgs:     runDoc,
	Usage:       "doc <type>",
	Description: "print godoc of type",
	Flag:        flag.NewFlagSet("doc", flag.ContinueOnError),
	Help: `
Doc displays detail information of a type by running running
'go doc github.com/vdobler/ht/ht <type>'. To use this subcommand
you need a working Go installation and a local copy of ht's source.
`,
}

func runDoc(cmd *Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: ht doc <type>")
		os.Exit(9)
	}

	typ := args[0]
	gocmd := exec.Command("go", "doc", "github.com/vdobler/ht/ht", typ)
	output, err := gocmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error invoking go doc: %s\n%s\n", err, string(output))
		os.Exit(9)
	}

	// This is inefficient, but okay direktly befor a os.Exit.
	buf := &bytes.Buffer{}
	for _, line := range bytes.Split(output, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("func (")) {
			continue
		}
		buf.Write(line)
		buf.WriteRune('\n')
	}
	fmt.Println(string(bytes.TrimSpace(buf.Bytes())))
	os.Exit(0)
}
