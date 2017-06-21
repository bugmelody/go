// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[3-over]]] 2017-6-20 16:37:01

package io_test
// 注意这个 package name, 是 external test package

import (
	"bytes"
	"errors"
	"fmt"
	. "io"
	"strings"
	"testing"
)

// An version of bytes.Buffer without ReadFrom and WriteTo
type Buffer struct {
	// bytes.Buffer拥有ReadFrom和WriteTo方法
	bytes.Buffer
	// interface 的 zero value 是 nil
	ReaderFrom // conflicts with and hides bytes.Buffer's ReaderFrom.
	// interface 的 zero value 是 nil
	WriterTo   // conflicts with and hides bytes.Buffer's WriterTo.
}

// Simple tests, primarily to verify the ReadFrom and WriteTo callouts inside Copy, CopyBuffer and CopyN.

func TestCopy(t *testing.T) {
	// Buffer未实现ReaderFrom和WriterTo
	rb := new(Buffer)
	// Buffer未实现ReaderFrom和WriterTo
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	// 对于 io.Copy, 文档中提到
	// If src implements the WriterTo interface,
	// the copy is implemented by calling src.WriteTo(dst).
	// Otherwise, if dst implements the ReaderFrom interface,
	// the copy is implemented by calling dst.ReadFrom(src).
	Copy(wb, rb)
	if wb.String() != "hello, world." {
		t.Errorf("Copy did not work properly")
	}
}

func TestCopyBuffer(t *testing.T) {
	// Buffer未实现ReaderFrom和WriterTo
	rb := new(Buffer)
	// Buffer未实现ReaderFrom和WriterTo
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	// 注意, CopyBuffer 与 Copy 一样
	// If src implements the WriterTo interface,
	// the copy is implemented by calling src.WriteTo(dst).
	// Otherwise, if dst implements the ReaderFrom interface,
	// the copy is implemented by calling dst.ReadFrom(src).
	CopyBuffer(wb, rb, make([]byte, 1)) // Tiny buffer to keep it honest.
	if wb.String() != "hello, world." {
		t.Errorf("CopyBuffer did not work properly")
	}
}

func TestCopyBufferNil(t *testing.T) {
	// Buffer未实现ReaderFrom和WriterTo
	rb := new(Buffer)
	// Buffer未实现ReaderFrom和WriterTo
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	// 根据CopyBuffer的文档,If buf is nil, one is allocated; otherwise if it has zero length, CopyBuffer panics.
	// 注意, CopyBuffer 与 Copy 一样
	// If src implements the WriterTo interface,
	// the copy is implemented by calling src.WriteTo(dst).
	// Otherwise, if dst implements the ReaderFrom interface,
	// the copy is implemented by calling dst.ReadFrom(src).
	CopyBuffer(wb, rb, nil) // Should allocate a buffer.
	if wb.String() != "hello, world." {
		t.Errorf("CopyBuffer did not work properly")
	}
}

func TestCopyReadFrom(t *testing.T) {
	// Buffer未实现ReaderFrom和WriterTo
	rb := new(Buffer)
	// bytes.Buffer实现了ReaderFrom和WriterTo
	wb := new(bytes.Buffer) // implements ReadFrom.
	rb.WriteString("hello, world.")
	Copy(wb, rb)
	if wb.String() != "hello, world." {
		t.Errorf("Copy did not work properly")
	}
}

func TestCopyWriteTo(t *testing.T) {
	// bytes.Buffer实现了ReaderFrom和WriterTo
	rb := new(bytes.Buffer) // implements WriteTo.
	// Buffer未实现ReaderFrom和WriterTo
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	Copy(wb, rb)
	if wb.String() != "hello, world." {
		t.Errorf("Copy did not work properly")
	}
}

// Version of bytes.Buffer that checks whether WriteTo was called or not
type writeToChecker struct {
	// 匿名struct
	bytes.Buffer
	// WriteTo 方法是否被调用
	writeToCalled bool
}

func (wt *writeToChecker) WriteTo(w Writer) (int64, error) {
	wt.writeToCalled = true
	return wt.Buffer.WriteTo(w)
}

