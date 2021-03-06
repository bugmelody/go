// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// [[[2-over]]] 2017-6-9 14:12:11

package multipart

import (
	"bytes"
	"io/ioutil"
	"net/textproto"
	"strings"
	"testing"
)

func TestWriter(t *testing.T) {
	fileContents := []byte("my file contents")

	var b bytes.Buffer
	// 这是multipart.NewWriter
	w := NewWriter(&b)
	// 注意下面这种写法,通过{}构造一个block,限定了变量的作用域
	{
		part, err := w.CreateFormFile("myfile", "my-file.txt")
		if err != nil {
			// w.CreateFormFile 失败
			t.Fatalf("CreateFormFile: %v", err)
		}
		// 写入body
		part.Write(fileContents)
		err = w.WriteField("key", "val")
		if err != nil {
			t.Fatalf("WriteField: %v", err)
		}
		part.Write([]byte("val"))
		err = w.Close()
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
		s := b.String()
		if len(s) == 0 {
			t.Fatal("String: unexpected empty result")
		}
		if s[0] == '\r' || s[0] == '\n' {
			t.Fatal("String: unexpected newline")
		}
	}

	r := NewReader(&b, w.Boundary())

	part, err := r.NextPart()
	if err != nil {
		t.Fatalf("part 1: %v", err)
	}
	if g, e := part.FormName(), "myfile"; g != e {
		t.Errorf("part 1: want form name %q, got %q", e, g)
	}
	// 读取body
	slurp, err := ioutil.ReadAll(part)
	if err != nil {
		t.Fatalf("part 1: ReadAll: %v", err)
	}
	if e, g := string(fileContents), string(slurp); e != g {
		t.Errorf("part 1: want contents %q, got %q", e, g)
	}

	part, err = r.NextPart()
	if err != nil {
		t.Fatalf("part 2: %v", err)
	}
	if g, e := part.FormName(), "key"; g != e {
		// 期望part2的 FormName=="key"
		t.Errorf("part 2: want form name %q, got %q", e, g)
	}
	slurp, err = ioutil.ReadAll(part)
	if err != nil {
		t.Fatalf("part 2: ReadAll: %v", err)
	}
	if e, g := "val", string(slurp); e != g {
		// 期望part2的 body=="val"
		t.Errorf("part 2: want contents %q, got %q", e, g)
	}

	part, err = r.NextPart()
	if part != nil || err == nil {
		// 期望part结束
		t.Fatalf("expected end of parts; got %v, %v", part, err)
	}
}

func TestWriterSetBoundary(t *testing.T) {
	/**
	SetBoundary文档中提到
	// SetBoundary must be called before any parts are created, may only
	// contain certain ASCII characters, and must be non-empty and
	// at most 70 bytes long.
	 */
	tests := []struct {
		// w.SetBoundary的参数
		b  string
		// w.SetBoundary返回是否成功
		ok bool
	}{
		{"abc", true},
		{"", false},
		// 根据文档: may only contain certain ASCII characters
		{"ungültig", false},
		// 根据文档: may only contain certain ASCII characters
		{"!", false},
		// 根据文档: must be non-empty and at most 70 bytes long
		{strings.Repeat("x", 70), true},
		// 根据文档: must be non-empty and at most 70 bytes long
		{strings.Repeat("x", 71), false},
		// 根据文档: may only contain certain ASCII characters
		{"bad!ascii!", false},
		{"my-separator", true},
		{"with space", true},
		{"badspace ", false},
	}
	for i, tt := range tests {
		var b bytes.Buffer
		w := NewWriter(&b)
		err := w.SetBoundary(tt.b)
		got := err == nil
		if got != tt.ok {
			t.Errorf("%d. boundary %q = %v (%v); want %v", i, tt.b, got, err, tt.ok)
		} else if tt.ok {
			// 获取刚刚设置的Boundary
			got := w.Boundary()
			if got != tt.b {
				// 获取的应该等于刚刚设置的
				t.Errorf("boundary = %q; want %q", got, tt.b)
			}
			w.Close()
			wantSub := "\r\n--" + tt.b + "--\r\n"
			if got := b.String(); !strings.Contains(got, wantSub) {
				// 这里想做什么,参考上面w.Close()的源码
				t.Errorf("expected %q in output. got: %q", wantSub, got)
			}
		}
	}
}

// @see
func TestWriterBoundaryGoroutines(t *testing.T) {
	// Verify there's no data race accessing any lazy boundary if it's used by
	// different goroutines. This was previously broken by
	// https://codereview.appspot.com/95760043/ and reverted in
	// https://codereview.appspot.com/117600043/
	w := NewWriter(ioutil.Discard)
	done := make(chan int)
	go func() {
		w.CreateFormField("foo")
		done <- 1
	}()
	w.Boundary()
	<-done
}

func TestSortedHeader(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.SetBoundary("MIMEBOUNDARY"); err != nil {
		t.Fatalf("Error setting mime boundary: %v", err)
	}

	header := textproto.MIMEHeader{
		"A": {"2"},
		"B": {"5", "7", "6"},
		"C": {"4"},
		"M": {"3"},
		"Z": {"1"},
	}

	part, err := w.CreatePart(header)
	if err != nil {
		t.Fatalf("Unable to create part: %v", err)
	}
	// 写入body
	part.Write([]byte("foo"))

	w.Close()

	want := "--MIMEBOUNDARY\r\nA: 2\r\nB: 5\r\nB: 7\r\nB: 6\r\nC: 4\r\nM: 3\r\nZ: 1\r\n\r\nfoo\r\n--MIMEBOUNDARY--\r\n"
	if want != buf.String() {
		t.Fatalf("\n got: %q\nwant: %q\n", buf.String(), want)
	}
}
