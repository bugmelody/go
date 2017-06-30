// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-19 21:31:43

// Package io provides basic interfaces to I/O primitives.
// Its primary job is to wrap existing implementations of such primitives,
// such as those in package os, into shared public interfaces that
// abstract the functionality, plus some other related primitives.
//
// Because these interfaces and primitives wrap lower-level operations with
// various implementations, unless otherwise informed clients should not
// assume they are safe for parallel execution.
package io

import (
	"errors"
)

// Seek whence values.
const (
	SeekStart   = 0 // seek relative to the origin of the file
	SeekCurrent = 1 // seek relative to the current offset
	SeekEnd     = 2 // seek relative to the end
)

// ErrShortWrite means that a write accepted fewer bytes than requested
// but failed to return an explicit error.
//
// 这里的write accepted是指实际写入的数据
// 请求写入x字节的数据,实际写入的数据小于x
var ErrShortWrite = errors.New("short write")

// ErrShortBuffer means that a read required a longer buffer than was provided.
//
// ErrShortBuffer表示读取操作需要大缓冲,但提供的缓冲不够大。
var ErrShortBuffer = errors.New("short buffer")

// EOF is the error returned by Read when no more input is available.
// Functions should return EOF only to signal a graceful end of input.
// If the EOF occurs unexpectedly in a structured data stream,
// the appropriate error is either ErrUnexpectedEOF or some other error
// giving more detail.
var EOF = errors.New("EOF")

// ErrUnexpectedEOF means that EOF was encountered in the
// middle of reading a fixed-size block or data structure.
//
// reading a (fixed-size block) or (data structure)
var ErrUnexpectedEOF = errors.New("unexpected EOF")

// ErrNoProgress is returned by some clients of an io.Reader when
// many calls to Read have failed to return any data or error,
// usually the sign of a broken io.Reader implementation.
var ErrNoProgress = errors.New("multiple Read calls return no data or error")

// Reader is the interface that wraps the basic Read method.
//
// Read reads up to len(p) bytes into p. It returns the number of bytes
// read (0 <= n <= len(p)) and any error encountered. Even if Read
// returns n < len(p), it may use all of p as scratch space during the call.
// If some data is available but not len(p) bytes, Read conventionally
// returns what is available instead of waiting for more.
//
// When Read encounters an error or end-of-file condition after
// successfully reading n > 0 bytes, it returns the number of
// bytes read. It may return the (non-nil) error from the same call
// or return the error (and n == 0) from a subsequent call.
// An instance of this general case is that a Reader returning
// a non-zero number of bytes at the end of the input stream may
// return either err == EOF or err == nil. The next Read should
// return 0, EOF.
//
// Callers should always process the n > 0 bytes returned before
// considering the error err. Doing so correctly handles I/O errors
// that happen after reading some bytes and also both of the
// allowed EOF behaviors.
//
// Implementations of Read are discouraged from returning a
// zero byte count with a nil error, except when len(p) == 0.
// Callers should treat a return of 0 and nil as indicating that
// nothing happened; in particular it does not indicate EOF.
//
// Implementations must not retain p.
//
//
// scratch space: 暂用空间,临时空间
// in particular: 尤其，特别
//
// 如果数据可用但不够len(p):习惯上,Read返回可用数据,而不是一直等待.
//
// 注意: Read 在每次调用的时候,不要求读取完毕,读取到的可能只是一部分,因此
// 调用方需要循环调用 Read 来进行所有数据的读取直到完毕
//
// 读取过程中发生的错误可能是本次调用Read返回,也可能是下次调用Read返回.
//
// 记住: Read 方法是需要多次调用的
type Reader interface {
	Read(p []byte) (n int, err error)
}

