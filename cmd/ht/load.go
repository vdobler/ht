// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/bender"
	"github.com/vdobler/ht/suite"
)

var cmdLoad = &Command{
	RunArgs:     runLoad,
	Usage:       "load [options] <loadtest>",
	Description: "run suites in a load test",
	Flag:        flag.NewFlagSet("load", flag.ContinueOnError),
	Help: `
Execute a throughput test with the given suite.
	`,
}

var queryPerSecond float64
var testDuration time.Duration

func init() {
	cmdLoad.Flag.Float64Var(&queryPerSecond, "rate", 20,
		"make `qps` reqest per second")
	cmdLoad.Flag.DurationVar(&testDuration, "duration", 30*time.Second,
		"duration of throughput test")
	addOutputFlag(cmdLoad.Flag)
}

func runLoad(cmd *Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

	raw, err := suite.LoadRawLoadtest(args[0], nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot load %q: %s\n", args[0], err)
		os.Exit(9)
	}

	scenarios := raw.ToScenario(variablesFlag)
	for i, scen := range scenarios {
		fmt.Printf("%d. %d%% %q (max %d threads)\n",
			i+1, scen.Percentage, scen.RawSuite.Name, scen.MaxThreads)
		logger := log.New(os.Stdout, fmt.Sprintf("Scenario %d %q: ", i+1, scen.Name), 0)
		scenarios[i].Log = logger
	}

	prepareHT()
	data, failures, lterr := suite.Throughput(scenarios, queryPerSecond, testDuration)

	if outputDir == "" {
		outputDir = time.Now().Format("2006-01-02_15h04m05s")
	}
	err = os.MkdirAll(outputDir, 0766)
	if err != nil {
		log.Panic(err)
	}
	failures.Name = "Failures of throughput test " + args[0]

	saveLoadtestData(data, failures)
	printStatistics(scenarios, data)
	interpretLTerrors(lterr)
}

func printStatistics(scenarios []suite.Scenario, data []bender.TestData) {
	// Per testfile
	pert := make(map[string][]bender.TestData)
	for _, d := range data {
		parts := strings.Split(d.ID, suite.IDSep)
		nums := strings.Split(parts[0], "/")
		s, _ := strconv.Atoi(nums[0])
		t, _ := strconv.Atoi(nums[3])
		name := scenarios[s-1].RawSuite.RawTests()[t-1].File.Name
		pert[name] = append(pert[name], d)
	}
	for file, fd := range pert {
		st := statsFor(fd)
		h := fmt.Sprintf("Testfile %q:", file)
		printStat(h, st)

	}

	// Per scenario
	for i, s := range scenarios {
		fd := make([]bender.TestData, 0, 200)
		pat := fmt.Sprintf("%d/", i+1)
		for _, d := range data {
			if strings.HasPrefix(d.ID, pat) {
				fd = append(fd, d)
			}
		}

		st := statsFor(fd)
		h := fmt.Sprintf("Scenario %d %q:", i+1, s.Name)
		printStat(h, st)
	}

	// All requests
	st := statsFor(data)
	printStat("All request:", st)

	/*
		// Per (scenario/test)
		for i, s := range scenarios {
			for j, t := range s.RawSuite.RawTests() {
				fd := make([]bender.TestData, 0, 200)
				ppat := fmt.Sprintf("%d/", i+1)
				spat := fmt.Sprintf("/%d%s", j+1, suite.IDSep)
				for _, d := range data {
					// fmt.Printf("ID = %s   %s  %s\n", d.ID, ppat, spat)
					if strings.HasPrefix(d.ID, ppat) &&
						strings.Contains(d.ID, spat) {
						fd = append(fd, d)
					}
				}
				st := statsFor(fd)
				fmt.Printf("Scenario %d %q, Test %d %q:\n",
					i+1, s.Name,
					j+1, t.File.Name,
				)
				printStat(st)
			}
		}
	*/

}

func printStat(headline string, st sdata) {
	fmt.Printf("%s Status:   Total=%d  Pass=%d (%.1f%%), Fail=%d (%.1f%%), Error=%d (%.1f%%), Bogus=%d (%.1f%%)\n",
		headline, st.n,
		st.good, 100*float64(st.good)/float64(st.n),
		st.fail, 100*float64(st.fail)/float64(st.n),
		st.erred, 100*float64(st.erred)/float64(st.n),
		st.bogus, 100*float64(st.bogus)/float64(st.n),
	)
	fmt.Printf("%s Duration: 0%%=%.1fms, 25%%=%.1fms, 50%%=%.1fms, 75%%=%.1fms, 90%%=%.1fms, 95%%=%.1fms, 99%%=%.1fms, 100%%=%.1fms\n",
		headline,
		float64(st.min/1000)/1000,
		float64(st.q25/1000)/1000,
		float64(st.median/1000)/1000,
		float64(st.q75/1000)/1000,
		float64(st.q90/1000)/1000,
		float64(st.q95/1000)/1000,
		float64(st.q99/1000)/1000,
		float64(st.max/1000)/1000,
	)
}

type sdata struct {
	n                       int
	fail, erred, bogus      int
	min, mean, max, median  time.Duration
	q25, q75, q90, q95, q99 time.Duration
	bad, good               int
}

func statsFor(data []bender.TestData) sdata {
	sd := sdata{
		n:    len(data),
		min:  99 * time.Hour,
		mean: time.Duration(0),
		max:  time.Duration(0),
	}
	if sd.n == 0 {
		return sd
	}

	x := make([]time.Duration, len(data))
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
	return sd
}

type durationSlice []time.Duration

func (p durationSlice) Len() int           { return len(p) }
func (p durationSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p durationSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func quantile(x []time.Duration, p float64) time.Duration {
	N := float64(len(x))
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
	if el, ok := lterr.(ht.ErrorList); ok {
		for _, msg := range el.AsStrings() {
			fmt.Println("    ", msg)
		}
	} else {
		fmt.Println("  ", lterr.Error())
	}

	fmt.Println("PROBLEMS")
	os.Exit(1)
}

func saveLoadtestData(data []bender.TestData, failures *suite.Suite) {
	err := suite.HTMLReport(outputDir, failures)
	if err != nil {
		log.Panic(err)
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
	ioutil.WriteFile(outputDir+"/throughput.R", []byte(script), 0666)

}
