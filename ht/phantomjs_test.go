// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/vdobler/ht/cookiejar"
)

var geometryTests = []struct {
	s             string
	width, height int
	left, top     int
	zoom          int
	ok            bool
}{
	{"1x2+3+4*5", 1, 2, 3, 4, 5, true},
	{"1x2+3+4", 1, 2, 3, 4, 0, true},
	{"1x2", 1, 2, 0, 0, 0, true},
	{"1x2*5", 1, 2, 0, 0, 5, true},
	{"hubbabubba", 0, 0, 0, 0, 0, false},
	{"1x2*5*6", 1, 2, 0, 0, 5, false},
	{"1x2+3+4+9*5", 1, 2, 3, 4, 5, false},
	{"axb+c+d*e", 0, 0, 0, 0, 0, false},
}

func TestNewGeometry(t *testing.T) {
	for i, tc := range geometryTests {
		g, err := newGeometry(tc.s)
		if err != nil {
			if tc.ok {
				t.Errorf("%d. %q: Unexpected error %s", i, tc.s, err)
			}
			continue
		}
		if err == nil && !tc.ok {
			t.Errorf("%d. %q: Missing error", i, tc.s)
			continue
		}
		if g.Width != tc.width || g.Height != tc.height {
			t.Errorf("%d. %q: Wrong size, got %dx%d", i, tc.s, g.Width, g.Height)
		}
		if g.Top != tc.top || g.Left != tc.left {
			t.Errorf("%d. %q: Wrong offset, got +%d+%d", i, tc.s, g.Left, g.Top)
		}
		if g.Zoom != tc.zoom {
			t.Errorf("%d. %q: Wrong zoom, go %d", i, tc.s, g.Zoom)
		}

	}
}

func TestDeltaImage(t *testing.T) {
	t.Skip("not ready jet")
	a, err := readImage("A.png")
	if err != nil {
		panic(err)
	}
	b, err := readImage("B.png")
	if err != nil {
		panic(err)
	}

	ignore := []image.Rectangle{
		image.Rect(500, 200, 1000, 400),
	}

	delta, low, high := imageDelta(a, b, ignore)
	deltaFile, err := os.Create("D.png")
	if err != nil {
		panic(err)
	}
	defer deltaFile.Close()
	png.Encode(deltaFile, delta)

	r := delta.Bounds()
	N := r.Dx() * r.Dy()
	fmt.Println(N, low, high)
	fmt.Printf("Low %.2f%%   High %.2f%%\n",
		float64(100*low)/float64(N), float64(100*high)/float64(N))
}

var screenshotHomeHTML = []byte(`<!doctype html>
<html>
  <head><title>Hello</title>
  <style>
    body { background-color: lightyellow; }
  </style>
  <body>
    <h1>Home</h1>
  </body>
</html>
`)

var screenshotGreetHTML = `<!doctype html>
<html>
  <head><title>Hello</title>
  <link rel="stylesheet" href="/screenshot/css">
  <body>
    <p id="p">Hello %s</p>
  </body>
</html>
`

var screenshotCSS = `
body {
  background-color: %s;
}
`

var screenshotWelcomeHTML = `<!doctype html>
<html>
  <head>
    <title>Welcome</title>
    <style>body { background-color: pink; }</style>
    <script src="/screenshot/script.js"></script>
  </head>
  <body onload="ChangeLink();">
    <h1>Welcome</h1>
    <p><script>YouAre()</script></p>
    <p>
      <a id="a1" href="/">Original</a>
    </p>
  </body>
</html>
`

var screenshotJavaScript = `
function YouAre() {
  document.write('You are: %s');
}
function ChangeLink() {
  document.getElementById("a1").innerHTML = "Changed";
  document.getElementById("a1").href = "http://www.example.com";
}
`

func screenshotHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	default:
		http.Redirect(w, r, "/screenshot/home", http.StatusSeeOther)
	case "/screenshot/home":
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write(screenshotHomeHTML)
	case "/screenshot/login":
		user := r.FormValue("user")
		cookie := &http.Cookie{
			Name:  "user",
			Value: user,
			Path:  "/screenshot",
		}
		if user == "" {
			cookie.MaxAge = -1
		}
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "/screenshot/home", http.StatusSeeOther)
	case "/screenshot/greet":
		name := r.FormValue("name")
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, screenshotGreetHTML, name)
	case "/screenshot/css":
		color := "white"
		if cookie, err := r.Cookie("user"); err == nil {
			color = cookie.Value
			if cookie.Value == "rt" {
				color = "olive"
				ban, bap, ok := r.BasicAuth()
				if ok && ban == "rt" && bap == "secret" {
					color = "lime"
				}
			}
		}
		w.Header().Set("Content-Type", "text/css")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, screenshotCSS, color)
	case "/screenshot/welcome":
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, screenshotWelcomeHTML)
	case "/screenshot/script.js":
		user := "Anon"
		if cookie, err := r.Cookie("user"); err == nil {
			user = cookie.Value
		} else {
			time.Sleep(80 * time.Millisecond)
		}
		w.Header().Set("Content-Type", "application/javascript")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, screenshotJavaScript, user)
	}
}

var passingScreenshotTests = []*Test{
	// Plain screenshot of homepage.
	{
		Name:    "Basic Screenshot of Home",
		Request: Request{URL: "/home"},
		Checks: []Check{
			&Screenshot{
				Browser:  Browser{Geometry: "128x64+0+0"},
				Expected: "./testdata/home.png",
				Actual:   "./testdata/home_actual.png",
			},
		},
	},

	// Anonymous user are greeted with white background.
	{
		Name:    "Greet Anonymous (white bg)",
		Request: Request{URL: "/greet"},
		Checks: []Check{
			&Screenshot{
				Browser:  Browser{Geometry: "96x32"},
				Expected: "./testdata/greet-anon.png",
				Actual:   "./testdata/greet-anon_actual.png",
			},
		},
	},

	// Loged in users are greeted with their username as background.
	{Request: Request{URL: "/login?user=red"}},
	{
		Name:    "Greet Red user (red bg)",
		Request: Request{URL: "/greet?name=Red"},
		Checks: []Check{
			&Screenshot{
				Browser:  Browser{Geometry: "96x32"},
				Expected: "./testdata/greet-red.png",
				Actual:   "./testdata/greet-red_actual.png",
			},
		},
	},

	// Log out again, clear cookie.
	{Request: Request{URL: "/login?user"}},

	// User rt's background depends on basic auth:
	// Without proper basic auth background is olive.
	{Request: Request{URL: "/login?user=rt"}},
	{
		Name:    "Greet RT user (olive bg)",
		Request: Request{URL: "/greet?name=rt"},
		Checks: []Check{
			&Screenshot{
				Browser:  Browser{Geometry: "96x32"},
				Expected: "./testdata/greet-rt.png",
				Actual:   "./testdata/greet-rt_actual.png",
			},
		},
	},

	// User rt's background depends on basic auth:
	// With proper basic auth background is lime.
	{Request: Request{URL: "/login?user=rt"}},
	{
		Name: "Greet RT user (lime bg)",
		Request: Request{
			URL: "/greet?name=rt",
			Authorization: Authorization{
				Basic: BasicAuth{"rt", "secret"},
			},
		},
		Checks: []Check{
			&Screenshot{
				Browser:  Browser{Geometry: "96x32"},
				Expected: "./testdata/greet-rt-auth.png",
				Actual:   "./testdata/greet-rt-auth_actual.png",
			},
		},
	},

	// Log out again, clear cookie.
	{Request: Request{URL: "/login?user"}},

	// Golden record has size 96x32: Compare to larger/smaller screenshot.
	{
		Name:    "Greet Anonymous (different sizes)",
		Request: Request{URL: "/greet"},
		Checks: []Check{
			&Screenshot{
				Browser:  Browser{Geometry: "64x16"},
				Expected: "./testdata/greet-anon.png"},
			&Screenshot{
				Browser:  Browser{Geometry: "128x48"},
				Expected: "./testdata/greet-anon.png"},
		},
	},

	// White background (no cookie) but with name Bob. Ignoring the rectangle.
	{
		Name:    "Greet Bob, ignoring name",
		Request: Request{URL: "/greet?name=Bob"},
		Checks: []Check{
			&Screenshot{
				Browser:      Browser{Geometry: "96x32"},
				Expected:     "./testdata/greet-anon.png",
				IgnoreRegion: []string{"30x40+57+18"},
			},
		},
	},

	// White background (no cookie) but with name Bob. Allowing some pixels to differ.
	{
		Name:    "Greet Bob, tollerating difference",
		Request: Request{URL: "/greet?name=Bob"},
		Checks: []Check{
			&Screenshot{
				Browser:           Browser{Geometry: "96x32"},
				Expected:          "./testdata/greet-anon.png",
				AllowedDifference: 60, // 51 is the hard limit
			},
		},
	},
}

