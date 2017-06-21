// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[3-over]]] 2017-6-21 13:22:06

package ioutil

import (
	"os"
	"testing"
)

// checkSize检查path的大小,看是否等于size参数
func checkSize(t *testing.T, path string, size int64) {
	dir, err := os.Stat(path)
	if err != nil {
		// 期望没有错误
		t.Fatalf("Stat %q (looking for size %d): %s", path, size, err)
	}
	if dir.Size() != size {
		// 期望相等.
		t.Errorf("Stat %q: size %d want %d", path, dir.Size(), size)
	}
}

func TestReadFile(t *testing.T) {
	// 这是个不存在的文件
	filename := "rumpelstilzchen"
	// ioutil.ReadFile 会读取整个文件内容
	contents, err := ReadFile(filename)
	if err == nil {
		// 期望找不到文件
		t.Fatalf("ReadFile %s: error expected, none found", filename)
	}

	// 这是个存在的文件,也就是本文件
	filename = "ioutil_test.go"
	contents, err = ReadFile(filename)
	if err != nil {
		// 期望没有错误
		t.Fatalf("ReadFile %s: %v", filename, err)
	}
	// contents 现在代表文件中的内容

	// 检测通过 os.Stat 统计出来的大小是否为 len(contents)
	checkSize(t, filename, int64(len(contents)))
}

func TestWriteFile(t *testing.T) {
	// 在系统临时目录中创建文件 'ioutil-test'
	f, err := TempFile("", "ioutil-test")
	if err != nil {
		// 期望不出错
		t.Fatal(err)
	}
	filename := f.Name()
	// 待写入的数据
	data := "Programming today is a race between software engineers striving to " +
		"build bigger and better idiot-proof programs, and the Universe trying " +
		"to produce bigger and better idiots. So far, the Universe is winning."

	if err := WriteFile(filename, []byte(data), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", filename, err)
	}

	// 写入后再通过 ReadFile 读入
	contents, err := ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", filename, err)
	}

	// 读到的数据应该等于写入的数据
	if string(contents) != data {
		t.Fatalf("contents = %q\nexpected = %q", string(contents), data)
	}

	// cleanup
	f.Close()
	// 清理临时文件
	// 注意这里的注释 // ignore error,因为系统会自动清理临时文件夹, 即使这里暂时 Remove 失败也没有关系, 因此这里选择忽略错误
	os.Remove(filename) // ignore error
}

func TestReadDir(t *testing.T) {
	// 测试不存在的目录
	dirname := "rumpelstilzchen"
	_, err := ReadDir(dirname)
	if err == nil {
		// 期望报错
		t.Fatalf("ReadDir %s: error expected, none found", dirname)
	}

	// 测试上级目录,也就是src/io/,这是一个存在的目录
	dirname = ".."
	list, err := ReadDir(dirname)
	if err != nil {
		// 期望无错
		t.Fatalf("ReadDir %s: %v", dirname, err)
	}

	foundFile := false
	foundSubDir := false
	// 循环目录的所有内容,看看能否找到 "io_test.go" 文件和 "ioutil" 目录
	for _, dir := range list {
		switch {
		case !dir.IsDir() && dir.Name() == "io_test.go":
			foundFile = true
		case dir.IsDir() && dir.Name() == "ioutil":
			foundSubDir = true
		}
	}
	if !foundFile {
		// 期望找到文件
		t.Fatalf("ReadDir %s: io_test.go file not found", dirname)
	}
	if !foundSubDir {
		// 期望找到目录
		t.Fatalf("ReadDir %s: ioutil directory not found", dirname)
	}
}
