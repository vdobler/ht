// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"bytes"
	"encoding/xml"
	"fmt"
	htmltemplate "html/template"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"path"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/vdobler/ht/ht"
	mimelist "github.com/vdobler/ht/mime"
)

// ----------------------------------------------------------------------------
// Templates to output

var defaultCheckTmpl = `{{define "CHECK"}}{{printf "%-7s %-15s %s" .Status .Name .JSON}}` +
	`{{if eq .Status 3 5}}{{range .Error}}
                {{.Error}}{{end}}{{end}}{{end}}`

var htmlCheckTmpl = `{{define "CHECK"}}
<div class="toggle{{if gt .Status 2}}Visible{{end}}2">
  <div class="collapsed2">
    <h3 class="toggleButton2">Check:
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span>
      <code>{{.Name}}</code> ▹
    </h3>
  </div>
  <div class="expanded2">
    <h3 class="toggleButton2">Check: 
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span>
      <code>{{.Name}}</code> ▾
    </h3>
    <div class="checkDetails">
      <div>Checking took {{.Duration}}</div>
      <div><code>{{.JSON}}</code></div>
      {{if eq .Status 3 5}}<pre class="description">{{.Error.Error}}</pre>{{end}}
    </div>
  </div>
</div>
{{end}}
`

var defaultTestTmpl = `{{define "TEST"}}{{ToUpper .Status.String}}: {{.Name}}{{if gt .Tries 1}}
  {{printf "(after %d tries)" .Tries}}{{end}}
  Started: {{.Started}}   Duration: {{.FullDuration}}   Request: {{.Duration}}{{if .Request.Request}}
  {{.Request.Request.Method}} {{.Request.Request.URL.String}}{{range .Response.Redirections}}
  GET {{.}}{{end}}{{end}}{{if .Response.Response}}
  {{.Response.Response.Proto}} {{.Response.Response.Status}}{{end}}{{if .Error}}
  Error: {{.Error}}{{end}}
{{if eq .Status 2 3 4 5}}  {{if .CheckResults}}Checks:
{{range $i, $c := .CheckResults}}{{printf "    %2d. " $i}}{{template "CHECK" .}}
{{end}}{{end}}{{end}}{{if .VarValues}}  Variables:
{{range $k, $v := .VarValues}}{{printf "    %s == %q\n" $k $v}}{{end}}{{end}}{{if .ExValues}}  Extracted:
{{range $k, $v := .ExValues}}{{if $v.Error}}{{printf "    %s : %s\n" $k $v.Error}}{{else}}{{printf "    %s == %q\n" $k $v.Value}}{{end}}{{end}}{{end}}{{end}}`

var shortTestTmpl = `{{define "SHORTTEST"}}{{.Status.String}}: {{.Name}}{{if .Request.Request}}
  {{.Request.Request.Method}} {{.Request.Request.URL.String}}{{range .Response.Redirections}}
  GET {{.}}{{end}}{{end}}{{if .Response.Response}}
  {{.Response.Response.Proto}} {{.Response.Response.Status}}{{end}}{{if .Error}}
  {{.Error}}{{end}}{{if gt .Status 2}}{{if .CheckResults}}{{range .CheckResults}}{{if eq .Status 3 5}}
    {{printf "%-7s %-15s %s" .Status .Name .JSON}}{{range .Error}}
      {{.Error}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .ExValues}}{{range $k, $v := .ExValues}}{{if $v.Error}}
  {{printf "Extraction of %s : %s\n" $k $v.Error}}{{end}}{{end}}{{end}}
{{end}}`

var htmlTestTmpl = `{{define "TEST"}}
<div class="toggle{{if gt .Status 2}}Visible{{end}}">
  <div class="collapsed">
    <h2 class="toggleButton">{{.Reporting.SeqNo}}:
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> 
      "{{.Name}}" <small>(<code>{{.Reporting.Filename}}</code>, {{.FullDuration}})</small> ▹
    </h2>
  </div>
  <div class="expanded">
    <h2 class="toggleButton">{{.Reporting.SeqNo}}: 
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> 
      "{{.Name}}" <small>(<code>{{.Reporting.Filename}}</code>, {{.FullDuration}})</small> ▾
    </h2>
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
      <div>
        {{if .Request.Request}}{{template "REQUEST" .}}{{end}}
        {{if .Response.Response}}{{template "RESPONSE" .}}{{end}}
        {{if .Request.SentParams}}{{template "FORMDATA" .Request.SentParams}}{{end}}
        <br/>
      </div>
      <div class="summary">
        <pre class="description">{{.Description}}</pre>
	Started: {{.Started}}<br/>
	Full Duration: {{.FullDuration}} <br/>
        Number of tries: {{.Tries}} <br/>
        Request Duration: {{.Duration}} <br/>
        {{if .VarValues}}Variables:<br/>
          {{range $k, $v := .VarValues}}
            <code>&nbsp;&nbsp;{{printf "%s = %q" $k $v}}</code><br/>
          {{end}}
        {{end}}
        {{if .ExValues}}Extractions:<br/>
          {{range $k, $v := .ExValues}}
            <code>&nbsp;&nbsp;{{printf "%s = %q" $k $v}}</code><br/>
          {{end}}
        {{end}}
        {{if .Error}}<br/><strong>Error:</strong> {{.Error}}{{end}}<br/>
      </div>
      {{if eq .Status 2 3 4 5}}{{if .CheckResults}}
        <div class="checks">
          {{range $i, $c := .CheckResults}}{{template "CHECK" .}}{{end}}
        </div>
      {{end}}{{end}}
    </div>
  </div>
</div>
{{end}}`