func TestScreenshotPass(t *testing.T) {
	if !WorkingPhantomJS() {
		t.Skip("PhantomJS is not installed")
	}

	ts := httptest.NewServer(http.HandlerFunc(screenshotHandler))
	defer ts.Close()

	for i := range passingScreenshotTests {
		u := ts.URL + "/screenshot" + passingScreenshotTests[i].Request.URL
		passingScreenshotTests[i].Request.URL = u
	}
	suite := Collection{
		Tests: passingScreenshotTests,
	}
	jar, _ := cookiejar.New(nil)
	suite.ExecuteConcurrent(1, jar)
	if *verboseTest {
		// suite.PrintReport(os.Stdout)
	}

	if suite.Status != Pass {
		for i, test := range suite.Tests {
			if test.Status != Pass {
				t.Errorf("%d. %s, %s: %s",
					i, test.Name, test.Status, test.Error)
			}
		}
	}
}

var failingScreenshotTests = []*Test{
	{
		Name:    "Screenshot of Home copared to Greeting",
		Request: Request{URL: "/home"},
		Checks: []Check{
			&Screenshot{
				Browser:  Browser{Geometry: "128x64+0+0"},
				Expected: "./testdata/greet-anon.png",
			},
		},
	},

	{
		Name:    "Greet Bob, Ignore region too small",
		Request: Request{URL: "/greet?name=Bob"},
		Checks: []Check{
			&Screenshot{
				Browser:      Browser{Geometry: "96x32"},
				Expected:     "./testdata/greet-anon.png",
				IgnoreRegion: []string{"30x30+57+18"},
			},
		},
	},

	{
		Name:    "Greet Bob, tolerating difference, but not enough",
		Request: Request{URL: "/greet?name=Bob"},
		Checks: []Check{
			&Screenshot{
				Browser:           Browser{Geometry: "96x32"},
				Expected:          "./testdata/greet-anon.png",
				AllowedDifference: 20, // 51 is the hard limit
			},
		},
	},

	{Request: Request{URL: "/login?user=rt"}},
	{
		Name: "Greet RT user with bad authentication",
		Request: Request{
			URL: "/greet?name=rt",
			Authorization: Authorization{
				Basic: BasicAuth{"rt", "wrong"},
			},
		},
		Checks: []Check{
			&Screenshot{
				Browser:  Browser{Geometry: "96x32"},
				Expected: "./testdata/greet-rt-auth.png",
				Actual:   "./testdata/greet-rt-auth_bad.png",
			},
		},
	},
}

func TestScreenshotFail(t *testing.T) {
	if !WorkingPhantomJS() {
		t.Skip("PhantomJS is not installed")
	}

	ts := httptest.NewServer(http.HandlerFunc(screenshotHandler))
	defer ts.Close()

	for i := range failingScreenshotTests {
		u := ts.URL + "/screenshot" + failingScreenshotTests[i].Request.URL
		failingScreenshotTests[i].Request.URL = u
	}
	suite := Collection{
		Tests: failingScreenshotTests,
	}

	jar, _ := cookiejar.New(nil)
	suite.ExecuteConcurrent(1, jar)
	if *verboseTest {
		// suite.PrintReport(os.Stdout)
	}

	if suite.Status != Fail {
		for i, test := range suite.Tests {
			if test.Status != Fail {
				t.Errorf("%d. %s, %s: %s",
					i, test.Name, test.Status, test.Error)
			}
		}
	}
}

// ----------------------------------------------------------------------------
// RenderedHTML

