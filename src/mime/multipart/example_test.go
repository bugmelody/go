// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// [[[3-over]]] 2017-6-9 13:59:26

package multipart_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
)

func ExampleNewReader() {
	msg := &mail.Message{
		Header: map[string][]string{
			"Content-Type": {"multipart/mixed; boundary=foo"},
		},
		Body: strings.NewReader(
			// 这是一个part, 'Foo: one'是header,'A section'是body
			"--foo\r\nFoo: one\r\n\r\nA section\r\n" +
			// 这是一个part,'Foo: two'是header,'And another'是body
				"--foo\r\nFoo: two\r\n\r\nAnd another\r\n" +
				"--foo--\r\n"),
	}
	// mediaType="multipart/mixed
	// params是从boundary=foo解析出来的map
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		log.Fatal(err)
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(msg.Body, params["boundary"])
		for {
			// p是 *multipart.Part
			p, err := mr.NextPart()
			if err == io.EOF {
				// 整个multipart读取完毕
				return
			}
			if err != nil {
				log.Fatal(err)
			}
			// slurp 计算机术语 大量读取
			// ioutil.ReadAll(p) 会引发 *Part.Read 调用, *Part.Read 其实是读取 part 的 body
			slurp, err := ioutil.ReadAll(p)
			if err != nil {
				log.Fatal(err)
			}
			// 现在, slurp 代表了 part 的 body 部分
			fmt.Printf("Part %q: %q\n", p.Header.Get("Foo"), slurp)
		}
	}

	// Output:
	// Part "one": "A section"
	// Part "two": "And another"
}
