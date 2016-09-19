// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// resilience.go contains resilience checking.

package ht

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func init() {
	RegisterCheck(Resilience{})
}

// ----------------------------------------------------------------------------
// Resilience

// Methods: GET, POST, HEAD, PUT, DELETE, PATCH, OPTIONS
// Different Parameter Transmissions: query/URL, x-www-form-urlencoded and multipart
// Different Header Values
// Different Parameter Values
// Value changes:

// Resilience checks the resilience of an URL against unexpected requests like
// different HTTP methods, changed or garbled parameters, different parameter
// transmission types and changed or garbled HTTP headers.
//
// Parameters and Header values can undergo several different types of
// modifications
//   * all:       all the individual modifications below (excluding 'space'
//                for HTTP headers)
//   * drop:      don't send at all
//   * none:      don't modify the individual parameters or header but
//                don't send any parameters or headers
//   * double:    send same value two times
//   * twice:     send two different values (original and "extraValue")
//   * change:    change a single character (first, middle and last one)
//   * delete:    drop single character (first, middle and last one)
//   * nonsense:  the values "p,f1u;p5c:h*", "hubba%12bubba(!" and "   "
//   * space:     the values " ", "       ", "\t", "\n", "\r", "\v", "\u00A0",
//                "\u2003", "\u200B", "\x00\x00", and "\t \v \r \n "
//   * malicious: the values "\uFEFF\u200B\u2029", "ʇunpᴉpᴉɔuᴉ",
//                "http://a/%%30%30" and "' OR 1=1 -- 1"
//   * user       use user defined values from Values
//   * empty:     ""
//   * type:      change the type (if obvious)
//       - "1234"     -->  "wwww"
//       - "3.1415"   -->  "wwwwww"
//       - "i@you.me" -->  "iXyouYme"
//       - "foobar  " -->  "123"
//   * large:     produce much larger values
//       - "1234"     -->  "9999999" (just large), "2147483648" (MaxInt32 + 1)
//                         "9223372036854775808" (MaxInt64 + 1)
//                         "18446744073709551616" (MaxUInt64 + 1)
//       - "56.78"    -->  "888888888.9999", "123.456e12",
//                         "3.5e38" (larger than MaxFloat32)
//                         "1.9e308" (larger than MaxFloat64)
//       - "foo"      -->  50 * "X", 160 * "Y" and 270 * "Z"
//   * tiny:      produce 0 or short values
//       - "1234"      -->  "0" and "1"
//       - "12.3"      -->  "0", "0.02", "0.0003", "1e-12" and "4.7e-324"
//       - "foobar"    --> "f"
//   * negative   produce negative values
//       - "1234"      -->  "-2"
//       - "56.78"     -->  "-3.3"
//
// This check will make a wast amount of request to the given URL including
// the modifying and non-idempotent methods POST, PUT, and DELETE. Some
// care using this check is advisable.
type Resilience struct {
	// Methods is the space separated list of HTTP methods to check,
	// e.g. "GET POST HEAD". The empty value will test the original
	// method of the test only.
	Methods string `json:",omitempty"`

	// ModParam and ModHeader control which modifications of parameter values
	// and header values are checked.
	// It is a space separated string of the modifications explained above
	// e.g. "drop nonsense empty".
	// An empty value turns off resilience testing.
	ModParam, ModHeader string `json:",omitempty"`

	// ParamsAs controls how parameter values are transmitted, it
	// is a space separated list of all transmission types like in
	// the Request.ParamsAs field, e.g. "URL body multipart" to check
	// URL query parameters, x-www-form-urlencoded and multipart/formdata.
	// The empty value will just check the type used in the original
	// test.
	ParamsAs string `json:",omitempty"`

	// SaveFailuresTo is the filename to which all failed checks shall
	// be logged. The data is appended to the file.
	SaveFailuresTo string `json:",omitempty"`

	// Checks is the list of checks to perform on the received responses.
	// In most cases the -- correct -- behaviour of the server will differ
	// from the response to a valid, unscrambled request; typically by
	// returning one of the 4xx status codes.
	// If Checks is empty, only a simple NoServerError will be executed.
	Checks CheckList `json:",omitempty"`

	// Values contains a list of values to use as header and parameter values.
	// Note that header and parameter checking uses the same list of Values,
	// you might want to do two Resilience checks, one for the headers and one
	// for the parameters.
	// If values is empty, then only the builtin modifications selected by
	// Mod{Param,Header} are used.
	Values []string
}