// It's preferable to choose WriterTo over ReaderFrom, since a WriterTo can issue one large write,
// while the ReaderFrom must read until EOF, potentially allocating when running out of buffer.
// Make sure that we choose WriterTo when both are implemented.
func TestCopyPriority(t *testing.T) {
	rb := new(writeToChecker)
	wb := new(bytes.Buffer)
	rb.WriteString("hello, world.")
	Copy(wb, rb)
	if wb.String() != "hello, world." {
		t.Errorf("Copy did not work properly")
	} else if !rb.writeToCalled {
		// 即使 Copy 成功, 仍然要检查一下  rb.WriteTo 是否被调用
		t.Errorf("WriteTo was not prioritized over ReadFrom")
	}
}

type zeroErrReader struct {
	err error
}

func (r zeroErrReader) Read(p []byte) (int, error) {
	// ??? []byte{0} ???
	// 使用0作为[]byte的第一个元素
	return copy(p, []byte{0}), r.err
}

type errWriter struct {
	err error
}

func (w errWriter) Write([]byte) (int, error) {
	return 0, w.err
}

// In case a Read results in an error with non-zero bytes read, and
// the subsequent Write also results in an error, the error from Write
// is returned, as it is the one that prevented progressing further.
// @see
func TestCopyReadErrWriteErr(t *testing.T) {
	er, ew := errors.New("readError"), errors.New("writeError")
	r, w := zeroErrReader{err: er}, errWriter{err: ew}
	n, err := Copy(w, r)
	if n != 0 || err != ew {
		t.Errorf("Copy(zeroErrReader, errWriter) = %d, %v; want 0, writeError", n, err)
	}
}

func TestCopyN(t *testing.T) {
	rb := new(Buffer)
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	// cp5个字节
	CopyN(wb, rb, 5)
	if wb.String() != "hello" {
		t.Errorf("CopyN did not work properly")
	}
}

func TestCopyNReadFrom(t *testing.T) {
	// 未实现ReadFrom.
	rb := new(Buffer)
	wb := new(bytes.Buffer) // implements ReadFrom.
	rb.WriteString("hello")
	// cp5个字节
	CopyN(wb, rb, 5)
	if wb.String() != "hello" {
		t.Errorf("CopyN did not work properly")
	}
}

func TestCopyNWriteTo(t *testing.T) {
	rb := new(bytes.Buffer) // implements WriteTo.
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	// cp5个字节
	CopyN(wb, rb, 5)
	if wb.String() != "hello" {
		t.Errorf("CopyN did not work properly")
	}
}

type noReadFrom struct {
	// Writer接口只包含Write方法,这样即使后面将bytes.Buffer赋值给w后,
	// 虽然bytes.Buffer包含ReadFrom,但是赋值后noReadFrom对外仅仅包含Write方法
	// 这里取名noReadFrom就是这个意思
	w Writer
}

func (w *noReadFrom) Write(p []byte) (n int, err error) {
	return w.w.Write(p)
}

type wantedAndErrReader struct{}

func (wantedAndErrReader) Read(p []byte) (int, error) {
	return len(p), errors.New("wantedAndErrReader error")
}

