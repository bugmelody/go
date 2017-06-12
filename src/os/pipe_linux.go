// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[5-over]]] 2017-6-12 17:29:18

package os

import "syscall"

// Pipe returns a connected pair of Files; reads from r return bytes written to w.
// It returns the files and an error, if any.
//
// 返回一对连接的文件,从r可以读取到写入w中的数据,即首先向w中写入数据,此时从r中便能够读取到写
// 入w中的数据,Pipe()函数返回文件和该过程中产生的错误．
//
// ??? 这个函数的使用场景是什么 ???
func Pipe() (r *File, w *File, err error) {
	var p [2]int

	e := syscall.Pipe2(p[0:], syscall.O_CLOEXEC)
	// pipe2 was added in 2.6.27 and our minimum requirement is 2.6.23, so it
	// might not be implemented.
	if e == syscall.ENOSYS {
		// See ../syscall/exec.go for description of lock.
		syscall.ForkLock.RLock()
		e = syscall.Pipe(p[0:])
		if e != nil {
			syscall.ForkLock.RUnlock()
			return nil, nil, NewSyscallError("pipe", e)
		}
		syscall.CloseOnExec(p[0])
		syscall.CloseOnExec(p[1])
		syscall.ForkLock.RUnlock()
	} else if e != nil {
		return nil, nil, NewSyscallError("pipe2", e)
	}

	return newFile(uintptr(p[0]), "|0", true), newFile(uintptr(p[1]), "|1", true), nil
}
