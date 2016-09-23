// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"fmt"
	"io/ioutil"
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
	if rand.Intn(100) < 10 {
		status = http.StatusBadRequest
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
{
    Name: Fast Suite
    Main: [
        { File: "testX.ht" }
        { File: "testY.ht" }
    ]
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
    Request: { URL: "{{URL}}?mean=25&stdd=5" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}
`
var tpSuiteSlooow = `
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
    Request: { URL: "{{URL}}?mean=120&stdd=2" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}

# testH.ht
{
    Name: Test H
    Request: { URL: "{{URL}}?mean=110&stdd=3" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}

# testI.ht
{
    Name: Test I
    Request: { URL: "{{URL}}?mean=100&stdd=4" }
    Checks: [ {Check: "StatusCode", Expect: 200} ]
}
`

func TestThroughput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(waitHandler))
	defer ts.Close()

	slow, err := ParseRawSuite(tpSuiteSlow)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	fast, err := ParseRawSuite(tpSuiteFast)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	slooow, err := ParseRawSuite(tpSuiteSlooow)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	scenarios := []Scenario{
		Scenario{RawSuite: fast, Percentage: 40},
		Scenario{RawSuite: slow, Percentage: 40},
		Scenario{RawSuite: slooow, Percentage: 20},
	}

	globals := map[string]string{
		"URL": ts.URL,
	}

	data, err := Throughput(scenarios, 75, 3*time.Second, globals)
	if err != nil {
		fmt.Println("==> ", err.Error())
	}

	cnt := make(map[string]int)
	for _, d := range data {
		part := strings.SplitN(d.ID, " ", 2)
		Q := strings.SplitN(part[1], "\u2237", 2)
		cnt[Q[0]] = cnt[Q[0]] + 1
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