var htmlHeaderTmpl = `{{define "HEADER"}}
<div class="httpheader">
  {{range $h, $v := .}}
    {{range $v}}
      <code><strong>{{printf "%25s: " $h}}</strong> {{.}}</code></br>
    {{end}}
  {{end}}
</div>
{{end}}`

var htmlResponseTmpl = `{{define "RESPONSE"}}
<div class="toggle2">
  <div class="expanded2">
    <h3 class="toggleButton2">HTTP Response ▾</h3>
    <div class="responseDetails">
      {{if .Response.Response}}
        {{.Response.Response.Proto}} <strong>{{.Response.Response.Status}}</strong><br/>
        {{template "HEADER" .Response.Response.Header}}
      {{end}}
      {{if .Response.BodyErr}}Error reading body: {{.Response.BodyErr.Error}}
      {{else}}
<pre class="responseBodySummary">
{{Summary .Response.BodyStr}}
</pre>
        <a href="{{.Reporting.SeqNo}}.ResponseBody.{{.Reporting.Extension}}" target="_blank">Response Body</a>
      {{end}}
    </div>
  </div>
  <div class="collapsed2">
    <h3 class="toggleButton2">HTTP Response ▹</h3>
  </div>
</div>
{{end}}
`

var htmlRequestTmpl = `{{define "REQUEST"}}
<div class="toggle2">
  <div class="expanded2">
    <h3 class="toggleButton2">HTTP Request ▾</h3>
    <div class="requestDetails">
      <code><strong>{{.Request.Request.Method}}</strong> {{.Request.Request.URL.String}}
          {{range .Response.Redirections}}</br>GET {{.}}{{end}}
      </code>
      {{template "HEADER" .Request.Request.Header}}
<pre>{{.Request.SentBody}}</pre>
    </div>
  </div>
  <div class="collapsed2">
    <h3 class="toggleButton2">HTTP Request ▹</h3>
  </div>
</div>
{{end}}
`

var htmlFormdataTmpl = `{{define "FORMDATA"}}
<div class="toggle2">
  <div class="expanded2">
    <h3 class="toggleButton2">Form Data ▾</h3>
    <div class="formdataDetails">
      {{range $k, $vs := .}}
        {{range $v := $vs}}
          <code><strong>{{printf "%25s: " $k}}</strong>{{printf "%q" $v}}</code></br>
        {{end}}
      {{end}}
    </div>
  </div>
  <div class="collapsed2">
    <h3 class="toggleButton2">Form Data ▹</h3>
  </div>
</div>
{{end}}
`

var defaultSuiteTmpl = `{{Box (printf "%s: %s" (ToUpper .Status.String) .Name) ""}}{{if .Error}}
Error: {{.Error}}{{end}}
Started: {{.Started}}   Duration: {{.Duration}}

{{range .Tests}}{{template "TEST" .}}
{{end}}
`

var shortSuiteTmpl = `======  Result of {{.Name}} =======
{{range .Tests}}{{template "SHORTTEST" .}}{{end}}{{printf "===> %s <=== %s" (ToUpper .Status.String) .Name}}
`

var htmlStyleTmpl = `{{define "STYLE"}}
<style>
.toggleButton { cursor: pointer; }
.toggleButton2 { cursor: pointer; }

.toggle .collapsed { display: block; }
.toggle .expanded { display: none; }
.toggleVisible .collapsed { display: none; }
.toggleVisible .expanded { display: block; }

.toggle2 .collapsed2 { display: block; }
.toggle2 .expanded2 { display: none; }
.toggleVisible2 .collapsed2 { display: none; }
.toggleVisible2 .expanded2 { display: block; }

h2 { 
  margin-top: 0.5em;
  margin-bottom: 0.2em;
}

h3 { 
  font-size: 1em;
  margin-top: 0.5em;
  margin-bottom: 0em;
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
</style>
{{end}}`

