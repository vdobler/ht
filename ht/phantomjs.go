// Copyright 2015 Volker Dobler.  All rights reserved.
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

	"github.com/hawx/rgoybiv/distance"
)

func init() {
	RegisterCheck(&Screenshot{})
}

const debugScreenshot = false

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

	// Script is JavaScript code to be evaluated after page loading but
	// before rendering the page. You can use it e.g. to hide elements
	// which are non-deterministic using code like:
	//    $("#keyvisual > div.slides").css("visibility", "hidden");
	Script string `json:",omitempty"`

	// Path is the full file path to the PhantomJS executable. It defaults to
	// "phantomjs" which works if phantomjs is on your PATH.
	Path string `json:",omitempty"`

	width, height, top, left, scale int // parsed geometry

	path string // path of PhantomJS executable

	golden image.Image // Expected screenshot.
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
		s.width, s.height = 1280, 720
		s.top, s.left = 0, 0
		s.scale = 100
	} else {
		err := s.parseGeometry()
		if err != nil {
			return err
		}
	}

	// Prepare golden record.
	var err error
	s.golden, err = readImage(s.Expected)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			return err // File exists but not an image
		}
	}

	return nil
}

func (s *Screenshot) parseGeometry() error {
	s.width, s.height, s.top, s.left, s.scale = 0, 0, 0, 0, 1.0
	var err error

	// "* scale" is optional
	p := strings.Split(s.Geometry, "*")
	if len(p) > 2 {
		return fmt.Errorf("malformed geometry %q", s.Geometry)
	}
	if len(p) == 2 {
		s.scale, err = strconv.Atoi(strings.Trim(p[1], " \t%"))
		if err != nil {
			return fmt.Errorf("malformed geometry %q: %s", s.Geometry, err)
		}
	}

	// "+ top + left" are optional
	p = strings.Split(p[0], "+")
	if len(p) > 3 {
		return fmt.Errorf("malformed geometry %q", s.Geometry)
	}
	if len(p) == 3 {
		s.top, err = strconv.Atoi(strings.TrimSpace(p[2]))
		if err != nil {
			return fmt.Errorf("malformed geometry %q: %s", s.Geometry, err)
		}
	}
	if len(p) >= 2 {
		s.left, err = strconv.Atoi(strings.TrimSpace(p[1]))
		if err != nil {
			return fmt.Errorf("malformed geometry %q: %s", s.Geometry, err)
		}
	}

	// "width x height" is mandatory
	p = strings.Split(p[0], "x")
	if len(p) != 2 {
		return fmt.Errorf("malformed geometry %q", s.Geometry)
	}
	s.width, err = strconv.Atoi(strings.TrimSpace(p[0]))
	if err != nil {
		return fmt.Errorf("malformed geometry %q: %s", s.Geometry, err)
	}
	s.height, err = strconv.Atoi(strings.TrimSpace(p[1]))
	if err != nil {
		return fmt.Errorf("malformed geometry %q: %s", s.Geometry, err)
	}

	return nil
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
		s.width, s.height,
		s.top, s.left, s.width, s.height,
		float64(s.scale)/100,
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

	delta, low, high := imageDelta(s.golden, actual)
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

func imageDelta(a, b image.Image) (image.Image, int, int) {
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
	low := color.Gray{85}
	high := color.Gray{255}

	lowN, highN := 0, 0

	ax0, ay0 := a.Bounds().Min.X, a.Bounds().Min.Y
	bx0, by0 := b.Bounds().Min.X, b.Bounds().Min.Y
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			delta := distance.Distance(a.At(ax0+x, ay0+y), b.At(bx0+x, by0+y))
			if delta < 1.5 {
				diff.SetGray(x, y, none)
			} else if delta < 10 {
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
