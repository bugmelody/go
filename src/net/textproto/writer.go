// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[5-over]]] 2017-7-4 14:17:49

package textproto

import (
	"bufio"
	"fmt"
	"io"
)

// A Writer implements convenience methods for writing
// requests or responses to a text protocol network connection.
//
// 对于client,Writer用于写入请求.
// 对于server,Writer用于写入响应.
type Writer struct {
	// 参考textproto.Dial,Writer.W是bufio.NewWriter(net.Conn),对W写入数据,其实就是写入数据到net.Conn
	W   *bufio.Writer
	dot *dotWriter
}

// NewWriter returns a new Writer writing to w.
func NewWriter(w *bufio.Writer) *Writer {
	return &Writer{W: w}
}

var crnl = []byte{'\r', '\n'}
var dotcrnl = []byte{'.', '\r', '\n'}

// PrintfLine writes the formatted output followed by \r\n.
func (w *Writer) PrintfLine(format string, args ...interface{}) error {
	w.closeDot()
	// 参考textproto.Dial,Writer.W是bufio.NewWriter(net.Conn),对W写入数据,其实就是写入数据到net.Conn
	fmt.Fprintf(w.W, format, args...)
	w.W.Write(crnl)
	// 将buffer中的数据刷到net.Conn
	return w.W.Flush()
}

// DotWriter returns a writer that can be used to write a dot-encoding to w.
// It takes care of inserting leading dots when necessary,
// translating line-ending \n into \r\n, and adding the final .\r\n line
// when the DotWriter is closed. The caller should close the
// DotWriter before the next call to a method on w.
//
// See the documentation for Reader's DotReader method for details about dot-encoding.
func (w *Writer) DotWriter() io.WriteCloser {
	w.closeDot()
	w.dot = &dotWriter{w: w}
	return w.dot
}

func (w *Writer) closeDot() {
	if w.dot != nil {
		w.dot.Close() // sets w.dot = nil
	}
}

type dotWriter struct {
	// dotWriter属于哪个textproto.Writer
	w     *Writer
	state int
}

const (
	wstateBeginLine = iota // beginning of line; initial state; must be zero
	wstateCR               // wrote \r (possibly at end of line)
	wstateData             // writing data in middle of line
)

func (d *dotWriter) Write(b []byte) (n int, err error) {
	// bw代表b要写入的目标,d.w.W类型是*bufio.Writer,实际对应net.Conn
	bw := d.w.W
	// b:要写入的内容,函数完成时候应该写入len(b)个字节.
	// n:函数返回值,表示已经写入的字节数..
	// n<len(b):在没有写完b的情况下持续循环写入
	for n < len(b) {
		// c是本轮循环需要写入的字节
		c := b[n]
		switch d.state {
		case wstateBeginLine:
			d.state = wstateData
			if c == '.' {
				// 如果当前循环是要写入'.'
				// escape leading dot
				// 根据文档:Lines beginning with a dot are escaped with an additional dot to avoid looking like the end of the sequence.
				bw.WriteByte('.')
			}
			fallthrough
			// 注意,假设这个case分支设置了d.state=wstateData,下面的case wstateData:就会被执行

		case wstateData:
			if c == '\r' {
				// 如果当前循环是要写入'\r'
				d.state = wstateCR
			}
			if c == '\n' {
				// 如果当前循环是要写入'\n'
				bw.WriteByte('\r')
				d.state = wstateBeginLine
			}

		case wstateCR:
			d.state = wstateData
			if c == '\n' {
				d.state = wstateBeginLine
			}
		}
		// 这里才是写入c,之前的都是在c之前写入其他东西和修改d.state
		if err = bw.WriteByte(c); err != nil {
			// 出错时break
			break
		}
		n++
	}
	return
}

func (d *dotWriter) Close() error {
	if d.w.dot == d {
		d.w.dot = nil
	}
	// bw代表b要写入的目标,d.w.W类型是*bufio.Writer,实际对应net.Conn
	bw := d.w.W
	switch d.state {
	default:
		// 注意:此分支一定会被执行;也就是说,Close调用时一定会先写入\r
		bw.WriteByte('\r')
		fallthrough
	case wstateCR:
		// Close调用时如果d.state==wstateCR,一定会写入\n
		bw.WriteByte('\n')
		fallthrough
	case wstateBeginLine:
		// Close调用时如果d.state==wstateBeginLine,一定会写入'.\r\n'
		bw.Write(dotcrnl)
	}
	return bw.Flush()
}
