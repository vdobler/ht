// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// phantomjs.go contains checks through PhantomJS (phantomjs.org)

package ht

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/internal/tempfile"
)

func init() {
	RegisterCheck(&Screenshot{})
	RegisterCheck(&RenderedHTML{})
	RegisterCheck(&RenderingTime{})
}

// PhantomJSExecutable is command to run PhantomJS. Use an absolute path if
// phantomjs is not on your PATH or you whish to use a special version.
var PhantomJSExecutable = "phantomjs"

// DefaultGeometry is the default screen size, viewport and zoom used in
// browser based test if no geometry is explicitly set.
// Its value represents a unscrolled (+0+=), desktop browser (1280x720)
// at 100% zoom.
var DefaultGeometry = "1280x720+0+0*100"

// TODO: PhantomJS is something external which might not be available
// Check via sync.Once and report PhantomJS based tests as bogus if not
// available.

const debugScreenshot = false
const debugRenderedHTML = false
const debugRenderingTime = false

// ----------------------------------------------------------------------------
// Browser

// Browser collects information needed for the checks Screenshot, RenderedHTML
// and RenderingTime which use PhantomJS as a headless browser.
type Browser struct {
	// Geometry of the screenshot in the form
	//     <width> x <height> [ + <left> + <top> [ * <zoom> ] ]
	// which generates a screenshot (width x height) pixels located
	// at (left,top) while simulating a browser viewport of
	// again (width x height) at a zoom level of zoom %.
	//
	// It defaults to DefaultGeometry if unset.
	Geometry string `json:",omitempty"`

	// WaitUntilVisible selects (via CSS selectors) those elements in the
	// DOM which must be visible before rendering the screenshot.
	WaitUntilVisible []string `json:",omitempty"`

	// WaitUntilInvisible selects (via CSS selectors) those elements in the
	// DOM which must be invisible before rendering the screenshot.
	WaitUntilInvisible []string `json:",omitempty"`

	// Script is JavaScript code to be evaluated after page loading but
	// before rendering the page. You can use it e.g. to hide elements
	// which are non-deterministic using code like:
	//    $("#keyvisual > div.slides").css("visibility", "hidden");
	Script string `json:",omitempty"`

	// Timeout is the maximum duration to wait for the headless browser
	// to prepare the page. Defaults to 5 seconds if unset.
	Timeout time.Duration

	geom geometry // parsed Geometry
}

// prepare Geometry, geoam and Timeout
func (b *Browser) prepare() error {
	// Prepare Geoometry.
	if b.Geometry == "" {
		b.Geometry = DefaultGeometry
	}
	var err error
	b.geom, err = newGeometry(b.Geometry)
	if err != nil {
		return err
	}
	if b.geom.Zoom == 0 {
		b.geom.Zoom = 100
	}

	if b.Timeout == 0 {
		b.Timeout = 5 * time.Second
	}

	return nil
}

type phantomjsData struct {
	Test       *Test
	Timeout    int
	Geom       geometry
	Script     string
	Cookies    []cookiejar.Entry
	Vis, Invis []string

	ReadyCode, TimeoutCode string
}