var htmlJavascriptTmpl = `{{define "JAVASCRIPT"}}
<script type="text/javascript" src="https://ajax.googleapis.com/ajax/libs/jquery/1.8.2/jquery.min.js"></script>
<script type="text/javascript">
(function() {
'use strict';

function bindToggle(el) {
  $('.toggleButton', el).click(function() {
    if ($(el).is('.toggle')) {
      $(el).addClass('toggleVisible').removeClass('toggle');
    } else {
      $(el).addClass('toggle').removeClass('toggleVisible');
    }
  });
}
function bindToggles(selector) {
  $(selector).each(function(i, el) {
    bindToggle(el);
  });
}

function bindToggle2(el) {
  console.log("bindToggle2 for " + el);
  $('.toggleButton2', el).click(function() {
    if ($(el).is('.toggle2')) {
      $(el).addClass('toggleVisible2').removeClass('toggle2');
    } else {
      $(el).addClass('toggle2').removeClass('toggleVisible2');
    }
  });
}

function bindToggles2(selector) {
console.log("bindToggles2("+selector+")");
  $(selector).each(function(i, el) {
    bindToggle2(el);
  });
}

$(document).ready(function() {
console.log("bindingstuff");
  bindToggles(".toggle");
  bindToggles(".toggleVisible");
  bindToggles2(".toggle2");
  bindToggles2(".toggleVisible2");
});

})();
</script>
{{end}}`

var htmlSuiteTmpl = `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  {{template "STYLE"}}
  <title>Suite {{.Name}}</title>
</head>
</body>
<a href="../../">Up/Back/Home</a>


<h1>Results of Suite "{{.Name}}"</h1>

{{.Description}}

<div id="summary">
  Status: <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> <br/>
  Started: {{.Started}} <br/>
  Full Duration: {{.Duration}}
</div>

{{range .Tests}}{{template "TEST" .}}{{end}}

{{template "JAVASCRIPT"}}
</body>
</html>
`

var (
	ShortTestTmpl  *template.Template
	TestTmpl       *template.Template
	SuiteTmpl      *template.Template
	ShortSuiteTmpl *template.Template
	HtmlSuiteTmpl  *htmltemplate.Template
)

// Box around title, indented by prefix.
//    +------------+
//    |    Title   |
//    +------------+
func Box(title string, prefix string) string {
	n := len(title)
	top := prefix + "+" + strings.Repeat("-", n+6) + "+"
	return fmt.Sprintf("%s\n%s|   %s   |\n%s", top, prefix, title, top)
}

func init() {
	fm := make(template.FuncMap)
	//fm["Underline"] = Underline
	fm["Box"] = Box
	fm["ToUpper"] = strings.ToUpper

	ShortTestTmpl = template.New("SHORTTEST")
	ShortTestTmpl.Funcs(fm)
	ShortTestTmpl = template.Must(ShortTestTmpl.Parse(shortTestTmpl))

	TestTmpl = template.New("TEST")
	TestTmpl.Funcs(fm)
	TestTmpl = template.Must(TestTmpl.Parse(defaultTestTmpl))
	TestTmpl = template.Must(TestTmpl.Parse(defaultCheckTmpl))

	SuiteTmpl = template.New("SUITE")
	SuiteTmpl.Funcs(fm)
	SuiteTmpl = template.Must(SuiteTmpl.Parse(defaultSuiteTmpl))
	SuiteTmpl = template.Must(SuiteTmpl.Parse(defaultTestTmpl))
	SuiteTmpl = template.Must(SuiteTmpl.Parse(defaultCheckTmpl))

	ShortSuiteTmpl = template.New("SHORTSUITE")
	ShortSuiteTmpl.Funcs(fm)
	ShortSuiteTmpl = template.Must(ShortSuiteTmpl.Parse(shortSuiteTmpl))
	ShortSuiteTmpl = template.Must(ShortSuiteTmpl.Parse(shortTestTmpl))

	HtmlSuiteTmpl = htmltemplate.New("SUITE")
	HtmlSuiteTmpl.Funcs(htmltemplate.FuncMap{
		"ToUpper": strings.ToUpper,
		"Summary": ht.Summary,
	})
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlSuiteTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlTestTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlCheckTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlResponseTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlRequestTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlHeaderTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlFormdataTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlStyleTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlJavascriptTmpl))
}

func PrintTestReport(w io.Writer, t ht.Test) error {
	return TestTmpl.Execute(w, t)
}

