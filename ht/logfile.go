// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"fmt"
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
type Logfile struct {
	// Path is the file system path to the logfile.
	Path string

	// Condition the written stuff must fulfill.
	Condition `json:",omitempty"`

	// Disallow states what is forbidden in the written log.
	Disallow []string `json:",omitempty"`

	Remote struct {
		Host     string
		User     string
		Password string `json:",omitempty"`
		KeyFile  string `json:",omitempty"`
	} `json:",omitempty"`

	pos    int64
	client *ssh.Client
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
	fmt.Println("Prepare")
	var written []byte
	var err error

	if f.Remote.Host == "" {
		written, err = f.localFile()
	} else {
		written, err = f.remoteFile()
	}
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

func (f *Logfile) localFile() ([]byte, error) {
	file, err := os.Open(f.Path)
	if err != nil {
		return nil, err
	}
	pos, err := file.Seek(f.pos, os.SEEK_SET)
	if err != nil {
		return nil, err
	}

	if pos != f.pos {
		return nil, fmt.Errorf("ht: cannot seek to %d in file %s, got to %d", f.pos, f.Path, pos)
	}
	return ioutil.ReadAll(file)
}

func (f *Logfile) remoteFile() ([]byte, error) {
	session, err := f.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	cmd := fmt.Sprintf("dd ibs=1 skip=%d status=none if=%s", f.pos, f.Path) // TODO: quote path
	data, err := session.CombinedOutput(cmd)
	if err != nil {
		return nil, err
	}
	fmt.Println("New data: ", string(data))
	return data, nil
}

// Prepare implements Check's Prepare method.
func (f *Logfile) Prepare() error {
	fmt.Println("Prepare")
	if f.Remote.Host == "" {
		return f.localFileSize()
	}

	// Establish ssh connection to remote host and keep for reuse in Execute.
	ams, err := f.prepareAuthMethods()
	if err != nil {
		return err
	}
	sshConfig := &ssh.ClientConfig{
		User: f.Remote.User,
		Auth: ams,
	}
	host := f.Remote.Host
	if strings.Index(host, ":") == -1 {
		host += ":22"
	}
	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return err
	}
	f.client = client

	return f.remoteFileSize()
}

func (f *Logfile) remoteFileSize() error {
	fmt.Println("remoteFileSize")
	session, err := f.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	cmd := fmt.Sprintf("cat %s | wc -c", f.Path) // TODO: quote path
	fmt.Println(cmd)
	data, err := session.CombinedOutput(cmd)
	if err != nil {
		fmt.Println(string(data))
		return err
	}
	s := strings.TrimSpace(string(data))
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}

	f.pos = n
	fmt.Println("Filesize befor: ", n)
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
