// Package asciistat draws boxplot like statistics as ASCII art.
//
//
//    Sensible:              --------[------M---------]------|--)-----
//       |------+----+--|--------------|--------------|--------------|--------------|
//      1ms            10ms          100ms           1s             10s            100s
//
//    Legend:
//       Full range indicated by  -----------------------------
//       Median indicated by      --------------M--------------
//       25% to 75% indicated by  --------[-----M--]-----------
//       90% indicated by         --------[-----M--]-----|-----
//       98% indicated by         --------[-----M--]-----|--)--
//
// Works well for data raning from 1 to 100*1e9 if the range is not to small.
//
package asciistat

import (
	"fmt"
	"io"
	"math"
	"sort"
)

var symbols = []struct {
	p float64
	r rune
}{
	{0.50, 'M'},
	{0.25, '['},
	{0.75, ']'},
	{0.98, '}'},
	{0.99, '>'},
	{0.90, '|'},
	{0.95, ')'},
}

// Percentil represents a percentil value.
type Percentil struct {
	P float64 // Percentil in [0, 1]
	V float64 // Value
	S rune    // Symbol to display
}

// Data is a named list of integers.
type Data struct {
	Name   string
	Values []int
}

// https://en.wikipedia.org/wiki/Quantile formula R-8
func quantile(x []int, p float64) float64 {
	N := float64(len(x))
	if N == 0 {
		return 0
	} else if N == 1 {
		return float64(x[0])
	}
	if p < 2.0/(3.0*(N+1.0/3.0)) {
		return float64(x[0])
	}
	if p >= (N-1.0/3.0)/(N+1.0/3.0) {
		return float64(x[len(x)-1])
	}

	h := (N+1.0/3.0)*p + 1.0/3.0
	fh := math.Floor(h)
	xl := x[int(fh)-1]
	xr := x[int(fh)]

	return float64(xl) + (h-fh)*float64(xr-xl)
}

const maxInt = int(^uint(0) >> 1)
const minInt = -maxInt - 1

// Plot the given measurement to w. The full (labels+graph) plot has the given
// width. If log than a logarithmic axis is used if possible.
func Plot(w io.Writer, measurements []Data, unit string, log bool, width int) {
	// Sort input data end determine overall data range.
	min, max := maxInt, minInt
	labelLen := 0
	for i := range measurements {
		if k := len(measurements[i].Name); k > labelLen {
			labelLen = k
		}

		if len(measurements[i].Values) == 0 {
			continue
		}
		sort.Ints(measurements[i].Values)
		if measurements[i].Values[0] < min {
			min = measurements[i].Values[0]
		}
		n := len(measurements[i].Values) - 1
		if measurements[i].Values[n] > max {
			max = measurements[i].Values[n]
		}
	}
	if min == max {
		min -= 1
		max += 1
	}

	if min == 0 || max == 0 || min*max < 0 {
		log = false
	}

	if log {
		plotLog(w, measurements, unit, width, labelLen, min, max)
	} else {
		plotLin(w, measurements, unit, width, labelLen, min, max)
	}
	fmt.Fprintln(w, "Percentils:  [=25,  M=50,  ]=75,  |=90,  )=95,  }=98,  >=99")
}

func plotData(w io.Writer, measurements []Data, screen func(x float64) int, dWidth, labelLen int) {
	// Plot all measurements.
	for _, m := range measurements {
		fmt.Fprintf(w, "%*s:  ", labelLen, m.Name)
		b := make([]rune, dWidth)
		for i := range b {
			b[i] = ' '
		}
		a, e := screen(float64(m.Values[0])), screen(float64(m.Values[len(m.Values)-1]))
		for i := a; i < e; i++ {
			b[i] = '-'
		}
		for _, sym := range symbols {
			q := quantile(m.Values, sym.p)
			i := screen(q)
			b[i] = sym.r
		}
		fmt.Fprintln(w, string(b))
	}
}

// ----------------------------------------------------------------------------
// Logarithmic scales

func plotLog(w io.Writer, measurements []Data, unit string, width, labelLen int, min, max int) error {
	min, max = roundDown(min), roundUp(max)
	max += max / 20
	dWidth := width - labelLen - 5
	logmin, logmax := math.Log(float64(min)), math.Log(float64(max))
	logRange := logmax - logmin

	screen := func(x float64) int {
		x = math.Log(x)
		x -= logmin
		x /= logRange
		return int(x*float64(dWidth) + 0.5)
	}

	plotData(w, measurements, screen, dWidth, labelLen)

	// Plot scale.
	b := make([]rune, dWidth+4) // 2 extra on left and right
	t := make([]rune, dWidth+4) // labels
	for i := range b {
		b[i] = '-'
		t[i] = ' '
	}

	for x := 1; x <= max; x *= 10 {
		for mtics := 1; mtics <= 3; mtics++ {
			i := screen(float64(mtics*x)) + 2 // offset from above
			if i < 0 || i >= len(b) {
				continue
			}

			b[i] = '+'
			if mtics == 1 ||
				(mtics == 2 && logRange < 4) ||
				(mtics == 3 && logRange < 3) {
				label := niceloglabel(mtics * x)
				ll := len(label)
				if i-ll/2 >= 0 && i-ll/2+ll < len(t) {
					for j, r := range label {
						t[i-ll/2+j] = r
					}
				}
			}
		}
	}
	fmt.Fprintf(w, "%*s ", labelLen, "")
	fmt.Fprintln(w, string(b))
	fmt.Fprintf(w, "%*s ", labelLen, "["+unit+"]")
	fmt.Fprintln(w, string(t))

	return nil
}

