// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// latency.go contains checks against response time latency.

package ht

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func init() {
	RegisterCheck(&Screenshot{})
}

const debugScreenshot = true // false

// ----------------------------------------------------------------------------
// Screenshot

// Screenshot checks actual screenshots rendered via the headless browser
// PhantomJS against a golden record of the expected screenshot.
type Screenshot struct {
	// Geometry of the screenshot in the form
	//     <width> x <height> [ + <left> + <top> [ * <zoom> ] ]
	// which generates a screenshot (width x height) pixels located
	// at (left,top) while simulating a browser viewport of
	// again (width x height) at a zoom level of zoom %.
	//
	// It defaults to "1280x720+0+0*100" which simulates a
	// (unscrolled) desktop browser at 100%.
	Geometry string `json:",omitempty"`

	// Expected is the file path of the 'golden record' image to test
	// the actual screenshot against.
	Expected string `json:",omitempty"`

	// AllowedDifference is the total number of pixels which may
	// differ between the two screenshots while still passing this check.
	AllowedDifference int `json:",omitempty"`

	// IgnoreRegion is a list of regions which are ignored during
	// comparing the actual screenshot to the golden record.
	// The entries are specify rectangles in the form of the Geometry
	// (with ignored zoom factor).
	IgnoreRegion []string `json:",omitempty"`

	// Script is JavaScript code to be evaluated after page loading but
	// before rendering the page. You can use it e.g. to hide elements
	// which are non-deterministic using code like:
	//    $("#keyvisual > div.slides").css("visibility", "hidden");
	Script string `json:",omitempty"`

	// Path is the full file path to the PhantomJS executable. It defaults to
	// "phantomjs" which works if phantomjs is on your PATH.
	Path string `json:",omitempty"`

	geom    geometry          // parsed Geometry
	ignored []image.Rectangle // parsed IgnoreRegion
	path    string            // path of PhantomJS executable
	golden  image.Image       // Expected screenshot.
}

// Prepare implements Check's Prepare method.
func (s *Screenshot) Prepare() error {
	// Prepare PhantomJS executable.
	if s.Path != "" {
		s.path = s.Path
		// TODO: check for existence and executability.
	} else {
		s.path = "phantomjs"
	}

	// Prepare Geoometry.
	if s.Geometry == "" {
		s.Geometry = "1280x720+0+0*100"
	}
	var err error
	s.geom, err = parseGeometry(s.Geometry)
	if err != nil {
		return err
	}

	// Parse IgnoredRegion
	for _, ign := range s.IgnoreRegion {
		geom, err := parseGeometry(ign)
		if err != nil {
			return err
		}
		r := image.Rect(geom.left, geom.top, geom.left+geom.width, geom.top+geom.height)
		s.ignored = append(s.ignored, r)
	}

	// Prepare golden record.
	s.golden, err = readImage(s.Expected)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			return err // File exists but not an image
		}
	}

	return nil
}

type geometry struct {
	width, height int
	left, top     int
	zoom          int // in percent
}

func parseGeometry(s string) (geometry, error) {
	geom := geometry{}
	var err error

	// "* zoom" is optional
	p := strings.Split(s, "*")
	if len(p) > 2 {
		return geom, fmt.Errorf("malformed geometry %q", s)
	}
	if len(p) == 2 {
		geom.zoom, err = strconv.Atoi(strings.Trim(p[1], " \t%"))
		if err != nil {
			return geom, fmt.Errorf("malformed geometry %q: %s", s, err)
		}
	}

	// "+ top + left" are optional
	p = strings.Split(p[0], "+")
	if len(p) > 3 {
		return geom, fmt.Errorf("malformed geometry %q", s)
	}
	if len(p) == 3 {
		geom.top, err = strconv.Atoi(strings.TrimSpace(p[2]))
		if err != nil {
			return geom, fmt.Errorf("malformed geometry %q: %s", s, err)
		}
	}
	if len(p) >= 2 {
		geom.left, err = strconv.Atoi(strings.TrimSpace(p[1]))
		if err != nil {
			return geom, fmt.Errorf("malformed geometry %q: %s", s, err)
		}
	}

	// "width x height" is mandatory
	p = strings.Split(p[0], "x")
	if len(p) != 2 {
		return geom, fmt.Errorf("malformed geometry %q", s)
	}
	geom.width, err = strconv.Atoi(strings.TrimSpace(p[0]))
	if err != nil {
		return geom, fmt.Errorf("malformed geometry %q: %s", s, err)
	}
	geom.height, err = strconv.Atoi(strings.TrimSpace(p[1]))
	if err != nil {
		return geom, fmt.Errorf("malformed geometry %q: %s", s, err)
	}

	return geom, nil
}

