// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// recorder is a reverse proxy to record requests and output tests.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/json5"
)

var (
	events []event // the global list of request/response events to generate tests for

	directory  = flag.String("dir", "./recorded", "save tests to directory `d`")
	name       = flag.String("name", "test", "set name of tests to `n`")
	disarm     = flag.Duration("disarm", 1*time.Second, "duration in which recording is off")
	ignoreCT   = flag.String("ignore.ct", "", "ignore request with content-type matching `regexp`")
	ignorePath = flag.String("ignore.path", "", "ignore request with path matching `regexp`")

	ignoreCTRe   *regexp.Regexp
	ignorePathRe *regexp.Regexp
)

func main() {
	flag.Parse()
	args := flag.Args()

	if *ignoreCT != "" {
		ignoreCTRe = regexp.MustCompile(*ignoreCT)
	}
	if *ignorePath != "" {
		ignorePathRe = regexp.MustCompile(*ignorePath)
	}

	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Missing target\n")
		os.Exit(9)
	}

	remote, err := url.Parse(args[0])
	if err != nil {
		panic(err)
	}

	requests := make(chan event, 10)
	go process(requests)

	proxy := httputil.NewSingleHostReverseProxy(remote)
	http.HandleFunc("/", handler(proxy, requests))
	http.HandleFunc("/DUMP", dumpEvents)
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

type event struct {
	request   *http.Request
	body      []byte
	response  *httptest.ResponseRecorder
	timestamp time.Time
}

func handler(p *httputil.ReverseProxy, requests chan event) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		rr := httptest.NewRecorder()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err.Error()) // Harsh but what else?
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		p.ServeHTTP(rr, r)
		requests <- event{r, body, rr, time.Now()}
		for h, v := range rr.HeaderMap {
			w.Header()[h] = v
		}
		w.WriteHeader(rr.Code)
		w.Write(rr.Body.Bytes())
	}
}

func process(requests chan event) {
	last := time.Now()
	log.Println("Started processing")
	for e := range requests {
		delta := e.timestamp.Sub(last)
		if delta < *disarm {
			continue
		}
		if ignorePathRe != nil && ignorePathRe.MatchString(e.request.URL.Path) {
			log.Println("Ignoring path", e.request.URL.Path)
			continue
		}
		if ignoreCTRe != nil && ignoreCTRe.MatchString(e.response.HeaderMap.Get("Content-Type")) {
			log.Println("Ignoring content type ", e.response.HeaderMap.Get("Content-Type"))
			continue
		}
		last = e.timestamp
		events = append(events, e)
		log.Println("Recorded", e.request.Method, e.request.URL, " --> ", e.response.Code)
	}
}

type Test struct {
	Name        string
	Description string   `json:",omitempty"`
	BasedOn     []string `json:",omitempty"`
	Request     ht.Request
	Checks      ht.CheckList `json:",omitempty"`
}

func dumpEvents(w http.ResponseWriter, r *http.Request) {
	err := os.MkdirAll(*directory, 0777)
	if err != nil {
		panic(err.Error())
	}

	// extract all common headers into mixin
	commonHeaders := extractCommonHeaders()
	test := &Test{
		Name: fmt.Sprintf("Common Header of %s", *name),
		Request: ht.Request{
			Header: commonHeaders,
		},
	}

	commonFilename := path.Join(*directory, "common-headers.mixin")
	writeTest(test, commonFilename)

	for i, e := range events {
		host := e.request.URL.Host
		e.request.URL.Host = "H.O.S.T.N.A.M.E"
		cookies := []ht.Cookie{}
		for _, c := range e.request.Cookies() {
			cookies = append(cookies, ht.Cookie{Name: c.Name, Value: c.Value})
		}
		e.request.Header.Del("Cookie")

		params := e.request.URL.Query()
		e.request.URL.RawQuery = ""
		urlString := e.request.URL.String()
		urlString = strings.Replace(urlString, "H.O.S.T.N.A.M.E", "{{HOSTNAME}}", 1)

		// TODO: scan body for parameters and set ParamsAs
		body := e.body

		checks := extractChecks(e)

		test := &Test{
			Name:        fmt.Sprintf("%s %d", *name, i+1),
			Description: fmt.Sprintf("Recorded from %s on %s", host, time.Now()),
			BasedOn:     []string{commonFilename},
			Request: ht.Request{
				Method:  e.request.Method,
				URL:     urlString,
				Cookies: cookies,
				Header:  e.request.Header,
				Params:  ht.URLValues(params),
				Body:    string(body),
			},
			Checks: checks,
		}

		filename := path.Join(*directory, fmt.Sprintf("%s_%02d.ht", *name, i+1))
		writeTest(test, filename)

		log.Println("Generate test:  ", e.request.Method, e.request.URL, " --> ", filename)
	}

	http.Error(w, fmt.Sprintf("Generated %d tests.", len(events)), 200)
	events = events[:0]
}

func writeTest(test *Test, filename string) {
	data, err := json5.MarshalIndent(test, "", "    ")
	if err != nil {
		log.Printf("Ooops: Test %s, cannot serialize: %s", test.Name, err)
		return
	}
	err = ioutil.WriteFile(filename, data, 0666)
	if err != nil {
		log.Printf("Ooops: Test %s, cannot write file %s: %s", test.Name, filename, err)
		return
	}
}

func extractChecks(e event) ht.CheckList {
	list := ht.CheckList{}

	// Allways add StatusCode check.
	list = append(list, ht.StatusCode{Expect: e.response.Code})

	// Check for Content-Type header.
	ct := e.response.Header().Get("Content-Type")
	if ct != "" {
		ct = strings.TrimSpace(strings.Split(ct, ";")[0])
		if i := strings.Index(ct, "/"); i != -1 {
			ct := ct[i+1:]
			list = append(list, ht.ContentType{Is: ct})
		} else {
			ct = ""
		}
	}

	// Checks for Set-Cookie headers:
	dummy := http.Response{Header: e.response.Header()}
	for _, c := range dummy.Cookies() {
		sc := &ht.SetCookie{Name: c.Name, Value: ht.Condition{Equals: c.Value}}
		// TODO: MinLifetime, Path, etc.
		list = append(list, sc)
	}

	return list
}

func extractCommonHeaders() http.Header {
	common := http.Header{}
	h0 := events[0].request.Header
	for h, v := range h0 {
		vs := fmt.Sprintf("%v", v)
		identical := true
		for j := 2; j < len(events); j++ {
			if vs != fmt.Sprintf("%v", events[j].request.Header[h]) {
				identical = false
				break
			}
		}
		if identical {
			common[h] = v
			for _, e := range events {
				e.request.Header.Del(h)
			}
		}
	}
	return common
}