var phantomjsTemplate = `
// Screenshot during test {{printf "%q" .Test.Name}}

/**
 * Wait until the test condition is true or a timeout occurs. Useful for
 * waiting on a server response or for a ui change (fadeIn, etc.) to occur.
 *
 *  - testFx javascript condition that evaluates to a boolean,
 *    passed in as a callback
 *  - onReady what to do when testFx condition is fulfilled,
 *    passed in as a callback
 *  - onTimeout what to do on timeout
 */
"use strict";
function waitFor(testFx, onReady, onTimeout) {
    var maxtimeOutMillis = {{.Timeout}}, 
        start = new Date().getTime(),
        condition = false,
        interval = setInterval(function() {
            if ( (new Date().getTime() - start < maxtimeOutMillis) && !condition ) {
                // If not time-out yet and condition not yet fulfilled
                condition = testFx();
            } else {
                if(!condition) {
                    // If condition still not fulfilled (timeout but
		    // condition is 'false')
                    onTimeout();
                } else {
                    // Condition fulfilled 
                    // Do what it's supposed to do once the condition is fulfilled
                    onReady(); 
                    clearInterval(interval); //< Stop this interval
                }
            }
        }, 50); // repeat check every 50ms
};

setTimeout(function(){console.log('FAIL timeout'); phantom.exit(1);}, {{.Timeout}}+1000);
var page = require('webpage').create();
var theURL = {{printf "%q" .Test.Request.Request.URL}};;
var theContent = {{printf "%q" .Test.Response.BodyStr}};

page.viewportSize = { width: {{.Geom.Width}}, height: {{.Geom.Height}} };
page.clipRect = { top: {{.Geom.Top}}, left: {{.Geom.Left}}, width: {{.Geom.Width}}, height: {{.Geom.Height}} };
page.zoomFactor = {{printf "%.4f" .Geom.FloatZoom}};
var system = require('system');
page.onConsoleMessage = function(msg) { system.stdout.writeLine('console: ' + msg); };

{{range .Cookies}}
phantom.addCookie({
  'name'    : {{printf "%q" .Name}},
  'value'   : {{printf "%q" .Value}},
  'domain'  : {{printf "%q" .Domain}},
  'path'    : {{printf "%q" .Path}},
  'httponly': {{.HttpOnly}},
  'secure'  : {{.Secure}},
  'expires' : "{{.Expires.Format "Mon, 02 Jan 2006 15:04:05 MST"}}"
});
{{end}}

{{with .Test.Request}}
{{if .BasicAuthUser}}
page.customHeaders={'Authorization': 'Basic '+btoa({{printf "%q" .BasicAuthUser}}+":"+{{printf "%q" .BasicAuthPass}})};
{{end}}{{end}}

// What to do once the content is set and the page loaded:
page.onLoadFinished = function(status){
    if(status === 'success') {

        waitFor(
            function() { // testFx
                return page.evaluate(function(){
		      function isVisible(selector) {
		          var e = document.querySelector(selector);
		          if ( e === null ) { return false; }
		          return e.offsetHeight > 0;
		      };
                      return true 
{{range .Vis}}          && (isVisible('{{.}}')) {{end}}
{{range .Invis}}        && !(isVisible('{{.}}')) {{end}} ;
                });
            }
            ,
            function() {  // onReady
                {{.Script}} // optional custom code
                {{.ReadyCode}}
            }
            ,
            function() { // onTimeout
                {{.TimeoutCode}}
            }
        );

    } else {
        console.log('FAIL loading');
        phantom.exit(1);
    }
};

page.setContent(theContent, theURL);
`

var phantomjsTmpl = template.Must(template.New("phantomscript").Parse(phantomjsTemplate))

// write a PhantomJS script to file which renders the response in t, waits
// for (in)visible elements as defined in b, and executes ready or timeout
// accordingly.
// So ready and timeout should contain the actual PhantomJS commands to
// execute and must terminate PhantomJS.
func (b Browser) writeScript(file io.WriteCloser, t *Test, ready, timeout string) error {
	data := phantomjsData{
		Test:        t,
		Timeout:     int(b.Timeout.Nanoseconds() / 1e6),
		Geom:        b.geom,
		Script:      b.Script,
		Vis:         b.WaitUntilVisible,
		Invis:       b.WaitUntilInvisible,
		ReadyCode:   ready,
		TimeoutCode: timeout,
	}
	for _, c := range t.allCookies() {
		// Something is bogus here. If the domain is unset or does not
		// start with a dot than PhantomJS will ignore it (addCookie
		// returns flase). So it seems as if it is impossible in
		// PhantomJS to distinguish a host-only form a domain cookie?
		c.Domain = "." + c.Domain
		data.Cookies = append(data.Cookies, c)
	}
	err := phantomjsTmpl.Execute(file, data)
	if err != nil {
		file.Close() // try at least
		return fmt.Errorf("cannot write temporary script: %s", err)
	}

	err = file.Close()
	if err != nil {
		return fmt.Errorf("cannot write temporary script: %s", err)
	}

	return nil
}

// ----------------------------------------------------------------------------
// Screenshot

