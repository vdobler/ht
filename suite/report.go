// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/vdobler/ht/ht"
	mimelist "github.com/vdobler/ht/mime"
)

// ----------------------------------------------------------------------------
// Templates to output

var htmlCheckTmpl = `{{define "CHECK"}}
<div class="toggle2{{if gt .Check.Status 2}}Visible{{end}}2">
  <input type="checkbox" value="selected" {{if gt .Check.Status 2}}checked{{end}}
         id="check-{{.SeqNo}}-{{.N}}" class="toggle-input">
  <label for="check-{{.SeqNo}}-{{.N}}" class="toggle-label">
    <h3><span class="{{ToUpper .Check.Status.String}}">{{ToUpper .Check.Status.String}}</span>
      <code>{{.Check.Name}}</code></h3>
  </label>
  <div class="toggle-content">
    <div class="checkDetails">
      <div>Checking took {{niceduration .Check.Duration}}</div>
      <div><code>{{.Check.JSON}}</code></div>
      {{if eq .Check.Status 3 5}}<code>Error: {{errlist .Check.Error}}</code>{{end}}
    </div>
  </div>
</div>
{{end}}
`

var htmlTestTmpl = `{{define "TEST"}}
{{$seqno := identifier .}}
<div class="toggle">
  <input type="checkbox" value="selected" {{if gt .Status 2}}checked{{end}}
         id="test-{{$seqno}}" class="toggle-input">
  <label for="test-{{$seqno}}" class="toggle-label">
    <h2>{{$seqno}}:
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> 
      "{{.Name}}" <small>(<code>{{filename .}}</code>, {{niceduration .FullDuration}})</small>
    </h2>
  </label>
  <div class="toggle-content">
    <div class="testDetails">
      <div class="reqresp"><code>
        {{if .Request.Request}}
          <strong>{{.Request.Request.Method}}</strong> {{.Request.Request.URL.String}}<br/>
          {{range .Response.Redirections}}
            <strong>GET</strong> {{.}}<br/>
          {{end}}
        {{end}}
        {{if .Response.Response}}
          {{.Response.Response.Proto}} <strong>{{.Response.Response.Status}}</strong>
        {{end}}
      </code></div>
      <div class="summary">
        <pre class="description">{{.Description}}</pre>
	Started: {{nicetime .Started}}<br/>
	Full Duration: {{niceduration .FullDuration}} <br/>
        Number of tries: {{.Tries}} <br/>
        Request Duration: {{niceduration .Duration}} <br/>
        {{if .Error}}<br/><strong>Error:</strong> {{errlist .Error}}<br/>{{end}}
      </div>
      {{if .Request.Request}}{{template "REQUEST" .}}{{end}}
      {{if .Response.Response}}{{template "RESPONSE" .}}{{end}}
      {{if .Request.SentParams}}{{template "FORMDATA" dict "Params" .Request.SentParams "SeqNo" $seqno}}{{end}}
      {{if or .Variables .ExValues}}{{template "VARIABLES" .}}{{end}}
      {{if eq .Status 2 3 4 5}}{{if .CheckResults}}
        <div class="checks">
          {{range $i, $e := .CheckResults}}
{{template "CHECK" dict "Check" . "SeqNo" $seqno "N" $i}}
          {{end}}
        </div>
      {{end}}{{end}}
      <div>
        <div class="toggle">
          <input type="checkbox" value="selected"
                 id="curl-{{$seqno}}" class="toggle-input">
          <label for="curl-{{$seqno}}" class="toggle-label"><h3>Curl Call</h3></label>
          <div class="toggle-content">
            <div>
<pre>
{{.CurlCall}}
</pre>
            </div>
          </div>
        </div>
      </div>
{{with subsuite .}}
      <div>
        <div class="toggle">
          <input type="checkbox" value="selected"
                 id="subsuite-{{$seqno}}" class="toggle-input">
          <label for="subsuite-{{$seqno}}" class="toggle-label"><h3>Sub-Suite</h3></label>
          <div class="toggle-content">
            <div  class="subsuite">
{{template "SUITE" .}}
            </div>
          </div>
        </div>
      </div>
{{end}}
    </div>
  </div>
</div>
{{end}}`

var htmlHeaderTmpl = `{{define "HEADER"}}
<div class="httpheader">
  {{range $h, $v := .}}
    {{range $v}}
      <code><strong>{{printf "%25s: " $h}}</strong> {{.}}</code><br>
    {{end}}
  {{end}}
</div>
{{end}}`

