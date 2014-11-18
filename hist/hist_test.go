// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hist

import (
	"fmt"
	"math/rand"
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

func TestAverage(t *testing.T) {
	type avgt struct {
		v    []int
		want int
	}
	for i, tc := range []avgt{
		{[]int{15}, 15},
		{[]int{10, 20}, 15},
		{[]int{40, 41}, 40},
		{[]int{40, 41, 40}, 40},
		{[]int{40, 41, 40, 20}, 35},
		{[]int{128}, 129},
		{[]int{128, 129}, 129},
		{[]int{128, 129, 130}, 129},
		{[]int{128, 129, 130, 131}, 129}, // all are covered by bucket 72 [128,132)
	} {
		h := NewLogHist(5, 200)
		for _, v := range tc.v {
			h.Add(v)
		}
		if got := h.Average(); got != tc.want {
			t.Errorf("%d: Average(%v)=%d, want %d", i, tc.v, got, tc.want)
		}

	}

}

func TestPercentils(t *testing.T) {
	h := NewLogHist(7, 10000)

	for i := 0; i < 100000; i++ {
		v := int(rand.NormFloat64()*500 + 3000)
		h.Add(v)
	}

	for i := 0; i < 100000; i++ {
		v := int(rand.NormFloat64()*500 + 1000)
		h.Add(v)
	}
	for i := 0; i < 100000; i++ {
		v := int(rand.NormFloat64()*500 + 5000)
		h.Add(v)
	}
	for i := 0; i < 1000; i++ {
		v := int(rand.NormFloat64()*500 + 9000)
		h.Add(v)
	}

	fmt.Printf("# Average = %d\n", h.Average())

	ps := make([]float64, 100)
	for p := 0; p < 100; p++ {
		ps[p] = float64(p) / 100
	}
	ps = append(ps, []float64{0.991, 0.992, 0.993, 0.994, 0.995, 0.9955,
		0.996, 0.9965, 0.997, 0.9975, 0.998, 0.9985, 0.999, 0.9995, 1}...)
	u := h.Percentils(ps)
	for i, p := range ps {
		fmt.Printf("%f %d\n", p, u[i])
	}

}