var passingRenderedHTMLTests = []*Test{
	{
		Name:    "Welcome Anonymous, raw body",
		Request: Request{URL: "/welcome"},
		Checks: []Check{
			&Body{Contains: "Welcome"},
			&Body{Contains: "Anon", Count: -1},
			&Body{Contains: "Joe", Count: -1},
			&Body{Contains: "Changed", Count: -1},
		},
	},
	{
		Name:    "Welcome Anonymous, rendered body",
		Request: Request{URL: "/welcome"},
		Checks: []Check{
			&RenderedHTML{
				Checks: []Check{
					&Body{Contains: "Welcome"},
					&Body{Contains: "Anon", Count: 1},
					&Body{Contains: "Joe", Count: -1},
					&Body{Contains: "Changed", Count: 1},
				},
			},
		},
	},

	// Loged in users are greeted with their username as background.
	{Request: Request{URL: "/login?user=Joe"}},
	{
		Name:    "Welcome Joe",
		Request: Request{URL: "/welcome"},
		Checks: []Check{
			&Body{Contains: "Welcome"},
			&RenderedHTML{
				Checks: []Check{
					&Body{Contains: "You are: Joe"},
					&HTMLContains{
						Selector: "a",
						Text:     []string{"Changed"},
					},
				},
				KeepAs: "testdata/welcome-rendered.html",
			},
		},
	},
}

func TestRenderedHTMLPassing(t *testing.T) {
	if !WorkingPhantomJS() {
		t.Skip("PhantomJS is not installed")
	}

	ts := httptest.NewServer(http.HandlerFunc(screenshotHandler))
	defer ts.Close()

	for i := range passingRenderedHTMLTests {
		u := ts.URL + "/screenshot" + passingRenderedHTMLTests[i].Request.URL
		passingRenderedHTMLTests[i].Request.URL = u
	}
	suite := Collection{
		Tests: passingRenderedHTMLTests,
	}

	jar, _ := cookiejar.New(nil)
	suite.ExecuteConcurrent(1, jar)
	if *verboseTest {
		// suite.PrintReport(os.Stdout)
	}

	if suite.Status != Pass {
		for i, test := range suite.Tests {
			if test.Status != Pass {
				t.Errorf("%d. %s, %s: %s",
					i, test.Name, test.Status, test.Error)
			}
		}
	}
}

// ----------------------------------------------------------------------------
// RenderingTime

var passingRenderingTimeTests = []*Test{
	{
		Name:    "Welcome Anonymous, rendered body",
		Request: Request{URL: "/welcome"},
		Checks: []Check{
			&RenderingTime{Max: 80 * time.Millisecond},
		},
	},
	{Request: Request{URL: "/login?user=Joe"}},
	{
		Name:    "Welcome Joe",
		Request: Request{URL: "/welcome"},
		Checks: []Check{
			&RenderingTime{Max: 120 * time.Millisecond},
		},
	},
}

func TestRenderingTime(t *testing.T) {
	if !WorkingPhantomJS() {
		t.Skip("PhantomJS is not installed")
	}

	ts := httptest.NewServer(http.HandlerFunc(screenshotHandler))
	defer ts.Close()

	for i := range passingRenderingTimeTests {
		u := ts.URL + "/screenshot" + passingRenderingTimeTests[i].Request.URL
		passingRenderingTimeTests[i].Request.URL = u
	}
	suite := Collection{
		Tests: passingRenderingTimeTests,
	}

	jar, _ := cookiejar.New(nil)
	suite.ExecuteConcurrent(1, jar)
	if *verboseTest {
		// suite.PrintReport(os.Stdout)
	}

	if test := suite.Tests[0]; test.Status != Fail {
		t.Errorf("%s, %s: %s",
			test.Name, test.Status, test.Error)
	}
	if test := suite.Tests[1]; test.Status != Pass {
		t.Errorf("%s, %s: %s",
			test.Name, test.Status, test.Error)
	}
}

