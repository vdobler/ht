// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/vdobler/ht/ht"
)

var cmdQuick = &Command{
	RunArgs:     runQuick,
	Usage:       "quick <URL>...",
	Description: "do quick checking of HTML pages",
	Flag:        flag.NewFlagSet("run", flag.ContinueOnError),
	Help: `
Quick performs a set of standard checks on the given URLs.
	`,
}

var defaultHeader = http.Header{}

func init() {
	addSkiptlsverifyFlag(cmdQuick.Flag)
	addVerbosityFlag(cmdQuick.Flag)
	addOutputFlag(cmdQuick.Flag)

	defaultHeader.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	defaultHeader.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Ubuntu Chromium/47.0.2526.73 Chrome/47.0.2526.73 Safari/537.36")
	defaultHeader.Set("Encoding", "gzip")
	defaultHeader.Set("Accept-Language", "en-US,en;q=0.8,de;q=0.6")
	defaultHeader.Set("Cache-Control", "max-age=0")
	defaultHeader.Set("Upgrade-Insecure-Requests", "1")
}

func sameHostCondition(s string) ht.Condition {
	u, err := url.Parse(s)
	if err != nil {
		log.Fatalf("Cannot parse URL %q: %s", s, err)
	}
	u.Path = "/"
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""

	absolute := `/.*`
	relative := `\.`
	full := regexp.QuoteMeta(u.String())

	re := fmt.Sprintf("^(%s|%s|%s)", absolute, relative, full)
	return ht.Condition{Regexp: re}
}

func runQuick(cmd *Command, urls []string) {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	suite := &ht.Suite{
		Name: "Quick Check",
		Log:  logger,
	}

	for _, u := range urls {
		test := &ht.Test{
			Name:        u,
			Description: "Quick test for " + u,
			Request: ht.Request{
				Method:          "GET",
				URL:             u,
				Header:          defaultHeader,
				FollowRedirects: true,
			},
			Checks: ht.CheckList{
				ht.StatusCode{Expect: 200},
				ht.ContentType{Is: "html"},
				ht.UTF8Encoded{},
				ht.ValidHTML{},
				ht.ResponseTime{Lower: ht.Duration(1 * time.Second)},
				&ht.Links{Which: "a img link script",
					Concurrency: 8, OnlyLinks: []ht.Condition{sameHostCondition(u)}},
				ht.Resilience{ModParam: "drop none nonsense type large", ModHeader: "drop none"},
				&ht.Latency{N: 21, Concurrent: 1, SkipChecks: true,
					Limits: "25% ≤ 700ms; 50% ≤ 750ms; 80% ≤ 1.0s; 90% ≤ 1.1s; 95% ≤ 1.25s"},
				&ht.Latency{N: 41, Concurrent: 4, SkipChecks: true,
					Limits: "25% ≤ 900ms; 50% ≤ 1000ms; 80% ≤ 1.5s; 90% ≤ 1.7s; 95% ≤ 2s"},
			},
		}
		suite.Tests = append(suite.Tests, test)
	}

	err := suite.Prepare()
	if err != nil {
		log.Println(err.Error())
		os.Exit(3)
	}
	if verbosity != -99 {
		for i := range suite.Tests {
			suite.Tests[i].Verbosity = verbosity
		}
	}
	suite.Variables = variablesFlag
	runExecute(cmd, []*ht.Suite{suite})
}
