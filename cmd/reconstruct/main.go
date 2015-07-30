// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// reconstruct reconstructs images from their fingerprints
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"

	"github.com/vdobler/ht/fingerprint"
)

func usage() {
	fmt.Fprintf(os.Stderr, `reconstruct images from fingerprints

Usage:
    fingerprint <hash> [ <width> <height> ]

The given <hash> may be a 12-byte color histogram hash or a 8-byte block mean
value hash as 24 or 16 hex digits. The resulting image reconstruction will
be <width> x <height> pixel (defaulting to 64x64) and is written to stdout
as a PNG file.
`)
	os.Exit(1)
}

func main() {
	width, height, hash := 64, 64, ""
	var err error
	switch len(os.Args) {
	case 2:
		hash = os.Args[1]
	case 4:
		hash = os.Args[1]
		width, err = strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			usage()
		}
		height, err = strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			usage()
		}
	default:
		usage()
	}
	reconstruct(hash, width, height)
	os.Exit(0)
}

func reconstruct(hash string, width, height int) {
	var img image.Image
	if len(hash) == 16 {
		bmv, err := fingerprint.BMVHashFromString(hash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Sorry, %q is not a BMV Hash value\n", hash)
			os.Exit(1)
		}
		img = bmv.Image(width, height)
	} else if len(hash) == 24 {
		ch, err := fingerprint.ColorHistFromString(hash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Sorry, %q is not a Color Histogram Hash value\n", hash)
			os.Exit(1)
		}
		img = ch.Image(width, height)
	}
	err := png.Encode(os.Stdout, img)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Problems writing image: %s\n", err.Error())
		os.Exit(1)
	}
}
