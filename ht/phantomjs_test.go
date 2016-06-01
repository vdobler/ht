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
		user := ""
		if cookie, err := r.Cookie("user"); err == nil {
			user = cookie.Value
		}
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, screenshotGreetHTML, user)
	case "/screenshot/css":
		user := "white"
		if cookie, err := r.Cookie("user"); err == nil {
			user = cookie.Value
		}
		w.Header().Set("Content-Type", "text/css")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, screenshotCSS, user)
	}
}

func TestScreenshotPass(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(screenshotHandler))
	defer ts.Close()
	println(ts.URL)
	concLevels := []int{1, 4, 16}
	if !testing.Short() {
		concLevels = append(concLevels, 64)
	}

	makeTest := func(name, path, geom, expect string) *Test {
		return &Test{
			Name: name,
			Request: Request{
				URL: ts.URL + "/screenshot/" + path,
			},
			Checks: []Check{
				&Screenshot{
					Geometry: geom,
					Expected: "./testdata/" + expect + ".png",
					Actual:   "./testdata/" + expect + "_actual.png",
				},
			},
		}
	}

	suite := Suite{
		KeepCookies: true,
		Tests: []*Test{
			makeTest("Basic Screenshot", "home", "128x64+0+0", "home"),
			makeTest("Anon Greet", "greet", "64x32", "greet-anon"),

			&Test{Request: Request{URL: ts.URL + "/screenshot/login?user=red"}},
			makeTest("Red Greet", "greet", "64x32", "greet-red"),

			// Log out again.
			&Test{Request: Request{URL: ts.URL + "/screenshot/login?user"}},
			// Golden record has size 64x32: Compare to larger/smaller screenshot.
			makeTest("Anon Greet", "greet", "32x16", "greet-anon"),
			makeTest("Anon Greet", "greet", "80x48", "greet-anon"),
		},
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

	fmt.Println(suite.Tests[0].Response.BodyStr)

	// time.Sleep(time.Minute)
}