// Screenshot checks actual screenshots rendered via the headless browser
// PhantomJS against a golden record of the expected screenshot.
//
// Note that PhantomJS will make additional request to fetch all linked
// resources in the HTML page. If the original request has BasicAuthUser
// (and BasicAuthPass) set this credentials will be sent to all linked
// resources of the page. Depending on where these resources are located
// this might be a security issue.
type Screenshot struct {
	Browser

	// Expected is the file path of the 'golden record' image to test
	// the actual screenshot against.
	Expected string `json:",omitempty"`

	// Actual is the name of the file the actual rendered screenshot is
	// saved to.
	// An empty value disables storing the generated screenshot.
	Actual string `json:",omitempty"`

	// AllowedDifference is the total number of pixels which may
	// differ between the two screenshots while still passing this check.
	AllowedDifference int `json:",omitempty"`

	// IgnoreRegion is a list of regions which are ignored during
	// comparing the actual screenshot to the golden record.
	// The entries are specify rectangles in the form of the Geometry
	// (with ignored zoom factor).
	IgnoreRegion []string `json:",omitempty"`

	ignored []image.Rectangle // parsed IgnoreRegion
	golden  image.Image       // Expected screenshot.
}

// Prepare implements Check's Prepare method.
func (s *Screenshot) Prepare() error {
	err := s.Browser.prepare()
	if err != nil {
		return err
	}

	// Parse IgnoredRegion
	for _, ign := range s.IgnoreRegion {
		geom, err := newGeometry(ign)
		if err != nil {
			return err
		}
		r := image.Rect(geom.Left, geom.Top, geom.Left+geom.Width, geom.Top+geom.Height)
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
	Width, Height int
	Left, Top     int
	Zoom          int // in percent
}

// FloatZoom returns the zoom as a float between 0 and 1.
func (g geometry) FloatZoom() float64 {
	return float64(g.Zoom) / 100
}

// "640x480+16+32*125%"  -->  geometry
func newGeometry(s string) (geometry, error) {
	geom := geometry{}
	var err error

	// "* zoom" is optional
	p := strings.Split(s, "*")
	if len(p) > 2 {
		return geom, fmt.Errorf("malformed geometry %q", s)
	}
	if len(p) == 2 {
		geom.Zoom, err = strconv.Atoi(strings.Trim(p[1], " \t%"))
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
		geom.Top, err = strconv.Atoi(strings.TrimSpace(p[2]))
		if err != nil {
			return geom, fmt.Errorf("malformed geometry %q: %s", s, err)
		}
	}
	if len(p) >= 2 {
		geom.Left, err = strconv.Atoi(strings.TrimSpace(p[1]))
		if err != nil {
			return geom, fmt.Errorf("malformed geometry %q: %s", s, err)
		}
	}

	// "width x height" is mandatory
	p = strings.Split(p[0], "x")
	if len(p) != 2 {
		return geom, fmt.Errorf("malformed geometry %q", s)
	}
	geom.Width, err = strconv.Atoi(strings.TrimSpace(p[0]))
	if err != nil {
		return geom, fmt.Errorf("malformed geometry %q: %s", s, err)
	}
	geom.Height, err = strconv.Atoi(strings.TrimSpace(p[1]))
	if err != nil {
		return geom, fmt.Errorf("malformed geometry %q: %s", s, err)
	}

	return geom, nil
}

// Execute implements Check's Execute method.
func (s *Screenshot) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}

	file, err := tempfile.TempFile("", "screenshot-", ".js")
	if err != nil {
		return fmt.Errorf("cannot write temporary script: %s", err)
	}
	script := file.Name()
	if !debugScreenshot {
		defer os.Remove(script)
	}

	actual := s.Actual
	if actual == "" {
		file, err := tempfile.TempFile("", "actual-ss-", ".png")
		if err != nil {
			return fmt.Errorf("cannot write actual screenshot: %s", err)
		}
		actual = file.Name()
		file.Close()
		if s.golden != nil {
			defer os.Remove(actual)
		}
	}

	readyCode := fmt.Sprintf("page.render(%q); "+
		"console.log('PASS'); "+
		"phantom.exit(0);",
		actual)
	// Generate screenshot even when timeout to facilitate debugging.
	timeoutCode := fmt.Sprintf("page.render(%q); "+
		"console.log('FAIL timeout waiting'); "+
		"phantom.exit(1);",
		actual)
	err = s.Browser.writeScript(file, t, readyCode, timeoutCode)
	if err != nil {
		return err
	}
	if debugScreenshot {
		fmt.Println("Created PhantomJS script:", script)
	}

	cmd := exec.Command(PhantomJSExecutable, script)
	output, err := cmd.CombinedOutput()
	if debugScreenshot {
		fmt.Println("PhantomJS output:", string(output))
	}
	if err != nil {
		return err
	}

	if s.golden == nil {
		return fmt.Errorf("Golden record %s not found; actual screenshot saved to %s",
			s.Expected, actual)
	}

	screenshot, err := readImage(actual)
	if err != nil {
		return err
	}

	delta, low, high := imageDelta(s.golden, screenshot, s.ignored)
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