func TestCopyNEOF(t *testing.T) {
	// Test that EOF behavior is the same regardless of whether
	// argument to CopyN has ReadFrom.

	// bytes.Buffer实现了Write方法,因此b是io.Writer
	b := new(bytes.Buffer)
	// 下面的&noReadFrom{b}: 将b赋值给noReadFrom.w后,隐藏了包括ReadFrom在内的众多方法,对外只公开Write方法
	n, err := CopyN(&noReadFrom{b}, strings.NewReader("foo"), 3)
	if n != 3 || err != nil {
		t.Errorf("CopyN(noReadFrom, foo, 3) = %d, %v; want 3, nil", n, err)
	}

	n, err = CopyN(&noReadFrom{b}, strings.NewReader("foo"), 4)
	if n != 3 || err != EOF {
		// 读取3个字节后,就应该遇到EOF了
		t.Errorf("CopyN(noReadFrom, foo, 4) = %d, %v; want 3, EOF", n, err)
	}

	// 这里没有使用&noReadFrom{b},而是直接使用b,因此对外暴露了ReadFrom方法
	n, err = CopyN(b, strings.NewReader("foo"), 3) // b has read from
	if n != 3 || err != nil {
		t.Errorf("CopyN(bytes.Buffer, foo, 3) = %d, %v; want 3, nil", n, err)
	}

	n, err = CopyN(b, strings.NewReader("foo"), 4) // b has read from
	if n != 3 || err != EOF {
		t.Errorf("CopyN(bytes.Buffer, foo, 4) = %d, %v; want 3, EOF", n, err)
	}

	n, err = CopyN(b, wantedAndErrReader{}, 5)
	if n != 5 || err != nil {
		t.Errorf("CopyN(bytes.Buffer, wantedAndErrReader, 5) = %d, %v; want 5, nil", n, err)
	}

	n, err = CopyN(&noReadFrom{b}, wantedAndErrReader{}, 5)
	if n != 5 || err != nil {
		t.Errorf("CopyN(noReadFrom, wantedAndErrReader, 5) = %d, %v; want 5, nil", n, err)
	}
}

func TestReadAtLeast(t *testing.T) {
	var rb bytes.Buffer
	// testReadAtLeast要求第二个参数是ReadWriter,bytes.Buffer是满足这个要求的
	testReadAtLeast(t, &rb)
}

// A version of bytes.Buffer that returns n > 0, err on Read
// when the input is exhausted.
type dataAndErrorBuffer struct {
	// 测试时期望的错误
	err error
	bytes.Buffer
}

func (r *dataAndErrorBuffer) Read(p []byte) (n int, err error) {
	n, err = r.Buffer.Read(p)
	if n > 0 && r.Buffer.Len() == 0 && err == nil {
		// 如果读取到的字节数>0 && r.Buffer的剩余字节数==0 && Read没有出错
		err = r.err
	}
	return
}

func TestReadAtLeastWithDataAndEOF(t *testing.T) {
	// ??? 想做什么 ???
	var rb dataAndErrorBuffer
	rb.err = EOF
	testReadAtLeast(t, &rb)
}

func TestReadAtLeastWithDataAndError(t *testing.T) {
	// ??? 想做什么 ???
	var rb dataAndErrorBuffer
	rb.err = fmt.Errorf("fake error")
	testReadAtLeast(t, &rb)
}

