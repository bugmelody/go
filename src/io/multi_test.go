// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[3-over]]] 2017-6-20 15:14:41

package io_test

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	. "io"
	"io/ioutil"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMultiReader(t *testing.T) {
	var mr Reader
	// 接收读取的缓冲区, 这里只是用var声明了buf,初始化是在withFooBar中使用make初始化的
	var buf []byte
	// nread代表第几次调用expectRead
	nread := 0
	// withFooBar是一个func,它的参数名是tests,类型为 func() 
	withFooBar := func(tests func()) {
		r1 := strings.NewReader("foo ")
		r2 := strings.NewReader("")
		r3 := strings.NewReader("bar")
		// 每次调用withFooBar都会重置mr和buf
		mr = MultiReader(r1, r2, r3)
		buf = make([]byte, 20)
		tests()
	}
	// size: 传递给mr.Read的参数,也就是对buf需要slice出多大的size
	// expected: 期望的字符串
	// eerr: expected error
	expectRead := func(size int, expected string, eerr error) {
		// nread是expectRead函数体外定义的
		// nread代表第几次调用expectRead
		nread++
		n, gerr := mr.Read(buf[0:size])
		if n != len(expected) {
			// 检测len(expected)是否符合预期
			t.Errorf("#%d, expected %d bytes; got %d",
				nread, len(expected), n)
		}
		got := string(buf[0:n])
		if got != expected {
			// 检测expected是否符合预期
			t.Errorf("#%d, expected %q; got %q",
				nread, expected, got)
		}
		if gerr != eerr {
			// 检查返回的gerr是否符合预期
			t.Errorf("#%d, expected error %v; got %v",
				nread, eerr, gerr)
		}
		buf = buf[n:]
	}
	// 每个单独的reader如果EOF了,会return当前reader读取到的字节数和nil
	// 此时并不代表整个MultiReader读取完了
	withFooBar(func() {
		// 因此r1不会被读完
		expectRead(2, "fo", nil)
		// r1读完
		expectRead(5, "o ", nil)
		// r2由于是空字符串,见MultiReader的源码,此时并不会return,而是继续下轮for循环
		// r3读完,此处是由for循环中返回的
		expectRead(5, "bar", nil)
		// 此处是由MultiReader最后的地方返回的
		expectRead(5, "", EOF)
	})
	withFooBar(func() {
		// r1读完
		expectRead(4, "foo ", nil)
		// r2由于是空字符串,见MultiReader的源码,此时并不会return,而是继续下轮for循环
		// r3读b
		expectRead(1, "b", nil)
		// r3读完,此处是由for循环中返回的
		expectRead(3, "ar", nil)
		// 此处是由MultiReader最后的地方返回的
		expectRead(1, "", EOF)
	})
	withFooBar(func() {
		// r1读完,并不会读r2
		expectRead(5, "foo ", nil)
	})
}

func TestMultiWriter(t *testing.T) {
	sink := new(bytes.Buffer)
	// Hide bytes.Buffer's WriteString method:
	// bytes.Buffer存在 String Write 两个方法
	// func (b *Buffer) String() string
	// func (b *Buffer) Write(p []byte)
	// 因此能用来初始化  struct { Writer; fmt.Stringer} 的两个字段
	// 第一个 sink 初始化 Writer 字段, 第二个 sink 初始化 Stringer 字段 
	// =========
	// 整个 struct { Writer; fmt.Stringer} 也满足 testMultiWriter 的第二个参数 sink interface { Writer;fmt.Stringer}
	// testMultiWriter 的第二个参数 sink 对外界来说,只有 Write 和 String 方法, 不存在 WriteString 方法
	testMultiWriter(t, struct {
		Writer
		fmt.Stringer
	}{sink, sink})
}

func TestMultiWriter_String(t *testing.T) {
	// new(bytes.Buffer)满足testMultiWriter 的第二个参数 sink interface
	// 但此时传入的sink带有WriteString方法
	// 这里并没有 hide, 传入 testMultiWriter 的第二个参数同时有 Write,String,WriteString 方法
	testMultiWriter(t, new(bytes.Buffer))
}

