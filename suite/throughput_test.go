// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func waitHandler(w http.ResponseWriter, r *http.Request) {
	mean, stdd := formValue(r, "mean"), formValue(r, "stdd")
	x := rand.NormFloat64()*stdd + mean
	if x <= 0 {
		x = 1
	}
	s := time.Millisecond * time.Duration(x)
	time.Sleep(s)

	status := http.StatusOK
	if rand.Intn(200) < 6 {
		status = http.StatusBadRequest
	}

	dv, exp := r.FormValue("dyn"), r.FormValue("exp")
	if dv != exp {
		http.Error(w, "Bad dyn value", http.StatusForbidden)
		return
	}

	http.Error(w, fmt.Sprintf("Slept for %s", s), status)
}

func formValue(r *http.Request, name string) float64 {
	s := r.FormValue(name)
	if s == "" {
		return 0
	}
	f64, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 1
	}
	return f64
}

var tpSuiteSlow = `
# slow.suite
{
    Name: Slow Suite
    Main: [
        { File: "testA.ht" }
        { File: "testB.ht" }
        { File: "testC.ht" }
        { File: "testD.ht" }
    ]
    Verbosity: 1
}

# testA.ht
{
    Name: Test A
    Request: { URL: "{{URL}}?mean=50&stdd=10" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}

# testB.ht
{
    Name: Test B
    Request: { URL: "{{URL}}?mean=50&stdd=30" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}

# testC.ht
{
    Name: Test C
    Request: {
        URL: "{{URL}}?mean=50&stdd=50"
        Timeout: 150ms
    }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}

# testD.ht
{
    Name: Test D
    Request: {
        URL: "{{URL}}?mean=70&stdd=70"
        Timeout: 150ms
    }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}
`

var tpSuiteFast = `
# fast.suite
{
    Name: Fast Suite
    Setup: [
        { File: "testX.ht" }
        { File: "setup.ht" }
        { File: "testY.ht", Variables: { "dyn": "{{DYNAMICVAL}}" } }
    ]
    Main: [
        { File: "testX.ht" }
        { File: "testY.ht", Variables: { "dyn": "{{DYNAMICVAL}}" }  }
    ]
    Teardown: [
        { File: "testX.ht" }
        { File: "testY.ht" }
    ]
    Variables: {
        DYNAMICVAL: "bar999"
    }
    Verbosity: 0
}

# testX.ht
{
    Name: Test X
    Request: { URL: "{{URL}}?mean=10&stdd=5" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}

# testY.ht
{
    Name: Test Y
    Request: { URL: "{{URL}}?mean=25&stdd=5&dyn={{dyn}}&exp=foo123" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}

# setup.ht
{
    Name: Setup for fast
    Request: { URL: "{{URL}}" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
    VarEx: {
        DYNAMICVAL: {Extractor: "SetVariable", To: "foo123"}
    }
}
`
var tpSuiteSlooow = `
# slooow.suite
{
    Name: Slooow Suite
    Main: [
        { File: "testG.ht" }
        { File: "testH.ht" }
        { File: "testI.ht" }
    ]
    Verbosity: 2
}

# testG.ht
{
    Name: Test G
    Request: { URL: "{{URL}}?mean=120&stdd=5" }
    Checks: [
        {Check: "StatusCode", Expect: 200}
        {Check: "ResponseTime", Lower: "126ms"}
    ]
}

# testH.ht
{
    Name: Test H
    Request: { URL: "{{URL}}?mean=110&stdd=8" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}

# testI.ht
{
    Name: Test I
    Request: { URL: "{{URL}}?mean=100&stdd=12" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}
`