var htmlResponseTmpl = `{{define "RESPONSE"}}
{{$seqno := identifier .}}
<div class="toggle">
  <input type="checkbox" value="selected" checked
         id="resp-{{$seqno}}" class="toggle-input">
  <label for="resp-{{$seqno}}" class="toggle-label"><h3>HTTP Response</h3></label>
  <div class="toggle-content">
    <div class="responseDetails">
      {{if .Response.Response}}
        {{.Response.Response.Proto}} <strong>{{.Response.Response.Status}}</strong><br/>
        {{template "HEADER" .Response.Response.Header}}
      {{end}}
      {{if .Response.BodyErr}}Error reading body: {{.Response.BodyErr.Error}}
      {{else}}
        {{if .Response.BodyStr}}
<pre class="responseBodySummary">
{{Summary .Response.BodyStr}}
</pre>
        <a href="{{$seqno}}.ResponseBody.{{fileext .}}" target="_blank">Response Body</a>
        {{else}}
          &#x2014; &#x2003; no body &#x2003; &#x2014;
        {{end}}
      {{end}}
    </div>
  </div>
</div>
{{end}}
`

var htmlRequestTmpl = `{{define "REQUEST"}}
{{$seqno := identifier .}}
<div class="toggle">
  <input type="checkbox" value="selected"
         id="req-{{$seqno}}" class="toggle-input">
  <label for="req-{{$seqno}}" class="toggle-label"><h3>HTTP Request</h3></label>
  <div class="toggle-content">
    <div class="requestDetails">
      <code><strong>{{.Request.Request.Method}}</strong> {{.Request.Request.URL.String}}
          {{range .Response.Redirections}}<br>GET {{.}}{{end}}
      </code>
      {{template "HEADER" .Request.Request.Header}}
<pre>{{clean .Request.SentBody}}</pre>
    </div>
  </div>
</div>
{{end}}
`

var htmlFormdataTmpl = `{{define "FORMDATA"}}
<div class="toggle">
  <input type="checkbox" value="selected"
         id="form-{{.SeqNo}}" class="toggle-input">
  <label for="form-{{.SeqNo}}" class="toggle-label"><h3>Form Data</h3></label>
  <div class="toggle-content">
    <div class="formdataDetails">
      {{range $k, $vs := .Params}}
        {{range $v := $vs}}
          <code><strong>{{printf "%25s: " $k}}</strong>{{printf "%q" $v}}</code><br>
        {{end}}
      {{end}}
    </div>
  </div>
</div>
{{end}}
`

var htmlVariablesTmpl = `{{define "VARIABLES"}}
{{$seqno := identifier .}}
<div class="toggle">
  <input type="checkbox" value="selected"
         id="var-{{$seqno}}" class="toggle-input">
  <label for="var-{{$seqno}}" class="toggle-label"><h3>Variables</h3></label>

  <div class="toggle-content">
    <div class="variableDetail">
        {{if .Variables}}Variables:<br/>
          {{range $k, $v := .Variables}}
            <code>&nbsp;&nbsp;{{printf "%s = %q" $k $v}}</code><br/>
          {{end}}
        {{end}}
        {{if .ExValues}}Extractions:<br/>
          {{range $k, $v := .ExValues}}
            <code>&nbsp;&nbsp;{{printf "%s = %q" $k $v}}</code><br/>
          {{end}}
        {{end}}
    </div>
  </div>
</div>
{{end}}
`

var defaultSuiteTmpl = `{{Box (printf "%s: %s" (ToUpper .Status.String) .Name) ""}}{{if .Error}}
Error: {{.Error}}{{end}}
Started: {{.Started}}   Duration: {{niceduration .Duration}}

{{range .Tests}}{{template "TEST" .}}
{{end}}
`

var shortSuiteTmpl = `{{range .Tests}}{{template "SHORTTEST" .}}
{{end}}{{printf "%s ==== Suite %s" (ToUpper .Status.String) .Name}}
`