// test that a multiWriter.WriteString calls results in at most 1 allocation,
// even if multiple targets don't support WriteString.
//
// 最多一次allocation是怎么实现的
// 去看看 func (t *multiWriter) WriteString(s string) (n int, err error) 的源码
//
// result in 结果是；结果造成；导致；引起
func TestMultiWriter_WriteStringSingleAlloc(t *testing.T) {
	var sink1, sink2 bytes.Buffer
	// 通过simpleWriter这样一个struct,隐藏了bytes.Buffer's WriteString
	type simpleWriter struct { // hide bytes.Buffer's WriteString
		Writer
	}
	mw := MultiWriter(simpleWriter{&sink1}, simpleWriter{&sink2})
	// testing.AllocsPerRun 会返回每次运行的平均分配内存次数
	allocs := int(testing.AllocsPerRun(1000, func() {
		// 这里是 io.WriteString, 函数内部会调用 mv.WriteString
		// mv.WriteString内部会调用mv的每一个 Writer(simpleWriter),进行stringWriter接口检测
		// 如果实现了stringWriter接口,调用对应的WriteString方法写入数据.
		// 如果没有实现stringWriter接口,使用Write方法写入(此时会进行类型转换而造成内存分配).
		WriteString(mw, "foo")
		// 上面初始化的 mw 中, 两个 simpleWriter 都没有实现 WriteString 方法,因此这里的 io.WriteString 一定会造成内存分配.
		// 参考: multiWriter.WriteString 的源码, 里面很小心的处理了类型转换可能造成的多次内存分配
	}))
	if allocs != 1 {
		t.Errorf("num allocations = %d; want 1", allocs)
	}
}

// 记录WriteString方法是否被调用过
type writeStringChecker struct{ called bool }

func (c *writeStringChecker) WriteString(s string) (n int, err error) {
	// 一旦WriteString方法被调用了,就设置called为true
	c.called = true
	// 只是为了测试 WriteString 方法是否被调用,无需进行真实的写入操作
	// 这里直接返回 len(s) 模拟WriteString调用成功的效果
	return len(s), nil
}

func (c *writeStringChecker) Write(p []byte) (n int, err error) {
	// Write 方法的调用并不会设置 c.called = true
	return len(p), nil
}

func TestMultiWriter_StringCheckCall(t *testing.T) {
	var c writeStringChecker
	mw := MultiWriter(&c)
	WriteString(mw, "foo")
	if !c.called {
		// 因为writeStringChecker实现了WriteString方法,
		// 因此WriteString方法一定会被调用,c.called应该为true
		t.Error("did not see WriteString call to writeStringChecker")
	}
}

// sink: 类型为 interface { Writer, fmt.Stringer},也就是要求有Write和String方法
func testMultiWriter(t *testing.T, sink interface {
	Writer
	fmt.Stringer
}) {
	// 注意这种写法,同一行中存在两个 sha1
	// sha1.New()返回hash.Hash,也具有Write方法
	sha1 := sha1.New()
	// MultiWriter要求参数类型为io.Writer,看hash.Hash接口的源码,其实是内嵌了io.Writer
	// 因此hash.Hash也实现了io.Writer接口
	// 写入sha1的同时也会写入sink
	mw := MultiWriter(sha1, sink)

	sourceString := "My input text."
	source := strings.NewReader(sourceString)
	// io.Copy内部自动调用multiWriter.Write,将从source读取到的输入同时写入sha1和sink
	written, err := Copy(mw, source)

	if written != int64(len(sourceString)) {
		// Copy应该写入len(sourceString)字节
		t.Errorf("short write of %d, not %d", written, len(sourceString))
	}

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	sha1hex := fmt.Sprintf("%x", sha1.Sum(nil))
	if sha1hex != "01cb303fa8c30a64123067c5aa6284ba7ec2d31b" {
		t.Error("incorrect sha1 value")
	}

	if sink.String() != sourceString {
		t.Errorf("expected %q; got %q", sourceString, sink.String())
	}
}

// Test that MultiReader copies the input slice and is insulated from future modification.
//
// insulated [ˈɪnsjuleɪtɪd] adj. [电] 绝缘的；隔热的 v. 使…绝缘（insulate的过去式）
func TestMultiReaderCopy(t *testing.T) {
	slice := []Reader{strings.NewReader("hello world")}
	r := MultiReader(slice...)
	// 这里即使修改为了nil,仍然不会影响后面的ioutil.ReadAll(r),说明r和slice是隔离的
	slice[0] = nil
	// 即使上面修改了slice[0],这里读取r也不会被影响
	data, err := ioutil.ReadAll(r)
	if err != nil || string(data) != "hello world" {
		t.Errorf("ReadAll() = %q, %v, want %q, nil", data, err, "hello world")
	}
}

// Test that MultiWriter copies the input slice and is insulated from future modification.
func TestMultiWriterCopy(t *testing.T) {
	var buf bytes.Buffer
	slice := []Writer{&buf}
	// MultiWriter内部会调用copy,因此slice与w已经绝缘
	w := MultiWriter(slice...)
	slice[0] = nil
	// 即使上面修改了slice[0], 调用 w.Write仍然会写入buf
	n, err := w.Write([]byte("hello world"))
	if err != nil || n != 11 {
		t.Errorf("Write(`hello world`) = %d, %v, want 11, nil", n, err)
	}
	if buf.String() != "hello world" {
		t.Errorf("buf.String() = %q, want %q", buf.String(), "hello world")
	}
}

