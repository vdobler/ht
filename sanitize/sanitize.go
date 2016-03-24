// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package norm contains functions to sanitize filenames.
package sanitize

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Some characters just do not belong into a filename.
// Several of these charaters are forbidden in Windows, others
// require quoting in normal shells and the rest is disliked by me.
// Note that '&' will be replaced by "_and_" and not just dropped.
var sickCharactersInFilenames = []string{
	"\"", ":", "/", "\\", "(", ")", "?", "*", "\n", "\t", "\r", " ",
	"{", "|", "}", "[", "]", "!", "#", "%", "<", ">", "~", "^", "'",
}

// commonCharaterReplacements contains URL safe replacements for some
// unsuitable -- but common -- characters in filenames.
var commonCharacterReplacements = []struct{ orig, repl string }{
	{"ä", "ae"}, {"Ä", "Ae"}, {"ö", "oe"}, {"Ö", "Oe"},
	{"ü", "ue"}, {"Ü", "Ue"}, {"ß", "ss"}, {"ç", "c"},
	{"&", "_and_"}, {"+", "_plus_"}, {"@", "_at_"},
	{"€", "Euro"}, {"£", "Pound"}, {"$", "Dollar"}, {"¥", "Yen"},
}

// SanitizeFilename produces something resembling name but being
// suitable as a filename.
func SanitizeFilename(name string) string {
	// Eradicate sick charcters and perform common replacements.
	for _, sick := range sickCharactersInFilenames {
		name = strings.Replace(name, sick, "_", -1)
	}
	for _, ccr := range commonCharacterReplacements {
		name = strings.Replace(name, ccr.orig, ccr.repl, -1)
	}

	// Remove accents (combining marks).
	nfd := norm.NFD.String(name)
	name = ""
	for _, r := range nfd {
		if unicode.IsMark(r) {
			continue
		}
		name += fmt.Sprintf("%c", r)
	}

	// Keep only printable ASCII.
	ft := ""
	for _, r := range name {
		if r < 32 || r > 126 {
			ft += "_"
		} else {
			ft += fmt.Sprintf("%c", r)
		}
	}
	name = ft

	// Collaps multiple _ to a single one.
	for strings.Index(name, "__") != -1 {
		name = strings.Replace(name, "__", "_", -1)
	}

	// Trim leading and trailing "-".
	name = strings.Trim(name, "-")
	return name
}
