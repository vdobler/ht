package hjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func getContent(file string) []byte {
	if data, err := ioutil.ReadFile(file); err != nil {
		panic(err)
	} else {
		return data
	}
}

func getTestContent(name string) []byte {
	p := fmt.Sprintf("./assets/%s_test.hjson", name)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		p = fmt.Sprintf("./assets/%s_test.json", name)
	}
	return getContent(p)
}

func getResultContent(name string) ([]byte, []byte) {
	p1 := fmt.Sprintf("./assets/sorted/%s_result.json", name)
	p2 := fmt.Sprintf("./assets/sorted/%s_result.hjson", name)
	return getContent(p1), getContent(p2)
}

func fixJSON(data []byte) []byte {
	data = bytes.Replace(data, []byte("\\u003c"), []byte("<"), -1)
	data = bytes.Replace(data, []byte("\\u003e"), []byte(">"), -1)
	data = bytes.Replace(data, []byte("\\u0026"), []byte("&"), -1)
	data = bytes.Replace(data, []byte("\\u0008"), []byte("\\b"), -1)
	data = bytes.Replace(data, []byte("\\u000c"), []byte("\\f"), -1)
	return data
}

func run(t *testing.T, file string) {
	name := strings.TrimSuffix(file, "_test"+filepath.Ext(file))
	t.Logf("running %s", name)
	shouldFail := strings.HasPrefix(file, "fail")

	testContent := getTestContent(name)
	var data interface{}
	if err := Unmarshal(testContent, &data); err != nil {
		if !shouldFail {
			panic(err)
		} else {
			return
		}
	} else if shouldFail {
		panic(errors.New(name + " should_fail!"))
	}

	rjson, rhjson := getResultContent(name)

	opt := EncoderOptions{}
	opt.Eol = "\n"
	opt.BracesSameLine = false
	opt.EmitRootBraces = true
	opt.QuoteAlways = false
	opt.IndentBy = "  "
	opt.AllowMinusZero = false
	opt.UnknownAsNull = false

	actualHjson, _ := MarshalWithOptions(data, opt)
	actualJSON, _ := json.MarshalIndent(data, "", "  ")
	actualJSON = fixJSON(actualJSON)

	// add fixes where go's json differs from javascript
	switch name {
	case "kan":
		actualJSON = []byte(strings.Replace(string(actualJSON), "    -0,", "    0,", -1))
	case "pass1":
		actualJSON = []byte(strings.Replace(string(actualJSON), "1.23456789e+09", "1234567890", -1))
	}

	hjsonOK := bytes.Equal(rhjson, actualHjson)
	jsonOK := bytes.Equal(rjson, actualJSON)
	if !hjsonOK {
		t.Logf("%s\n---hjson expected\n%s\n---hjson actual\n%s\n---\n", name, rhjson, actualHjson)
	}
	if !jsonOK {
		t.Logf("%s\n---json expected\n%s\n---json actual\n%s\n---\n", name, rjson, actualJSON)
	}
	if !hjsonOK || !jsonOK {
		panic("fail!")
	}
}

func TestHjson(t *testing.T) {

	files := strings.Split(string(getContent("assets/testlist.txt")), "\n")

	for _, file := range files {
		run(t, file)
	}
}

func TestIntegers(t *testing.T) {
	for i, tc := range []struct {
		in   string
		want int64
	}{
		{"-1", -1},
		{"-0", 0},
		{"0", 0},
		{"1", 1},
		{"123", 123},
		{"-234", -234},
		{"9223372036854775807", 9223372036854775807},
		{"-9223372036854775806", -9223372036854775806},
	} {
		var data interface{}
		if err := Unmarshal([]byte(tc.in), &data); err != nil {
			t.Errorf("Unexpected error %s", err)
		}
		if n, ok := data.(int64); !ok {
			t.Errorf("Bad type %T", data)
		} else if n != tc.want {
			t.Errorf("%d. %q: Got %d, want %d", i, tc.in, n, tc.want)
		}
	}

	// Too large for int --> uint !
	var data interface{}
	if err := Unmarshal([]byte("9223372036854775808"), &data); err != nil {
		t.Errorf("Unexpected error %s", err)
	}
	if n, ok := data.(uint64); !ok {
		t.Errorf("Bad type %T", data)
	} else if n != 9223372036854775808 {
		t.Errorf("Got %d", n)
	}
}

func TestStructEncoding(t *testing.T) {
	type T struct {
		B int
		A string
		C []float64
	}

	v := T{B: 123, A: "foo\nbar\n  wuz  ", C: []float64{3.141, 2.718}}

	opt := EncoderOptions{}
	opt.Eol = "\n"
	opt.BracesSameLine = true
	opt.EmitRootBraces = true
	opt.QuoteAlways = false
	opt.IndentBy = "    "
	opt.AllowMinusZero = false
	opt.UnknownAsNull = false

	data, err := MarshalWithOptions(v, opt)
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}
	fmt.Println(string(data))
}
