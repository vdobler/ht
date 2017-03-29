// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/asaskevich/govalidator"
)

// Condition is a conjunction of tests against a string. Note that Contains and
// Regexp conditions both use the same Count; most likely one would use either
// Contains or Regexp but not both.
type Condition struct {
	// Equals is the exact value to be expected.
	// No other tests are performed if Equals is non-zero as these
	// other tests would be redundant.
	Equals string `json:",omitempty"`

	// Prefix is the required prefix
	Prefix string `json:",omitempty"`

	// Suffix is the required suffix.
	Suffix string `json:",omitempty"`

	// Contains must be contained in the string.
	Contains string `json:",omitempty"`

	// Regexp is a regular expression to look for.
	Regexp string `json:",omitempty"`

	// Count determines how many occurrences of Contains or Regexp
	// are required for a match:
	//     0: Any positive number of matches is okay
	//   > 0: Exactly that many matches required
	//   < 0: No match allowed (invert the condition)
	Count int `json:",omitempty"`

	// Min and Max are the minimum and maximum length the string may
	// have. Two zero values disables this test.
	Min, Max int `json:",omitempty"`

	// GreaterThan and LessThan are lower and upper bound on the numerical
	// value of the string: The string is trimmed from spaces as well as
	// from single and double quotes before parsed as a float64. If the
	// string is not float value these conditions fail.
	// Nil disables these conditions.
	GreaterThan, LessThan *float64 `json:",omitempty"`

	// Is checks whether the string under test matches one of a given
	// list of given types. Double quotes are trimmed from the string
	// before validation its type.
	//
	// The following types are available:
	//     Alpha          Alphanumeric  ASCII             Base64
	//     CIDR           CreditCard    DataURI           DialString
	//     DNSName        Email         FilePath          Float
	//     FullWidth      HalfWidth     Hexadecimal       Hexcolor
	//     Host           Int           IP                IPv4
	//     IPv6           ISBN10        ISBN13            ISO3166Alpha2
	//     ISO3166Alpha3  JSON          Latitude          Longitude
	//     LowerCase      MAC           MongoID           Multibyte
	//     Null           Numeric       Port              PrintableASCII
	//     RequestURI     RequestURL    RFC3339           RGBcolor
	//     Semver         SSN           UpperCase         URL
	//     UTFDigit       UTFLetter     UTFLetterNumeric  UTFNumeric
	//     UUID           UUIDv3        UUIDv4            UUIDv5
	//     VariableWidth
	// See github.com/asaskevich/govalidator for a detailed description.
	//
	// The string "OR" is ignored an can be used to increase the
	// readability of this condition in sutiation like
	//     Condition{Is: "Hexcolor OR RGBColor OR MongoID"}
	Is string `json:",omitempty"`

	// Time checks whether the string is a valid time if parsed
	// with Time as the layout string.
	Time string `json:",omitempty"`

	re *regexp.Regexp
}

func isFilePath(s string) bool {
	is, _ := govalidator.IsFilePath(s)
	return is
}

var ValidationMap = map[string]func(string) bool{
	"Alpha":            govalidator.IsAlpha,
	"Alphanumeric":     govalidator.IsAlphanumeric,
	"ASCII":            govalidator.IsASCII,
	"Base64":           govalidator.IsBase64,
	"CIDR":             govalidator.IsCIDR,
	"CreditCard":       govalidator.IsCreditCard,
	"DataURI":          govalidator.IsDataURI,
	"DialString":       govalidator.IsDialString,
	"DNSName":          govalidator.IsDNSName,
	"Email":            govalidator.IsEmail,
	"FilePath":         isFilePath,
	"Float":            govalidator.IsFloat,
	"FullWidth":        govalidator.IsFullWidth,
	"HalfWidth":        govalidator.IsHalfWidth,
	"Hexadecimal":      govalidator.IsHexadecimal,
	"Hexcolor":         govalidator.IsHexcolor,
	"Host":             govalidator.IsHost,
	"Int":              govalidator.IsInt,
	"IP":               govalidator.IsIP,
	"IPv4":             govalidator.IsIPv4,
	"IPv6":             govalidator.IsIPv6,
	"ISBN10":           govalidator.IsISBN10,
	"ISBN13":           govalidator.IsISBN13,
	"ISO3166Alpha2":    govalidator.IsISO3166Alpha2,
	"ISO3166Alpha3":    govalidator.IsISO3166Alpha3,
	"JSON":             govalidator.IsJSON,
	"Latitude":         govalidator.IsLatitude,
	"Longitude":        govalidator.IsLongitude,
	"LowerCase":        govalidator.IsLowerCase,
	"MAC":              govalidator.IsMAC,
	"MongoID":          govalidator.IsMongoID,
	"Multibyte":        govalidator.IsMultibyte,
	"Null":             govalidator.IsNull,
	"Numeric":          govalidator.IsNumeric,
	"Port":             govalidator.IsPort,
	"PrintableASCII":   govalidator.IsPrintableASCII,
	"RequestURI":       govalidator.IsRequestURI,
	"RequestURL":       govalidator.IsRequestURL,
	"RFC3339":          govalidator.IsRFC3339,
	"RGBcolor":         govalidator.IsRGBcolor,
	"Semver":           govalidator.IsSemver,
	"SSN":              govalidator.IsSSN,
	"UpperCase":        govalidator.IsUpperCase,
	"URL":              govalidator.IsURL,
	"UTFDigit":         govalidator.IsUTFDigit,
	"UTFLetter":        govalidator.IsUTFLetter,
	"UTFLetterNumeric": govalidator.IsUTFLetterNumeric,
	"UTFNumeric":       govalidator.IsUTFNumeric,
	"UUID":             govalidator.IsUUID,
	"UUIDv3":           govalidator.IsUUIDv3,
	"UUIDv4":           govalidator.IsUUIDv4,
	"UUIDv5":           govalidator.IsUUIDv5,
	"VariableWidth":    govalidator.IsVariableWidth,
}

