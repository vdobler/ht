// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"strings"
)

// Underline title with c, indented by prefix.
//     Title
//     ~~~~~
func Underline(title string, c string, prefix string) string {
	// TODO: add at least some math
	return prefix + title + "\n" + prefix + strings.Repeat(c, len(title))[:len(title)]
}

// BoldBox around title, indented by prefix.
//    #################
//    ##             ##
//    ##    Title    ##
//    ##             ##
//    #################
func BoldBox(title string, prefix string) string {
	n := len(title)
	top := prefix + strings.Repeat("#", n+12)
	pad := prefix + "##" + strings.Repeat(" ", n+8) + "##"
	return fmt.Sprintf("%s\n%s\n%s##    %s    ##\n%s\n%s", top, pad, prefix, title, pad, top)
}

// Box around title, indented by prefix.
//    +------------+
//    |    Title   |
//    +------------+
func Box(title string, prefix string) string {
	n := len(title)
	top := prefix + "+" + strings.Repeat("-", n+6) + "+"
	return fmt.Sprintf("%s\n%s|   %s   |\n%s", top, prefix, title, top)
}
