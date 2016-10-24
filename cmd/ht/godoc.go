// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var cmdDoc = &Command{
	RunArgs:     runDoc,
	Usage:       "doc <type>",
	Description: "print godoc of type",
	Flag:        flag.NewFlagSet("doc", flag.ContinueOnError),
	Help: `
Doc displays detail information of types used in for writing tests.
Only the most relevant type documentation is available.
`,
}

func runDoc(cmd *Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: ht doc <type>")
		os.Exit(9)
	}

	typ := strings.ToLower(args[0])
	doc, ok := typeDoc[typ]
	if !ok {
		fmt.Fprintf(os.Stderr, "No information available for type %q\n", typ)
		fmt.Fprintf(os.Stderr, "For serialization types try 'raw%s'.\n", typ)
		os.Exit(9)
	}

	fmt.Println(doc)

	os.Exit(0)
}