func (t *Test) allCookies() []cookiejar.Entry {
	if t.Jar == nil {
		return nil
	}
	cookies := []cookiejar.Entry{}
	for _, tld := range t.Jar.ETLDsPlus1(nil) {
		cookies = t.Jar.Entries(tld, cookies)
	}

	return cookies
}

// ----------------------------------------------------------------------------
// RenderedHTML

// RenderedHTML applies checks to the HTML after processing through the
// headless browser PhantomJS. This processing will load external resources
// and evaluate the JavaScript. The checks are run against this 'rendered'
// HTML code.
type RenderedHTML struct {
	Browser

	// Checks to perform on the rendered HTML.
	// Sensible checks are those operating on the response body.
	Checks CheckList

	// KeepAs is the file name to store the rendered HTML to.
	// Useful for debugging purpose.
	KeepAs string `json:",omitempty"`
}

// Prepare implements Check's Prepare method.
func (r *RenderedHTML) Prepare() error {
	err := r.Browser.prepare()
	if err != nil {
		return err
	}

	if len(r.Checks) == 0 {
		return fmt.Errorf("RenderedHTML without checks is a useless noop")
	}

	// Prepare each sub-check.
	errs := ErrorList{}
	for i, check := range r.Checks {
		err := check.Prepare()
		if err != nil {
			errs = append(errs, fmt.Errorf("%d. %s: %s", i, NameOf(check), err))
		}
	}

	if len(errs) != 0 {
		return errs
	}
	return nil
}

// Execute implements Check's Execute method.
func (r *RenderedHTML) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}

	content, err := r.content(t)
	if err != nil {
		return err
	}

	// rt is a fake test which contains the PhantomJS rendered body.
	rt := &Test{
		Name: t.Name,
		Request: Request{
			URL: t.Request.URL,
		},
		Response: Response{
			Response:     t.Response.Response,
			Duration:     t.Response.Duration,
			BodyStr:      content,
			Redirections: t.Response.Redirections,
		},
	}

	// Execute all sub-checks, collecting the errors.
	errs := ErrorList{}
	for i, check := range r.Checks {
		err := check.Execute(rt)
		if err != nil {
			errs = append(errs, fmt.Errorf("%d. %s: %s",
				i, NameOf(check), err))
		}
	}

	// Optionally save the rendered HTML.
	if r.KeepAs != "" {
		err := ioutil.WriteFile(r.KeepAs, []byte(content), 0666)
		if err != nil {
			errs = append(errs,
				fmt.Errorf("failed to save rendered HTML: %s",
					err))
		}
	}

	if len(errs) != 0 {
		return errs
	}
	return nil
}

// content returns the page content after rendering (and evaluating JavaScript)
// via PhantomJS.
func (r *RenderedHTML) content(t *Test) (string, error) {
	file, err := tempfile.TempFile("", "renderedhtml-", ".js")
	if err != nil {
		return "", fmt.Errorf("cannot write temporary script: %s", err)
	}
	script := file.Name()
	if !debugRenderedHTML {
		defer os.Remove(script)
	}

	readyCode := "console.log('PASS'); " +
		"console.log(''+page.content);" +
		"phantom.exit(0);"
	timeoutCode := "console.log('FAIL timeout waiting'); " +
		"console.log(''+page.content);" +
		"phantom.exit(1);"
	err = r.Browser.writeScript(file, t, readyCode, timeoutCode)
	if err != nil {
		return "", err
	}
	if debugRenderedHTML {
		fmt.Println("Created PhantomJS script:", script)
	}

	cmd := exec.Command(PhantomJSExecutable, script)
	output, err := cmd.CombinedOutput()
	if debugScreenshot {
		fmt.Println("PhantomJS output:", string(output))
	}
	if err != nil {
		return "", err
	}
	if bytes.HasPrefix(output, []byte("PASS\n")) {
		return string(output[5:]), nil
	}

	if i := bytes.Index(output, []byte("\n")); i != -1 {
		output = output[:i]
	}
	return "", fmt.Errorf("%s", output)

}

