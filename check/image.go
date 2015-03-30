// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// image.go contains checks against image data.

package check

import (
	"fmt"

	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/vdobler/ht/fingerprint"
	"github.com/vdobler/ht/response"
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
//
// Note that you have to register the apropriate image decoder functions
// with package image, e.g. by
//     import _ "image/png"
// if you want to check PNG images.
type Image struct {
	// Format is the format of the image as registered in package image.
	Format string

	// If > 0 check width or height of image.
	Width, Height int

	// BMV is the 16 hex digit long Block Mean Value hash of the image.
	BMV string

	// ColorHist is the 24 hex digit long Color Histogram hash of
	// the image.
	ColorHist string

	// Threshold is the limit up to which the received image may differ
	// from the given BMV or ColorHist fingerprint.
	Threshold float64
}

// Okay implements the Check interface.
func (c Image) Okay(response *response.Response) error {
	img, format, err := image.Decode(response.BodyReader())
	if err != nil {
		fmt.Printf("Image.Okay resp.BodyReader=%#v\n", response.BodyReader())
		return CantCheck{err}
	}
	// TODO: Do not abort on first failure.
	if c.Format != "" && format != c.Format {
		return fmt.Errorf("Got %s image, want %s", format, c.Format)
	}

	bounds := img.Bounds()
	if c.Width > 0 && c.Width != bounds.Dx() {
		return fmt.Errorf("Got %d px wide image, want %d",
			bounds.Dx(), c.Width)

	}
	if c.Height > 0 && c.Height != bounds.Dy() {
		return fmt.Errorf("Got %d px heigh image, want %d",
			bounds.Dy(), c.Height)

	}

	if c.BMV != "" {
		targetBMV, err := fingerprint.BMVHashFromString(c.BMV)
		if err != nil {
			return CantCheck{fmt.Errorf("bad BMV hash: %s", err)}
		}
		imgBMV := fingerprint.NewBMVHash(img)
		if d := fingerprint.BMVDelta(targetBMV, imgBMV); d > c.Threshold {
			return fmt.Errorf("Got BMV of %s, want %s (delta=%.4f)",
				imgBMV.String(), targetBMV.String(), d)
		}

	}
	if c.ColorHist != "" {
		targetCH, err := fingerprint.ColorHistFromString(c.ColorHist)
		if err != nil {
			return CantCheck{fmt.Errorf("bad ColorHist hash: %s", err)}
		}
		imgCH := fingerprint.NewColorHist(img)
		if d := fingerprint.ColorHistDelta(targetCH, imgCH); d > c.Threshold {
			return fmt.Errorf("Got ColorHist of %s, want %s (delta=%.4f)",
				imgCH.String(), targetCH.String(), d)
		}
	}

	return nil
}
