// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vdobler/ht/errorlist"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/suite"
)

var cmdLoad = &Command{
	RunArgs:     runLoad,
	Usage:       "load [options] <loadtest>",
	Description: "perform a load/throughput test",
	Flag:        flag.NewFlagSet("load", flag.ContinueOnError),
	Help: `Load executes a throughput load test.

The length of the throuput test can be set with the 'duration' command line
flag. The desired target rate of requests/seconds (QPS) is set with the
'rate' command line flag.
The results are collected in the folder setable via the -output flag, the
main performace results are stored to 'live.csv' which can be analysed by
running 'ht stat <live.csv>'.
`,
}

var queryPerSecond float64
var testDuration time.Duration
var rampDuration time.Duration
var collectFrom string
var maxErrorRate float64

func init() {
	cmdLoad.Flag.Float64Var(&queryPerSecond, "rate", 20,
		"make `qps` reqest per second")
	cmdLoad.Flag.DurationVar(&testDuration, "duration", 30*time.Second,
		"duration of throughput test")
	cmdLoad.Flag.DurationVar(&rampDuration, "ramp", 5*time.Second,
		"ramp duration to reach desired request rate")
	cmdLoad.Flag.StringVar(&collectFrom, "collect", "FAIL",
		"collect Test with status at least `limit`")
	cmdLoad.Flag.Float64Var(&maxErrorRate, "errors", 0.9,
		"abort load test if error rate exceeds `rate`")
	addOutputFlag(cmdLoad.Flag)
	addVarsFlags(cmdLoad.Flag)
}

func readRawLoadtest(arg string) (*suite.RawLoadTest, error) {
	// Process arguments of the form <name>@<archive>.
	var fs suite.FileSystem
	if i := strings.Index(arg, "@"); i != -1 {
		blob, err := ioutil.ReadFile(arg[i+1:])
		if err != nil {
			return nil, fmt.Errorf("Cannot load %q: %s\n", arg[i+1:], err)
		}
		fs, err = suite.NewFileSystem(string(blob))
		if err != nil {
			return nil, fmt.Errorf("Cannot load %q: %s\n", arg[i+1:], err)
		}
		arg = arg[:i]
	}
	raw, err := suite.LoadRawLoadtest(arg, fs)
	if err != nil {
		return nil, fmt.Errorf("Cannot load %q: %s\n", arg, err)
	}

	return raw, nil
}

func runLoad(cmd *Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}
	collectStatus := ht.StatusFromString(collectFrom)
	if collectStatus < 0 {
		fmt.Fprintf(os.Stderr, "Unknown status %q.", collectFrom)
		os.Exit(9)
	}

	raw, err := readRawLoadtest(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(9)
	}

	// Prepare scenarios, output folder and the live data log.
	scenarios := raw.ToScenario(variablesFlag)
	bufferedStdout := bufio.NewWriterSize(os.Stdout, 512)
	defer bufferedStdout.Flush()
	for i, scen := range scenarios {
		fmt.Printf("%d. %3d%% %q (max %d threads, verbosity %d)\n",
			i+1, scen.Percentage, scen.RawSuite.Name, scen.MaxThreads,
			scen.Verbosity)
	}

	prepareOutputDir()
	livefile, err := os.Create(filepath.Join(outputDir, "live.csv"))
	if err != nil {
		log.Panic(err)
	}
	defer livefile.Close()

	// Action here.
	prepareHT()
	opts := suite.ThroughputOptions{
		Rate:         queryPerSecond,
		Duration:     testDuration,
		Ramp:         rampDuration,
		CollectFrom:  collectStatus,
		MaxErrorRate: maxErrorRate,
	}
	data, failures, lterr := suite.Throughput(scenarios, opts, livefile)

	if len(data) == 0 && failures == nil && lterr != nil {
		fmt.Fprintf(os.Stderr, "Bad test setup: %s\n", lterr)
		os.Exit(8)
	}

	if failures != nil {
		failures.Name = "Failures of throughput test " + args[0]
	}
	saveLoadtestData(data, failures, scenarios)

	interpretLTerrors(lterr)
}

type sdata struct {
	n                       int
	fail, erred, bogus      int
	min, mean, max, median  time.Duration
	q25, q75, q90, q95, q99 time.Duration
	bad, good               int
	data                    []int // in msec
}

func statsFor(data []suite.TestData) sdata {
	sd := sdata{
		n:    len(data),
		min:  999 * time.Hour,
		mean: time.Duration(0),
		max:  time.Duration(0),
	}
	if sd.n == 0 {
		return sd
	}

	x := make([]time.Duration, len(data))
	msec := make([]int, len(data))
	sum := time.Duration(0)
	for i, d := range data {
		if d.Status == ht.Fail {
			sd.fail++
		} else if d.Status == ht.Error {
			sd.erred++
		} else if d.Status == ht.Bogus {
			sd.bogus++
		}
		x[i] = d.ReqDuration
		sum += d.ReqDuration
		msec[i] = int(d.ReqDuration / 1e6)
		if msec[i] == 0 {
			msec[i] = 1
		}
	}
	sd.bad = sd.fail + sd.erred + sd.bogus
	sd.good = sd.n - sd.bad

	sd.mean = sum / time.Duration(len(data))
	sort.Sort(durationSlice(x))
	sd.min, sd.max = x[0], x[len(x)-1]
	sd.median = quantile(x, 0.5)

	sd.q25, sd.median = quantile(x, 0.25), quantile(x, 0.5)
	sd.q75, sd.q90 = quantile(x, 0.75), quantile(x, 0.90)
	sd.q95, sd.q99 = quantile(x, 0.95), quantile(x, 0.99)
	sd.data = msec
	return sd
}

