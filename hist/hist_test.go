// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hist

import (
	"fmt"
	"testing"
)

func TestNewLogHist(t *testing.T) {
	h := NewLogHist(2, 300)
	for v := 0; v < 300; v++ {
		bucket := h.Bucket(v)
		a, b := h.Cover(bucket)
		fmt.Printf("v =%3d  b=%2d  bs=%2d  %3d-%3d\n", v, bucket, b-a, a, b)
	}

}
