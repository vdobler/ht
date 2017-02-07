package asciistat

import (
	"fmt"
	"os"
	"testing"
)

func TestPlot(t *testing.T) {
	data1 := Data{
		Name: "First",
		Values: []int{200, 220, 240, 250, 260, 270, 280, 290, 300,
			320, 340, 360, 380, 400, 420, 440, 460, 480, 500},
	}

	data2 := Data{
		Name: "Second",
		Values: []int{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110,
			200, 250, 300, 400, 401, 402, 402, 403, 404, 405, 406, 407, 408},
	}

	Plot(os.Stdout, []Data{data1, data2}, "s", false, 100)
}

func TestLinScale(t *testing.T) {
	data := Data{
		Name:   "Data",
		Values: []int{400, 401, 402, 403},
	}
	for n := 0; n < 50; n++ {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			Plot(os.Stdout, []Data{data}, "ms", false, 100)
		})
		for i, v := range data.Values {
			data.Values[i] = int(float64(v) * 1.4)
		}
	}
}

func TestLogScale(t *testing.T) {
	data := Data{
		Name:   "Data",
		Values: []int{4, 5, 6, 20, 30, 50, 90},
	}
	for n := 0; n < 20; n++ {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			Plot(os.Stdout, []Data{data}, "ms", true, 100)
		})
		for i, v := range data.Values {
			data.Values[i] = int(float64(v) * 1.4)
		}
	}
}

func TestNicelabel(t *testing.T) {
	for n := 1; n < 99999999; n *= 2 {
		fmt.Println(nicelabel(n, 100))
	}
}
