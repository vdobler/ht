// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"
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

func TestParseGeometry(t *testing.T) {
	for i, tc := range geometryTests {
		g, err := parseGeometry(tc.s)
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
		if g.width != tc.width || g.height != tc.height {
			t.Errorf("%d. %q: Wrong size, got %dx%d", i, tc.s, g.width, g.height)
		}
		if g.top != tc.top || g.left != tc.left {
			t.Errorf("%d. %q: Wrong offset, got +%d+%d", i, tc.s, g.left, g.top)
		}
		if g.zoom != tc.zoom {
			t.Errorf("%d. %q: Wrong zoom, go %d", i, tc.s, g.zoom)
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
		user := "white"
		if cookie, err := r.Cookie("user"); err == nil {
			user = cookie.Value
		}
		w.Header().Set("Content-Type", "text/css")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, screenshotCSS, user)
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
	&Test{
		Name:    "Basic Screenshot of Home",
		Request: Request{URL: "/home"},
		Checks: []Check{
			&Screenshot{
				Geometry: "128x64+0+0",
				Expected: "./testdata/home.png",
				Actual:   "./testdata/home_actual.png",
			},
		},
	},

	// Anonymous user are greeted with white background.
	&Test{
		Name:    "Greet Anonymous (white bg)",
		Request: Request{URL: "/greet"},
		Checks: []Check{
			&Screenshot{
				Geometry: "96x32",
				Expected: "./testdata/greet-anon.png",
				Actual:   "./testdata/greet-anon_actual.png",
			},
		},
	},

	// Loged in users are greeted with their username as background.
	&Test{Request: Request{URL: "/login?user=red"}},
	&Test{
		Name:    "Greet Red user (red bg)",
		Request: Request{URL: "/greet?name=Red"},
		Checks: []Check{
			&Screenshot{
				Geometry: "96x32",
				Expected: "./testdata/greet-red.png",
				Actual:   "./testdata/greet-red_actual.png",
			},
		},
	},

	// Log out again, clear cookie.
	&Test{Request: Request{URL: "/login?user"}},

	// Golden record has size 96x32: Compare to larger/smaller screenshot.
	&Test{
		Name:    "Greet Anonymous (different sizes)",
		Request: Request{URL: "/greet"},
		Checks: []Check{
			&Screenshot{Geometry: "64x16", Expected: "./testdata/greet-anon.png"},
			&Screenshot{Geometry: "128x48", Expected: "./testdata/greet-anon.png"},
		},
	},

	// White background (no cookie) but with name Bob. Ignoring the rectangle.
	&Test{
		Name:    "Greet Bob, ignoring name",
		Request: Request{URL: "/greet?name=Bob"},
		Checks: []Check{
			&Screenshot{
				Geometry:     "96x32",
				Expected:     "./testdata/greet-anon.png",
				IgnoreRegion: []string{"30x40+57+18"},
			},
		},
	},

	// White background (no cookie) but with name Bob. Allowing some pixels to differ.
	&Test{
		Name:    "Greet Bob, tollerating difference",
		Request: Request{URL: "/greet?name=Bob"},
		Checks: []Check{
			&Screenshot{
				Geometry:          "96x32",
				Expected:          "./testdata/greet-anon.png",
				AllowedDifference: 60, // 51 is the hard limit
			},
		},
	},
}

func TestScreenshotPass(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(screenshotHandler))
	defer ts.Close()

	for i := range passingScreenshotTests {
		u := ts.URL + "/screenshot" + passingScreenshotTests[i].Request.URL
		passingScreenshotTests[i].Request.URL = u
	}
	suite := Suite{
		KeepCookies: true,
		Tests:       passingScreenshotTests,
	}

	err := suite.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	suite.Execute()
	if *verboseTest {
		suite.PrintReport(os.Stdout)
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
	&Test{
		Name:    "Screenshot of Home copared to Greeting",
		Request: Request{URL: "/home"},
		Checks: []Check{
			&Screenshot{
				Geometry: "128x64+0+0",
				Expected: "./testdata/greet-anon.png",
			},
		},
	},

	&Test{
		Name:    "Greet Bob, Ignore region too small",
		Request: Request{URL: "/greet?name=Bob"},
		Checks: []Check{
			&Screenshot{
				Geometry:     "96x32",
				Expected:     "./testdata/greet-anon.png",
				IgnoreRegion: []string{"30x30+57+18"},
			},
		},
	},

	&Test{
		Name:    "Greet Bob, tolerating difference, but not enough",
		Request: Request{URL: "/greet?name=Bob"},
		Checks: []Check{
			&Screenshot{
				Geometry:          "96x32",
				Expected:          "./testdata/greet-anon.png",
				AllowedDifference: 20, // 51 is the hard limit
			},
		},
	},
}

func TestScreenshotFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(screenshotHandler))
	defer ts.Close()
	println(ts.URL)

	for i := range failingScreenshotTests {
		u := ts.URL + "/screenshot" + failingScreenshotTests[i].Request.URL
		failingScreenshotTests[i].Request.URL = u
	}
	suite := Suite{
		KeepCookies: true,
		Tests:       failingScreenshotTests,
	}

	err := suite.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	suite.Execute()
	if *verboseTest {
		suite.PrintReport(os.Stdout)
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
	&Test{
		Name:    "Welcome Anonymous, raw body",
		Request: Request{URL: "/welcome"},
		Checks: []Check{
			&Body{Contains: "Welcome"},
			&Body{Contains: "Anon", Count: -1},
			&Body{Contains: "Joe", Count: -1},
			&Body{Contains: "Changed", Count: -1},
		},
	},
	&Test{
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
	&Test{Request: Request{URL: "/login?user=Joe"}},
	&Test{
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
	ts := httptest.NewServer(http.HandlerFunc(screenshotHandler))
	defer ts.Close()

	for i := range passingRenderedHTMLTests {
		u := ts.URL + "/screenshot" + passingRenderedHTMLTests[i].Request.URL
		passingRenderedHTMLTests[i].Request.URL = u
	}
	suite := Suite{
		KeepCookies: true,
		Tests:       passingRenderedHTMLTests,
	}

	err := suite.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	suite.Execute()
	if *verboseTest {
		suite.PrintReport(os.Stdout)
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

func TestRenderingTimeOffset(t *testing.T) {
	if !debugRenderingTime || !testing.Verbose() {
		return
	}
	ioutil.WriteFile("testdata/exit.js", []byte("phantom.exit();\n"), 0666)
	defer os.Remove("testdata/exit.js")

	total := time.Duration(0)
	for i := 0; i < 25; i++ {
		start := time.Now()
		cmd := exec.Command(PhantomJSExecutable, "exit.js")
		cmd.CombinedOutput()
		total += time.Since(start)
	}
	t.Logf("PhantomJS invocation takes %s.", total/25)
}

var passingRenderingTimeTests = []*Test{
	&Test{
		Name:    "Welcome Anonymous, rendered body",
		Request: Request{URL: "/welcome"},
		Checks: []Check{
			&RenderingTime{Max: Duration(80 * time.Millisecond)},
		},
	},
	&Test{Request: Request{URL: "/login?user=Joe"}},
	&Test{
		Name:    "Welcome Joe",
		Request: Request{URL: "/welcome"},
		Checks: []Check{
			&RenderingTime{Max: Duration(120 * time.Millisecond)},
		},
	},
}

func TestRenderingTime(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(screenshotHandler))
	defer ts.Close()

	for i := range passingRenderingTimeTests {
		u := ts.URL + "/screenshot" + passingRenderingTimeTests[i].Request.URL
		passingRenderingTimeTests[i].Request.URL = u
	}
	suite := Suite{
		KeepCookies: true,
		Tests:       passingRenderingTimeTests,
	}

	err := suite.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	suite.Execute()
	if *verboseTest {
		suite.PrintReport(os.Stdout)
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