func roundUp(max int) int {
	logmax := math.Log10(float64(max))
	lmi := math.Floor(logmax)
	lmr := logmax - lmi
	var f int
	if lmr < 0.002 {
		f = 1
	} else if lmr < 0.301 {
		f = 2
	} else if lmr < 0.477 {
		f = 3
	} else if lmr < 0.698 {
		f = 5
	} else {
		f = 10
	}
	return f * int(math.Pow10(int(lmi))) // BUG: may overflow
}

func roundDown(max int) int {
	logmax := math.Log10(float64(max))
	lmi := math.Floor(logmax)
	lmr := logmax - lmi
	var f int
	if lmr > 0.698 {
		f = 5
	} else if lmr > 0.477 {
		f = 3
	} else if lmr > 0.301 {
		f = 2
	} else {
		f = 1
	}
	return f * int(math.Pow10(int(lmi))) // BUG: may underflow
}

func niceloglabel(x int) string {
	if x < 1e3 {
		return fmt.Sprintf("%d", x)
	} else if x < 1e6 {
		return fmt.Sprintf("%dk", x/1e3)
	} else if x < 1e9 {
		return fmt.Sprintf("%dM", x/1e6)
	} else if x < 1e6 {
		return fmt.Sprintf("%dG", x/1e9)
	}
	return fmt.Sprintf("%dT", x/1e12)
}

// ----------------------------------------------------------------------------
// Linear scales

func plotLin(w io.Writer, measurements []Data, unit string, width, labelLen int, min, max int) error {
	// Data to screen coordinates.
	dRange := float64(max - min)
	dWidth := width - labelLen - 5
	gap := float64(dRange) / 20
	screen := func(x float64) int {
		x -= float64(min) - gap
		x /= (dRange + 2*gap)
		return int(x*float64(dWidth) + 0.5)
	}

	plotData(w, measurements, screen, dWidth, labelLen)

	// Plot scale.
	b := make([]rune, dWidth+4) // 2 extra left and right
	t := make([]rune, dWidth+4) // labels
	for i := range b {
		b[i] = '-'
		t[i] = ' '
	}
	// Tick labels are of the form .01, 1, 10, 100, 1e3, 1e4 so they are 3 runes wide.
	// Tics need to be spaced "  0.3  " so a label is 9 rune wide, except first and
	// last which are only 5 wide. So n lables are  2*5 + 9*(n-2)  wide which must
	// fit into dWidth:
	//    dWidth >= 2*5 + 9*(n-2)
	//    dWidth-10 >= 9*(n-2)
	//    (dWidth-10)/9 >= n - 2
	//    n <= (dWidth-10)/9 + 2
	numLab := int(float64(dWidth-10)/9 + 2)
	step := (dRange + 2*gap) / float64(numLab-1)
	// Increase step to be of the form {1,2,5} * 10^n:
	n := int(math.Log10(step))
	ord := int(math.Pow10(n))
	step /= float64(ord)
	var delta int
	if step <= 1.2 {
		delta = ord
	} else if step <= 2.4 {
		delta = 2 * ord
	} else if step < 8 {
		delta = 5 * ord
	} else {
		ord *= 10
		delta = ord
	}

	for x := delta * (min / delta); x <= max; x += delta {
		i := screen(float64(x)) + 2 // offset from above
		if i < 0 || i >= len(b) {
			continue
		}
		b[i] = '+'
		label := nicelabel(x, ord)
		ll := len(label)
		if i-ll/2 >= 0 && i-ll/2+ll < len(t) {
			for j, r := range label {
				t[i-ll/2+j] = r
			}
		}
	}
	fmt.Fprintf(w, "%*s ", labelLen, "")
	fmt.Fprintln(w, string(b))
	fmt.Fprintf(w, "%*s ", labelLen, "["+unit+"]")
	fmt.Fprintln(w, string(t))

	return nil
}

func nicelabel(x int, ord int) string {
	if x <= 9999 || ord == 1 {
		return fmt.Sprintf("%d", x)
	}

	unit := ""
	scale := 1
	if x < 1e6 {
		unit = "k"
		scale = 1e3
	} else if x < 1e9 {
		unit = "M"
		scale = 1e6
	} else if x < 1e12 {
		unit = "G"
		scale = 1e9
	} else {
		return fmt.Sprintf("%g", float64(x))
	}

	if ord >= scale {
		return fmt.Sprintf("%d%s", x/scale, unit)
	} else if ord >= scale/10 {
		return fmt.Sprintf("%.1f%s", float64(x)/float64(scale), unit)
	} else if ord >= scale/100 {
		return fmt.Sprintf("%.2f%s", float64(x)/float64(scale), unit)
	} else if ord >= scale/1000 {
		return fmt.Sprintf("%.3f%s", float64(x)/float64(scale), unit)
	}

	return fmt.Sprintf("%d", x)

}