// Fulfilled returns whether s matches all requirements of c.
// A nil return value indicates that s matches the defined conditions.
// A non-nil return indicates missmatch.
func (c Condition) Fulfilled(s string) error {
	if c.Equals != "" {
		if s == c.Equals {
			return nil
		}
		ls, le := len(s), len(c.Equals)
		if ls <= (15*le)/10 {
			// Show full value if not 50% longer.
			return fmt.Errorf("Unequal, was %q", s)
		}
		// Show 10 more characters
		end := le + 10
		if end > ls {
			end = ls
			return fmt.Errorf("Unequal, was %q", s)
		}
		return fmt.Errorf("Unequal, was %q...", s[:end])
	}

	if c.Prefix != "" && !strings.HasPrefix(s, c.Prefix) {
		n := len(c.Prefix)
		if len(s) < n {
			n = len(s)
		}
		return fmt.Errorf("Bad prefix, got %q", s[:n])
	}

	if c.Suffix != "" && !strings.HasSuffix(s, c.Suffix) {
		n := len(c.Suffix)
		if len(s) < n {
			n = len(s)
		}
		return fmt.Errorf("Bad suffix, got %q", s[len(s)-n:])
	}

	if c.Contains != "" {
		if c.Count == 0 && !strings.Contains(s, c.Contains) {
			return ErrNotFound
		} else if c.Count < 0 && strings.Contains(s, c.Contains) {
			return ErrFoundForbidden
		} else if c.Count > 0 {
			if cnt := strings.Count(s, c.Contains); cnt != c.Count {
				return WrongCount{Got: cnt, Want: c.Count}
			}
		}
	}

	if c.Regexp != "" && c.re == nil {
		c.re = regexp.MustCompile(c.Regexp)
	}

	if c.re != nil {
		if c.Count == 0 && c.re.FindStringIndex(s) == nil {
			return ErrNotFound
		} else if c.Count < 0 && c.re.FindStringIndex(s) != nil {
			return ErrFoundForbidden
		} else if c.Count > 0 {
			if m := c.re.FindAllString(s, -1); len(m) != c.Count {
				return WrongCount{Got: len(m), Want: c.Count}
			}
		}
	}

	if c.Min > 0 {
		if len(s) < c.Min {
			return fmt.Errorf("Too short, was %d", len(s))
		}
	}

	if c.Max > 0 {
		if len(s) > c.Max {
			return fmt.Errorf("Too long, was %d", len(s))
		}
	}

	if c.GreaterThan != nil || c.LessThan != nil {
		// Trim and parse s.
		trim := func(r rune) bool {
			return unicode.IsSpace(r) || r == '"' || r == '\''
		}
		t := strings.TrimFunc(s, trim)
		numericVal, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return err
		}

		// Apply conditions.
		if c.GreaterThan != nil && numericVal <= *c.GreaterThan {
			return fmt.Errorf("Not greater than %g, was %g", *c.GreaterThan, numericVal)
		}
		if c.LessThan != nil && numericVal >= *c.LessThan {
			return fmt.Errorf("Not less than %g, was %g", *c.LessThan, numericVal)
		}
	}

	if c.Is != "" {
		if err := c.checkIs(s); err != nil {
			return err
		}
	}

	if c.Time != "" {
		if _, err := time.Parse(c.Time, dequoteString(s)); err != nil {
			return err
		}
	}

	return nil
}

// FulfilledBytes provides a optimized version for Fullfilled(string(byteSlice)).
// TODO: Make this a non-lie.
func (c Condition) FulfilledBytes(b []byte) error {
	return c.Fulfilled(string(b))
}

// Compile pre-compiles the regular expression if part of c.
func (c *Condition) Compile() (err error) {
	if c.Regexp != "" {
		c.re, err = regexp.Compile(c.Regexp)
		if err != nil {
			c.re = nil
			return MalformedCheck{Err: err}
		}
	}
	return nil
}

// Dequote s:  "foobar"  -->  fobar
func dequoteString(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

func (c *Condition) checkIs(s string) error {
	s = dequoteString(s)
	var err error
	for _, typ := range strings.Split(c.Is, " ") {
		ctyp := strings.TrimSpace(typ)
		if ctyp == "" || ctyp == "OR" {
			continue
		}
		validationFn, ok := ValidationMap[ctyp]
		if !ok {
			return MalformedCheck{
				Err: fmt.Errorf("No such type check %q", typ),
			}
		}
		if validationFn(s) {
			return nil // s is of type typ
		}
		err = fmt.Errorf("%q is not a %s", s, typ)
	}

	return err
}
