// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// fingerprint computes image fingerprints.
package main

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/vdobler/ht/fingerprint"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Printf("Usage: %s filename....\n", os.Args[0])
		os.Exit(1)
	}

	fmt.Println("#       Filename:  BMV-Hash         ColorHist-Hash")
	for _, path := range os.Args[1:] {
		fmt.Printf("%16s:  %s\n", path, compute(path))
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
