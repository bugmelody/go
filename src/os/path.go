// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// [[[6-over]]] 2017-6-12 10:54:34

package os

import (
	"io"
	"syscall"
)

// MkdirAll creates a directory named path,
// along with any necessary parents, and returns nil,
// or else returns an error.
// The permission bits perm are used for all
// directories that MkdirAll creates.
// If path is already a directory, MkdirAll does nothing
// and returns nil.
//
// @看源码
func MkdirAll(path string, perm FileMode) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	// Fast path: 首先,尝试看看path是否是一个已经存在的文件或者目录
	dir, err := Stat(path)
	if err == nil {
		// Stat成功
		if dir.IsDir() {
			// 如果path是目录
			// 根据函数文档: If path is already a directory, MkdirAll does nothing and returns nil.
			return nil
		}
		// 如果path不是目录,返回syscall.ENOTDIR错误表示path不是目录
		return &PathError{"mkdir", path, syscall.ENOTDIR}
	}
	// 现在,说明无法Stat或者path不存在

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}
	// 现在,i代表path中最后一个不是PathSeparator的位置

	j := i
	for j > 0 && !IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}
	// 现在,j代表最后一个是PathSeparator的位置

	// 比如:path="/a/b/c/d///"
	//                  i
	//                 j

	// 这里假设所有的父级目录都不存在,下面判断是否有父级目录需要创建,如果有,进行递归创建

	if j > 1 {
		// Create parent
		// 如果最后一个'/'的位置大于1
		// 递归的创建父级目录, 比如这里调用 MkdirAll("/a/b/c", perm)
		err = MkdirAll(path[0:j-1], perm)
		if err != nil {
			return err
		}
	}
	// 现在,所有父目录已经创建完毕,下面创建最后结尾的目录.

	// Parent now exists; invoke Mkdir and use its result.
	err = Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		// Lstat不会跟踪符号链接,它会统计符号链接
		dir, err1 := Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}
	return nil
}

// RemoveAll removes path and any children it contains.
// It removes everything it can but returns the first error
// it encounters. If the path does not exist, RemoveAll
// returns nil (no error).
//
// 看源码, path可以是文件,也可以是目录
// @看源码
func RemoveAll(path string) error {
	// Simple case: if Remove works, we're done.
	// 首先尝试直接删除path(文件或空目录)
	err := Remove(path)
	if err == nil || IsNotExist(err) {
		// 如果Remove成功 || Remove失败是因为path不存在
		return nil
	}
	// 现在,path是非空目录

	// Otherwise, is this a directory we need to recurse into?
	// Lstat不会跟踪符号链接
	dir, serr := Lstat(path)
	if serr != nil {
		if serr, ok := serr.(*PathError); ok && (IsNotExist(serr.Err) || serr.Err == syscall.ENOTDIR) {
			// Lstat报告path不存在或者path不是目录 (不是文件,也不是目录,还可能是其他类型,比如fifo文件)
			return nil
		}
		return serr
	}
	if !dir.IsDir() {
		// Not a directory; return the error from Remove.
		return err
	}
	// 现在,path是目录 并且 非空

	// Directory.
	fd, err := Open(path)
	if err != nil {
		if IsNotExist(err) {
			// Race. It was deleted between the Lstat and Open.
			// Return nil per RemoveAll's docs.
			// 按照 RemoveAll 文档: If the path does not exist, RemoveAll returns nil (no error).
			return nil
		}
		return err
	}

	// Remove contents & return first error.
	// 删除所有目录中的内容 & 返回第一个遇到的错误
	err = nil
	for {
		names, err1 := fd.Readdirnames(100)
		for _, name := range names {
			// 递归删除
			err1 := RemoveAll(path + string(PathSeparator) + name)
			if err == nil {
				err = err1
			}
		}
		if err1 == io.EOF {
			// 目录中的内容循环完毕
			break
		}
		// If Readdirnames returned an error, use it.
		if err == nil {
			err = err1
		}
		if len(names) == 0 {
			break
		}
	}

	// Close directory, because windows won't remove opened directory.
	// 必须关闭了fd,才能进行下面的Remove(path)
	fd.Close()

	// Remove directory.
	err1 := Remove(path)
	if err1 == nil || IsNotExist(err1) {
		// path已经不存在了(调用间被其他人删除了)
		return nil
	}
	if err == nil {
		// 在 err == nil 的情况下, 如果 Remove 返回了错误,使用它
		err = err1
	}
	return err
}