// Writer is the interface that wraps the basic Write method.
//
// Write writes len(p) bytes from p to the underlying data stream.
// It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// Write must return a non-nil error if it returns n < len(p).
// Write must not modify the slice data, even temporarily.
//
// Implementations must not retain p.
//
// 注意注意: [[Write must return a non-nil error if it returns n < len(p).]]
// Write期望将p的数据一次性写完, 如果没有写完, 应该报错.
type Writer interface {
	Write(p []byte) (n int, err error)
}

// Closer is the interface that wraps the basic Close method.
//
// The behavior of Close after the first call is undefined.
// Specific implementations may document their own behavior.
//
// 在第一次调用 Close 之后, 之后再次调用 Close 行为是未定
// 义的, 特定的实现可能会说明它们自己的行为.
type Closer interface {
	Close() error
}

// Seeker is the interface that wraps the basic Seek method.
//
// Seek sets the offset for the next Read or Write to offset,
// interpreted according to whence:
// SeekStart means relative to the start of the file,
// SeekCurrent means relative to the current offset, and
// SeekEnd means relative to the end.
// Seek returns the new offset relative to the start of the
// file and an error, if any.
//
// Seeking to an offset before the start of the file is an error.
// Seeking to any positive offset is legal, but the behavior of subsequent
// I/O operations on the underlying object is implementation-dependent.
//
// Seek会设置reciever的读写位移. 上文中的file可以看做receiver.
type Seeker interface {
	Seek(offset int64, whence int) (int64, error)
}

// ReadWriter is the interface that groups the basic Read and Write methods.
type ReadWriter interface {
	Reader
	Writer
}

// ReadCloser is the interface that groups the basic Read and Close methods.
type ReadCloser interface {
	Reader
	Closer
}

// WriteCloser is the interface that groups the basic Write and Close methods.
type WriteCloser interface {
	Writer
	Closer
}

// ReadWriteCloser is the interface that groups the basic Read, Write and Close methods.
type ReadWriteCloser interface {
	Reader
	Writer
	Closer
}

// ReadSeeker is the interface that groups the basic Read and Seek methods.
type ReadSeeker interface {
	Reader
	Seeker
}

// WriteSeeker is the interface that groups the basic Write and Seek methods.
type WriteSeeker interface {
	Writer
	Seeker
}

// ReadWriteSeeker is the interface that groups the basic Read, Write and Seek methods.
type ReadWriteSeeker interface {
	Reader
	Writer
	Seeker
}

// ReaderFrom is the interface that wraps the ReadFrom method.
//
// ReadFrom reads data from r until EOF or error.
// The return value n is the number of bytes read.
// Any error except io.EOF encountered during the read is also returned.
//
// The Copy function uses ReaderFrom if available.
//
// 读取过程中遇到io.EOF不会被当做错误返回.
// 希望一次性从r中读完.不像Read需要重复调用.
type ReaderFrom interface {
	ReadFrom(r Reader) (n int64, err error)
}

// WriterTo is the interface that wraps the WriteTo method.
//
// WriteTo writes data to w until there's no more data to write or
// when an error occurs. The return value n is the number of bytes
// written. Any error encountered during the write is also returned.
//
// The Copy function uses WriterTo if available.
//
// WriteTo期望将reciever的数据一次性全部写入w,如果写入不全,会返回错误.
type WriterTo interface {
	WriteTo(w Writer) (n int64, err error)
}

