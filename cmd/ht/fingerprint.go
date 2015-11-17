// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/vdobler/ht/fingerprint"
)

var cmdFingerprint = &Command{
	RunArgs:     runFingerprint,
	Usage:       "fingerprint <fileOrURL>...",
	Description: "calculate image fingerprints",
	Flag:        flag.NewFlagSet("fingerprint", flag.ContinueOnError),
	Help: `
Fingerprint calculates image fingerprints for the given arguments.
If an argument start with 'http://' or 'https://' it is treated as
an URL otherwise as a filename.
The files or URLs are loaded and the two image fingerprints are
displayed.
	`,
}

func runFingerprint(cmd *Command, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Missing arguments to fingerprint")
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

	max := 0
	for _, a := range args {
		if l := len(a); l > max {
			max = l
		}
	}

	fmt.Printf("%-*s :  %-16s  %-24s\n", max, "# Image Path", "BMV-Hash", "ColorHist-Hash")
	okay := true
	for _, a := range args {
		fmt.Printf("%-*s :  ", max, a)
		img, err := readImage(a)
		if err != nil {
			fmt.Printf("Error %s\n", err)
			okay = false
		} else {
			ch := fingerprint.NewColorHist(img)
			bmv := fingerprint.NewBMVHash(img)
			fmt.Printf("%s  %s\n", bmv.String(), ch.String())
		}
	}

	if okay {
		os.Exit(0)
	}
	os.Exit(8)
}

func readImage(name string) (image.Image, error) {
	var in io.Reader
	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		resp, err := http.Get(name)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		in = resp.Body
	} else {
		file, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		in = file
	}
	img, _, err := image.Decode(in)
	if err != nil {
		return nil, err
	}
	return img, nil
}