// ----------------------------------------------------------------------------
// RenderingTime

// RenderingTime limits the maximal allowed time to render a whole HTML page.
//
// The "rendering time" is how long it takes PhantomJS to load all referenced
// assets and render the page. For obvious reason this cannot be determined
// with absolute accuracy.
type RenderingTime struct {
	Browser

	Max time.Duration
}

// Prepare implements Check's Prepare method.
func (d *RenderingTime) Prepare() error {
	return d.Browser.prepare()
}

var phantomjsInvocationOverhead time.Duration
var phantomjsOnce sync.Once

// Execute implements Check's Execute method.
func (d *RenderingTime) Execute(t *Test) error {
	if t.Response.BodyErr != nil {
		return ErrBadBody
	}

	file, err := tempfile.TempFile("", "renderingtime-", ".js")
	if err != nil {
		return err // TODO: wrap to mark as bogus ?
	}
	script := file.Name()
	if !debugRenderingTime {
		defer os.Remove(script)
	}
	readyCode := "console.log('PASS'); phantom.exit(0);"
	timeoutCode := "console.log('FAIL timeout waiting'); phantom.exit(1);"
	err = d.Browser.writeScript(file, t, readyCode, timeoutCode)
	if err != nil {
		return err
	}
	if debugRenderingTime {
		fmt.Println("Created PhantomJS script:", script)
	}

	phantomjsOnce.Do(calibratePhantomjsOverhead)
	t.debugf("PhantomJS invocation overhead: %s", phantomjsInvocationOverhead)

	start := time.Now()
	cmd := exec.Command(PhantomJSExecutable, script)
	output, err := cmd.CombinedOutput()
	took := time.Since(start)
	if debugRenderingTime {
		fmt.Println("PhantomJS output:", string(output))
	}
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(output, []byte("PASS\n")) {
		return fmt.Errorf("Problems with PhantomJS: %q", string(output))
	}

	took -= phantomjsInvocationOverhead
	if took < 1*time.Millisecond {
		took = 1 * time.Millisecond
	}
	t.infof("Rendering page took %s", took)
	if took <= d.Max {
		return nil
	}

	return fmt.Errorf("rendering time %s", took)
}

func calibratePhantomjsOverhead() {
	tf, err := ioutil.TempFile("", "phantomjs")
	if err != nil {
		return
	}
	name := tf.Name()
	_, err = tf.WriteString("phantom.exit(0);")
	tf.Close() // eat error, sorry
	if err != nil {
		return
	}

	d := make([]time.Duration, 7)
	for i := range d {
		start := time.Now()
		cmd := exec.Command(PhantomJSExecutable, name)
		cmd.CombinedOutput()
		d[i] = time.Since(start)
	}

	// Average all but the first warmup ones.
	warmup := 2
	sum := time.Duration(0)
	for i := warmup; i < len(d); i++ {
		sum += d[i]
	}
	phantomjsInvocationOverhead = sum / time.Duration(len(d)-warmup)

	os.Remove(name)
}

var havePhantomJSOnce sync.Once // fills bool below once
var havePhantomJS = false

// WorkingPhantomJS reports if a suitable PhantomJS is available.
func WorkingPhantomJS() bool {
	havePhantomJSOnce.Do(func() {
		cmd := exec.Command(PhantomJSExecutable, "--version")
		output, err := cmd.CombinedOutput()
		if err == nil && bytes.HasPrefix(output, []byte("2.")) {
			havePhantomJS = true
		}

	})
	return havePhantomJS
}