func PrintSuiteReport(w io.Writer, s *Suite) error {
	return SuiteTmpl.Execute(w, s)
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

// HTMLReport generates a report of the outcome of s to directory dir.
func HTMLReport(dir string, s *Suite) error {
	errs := ht.ErrorList{}

	for i, test := range s.Tests {
		if tn, ok := test.Variables["TEST_NAME"]; ok {
			test.Reporting.Filename = tn
		} else {
			test.Reporting.Filename = "??"
		}
		test.Reporting.Extension = guessResponseExtension(test)

		body := []byte(test.Response.BodyStr)
		fn := fmt.Sprintf("ResponseBody_%02d.%s", i+1, test.Reporting.Extension)
		name := path.Join(dir, fn)
		err := ioutil.WriteFile(name, body, 0666)
		if err != nil {
			errs = append(errs, err)
		}
	}

	report, err := os.Create(path.Join(dir, "_Report_.html"))
	if err != nil {
		errs = append(errs, err)
	} else {
		err = HtmlSuiteTmpl.Execute(report, s)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// JUnit style output.
// ----------------------------------------------------------------------------

// JUnit4XML generates a JUnit 4 compatible XML result with each Check
// reported as an individual testcase.
// NotRun checks are reported as Skipped and Bogus checks are counted as
// Errored tests.
func (s *Suite) JUnit4XML() (string, error) {
	// Local types used for XML encoding
	type SysOut struct {
		XMLName xml.Name `xml:"system-out"`
		Data    string   `xml:",innerxml"`
	}
	type ErrorMsg struct {
		Message string `xml:"message,attr"`
		Typ     string `xml:"type,attr"`
	}
	type Testcase struct {
		XMLName   xml.Name  `xml:"testcase"`
		Name      string    `xml:"name,attr"`
		Classname string    `xml:"classname,attr"`
		Time      float64   `xml:"time,attr"`
		Skipped   *struct{} `xml:"Skipped,omitempty"`
		Error     *ErrorMsg `xml:"error,omitempty"`
		Failure   *ErrorMsg `xml:"failure,omitempty"`
		SystemOut string    `xml:"system-out,omitempty"`
	}
	type Property struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	}
	type Testsuite struct {
		XMLName xml.Name `xml:"testsuite"`
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
	for _, test := range s.Tests {
		if test.Status >= ht.Error {
			// report all checks as errored but with special message
			for _, cr := range test.CheckResults {
				tc := Testcase{
					Name:      cr.Name,
					Classname: test.Name,
					SystemOut: cr.JSON,
				}
				tc.Error = &ErrorMsg{
					Message: test.Error.Error(),
					Typ:     fmt.Sprintf("main test error, check not run"),
				}
				errored++
				testcases = append(testcases, tc)
			}

		} else {
			for _, cr := range test.CheckResults {
				tc := Testcase{
					Name:      cr.Name,
					Classname: test.Name,
					Time:      float64(cr.Duration) / 1e9,
					SystemOut: cr.JSON,
				}

				switch cr.Status {
				case ht.NotRun, ht.Skipped:
					tc.Skipped = &struct{}{}
					skipped++
				case ht.Pass:
					passed++
				case ht.Fail:
					tc.Failure = &ErrorMsg{
						Message: cr.Error.Error(),
						Typ:     fmt.Sprintf("%T", test.Error),
					}
					failed++
				case ht.Error, ht.Bogus:
					tc.Error = &ErrorMsg{
						Message: test.Error.Error(),
						Typ:     fmt.Sprintf("%T", test.Error),
					}
					errored++
				default:
					panic(cr.Status)
				}

				testcases = append(testcases, tc)
			}
		}
	}

	// Generate a standard text report which becomes the standard-out of
	// the generated JUnit report.
	buf := &bytes.Buffer{}
	var sysout string
	err := PrintSuiteReport(buf, s)
	if err != nil {
		sysout = err.Error()
	} else {
		sysout = xmlEscapeChars(buf.Bytes())
	}

	// Populate the Testsuite type for marshalling.
	ts := Testsuite{
		Tests:     skipped + passed + failed + errored,
		Errors:    errored,
		Failures:  failed,
		Skipped:   skipped,
		Time:      float64(s.Duration) / 1e9,
		Timestamp: s.Started.Format("2006-01-02T15:04:05"),
		Testcase:  testcases,
		SystemOut: SysOut{Data: "\n" + sysout},
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

// xmlEscapeChars escapes the reserved characters. TODO: \r ?
func xmlEscapeChars(s []byte) string {
	buf := &bytes.Buffer{}
	for i := 0; i < len(s); {
		rune, width := utf8.DecodeRune(s[i:])
		i += width
		switch rune {
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '&':
			buf.WriteString("&amp;")
		case '"':
			buf.WriteString("&quot;")
		case '\'':
			buf.WriteString("&apos;")
		case '\t':
			buf.WriteString("&#x9;")
		default:
			// TODO: not every rune is allowed in XML
			buf.WriteRune(rune)
		}
	}
	return buf.String()
}