var htmlStyleTmpl = `{{define "STYLE"}}
<style>

.toggle {
	margin: 0 auto;
        padding-top: 0.2em;
        padding-bottom: 0.2em;
}

.toggle-label {
	cursor: pointer;
	display: block;
}

.toggle-label:after {
	content: " ▾";
}

.toggle-content {
	margin-bottom: 0.5ex;
}

.toggle-input {
	display: none;
}

.toggle-input:not(checked) ~ .toggle-content {
	display: none;
}

.toggle-input:checked ~ .toggle-content {
	display: block;
}

.toggle-input:checked ~ .toggle-label:after {
	content: " ▹";
}

.summary {
  padding: 1ex 0 1ex 0;
}

.checks {
  padding: 1ex 0 1ex 0;
}

h2 { 
  margin-top: 0.5em;
  margin-bottom: 0.2em;
  display: inline;
}

h3 { 
  font-size: 1em;
  margin-top: 0.5em;
  margin-bottom: 0em;
  display: inline;
}
.testDetails { margin-left: 1em; }
.checkDetails { margin-left: 2em; }
.requestDetails { margin-left: 2em; }
.responseDetails { margin-left: 2em; }
.formdataDetails { margin-left: 2em; }

.PASS { color: green; }
.FAIL { color: red; }
.ERROR { color: magenta; }
.NOTRUN { color: grey; }

pre.description { font-family: serif; margin: 0px; }

div.subsuite {
  margin-left: 2em;
  padding: 0.5ex 1em 0.5ex 2em;
  background-color: lightblue;
}

div.subsuite h1 { font-size: 1.2em; }
div.subsuite h2 { font-size: 1.1em; }
div.subsuite h3 { font-size: 1em; }

ul.error-list { margin-top: 0; margin-bottom: 0; }

</style>
{{end}}`

var htmlDocumentTmpl = `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  {{template "STYLE"}}
  <title>Suite {{.Name}}</title>
</head>
<body>
<a href="../">Up/Back/Home</a>

{{template "SUITE" .}}

</body>
</html>
`

var htmlSuiteTmpl = `{{define "SUITE"}}
<h1>Results of Suite "{{.Name}}"</h1>

{{.Description}}

<div class="summary">
  Status: <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> <br/>
  Started: {{.Started}} <br/>
  Full Duration: {{niceduration .Duration}}
</div>

{{range .Tests}}{{template "TEST" .}}{{end}}
{{end}}
`

// Templates used to generate default and short text output and HTML page.
var (
	SuiteTmpl      *template.Template
	ShortSuiteTmpl *template.Template
	HtmlSuiteTmpl  *htmltemplate.Template
)

// LoopIteration helps ranging over Data in a template.
type LoopIteration struct {
	Data      interface{}
	I         int // 0-based loop index
	N         int // 1-based loop index
	Odd, Even bool
}

func loopIteration(idx interface{}, data interface{}) (LoopIteration, error) {
	i, ok := idx.(int)
	if !ok {
		return LoopIteration{}, fmt.Errorf("idx is not int")
	}
	return LoopIteration{
		Data: data,
		I:    i,
		N:    i + 1,
		Odd:  i%2 == 0,
		Even: i%2 == 1,
	}, nil
}

func dict(args ...interface{}) (map[string]interface{}, error) {
	n := len(args)
	if n%2 == 1 {
		return nil, errors.New("odd number of arguments to dict")
	}
	dict := make(map[string]interface{}, n/2)
	for i := 0; i < n; i += 2 {
		key, ok := args[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key of type %T", args[i])
		}
		dict[key] = args[i+1]
	}
	return dict, nil
}

