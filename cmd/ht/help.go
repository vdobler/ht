// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/vdobler/ht/ht"
)

var cmdHelp = &Command{
	RunArgs:     runHelp,
	Usage:       "help [command | topic]",
	Description: "print help information",
	Flag:        flag.NewFlagSet("help", flag.ContinueOnError),
	Help: `Help shows help for ht its subcommands and selected topics.

The available help topics are:
    checks       displays the list of builtin checks
    extractors   displays the builtin variable extractors
    archive      explains archive files
`,
}

func runHelp(cmd *Command, args []string) {
	if len(args) == 0 {
		usage()
		os.Exit(0)
	}

	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

	arg := args[0]

	// Special case of list of checks/extractors and archives
	switch arg {
	case "check", "checks", "extractor", "extractors":
		displayChecksOrExtractors(arg)
		os.Exit(0)
	case "archive", "archives":
		displayArchiveHelp()
		os.Exit(0)
	}

	for _, cmd := range commands {
		if cmd.Name() == arg {
			fmt.Printf(`Usage:

    ht %s
%s
Flags:
`, cmd.Usage, cmd.Help)
			cmd.Flag.PrintDefaults()
			os.Exit(0)
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown help topic %#q.  Run 'ht help'.\n", arg)
	os.Exit(9) // failed at 'go help cmd'

}

func displayChecksOrExtractors(which string) {
	names := []string{}
	if which[0] == 'c' {
		for name := range ht.CheckRegistry {
			names = append(names, name)
		}
	} else {
		for name := range ht.ExtractorRegistry {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	fmt.Println(strings.Join(names, "\n"))
}

func displayArchiveHelp() {
	fmt.Println(`
Several commands accept archive files which combine everything into one
large file. Such an archive file consists of the concatenation of the
different Hjson documents where each document is preceded by a comment
stating the filename of the document. Any Hjson document can be accessed
with the syntax <filename>@<archive>. Inside the archive the plain filename
is sufficient.

The following example demonstrates using this feature:

    $ cat archive
    # some.suite
    {
        Name: "Some Suite"
        Main: [ {File: sometest.ht}, {File: other.ht} ]
    }

    # sometest.ht
    {
        Request: { URL: "http://localhost/foo" }
        Checks: [ {Check: "StatusCode", Expect: 200} ]
    }

    # other.ht
    {
        Request: { URL: "http://localhost/foo" }
        Checks: [ {Check: "StatusCode", Expect: 505} ]
    }

    $ ht exec some.suite@archive
`)
}
