// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[6-over]]] 2017-6-12 11:27:40

package os

import (
	"syscall"
	"time"
)

// Getpagesize returns the underlying system's memory page size.
//
// Getpagesize 获取操作系统的内存页大小
func Getpagesize() int { return syscall.Getpagesize() }

// File represents an open file descriptor.
type File struct {
	*file // os specific
}

// A FileInfo describes a file and is returned by Stat and Lstat.
type FileInfo interface {
	Name() string       // base name of the file
	Size() int64        // length in bytes for regular files; system-dependent for others
	Mode() FileMode     // file mode bits
	ModTime() time.Time // modification time
	IsDir() bool        // abbreviation for Mode().IsDir()
	Sys() interface{}   // underlying data source (can return nil)
}

// A FileMode represents a file's mode and permission bits.
// The bits have the same definition on all systems, so that
// information about files can be moved from one system
// to another portably. Not all bits apply to all systems.
// The only required bit is ModeDir for directories.
type FileMode uint32

// The defined file mode bits are the most significant bits of the FileMode.
// The nine least-significant bits are the standard Unix rwxrwxrwx permissions.
// The values of these bits should be considered part of the public API and
// may be used in wire protocols or disk representations: they must not be
// changed, although new bits might be added.
//
// The nine least-significant bits(9个最低有效位)
// least significant bit [计] 最低有效位
const (
	// The single letters are the abbreviations
	// used by the String method's formatting.
	//
	// 这刚好是32位,因为之前定义了 type FileMode uint32
	// 10000000000000000000000000000000
	ModeDir        FileMode = 1 << (32 - 1 - iota) // d: is a directory
	// 01000000000000000000000000000000
	ModeAppend                                     // a: append-only
	// 00100000000000000000000000000000
	ModeExclusive                                  // l: exclusive use
	ModeTemporary                                  // T: temporary file; Plan 9 only
	ModeSymlink                                    // L: symbolic link
	ModeDevice                                     // D: device file
	ModeNamedPipe                                  // p: named pipe (FIFO)
	ModeSocket                                     // S: Unix domain socket
	ModeSetuid                                     // u: setuid
	ModeSetgid                                     // g: setgid
	ModeCharDevice                                 // c: Unix character device, when ModeDevice is set
	ModeSticky                                     // t: sticky

	// Mask for the type bits. For regular files, none will be set.
	// 也就是说, 对于 regular files, ModeType 中的 bit 都不会被设置
	ModeType = ModeDir | ModeSymlink | ModeNamedPipe | ModeSocket | ModeDevice

	// 00000000000a00000000000111111111
	//                        rwxrwxrwx
	ModePerm FileMode = 0777 // Unix permission bits
)

func (m FileMode) String() string {
	// 参考 ModeDir 等常量的注释, 其中有每个常量的意义缩写
	const str = "dalTLDpSugct"
	// type FileMode 定义为 uint32
	// buf中的每个字节代表FileMode中的一个bit
	var buf [32]byte // Mode is uint32.
	// buf[:w] 是最后要返回的内容, w代表了buf返回时的截取位置
	w := 0
	for i, c := range str {
		if m&(1<<uint(32-1-i)) != 0 {
			// 将"dalTLDpSugct"中的任一字节添加进buf
			buf[w] = byte(c)
			w++
		}
	}
	// 如果之前的for循环未向buf写入任何内容
	if w == 0 {
		buf[w] = '-'
		w++
	}
	const rwx = "rwxrwxrwx"
	/**
	"rwxrwxrwx" 是FileMode中最右边的9位
	"rwxrwxrwx"
	 111111111
	 */
	for i, c := range rwx {
		if m&(1<<uint(9-1-i)) != 0 {
			buf[w] = byte(c)
		} else {
			// 该位未设置
			buf[w] = '-'
		}
		w++
	}
	return string(buf[:w])
}

// IsDir reports whether m describes a directory.
// That is, it tests for the ModeDir bit being set in m.
func (m FileMode) IsDir() bool {
	// 测试 ModeDir bit 是否被设置.
	return m&ModeDir != 0
}

// IsRegular reports whether m describes a regular file.
// That is, it tests that no mode type bits are set.
func (m FileMode) IsRegular() bool {
	// 测试 ModeType 中的 bit 没有被设置
	return m&ModeType == 0
}

// Perm returns the Unix permission bits in m.
func (m FileMode) Perm() FileMode {
	// 注意, receiver 是 FileMode, 返回值仍然是 FileMode
	// m的类型是FileMode,ModePerm的类型也是FileMode
	return m & ModePerm
}

// 注意: A fileStat is the implementation of FileInfo returned by Stat and Lstat.
// fileStat是各操作系统对FileInfo接口的实现.
// fileStat 是由各个操作系统的代码单独定义的
func (fs *fileStat) Name() string { return fs.name }
func (fs *fileStat) IsDir() bool  { return fs.Mode().IsDir() }

// SameFile reports whether fi1 and fi2 describe the same file.
// For example, on Unix this means that the device and inode fields
// of the two underlying structures are identical; on other systems
// the decision may be based on the path names.
// SameFile only applies to results returned by this package's Stat.
// It returns false in other cases.
//
// FileInfo 是一个接口, SameFile 虽然支持 FileInfo 接口参数, 但是这里
// 明确说明只支持本package中Stat返回的结果.
func SameFile(fi1, fi2 FileInfo) bool {
	// 确认 fi1, fi2 是本包中对 FileInfo 接口的实现.
	fs1, ok1 := fi1.(*fileStat)
	fs2, ok2 := fi2.(*fileStat)
	if !ok1 || !ok2 {
		return false
	}
	return sameFile(fs1, fs2)
}