func TestRenderingTime2(t *testing.T) {
	if !WorkingPhantomJS() {
		t.Skip("PhantomJS is not installed")
	}

	ts := httptest.NewServer(http.HandlerFunc(animationHandler))
	defer ts.Close()

	test1 := &Test{
		Request: Request{URL: ts.URL},
		Checks: []Check{
			&RenderingTime{
				Browser: Browser{Geometry: "128x64+0+0",
					WaitUntilVisible:   []string{"#ready"},
					WaitUntilInvisible: []string{"#waiting"},
				},
				Max: 1050 * time.Millisecond,
			},
		},
	}
	err1 := test1.Run()
	if err1 != nil {
		t.Errorf("Unexpected error %s (%#v)", err1, err1)
	}
	if test1.Status != Pass || test1.Error != nil {
		t.Errorf("Want status=Pass and nil error, got %s, %s <%T>",
			test1.Status, test1.Error, test1.Error)
	}

	test2 := &Test{
		Request: Request{URL: ts.URL},
		Checks: []Check{
			&RenderingTime{
				Browser: Browser{Geometry: "128x64+0+0",
					WaitUntilVisible:   []string{"#ready"},
					WaitUntilInvisible: []string{"#waiting"},
				},
				Max: 500 * time.Millisecond,
			},
		},
	}
	err2 := test2.Run()
	if err2 != nil {
		t.Errorf("Unexpected error %s (%#v)", err2, err2)
	}
	if test2.Status != Fail || test2.Error == nil {
		t.Errorf("Want status=Fail and error, got %s, %s <%T>",
			test2.Status, test2.Error, test2.Error)
	}

}

// ----------------------------------------------------------------------------
// Fany stuff in screenshooting

var animationHTML = `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  <title>Animation</title>
</head>
<body style="background-color: white">

<h3 id="waiting"><p>Waiting...</p></h3>
<h3 id="ready" style="display: none"><p>Ready!</p></h3>

<script>
setTimeout(function(){
  var e = document.getElementById('waiting');
  e.style.display = 'none';
}, 400);

setTimeout(function(){
  var e = document.getElementById('ready');
  e.style.display = 'block';
}, %s);
</script>

</body>
</html>
`

func animationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	readyDelay := "800"
	delay := r.FormValue("delay")
	if delay != "" {
		readyDelay = delay
	}

	fmt.Fprintf(w, animationHTML, readyDelay)
}

func TestFancyScreenshotPass(t *testing.T) {
	if !WorkingPhantomJS() {
		t.Skip("PhantomJS is not installed")
	}

	ts := httptest.NewServer(http.HandlerFunc(animationHandler))
	defer ts.Close()

	test := &Test{
		Name:    "Fancy Screenshot",
		Request: Request{URL: ts.URL},
		Checks: []Check{
			&Screenshot{
				Browser: Browser{Geometry: "128x64+0+0",
					WaitUntilVisible:   []string{"#ready"},
					WaitUntilInvisible: []string{"#waiting"},
					Timeout:            1500 * time.Millisecond,
				},
				Expected: "./testdata/animated.png",
				Actual:   "./testdata/animated_actual.png",
			},
		},
	}

	err := test.Run()
	if err != nil {
		t.Errorf("Unexpected error %s (%#v)", err, err)
	}
	if test.Status != Pass || test.Error != nil {
		t.Errorf("Want status=Pass and nil error, got %s, %s <%T>",
			test.Status, test.Error, test.Error)
	}
}

func TestFancyScreenshotFail(t *testing.T) {
	if !WorkingPhantomJS() {
		t.Skip("PhantomJS is not installed")
	}

	ts := httptest.NewServer(http.HandlerFunc(animationHandler))
	defer ts.Close()

	test := &Test{
		Name:    "Fancy Screenshot",
		Request: Request{URL: ts.URL + "?delay=2000"},
		Checks: []Check{
			&Screenshot{
				Browser: Browser{
					Geometry:           "128x64+0+0",
					WaitUntilVisible:   []string{"#ready"},
					WaitUntilInvisible: []string{"#waiting"},
					Timeout:            1500 * time.Millisecond,
				},
				Expected: "./testdata/animated-fail.png",
				Actual:   "./testdata/animated-fail_actual.png",
			},
		},
	}

	err := test.Run()
	if err != nil {
		t.Errorf("Unexpected error %s (%#v)", err, err)
	}
	if test.Status != Fail || test.Error == nil {
		t.Errorf("Want status=Fail and non-nil error, got %s, %s <%T>", test.Status, err, err)
	}
}
