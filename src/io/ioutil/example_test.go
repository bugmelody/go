// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-21 15:05:28

package ioutil_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func ExampleReadAll() {
	r := strings.NewReader("Go is a general-purpose language designed with systems programming in mind.")

	b, err := ioutil.ReadAll(r)
	if err != nil {
		// 因为ioutil.ReadAll并不会返回 EOF 的错误,因此 if err != nil 一定说明真正出错了
		log.Fatal(err)
	}

	fmt.Printf("%s", b)

	// Output:
	// Go is a general-purpose language designed with systems programming in mind.
}

func ExampleReadDir() {
	// ioutil.ReadDir返回的信息会经过文件名排序
	files, err := ioutil.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		fmt.Println(file.Name())
	}
}

func ExampleTempDir() {
	// 将要写入临时文件的内容
	content := []byte("temporary file's content")
	// 在系统临时目录中创建'example'作为前缀的目录,比如'example321'
	dir, err := ioutil.TempDir("", "example")
	if err != nil {
		log.Fatal(err)
	}

	// 函数退出时清理刚刚创建的临时目录
	defer os.RemoveAll(dir) // clean up

	tmpfn := filepath.Join(dir, "tmpfile")
	if err := ioutil.WriteFile(tmpfn, content, 0666); err != nil {
		log.Fatal(err)
	}
}

func ExampleTempFile() {
	content := []byte("temporary file's content")
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(tmpfile.Name()) // clean up

	// 向临时文件写入数据
	if _, err := tmpfile.Write(content); err != nil {
		log.Fatal(err)
	}
	// 关闭 tmpfile
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}
}

func ExampleReadFile() {
	content, err := ioutil.ReadFile("testdata/hello")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("File contents: %s", content)

	// Output:
	// File contents: Hello, Gophers!
}
