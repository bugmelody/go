// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-21 13:07:51

package ioutil

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestTempFile(t *testing.T) {
	dir, err := TempDir("", "TestTempFile_BadDir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	nonexistentDir := filepath.Join(dir, "_not_exists_")
	// TempFile函数参数dir: 故意传入一个不存在的目录
	f, err := TempFile(nonexistentDir, "foo")
	if f != nil || err == nil {
		// 期望出错
		// f != nil 表示文件居然创建成功
		// err == nil 表示没有错误
		t.Errorf("TempFile(%q, `foo`) = %v, %v", nonexistentDir, f, err)
	}

	dir = os.TempDir()
	f, err = TempFile(dir, "ioutil_test")
	if f == nil || err != nil {
		// 期望文件创建成功并且没有错误
		t.Errorf("TempFile(dir, `ioutil_test`) = %v, %v", f, err)
	}
	if f != nil {
		f.Close()
		os.Remove(f.Name())
		// regexp.QuoteMeta 说明
		// QuoteMeta returns a string that quotes all regular expression metacharacters
		// inside the argument text; the returned string is a regular expression matching
		// the literal text. For example, QuoteMeta(`[foo]`) returns `\[foo\]`.
		// func QuoteMeta(s string) string {
		// ------------------------------------------------------------------------------------------
		// 因为上面调用 ioutil.TempFile 会在文件名后增加随即数字,因此这里需要用 "[0-9]+$ 进行 匹配
		// ------------------------------------------------------------------------------------------
		re := regexp.MustCompile("^" + regexp.QuoteMeta(filepath.Join(dir, "ioutil_test")) + "[0-9]+$")
		// 注意: 上面已经 调用过 f.Close() 了, 但这里调用 f.Name() 是没有问题的
		if !re.MatchString(f.Name()) {
			t.Errorf("TempFile(`"+dir+"`, `ioutil_test`) created bad name %s", f.Name())
		}
	}
}

func TestTempDir(t *testing.T) {
	// /_not_exists_ 是不存在的目录
	name, err := TempDir("/_not_exists_", "foo")
	if name != "" || err == nil {
		t.Errorf("TempDir(`/_not_exists_`, `foo`) = %v, %v", name, err)
	}

	dir := os.TempDir()
	name, err = TempDir(dir, "ioutil_test")
	if name == "" || err != nil {
		t.Errorf("TempDir(dir, `ioutil_test`) = %v, %v", name, err)
	}
	if name != "" {
		// 清理创建的临时目录
		os.Remove(name)
		re := regexp.MustCompile("^" + regexp.QuoteMeta(filepath.Join(dir, "ioutil_test")) + "[0-9]+$")
		if !re.MatchString(name) {
			t.Errorf("TempDir(`"+dir+"`, `ioutil_test`) created bad name %s", name)
		}
	}
}

// test that we return a nice error message if the dir argument to TempDir doesn't
// exist (or that it's empty and os.TempDir doesn't exist)
func TestTempDir_BadDir(t *testing.T) {
	// dir是存在的
	dir, err := TempDir("", "TestTempDir_BadDir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// badDir是不存在的
	badDir := filepath.Join(dir, "not-exist")
	_, err = TempDir(badDir, "foo")
	if pe, ok := err.(*os.PathError); !ok || !os.IsNotExist(err) || pe.Path != badDir {
		t.Errorf("TempDir error = %#v; want PathError for path %q satisifying os.IsNotExist", err, badDir)
	}
}
