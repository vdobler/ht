// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run genexample.go

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

var cmdExample = &Command{
	RunArgs:     runExample,
	Usage:       "example [topic]",
	Description: "display examples for common tasks",
	Flag:        flag.NewFlagSet("example", flag.ContinueOnError),
	Help: `Example prints examples for common tasks.

Examples including comments are sometimes easier to understand and adopt
than plain documentation.
`,
}

// Examples in the example subcommand.
type Example struct {
	Name        string     // Name like "Test" or "Test.SQL.Update"
	Description string     // Short description
	Data        string     // The real example
	Sub         []*Example // Subtopics below this Example
}

func subexamples(names []string, prefix string, level int) []*Example {
	var subs []*Example
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) ||
			strings.Count(name, ".") != level ||
			strings.HasSuffix(name, "~") {
			// fmt.Println("  skipping", name)
			continue
		}

		bdata, err := ioutil.ReadFile("./examples/" + name)
		if err != nil {
			panic(err)
		}
		bdata = bytes.TrimRight(bdata, " \n\t")

		example := Example{
			Name: name,
			Data: string(bdata),
			Sub:  subexamples(names, name+".", level+1),
		}
		eol := strings.Index(example.Data, "\n")
		example.Description = example.Data[3:eol]

		subs = append(subs, &example)
	}

	return subs
}

// findExample looks up an example needle like ["test", "post", "upload]
func findExample(needle []string, ex *Example) *Example {
	level := strings.Count(ex.Name, ".")
	if ex.Name != "" {
		level++
	}

	for _, sub := range ex.Sub {
		name := sub.Name
		if ex.Name != "" {
			name = name[len(ex.Name)+1:]
		}
		name = strings.ToLower(name)
		if !strings.HasPrefix(name, needle[level]) {
			continue
		}

		// Descend or done?
		if level+1 == len(needle) {
			return sub
		}
		return findExample(needle, sub)
	}

	return nil
}

func runExample(cmd *Command, args []string) {
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

	example := RootExample
	if len(args) == 1 {
		arg := args[0]
		arg = strings.ToLower(arg)
		example = findExample(strings.Split(arg, "."), RootExample)
		if example == nil {
			fmt.Fprintf(os.Stderr, "No example for %q.\n", arg)
			os.Exit(8)
		}
	}

	fmt.Fprintln(os.Stdout, example.Data)

	if len(example.Sub) > 0 {
		fmt.Fprintln(os.Stderr, "Available subtopics:")
		width := 0
		for _, sub := range example.Sub {
			if n := len(sub.Name); n > width {
				width = n
			}
		}
		for _, sub := range example.Sub {
			fmt.Fprintf(os.Stderr, "  * %-*s   %s\n",
				width, sub.Name, sub.Description)
		}
	}
	os.Exit(0)
}