var screenshotScript = `
// Screenshot during test %q.
setTimeout(function() {
    console.log('Timeout');
    phantom.exit();
}, %d);
var page = require('webpage').create();
var theContent = %q;
var theURL = %q;
page.viewportSize = { width: %d, height: %d };
page.clipRect = { top: %d, left: %d, width: %d, height: %d };
page.zoomFactor = %.4f;
page.onLoadFinished = function(status){
    if(status === 'success') {
        page.evaluate(function() {
             %s
        });
        page.render(%q);
    } else {
        console.log('Failure');
    }
    phantom.exit();
};
page.setContent(theContent, theURL);
`

func (s *Screenshot) writeScript(file *os.File, t *Test, out string) error {
	defer file.Close() // eat error, sorry
	_, err := fmt.Fprintf(file, screenshotScript,
		t.Name, 15000,
		t.Response.BodyStr, t.Request.Request.URL.String(),
		s.geom.width, s.geom.height,
		s.geom.top, s.geom.left, s.geom.width, s.geom.height,
		float64(s.geom.zoom)/100,
		s.Script,
		out,
	)
	return err
}

// Execute implements Check's Execute method.
func (s *Screenshot) Execute(t *Test) error {
	file, err := ioutil.TempFile("", "screenshot-")
	if err != nil {
		return err // TODO: wrap to mark as bogus ?
	}
	script := file.Name()
	if !debugScreenshot {
		defer os.Remove(script)
	}

	out := s.Expected + "_" + time.Now().Format("2006-01-02_15h04m05s") + ".png"
	err = s.writeScript(file, t, out)
	if err != nil {
		return err // TODO: wrap to mark as bogus ?
	}
	if debugScreenshot {
		fmt.Println("Created PhantomJS script:", script)
	}

	cmd := exec.Command(s.path, script)
	output, err := cmd.CombinedOutput()
	if debugScreenshot {
		fmt.Println("PhantomJS output:", output)
	}
	if err != nil {
		return err
	}

	if s.golden == nil {
		return fmt.Errorf("Golden record %s not found; actual screenshot saved to %s",
			s.Expected, out)
	}

	actual, err := readImage(out)
	if err != nil {
		return err
	}

	delta, low, high := imageDelta(s.golden, actual, s.ignored)
	if debugScreenshot {
		deltaFile, err := os.Create(s.Expected + "_delta.png")
		if err != nil {
			return err
		}
		defer deltaFile.Close()
		png.Encode(deltaFile, delta)
	}
	totalDiff := low + high
	if totalDiff > s.AllowedDifference {
		return fmt.Errorf("Found %d different pixels", totalDiff)
	}
	return nil
}

func readImage(filename string) (image.Image, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Decode the image.
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// imageDelta computes the difference of a and b while ignoring the given
// rectangles. In the result black means identical, dark gray means almost
// equal, light gray means really different and white means ignored.
func imageDelta(a, b image.Image, ignore []image.Rectangle) (image.Image, int, int) {
	width := a.Bounds().Dx()
	if bw := b.Bounds().Dx(); bw < width {
		width = bw
	}
	height := a.Bounds().Dy()
	if bh := b.Bounds().Dy(); bh < height {
		height = bh
	}

	diff := image.NewGray(image.Rect(0, 0, width, height))

	none := color.Gray{0}
	low := color.Gray{80}
	high := color.Gray{160}
	skip := color.Gray{255}
	lowN, highN := 0, 0

	ax0, ay0 := a.Bounds().Min.X, a.Bounds().Min.Y
	bx0, by0 := b.Bounds().Min.X, b.Bounds().Min.Y
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			ign := false
			for _, r := range ignore {
				if x < r.Min.X || x >= r.Max.X || y < r.Min.Y || y >= r.Max.Y {
					continue
				}
				diff.SetGray(x, y, skip)
				ign = true
				break
			}
			if ign {
				continue
			}
			delta := colorDistance(a.At(ax0+x, ay0+y), b.At(bx0+x, by0+y))
			if delta < 15 {
				diff.SetGray(x, y, none)
			} else if delta < 77 {
				diff.SetGray(x, y, low)
				lowN++
			} else {
				diff.SetGray(x, y, high)
				highN++
			}
		}
	}

	return diff, lowN, highN
}

// colorDistance computes the L¹ norm of a-b in RGB space.
// It ranges from 0 (equal) to 765 (white vs black).
func colorDistance(a, b color.Color) int {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	d := func(x, y uint32) int {
		if x > y {
			return int((x - y) >> 8)
		}
		return int((y - x) >> 8)
	}
	delta := d(ar, br) + d(ag, bg) + d(ab, bb)
	return delta
}
