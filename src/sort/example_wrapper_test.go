// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[3-over]]] 2017-6-13 10:33:08

package sort_test

import (
	"fmt"
	"sort"
)

/**
10厘克（cg）= 1分克（dg）
10分克（dg）= 1克(公分)（g）
10克(公分)（g）= 1十克（dag）
10 十克(公钱)（dag）= 1百克（hg）
1,000 毫克（mg）=1克（g）
 */

// Gram: 重量单位
type Grams int

func (g Grams) String() string { return fmt.Sprintf("%dg", int(g)) }

// organ ['ɔːg(ə)n] n. [生物] 器官；机构；风琴；管风琴；嗓音
type Organ struct {
	// 器官名
	Name   string
	// 重量
	Weight Grams
}

type Organs []*Organ

func (s Organs) Len() int      { return len(s) }
func (s Organs) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// ByName implements sort.Interface by providing Less and using the Len and
// Swap methods of the embedded Organs value.
//
// ByName实现了sort.Interface
type ByName struct{ Organs }

// ByName 是 receiver, 参数是 (i, j int), 返回类型是 bool
func (s ByName) Less(i, j int) bool { return s.Organs[i].Name < s.Organs[j].Name }

// ByWeight implements sort.Interface by providing Less and using the Len and
// Swap methods of the embedded Organs value.
//
// ByWeight实现了sort.Interface
type ByWeight struct{ Organs }

func (s ByWeight) Less(i, j int) bool { return s.Organs[i].Weight < s.Organs[j].Weight }

// 注意,上面的 ByName, ByWeight 都实现了 sort.Interface
// 其中 Less 是 ByName, ByWeight 自己实现的
// Len, Swap 是内嵌的 Organs 字段实现的
func Example_sortWrapper() {
	s := []*Organ{
		{"brain", 1340},
		{"heart", 290},
		{"liver", 1494},
		{"pancreas", 131},
		{"prostate", 62},
		{"spleen", 162},
	}

	sort.Sort(ByWeight{s})
	fmt.Println("Organs by weight:")
	printOrgans(s)

	sort.Sort(ByName{s})
	fmt.Println("Organs by name:")
	printOrgans(s)

	// Output:
	// Organs by weight:
	// prostate (62g)
	// pancreas (131g)
	// spleen   (162g)
	// heart    (290g)
	// brain    (1340g)
	// liver    (1494g)
	// Organs by name:
	// brain    (1340g)
	// heart    (290g)
	// liver    (1494g)
	// pancreas (131g)
	// prostate (62g)
	// spleen   (162g)
}

func printOrgans(s []*Organ) {
	for _, o := range s {
		fmt.Printf("%-8s (%v)\n", o.Name, o.Weight)
	}
}
