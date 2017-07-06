// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[5-over]]] 2017-7-4 13:12:10

// Package textproto implements generic support for text-based request/response
// protocols in the style of HTTP, NNTP, and SMTP.
//
// The package provides:
//
// Error, which represents a numeric error response from
// a server.
//
// Pipeline, to manage pipelined requests and responses
// in a client.
//
// Reader, to read numeric response code lines,
// key: value headers, lines wrapped with leading spaces
// on continuation lines, and whole text blocks ending
// with a dot on a line by itself.
//
// Writer, to write dot-encoded text blocks.
//
// Conn, a convenient packaging of Reader, Writer, and Pipeline for use
// with a single network connection.
//
package textproto

import (
	"bufio"
	"fmt"
	"io"
	"net"
)

// An Error represents a numeric error response from a server.
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string {
	// ??? %03d 是什么意思 ??? 0是指使用0进行填充,3是指宽度为3
	return fmt.Sprintf("%03d %s", e.Code, e.Msg)
}

// A ProtocolError describes a protocol violation such
// as an invalid response or a hung-up connection.
type ProtocolError string

func (p ProtocolError) Error() string {
	return string(p)
}

// A Conn represents a textual network protocol connection.
// It consists of a Reader and Writer to manage I/O
// and a Pipeline to sequence concurrent requests on the connection.
// These embedded types carry methods with them;
// see the documentation of those types for details.
//
// Conn拥有Reader,Writer,Pipeline的所有方法
type Conn struct {
	Reader
	Writer
	Pipeline
	conn io.ReadWriteCloser
}

// NewConn returns a new Conn using conn for I/O.
func NewConn(conn io.ReadWriteCloser) *Conn {
	return &Conn{
		Reader: Reader{R: bufio.NewReader(conn)},
		Writer: Writer{W: bufio.NewWriter(conn)},
		conn:   conn,
	}
}

// Close closes the connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// Dial connects to the given address on the given network using net.Dial
// and then returns a new Conn for the connection.
//
// Dial是对NewConn的封装.
func Dial(network, addr string) (*Conn, error) {
	// net.Dial返回的net.Conn,是interface,定义中拥有Read,Write,Close方法
	c, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	return NewConn(c), nil
}

// Cmd is a convenience method that sends a command after
// waiting its turn in the pipeline. The command text is the
// result of formatting format with args and appending \r\n.
// Cmd returns the id of the command, for use with StartResponse and EndResponse.
//
// 上文中format和args都是指本方法的参数
// Cmd函数返回后,说明命令已经发送完毕.
//
// For example, a client might run a HELP command that returns a dot-body
// by using:
//
//	id, err := c.Cmd("HELP")
//	// 到了这里,说明HELP命令已经发送完毕
//	if err != nil {
//		return nil, err
//	}
//
//	c.StartResponse(id)
//	defer c.EndResponse(id)
//
//	if _, _, err = c.ReadCodeLine(110); err != nil {
//		return nil, err
//	}
//	text, err := c.ReadDotBytes()
//	if err != nil {
//		return nil, err
//	}
//	return c.ReadCodeLine(250)
//
//
// Cmd方法会阻塞直到是时间发送(client)或接受(server)请求,因为内部调用了c.StartRequest.
func (c *Conn) Cmd(format string, args ...interface{}) (id uint, err error) {
	// 实际是调用c.Pipeline.Next()
	id = c.Next()
	// 实际是调用c.Pipeline.StartRequest(id)
	c.StartRequest(id)
	// 实际是调用c.Writer.PrintfLine()
	err = c.PrintfLine(format, args...)
	// 实际是调用c.Pipeline.EndRequest(id)
	c.EndRequest(id)
	if err != nil {
		return 0, err
	}
	// 返回的id代表已经发送的命令的id
	return id, nil
}

// TrimString returns s without leading and trailing ASCII space.
func TrimString(s string) string {
	// 循环处理字符串起始处,删除ASCIISpace.
	for len(s) > 0 && isASCIISpace(s[0]) {
		s = s[1:]
	}
	// 循环处理字符串结尾处,删除ASCIISpace.
	for len(s) > 0 && isASCIISpace(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

// TrimBytes returns b without leading and trailing ASCII space.
// 这个TrimBytes跟上面的TrimString的代码完全一样,只是参数和返回值类型是[]byte.
// 如果支持泛型,就更好了
func TrimBytes(b []byte) []byte {
	for len(b) > 0 && isASCIISpace(b[0]) {
		b = b[1:]
	}
	for len(b) > 0 && isASCIISpace(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	return b
}

func isASCIISpace(b byte) bool {
	// 这里将byte直接和rune常量进行比较,因为他们底层类型都是整型,因此可以进行比较
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// 判断b是否是[a-zA-Z]
func isASCIILetter(b byte) bool {
	// A: 十进制 65, 二进制 1000001
	// a: 十进制 97, 二进制 1100001
	// 0x20=        二进制  100000
	//                     543210
	// b |= 0x20 : 也就是将第5位强制为1,转换为小写字符
	b |= 0x20 // make lower case
	return 'a' <= b && b <= 'z'
}
