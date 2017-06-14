// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-14 12:52:09

package path_test

import (
	"fmt"
	"path"
)

func ExampleBase() {
	fmt.Println(path.Base("/a/b"))
	// 文档: If the path consists entirely of slashes, Base returns "/".
	fmt.Println(path.Base("/"))
	// 文档: If the path is empty, Base returns ".".
	fmt.Println(path.Base(""))
	// Output:
	// b
	// /
	// .
}

func ExampleClean() {
	paths := []string{
		// Clean("a/c") = "a/c" : 原样返回
		"a/c",
		// Clean("a//c") = "a/c" : Replace multiple slashes with a single slash
		"a//c",
		// Clean("a/c/.") = "a/c" : Eliminate each . path name element (the current directory).
		"a/c/.",
		// Clean("a/c/b/..") = "a/c" : Eliminate each inner .. path name element (the parent directory) along with the non-.. element that precedes it.
		"a/c/b/..",
		// Clean("/../a/c") = "/a/c" :Eliminate .. elements that begin a rooted path: that is, replace "/.." by "/" at the beginning of a path.
		"/../a/c",
		// Clean("/../a/b/../././/c") => "/a/b/../././/c" => "/a/././/c" => "/a/c"
		"/../a/b/../././/c",
		"",
	}

	for _, p := range paths {
		fmt.Printf("Clean(%q) = %q\n", p, path.Clean(p))
	}

	// Output:
	// Clean("a/c") = "a/c"
	// Clean("a//c") = "a/c"
	// Clean("a/c/.") = "a/c"
	// Clean("a/c/b/..") = "a/c"
	// Clean("/../a/c") = "/a/c"
	// Clean("/../a/b/../././/c") = "/a/c"
	// Clean("") = "."
}

func ExampleDir() {
	fmt.Println(path.Dir("/a/b/c"))
	fmt.Println(path.Dir("a/b/c"))
	// 文档:If the path consists entirely of slashes followed by non-slash bytes, Dir returns a single slash.
	fmt.Println(path.Dir("/"))
	// 文档:If the path is empty, Dir returns ".".
	fmt.Println(path.Dir(""))
	// Output:
	// /a/b
	// a/b
	// /
	// .
}

func ExampleExt() {
	fmt.Println(path.Ext("/a/b/c/bar.css"))
	fmt.Println(path.Ext("/"))
	fmt.Println(path.Ext(""))
	// Output:
	// .css
	//
	//
}

func ExampleIsAbs() {
	fmt.Println(path.IsAbs("/dev/null"))
	// Output: true
}

func ExampleJoin() {
	fmt.Println(path.Join("a", "b", "c"))
	fmt.Println(path.Join("a", "b/c"))
	fmt.Println(path.Join("a/b", "c"))
	// 文档:all empty strings are ignored.
	fmt.Println(path.Join("", ""))
	fmt.Println(path.Join("a", ""))
	fmt.Println(path.Join("", "a"))
	// Output:
	// a/b/c
	// a/b/c
	// a/b/c
	//
	// a
	// a
}

func ExampleSplit() {
	fmt.Println(path.Split("static/myfile.css"))
	fmt.Println(path.Split("myfile.css"))
	fmt.Println(path.Split(""))
	// Output:
	// static/ myfile.css
	//  myfile.css
	//
}
