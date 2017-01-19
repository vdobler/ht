// Package hist provides spark-line type histograms
package hist

import (
	"fmt"
	"io"
	"math"
)

type Histogram struct {
	Name string
	Data []uint32 // musec
}

func (h Histogram) MinMax() (uint32, uint32) {
	if len(h.Data) == 0 {
		return 1, 1
	}
	min, max := h.Data[0], h.Data[0]
	for _, d := range h.Data {
		if d < min {
			min = d
		} else if d > max {
			max = d
		}
	}
	return min, max
}

func roundUp(max uint32) uint32 {
	logmax := math.Log10(float64(max))
	lmi := math.Floor(logmax)
	lmr := logmax - lmi
	var f uint32
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
	return f * uint32(math.Pow10(int(lmi))) // BUG: may overflow
}

func roundDown(max uint32) uint32 {
	logmax := math.Log10(float64(max))
	lmi := math.Floor(logmax)
	lmr := logmax - lmi
	var f uint32
	if lmr > 0.698 {
		f = 5
	} else if lmr > 0.477 {
		f = 3
	} else if lmr > 0.301 {
		f = 2
	} else {
		f = 1
	}
	return f * uint32(math.Pow10(int(lmi))) // BUG: may underflow
}

func fmtMs(ms uint32) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	} else if ms < 10000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%ds", ms/1000)
}

var (
	blocks = []rune{'\u2581', '\u2582', '\u2583', '\u2584',
		'\u2585', '\u2586', '\u2587', '\u2588'}
)

const bins = 100

func PrintLogHistograms(out io.Writer, hists []Histogram) {
	min, max := uint32(math.MaxUint32), uint32(0) // global minium and maximum
	labelLength := 0
	for _, hist := range hists {
		hmin, hmax := hist.MinMax()
		if hmin < min {
			min = hmin
		}
		if hmax > max {
			max = hmax
		}
		if n := len(hist.Name); n > labelLength {
			labelLength = n
		}
	}
	min, max = roundDown(min), roundUp(max)
	legend := fmt.Sprintf("%s -- %s", fmtMs(min), fmtMs(max))
	if n := len(legend); n > labelLength {
		labelLength = n
	}
	if min < 1 {
		min = 1 // otherwise sparkline degenerates due to log
	}
	logmin := math.Log(float64(min))
	delta := (math.Log(float64(max)) - logmin) / bins

	for _, hist := range hists {
		cnt := make([]int, bins)
		maxcnt := 1
		// Count
		for _, t := range hist.Data {
			bin := int((math.Log(float64(t)) - logmin) / delta)
			if bin < 0 {
				bin = 0
			} else if bin >= bins {
				bin = bins - 1
			}
			cnt[bin]++
			if cnt[bin] > maxcnt {
				maxcnt = cnt[bin]
			}
		}
		// Print
		fmt.Fprintf(out, "%*s ", labelLength, hist.Name)
		for i := 0; i < bins; i++ {
			v := cnt[i] * 7 / maxcnt
			fmt.Fprintf(out, "%c", blocks[v])
		}
		fmt.Fprintln(out)
	}

	// Scale

	fmt.Fprintf(out, "%*s ", labelLength, legend)
	cnt := make([]int, bins)
	for pt := uint32(2); pt <= max; pt *= 10 {
		bin := int((math.Log(float64(pt)) - logmin) / delta)
		if bin < 0 || bin >= bins {
			continue
		}
		cnt[bin] = 2
	}
	for pt := uint32(3); pt <= max; pt *= 10 {
		bin := int((math.Log(float64(pt)) - logmin) / delta)
		if bin < 0 || bin >= bins {
			continue
		}
		cnt[bin] = 3
	}
	for pt := uint32(5); pt <= max; pt *= 10 {
		bin := int((math.Log(float64(pt)) - logmin) / delta)
		if bin < 0 || bin >= bins {
			continue
		}
		cnt[bin] = 5
	}
	for pt := uint32(1); pt <= max; pt *= 10 {
		bin := int((math.Log(float64(pt)) - logmin) / delta)
		if bin < 0 || bin >= bins {
			continue
		}
		cnt[bin] = 7
	}
	for i := 0; i < bins; i++ {
		v := cnt[i]
		fmt.Fprintf(out, "%c", blocks[v])
	}
	fmt.Fprintln(out)
}