// ReaderAt is the interface that wraps the basic ReadAt method.
//
// ReadAt reads len(p) bytes into p starting at offset off in the
// underlying input source. It returns the number of bytes
// read (0 <= n <= len(p)) and any error encountered.
//
// When ReadAt returns n < len(p), it returns a non-nil error
// explaining why more bytes were not returned. In this respect,
// ReadAt is stricter than Read.
//
// 对比,Read方法除非确实是出错了,否则不会将n<len(p)视为错误.
// 而ReadAt在n<len(p)的情况下,一定会返回 error 解释为何没有读完.
// 也就是 ReadAt 要求一次性读完.
//
//
// Even if ReadAt returns n < len(p), it may use all of p as scratch
// space during the call. If some data is available but not len(p) bytes,
// ReadAt blocks until either all the data is available or an error occurs.
// In this respect ReadAt is different from Read.
//
// ReadAt 与 Read 的不同在于 ReadAt 如果读取数据长度不足会阻塞
//
// If the n = len(p) bytes returned by ReadAt are at the end of the
// input source, ReadAt may return either err == EOF or err == nil.
//
// err == EOF 可能是本轮返回,也可能是下一轮返回.
//
// If ReadAt is reading from an input source with a seek offset,
// ReadAt should not affect nor be affected by the underlying
// seek offset.
//
//
// 也就是说,ReadAt的off参数跟读取源的seek offset没有任何关系.
// ReaderAt是从读取源的off位置开始读取. 这个
// 可以参考 bytes.Reader.ReadAt 的源码,
// 里面是从 r.s[off:] 开始读, 跟 r.i 没有任何关系.
//
//
// Clients of ReadAt can execute parallel ReadAt calls on the
// same input source.
//
// Implementations must not retain p.
type ReaderAt interface {
	ReadAt(p []byte, off int64) (n int, err error)
}

// WriterAt is the interface that wraps the basic WriteAt method.
//
// WriteAt writes len(p) bytes from p to the underlying data stream
// at offset off. It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// WriteAt must return a non-nil error if it returns n < len(p).
//
// If WriteAt is writing to a destination with a seek offset,
// WriteAt should not affect nor be affected by the underlying
// seek offset.
//
// Clients of WriteAt can execute parallel WriteAt calls on the same
// destination if the ranges do not overlap.
//
// Implementations must not retain p.
//
// WriterAt会将p写入接受者的off.
// 要求一次性写完.
type WriterAt interface {
	WriteAt(p []byte, off int64) (n int, err error)
}

// ByteReader is the interface that wraps the ReadByte method.
//
// ReadByte reads and returns the next byte from the input or
// any error encountered. If ReadByte returns an error, no input
// byte was consumed, and the returned byte value is undefined.
type ByteReader interface {
	ReadByte() (byte, error)
}

// ByteScanner is the interface that adds the UnreadByte method to the
// basic ReadByte method.
//
// UnreadByte causes the next call to ReadByte to return the same byte
// as the previous call to ReadByte.
// It may be an error to call UnreadByte twice without an intervening
// call to ReadByte.
type ByteScanner interface {
	/* interface 内嵌匿名 interface */
	ByteReader
	UnreadByte() error
}

// ByteWriter is the interface that wraps the WriteByte method.
type ByteWriter interface {
	WriteByte(c byte) error
}

// RuneReader is the interface that wraps the ReadRune method.
//
// ReadRune reads a single UTF-8 encoded Unicode character
// and returns the rune and its size in bytes. If no character is
// available, err will be set.
type RuneReader interface {
	ReadRune() (r rune, size int, err error)
}

// RuneScanner is the interface that adds the UnreadRune method to the
// basic ReadRune method.
//
// UnreadRune causes the next call to ReadRune to return the same rune
// as the previous call to ReadRune.
// It may be an error to call UnreadRune twice without an intervening
// call to ReadRune.
type RuneScanner interface {
	RuneReader
	UnreadRune() error
}

// stringWriter is the interface that wraps the WriteString method.
//
// 此接口是非导出的,仅仅用于在func WriteString中进行能力检测
type stringWriter interface {
	WriteString(s string) (n int, err error)
}

