// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[2-over]]] 2017-6-9 15:58:06
// 本文件全部代码都看看

package multipart

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestReadForm(t *testing.T) {
	b := strings.NewReader(strings.Replace(message, "\n", "\r\n", -1))
	// 是 multipart.NewReader
	r := NewReader(b, boundary)
	// r.ReadForm(25) 返回 *multipart.Form
	// 读取25字节到内存,25字节之外的到磁盘文件
	f, err := r.ReadForm(25)
	if err != nil {
		t.Fatal("ReadForm:", err)
	}
	// 函数结束时清理磁盘上的临时文件
	// 文档: RemoveAll removes any temporary files associated with a Form.
	defer f.RemoveAll()
	// g是get, e是expect
	if g, e := f.Value["texta"][0], textaValue; g != e {
		t.Errorf("texta value = %q, want %q", g, e)
	}
	if g, e := f.Value["textb"][0], textbValue; g != e {
		t.Errorf("texta value = %q, want %q", g, e)
	}
	fd := testFile(t, f.File["filea"][0], "filea.txt", fileaContents)
	if _, ok := fd.(*os.File); ok {
		// 期望f不是*os.File
		// ??????????????
		t.Error("file is *os.File, should not be")
	}
	fd.Close()
	fd = testFile(t, f.File["fileb"][0], "fileb.txt", filebContents)
	if _, ok := fd.(*os.File); !ok {
		// 期望f是*os.File
		// ??????????????
		t.Errorf("file has unexpected underlying type %T", fd)
	}
	fd.Close()
}

// efn: expected file name
// econtent: expected content
func testFile(t *testing.T, fh *FileHeader, efn, econtent string) File {
	if fh.Filename != efn {
		t.Errorf("filename = %q, want %q", fh.Filename, efn)
	}
	if fh.Size != int64(len(econtent)) {
		t.Errorf("size = %d, want %d", fh.Size, len(econtent))
	}
	f, err := fh.Open()
	if err != nil {
		t.Fatal("opening file:", err)
	}
	b := new(bytes.Buffer)
	_, err = io.Copy(b, f)
	if err != nil {
		// 期望io.Copy成功
		t.Fatal("copying contents:", err)
	}
	if g := b.String(); g != econtent {
		t.Errorf("contents = %q, want %q", g, econtent)
	}
	return f
}

const (
	fileaContents = "This is a test file."
	filebContents = "Another test file."
	textaValue    = "foo"
	textbValue    = "bar"
	boundary      = `MyBoundary`
)

const message = `
--MyBoundary
Content-Disposition: form-data; name="filea"; filename="filea.txt"
Content-Type: text/plain

` + fileaContents + `
--MyBoundary
Content-Disposition: form-data; name="fileb"; filename="fileb.txt"
Content-Type: text/plain

` + filebContents + `
--MyBoundary
Content-Disposition: form-data; name="texta"

` + textaValue + `
--MyBoundary
Content-Disposition: form-data; name="textb"

` + textbValue + `
--MyBoundary--
`

func TestReadForm_NoReadAfterEOF(t *testing.T) {
	maxMemory := int64(32) << 20
	boundary := `---------------------------8d345eef0d38dc9`
	body := `
-----------------------------8d345eef0d38dc9
Content-Disposition: form-data; name="version"

171
-----------------------------8d345eef0d38dc9--`

	mr := NewReader(&failOnReadAfterErrorReader{t: t, r: strings.NewReader(body)}, boundary)

	f, err := mr.ReadForm(maxMemory)
	if err != nil {
		t.Fatal(err)
	}
	// 下面会输出: Got: &multipart.Form{Value:map[string][]string{"version":[]string{"171"}}, File:map[string][]*multipart.FileHeader{}}
	t.Logf("Got: %#v", f)
}

// failOnReadAfterErrorReader is an io.Reader wrapping r.
// It fails t if any Read is called after a failing Read.
type failOnReadAfterErrorReader struct {
	t      *testing.T
	r      io.Reader
	sawErr error
}

func (r *failOnReadAfterErrorReader) Read(p []byte) (n int, err error) {
	if r.sawErr != nil {
		r.t.Fatalf("unexpected Read on Reader after previous read saw error %v", r.sawErr)
	}
	n, err = r.r.Read(p)
	r.sawErr = err
	return
}

// TestReadForm_NonFileMaxMemory asserts that the ReadForm maxMemory limit is applied
// while processing non-file form data as well as file form data.
func TestReadForm_NonFileMaxMemory(t *testing.T) {
	largeTextValue := strings.Repeat("1", (10<<20)+25)
	message := `--MyBoundary
Content-Disposition: form-data; name="largetext"

` + largeTextValue + `
--MyBoundary--
`

	testBody := strings.Replace(message, "\n", "\r\n", -1)
	testCases := []struct {
		name      string
		maxMemory int64
		err       error
	}{
		{"smaller", 50, nil},
		{"exact-fit", 25, nil},
		{"too-large", 0, ErrMessageTooLarge},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := strings.NewReader(testBody)
			r := NewReader(b, boundary)
			f, err := r.ReadForm(tc.maxMemory)
			if err == nil {
				defer f.RemoveAll()
			}
			if tc.err != err {
				t.Fatalf("ReadForm error - got: %v; expected: %v", tc.err, err)
			}
			if err == nil {
				if g := f.Value["largetext"][0]; g != largeTextValue {
					t.Errorf("largetext mismatch: got size: %v, expected size: %v", len(g), len(largeTextValue))
				}
			}
		})
	}
}
