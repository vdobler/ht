// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Pseudo Request
//
// Ht allows to make several types of pseudo request: A request which is not a
// HTTP1.1 request but generates output which can be checked via the existing
// checks. The schema of the Test.Request.URL determines the request type.
// Normal HTTP request are made with the two schemas "http" and "https".
// Additionaly the following types of pseudo request are available:
//
// "file" pseudo request
//     This type of pseudo request can be used to read, write and delete a file
//     on the filesystem:
//        - The GET request method tries to read the file given by the URL.Path
//          and returns the content as the response body.
//        - The PUT requets method tries to store the Request.Body under the
//          path given by URL.Path.
//        - The DELETE request method tries to delete the file given by the
//          URL.Path.
//        - The returned HTTP status code is 200 except if any file operation
//          fails in which the Test has status Error.
//
// "bash" pseudo request
//     This type of pseudo request executes a bash script and captures its
//     output as the response.
//        - The script is providen in the Request.Body
//        - The working directory is taken to be URL.Path
//        - Environment is populated from the Request.Params
//        - The Request.Method and the Request.Header are ignored.
//        - The script execution is caneceld after Request.Timout (or the
//          default timeout).
//     The outcome is encoded as follows:
//        - The combined output (stdout and stderr) of the script is returned
//          as the Response.BodyStr.
//        - The HTTP status code is
//             200 if the script's exit code is 0.
//             408 if the script was canceled due to timeout
//             500 if the exit code is != 0.
//        - The Response.Header["Exit-Status"] is used to return the exit
//          status in case of 200 and 500 (sucess and failure).
//
// "sql" pseudo request
//     This type of pseudo request executes a database query (using package
//     database/sql.
//        - The database driver is selected via URL.Host
//        - The data source name is taken from Params["DSN"]
//        - The SQL query is read from the Request.Body
//        - For a POST method the SQL query is passed to sql.Execute
//          and the response body is a JSON with LastInsertId and RowsAffected.
//        - For a GET Request.Method the SQL query is passed to sql.Query
//          and the resulting rows are returned as the response body.
//          The format of the response body is determined by the Accept
//          header:
//             "application/json" (the default): a JSON array with the rows
//                 as objects
//             "text/csv": as a csv file
//     The result if the query is returned in the Response.BodyStr
//
package ht

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func (t *Test) executeFile() error {
	t.infof("%s %q", t.Request.Request.Method, t.Request.Request.URL.String())

	start := time.Now()
	defer func() {
		t.Response.Duration = time.Since(start)
	}()

	u := t.Request.Request.URL
	if u.Host != "" {
		if u.Host != "localhost" && u.Host != "127.0.0.1" { // TODO IPv6
			return fmt.Errorf("file:// on remote host not implemented")
		}
	}

	switch t.Request.Method {
	case "GET":
		file, err := os.Open(u.Path)
		if err != nil {
			return err
		}
		defer file.Close()
		body, err := ioutil.ReadAll(file)
		t.Response.BodyStr = string(body)
		t.Response.BodyErr = err
	case "DELETE":
		err := os.Remove(u.Path)
		if err != nil {
			return err
		}
		t.Response.BodyStr = fmt.Sprintf("Successfully deleted %s", u)
		t.Response.BodyErr = nil
	case "PUT":
		err := ioutil.WriteFile(u.Path, []byte(t.Request.Body), 0666)
		if err != nil {
			return err
		}
		t.Response.BodyStr = fmt.Sprintf("Successfully wrote %s", u)
		t.Response.BodyErr = nil

	default:
		return fmt.Errorf("method %s not supported on file:// URL", t.Request.Method)
	}

	// Fake a http.Response
	t.Response.Response = &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       nil, // already close and consumed
		Trailer:    make(http.Header),
		Request:    t.Request.Request,
	}

	return nil
}

// executeBash executes a bash script:
func (t *Test) executeBash() error {
	t.infof("Bash script in %q", t.Request.Request.URL.String())

	start := time.Now()
	defer func() {
		t.Response.Duration = time.Since(start)
	}()

	u := t.Request.Request.URL
	if u.Host != "" {
		if u.Host != "localhost" && u.Host != "127.0.0.1" { // TODO IPv6
			return fmt.Errorf("bash:// on remote host not implemented")
		}
	}

	workDir := t.Request.Request.URL.Path

	// Save the request body to a temporary file in the working directory.
	temp, err := ioutil.TempFile(workDir, "bashscript")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	_, err = temp.WriteString(t.Request.SentBody)
	cerr := temp.Close()
	if err != nil {
		return err
	}
	if cerr != nil {
		return cerr
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.Request.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/bash", name)
	cmd.Dir = workDir
	for k, v := range t.Request.Params {
		if strings.Contains(k, "=") {
			t.errorf("Environment variable %q from Params contains =; dropped.", k)
			continue
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v[0]))
	}
	output, err := cmd.CombinedOutput()

	// Fake a http.Response
	t.Response.Response = &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       nil, // already close and consumed
		Trailer:    make(http.Header),
		Request:    t.Request.Request,
	}
	t.Response.BodyStr = string(output)

	if ctx.Err() == context.DeadlineExceeded {
		t.Response.Response.StatusCode = http.StatusRequestTimeout
		t.Response.Response.Status = "408 Timeout" // TODO check!
	} else if err != nil {
		t.Response.Response.Status = "500 Internal Server Error"
		t.Response.Response.StatusCode = 500
		t.Response.Response.Header.Set("Exit-Status", err.Error())
	} else {
		t.Response.Response.Header.Set("Exit-Status", "exit status 0")
	}

	return nil
}