// WriteString writes the contents of the string s to w, which accepts a slice of bytes.
// If w implements a WriteString method, it is invoked directly.
// Otherwise, w.Write is called exactly once.
func WriteString(w Writer, s string) (n int, err error) {
	if sw, ok := w.(stringWriter); ok {
		// 如果w实现了stringWriter接口,
		// 也就是,如果可以在w上调用WriteString方法
		// stringWriter 接口对应的 WriteString 方法会被直接调用
		// 这是为了避免频繁的在byte和string之间做转换
		return sw.WriteString(s)
	}
	// 否则,将s转型为[]byte后写入w(类型转换会造成一次内存分配,性能较低)
	// 根据文档:w.Write is called exactly once.
	return w.Write([]byte(s))
}

// ReadAtLeast reads from r into buf until it has read at least min bytes.
// It returns the number of bytes copied and an error if fewer bytes were read.
// The error is EOF only if no bytes were read.
// If an EOF happens after reading fewer than min bytes,
// ReadAtLeast returns ErrUnexpectedEOF.
// If min is greater than the length of buf, ReadAtLeast returns ErrShortBuffer.
// On return, n >= min if and only if err == nil.
//
//
// ReadAtLeast从r读取数据到buf,至少读够min bytes,不够min bytes则视为错误.
// 函数内部循环调用了r.Read,直到满足条件.
// 注意:The error is EOF only if no bytes were read.
func ReadAtLeast(r Reader, buf []byte, min int) (n int, err error) {
	if len(buf) < min {
		// len(buf) < min 说明 buf 太小了,不够空间装
		// 文档:If min is greater than the length of buf, ReadAtLeast returns ErrShortBuffer.
		return 0, ErrShortBuffer
	}
	// 现在,buf中空间肯定是足够的,循环调用r.Read进行读取,直到读够或者出错为止
	for n < min && err == nil {
		// nn 代表本轮 Read 的长度
		var nn int
		// 因为ReadAtLeast规定了r是io.Reader,根据函数签名,r肯定有Read方法
		nn, err = r.Read(buf[n:])
		// 将本轮读取到的字节数nn加到总的读取字节数n上面
		n += nn
	}
	// 到这里,说明要么读取完毕,要么出错
	// 下面对err进行修正
	if n >= min {
		// ReadAtLeast意思是至少读取min字节
		// 因此如果n>=min,说明已经读取完毕,没有错误
		// 为什么还要这样做?因为可能在上面的for循环中最后一次读取的时
		// 候,已经读够了,却返回错误,此时应该整个函数不算错

		// 只要读够了,就不应该返回错误
		err = nil
	} else if n > 0 && err == EOF {
		// 文档:If an EOF happens after reading fewer than min
		// bytes, ReadAtLeast returns ErrUnexpectedEOF.
		err = ErrUnexpectedEOF
	}
	return
}

// ReadFull reads exactly len(buf) bytes from r into buf.
// It returns the number of bytes copied and an error if fewer bytes were read.
// The error is EOF only if no bytes were read.
// If an EOF happens after reading some but not all the bytes,
// ReadFull returns ErrUnexpectedEOF.
// On return, n == len(buf) if and only if err == nil.
//
// ReadFull 顾名思义,要把 buf 读满.
// 注意:The error is EOF only if no bytes were read.
func ReadFull(r Reader, buf []byte) (n int, err error) {
	// ReadFull实现还是通过调用ReadAtLeast,只不过是将最小读取字节数精确设置成了buf的长度
	return ReadAtLeast(r, buf, len(buf))
}

// CopyN copies n bytes (or until an error) from src to dst.
// It returns the number of bytes copied and the earliest
// error encountered while copying.
// On return, written == n if and only if err == nil.
//
// If dst implements the ReaderFrom interface,
// the copy is implemented using it.
//
// CopyN会从src拷贝n个字节到dst.
//
// 注意:这里文档并没有说'If src implements the WriterTo interface'的条件,这是没有问题的,参考源码
func CopyN(dst Writer, src Reader, n int64) (written int64, err error) {
	// 通过构造LimitReader,来限制src中要读出的字节数
	// 注意: LimitReader 并没有实现 WriterTo interface, 因此, 根据 io.Copy 的说明: 并不会出现调用 src.WriteTo(dst) 的情况
	// 因此 CopyN 的文档中只说了 : If dst implements the ReaderFrom interface, the copy is implemented using it.
	written, err = Copy(dst, LimitReader(src, n))
	if written == n {
		// 文档: On return, written == n if and only if err == nil.
		return n, nil
	}
	// 现在,written不等于n,说明出错
	if written < n && err == nil {
		// src stopped early; must have been EOF.
		err = EOF
	}
	return
}

