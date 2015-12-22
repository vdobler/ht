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
Quick performs a set of standard checks on the given URLs which must
be HTML pages (and not images, JSON files, scripts or that like).
	`,
}

var defaultHeader = http.Header{}
var fullChecksFlag = false

func init() {
	addSkiptlsverifyFlag(cmdQuick.Flag)
	addVerbosityFlag(cmdQuick.Flag)
	addOutputFlag(cmdQuick.Flag)
	cmdQuick.Flag.BoolVar(&fullChecksFlag, "full", false,
		"check links, latency and resilience too")

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

func makeChecks(u string) ht.CheckList {
	cl := ht.CheckList{
		ht.StatusCode{Expect: 200},
		ht.ContentType{Is: "html"},
		ht.UTF8Encoded{},
		ht.ValidHTML{},
		ht.ResponseTime{Lower: ht.Duration(1500 * time.Millisecond)},
	}

	if fullChecksFlag {
		cl = append(cl,
			&ht.Links{
				Which:       "a img link script",
				Concurrency: 8,
				OnlyLinks:   []ht.Condition{sameHostCondition(u)},
			})

		cl = append(cl,
			ht.Resilience{
				ModParam:  "all",
				ModHeader: "drop twice nonsense empty none",
			})

		cl = append(cl,
			&ht.Latency{
				Concurrent:         1,
				N:                  31,
				SkipChecks:         true,
				Limits:             "1% ≤ 1ms; 25% ≤ 2ms; 50% ≤ 3ms; 80% ≤ 4ms; 90% ≤ 5ms; 95% ≤ 6ms",
				IndividualSessions: true,
			})

		cl = append(cl,
			&ht.Latency{
				Concurrent:         3,
				N:                  51,
				SkipChecks:         true,
				Limits:             "1% ≤ 1ms; 25% ≤ 2ms; 50% ≤ 3ms; 80% ≤ 4ms; 90% ≤ 5ms; 95% ≤ 6ms",
				IndividualSessions: true,
			})

		cl = append(cl,
			&ht.Latency{
				Concurrent:         6,
				N:                  71,
				SkipChecks:         true,
				Limits:             "1% ≤ 1ms; 25% ≤ 2ms; 50% ≤ 3ms; 80% ≤ 4ms; 90% ≤ 5ms; 95% ≤ 6ms",
				IndividualSessions: true,
			})
	}

	return cl
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
			Checks: makeChecks(u),
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