// 注意: 本函数有个假定没有说明
// 参数rb在函数声明中只表明是ReadWriter
// 其实在实际的三处调用中(func TestReadAtLeast, func TestReadAtLeastWithDataAndEOF, func TestReadAtLeastWithDataAndError)
// 这三个函数内,要么是直接传递了 bytes.Buffer,要么是传递了dataAndErrorBuffer(内嵌bytes.Buffer)
// 他们的行为都是 bytes.Buffer
// 而bytes.Buffer的Write其实是添加内容到buffer的末尾,Read是读取未读区域的内容
func testReadAtLeast(t *testing.T, rb ReadWriter) {
	// []byte("0123") : 将"0123"这个字符串转换为slice of byte
	// rb其实是bytes.Buffer,rb.Write其实是append,rb.Read其实是读取未读区域
	rb.Write([]byte("0123"))
	// 初始化一个两字节长度的缓冲buf
	buf := make([]byte, 2)
	// 测试至少读取2字节
	// rb其实是bytes.Buffer,rb.Write其实是append,rb.Read其实是读取未读区域
	// 现在,读取位置是 "^0123"
	n, err := ReadAtLeast(rb, buf, 2)
	if err != nil {
		t.Error(err)
	}
	// 现在,读取位置是 "01^23"
	// 测试至少读取4字节
	// rb其实是bytes.Buffer,rb.Write其实是append,rb.Read其实是读取未读区域
	n, err = ReadAtLeast(rb, buf, 4)
	if err != ErrShortBuffer {
		// buf只通过make分配了2字节,却希望读4字节,因此buffer太小
		// ReadAtLeast 文档中提到: If min is greater than the length of buf, ReadAtLeast returns ErrShortBuffer.
		t.Errorf("expected ErrShortBuffer got %v", err)
	}
	if n != 0 {
		// 为什么n应该返回0 ? 看看ReadAtLeast的源码,在ReadAtLeast函数的一开始就处理了buffer过小的问题,此情况下并不会做任何读取操作
		t.Errorf("expected to have read 0 bytes, got %v", n)
	}
	// 现在,读取位置是 "01^23", 上面的 ReadAtLeast 没有读取到任何数据

	// 测试至少读取1字节
	n, err = ReadAtLeast(rb, buf, 1)
	if err != nil {
		t.Error(err)
	}
	if n != 2 {
		// 至少读取1字节是最少要求,但是因为buf初始化为2字节长度,在ReadAtLeast内部是一轮循环一轮循环的读取,因此还是会读到2个字节
		t.Errorf("expected to have read 2 bytes, got %v", n)
	}
	// 现在,读取位置是 "0123^", 现在,已经到了EOF的位置
	n, err = ReadAtLeast(rb, buf, 2)
	if err != EOF {
		// 肯定是已经到了末尾位置
		// ReadAtLeast 文档中提到: The error is EOF only if no bytes were read.
		t.Errorf("expected EOF, got %v", err)
	}
	if n != 0 {
		// 没有数据供读取了,因此n应该==0
		// ReadAtLeast 文档中提到: The error is EOF only if no bytes were read.
		t.Errorf("expected to have read 0 bytes, got %v", n)
	}
	// 向buffer结尾添加1个字节
	rb.Write([]byte("4"))
	// 现在,读取位置是 "0123^4"


	n, err = ReadAtLeast(rb, buf, 2)
	// ReadAtLeast 文档中提到, If an EOF happens after reading fewer than min bytes, ReadAtLeast returns ErrUnexpectedEOF.
	want := ErrUnexpectedEOF
	if rb, ok := rb.(*dataAndErrorBuffer); ok && rb.err != EOF {
		// rb是本函数参数,类型为ReadWriter接口,这里通过type assertion判断,如果是dataAndErrorBuffer,修改want为dataAndErrorBuffer的err字段
		want = rb.err
	}
	if err != want {
		t.Errorf("expected %v, got %v", want, err)
	}
	if n != 1 {
		// 之前向buffer结尾添加1个字节,这里应该只读取到1字节
		t.Errorf("expected to have read 1 bytes, got %v", n)
	}
}

func TestTeeReader(t *testing.T) {
	src := []byte("hello, world")
	// 声明并初始化一个和src长度相同的dst
	dst := make([]byte, len(src))
	// bytes.NewBuffer返回 *bytes.Buffer
	rb := bytes.NewBuffer(src)
	// wb也是 *bytes.Buffer
	wb := new(bytes.Buffer)
	// 这里TeeReader构造出的r代表:从r中读取,其实是从rb中读取,会同时写入wb
	r := TeeReader(rb, wb)
	if n, err := ReadFull(r, dst); err != nil || n != len(src) {
		// ReadFull 返回的 err 有错误 || 读取到的字节数 != len(src)
		t.Fatalf("ReadFull(r, dst) = %d, %v; want %d, nil", n, err, len(src))
	}
	if !bytes.Equal(dst, src) {
		// dst和src应该相等
		t.Errorf("bytes read = %q want %q", dst, src)
	}
	if !bytes.Equal(wb.Bytes(), src) {
		// 因为用了TeeReader,从rb读取的同时,数据会同时写入wb
		t.Errorf("bytes written = %q want %q", wb.Bytes(), src)
	}
	// 上面已经调用了 r.Read (在 ReadFull(r, dst) 中), 现在, r 中的数据应该已经读取完毕
	if n, err := r.Read(dst); n != 0 || err != EOF {
		// 因为r实际是从bytes.Buffer读取,bytes.Buffer读取其实是读取未读区域,而之前rb已经被读完了
		t.Errorf("r.Read at EOF = %d, %v want 0, EOF", n, err)
	}
	// 用src重新初始化一个bytes.Buffer
	rb = bytes.NewBuffer(src)
	pr, pw := Pipe()
	pr.Close()
	// 这里TeeReader构造出的r代表:从r中读取,其实是从rb中读取,会同时写入pw,但是写入pw会返回错误,因为pr已经被Close掉了
	r = TeeReader(rb, pw)
	if n, err := ReadFull(r, dst); n != 0 || err != ErrClosedPipe {
		// 应该返回n=0,err=ErrClosedPipe
		t.Errorf("closed tee: ReadFull(r, dst) = %d, %v; want 0, EPIPE", n, err)
	}
}

