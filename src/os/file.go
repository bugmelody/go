// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[6-over]]] 2017-6-12 10:07:10

// Package os provides a platform-independent interface to operating system
// functionality. The design is Unix-like, although the error handling is
// Go-like; failing calls return values of type error rather than error numbers.
// Often, more information is available within the error. For example,
// if a call that takes a file name fails, such as Open or Stat, the error
// will include the failing file name when printed and will be of type
// *PathError, which may be unpacked for more information.
//
// platform-independent interface(跨平台的接口)
//
// The os interface is intended to be uniform across all operating systems.
// Features not generally available appear in the system-specific package syscall.
//
// uniform ['juːnɪfɔːm] adj. 统一的；一致的；相同的；均衡的；始终如一的 n. 制服 vt. 使穿制服；使成一样
//
// Here is a simple example, opening a file and reading some of it.
//
//	file, err := os.Open("file.go") // For read access.
//	if err != nil {
//		log.Fatal(err)
//	}
//
// If the open fails, the error string will be self-explanatory, like
//
//	open file.go: no such file or directory
//
// The file's data can then be read into a slice of bytes. Read and
// Write take their byte counts from the length of the argument slice.
//
// Read和Write会使用length of the argument slice作为要读取的字节数或是要写入的字节数.
//
//	data := make([]byte, 100)
//	count, err := file.Read(data)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("read %d bytes: %q\n", count, data[:count])
//
package os

import (
	"errors"
	"internal/poll"
	"io"
	"syscall"
)

// Name returns the name of the file as presented to Open.
//
// Name 返回的就是 os.Open 中传入的
func (f *File) Name() string { return f.name }

// Stdin, Stdout, and Stderr are open Files pointing to the standard input,
// standard output, and standard error file descriptors.
//
// Note that the Go runtime writes to standard error for panics and crashes;
// closing Stderr may cause those messages to go elsewhere, perhaps
// to a file opened later.
//
// 因此,最好不要 closing Stderr
var (
	Stdin  = NewFile(uintptr(syscall.Stdin), "/dev/stdin")
	Stdout = NewFile(uintptr(syscall.Stdout), "/dev/stdout")
	Stderr = NewFile(uintptr(syscall.Stderr), "/dev/stderr")
)

// Flags to OpenFile wrapping those of the underlying system. Not all
// flags may be implemented on a given system.
const (
	O_RDONLY int = syscall.O_RDONLY // open the file read-only.
	O_WRONLY int = syscall.O_WRONLY // open the file write-only.
	O_RDWR   int = syscall.O_RDWR   // open the file read-write.
	O_APPEND int = syscall.O_APPEND // append data to the file when writing.
	O_CREATE int = syscall.O_CREAT  // create a new file if none exists.
	O_EXCL   int = syscall.O_EXCL   // used with O_CREATE, file must not exist
	O_SYNC   int = syscall.O_SYNC   // open for synchronous I/O.
	O_TRUNC  int = syscall.O_TRUNC  // if possible, truncate file when opened.
)

// Seek whence values.
//
// Deprecated: Use io.SeekStart, io.SeekCurrent, and io.SeekEnd.
const (
	SEEK_SET int = 0 // seek relative to the origin of the file
	SEEK_CUR int = 1 // seek relative to the current offset
	SEEK_END int = 2 // seek relative to the end
)

// LinkError records an error during a link or symlink or rename
// system call and the paths that caused it.
type LinkError struct {
	Op  string
	Old string
	New string
	Err error
}

func (e *LinkError) Error() string {
	return e.Op + " " + e.Old + " " + e.New + ": " + e.Err.Error()
}

// Read reads up to len(b) bytes from the File.
// It returns the number of bytes read and any error encountered.
// At end of file, Read returns 0, io.EOF.
//
// 如何判断文件读取完毕: (n==0 && err==io.EOF)
func (f *File) Read(b []byte) (n int, err error) {
	if err := f.checkValid("read"); err != nil {
		return 0, err
	}
	// f.read(b) 是每个操作系统特定的读取函数
	n, e := f.read(b)
	return n, f.wrapErr("read", e)
}

// ReadAt reads len(b) bytes from the File starting at byte offset off.
// It returns the number of bytes read and the error, if any.
// ReadAt always returns a non-nil error when n < len(b).
// At end of file, that error is io.EOF.
//
// 将f从off之后的所有数据读入b
func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	if err := f.checkValid("read"); err != nil {
		return 0, err
	}

	if off < 0 {
		return 0, &PathError{"readat", f.name, errors.New("negative offset")}
	}

	// len(b): 剩余需要读取的字节数, 变量 b 在每个循环中会重新 slice
	// 只要 len(b) > 0 , 说明还需要再读取数据
	for len(b) > 0 {
		m, e := f.pread(b, off)
		if e != nil {
			err = f.wrapErr("read", e)
			break
		}
		// n 是整个函数的已读取字节数, m 是本次循环中 f.pread 读取到的字节数
		// 更新 n
		n += m
		// 对 b 重新 slice,因为每次for循环的时候,都要用 len(b) 来判断是否还有操作需要继续
		b = b[m:]
		// 更新偏移量,下次for循环从off开始读取
		off += int64(m)
	}
	return
}

