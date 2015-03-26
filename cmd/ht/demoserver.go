// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// A simple demoserver for local tests.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

var (
	port = flag.String("port", ":4490", "Port on localhost to run the demo server.")
)

func main() {
	flag.Parse()
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/echo", echoHandler)
	http.HandleFunc("/redirect", redirectHandler)
	http.HandleFunc("/cookie", cookieHandler)
	log.Fatal(http.ListenAndServe(*port, nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Demo Server</title></head>
<body>
<h1>Welcome to the demo server</h1>
</bod>
</html>`)
}

// redirectHandler called with a path of /<n> will redirect to /<n-1> if n>0.
// A path of /0 prints "Hello World" and any other path results in a 500 error.
func redirectHandler(w http.ResponseWriter, r *http.Request) {
	np := r.URL.Path[1:]
	n, err := strconv.Atoi(np)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if n > 0 {
		u := r.URL
		u.Path = fmt.Sprintf("/%d", n-1)
		http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
		return
	}
	http.Error(w, "Hello World!", http.StatusOK)
}

// parse the form value name as an int, defaulting to 0.
func intFormValue(r *http.Request, name string) int {
	s := r.FormValue(name)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// echoHandler answers the request based on the parameters status (HTTP status
// code), text (the response body) and header and value (any HTTP header).
// The handler sleeps for a random duration beteen smin and smax milliseconds.
// If echoHandler is called with parameter fail it timeout with the given
// probability. The parameter bad controls the probability if bad responses.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	fail := intFormValue(r, "fail")
	if fail > 0 {
		k := rand.Intn(100)
		if k < fail {
			panic("Fail")
		}
	}
	status := intFormValue(r, "status")
	if status == 0 {
		status = 200
	}
	smin, smax := intFormValue(r, "smin"), intFormValue(r, "smax")
	if smin > 0 {
		if smax <= smin {
			smax = smin + 1
		}
		sleep := rand.Intn(1000*(smax-smin)) + 1000*smin // in microseconds
		time.Sleep(1000 * time.Duration(sleep))          // now in nanoseconds
	}
	header, value := r.FormValue("header"), r.FormValue("value")
	if header != "" {
		w.Header().Set(header, value)
	}
	text := r.FormValue("text")
	bad := intFormValue(r, "bad")
	if bad > 0 {
		k := rand.Intn(100)
		if k < bad {
			text, status = "XXXXXXX", 500
		}
	}

	http.Error(w, text, status)
}

// cookieHandler
func cookieHandler(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	value := r.FormValue("value")
	httpOnly := r.FormValue("httponly") != ""
	maxAge := intFormValue(r, "maxage")
	if name != "" {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: value,
			HttpOnly: httpOnly,
			MaxAge:   maxAge,
		})
	}

	fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Received Cookies</title></head>
<body>
<h1>Received Cookies</h1>
<ul>`)
	for _, cookie := range r.Cookies() {
		fmt.Fprintf(w, "<li>Name=%q Value=%q</li>\n", cookie.Name, cookie.Value)
	}
	fmt.Fprintf(w, `</ul>
</body>
</html>
`)
}