func TestSectionReader_ReadAt(t *testing.T) {
	// 这个测试说明: SectionReader.ReadAt的结果会受到很多参数影响
	// 包括NewSectionReader的三个参数,NewSectionReader.ReadAt的两个参数

	// dat刚好30个字符
	dat := "a long sample data, 1234567890"
	tests := []struct {
		// 从data中读取,会使用data来构造Reader, NewSectionReader 的第一个参数: func NewSectionReader(r ReaderAt, off int64, n int64) *SectionReader {
		data   string
		// 要从Reader的哪个位移开始读, NewSectionReader 的第二个参数: func NewSectionReader(r ReaderAt, off int64, n int64) *SectionReader {
		off    int
		// 读取多少字节, NewSectionReader 的第三个参数: func NewSectionReader(r ReaderAt, off int64, n int64) *SectionReader {
		n      int
		// 要make多少字节的buf以供从Reader中接收数据
		bufLen int
		// func (s *SectionReader) ReadAt(p []byte, off int64) (n int, err error) {, 传入它的 off
		at     int
		// 期望结果字符串
		exp    string
		// 期望err
		err    error
	}{
		{data: "", off: 0, n: 10, bufLen: 2, at: 0, exp: "", err: EOF},
		{data: dat, off: 0, n: len(dat), bufLen: 0, at: 0, exp: "", err: nil},
		{data: dat, off: len(dat), n: 1, bufLen: 1, at: 0, exp: "", err: EOF},
		// n: len(dat) + 2 只是用于在 NewSectionReader 中确定结束位置,但读取源只有len(dat)这么长
		//  012345678901234567890123456789
		// "a long sample data, 1234567890__"
		//  |                              |
		//  base                           limit
		//  |                            |
		//  readAt                       bufend
		{data: dat, off: 0, n: len(dat) + 2, bufLen: len(dat), at: 0, exp: dat, err: nil},
		// len(dat)=30, 分配的buf大小为 (len(dat)/2) =15,
		//  012345678901234567890123456789
		// "a long sample data, 1234567890"
		//  |                            |
		//  base                         limit
		//  |              |
		//  readAt         bufend
		{data: dat, off: 0, n: len(dat), bufLen: len(dat) / 2, at: 0, exp: dat[:len(dat)/2], err: nil},
		//  012345678901234567890123456789
		// "a long sample data, 1234567890"
		//  |                            |
		//  base                         limit
		//  |                            |
		//  readAt                       bufend
		{data: dat, off: 0, n: len(dat), bufLen: len(dat), at: 0, exp: dat, err: nil},
		//  012345678901234567890123456789
		// "a long sample data, 1234567890"
		//  |                            |
		//  base                         limit
		//    |              |
		//  readAt           bufend
		{data: dat, off: 0, n: len(dat), bufLen: len(dat) / 2, at: 2, exp: dat[2 : 2+len(dat)/2], err: nil},
		// 2+3=5,最终 是从dat的5的位置开始读
		//  012345678901234567890123456789
		// "a long sample data, 1234567890___"
		//     |                            |
		//     base                         limit
		//       |              |
		//       readAt         bufend
		{data: dat, off: 3, n: len(dat), bufLen: len(dat) / 2, at: 2, exp: dat[5 : 5+len(dat)/2], err: nil},
		//  012345678901234567890123456789
		// "a long sample data, 1234567890"
		//     |              |
		//     base           limit
		//       |            |
		//       readAt       bufend
		{data: dat, off: 3, n: len(dat) / 2, bufLen: len(dat)/2 - 2, at: 2, exp: dat[5 : 5+len(dat)/2-2], err: nil},
		//  012345678901234567890123456789
		// "a long sample data, 1234567890"
		//     |              |
		//     base           limit
		//       |                |
		//       readAt       bufend
		// 实际读取的时候没有到bufend,到limit就返回EOF了
		{data: dat, off: 3, n: len(dat) / 2, bufLen: len(dat)/2 + 2, at: 2, exp: dat[5 : 5+len(dat)/2-2], err: EOF},
		//  012345678901234567890123456789
		// "a long sample data, 1234567890"
		//  |
		//  base位置,同时也是limit位置
		// |
		// readAt位置,同时也是bufend位置
		// 参见func (s *SectionReader) ReadAt(p []byte, off int64) (n int, err error)的实现,
		// 当ReadAt的off参数没在SectionReader的范围之内的时候,会返回EOF
		{data: dat, off: 0, n: 0, bufLen: 0, at: -1, exp: "", err: EOF},
		//  012345678901234567890123456789
		// "a long sample data, 1234567890"
		//  |
		//  base位置,同时也是limit位置
		//   |
		//   readAt位置,同时也是bufend位置
		// 参见func (s *SectionReader) ReadAt(p []byte, off int64) (n int, err error)的实现,
		// 当ReadAt的off参数没在SectionReader的范围之内的时候,会返回EOF
		{data: dat, off: 0, n: 0, bufLen: 0, at: 1, exp: "", err: EOF},
	}
	for i, tt := range tests {
		r := strings.NewReader(tt.data)
		s := NewSectionReader(r, int64(tt.off), int64(tt.n))
		buf := make([]byte, tt.bufLen)
		if n, err := s.ReadAt(buf, int64(tt.at)); n != len(tt.exp) || string(buf[:n]) != tt.exp || err != tt.err {
			t.Fatalf("%d: ReadAt(%d) = %q, %v; expected %q, %v", i, tt.at, buf[:n], err, tt.exp, tt.err)
		}
	}
}

