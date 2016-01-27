// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// showcase implements a server to run the showcase.suite against
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

var (
	port = flag.String("port", ":8080", "Port on localhost to run the showcase server.")
)

func main() {
	flag.Parse()
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/admin/load", loadHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/not/there", missingHandler)
	http.HandleFunc("/api/v1/books", booksHandler)
	http.HandleFunc("/api/v1", jsonHandler)
	http.HandleFunc("/static/image/", logoHandler)
	log.Fatal(http.ListenAndServe(*port, nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Duration(rand.Intn(9)+1) * time.Millisecond)
	w.Header().Set("Warning", "Demo only!")
	w.Header().Set("X-Frame-Options", "none")
	fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Showcase</title></head>
<link href="/not/there" />
<body>
 <img src="/static/image/logo.png" alt="Logo" width="32", height="24"/><br/>

 <h1>Welcome to the demo server</h1>
 <div class="special-offer"><h3>Less Bugs</h3></div>

 <a href="/not/there">A broken link</a>
 <img src="/not/there" alt="Logo" />

 <div id="teaser">
  <div id="DD" class="promo">Offer 1</div>
  <div id="DD" class="promo">Offer 2</div>
 </div>

 <div class="special-offer"><h3>Happiness</h3></div>

 Other endpoints: <a href="/api/v1">some JSON</a> and
 <a href="/api/v1/books">some XML</>

 <div><a href="/login">Login</a></div>

 <p></ul>
</body>
</html>`)
}

func loadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Load</title></head>
<body>
<h1>Loading...</h1>
</body>
</html>`)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.SetCookie(w, &http.Cookie{Name: "history", Value: "", Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: "username", Value: "Joe Average", Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "123random",
		Path: "/", MaxAge: 300})
	fmt.Fprintf(w, "Welcome Joe Average!")
}

func missingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "Ooops")
}

func booksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<library>
  <!-- Great book. -->
  <book id="b0836217462" available="true">
    <isbn>0836217462</isbn>
    <title lang="en">Being a Dog Is a Full-Time Job</title>
    <quote>I'd dog paddle the deepest ocean.</quote>
    <author id="CMS">
      <?echo "go rocks"?>
      <name>Charles M Schulz</name>
      <born>1922-11-26</born>
      <dead>2000-02-12</dead>
    </author>
    <character id="PP">
      <name>Peppermint Patty</name>
      <born>1966-08-22</born>
      <qualification>bold, brash and tomboyish</qualification>
    </character>
    <character id="Snoopy">
      <name>Snoopy</name>
      <born>1950-10-04</born>
      <qualification>extroverted beagle</qualification>
    </character>
  </book>
  <book id="299,792,459" available="true">
    <title lang="en">Faster than light</title>
    <character>
      <name>Flash Gordon</name>
    </character>
  </book>
  <book unpublished="true">
    <title lang="en">The year 3826 in pictures</title>
  </book>

</library>`)
}

func logoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "\x89\x50\x4e\x47\x0d\x0a\x1a\x0a\x00\x00\x00\x0d\x49\x48\x44\x52"+
		"\x00\x00\x00\x08\x00\x00\x00\x06\x08\x06\x00\x00\x00\xfe\x05\xdf"+
		"\xfb\x00\x00\x00\x01\x73\x52\x47\x42\x00\xae\xce\x1c\xe9\x00\x00"+
		"\x00\x06\x62\x4b\x47\x44\x00\x00\x00\x00\x00\x00\xf9\x43\xbb\x7f"+
		"\x00\x00\x00\x34\x49\x44\x41\x54\x08\xd7\x85\x8e\x41\x0e\x00\x20"+
		"\x0c\xc2\x28\xff\xff\x33\x9e\x30\x6a\xa2\x72\x21\xa3\x5b\x06\x49"+
		"\xa2\x87\x2c\x49\xc0\x16\xae\xb3\xcf\x8b\xc2\xba\x57\x00\xa8\x1f"+
		"\xeb\x73\xe1\x56\xc5\xfa\x68\x00\x8c\x59\x0d\x11\x87\x39\xe4\xc3"+
		"\x00\x00\x00\x00\x49\x45\x4e\x44\xae\x42\x60\x82")
}

func jsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `
{
  "query": "jo nesbo",
  "result: [ 1, 2 ]
}`)
}
