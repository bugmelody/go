// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// [[[3-over]]] 2017-6-8 17:39:40

package log_test

import (
	"bytes"
	"fmt"
	"log"
)

func ExampleLogger() {
	var buf bytes.Buffer
	// &buf 实现了 io.Writer 接口
	// "logger: " 是日志中每一行的前缀
	logger := log.New(&buf, "logger: ", log.Lshortfile)
	logger.Print("Hello, log file!")

	fmt.Print(&buf)
	// Output:
	// logger: example_test.go:16: Hello, log file!
}
