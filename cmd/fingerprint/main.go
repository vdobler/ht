// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// fingerprint computes image fingerprints.
package main

import (
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"strconv"

	"github.com/vdobler/ht/fingerprint"
)

func usage() {
	fmt.Fprintf(os.Stderr, `fingerprint: calculate image fingerprints

Usage:
    fingerprint <filename>...
    fingerprint -reconstruct <hash> [ <width> <height> ]
`)
	os.Exit(1)
}

func main() {
	if len(os.Args) == 1 {
		usage()
	}

	if os.Args[1] == "-reconstruct" {
		width, height, hash := 64, 64, ""
		var err error
		switch len(os.Args) {
		case 3:
			hash = os.Args[2]
		case 5:
			hash = os.Args[2]
			width, err = strconv.Atoi(os.Args[3])
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				usage()
			}
			height, err = strconv.Atoi(os.Args[4])
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

	fmt.Println("#       Filename:  BMV-Hash         ColorHist-Hash")
	for _, path := range os.Args[1:] {
		fmt.Fprintf(os.Stderr, "%16s:  %s\n", path, compute(path))
	}
}

func compute(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return err.Error()
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return err.Error()
	}

	ch := fingerprint.NewColorHist(img)
	bmv := fingerprint.NewBMVHash(img)

	return bmv.String() + " " + ch.String()
}

func reconstruct(hash string, width, height int) {
	fmt.Fprintf(os.Stderr, "%d %d\n", width, height)
	bmv, err := fingerprint.BMVHashFromString(hash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Sorry, %q is not a BMV Hash value\n", hash)
		os.Exit(1)
	}
	img := bmv.Image(width, height)
	err = jpeg.Encode(os.Stdout, img, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Problems writing image: %s\n", err.Error())
		os.Exit(1)
	}
}