// Write writes len(b) bytes to the File.
// It returns the number of bytes written and an error, if any.
// Write returns a non-nil error when n != len(b).
//
// 注意: 当写入的字节数 != len(b) 的时候,会返回非空的err
// Write要求将b全部写入f,没有全部写入算作错误.
func (f *File) Write(b []byte) (n int, err error) {
	if err := f.checkValid("write"); err != nil {
		return 0, err
	}
	// f.write 是每个操作系统独立的写入函数
	n, e := f.write(b)
	if n < 0 {
		n = 0
	}
	if n != len(b) {
		err = io.ErrShortWrite
	}

	epipecheck(f, e)

	if e != nil {
		err = f.wrapErr("write", e)
	}

	return n, err
}

// WriteAt writes len(b) bytes to the File starting at byte offset off.
// It returns the number of bytes written and an error, if any.
// WriteAt returns a non-nil error when n != len(b).
//
// 注意: off 是指 receiver f 的位移
// Write要求将b全部写入f的off位移,没有全部写入算作错误.
func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	if err := f.checkValid("write"); err != nil {
		return 0, err
	}

	if off < 0 {
		return 0, &PathError{"writeat", f.name, errors.New("negative offset")}
	}

	for len(b) > 0 {
		// b 中还有待写入的数据
		m, e := f.pwrite(b, off)
		if e != nil {
			err = f.wrapErr("write", e)
			break
		}
		// n 是整个函数的已写入字节数, m 是本次循环中 f.pwrite 写入的字节数
		n += m
		// 待写入的数据
		b = b[m:]
		// 更新偏移量,下次for循环从off开始写入
		off += int64(m)
	}
	return
}

// Seek sets the offset for the next Read or Write on file to offset, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end.
// It returns the new offset and an error, if any.
// The behavior of Seek on a file opened with O_APPEND is not specified.
//
// 如果一个file是通过O_APPEND打开的,则不应该在这个file上面调用Seek(行为未定义).
func (f *File) Seek(offset int64, whence int) (ret int64, err error) {
	if err := f.checkValid("seek"); err != nil {
		return 0, err
	}
	r, e := f.seek(offset, whence)
	if e == nil && f.dirinfo != nil && r != 0 {
		// f.dirinfo != nil : 读取的是目录
		// 操作的是目录,但是目录不应该有 seek 操作
		// 修正 e
		e = syscall.EISDIR
	}
	if e != nil {
		return 0, f.wrapErr("seek", e)
	}
	return r, nil
}

// WriteString is like Write, but writes the contents of string s rather than
// a slice of bytes.
func (f *File) WriteString(s string) (n int, err error) {
	return f.Write([]byte(s))
}

// Mkdir creates a new directory with the specified name and permission bits.
// If there is an error, it will be of type *PathError.
func Mkdir(name string, perm FileMode) error {
	e := syscall.Mkdir(fixLongPath(name), syscallMode(perm))

	if e != nil {
		return &PathError{"mkdir", name, e}
	}

	// mkdir(2) itself won't handle the sticky bit on *BSD and Solaris
	if !supportsCreateWithStickyBit && perm&ModeSticky != 0 {
		Chmod(name, perm)
	}

	return nil
}

// Chdir changes the current working directory to the named directory.
// If there is an error, it will be of type *PathError.
func Chdir(dir string) error {
	if e := syscall.Chdir(dir); e != nil {
		return &PathError{"chdir", dir, e}
	}
	return nil
}

// Open opens the named file for reading. If successful, methods on
// the returned file can be used for reading; the associated file
// descriptor has mode O_RDONLY.
// If there is an error, it will be of type *PathError.
func Open(name string) (*File, error) {
	return OpenFile(name, O_RDONLY, 0)
}

// Create creates the named file with mode 0666 (before umask), truncating
// it if it already exists. If successful, methods on the returned
// File can be used for I/O; the associated file descriptor has mode
// O_RDWR.
// If there is an error, it will be of type *PathError.
func Create(name string) (*File, error) {
	return OpenFile(name, O_RDWR|O_CREATE|O_TRUNC, 0666)
}

// lstat is overridden in tests.
var lstat = Lstat

// Rename renames (moves) oldpath to newpath.
// If newpath already exists and is not a directory, Rename replaces it.
// OS-specific restrictions may apply when oldpath and newpath are in different directories.
// If there is an error, it will be of type *LinkError.
func Rename(oldpath, newpath string) error {
	return rename(oldpath, newpath)
}

// Many functions in package syscall return a count of -1 instead of 0.
// Using fixCount(call()) instead of call() corrects the count.
func fixCount(n int, err error) (int, error) {
	if n < 0 {
		n = 0
	}
	return n, err
}

// wrapErr wraps an error that occurred during an operation on an open file.
// It passes io.EOF through unchanged, otherwise converts
// poll.ErrFileClosing to ErrClosed and wraps the error in a PathError.
func (f *File) wrapErr(op string, err error) error {
	if err == nil || err == io.EOF {
		return err
	}
	if err == poll.ErrFileClosing {
		err = ErrClosed
	}
	return &PathError{op, f.name, err}
}

// TempDir returns the default directory to use for temporary files.
//
// On Unix systems, it returns $TMPDIR if non-empty, else /tmp.
// On Windows, it uses GetTempPath, returning the first non-empty
// value from %TMP%, %TEMP%, %USERPROFILE%, or the Windows directory.
// On Plan 9, it returns /tmp.
//
// The directory is neither guaranteed to exist nor have accessible
// permissions.
func TempDir() string {
	return tempDir()
}
