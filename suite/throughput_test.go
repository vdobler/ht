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

	suites := []*RawSuite{slow, fast}
	globals := map[string]string{
		"URL": ts.URL,
	}

	data := Throughput(suites, 200, 10*time.Second, globals)

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
p <- p + geom_point()
ggsave("scatter.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=ReqDuration, colour=Status))
p <- p + geom_point()
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
