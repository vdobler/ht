// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// image.go contains checks against image data.

package ht

import (
	"errors"
	"fmt"
	"strings"

	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/vdobler/ht/fingerprint"
)

func init() {
	RegisterCheck(Image{})
}

// ----------------------------------------------------------------------------
// Image

// Image checks image format, size and fingerprint. As usual a zero value of
// a field skipps the check of that property.
// Image fingerprinting is done via github.com/vdobler/ht/fingerprint.
// Only one of BMV or ColorHist should be used as there is just one threshold.
type Image struct {
	// Format is the format of the image as registered in package image.
	Format string `json:",omitempty"`

	// If > 0 check width or height of image.
	Width, Height int `json:",omitempty"`

	// Fingerprint is either the 16 hex digit long Block Mean Value hash or
	// the 24 hex digit long Color Histogram hash of the image.
	Fingerprint string `json:",omitempty"`

	// Threshold is the limit up to which the received image may differ
	// from the given BMV or ColorHist fingerprint.
	Threshold float64 `json:",omitempty"`
}

// Execute implements Checks Execute method.
func (c Image) Execute(t *Test) error {
	img, format, err := image.Decode(t.Response.Body())
	if err != nil {
		return CantCheck{err}
	}

	failures := []string{}
	if c.Format != "" && format != c.Format {
		failures = append(failures,
			fmt.Sprintf("got %s image, want %s", format, c.Format))
	}

	bounds := img.Bounds()
	if c.Width > 0 && c.Width != bounds.Dx() {
		failures = append(failures,
			fmt.Sprintf("got %d px wide image, want %d", bounds.Dx(), c.Width))

	}
	if c.Height > 0 && c.Height != bounds.Dy() {
		failures = append(failures,
			fmt.Sprintf("got %d px heigh image, want %d", bounds.Dy(), c.Height))

	}

	if len(c.Fingerprint) == 16 {
		targetBMV, _ := fingerprint.BMVHashFromString(c.Fingerprint)
		imgBMV := fingerprint.NewBMVHash(img)
		if d := fingerprint.BMVDelta(targetBMV, imgBMV); d > c.Threshold {
			failures = append(failures, fmt.Sprintf("got BMV of %s, want %s (delta=%.4f)",
				imgBMV.String(), targetBMV.String(), d))
		}

	} else if len(c.Fingerprint) == 24 {
		targetCH, _ := fingerprint.ColorHistFromString(c.Fingerprint)
		imgCH := fingerprint.NewColorHist(img)
		if d := fingerprint.ColorHistDelta(targetCH, imgCH); d > c.Threshold {
			failures = append(failures,
				fmt.Sprintf("got color histogram of %s, want %s (delta=%.4f)",
					imgCH.String(), targetCH.String(), d))
		}
	}

	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}

	return nil
}

// Prepare implements Checks Prepare method.
func (i Image) Prepare() error {
	switch len(i.Fingerprint) {
	case 0:
		return nil
	case 16:
		_, err := fingerprint.BMVHashFromString(i.Fingerprint)
		if err != nil {
			return MalformedCheck{err}
		}
	case 24:
		_, err := fingerprint.ColorHistFromString(i.Fingerprint)
		if err != nil {
			return MalformedCheck{err}
		}
	default:
		return MalformedCheck{
			fmt.Errorf("fingerprint has illegal length %d", len(i.Fingerprint)),
		}
	}
	return nil
}