// Copy copies from src to dst until either EOF is reached
// on src or an error occurs. It returns the number of bytes
// copied and the first error encountered while copying, if any.
//
// A successful Copy returns err == nil, not err == EOF.
// Because Copy is defined to read from src until EOF, it does
// not treat an EOF from Read as an error to be reported.
//
// If src implements the WriterTo interface,
// the copy is implemented by calling src.WriteTo(dst).
// Otherwise, if dst implements the ReaderFrom interface,
// the copy is implemented by calling dst.ReadFrom(src).
//
//
// Copy内部会自动分配一个中间buffer进行转接.
//
// 参考: func TestCopyPriority,里面提到:
// It's preferable to choose WriterTo over ReaderFrom, since a WriterTo can issue one large write,
// while the ReaderFrom must read until EOF, potentially allocating when running out of buffer.
func Copy(dst Writer, src Reader) (written int64, err error) {
	// nil表示内部会自动分配一个中间buffer进行转接.
	return copyBuffer(dst, src, nil)
}

// CopyBuffer is identical to Copy except that it stages through the
// provided buffer (if one is required) rather than allocating a
// temporary one. If buf is nil, one is allocated; otherwise if it has
// zero length, CopyBuffer panics.
//
// 如果buf不是nil,CopyBuffer会使用指定的buf作为中间buffer进行转接.
// 如果buf是nil,函数内部会自动分配一个buffer.
func CopyBuffer(dst Writer, src Reader, buf []byte) (written int64, err error) {
	if buf != nil && len(buf) == 0 {
		panic("empty buffer in io.CopyBuffer")
	}
	return copyBuffer(dst, src, buf)
}

// copyBuffer is the actual implementation of Copy and CopyBuffer.
// if buf is nil, one is allocated.
//
// 如果buf是nil,内部会分配一个buffer.
// 如果buf不是nil,会直接将buf作为缓冲
func copyBuffer(dst Writer, src Reader, buf []byte) (written int64, err error) {
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(WriterTo); ok {
		// 这是最优情况,会避免内存分配和拷贝
		return wt.WriteTo(dst)
	}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rt, ok := dst.(ReaderFrom); ok {
		// 这是最优情况,会避免内存分配和拷贝
		return rt.ReadFrom(src)
	}
	if buf == nil {
		// 如果buf是nil,会分配一个buffer内存
		buf = make([]byte, 32*1024)
	}
	for {
		// 每轮循环中,从src中读取数据到buf,然后再从buf写入dst,也就是使用buf进行了中转.
		// nr代表本轮读取的字节数; er代表,本轮读取时返回的err
		// nr: number of read, er: error of read
		nr, er := src.Read(buf)
		// 根据Read的文档,应当优先处理返回的数据,然后再处理error
		if nr > 0 {
			// 如果读取到了数据(nr>0),将之写入dst
			// nw: number of write, ew: error of write
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				// written 是函数的返回值,代表 the number of bytes copied
				written += int64(nw)
			}
			if ew != nil {
				// 如果本轮在dst.Write的时候出了错,跳出死循环
				err = ew
				break
			}
			if nr != nw {
				// number of bytes read != number of bytes write
				// 说明该写nr字节,但只写了nw字节,写的太少,跳出死循环
				err = ErrShortWrite
				break
			}
		}
		// 根据Read的文档,应当优先处理返回的数据,然后再处理error
		if er != nil {
			if er != EOF {
				err = er
			}
			// 根据文档:Because Copy is defined to read from src until EOF, it does
			// not treat an EOF from Read as an error to be reported.
			break
		}
	}
	return written, err
}

