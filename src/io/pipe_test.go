// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[3-over]]] 2017-4-6 16:41:19

package io_test

import (
	"fmt"
	. "io"
	"testing"
	"time"
)

// data: 要写入的数据
// w: 要写入的目标
// c: w写完后,不论是否成功,会发送0到c,表示写入操作已经完毕
func checkWrite(t *testing.T, w Writer, data []byte, c chan int) {
	n, err := w.Write(data)
	if err != nil {
		// w.Write出错
		t.Errorf("write: %v", err)
	}
	if n != len(data) {
		// 写入长度太少
		t.Errorf("short write: %d != %d", n, len(data))
	}
	// 注意,上面的t.Errorf调用之后,程序还会继续运行
	// w写完后,不论是否成功,会发送0到c,表示写入操作已经完毕
	c <- 0
}

// Test a single read/write pair.
func TestPipe1(t *testing.T) {
	c := make(chan int)
	// io.Pipe,从r中读取的就是写入w的
	r, w := Pipe()
	// buf用于在r.Read读取操作中接收数据
	var buf = make([]byte, 64)
	// 启动一个goroutine,在其中,向w写入数据,写入完成,不论是否写入成功,会向c发送0表示写入完成
	go checkWrite(t, w, []byte("hello, world"), c)
	// 注意,io.Pipe返回的r和w,写入和读取是同步的,也就是说,这里会阻塞,直到有写入操作
	n, err := r.Read(buf)
	if err != nil {
		t.Errorf("read: %v", err)
	} else if n != 12 || string(buf[0:12]) != "hello, world" {
		// 如果没有错
		// "hello, world"的长度刚好是12字节
		t.Errorf("bad read: got %q", buf[0:n])
	}
	// 等待checkWrite的goroutine的完成通知
	<-c
	// 注意,对于io.Pipe返回的r和w,如果写入over,那么读取肯定也over
	r.Close()
	w.Close()
}

// r参数: io.Pipe() 返回的 *PipeReader
// 可以从 c 中获取到每轮循环读取到的字节数
func reader(t *testing.T, r Reader, c chan int) {
	// 用于接收数据的buffer
	var buf = make([]byte, 64)
	for {
		n, err := r.Read(buf)
		if err == EOF {
			// 当前循环如果读取到EOF,向channel发送0,跳出循环,退出函数
			c <- 0
			break
		}
		if err != nil {
			// 读取出错
			t.Errorf("read: %v", err)
		}
		// 当前循环读取成功,向channel c发送本轮循环读取到的字节数
		c <- n
	}
}

// Test a sequence of read/write pairs.
func TestPipe2(t *testing.T) {
	c := make(chan int)
	r, w := Pipe()
	// 启动 reader goroutine
	// 可以从 c 中获取到每轮循环读取到的字节数
	go reader(t, r, c)
	var buf = make([]byte, 64)
	for i := 0; i < 5; i++ {
		p := buf[0 : 5+i*10]
		n, err := w.Write(p)
		if n != len(p) {
			// 写入字节数不符合预期
			t.Errorf("wrote %d, got %d", len(p), n)
		}
		if err != nil {
			// 写入出错
			t.Errorf("write: %v", err)
		}
		// 在 reader goroutine 中,每轮读取完,会向c发送当轮读取的 字节数
		// 得到当轮循环读取到的字节数
		nn := <-c
		if nn != n {
			// 如果本轮循环读取到的字节数不符合写入的字节数
			t.Errorf("wrote %d, read got %d", n, nn)
		}
	}
	w.Close()
	nn := <-c
	// 将写入端Close了之后再从读取端进行读取,应该是EOF
	if nn != 0 {
		// 在 reader goroutine 中,如果读取到EOF,会向channel发送0,跳出循环,退出函数
		t.Errorf("final read got %d", nn)
	}
}

type pipeReturn struct {
	n   int
	err error
}

// Test a large write that requires multiple reads to satisfy.
//
// 这里进行测试: 如果向 io.Pipe 返回的 w 写入一块超大的数据, 需要多次从 r 进行读取
// 参数buf: 要写入的数据
//
// 函数签名中的 w 类型为 WriteCloser , 一般就意味着函数内部会调用 Write, 然后调用 Close.
func writer(w WriteCloser, buf []byte, c chan pipeReturn) {
	n, err := w.Write(buf)
	w.Close()
	c <- pipeReturn{n, err}
}

