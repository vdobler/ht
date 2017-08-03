// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

func init() {
	RegisterCheck(&Logfile{})
}

// ----------------------------------------------------------------------------
// Logfile

// Logfile provides checks on a file (i.e. it ignores the HTTP response).
//
// During preparation the current size of the file identified by Path is
// determined. When the check executes it seeks to that position and
// examines anything written to the file since the preparation of the check.
//
// Logfile on remote (Unix) machines may be accessed via ssh (experimental).
type Logfile struct {
	// Path is the file system path to the logfile.
	Path string

	// Condition the newly written stuff must fulfill.
	Condition `json:",omitempty"`

	// Disallow states what is forbidden in the written log.
	Disallow []string `json:",omitempty"`

	// Remote contains access data for a foreign (Unix) server which
	// is contacted via ssh.
	Remote struct {
		// Host is the hostname:port. A port of :22 is optional.
		Host string

		// User contains the name of the user used to make the
		// ssh connection to Host
		User string

		// Password and/or Keyfile used to authenticate
		Password string `json:",omitempty"`
		KeyFile  string `json:",omitempty"`
	} `json:",omitempty"`

	pos        int64
	clientConf *ssh.ClientConfig
	host       string
}

func (f *Logfile) prepareAuthMethods() ([]ssh.AuthMethod, error) {
	am := []ssh.AuthMethod{}
	if f.Remote.Password != "" {
		am = append(am, ssh.Password(f.Remote.Password))
	}
	if f.Remote.KeyFile != "" {
		buffer, err := ioutil.ReadFile(f.Remote.KeyFile)
		if err != nil {
			return am, err
		}
		key, err := ssh.ParsePrivateKey(buffer)
		if err != nil {
			return am, err
		}
		am = append(am, ssh.PublicKeys(key))
	}

	return am, nil
}

// Execute implements Check's Execute method.
func (f *Logfile) Execute(t *Test) error {
	written, err := f.newFileData()
	if err != nil {
		return err
	}

	if err := f.FulfilledBytes(written); err != nil {
		return err
	}
	for _, disallow := range f.Disallow {
		if bytes.Contains(written, []byte(disallow)) {
			return fmt.Errorf("found forbidden %q", disallow)
		}
	}
	return nil
}

// newFileData returns what has been written to f.Path since Prepare.
func (f *Logfile) newFileData() ([]byte, error) {
	if f.Remote.Host != "" {
		f.newRemoteFileData()
	}
	return f.newLocalFileData()
}

func (f *Logfile) newLocalFileData() ([]byte, error) {
	file, err := os.Open(f.Path)
	if err != nil {
		return nil, err
	}
	pos, err := file.Seek(f.pos, io.SeekStart)
	if err != nil {
		return nil, err
	}

	if pos != f.pos {
		return nil, fmt.Errorf("ht: cannot seek to %d in file %s, got to %d", f.pos, f.Path, pos)
	}
	return ioutil.ReadAll(file)
}

func (f *Logfile) newRemoteFileData() ([]byte, error) {
	client, err := ssh.Dial("tcp", f.host, f.clientConf)
	if err != nil {
		return nil, err
	}
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	cmd := fmt.Sprintf("dd ibs=1 skip=%d status=none 'if=%s'", f.pos,
		quoteShellFilename(f.Path))
	data, err := session.CombinedOutput(cmd)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Prepare implements Check's Prepare method.
func (f *Logfile) Prepare(*Test) error {
	if f.Remote.Host == "" {
		return f.localFileSize()
	}

	// Prepare ssh client config only once.
	if f.clientConf == nil {
		ams, err := f.prepareAuthMethods()
		if err != nil {
			return err
		}
		f.clientConf = &ssh.ClientConfig{
			User: f.Remote.User,
			Auth: ams,
		}
		f.host = f.Remote.Host
		if !strings.Contains(f.host, ":") {
			f.host += ":22"
		}

		// Dail early.
		client, err := ssh.Dial("tcp", f.host, f.clientConf)
		if err != nil {
			return err
		}
		client.Close()
	}

	return f.remoteFileSize()
}

var _ Preparable = &Logfile{}

func quoteShellFilename(n string) string {
	return strings.Replace(n, "'", "\\'", -1) // better than nothing
}

func (f *Logfile) remoteFileSize() error {
	client, err := ssh.Dial("tcp", f.host, f.clientConf)
	if err != nil {
		return err
	}
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	cmd := fmt.Sprintf("wc -c '%s'", quoteShellFilename(f.Path))
	data, err := session.CombinedOutput(cmd)
	if err != nil {
		f.pos = 0
		return nil // okay if file does not exist
	}
	s := string(data)
	i := strings.Index(s, " ")
	if i == -1 {
		return fmt.Errorf("unexpected output of wc: %q", s)
	}
	n, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		return err
	}

	f.pos = n
	return nil
}

func (f *Logfile) localFileSize() error {
	file, err := os.Open(f.Path)
	if err != nil {
		f.pos = 0
		return nil
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	f.pos = stat.Size()
	return nil
}