// LimitReader returns a Reader that reads from r
// but stops with EOF after n bytes.
// The underlying implementation is a *LimitedReader.
//
// LimitReader函数用于构造LimitedReader对象
// 注意:函数LimitReader返回的还是一个io.Reader,只是它的底层是*LimitedReader.
func LimitReader(r Reader, n int64) Reader { return &LimitedReader{r, n} }

// A LimitedReader reads from R but limits the amount of
// data returned to just N bytes. Each call to Read
// updates N to reflect the new amount remaining.
// Read returns EOF when N <= 0 or when the underlying R returns EOF.
type LimitedReader struct {
	R Reader // underlying reader
	// 剩余还能读取多少字节
	N int64  // max bytes remaining
}

func (l *LimitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		// 已经读完了
		return 0, EOF
	}
	if int64(len(p)) > l.N {
		// p的空间太大了,大于可以读取到的字节数(l.N),没有必要
		p = p[0:l.N]
	}
	// 现在,p的长度是剩余数据的长度
	// 从l.R中读取数据到p
	n, err = l.R.Read(p)
	// 这里可能返回EOF,根据LimitedReader文档: Read returns EOF when N <= 0 or when the underlying R returns EOF.
	l.N -= int64(n)
	return
}

// NewSectionReader returns a SectionReader that reads from r
// starting at offset off and stops with EOF after n bytes.
//
// 相当于是LimitReader函数的升级版本,在LimitReader的基础上增加了off
// LimitReader是取底层数据的N个字节后完毕
// NewSectionReader是从底层数据的off开始,取底层数据的N个字节后完毕
func NewSectionReader(r ReaderAt, off int64, n int64) *SectionReader {
	return &SectionReader{r, off, off, off + n}
}

// SectionReader implements Read, Seek, and ReadAt on a section
// of an underlying ReaderAt.
type SectionReader struct {
	r     ReaderAt
	// SectionReader读取的起始位移,此值不会变
	base  int64
	// SectionReader读取的当前位移,此值会变
	// off是相对于base而言的位置
	// SectionReader.Seek会修改此值
	off   int64
	// 限制读取到什么位置,代表一个位移,而不是count
	limit int64
}

func (s *SectionReader) Read(p []byte) (n int, err error) {
	if s.off >= s.limit {
		// 已经读取完毕
		return 0, EOF
	}
	// max := s.limit - s.off; 也就是在s这个SectionReader中能读取的最大长度
	// 通过max和p长度进行比较,从而修正p这个slice 的长度
	if max := s.limit - s.off; int64(len(p)) > max {
		// max代表了本轮最大可以读取到的字节数
		// p代表本轮应该读取的字节数
		p = p[0:max]
	}
	// 从s的底层数据r的s.off位置读取数据到p
	n, err = s.r.ReadAt(p, s.off)
	s.off += int64(n)
	return
}

var errWhence = errors.New("Seek: invalid whence")
var errOffset = errors.New("Seek: invalid offset")

// Seeker interface 要求实现的方法
func (s *SectionReader) Seek(offset int64, whence int) (int64, error) {
	// 根据 whence 的值来决定真实的 offset
	switch whence {
	default:
		// whence的取值只能是固定的三个值之一,其余值都是非法
		return 0, errWhence
	case SeekStart:
		// SeekStart means relative to the start of the file
		// s.base相当于是start of the file
		offset += s.base
	case SeekCurrent:
		// SeekCurrent means relative to the current offset
		// s.off相当于是current offset
		offset += s.off
	case SeekEnd:
		// 将s.limit看做是终点
		offset += s.limit
	}
	if offset < s.base {
		// 如果刚才计算出来的 offset 不合法,返回错误信息
		// 上面对offset计算之后,offset不应该小于起始位移base,如果小于,说明offset参数设置错误
		// Seeking to an offset before the start of the file is an error.
		return 0, errOffset
	}
	// 现在 offset 变量合法,设置到 s.off,也就是下次read或write的位移
	s.off = offset
	// 根据Seeker接口规范, Seek returns the new offset relative to the start of the file and an error, if any.
	return offset - s.base, nil
}