type durationSlice []time.Duration

func (p durationSlice) Len() int           { return len(p) }
func (p durationSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p durationSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func quantile(x []time.Duration, p float64) time.Duration {
	N := float64(len(x))
	if N == 0 {
		return 0
	} else if N == 1 {
		return x[0]
	}

	if p < 2.0/(3.0*(N+1.0/3.0)) {
		return x[0]
	}
	if p >= (N-1.0/3.0)/(N+1.0/3.0) {
		return x[len(x)-1]
	}

	h := (N+1.0/3.0)*p + 1.0/3.0
	fh := math.Floor(h)
	xl := x[int(fh)-1]
	xr := x[int(fh)]

	return time.Duration(float64(xl) + (h-fh)*float64(xr-xl))
}

func interpretLTerrors(lterr error) {
	if lterr == nil {
		fmt.Println("OKAY")
		os.Exit(0)
	}

	fmt.Println("Problems running this throughpout tests:")
	if el, ok := lterr.(errorlist.List); ok {
		for _, msg := range el.AsStrings() {
			fmt.Println("    ", msg)
		}
	} else {
		fmt.Println("  ", lterr.Error())
	}

	fmt.Println("PROBLEMS")
	os.Exit(1)
}

func saveLoadtestData(data []suite.TestData, failures *suite.Suite, scenarios []suite.Scenario) {
	if failures != nil {
		err := suite.HTMLReport(outputDir, failures)
		if err != nil {
			log.Panic(err)
		}
	}

	file, err := os.Create(outputDir + "/throughput.csv")
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	suite.DataToCSV(data, file)
	script := `
library(ggplot2)
d <- read.csv("throughput.csv")
d$Status <- factor(d$Status, levels <- c("NotRun", "Skipped", "Pass", "Fail", "Error", "Bogus"))

myColors <- c("#999999", "#ffff00", "#339900", "#660000", "#ff0000", "#ff3399")
names(myColors) <- levels(d$Status)
colScale <- scale_colour_manual(name = "status",values = myColors)
fillScale <- scale_fill_manual(name = "status",values = myColors)

shift <- function(x, lag) {
    n <- length(x)
    xnew <- rep(NA, n)
    xnew[(lag+1):n] <- x[1:(n-lag)]
    return(xnew)
}

p <- ggplot(d, aes(x=Elapsed, y=Rate))
p <- p + geom_point(size=3) + geom_smooth()
p <- p + xlab("Elapsed [ms]") + ylab("Requests/Second (QPS)")
ggsave("rate.png", plot=p, width=10, height=8, dpi=100)

d$Delta <- c(d$Elapsed[2:length(d$Elapsed)] - d$Elapsed[1:length(d$Elapsed)-1], NA)
p <- ggplot(d, aes(x=Elapsed, y=Delta)) + geom_point(size=2) + xlab("Elapsed [ms]")
ggsave("delta.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=ReqDuration, colour=Test))
p <- p + geom_point(size=3) + xlab("Elapsed [ms]") + ylab("Request Duration [ms]")
ggsave("scatter.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Test, y=ReqDuration))
p <- p + geom_boxplot() + ylab("Request Duration [ms]")
p <- p + theme(axis.text.x = element_text(angle = -30, hjust = 0))
ggsave("boxplot.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Test, y=ReqDuration)) + scale_y_log10()
p <- p + annotation_logticks(sides = "l")
p <- p + geom_boxplot() + ylab("Request Duration [ms]")
p <- p + theme(axis.text.x = element_text(angle = -30, hjust = 0))
ggsave("logboxplot.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=ReqDuration, colour=Status))
p <- p + geom_point(size=3) + xlab("Elapsed [ms]")  + ylab("Request Duration [ms]")
p <- p + colScale
ggsave("status.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=ConcTot, colour=Status)) + colScale
p <- p + geom_point(size=3) + xlab("Elapsed [ms]") + ylab("Total Concurrency")
ggsave("conctot.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=Elapsed, y=ConcOwn, colour=Status)) + colScale
p <- p + facet_grid(Test ~ ., scales="free_y")
p <- p + geom_point(size=3) + xlab("Elapsed [ms]")
p <- p + theme(strip.text.y = element_text(angle=0))
ggsave("concown.png", plot=p, width=10, height=8, dpi=100)

p <- ggplot(d, aes(x=ReqDuration, fill=Status))
p <- p + geom_histogram(binwidth=3) + xlab("Request Duration [ms]")
p <- p + facet_grid(Test ~ ., scales="free_y")
p <- p + theme(strip.text.y = element_text(angle=0))
p <- p + fillScale
ggsave("hist.png", plot=p, width=10, height=8, dpi=100)


`
	ioutil.WriteFile(outputDir+"/throughput.R", []byte(script), 0666)

}