func TestThroughput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(waitHandler))
	defer ts.Close()

	slow, err := parseRawSuite("slow.suite", tpSuiteSlow)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	fast, err := parseRawSuite("fast.suite", tpSuiteFast)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	slooow, err := parseRawSuite("slooow.suite", tpSuiteSlooow)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	globals := map[string]string{
		"URL": ts.URL,
	}
	scenarios := []Scenario{
		{
			Name:       "Fast",
			RawSuite:   fast,
			Percentage: 40,
			MaxThreads: 10,
			globals:    globals,
		},
		{
			Name:       "Slow",
			RawSuite:   slow,
			Percentage: 40,
			MaxThreads: 12,
			globals:    globals,
		},
		{
			Name:       "Slooow",
			RawSuite:   slooow,
			Percentage: 20,
			MaxThreads: 8,
			globals:    globals,
		},
	}

	data, failures, err := Throughput(scenarios, 50, 10*time.Second, 3*time.Second)
	if err != nil {
		fmt.Println("==> ", err.Error())
	}
	if testing.Verbose() && false {
		fmt.Println("")
		fmt.Println("   FAILURES")
		fmt.Println("=================")
		failures.PrintReport(os.Stdout)
		err = HTMLReport("./testdata", failures)
		if err != nil {
			log.Panic(err)
		}
	}

	cnt := make(map[string]int)
	for _, d := range data {
		parts := strings.SplitN(d.ID, IDSep, 3)
		sn := parts[1]
		cnt[sn] = cnt[sn] + 1
	}

	N := float64(len(data))
	fastP := float64(cnt["Fast"]) / N
	slowP := float64(cnt["Slow"]) / N
	slooowP := float64(cnt["Slooow"]) / N
	if fastP < 0.30 || fastP > 0.50 ||
		slowP < 0.30 || slowP > 0.50 ||
		slooowP < 0.10 || slooowP > 0.30 {
		t.Errorf("Bad distribution of scenarios: fast=%f slow=%f slooow=%f",
			fastP, slowP, slooowP)
	}
	fmt.Println("Recorded ", len(data), "points")

	file, err := os.Create("testdata/throughput.csv")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer file.Close()

	err = DataToCSV(data, file)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	script := `
library(ggplot2)
d <- read.csv("throughput.csv")
d$Status <- factor(d$Status, levels <- c("NotRun", "Skipped", "Pass", "Fail", "Error", "Bogus"))

myColors <- c("#999999", "#ffff00", "#339900", "#660000", "#ff0000", "#ff3399")
names(myColors) <- levels(d$Status)
colScale <- scale_colour_manual(name = "status",values = myColors)
fillScale <- scale_fill_manual(name = "status",values = myColors)

p <- ggplot(d, aes(x=Elapsed, y=ReqDuration, colour=Test))
p <- p + geom_point(size=3)
ggsave("scatter.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=ReqDuration, colour=Status))
p <- p + geom_point(size=3)
p <- p + colScale
ggsave("status.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=ReqDuration, fill=Status))
p <- p + geom_histogram(binwidth=3)
p <- p + facet_grid(Test ~ ., scales="free_y")
p <- p + fillScale
ggsave("hist.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=Rate))
p <- p + geom_point(size=3) + geom_smooth()
ggsave("rate.png", plot=p, width=10, height=8, dpi=100)
`
	ioutil.WriteFile("testdata/throughput.R", []byte(script), 0666)
}

// ###############################################################

func waitHandler2(w http.ResponseWriter, r *http.Request) {
	means := map[string]float64{
		"/robots.txt":   20,
		"/index.html":   100,
		"/sitemap.xml":  40,
		"/category/abc": 60,
		"/search":       80,
	}
	path := r.URL.Path
	mean, stdd := 150.0, 10.0
	if m, ok := means[path]; ok {
		mean = m
	}
	x := rand.NormFloat64()*stdd + mean
	if x <= 0 {
		x = 1
	}
	s := time.Millisecond * time.Duration(x)
	time.Sleep(s)

	status := http.StatusOK
	if rand.Intn(200) < 6 {
		status = http.StatusBadRequest
	}
	http.Error(w, fmt.Sprintf("Slept for %s. Welcome! allow Sitemap letters Found34 matches", s), status)
}

func TestThroughput2(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(waitHandler2))
	defer ts.Close()

	raw, err := parseRawLoadtest("dummy.load", sampleLoadtest)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	global := map[string]string{
		"HOST": ts.URL,
	}

	scenarios := raw.ToScenario(global)
	for i, scen := range scenarios {
		fmt.Printf("%d. %d%% %q (max %d threads)\n",
			i+1, scen.Percentage, scen.Name, scen.MaxThreads)
	}

	data, _, err := Throughput(scenarios, 100, 4*time.Second, 0)
	if err != nil {
		fmt.Println("==> ", err.Error())
	}

	fmt.Println("Recorded ", len(data), "points")

	file, err := os.Create("testdata/throughput2.csv")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer file.Close()

	DataToCSV(data, file)

	script := `
library(ggplot2)
d <- read.csv("throughput2.csv")
d$Status <- factor(d$Status, levels <- c("NotRun", "Skipped", "Pass", "Fail", "Error", "Bogus"))

myColors <- c("#999999", "#ffff00", "#339900", "#660000", "#ff0000", "#ff3399")
names(myColors) <- levels(d$Status)
colScale <- scale_colour_manual(name = "status",values = myColors)
fillScale <- scale_fill_manual(name = "status",values = myColors)

p <- ggplot(d, aes(x=Elapsed, y=ReqDuration, colour=Test))
p <- p + geom_point(size=3)
ggsave("scatter.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=ReqDuration, colour=Status))
p <- p + geom_point(size=3)
p <- p + colScale
ggsave("status.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=Rate))
p <- p + geom_point(size=3)
ggsave("rate.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=ConcTot, colour=Status)) + colScale
p <- p + geom_point(size=3) + xlab("Elapsed [ms]") + ylab("Total Concurrency")
ggsave("conctot.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=ConcOwn, colour=Status)) + colScale
p <- p + facet_grid(Test ~ ., scales="free_y")
p <- p + geom_point(size=3) + xlab("Elapsed [ms]")
ggsave("concown.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=ReqDuration, fill=Status))
p <- p + geom_histogram(binwidth=3)
p <- p + facet_grid(Test ~ ., scales="free_y")
p <- p + fillScale
ggsave("hist.png", plot=p, width=10, height=8, dpi=100)
`
	ioutil.WriteFile("testdata/throughput2.R", []byte(script), 0666)
}
