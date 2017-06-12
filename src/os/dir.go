// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[6-over]]] 2017-6-9 17:34:39

package os

// Readdir reads the contents of the directory associated with file and
// returns a slice of up to n FileInfo values, as would be returned
// by Lstat, in directory order. Subsequent calls on the same file will yield
// further FileInfos.
//
// If n > 0, Readdir returns at most n FileInfo structures. In this case, if
// Readdir returns an empty slice, it will return a non-nil error
// explaining why. At the end of a directory, the error is io.EOF.
//
// If n <= 0, Readdir returns all the FileInfo from the directory in
// a single slice. In this case, if Readdir succeeds (reads all
// the way to the end of the directory), it returns the slice and a
// nil error. If it encounters an error before the end of the
// directory, Readdir returns the FileInfo read until that point
// and a non-nil error.
//
// 参考: $ go doc os.Lstat , os.Lstat 不会跟踪符号链接.
// 对f.Readdir(100)进行多次调用可以返回不同的数据
func (f *File) Readdir(n int) ([]FileInfo, error) {
	// 注意这里在将receiver跟nil做比较
	if f == nil {
		return nil, ErrInvalid
	}
	return f.readdir(n)
}

// Readdirnames reads and returns a slice of names from the directory f.
//
// If n > 0, Readdirnames returns at most n names. In this case, if
// Readdirnames returns an empty slice, it will return a non-nil error
// explaining why. At the end of a directory, the error is io.EOF.
//
// If n <= 0, Readdirnames returns all the names from the directory in
// a single slice. In this case, if Readdirnames succeeds (reads all
// the way to the end of the directory), it returns the slice and a
// nil error. If it encounters an error before the end of the
// directory, Readdirnames returns the names read until that point and
// a non-nil error.
//
// 其实 File.Readdirnames 和 File.Readdir 一样, 只是返回值是 name 而不是 FileInfo.
// 研究源码后发现,这里所谓的name是指base name of the file.
// 对f.Readdirnames(100)进行多次调用可以返回不同的数据
func (f *File) Readdirnames(n int) (names []string, err error) {
	// 注意这里在将receiver跟nil做比较
	if f == nil {
		return nil, ErrInvalid
	}
	// 跟进去看看源码
	return f.readdirnames(n)
}
