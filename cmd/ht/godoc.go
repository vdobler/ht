// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gendoc.go

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
	Help: `Doc displays detail information for types used in writing tests.

Only the most relevant type documentation is available. The doc
subcommand outputs the list of checks and extractors if called with
argument 'checks' or 'extractors'.
`,
}

func runDoc(cmd *Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Wrong number of arguments.")
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

	typ := strings.ToLower(args[0])

	// Special case of list of checks/extractors.
	if typ == "check" || typ == "checks" ||
		typ == "extractor" || typ == "extractors" {
		displayChecksOrExtractors(typ)
	}

	doc, ok := typeDoc[typ]
	if !ok {
		fmt.Fprintf(os.Stderr, "No information available for type %q\n", typ)
		fmt.Fprintf(os.Stderr, "For serialization types try 'raw%s'.\n", typ)
		os.Exit(9)
	}

	fmt.Println(doc)

	os.Exit(0)
}

// https://en.wikipedia.org/wiki/Damerau%E2%80%93Levenshtein_distance#Distance_with_adjacent_transpositions
func damerauLevenshtein(s1, s2 string) int {
	a, b := []rune(s1), []rune(s2)
	maxdist := len(a) + len(b)

	// DL("", x) == DL(x, "") == len(x)
	if len(a) == 0 {
		return len(b)
	} else if len(b) == 0 {
		return len(a)
	}

	d := make([][]int, len(a)+2)
	for i := range d {
		d[i] = make([]int, len(b)+2)
	}
	seen := make(map[rune]int)

	d[0][0] = maxdist
	for i := 0; i <= len(a); i++ {
		d[i+1][0] = maxdist
		d[i+1][1] = i
	}
	for j := 0; j <= len(b); j++ {
		d[0][j+1] = maxdist
		d[1][j+1] = j
	}

	for i := 1; i <= len(a); i++ {
		db := 0
		for j := 1; j <= len(b); j++ {
			k := seen[b[j-1]]
			ℓ := db
			cost := 0
			if a[i-1] == b[j-1] {
				db = j
			} else {
				cost = 1
			}
			d[i+1][j+1] = min4(
				d[i][j]+cost,              // substitution
				d[i+1][j]+1,               // insertion
				d[i][j+1]+1,               // deletion
				d[k][ℓ]+(i-k-1)+1+(j-ℓ-1), // transposition
			)
			seen[a[i-1]] = i
		}
	}

	return d[len(a)+1][len(b)+1]
}

func min3(a, b, c int) int {
	if a < b {
		if c < a {
			return c
		}
		return a
	}

	// b < a   (or equal)
	if c < b {
		return c
	}
	return b
}

func min4(a, b, c, d int) int {
	if a < b {
		return min3(a, c, d)
	}
	return min3(b, c, d)
}
