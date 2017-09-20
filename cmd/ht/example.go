// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
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

type Example struct {
	Name        string     // Name like "Test" or "Test.SQL.Update"
	Description string     // Short description
	Data        string     // The real example
	Sub         []*Example // Subtopics below this Example
}

func init() {
	loadExamples()
}

func loadExamples() {
	f, err := os.Open("./examples")
	if err != nil {
		panic(err)
	}
	names, err := f.Readdirnames(-1)
	if err != nil {
		panic(err)
	}

	sort.Strings(names)
	RootExample.Sub = subexamples(names, "", 0)
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

var RootExample = &Example{}

func findExample(name string, ex *Example) *Example {
	if name == ex.Name {
		return ex
	}
	for _, sub := range ex.Sub {
		if strings.HasPrefix(name, sub.Name) {
			return findExample(name, sub)
		}
	}

	return nil
}

func runExample(cmd *Command, args []string) {
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

	arg := ""
	if len(args) == 1 {
		arg = args[0]
	}

	ex := findExample(arg, RootExample)
	if ex == nil {
		fmt.Fprintf(os.Stderr, "No example for %q.\n", arg)
		os.Exit(8)
	}

	fmt.Fprintln(os.Stdout, ex.Data)

	if len(ex.Sub) > 0 {
		fmt.Fprintln(os.Stderr, "Available subtopics")
		width := 0
		for _, sub := range ex.Sub {
			if n := len(sub.Name); n > width {
				width = n
			}
		}
		for _, sub := range ex.Sub {
			fmt.Fprintf(os.Stderr, "  * %-*s   %s\n",
				width, sub.Name, sub.Description)
		}
	}
	os.Exit(0)
}