// cleanSentBody is used from the REQUEST template to render a representation
// if the request body suitable for display via a <pre> tag in the HTML report.
// The <pre> tag uses "paraphrasing content" (see
// https://html.spec.whatwg.org/multipage/grouping-content.html#the-pre-element)
// which displays "Text" nodes (from
// https://html.spec.whatwg.org/multipage/dom.html#phrasing-content-2):
//     Text nodes and attribute values must consist of scalar values,
//     excluding noncharacters, and controls other than ASCII whitespace.
// Noncharaters are (https://infra.spec.whatwg.org/#noncharacter):
//     A noncharacter is a code point that is in the range U+FDD0 to U+FDEF, inclusive,
//     or U+FFFE, U+FFFF, U+1FFFE, U+1FFFF, U+2FFFE, U+2FFFF, U+3FFFE, U+3FFFF,
//     U+4FFFE, U+4FFFF, U+5FFFE, U+5FFFF, U+6FFFE, U+6FFFF, U+7FFFE, U+7FFFF,
//     U+8FFFE, U+8FFFF, U+9FFFE, U+9FFFF, U+AFFFE, U+AFFFF, U+BFFFE, U+BFFFF,
//     U+CFFFE, U+CFFFF, U+DFFFE, U+DFFFF, U+EFFFE, U+EFFFF, U+FFFFE, U+FFFFF,
//     U+10FFFE, or U+10FFFF.
// Controls are (https://infra.spec.whatwg.org/#control)
//     A C0 control is a code point in the range U+0000 NULL to U+001F
//     INFORMATION SEPARATOR ONE, inclusive.
// ASCII whitespace are (https://infra.spec.whatwg.org/#ascii-whitespace)
//     U+0009 TAB, U+000A LF, U+000C FF, U+000D CR, or U+0020 SPACE
func cleanSentBody(s string) string {
	runes := make([]rune, 0, 2000)
	for i, r := range s {
		if r < 0x1f {
			// Control
			if r == 0x09 || r == 0x0a || r == 0x0c || r == 0x0d {
				// ASCII whitespace is okay in <pre>
				runes = append(runes, r)
			} else {
				// Show controls as \x1e
				s := fmt.Sprintf("\\x%x", r)
				runes = append(runes, []rune(s)...)
			}
		} else if r >= 0x7f && r <= 0x9f {
			// These should work. But somehow they dont.
			// Show like controls \x8a
			s := fmt.Sprintf("\\x%x", r)
			runes = append(runes, []rune(s)...)
		} else if m := r & 0xffff; m == 0xfffe || m == 0xffff {
			runes = append(runes, '\uFFFD')
		} else {
			// This rune is okay.
			runes = append(runes, r)
		}

		// 4k of output should be enough for debugging a request.
		if i >= 4090 {
			runes = append(runes, '\u2026')
			break
		}
	}
	return string(runes)
}

func identifier(t *ht.Test) string { return t.GetStringMetadata("SeqNo") }
func filename(t *ht.Test) string   { return t.GetStringMetadata("Filename") }
func fileext(t *ht.Test) string    { return t.GetStringMetadata("Extension") }

func roundTimeToMS(t time.Time) time.Time {
	return t.Round(time.Millisecond)
}

// roundDuration d to approximately 3 significant digits (but not less than
// to full second).
func roundDuration(d time.Duration) time.Duration {
	round := func(d time.Duration, to time.Duration) time.Duration {
		return to * ((d + to/2) / to)
	}
	min, sec, ms, mu, ns := time.Minute, time.Second, time.Millisecond, time.Microsecond, time.Nanosecond

	// TODO: refactor once loops are invented.
	if d >= 1*min {
		return round(d, sec)
	} else if d >= 10*sec {
		return round(d, 100*ms)
	} else if d >= 1*sec {
		return round(d, 10*ms)
	} else if d >= 100*ms {
		return round(d, 1*ms)
	} else if d >= 10*ms {
		return round(d, 100*mu)
	} else if d >= 1*ms {
		return round(d, 10*mu)
	} else if d >= 100*mu {
		return round(d, 1*mu)
	} else if d >= 10*mu {
		return round(d, 100*ns)
	} else if d >= 1*mu {
		return round(d, 100*ns)
	} else if d >= 100*ns {
		return round(d, 10*ns)
	}
	return d
}

func subsuite(dot interface{}) (*Suite, error) {
	test, ok := dot.(*ht.Test)
	if !ok {
		return nil,
			fmt.Errorf("suite: argument to subsuite must be *ht.Test, got %T", dot)
	}

	md := test.GetMetadata("Subsuite")
	if md == nil {
		return nil, nil
	}
	ss, ok := md.(*Suite)
	if !ok {
		return nil,
			fmt.Errorf("suite: type of Subsuite metadata must be *suite.Suite, got %T", md)
	}

	return ss, nil
}

var errorListTmpl = `<ul class="error-list">
    {{range .}}<li>{{.Error}}</li>
{{end}}
</ul>
`

var errorListTemplate = htmltemplate.Must(htmltemplate.New("ERRLIST").Parse(errorListTmpl))

// ErrorList renders err in HTML. It creates an <ul> if err is of
// type ht.ErrorList and has more than one entry.
func ErrorList(err error) htmltemplate.HTML {
	if err == nil {
		return htmltemplate.HTML("")
	}
	list, ok := err.(ht.ErrorList)
	if !ok {
		list = ht.ErrorList([]error{err})
	} else if len(list) == 0 {
		return htmltemplate.HTML("")
	}

	if len(list) == 1 {
		msg := list[0].Error()
		msg = htmltemplate.HTMLEscapeString(msg)
		return htmltemplate.HTML(msg)
	}

	buf := &bytes.Buffer{}
	errorListTemplate.Execute(buf, list)
	return htmltemplate.HTML(buf.String())
}