// ReaderAt 接口要求实现
// 注意:ReadAt的off参数跟读取源的seek offset没有任何关系.ReaderAt是从读取源的off参数位置开始读取.
// 参数off:从s.base之后多少字节开始读取
func (s *SectionReader) ReadAt(p []byte, off int64) (n int, err error) {
	// ...base...limit
	if off < 0 || off >= s.limit-s.base {
		// off是相对于base而言的位置
		// off应该在base和limit之间,如果不在之间,返回EOF
		return 0, EOF
	}
	off += s.base
	// 现在,off代表相对于s.r起始位置的位移
	// max代表了最大可能读取多少字节
	if max := s.limit - off; int64(len(p)) > max {
		// 如果len(p)大于最大可能读取到的字节数,修正p
		p = p[0:max]
		n, err = s.r.ReadAt(p, off)
		if err == nil {
			err = EOF
		}
		return n, err
	}
	// 如果len(p)<=最大可能读取到的字节数,不用修正,
	// 直接调用s.r.ReadAt,使用 s.r.ReadAt 的返回值
	// 委托调用
	return s.r.ReadAt(p, off)
}

// Size returns the size of the section in bytes.
//
// 返回你设置的区间(section)有多少个字节
// 所谓的 section , 是指 s.base 到 s.limit 的这段区间
func (s *SectionReader) Size() int64 { return s.limit - s.base }

// TeeReader returns a Reader that writes to w what it reads from r.
// All reads from r performed through it are matched with
// corresponding writes to w. There is no internal buffering -
// the write must complete before the read completes.
// Any error encountered while writing is reported as a read error.
//
// 通过观察源码发现:
// 通过此函数返回的Reader,会类似linux中的tee命令.比如:
// teeReader := io.TeeReader(r,w)
// teeReader.Read(p)
// 此时,仍然可以通过p获取到从r中读取的数据.
// 但是,这些被读取到的数据也同时被写入了w.
// 因此也可以根据向w写入的数据拿到p中的数据
func TeeReader(r Reader, w Writer) Reader {
	return &teeReader{r, w}
}

type teeReader struct {
	// interface类型,只要能满足io.Reader接口,就能存入字段
	r Reader
	// interface类型,只要能满足io.Reader接口,就能存入字段
	w Writer
}

func (t *teeReader) Read(p []byte) (n int, err error) {
	n, err = t.r.Read(p)
	if n > 0 {
		// 如果从t.r中读取到了数据,写入t.w中
		// 注意对照 t.w.Write 的文档, Write must return a non-nil error if it returns n < len(p).
		// 也就是说,写入的字节数应该等于n,否则说明有错
		// -- 注意:这里使用了n, err := t.w.Write(p[:n]),按理说上面已经定义了n和err,这里不能这样写
		// 其实是因为if语句有自己的作用域
		if n, err := t.w.Write(p[:n]); err != nil {
			// 如果出了错 ,直接返回这个错误
			// 根据 TeeReader 的文档: Any error encountered while writing is reported as a read error.
			return n, err
		}
	}
	// 如果从t.r中没有读取到数据,将t.r.Read(p)的返回值返回
	return
	/**
	可以用如下代码验证下作用域:
	func main() {
		x := "x"
		y := "y"
		if(true){
			x,y := d()
			fmt.Println(x,y)
		}
		fmt.Println(x,y)
	}
	
	func d() (string, string){
		return "X", "Y"
	}
	 */
}