// Execute implements Check's Execute method.
func (r Resilience) Execute(t *Test) error {
	suite := &Collection{}

	for _, method := range r.methods(t) {
		// Just an other method.
		if method != t.Request.Method {
			suite.Tests = append(suite.Tests, r.resilienceTest(t, method, t.Request.ParamsAs))
		}

		// Fiddle with HTTP header.
		if r.ModHeader != "" {
			// This block is literally the same like the code below.
			// Keep in sync or refactore once.

			// No headers at all.
			rt := r.resilienceTest(t, method, t.Request.ParamsAs)
			rt.Request.Header = nil
			suite.Tests = append(suite.Tests, rt)

			// Modify each header individually.
			wantedMods, _ := parseModifications(r.ModHeader)
			// clear 'space' modification which is unsuitable for HTTP headers
			wantedMods ^= modSpace
			for name, origvals := range t.Request.Header {
				for _, modvals := range r.modify(origvals, wantedMods) {
					rt := r.resilienceTest(t, method, t.Request.ParamsAs)
					rt.Name += prettyprintParams(name, modvals)
					if modvals == nil {
						// drop parameter
						delete(rt.Request.Header, name)
					} else {
						// change parameter
						rt.Request.Header[name] = modvals
					}
					suite.Tests = append(suite.Tests, rt)
				}
			}
		}

		// Fiddle with parameters.
		for _, pas := range r.paramsAs(t) {
			if (method == "GET" || method == "HEAD" || method == "OPTIONS") && pas != "URL" {
				continue
			}

			// Just an other parameter transmission type.
			if pas != t.Request.ParamsAs {
				suite.Tests = append(suite.Tests, r.resilienceTest(t, method, pas))
			}

			if r.ModParam != "" {
				// No parameters at all.
				rt := r.resilienceTest(t, method, pas)
				rt.Request.Params = nil
				suite.Tests = append(suite.Tests, rt)

				// Modify each parameter individually.
				wantedMods, _ := parseModifications(r.ModParam)
				for name, origvals := range t.Request.Params {
					for _, modvals := range r.modify(origvals, wantedMods) {
						rt := r.resilienceTest(t, method, pas)
						rt.Name += prettyprintParams(name, modvals)
						if modvals == nil {
							// drop parameter
							delete(rt.Request.Params, name)
						} else {
							// change parameter
							rt.Request.Params[name] = modvals
						}
						suite.Tests = append(suite.Tests, rt)
					}
				}
			}
		}
	}

	t.infof("Start of resilience suite")
	suite.ExecuteConcurrent(1, nil) // TODO: why not higher concurrency ??
	t.infof("End of resilience suite")
	if suite.Status != Pass {
		return r.collectErrors(t, suite)
	}
	return nil
}

// collectErrors collects all test failures/errors in the resilience suite.
// If logging the results is desired via r.SaveFailuresTo this is done
// here. CollectErros allways returns a non-nil error.
func (r Resilience) collectErrors(t *Test, suite *Collection) error {
	var failures = []string{}

	var file *os.File
	var err error

	if r.SaveFailuresTo != "" {
		file, err = os.OpenFile(r.SaveFailuresTo, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			file = nil
			// Cannot do much more than return this info and log it
			failures = append(failures, fmt.Sprintf("cannot SaveFailuresTo %q: %s",
				r.SaveFailuresTo, err))
		}
	} else {
		defer file.Close()
		fmt.Fprintf(file, "#### List of failed Reslience checks in test %q started at %s:\n",
			t.Name, t.Started)
	}

	for _, t := range suite.Tests {
		if t.Status != Pass {
			failures = append(failures, t.Name)
			if file == nil {
				continue
			}

			// Log to file name SaveFailuresTo.
			data, err := t.AsJSON()
			if err != nil {
				fmt.Fprintf(file, "# %q: Cannot serialize: %q\n", t.Name, err.Error())
			} else {
				fmt.Fprintf(file, "# %q: %s\n", t.Name, t.Status)
				fmt.Fprintln(file, string(data))
				fmt.Fprintln(file)
			}
		}
	}

	collected := strings.Join(failures, "; ")
	return errors.New(collected)
}

func prettyprintParams(name string, mv []string) string {
	if mv == nil {
		return fmt.Sprintf(" %s dropped", name)
	}
	v := make([]string, len(mv))
	copy(v, mv)
	for i, s := range v {
		if len(s) > 20 {
			v[i] = fmt.Sprintf("%s..[%d]", s[:15], len(s))
		}
	}
	return fmt.Sprintf(" %s=%v", name, v)
}

func (r Resilience) methods(t *Test) []string {
	if r.Methods == "" {
		return []string{t.Request.Method}
	}
	return strings.Split(r.Methods, " ")
}

