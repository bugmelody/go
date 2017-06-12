// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// [[[6-over]]] 2017-6-12 09:12:45

package os

import (
	"errors"
)

// Portable analogs of some common system call errors.
// analogs n. 类似物（analog的复数）；同型物
// analog ['ænəlɒɡ] n. [自] 模拟；类似物 adj. [自] 模拟的；有长短针的
var (
	ErrInvalid    = errors.New("invalid argument") // methods on File will return this error when the receiver is nil
	ErrPermission = errors.New("permission denied")
	ErrExist      = errors.New("file already exists")
	ErrNotExist   = errors.New("file does not exist")
	ErrClosed     = errors.New("file already closed")
)

// PathError records an error and the operation and file path that caused it.
type PathError struct {
	// 什么操作
	Op   string
	// 在什么路径上
	Path string
	// 引发了什么错误
	Err  error
}

func (e *PathError) Error() string { return e.Op + " " + e.Path + ": " + e.Err.Error() }

// SyscallError records an error from a specific system call.
type SyscallError struct {
	Syscall string
	Err     error
}

func (e *SyscallError) Error() string { return e.Syscall + ": " + e.Err.Error() }

// NewSyscallError returns, as an error, a new SyscallError
// with the given system call name and error details.
// As a convenience, if err is nil, NewSyscallError returns nil.
func NewSyscallError(syscall string, err error) error {
	if err == nil {
		return nil
	}
	return &SyscallError{syscall, err}
}

// IsExist returns a boolean indicating whether the error is known to report
// that a file or directory already exists. It is satisfied by ErrExist as
// well as some syscall errors.
func IsExist(err error) bool {
	return isExist(err)
}

// IsNotExist returns a boolean indicating whether the error is known to
// report that a file or directory does not exist. It is satisfied by
// ErrNotExist as well as some syscall errors.
func IsNotExist(err error) bool {
	return isNotExist(err)
}

// IsPermission returns a boolean indicating whether the error is known to
// report that permission is denied. It is satisfied by ErrPermission as well
// as some syscall errors.
func IsPermission(err error) bool {
	return isPermission(err)
}

// underlyingError returns the underlying error for known os error types.
func underlyingError(err error) error {
	switch err := err.(type) {
	case *PathError:
		return err.Err
	case *LinkError:
		return err.Err
	case *SyscallError:
		return err.Err
	}
	return err
}
