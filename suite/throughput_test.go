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

	slow, err := ParseRawSuite("slow.suite", tpSuiteSlow)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	fast, err := ParseRawSuite("fast.suite", tpSuiteFast)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	slooow, err := ParseRawSuite("slooow.suite", tpSuiteSlooow)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	logger := log.New(os.Stdout, "", 0)

	globals := map[string]string{
		"URL": ts.URL,
	}
	scenarios := []Scenario{
		Scenario{
			RawSuite:   fast,
			Percentage: 40,
			Log:        logger,
			globals:    globals,
		},
		Scenario{
			RawSuite:   slow,
			Percentage: 40,
			globals:    globals,
		},
		Scenario{
			RawSuite:   slooow,
			Percentage: 20,
			globals:    globals,
		},
	}

	data, failures, err := Throughput(scenarios, 100, 4*time.Second)
	if err != nil {
		fmt.Println("==> ", err.Error())
	}

	fmt.Println("\n   FAILURES\n=================\n")
	PrintSuiteReport(os.Stdout, failures)
	err = HTMLReport("./testdata", failures)
	if err != nil {
		log.Panic(err)
	}

	cnt := make(map[string]int)
	for _, d := range data {
		parts := strings.SplitN(d.ID, IDSep, 3)
		sn := parts[1]
		cnt[sn] = cnt[sn] + 1
	}

	N := float64(len(data))
	fastP := float64(cnt["Fast Suite"]) / N
	slowP := float64(cnt["Slow Suite"]) / N
	slooowP := float64(cnt["Slooow Suite"]) / N
	if fastP < 0.35 || fastP > 0.45 ||
		slowP < 0.35 || slowP > 0.45 ||
		slooowP < 0.15 || slooowP > 0.25 {
		t.Errorf("Bad distribution of scenarios: fast=%f slow=%f slooow=%f",
			fastP, slowP, slooowP)
	}
	fmt.Println("Recorded ", len(data), "points")

	file, err := os.Create("testdata/throughput.csv")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer file.Close()

	DataToCSV(data, file)

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

	raw, err := ParseRawLoadtest("dummy.load", sampleLoadtest)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	global := map[string]string{
		"HOST": ts.URL,
	}

	scenarios := raw.ToScenario(global)
	for i, scen := range scenarios {
		fmt.Printf("%d. %d%% %q (max %d threads)\n",
			i+1, scen.Percentage, scen.RawSuite.Name, scen.MaxThreads)
		if i > 1 {
			logger := log.New(os.Stdout, fmt.Sprintf("Scenario %d: ", i+1), 0)
			scenarios[i].Log = logger
		}
	}

	data, failures, err := Throughput(scenarios, 100, 4*time.Second)
	if err != nil {
		fmt.Println("==> ", err.Error())
	}

	fmt.Println("\n   FAILURES\n=================\n")
	PrintSuiteReport(os.Stdout, failures)
	err = HTMLReport("./testdata", failures)
	if err != nil {
		log.Panic(err)
	}

	cnt := make(map[string]int)
	for _, d := range data {
		parts := strings.SplitN(d.ID, IDSep, 3)
		sn := parts[1]
		cnt[sn] = cnt[sn] + 1
	}

	N := float64(len(data))
	fastP := float64(cnt["Fast Suite"]) / N
	slowP := float64(cnt["Slow Suite"]) / N
	slooowP := float64(cnt["Slooow Suite"]) / N
	if fastP < 0.35 || fastP > 0.45 ||
		slowP < 0.35 || slowP > 0.45 ||
		slooowP < 0.15 || slooowP > 0.25 {
		t.Errorf("Bad distribution of scenarios: fast=%f slow=%f slooow=%f",
			fastP, slowP, slooowP)
	}
	fmt.Println("Recorded ", len(data), "points")

	file, err := os.Create("testdata/throughput.csv")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer file.Close()

	DataToCSV(data, file)

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
`
	ioutil.WriteFile("testdata/throughput.R", []byte(script), 0666)
}