func TestPipe3(t *testing.T) {
	c := make(chan pipeReturn)
	r, w := Pipe()
	// wdat: 要写入的数据
	// 使用make初始化wdat
	// 这是一块超大的数据, 因此写入 w 后, 需要从 r 中进行多次读取
	var wdat = make([]byte, 128)
	for i := 0; i < len(wdat); i++ {
		// 设置wdat
		wdat[i] = byte(i)
	}
	// 启动goroutine进行写入
	go writer(w, wdat, c)
	// 声明read的目标
	var rdat = make([]byte, 1024)
	// tot [tɒt] n. 小孩；合计；少量 vt. 合计 vi. 总计
	tot := 0
	for n := 1; n <= 256; n *= 2 {
		// n序列: 1,2,4,8,16,32,64,128,256
		// 1+2+4+8+16+32+64=127
		// 1+2+4+8+16+32+64+128=255
		nn, err := r.Read(rdat[tot : tot+n])
		if err != nil && err != EOF {
			// 期望 err 要么是 nil, 要么是 EOF
			t.Fatalf("read: %v", err)
		}

		// only final two reads should be short - 1 byte, then 0
		expect := n
		if n == 128 {
			expect = 1
		} else if n == 256 {
			expect = 0
			if err != EOF {
				// n == 256 的时候, Read 应该返回 EOF 了
				t.Fatalf("read at end: %v", err)
			}
		}
		if nn != expect {
			// PipeReader.Read 读取到的数据长度不符合预期
			t.Fatalf("read %d, expected %d, got %d", n, expect, nn)
		}
		tot += nn
	}
	// pr 代表 pipeReturn
	// 等待writer函数的完成通知
	pr := <-c
	if pr.n != 128 || pr.err != nil {
		// 期望 PipeReader.Write 写入 128 字节,并且没有错误
		t.Fatalf("write 128: %d, %v", pr.n, pr.err)
	}
	if tot != 128 {
		// 期望总共读取到 128 字节
		t.Fatalf("total read %d != 128", tot)
	}
	for i := 0; i < 128; i++ {
		if rdat[i] != byte(i) {
			// 读取到的每个字节是否符合预期
			t.Fatalf("rdat[%d] = %d", i, rdat[i])
		}
	}
}

// Test read after/before writer close.

type closer interface {
	CloseWithError(error) error
	Close() error
}

type pipeTest struct {
	// 是否使用用goroutine运行
	async          bool
	// 向CloseWithError传递的参数
	err            error
	// 是否在delayClose中调用CloseWithError. 如果是true,会调用CloseWithError;否则,调用Close
	closeWithError bool
}

// 定义String方法便于输出p的状态
func (p pipeTest) String() string {
	return fmt.Sprintf("async=%v err=%v closeWithError=%v", p.async, p.err, p.closeWithError)
}

var pipeTests = []pipeTest{
	// 使用 goroutine 启动 delayClose, 调用 Close
	{true, nil, false},
	{true, nil, true},
	{true, ErrShortWrite, true},
	{false, nil, false},
	{false, nil, true},
	{false, ErrShortWrite, true},
}

// closer是在上方临时定义的一个interface
func delayClose(t *testing.T, cl closer, ch chan int, tt pipeTest) {
	time.Sleep(1 * time.Millisecond)
	var err error
	if tt.closeWithError {
		err = cl.CloseWithError(tt.err)
	} else {
		err = cl.Close()
	}
	if err != nil {
		// CloseWithError和Close都不应该出错
		t.Errorf("delayClose: %v", err)
	}
	// 发送 close 完毕的通知
	ch <- 0
}

func TestPipeReadClose(t *testing.T) {
	// pipeTests变量代表了要循环进行的每一项测试
	for _, tt := range pipeTests {
		// buffer capacity = 1 的 channel
		c := make(chan int, 1)
		r, w := Pipe()
		if tt.async {
			// 如果tt.async为true,代表使用goroutine运行delayClose
			go delayClose(t, w, c, tt)
		} else {
			// 如果tt.async为false,代表普通方式运行delayClose
			delayClose(t, w, c, tt)
		}
		// delayClose,不论是否通过goroutine运行,最后都会有 ch <- 0
		// 那么,这两种情况下,有什么分别??(普通运行和goroutine运行)
		var buf = make([]byte, 64)
		n, err := r.Read(buf)
		// 等待delayClose的运行完毕
		<-c
		// 到了这里,肯定已经执行了 w.Close 或者 w.CloseWithError(tt.err)
		want := tt.err
		if want == nil {
			want = EOF
		}
		if err != want {
			t.Errorf("read from closed pipe: %v want %v", err, want)
		}
		if n != 0 {
			// n一定等于0,因为并没有任何写入操作
			t.Errorf("read on closed pipe returned %d", n)
		}
		// 因为w已经被Close,此时再调用r.Close,一定没有问题
		if err = r.Close(); err != nil {
			t.Errorf("r.Close: %v", err)
		}
	}
}