// readerFunc is an io.Reader implemented by the underlying func.
type readerFunc func(p []byte) (int, error)

// 将函数作为receiver
func (f readerFunc) Read(p []byte) (int, error) {
	// 具体的操作委托给receiver
	return f(p)
}
// 后面的不看了

// callDepth returns the logical call depth for the given PCs.
func callDepth(callers []uintptr) (depth int) {
	frames := runtime.CallersFrames(callers)
	more := true
	for more {
		_, more = frames.Next()
		depth++
	}
	return
}

// Test that MultiReader properly flattens chained multiReaders when Read is called
func TestMultiReaderFlatten(t *testing.T) {
	pc := make([]uintptr, 1000) // 1000 should fit the full stack
	n := runtime.Callers(0, pc)
	var myDepth = callDepth(pc[:n])
	var readDepth int // will contain the depth from which fakeReader.Read was called
	var r Reader = MultiReader(readerFunc(func(p []byte) (int, error) {
		n := runtime.Callers(1, pc)
		readDepth = callDepth(pc[:n])
		return 0, errors.New("irrelevant")
	}))

	// chain a bunch of multiReaders
	for i := 0; i < 100; i++ {
		r = MultiReader(r)
	}

	r.Read(nil) // don't care about errors, just want to check the call-depth for Read

	if readDepth != myDepth+2 { // 2 should be multiReader.Read and fakeReader.Read
		t.Errorf("multiReader did not flatten chained multiReaders: expected readDepth %d, got %d",
			myDepth+2, readDepth)
	}
}

// byteAndEOFReader is a Reader which reads one byte (the underlying
// byte) and io.EOF at once in its Read call.
type byteAndEOFReader byte

func (b byteAndEOFReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		// Read(0 bytes) is useless. We expect no such useless
		// calls in this test.
		panic("unexpected call")
	}
	p[0] = byte(b)
	return 1, EOF
}

// This used to yield bytes forever; issue 16795.
func TestMultiReaderSingleByteWithEOF(t *testing.T) {
	got, err := ioutil.ReadAll(LimitReader(MultiReader(byteAndEOFReader('a'), byteAndEOFReader('b')), 10))
	if err != nil {
		t.Fatal(err)
	}
	const want = "ab"
	if string(got) != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

// Test that a reader returning (n, EOF) at the end of an MultiReader
// chain continues to return EOF on its final read, rather than
// yielding a (0, EOF).
func TestMultiReaderFinalEOF(t *testing.T) {
	r := MultiReader(bytes.NewReader(nil), byteAndEOFReader('a'))
	buf := make([]byte, 2)
	n, err := r.Read(buf)
	if n != 1 || err != EOF {
		t.Errorf("got %v, %v; want 1, EOF", n, err)
	}
}

func TestMultiReaderFreesExhaustedReaders(t *testing.T) {
	var mr Reader
	closed := make(chan struct{})
	// The closure ensures that we don't have a live reference to buf1
	// on our stack after MultiReader is inlined (Issue 18819).  This
	// is a work around for a limitation in liveness analysis.
	func() {
		buf1 := bytes.NewReader([]byte("foo"))
		buf2 := bytes.NewReader([]byte("bar"))
		mr = MultiReader(buf1, buf2)
		runtime.SetFinalizer(buf1, func(*bytes.Reader) {
			close(closed)
		})
	}()

	buf := make([]byte, 4)
	if n, err := ReadFull(mr, buf); err != nil || string(buf) != "foob" {
		t.Fatalf(`ReadFull = %d (%q), %v; want 3, "foo", nil`, n, buf[:n], err)
	}

	runtime.GC()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for collection of buf1")
	}

	if n, err := ReadFull(mr, buf[:2]); err != nil || string(buf[:2]) != "ar" {
		t.Fatalf(`ReadFull = %d (%q), %v; want 2, "ar", nil`, n, buf[:n], err)
	}
}

func TestInterleavedMultiReader(t *testing.T) {
	r1 := strings.NewReader("123")
	r2 := strings.NewReader("45678")

	mr1 := MultiReader(r1, r2)
	mr2 := MultiReader(mr1)

	buf := make([]byte, 4)

	// Have mr2 use mr1's []Readers.
	// Consume r1 (and clear it for GC to handle) and consume part of r2.
	n, err := ReadFull(mr2, buf)
	if got := string(buf[:n]); got != "1234" || err != nil {
		t.Errorf(`ReadFull(mr2) = (%q, %v), want ("1234", nil)`, got, err)
	}

	// Consume the rest of r2 via mr1.
	// This should not panic even though mr2 cleared r1.
	n, err = ReadFull(mr1, buf)
	if got := string(buf[:n]); got != "5678" || err != nil {
		t.Errorf(`ReadFull(mr1) = (%q, %v), want ("5678", nil)`, got, err)
	}
}
