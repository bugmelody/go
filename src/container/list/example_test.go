// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-23 16:11:20

package list_test

import (
	"container/list"
	"fmt"
)

func Example() {
	// Create a new list and put some numbers in it.
	l := list.New()
	// l : 4
	e4 := l.PushBack(4)
	// l : 1,4
	e1 := l.PushFront(1)
	// l : 1,3,4
	l.InsertBefore(3, e4)
	// l : 1,2,3,4
	l.InsertAfter(2, e1)

	// Iterate through list and print its contents.
	for e := l.Front(); e != nil; e = e.Next() {
		fmt.Println(e.Value)
	}

	// Output:
	// 1
	// 2
	// 3
	// 4
}