// executeSQL executes a SQL query:
func (t *Test) executeSQL() error {
	t.infof("SQL query in %q", t.Request.Request.URL.String())

	start := time.Now()
	defer func() {
		t.Response.Duration = time.Since(start)
	}()

	u := t.Request.Request.URL
	if u.Host == "" {
		return fmt.Errorf("ht: missing database driver name (host of URL) in sql pseudo query")
	}
	dsn := t.Request.Params.Get("DSN")
	if dsn == "" {
		return fmt.Errorf("sql:// missing data source name (DSN parameter) in sql pseudo query")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	if t.Request.Body == "" {
		return fmt.Errorf("ht: missing query (reqquest body) in sql pseudo query")
	}

	// Fake a http.Response
	t.Response.Response = &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       nil, // already close and consumed
		Trailer:    make(http.Header),
		Request:    t.Request.Request,
	}
	// TODO: set content type to JSON

	switch t.Request.Method {
	case "GET":
		t.Response.BodyStr, err = sqlQuery(db, t.Request.Body)
		if err != nil {
			return err
		}
	case "POST":
		t.Response.BodyStr, err = sqlExecute(db, t.Request.Body)
		if err != nil {
			return err
		}

	default:
		// TODO: this should render the Test into state Bogus, not just Error.
		return fmt.Errorf("ht: illegal method %s for sql pseuo query", t.Request.Method)
	}

	return nil
}

// Returns a json like
//    {
//        "LastInsertId": { "Value": 1234 },
//        "RowsAffected": {
//            "Value": 0,
//            "Error": "something went wrong"
//        }
//    }
func sqlExecute(db *sql.DB, query string) (string, error) {
	result, err := db.Exec(query)
	if err != nil {
		return "", err
	}

	tmp := struct {
		LastInsertId struct {
			Value int64
			Error string `json:",omitempty"`
		}
		RowsAffected struct {
			Value int64
			Error string `json:",omitempty"`
		}
	}{}

	lii, liiErr := result.LastInsertId()
	tmp.LastInsertId.Value = lii
	if liiErr != nil {
		tmp.LastInsertId.Error = liiErr.Error()
	}

	ra, raErr := result.RowsAffected()
	tmp.RowsAffected.Value = ra
	if raErr != nil {
		tmp.RowsAffected.Error = raErr.Error()
	}
	body, err := json.MarshalIndent(tmp, "", "    ")
	if err != nil {
		return "", err
	}

	return string(body), nil
}

type recordWriter interface {
	WriteRecord([]string) error
	Close() (string, error)
}

type jsonRecorder struct {
	cols  []string
	buf   *bytes.Buffer
	first bool
	tmp   map[string]string
	err   error
}

func newJsonRecorder(buf *bytes.Buffer, cols []string) *jsonRecorder {
	buf.WriteString("[\n  ")
	return &jsonRecorder{
		cols:  cols,
		buf:   buf,
		first: true,
		tmp:   make(map[string]string),
	}
}

func (jr *jsonRecorder) Close() (string, error) {
	_, err := jr.buf.WriteString("\n]")
	if err != nil {
		jr.err = err
	}
	return jr.buf.String(), jr.err
}
func (jr *jsonRecorder) WriteRecord(values []string) {
	if jr.err != nil {
		return
	}
	for i, col := range jr.cols {
		jr.tmp[col] = values[i]
	}
	record, err := json.MarshalIndent(jr.tmp, "  ", "  ")
	if err != nil {
		jr.err = err
		return
	}
	if jr.first {
		jr.first = false
	} else {
		_, err = jr.buf.WriteString(",\n  ")
		if err != nil {
			jr.err = err
			return
		}
	}
	_, err = jr.buf.Write(record)
	if err != nil {
		jr.err = err
	}
}

func sqlQuery(db *sql.DB, query string) (string, error) {
	rows, err := db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	recorder := newJsonRecorder(buf, columns)

	values := make([]string, len(columns))
	ptrs := make([]interface{}, len(columns))
	for i := range values {
		ptrs[i] = &values[i]
	}
	for rows.Next() {
		err = rows.Scan(ptrs...)
		if err != nil {
			return "", err
		}
		recorder.WriteRecord(values)
	}
	err = rows.Err() // get any error encountered during iteration
	body, _ := recorder.Close()
	if err != nil {
		return body, err
	}

	return body, nil
}
