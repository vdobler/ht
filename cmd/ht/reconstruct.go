// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/vdobler/ht/fingerprint"
)

var cmdReconstruct = &Command{
	RunArgs:     runReconstruct,
	Usage:       "reconstruct [-width w] [-height h] <hash>",
	Description: "reconstruct image from hash",
	Flag:        flag.NewFlagSet("reconstruct", flag.ContinueOnError),
	Help: `
Reconstruct produces an image from the given <hash> whcih may be a 12-byte
color histogram hash or a 8-byte block mean value hash as 24 or 16 hex digits.
The resulting image reconstruction will be <width> x <height> pixel
(defaulting to 64x64) and is written to stdout as a PNG file.
`,
}

func init() {
	cmdReconstruct.Flag.IntVar(&recunstructWidth, "width", 64,
		"make recunstructed image `w` pixel wide")
	cmdReconstruct.Flag.IntVar(&recunstructHeight, "height", 64,
		"make recunstructed image `h` pixel heigh")
}

var (
	recunstructWidth  int
	recunstructHeight int
)

func runReconstruct(cmd *Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Missing arguments to fingerprint")
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(1)
	}
	hash := args[0]

	var img image.Image
	if len(hash) == 16 {
		bmv, err := fingerprint.BMVHashFromString(hash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Sorry, %q is not a BMV Hash value\n", hash)
			os.Exit(1)
		}
		img = bmv.Image(recunstructWidth, recunstructHeight)
	} else if len(hash) == 24 {
		ch, err := fingerprint.ColorHistFromString(hash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Sorry, %q is not a Color Histogram Hash value\n", hash)
			os.Exit(1)
		}
		img = ch.Image(recunstructWidth, recunstructHeight)
	} else {
		fmt.Fprintf(os.Stderr, "Uncrecognised hash (neither 16 nor 24 hex digit long).")
		os.Exit(1)
	}
	err := png.Encode(os.Stdout, img)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Problems writing image: %s\n", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}