func init() {
	fm := make(template.FuncMap)
	//fm["Underline"] = Underline
	fm["Box"] = ht.Box
	fm["ToUpper"] = strings.ToUpper
	fm["nicetime"] = roundTimeToMS
	fm["niceduration"] = roundDuration

	SuiteTmpl = template.New("SUITE")
	SuiteTmpl.Funcs(fm)
	SuiteTmpl = template.Must(SuiteTmpl.Parse(defaultSuiteTmpl))
	SuiteTmpl = template.Must(SuiteTmpl.Parse(ht.DefaultTestTemplate))
	SuiteTmpl = template.Must(SuiteTmpl.Parse(ht.DefaultCheckTemplate))

	ShortSuiteTmpl = template.New("SHORTSUITE")
	ShortSuiteTmpl.Funcs(fm)
	ShortSuiteTmpl = template.Must(ShortSuiteTmpl.Parse(shortSuiteTmpl))
	ShortSuiteTmpl = template.Must(ShortSuiteTmpl.Parse(ht.ShortTestTemplate))

	HtmlSuiteTmpl = htmltemplate.New("DOCUMENT")
	HtmlSuiteTmpl.Funcs(htmltemplate.FuncMap{
		"ToUpper":      strings.ToUpper,
		"Summary":      ht.Summary,
		"loop":         loopIteration,
		"dict":         dict,
		"clean":        cleanSentBody,
		"nicetime":     roundTimeToMS,
		"niceduration": roundDuration,
		"identifier":   identifier,
		"filename":     filename,
		"fileext":      fileext,
		"subsuite":     subsuite,
		"errlist":      ErrorList,
	})
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlDocumentTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlStyleTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlSuiteTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlTestTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlCheckTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlResponseTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlRequestTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlHeaderTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlFormdataTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlVariablesTmpl))
}

// PrintReport outputs a textual report of s to w.
func (s *Suite) PrintReport(w io.Writer) error {
	return SuiteTmpl.Execute(w, s)
}

// PrintShortReport outputs a short textual report of s to w.
func (s *Suite) PrintShortReport(w io.Writer) error {
	return ShortSuiteTmpl.Execute(w, s)
}

// TODO: sniff if unavailable
func guessResponseExtension(test *ht.Test) string {
	if test.Response.Response == nil || len(test.Response.BodyStr) == 0 {
		return "nil"
	}

	ct := test.Response.Response.Header.Get("Content-Type")
	if ct == "" {
		return "txt"
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return "txt"
	}

	if ext, ok := mimelist.MimeTypeExtension[mt]; ok {
		return ext
	}

	return "txt"
}

// make sure HTML-report relevante metadata are present in Test
// and it's attached subsuites. Especially SeqNo is made non-empty.
// Dump all bodies while traversing.
func augmentMetadataAndDumpBody(s *Suite, dir string, prefix string) error {
	errs := ht.ErrorList{}

	for i, test := range s.Tests {
		if test.GetStringMetadata("Filename") == "" {
			test.SetMetadata("Filename", "??")
		}
		extension := guessResponseExtension(test)
		test.SetMetadata("Extension", extension)

		body := []byte(test.Response.BodyStr)
		seqno := test.GetStringMetadata("SeqNo")
		if seqno == "" {
			seqno = fmt.Sprintf("%d", i+1)
		}
		seqno = prefix + seqno
		test.SetMetadata("SeqNo", seqno) // write back to be used in template for HTML ids.

		if sub := test.GetMetadata("Subsuite"); sub != nil {
			subsuite := sub.(*Suite)
			err := augmentMetadataAndDumpBody(subsuite, dir, seqno+"_sub")
			errs = errs.Append(err)
		}
		fn := fmt.Sprintf("%s.ResponseBody.%s", seqno, extension)
		name := path.Join(dir, fn)
		err := ioutil.WriteFile(name, body, 0666)
		errs = errs.Append(err)
	}

	return errs.AsError()
}

// HTMLReport generates a report of the outcome of s to directory dir.
func HTMLReport(dir string, s *Suite) error {
	errs := ht.ErrorList{}

	err := augmentMetadataAndDumpBody(s, dir, "")
	errs = errs.Append(err)

	report, err := os.Create(path.Join(dir, "_Report_.html"))
	errs = errs.Append(err)
	if err == nil {
		err = HtmlSuiteTmpl.Execute(report, s)
		errs = errs.Append(err)
	}

	return errs.AsError()
}