func (r Resilience) paramsAs(t *Test) []string {
	if r.ParamsAs == "" {
		return []string{t.Request.ParamsAs}
	}
	return strings.Split(r.ParamsAs, " ")
}

type modification uint32

const (
	modNone modification = 0
	modDrop modification = 1 << iota
	modDouble
	modTwice
	modChange
	modDelete
	modNonsense
	modSpace
	modMalicious
	modUser
	modEmpty
	modType
	modLarge
	modNegative
	modTiny

	modAll = 2*modTiny - 1
)

var modNames = strings.Split("none drop double twice change delete nonsense space malicious user empty type large negative tiny", " ")

func parseModifications(s string) (modification, error) {
	m := modNone
	if s == "" {
		return modNone, nil
	}
	for _, f := range strings.Split(s, " ") {
		if f == "all" {
			return modAll, nil
		}

		i := -1
		for k, name := range modNames {
			if name == f {
				i = k
				break
			}
		}
		if i == -1 {
			return m, fmt.Errorf("ht: no such modification %q", f)
		}
		if i == 0 {
			continue
		}
		m |= 1 << uint(i)
	}

	return m, nil
}

// modify takes an original set of parameter or header values and produces a
// list of new ones based on the desired modifications.
func (r Resilience) modify(orig []string, mod modification) [][]string {
	list := [][]string{}

	if mod&modDrop != 0 {
		list = append(list, nil)
	}

	if mod&modDouble != 0 {
		val := orig[0]
		list = append(list, []string{val, val})
	}

	if mod&modTwice != 0 {
		val := append(orig, "extraValue")
		list = append(list, val)
	}

	if mod&modChange != 0 {
		for o := range orig {
			L := len(orig[0])
			if L >= 1 {
				list = append(list, doChange(orig, o, 0))
			}
			if L >= 2 {
				list = append(list, doChange(orig, o, L-1))
			}
			if L >= 3 {
				list = append(list, doChange(orig, o, L/2))
			}
		}
	}

	if mod&modDelete != 0 {
		for o := range orig {
			L := len(orig[o])
			if L >= 1 {
				list = append(list, doDelete(orig, o, 0))
			}
			if L >= 2 {
				list = append(list, doDelete(orig, o, L-1))
			}
			if L >= 3 {
				list = append(list, doDelete(orig, o, L/2))
			}
		}
	}

	if mod&modNonsense != 0 {
		list = append(list, []string{"p,f1u;p5c:h*"})    // arbitrary garbage
		list = append(list, []string{"hubba%12bubba(!"}) // arbitrary garbage

	}

	if mod&modSpace != 0 {
		list = append(list, []string{" "})
		list = append(list, []string{"       "})
		list = append(list, []string{"\t"})
		list = append(list, []string{"\n"})
		list = append(list, []string{"\r"})
		list = append(list, []string{"\v"})
		list = append(list, []string{"\x00\x00"})
		list = append(list, []string{"\u00A0"})
		list = append(list, []string{"\u2003"})
		list = append(list, []string{"\u200B"})
		list = append(list, []string{"\t \v \r \n "})
	}

	if mod&modMalicious != 0 {
		// list = append(list, []string{"X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*"})
		list = append(list, []string{"\uFEFF\u200B\u2029"})
		list = append(list, []string{"ʇunpᴉpᴉɔuᴉ"})
		list = append(list, []string{"http://a/%%30%30"})
		list = append(list, []string{"' OR 1=1 -- 1"})
	}

	if mod&modUser != 0 {
		for _, v := range r.Values {
			list = append(list, []string{v})
		}
	}

	if mod&modEmpty != 0 {
		list = append(list, []string{""})
	}

	if mod&modType != 0 {
		switch parameterType(orig[0]) {
		case integerType, floatType:
			val := strings.Repeat("w", len(orig[0]))
			list = append(list, []string{val})
		case emailType:
			val := strings.Replace(orig[0], "@", "X", -1)
			val = strings.Replace(val, ".", "Y", -1)
			list = append(list, []string{val})
		case stringType:
			list = append(list, []string{"123"})
		default:
			// Unknown type, no extra modifications done
		}
	}

	if mod&modLarge != 0 {
		switch parameterType(orig[0]) {
		case integerType:
			list = append(list, []string{"9999999"})              // just large
			list = append(list, []string{"2147483648"})           // MaxInt32 + 1
			list = append(list, []string{"9223372036854775808"})  // MaxInt64 + 1
			list = append(list, []string{"18446744073709551616"}) // MaxUInt64 + 1
		case floatType:
			list = append(list, []string{"888888888.9999"}) // just large
			list = append(list, []string{"3.5e38"})         //  (larger than MaxFloat32)
			list = append(list, []string{"1.9e308"})        //  (larger than MaxFloat64)
		case stringType:
			list = append(list, []string{strings.Repeat("X", 50)})
			list = append(list, []string{strings.Repeat("Y", 160)})
			list = append(list, []string{strings.Repeat("Z", 270)}) // more than 256
		}
	}

	if mod&modTiny != 0 {
		switch parameterType(orig[0]) {
		case integerType:
			list = append(list, []string{"0"})
			list = append(list, []string{"1"})
		case floatType:
			list = append(list, []string{"0"})
			list = append(list, []string{"0.02"})
			list = append(list, []string{"0.0003"})
			list = append(list, []string{"1e-12"})
			list = append(list, []string{"4.7e-324"}) // maler than tiniest float64
		case stringType:
			if len(orig[0]) > 0 {
				list = append(list, []string{orig[0][:1]})
			}
		}
	}

	if mod&modNegative != 0 {
		switch parameterType(orig[0]) {
		case integerType:
			list = append(list, []string{"-2"})
		case floatType:
			list = append(list, []string{"-3.3"})
		}
	}

	return list
}

