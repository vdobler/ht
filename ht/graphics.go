// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"os"
	"os/exec"
)

// RTHistogram produces a png file containing a histogram of the int values in
// data. Several histograms can be plotted, the keys of data are the labels.
// If dodge is true the histogram bars are not stacked.
func RTHistogram(title string, data map[string][]int, dodge bool, filename string) error {
	file, err := os.Create(filename + ".R")
	if err != nil {
		return err
	}
	defer file.Close()
	fmt.Fprintf(file, "library(ggplot2)\n")

	// The default of 30 bin in ggplot2 is too high for most
	// of our mesurements: Reduce if only a few measurements are present
	min, max, fewest, most := 9999999999, -9999999999, 9999999999, 0
	for _, rt := range data {
		if len(rt) < fewest {
			fewest = len(rt)
		} else if len(rt) > most {
			most = len(rt)
		}
		for _, n := range rt {
			if n < 0 {
				// Negative values are for failed test, ignore
				// for range calculation.
				continue
			}
			if n < min {
				min = n
			} else if n > max {
				max = n
			}
		}
	}
	binwidth := optimumBinBiwdth(min, max, fewest, most)
	fmt.Fprintf(file, "ResponseTime <- c(")
	label := ""
	first := true
	for row, rt := range data {
		if label == "" {
			label += fmt.Sprintf(`Label <- c(rep("%s", times=%d)`, row, len(rt))
		} else {
			label += fmt.Sprintf(`, rep("%s", times=%d)`, row, len(rt))
		}

		for _, n := range rt {
			if first {
				fmt.Fprintf(file, "%d", n)
				first = false
			} else {
				fmt.Fprintf(file, ", %d", n)
			}
		}
		fmt.Fprintf(file, "\n")
	}
	fmt.Fprintf(file, ")\n")
	label += ")"
	fmt.Fprintf(file, "%s\n", label)
	fmt.Fprintf(file, "rt.data <- data.frame(Label, ResponseTime)\n")

	fmt.Fprintf(file, "p <- ggplot(rt.data, aes(x=ResponseTime, fill=Label))\n")
	fmt.Fprintf(file, "png(%q)\n", filename)
	pos := ""
	if dodge {
		pos = `, position="dodge"`
	}
	fmt.Fprintf(file, "print(p + geom_histogram(binwidth=%d%s) + ggtitle(%q))\n",
		binwidth, pos, title)
	fmt.Fprintf(file, "dev.off()\n")
	file.Close()

	args := []string{filename + ".R"}
	cmd := exec.Command(rScriptPath, args...)
	err = cmd.Run()
	return err
}

const rScriptPath = `/usr/bin/Rscript`

// IsRScriptInstalled returns true if Rscript is available
func IsRScriptInstalled() bool {
	fi, err := os.Stat(rScriptPath)
	return !os.IsNotExist(err) && fi.Size() > 0
}

func optimumBinBiwdth(min, max, fewest, most int) int {
	n := (fewest + most) / 2
	rng := float64(max - min)
	if n > 50 {
		rng /= 30
	} else if n > 20 {
		rng /= 15
	} else {
		rng /= 10
	}
	p := 1
	for rng > 10 {
		rng /= 10
		p *= 10
	}
	switch {
	case rng < 1.5:
		return p
	case rng < 4:
		return 2 * p
	case rng < 8:
		return 5 * p
	default:
		return 10 * p
	}
}
