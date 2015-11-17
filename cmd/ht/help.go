// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
)

var cmdHelp = &Command{
	RunArgs:     runHelp,
	Usage:       "help [subcommand]",
	Description: "print help information",
	Flag:        flag.NewFlagSet("help", flag.ContinueOnError),
	Help: `
Help shows help for ht as well as for the different subcommands.
	`,
}

func runHelp(cmd *Command, args []string) {
	if len(args) == 0 {
		usage()
		os.Exit(0)
	}

	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(1)
	}

	arg := args[0]
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