// resilienceTest makes a copy of orig. The copy uses the given HTTP method and
// paramater transport type paramsAs and has just one check, a No ServerError.
// Header fields and parameters are deep copied. The actual set of cookies is
// copied from the orig's jar.
func (r Resilience) resilienceTest(orig *Test, method string, paramsAs string) *Test {
	cpy := &Test{
		Name: fmt.Sprintf("%s %s", method, paramsAs),
		Request: Request{
			Method:          method,
			URL:             orig.Request.Request.URL.String(),
			FollowRedirects: false,
			ParamsAs:        paramsAs,
			BasicAuthUser:   orig.Request.BasicAuthUser,
			BasicAuthPass:   orig.Request.BasicAuthPass,
		},
		Execution: Execution{
			Verbosity: orig.Execution.Verbosity - 1,
			PreSleep:  Duration(10 * time.Millisecond),
		},
	}

	cpy.Request.Header = make(http.Header)
	for h, v := range orig.Request.Header {
		vc := make([]string, len(v))
		copy(vc, v)
		cpy.Request.Header[h] = vc
	}

	cpy.Request.Params = make(url.Values)
	for p, v := range orig.Request.Params {
		vc := make([]string, len(v))
		copy(vc, v)
		cpy.Request.Params[p] = vc
	}

	if len(r.Checks) == 0 {
		cpy.Checks = CheckList{
			NoServerError{},
		}
	} else {
		cpy.Checks = r.Checks
	}

	cpy.PopulateCookies(orig.Jar, orig.Request.Request.URL)

	return cpy
}

// doChange returns a copy of orig with the character (i,j) changed.
func doChange(orig []string, i, j int) []string {
	cpy := make([]string, len(orig))
	copy(cpy, orig)
	val := []byte(cpy[i])
	if val[j] < 127 {
		val[j]++
	} else {
		val[j]--
	}
	cpy[i] = string(val)
	return cpy
}

// doChange returns a copy of orig with the character (i,j) deleted.
func doDelete(orig []string, i, j int) []string {
	cpy := make([]string, len(orig))
	copy(cpy, orig)
	val := []byte(cpy[i])
	mod := append(val[:j], val[j+1:]...)
	cpy[i] = string(mod)
	return cpy
}

type paramT uint32

const (
	arbitaryType paramT = iota
	integerType
	floatType
	emailType
	stringType
)

func parameterType(s string) paramT {
	_, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return integerType
	}

	_, err = strconv.ParseFloat(s, 64)
	if err == nil {
		return floatType
	}

	// TODO: a bit more email-like would be fine.
	if strings.Count(s, "@") == 1 && strings.Count(s, ".") >= 1 {
		return emailType
	}

	if len(s) > 0 {
		// TODO: better definition of 'string type' needed.
		if (s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z') {
			return stringType
		}
	}

	return arbitaryType
}

// Prepare implements Check's Prepare method.
func (r Resilience) Prepare() error {
	_, err := parseModifications(r.ModParam)
	if err != nil {
		return fmt.Errorf("cannot parse ModParam: %s", err)
	}

	_, err = parseModifications(r.ModHeader)
	if err != nil {
		return fmt.Errorf("cannot parse ModHeader: %s", err)
	}
	return nil
}