// JUnit style output.
// ----------------------------------------------------------------------------

// JUnit4XML generates a JUnit 4 compatible XML result with each Test
// reported as an individual testcase.
// The following mapping is used from ht to JUnit
//     Suite  -->  testsuite
//     Test   -->  testcase
//     Check  -->  assertion (reported only as number)
//
// NotRun checks are reported as Skipped and Bogus checks are counted as
// Errored tests.
//
// Teardown tests are _not_ included in the JUnit XML output: A failed or
// errored teardwon test is not a suite failure/error but this cannot be
// modeled in the XML output (such test setup/teardwon code is not part
// of the testsuite in JUnit). Tools like Teamcity would report a broken
// build if any failed/errored teardown test was included in the JUnit XML
// output. We do not omit the Setup Tests as these are supposed to pass.
func (s *Suite) JUnit4XML() (string, error) {
	// Local types used for XML encoding
	type SysOut struct {
		XMLName xml.Name `xml:"system-out"`
		Data    string   `xml:",cdata"`
	}
	type ErrorMsg struct {
		Message string `xml:"message,attr"`
		Typ     string `xml:"type,attr"`
	}
	type Testcase struct {
		XMLName    xml.Name  `xml:"testcase"`
		Name       string    `xml:"name,attr"`
		Classname  string    `xml:"classname,attr"`
		Assertions int       `xml:"assertions,attr"`
		Time       float64   `xml:"time,attr"`
		Skipped    *struct{} `xml:"Skipped,omitempty"`
		Error      *ErrorMsg `xml:"error,omitempty"`
		Failure    *ErrorMsg `xml:"failure,omitempty"`
		SystemOut  SysOut
	}
	type Property struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	}
	type Testsuite struct {
		XMLName xml.Name `xml:"testsuite"`
		Name    string   `xml:"name,attr"`
		Tests   int      `xml:"tests,attr"`
		// Disabled   int        `xml:"disabled,attr"`
		Errors     int        `xml:"errors,attr"`
		Failures   int        `xml:"failures,attr"`
		Skipped    int        `xml:"skipped,attr"`
		Time       float64    `xml:"time,attr"`
		Timestamp  string     `xml:"timestamp,attr"`
		Properties []Property `xml:"properties>property"`
		Testcase   []Testcase
		SystemOut  SysOut
	}

	// Unwind all Checks to their own testcase.
	skipped, passed, failed, errored := 0, 0, 0, 0
	testcases := []Testcase{}
	for testno, test := range s.Tests {
		if testno >= s.noneTeardownTest {
			break
		}
		buf := &bytes.Buffer{}
		var sysout string
		err := test.PrintReport(buf)
		if err != nil {
			sysout = err.Error()
		} else {
			sysout = buf.String()
		}
		tc := Testcase{
			Name:       test.Name,
			Classname:  s.Name,
			Time:       float64(test.FullDuration) / 1e9,
			Assertions: len(test.CheckResults),
			SystemOut:  SysOut{Data: sysout},
		}

		switch test.Status {
		case ht.NotRun, ht.Skipped:
			tc.Skipped = &struct{}{}
			skipped++
		case ht.Pass:
			passed++
		case ht.Fail:
			tc.Failure = &ErrorMsg{Message: test.Error.Error()}
			failed++
		case ht.Error, ht.Bogus:
			errored++
			tc.Error = &ErrorMsg{Message: test.Error.Error()}
		default:
			panic("Oooops")
		}
		testcases = append(testcases, tc)
	}

	// Generate a standard text report which becomes the standard-out of
	// the generated JUnit report.
	buf := &bytes.Buffer{}
	var sysout string
	err := s.PrintReport(buf)
	if err != nil {
		sysout = err.Error()
	} else {
		sysout = buf.String()
	}

	// Populate the Testsuite type for marshalling.
	ts := Testsuite{
		Name:      s.Name,
		Tests:     skipped + passed + failed + errored,
		Errors:    errored,
		Failures:  failed,
		Skipped:   skipped,
		Time:      float64(s.Duration) / 1e9,
		Timestamp: s.Started.Format("2006-01-02T15:04:05"),
		Testcase:  testcases,
		SystemOut: SysOut{Data: sysout},
	}
	for k, v := range s.Variables {
		ts.Properties = append(ts.Properties, Property{Name: k, Value: v})
	}

	data, err := xml.MarshalIndent(ts, "", "  ")
	if err != nil {
		return string(data), err
	}
	return xml.Header + string(data) + "\n", nil
}