// Test close on Read side during Read.
func TestPipeReadClose2(t *testing.T) {
	// buffer capacity = 1 的 channel
	c := make(chan int, 1)
	// 只需要r,不需要w
	r, _ := Pipe()
	// pipeTest会被初始化为默认值(zero value)
	// 由于 pipeTest.closeWithError 是 bool 类型,zero value 是 false,因此 delayClose 内部会调用 r.Close
	go delayClose(t, r, c, pipeTest{})
	// 刚开始调用时会阻塞,直到另一个goroutine中被close
	n, err := r.Read(make([]byte, 64))
	// 等待r在另一个goroutine中被close, 等待 delayClose 函数的完成
	<-c
	// 现在,r已经被Close
	if n != 0 || err != ErrClosedPipe {
		// 期望n==0 && err == ErrClosedPipe
		// ErrClosedPipe is the error used for read or write operations on a closed pipe.
		t.Errorf("read from closed pipe: %v, %v want %v, %v", n, err, 0, ErrClosedPipe)
	}
}

// Test write after/before reader close.

func TestPipeWriteClose(t *testing.T) {
	for _, tt := range pipeTests {
		c := make(chan int, 1)
		r, w := Pipe()
		if tt.async {
			go delayClose(t, r, c, tt)
		} else {
			delayClose(t, r, c, tt)
		}
		n, err := WriteString(w, "hello, world")
		// 等待delayClose函数的完成,这里是等待r的Close
		<-c
		expect := tt.err
		if expect == nil {
			// 见 PipeReader.Close 和 PipeReader.CloseWithError
			// PipeReader被Close后,PipeWriter进行写入的时候会返回 ErrClosedPipe
			expect = ErrClosedPipe
		}
		if err != expect {
			t.Errorf("write on closed pipe: %v want %v", err, expect)
		}
		if n != 0 {
			// r被关闭了,w应该无法写入任何数据
			t.Errorf("write on closed pipe returned %d", n)
		}
		if err = w.Close(); err != nil {
			t.Errorf("w.Close: %v", err)
		}
	}
}

// Test close on Write side during Write.
func TestPipeWriteClose2(t *testing.T) {
	c := make(chan int, 1)
	_, w := Pipe()
	go delayClose(t, w, c, pipeTest{})
	n, err := w.Write(make([]byte, 64))
	<-c
	if n != 0 || err != ErrClosedPipe {
		t.Errorf("write to closed pipe: %v, %v want %v, %v", n, err, 0, ErrClosedPipe)
	}
}

func TestWriteEmpty(t *testing.T) {
	r, w := Pipe()
	go func() {
		w.Write([]byte{})
		w.Close()
	}()
	// 声明一个2字节的数组
	var b [2]byte
	// ReadFull 要求读满传入的buf长度, 写入时是写入了 empty. 此时ReadFull会等待w被Close
	ReadFull(r, b[0:2])
	r.Close()
}

func TestWriteNil(t *testing.T) {
	r, w := Pipe()
	go func() {
		w.Write(nil)
		w.Close()
	}()
	var b [2]byte
	// ReadFull 要求读满传入的buf长度, 写入时是写入了 nil, 这如何是好. 此时ReadFull会等待w被Close
	ReadFull(r, b[0:2])
	r.Close()
}

func TestWriteAfterWriterClose(t *testing.T) {
	r, w := Pipe()

	// 是否完成的标记
	done := make(chan bool)
	var writeErr error
	go func() {
		_, err := w.Write([]byte("hello"))
		if err != nil {
			t.Errorf("got error: %q; expected none", err)
		}
		w.Close()
		// w.Close()之后继续调用w.Write, 这里是期待返回 ErrClosedPipe
		// writeErr是外层函数中定义的变量,这里匿名函数是闭包
		_, writeErr = w.Write([]byte("world"))
		// 发送完成通知
		done <- true
	}()

	buf := make([]byte, 100)
	var result string
	// 对于 ReadFull,其文档中说到:
	// If an EOF happens after reading some but not all the bytes, ReadFull returns ErrUnexpectedEOF
	n, err := ReadFull(r, buf)
	if err != nil && err != ErrUnexpectedEOF {
		// 期望 err==ErrUnexpectedEOF
		t.Fatalf("got: %q; want: %q", err, ErrUnexpectedEOF)
	}
	// n 是之前 ReadFull 返回的读取到的字节数
	result = string(buf[0:n])
	// 接收channel完成通知
	<-done
	// 现在, 启动的 goroutine 已经执行完毕

	if result != "hello" {
		// 期望 result == "hello"
		t.Errorf("got: %q; want: %q", result, "hello")
	}
	if writeErr != ErrClosedPipe {
		// 期望 writeErr == ErrClosedPipe
		t.Errorf("got: %q; want: %q", writeErr, ErrClosedPipe)
	}
}