func TestSectionReader_Seek(t *testing.T) {
	// Verifies that NewSectionReader's Seeker behaves like bytes.NewReader (which is like strings.NewReader)
	// br 是 bytes.Reader 的缩写
	br := bytes.NewReader([]byte("foo"))
	// sr 是 SectionReader 的缩写
	sr := NewSectionReader(br, 0, int64(len("foo")))

	// 循环测试三种 whence 值
	for _, whence := range []int{SeekStart, SeekCurrent, SeekEnd} {
		// 对于每种 whence, 测试各种 offset 下, br.Seek 和 sr.Seek 的返回值是否相同
		for offset := int64(-3); offset <= 4; offset++ {
			brOff, brErr := br.Seek(offset, whence)
			srOff, srErr := sr.Seek(offset, whence)
			if (brErr != nil) != (srErr != nil) || brOff != srOff {
				// 对于每组whence,offset的组合
				// bytes.Reader.Seek 和 SectionReader.Seek 应该返回相同的结果
				t.Errorf("For whence %d, offset %d: bytes.Reader.Seek = (%v, %v) != SectionReader.Seek = (%v, %v)",
					whence, offset, brOff, brErr, srErr, srOff)
			}
		}
	}

	// And verify we can just seek past the end and get an EOF
	// 文档: Seek returns the new offset relative to the start of the file and an
	// error, if any. 因此这里got应该=100
	got, err := sr.Seek(100, SeekStart)
	if err != nil || got != 100 {
		t.Errorf("Seek = %v, %v; want 100, nil", got, err)
	}

	// 因为之前已经seek到100的位置,这里读取直接返回0,EOF
	n, err := sr.Read(make([]byte, 10))
	if n != 0 || err != EOF {
		t.Errorf("Read = %v, %v; want 0, EOF", n, err)
	}
}

// @see
func TestSectionReader_Size(t *testing.T) {
	tests := []struct {
		data string
		want int64
	}{
		{"a long sample data, 1234567890", 30},
		{"", 0},
	}

	for _, tt := range tests {
		r := strings.NewReader(tt.data)
		sr := NewSectionReader(r, 0, int64(len(tt.data)))
		if got := sr.Size(); got != tt.want {
			t.Errorf("Size = %v; want %v", got, tt.want)
		}
	}
}
