// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package check

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"reflect"
)

func MarshalCheck(c Check) ([]byte, error) {
	data, err := xml.Marshal(c)
	if err != nil {
		return nil, err
	}
	// Most checks have no nested elements and can be selfclosing which is
	// a bit more plesant.
	if bytes.Count(data, []byte{'<'}) == 2 {
		i := bytes.LastIndex(data, []byte{'<'})
		if data[i-1] == '>' {
			data[i-1] = ' '
			data[i] = '/'
			data[i+1] = '>'
			data = data[:i+2]
		}
	}
	return data, nil
}

func UnmarshalCheck(data []byte) (Check, error) {
	d := struct {
		XMLName xml.Name
	}{}
	if err := xml.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	check, err := CreateCheck(d.XMLName.Local, data)
	return check, err
}

// CreateCheck constructs the Check name from data.
func CreateCheck(name string, data []byte) (Check, error) {
	typ, ok := CheckRegistry[name]
	if !ok {
		return nil, fmt.Errorf("no such check registered")
	}
	check := reflect.New(typ)
	err := xml.Unmarshal(data, check.Interface())
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal check: %s", err.Error())
	}
	check = reflect.Indirect(check)
	return check.Interface().(Check), nil
}
