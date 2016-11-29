// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
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
//   - the script is executed on the host given by the request URL
//   - the script is taken from the request body
//   - the working directory is the path of the request URL
//   - the environment is populated from the request header
//   - the combined output of the script os the response body
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
	for k, v := range t.Request.Header {
		if strings.Contains(k, "=") {
			t.errorf("Environment variable %q from HTTP header contains =; dropped.", k)
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
	}

	return nil
}
