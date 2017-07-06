// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-7-4 15:17:44

package textproto

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
)

// A Reader implements convenience methods for reading requests
// or responses from a text protocol network connection.
//
// 用于(server读取请求)或(client读取响应)
type Reader struct {
	// 参考textproto.Dial,Reader.R实际是封装了底层的net.Conn
	R   *bufio.Reader
	dot *dotReader
	buf []byte // a re-usable buffer for readContinuedLineSlice
}

// NewReader returns a new Reader reading from r.
//
// To avoid denial of service attacks, the provided bufio.Reader
// should be reading from an io.LimitReader or similar Reader to bound
// the size of responses.
//
// DoS是Denial of Service的简称,即拒绝服务.
// 造成DoS的攻击行为被称为DoS攻击,其目的是使计算机或网络无法提供正常的服务.
func NewReader(r *bufio.Reader) *Reader {
	return &Reader{R: r}
}

// ReadLine reads a single line from r,
// eliding the final \n or \r\n from the returned string.
//
// elide [ɪ'laɪd] vt. 省略；取消；删去；不予考虑删节
func (r *Reader) ReadLine() (string, error) {
	line, err := r.readLineSlice()
	return string(line), err
}

// ReadLineBytes is like ReadLine but returns a []byte instead of a string.
func (r *Reader) ReadLineBytes() ([]byte, error) {
	line, err := r.readLineSlice()
	/**
	如果r.readLineSlice()返回的是nil,会略过下面的if,直接返回nil,err
	如果r.readLineSlice()返回的不是nil,根据r.readLineSlice()的命名规则(readXXXSlice),说明returned buffer is only valid until the next call,
	因此需要复制一份新的line返回
	 */
	if line != nil {
		buf := make([]byte, len(line))
		copy(buf, line)
		// 这里为什么要进行反向赋值?
		// 如果不反向赋值,line会不安全
		// 反向赋值后,line不是原始数据中的byte slice,line已经指向一块新的内存,这块新的内存拥有原始byte slice的一份拷贝,因此可以被安全使用.
		line = buf
	}
	return line, err
}

/**
因为 r.readLineSlice() 内部调用了 bufio.ReadLine, 根据 ReadLine 的文档:
The returned buffer is only valid until the next call to ReadLine.

当内部bufio.ReadLine能读取到完整行时,返回的slice是不能被随便修改的并且 returned buffer is only valid until the next call
所以当其他人调用readLineSlice时候,需要复制readLineSlice的返回结果


我发现很多标准库中的方法都是这样:
如果一个方法类型 readXXXSlice, 返回的 []byte, 其实都是源数据的slice,
说明 The returned buffer is only valid until the next call to ReadLine.
 */

func (r *Reader) readLineSlice() ([]byte, error) {
	r.closeDot()
	// line是本方法最后要返回的[]byte
	var line []byte
	for {
		l, more, err := r.R.ReadLine()
		if err != nil {
			return nil, err
		}
		// Avoid the copy if the first call produced a full line.
		if line == nil && !more {
			// 第一次循环就读取到完整的行,直接返回,注意,返回的是buffer中的一段slice,不能随便修改
			return l, nil
		}
		// 不是完整的一行,需要在循环中不停的append
		line = append(line, l...)
		if !more {
			// 处理完一行的连续读取
			break
		}
	}
	return line, nil
}

// ReadContinuedLine reads a possibly continued line from r,
// eliding the final trailing ASCII white space.
// Lines after the first are considered continuations if they
// begin with a space or tab character. In the returned data,
// continuation lines are separated from the previous line
// only by a single space: the newline and leading white space
// are removed.
//
// For example, consider this input:
//
//	Line 1
//	  continued...
//	Line 2
//
// The first call to ReadContinuedLine will return "Line 1 continued..."
// and the second will return "Line 2".
//
// A line consisting of only white space is never continued.
//
func (r *Reader) ReadContinuedLine() (string, error) {
	line, err := r.readContinuedLineSlice()
	return string(line), err
}

// trim returns s with leading and trailing spaces and tabs removed.
// It does not assume Unicode or UTF-8.
func trim(s []byte) []byte {
	// i代表第一个非空白符的位置
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	// n代表最后一个非空白符的位置
	n := len(s)
	for n > i && (s[n-1] == ' ' || s[n-1] == '\t') {
		n--
	}
	return s[i:n]
}

// ReadContinuedLineBytes is like ReadContinuedLine but
// returns a []byte instead of a string.
func (r *Reader) ReadContinuedLineBytes() ([]byte, error) {
	line, err := r.readContinuedLineSlice()
	if line != nil {
		buf := make([]byte, len(line))
		copy(buf, line)
		// 这里为什么要进行反向赋值?
		// 如果不反向赋值,line会不安全
		// 反向赋值后,line不是原始数据中的byte slice,line已经指向一块新的内存,这块新的内存拥有原始byte slice的一份拷贝,因此可以被安全使用.
		line = buf
	}
	return line, err
}

func (r *Reader) readContinuedLineSlice() ([]byte, error) {
	// Read the first line.
	line, err := r.readLineSlice()
	if err != nil {
		return nil, err
	}
	if len(line) == 0 { // blank line - no continuation
		// 文档:A line consisting of only white space is never continued.
		return line, nil
	}
	// 现在,已经读到了一行.但是可能存在续行.
	// optimistically adv. optimistic的变形
	// optimistic [,ɔpti'mistik] adj. 1.乐观的 2.乐观主义的 3.体现乐观主义的 [亦作 optimistical]

	// Optimistically assume that we have started to buffer the next line
	// and it starts with an ASCII letter (the next header key), so we can
	// avoid copying that buffered data around in memory and skipping over
	// non-existent whitespace.
	if r.R.Buffered() > 1 {
		// 如果可从buffer中读取的字节数>1,Peek buffer中的那个字节
		peek, err := r.R.Peek(1)
		if err == nil && isASCIILetter(peek[0]) {
			// 如果isASCIILetter,说明不是续行,直接返回之前读取到的行数据
			return trim(line), nil
		}
	}
	// 现在,说明是续行

	// ReadByte or the next readLineSlice will flush the read buffer;
	// copy the slice into buf.
	r.buf = append(r.buf[:0], trim(line)...)

	// Read continuation lines.
	for r.skipSpace() > 0 {
		// 只要略过了空白符,说明是续行
		// 读取一行
		line, err := r.readLineSlice()
		if err != nil {
			break
		}
		// 根据文档,续行之间空格分隔,这里append空格
		r.buf = append(r.buf, ' ')
		// 再append读取到的续行
		r.buf = append(r.buf, trim(line)...)
	}
	// 注意:r.buf是共用的
	return r.buf, nil
}

// skipSpace skips R over all spaces and returns the number of bytes skipped.
//
// 注意这里的技巧,对bufio.Reader的ReadByte和UnreadByte的使用
func (r *Reader) skipSpace() int {
	// 方法最后的返回值,代表实际略过了多少Space
	n := 0
	for {
		c, err := r.R.ReadByte()
		if err != nil {
			// Bufio will keep err until next read.
			break
		}
		if c != ' ' && c != '\t' {
			// ReadByte读取到的字节说明没有续行,恢复数据
			r.R.UnreadByte()
			break
		}
		n++
	}
	return n
}

// expectCode实际是代表期望的code前缀:
//   if expectCode is 31, an error will be returned if the status is not in the range [310,319].
// code: 实际读取到的code
// continued: 读取到的行是否是连续行(是否还有后续行)
// message: code对应的message
func (r *Reader) readCodeLine(expectCode int) (code int, continued bool, message string, err error) {
	// 读取一行的数据
	line, err := r.ReadLine()
	if err != nil {
		return
	}
	// 进行解析
	return parseCodeLine(line, expectCode)
}

// expectCode实际是代表期望的code前缀:
//   if expectCode is 31, an error will be returned if the status is not in the range [310,319].
//   An expectCode <= 0 disables the check of the status code.
// code: 实际读取到的code
// continued: 读取到的行是否是连续行(是否还有后续行)
// message: code对应的message
func parseCodeLine(line string, expectCode int) (code int, continued bool, message string, err error) {
	// len(line) < 4: 不可能发生,因为code是3位数,之后是空格. 这就已经4个字节了
	// line[3] != ' ': 比如'310 d',line[3]一定是' '或'-'
	if len(line) < 4 || line[3] != ' ' && line[3] != '-' {
		// '310' 错误   (只含数字码)
		// '310d' 错误  (数字码后不是' ' 或 '-')
		err = ProtocolError("short response: " + line)
		return
	}
	// 是否是连续行
	continued = line[3] == '-'
	code, err = strconv.Atoi(line[0:3])
	if err != nil || code < 100 {
		// 不可能出现100以内的code
		err = ProtocolError("invalid response code: " + line)
		return
	}
	// 310-msg, msg是从line[4]开始
	// expectCode实际代表一个数字前缀
	// 下面的 func (r *Reader) ReadCodeLine(expectCode int) (code int, message string, err error) 中文档解释
	// expectCode 实际是代表期望的code前缀: if expectCode is 31, an error will be returned if the status
	// is not in the range [310,319].
	message = line[4:]
	if 1 <= expectCode && expectCode < 10 && code/100 != expectCode ||
		10 <= expectCode && expectCode < 100 && code/10 != expectCode ||
		100 <= expectCode && expectCode < 1000 && code != expectCode {
		err = &Error{code, message}
	}
	return
}

// ReadCodeLine reads a response code line of the form
//	code message
// where code is a three-digit status code and the message
// extends to the rest of the line. An example of such a line is:
//	220 plan9.bell-labs.com ESMTP
//
// If the prefix of the status does not match the digits in expectCode,
// ReadCodeLine returns with err set to &Error{code, message}.
// For example, if expectCode is 31, an error will be returned if
// the status is not in the range [310,319].
//
// 上文中 &Error{code, message} 是指 textproto.Error 这个类型
//
// If the response is multi-line, ReadCodeLine returns an error.
//
// An expectCode <= 0 disables the check of the status code.
//
// ReadCodeLine的一次调用,无论是否成功,一定会跨越一行,参考: func TestReadCodeLine(t *testing.T) {
func (r *Reader) ReadCodeLine(expectCode int) (code int, message string, err error) {
	code, continued, message, err := r.readCodeLine(expectCode)
	if err == nil && continued {
		// 文档:If the response is multi-line, ReadCodeLine returns an error.
		err = ProtocolError("unexpected multi-line response: " + message)
	}
	return
}

// ReadResponse reads a multi-line response of the form:
//
//	code-message line 1
//	code-message line 2
//	...
//	code message line n
//
// where code is a three-digit status code. The first line starts with the
// code and a hyphen. The response is terminated by a line that starts
// with the same code followed by a space. Each line in message is
// separated by a newline (\n).
//
// See page 36 of RFC 959 (http://www.ietf.org/rfc/rfc959.txt) for
// details of another form of response accepted:
//
//  code-message line 1
//  message line 2
//  ...
//  code message line n
//
// If the prefix of the status does not match the digits in expectCode,
// ReadResponse returns with err set to &Error{code, message}.
// For example, if expectCode is 31, an error will be returned if
// the status is not in the range [310,319].
//
// An expectCode <= 0 disables the check of the status code.
//
//
// 上文中 &Error{code, message} 是指 textproto.Error 这个类型
func (r *Reader) ReadResponse(expectCode int) (code int, message string, err error) {
	// 读取第一行
	code, continued, message, err := r.readCodeLine(expectCode)
	// multi代表第一行是否是连续行
	multi := continued
	for continued {
		// continued代表上次循环时读取到的行是否是连续行
		// 读取单独的一行
		line, err := r.ReadLine()
		if err != nil {
			return 0, "", err
		}

		// 当前读取行的code
		var code2 int
		// 当前读取行的message
		var moreMessage string
		// 根据文档:An expectCode <= 0 disables the check of the status code.
		code2, continued, moreMessage, err = parseCodeLine(line, 0)
		if err != nil || code2 != code {
			// parseCodeLine出错 || code2 != code
			message += "\n" + strings.TrimRight(line, "\r\n")
			continued = true
			// 忽略当前行
			continue
		}
		// 拼接读取到的message
		message += "\n" + moreMessage
	}
	if err != nil && multi && message != "" {
		// replace one line error message with all lines (full message)
		err = &Error{code, message}
	}
	return
}

// DotReader returns a new Reader that satisfies Reads using the
// decoded text of a dot-encoded block read from r.
// The returned Reader is only valid until the next call
// to a method on r.
//
// DotReader返回一个新的io.Reader,假设为rr,
// 从rr进行读取,读取到的是decoded text(decoded text是从r中读取到的dot-encoded block进行的解密).
//
// Dot encoding is a common framing used for data blocks
// in text protocols such as SMTP.  The data consists of a sequence
// of lines, each of which ends in "\r\n".  The sequence itself
// ends at a line containing just a dot: ".\r\n".  Lines beginning
// with a dot are escaped with an additional dot to avoid
// looking like the end of the sequence.
//
// The decoded form returned by the Reader's Read method
// rewrites the "\r\n" line endings into the simpler "\n",
// removes leading dot escapes if present, and stops with error io.EOF
// after consuming (and discarding) the end-of-sequence line.
//
// 上文中:The decoded form returned by the Reader's Read method:
// (指DotReader返回的Reader的Read方法)
//
//
// SMTP协议简介: http://www.cnpaf.net/Class/SMTP/200408/106.html
// SMTP协议的命令和应答: http://www.cnpaf.net/Class/SMTP/200408/107.html
func (r *Reader) DotReader() io.Reader {
	// closeDot后r.dot=nil
	r.closeDot()
	// 重新构造r.dot
	r.dot = &dotReader{r: r}
	return r.dot
}

type dotReader struct {
	r     *Reader
	state int
}

// Read satisfies reads by decoding dot-encoded data read from d.r.
func (d *dotReader) Read(b []byte) (n int, err error) {
	// Run data through a simple state machine to
	// elide leading dots, rewrite trailing \r\n into \n,
	// and detect ending .\r\n line.
	const (
		stateBeginLine = iota // beginning of line; initial state; must be zero
		stateDot              // read . at beginning of line
		stateDotCR            // read .\r at beginning of line
		stateCR               // read \r (possibly at end of line)
		stateData             // reading data in middle of line
		stateEOF              // reached .\r\n end marker line
	)
	// 取名br,因为d.r.R是*bufio.Reader
	br := d.r.R
	for n < len(b) && d.state != stateEOF {
		// 在需要进行读取的情况下循环读取
		var c byte
		// 读取一个字节
		c, err = br.ReadByte()
		if err != nil {
			if err == io.EOF {
				// for循环中声明了n<len(b),因此肯定是遇到了非预期的EOF
				err = io.ErrUnexpectedEOF
			}
			break
		}
		switch d.state {
		case stateBeginLine:
			if c == '.' {
				d.state = stateDot
				continue
			}
			if c == '\r' {
				d.state = stateCR
				continue
			}
			d.state = stateData

		case stateDot:
			if c == '\r' {
				d.state = stateDotCR
				continue
			}
			if c == '\n' {
				d.state = stateEOF
				continue
			}
			d.state = stateData

		case stateDotCR:
			if c == '\n' {
				d.state = stateEOF
				continue
			}
			// Not part of .\r\n.
			// Consume leading dot and emit saved \r.
			br.UnreadByte()
			c = '\r'
			d.state = stateData

		case stateCR:
			if c == '\n' {
				d.state = stateBeginLine
				break
			}
			// Not part of \r\n. Emit saved \r
			br.UnreadByte()
			c = '\r'
			d.state = stateData

		case stateData:
			if c == '\r' {
				d.state = stateCR
				continue
			}
			if c == '\n' {
				d.state = stateBeginLine
			}
		}
		// 读取到的字节最终存入b
		b[n] = c
		n++
	}
	if err == nil && d.state == stateEOF {
		err = io.EOF
	}
	if err != nil && d.r.dot == d {
		// 文档:When Read reaches EOF or an error, it will set r.dot == nil.
		d.r.dot = nil
	}
	return
}

// closeDot drains the current DotReader if any,
// making sure that it reads until the ending dot line.
func (r *Reader) closeDot() {
	if r.dot == nil {
		return
	}
	// 这里只是声明了一个很小的buf,然后循环读取消费;并不是一次性声明一个大的内存
	buf := make([]byte, 128)
	for r.dot != nil {
		// When Read reaches EOF or an error,
		// it will set r.dot == nil.
		// 消费掉
		r.dot.Read(buf)
	}
}

// ReadDotBytes reads a dot-encoding and returns the decoded data.
//
// See the documentation for the DotReader method for details about dot-encoding.
//
// 返回解码后的数据.
// 本方法的文档可以查看DotReader的文档.
func (r *Reader) ReadDotBytes() ([]byte, error) {
	// r.DotReader()返回io.Reader, 正好是ioutil.ReadAll需要的参数
	return ioutil.ReadAll(r.DotReader())
}

// ReadDotLines reads a dot-encoding and returns a slice
// containing the decoded lines, with the final \r\n or \n elided from each.
//
// See the documentation for the DotReader method for details about dot-encoding.
//
// @see
func (r *Reader) ReadDotLines() ([]string, error) {
	// We could use ReadDotBytes and then Split it,
	// but reading a line at a time avoids needing a
	// large contiguous block of memory and is simpler.
	// v,err是最终要返回的结果.这里的v声明后还只是一个nil slice.
	// 注意,append可以作用于nil slice.
	var v []string
	var err error
	for {
		var line string
		line, err = r.ReadLine()
		if err != nil {
			if err == io.EOF {
				// 不可能遇到EOF,只有下面遇到单行数据'.'才能是数据读完
				err = io.ErrUnexpectedEOF
			}
			// 这里跳出循环是因为出错
			break
		}

		// Dot by itself marks end; otherwise cut one dot.
		if len(line) > 0 && line[0] == '.' {
			// 如果是 '.' 打头
			if len(line) == 1 {
				// 数据全部读完,跳出整个循环.
				break
			}
			// 数据还未读完,还有数据; // otherwise cut one dot. 
			line = line[1:]
		}
		// 写入读取到的数据,注意,append可以作用于nil slice.
		v = append(v, line)
	}
	return v, err
}

// ReadMIMEHeader reads a MIME-style header from r.
// The header is a sequence of possibly continued Key: Value lines
// ending in a blank line.
// The returned map m maps CanonicalMIMEHeaderKey(key) to a
// sequence of values in the same order encountered in the input.
//
// For example, consider this input:
//
//	My-Key: Value 1
//	Long-Key: Even
//	       Longer Value
//	My-Key: Value 2
//
// Given that input, ReadMIMEHeader returns the map:
//
//	map[string][]string{
//		"My-Key": {"Value 1", "Value 2"},
//		"Long-Key": {"Even Longer Value"},
//	}
//
//
// @see
func (r *Reader) ReadMIMEHeader() (MIMEHeader, error) {
	// Avoid lots of small slice allocations later by allocating one
	// large one ahead of time which we'll cut up into smaller
	// slices. If this isn't big enough later, we allocate small ones.
	// 仅仅是声明,下面会根据猜测header头部大概有hint行进行make
	// strs是提前分配的内存,后面会拿给MIMEHeader用
	var strs []string
	// 猜测header头部大概有hint行
	hint := r.upcomingHeaderNewlines()
	if hint > 0 {
		strs = make([]string, hint)
	}
	// 如果猜测失败,strs是一个nil slice.

	// 最后要返回的结果; MIMEHeader的底层类型是map[string][]string,但仍然可以进行make(MIMEHeader)
	m := make(MIMEHeader, hint)
	for {
		// 读取一个连续行
		kv, err := r.readContinuedLineSlice()
		if len(kv) == 0 {
			return m, err
		}

		// Key ends at first colon; should not have spaces but
		// they appear in the wild, violating specs, so we
		// remove them if present.
		i := bytes.IndexByte(kv, ':')
		if i < 0 {
			// 找不到':'
			return m, ProtocolError("malformed MIME header line: " + string(kv))
		}
		// key的结束位置,默认是冒号的位置
		endKey := i
		for endKey > 0 && kv[endKey-1] == ' ' {
			// 略过':'之前的空格
			endKey--
		}
		// 现在,endKey代表实际的key的结束位移
		// kv[:endKey] 代表当前读取到的连续行的 key
		key := canonicalMIMEHeaderKey(kv[:endKey])

		// As per RFC 7230 field-name is a token, tokens consist of one or more chars.
		// We could return a ProtocolError here, but better to be liberal in what we
		// accept, so if we get an empty key, skip it.
		// liberal ['lɪb(ə)r(ə)l] adj. 自由主义的；慷慨的；不拘泥的；宽大的
		if key == "" {
			continue
		}

		// Skip initial spaces in value.
		i++ // skip colon
		for i < len(kv) && (kv[i] == ' ' || kv[i] == '\t') {
			// 略过冒号后的空白
			i++
		}
		// i现在代表value的起始位置, kv[i:] 代表value
		value := string(kv[i:])

		vv := m[key]
		if vv == nil && len(strs) > 0 {
			// m[key]不存在 && strs还有多余
			// More than likely this will be a single-element key.
			// Most headers aren't multi-valued.
			// Set the capacity on strs[0] to 1, so any future append
			// won't extend the slice into the other strings.
			// 注意这个技巧,限制cap=1,以便将来的append不会扩充数据导致混乱
			// 即使后面对vv进行append,也是分配新的内存,跟strs没关系.
			vv, strs = strs[:1:1], strs[1:]
			vv[0] = value
			m[key] = vv
		} else {
			// append会自动处理新的内存分配,并且append也能作用于nil slice
			m[key] = append(vv, value)
		}

		if err != nil {
			return m, err
		}
	}
}

// upcomingHeaderNewlines returns an approximation of the number of newlines
// that will be in this header. If it gets confused, it returns 0.
//
// 猜测header部分大概会有多少行
func (r *Reader) upcomingHeaderNewlines() (n int) {
	// Try to determine the 'hint' size.
	// 下面的force a buffer load if empty是什么意思?Peek内部会调用fill向缓冲中填充数据
	r.R.Peek(1) // force a buffer load if empty
	s := r.R.Buffered()
	if s == 0 {
		return
	}
	// Peek出缓冲中的数据,s是上方计算好的缓冲长度
	peek, _ := r.R.Peek(s)
	for len(peek) > 0 {
		i := bytes.IndexByte(peek, '\n')
		if i < 3 {
			// Not present (-1) or found within the next few bytes,
			// implying we're at the end ("\r\n\r\n" or "\n\n")
			// 如果i=-1:没找到'\n',也就是猜测失败
			// 如果i<3,说明是找到了header的结尾("\r\n\r\n" or "\n\n")
			return
		}
		n++
		peek = peek[i+1:]
	}
	return
}

// CanonicalMIMEHeaderKey returns the canonical format of the
// MIME header key s. The canonicalization converts the first
// letter and any letter following a hyphen to upper case;
// the rest are converted to lowercase. For example, the
// canonical key for "accept-encoding" is "Accept-Encoding".
// MIME header keys are assumed to be ASCII only.
// If s contains a space or invalid header field bytes, it is
// returned without modifications.
func CanonicalMIMEHeaderKey(s string) string {
	// Quick check for canonical encoding.
	// upper:当前for循环中的字节是否应该是大写; 最开始的字节应该是大写,因此默认为true
	upper := true
	for i := 0; i < len(s); i++ {
		// 当前循环的字节
		c := s[i]
		if !validHeaderFieldByte(c) {
			// 文档:If s contains a space or invalid header field bytes, it is returned without modifications.
			return s
		}
		if upper && 'a' <= c && c <= 'z' {
			// 如果该大写的没有大写,只在必要时进行函数调用
			return canonicalMIMEHeaderKey([]byte(s))
		}
		if !upper && 'A' <= c && c <= 'Z' {
			// 如果该小写的没有小写,只在必要时进行函数调用
			return canonicalMIMEHeaderKey([]byte(s))
		}
		// 下一轮循环的字符是否应该是大写,根据当前循环的字符是否是横线来决定
		upper = c == '-'
	}
	return s
}

// 大写转换为小写,需要减去多少
const toLower = 'a' - 'A'

// validHeaderFieldByte reports whether b is a valid byte in a header
// field name. RFC 7230 says:
//   header-field   = field-name ":" OWS field-value OWS
//   field-name     = token
//   tchar = "!" / "#" / "$" / "%" / "&" / "'" / "*" / "+" / "-" / "." /
//           "^" / "_" / "`" / "|" / "~" / DIGIT / ALPHA
//   token = 1*tchar
func validHeaderFieldByte(b byte) bool {
	return int(b) < len(isTokenTable) && isTokenTable[b]
}

// canonicalMIMEHeaderKey is like CanonicalMIMEHeaderKey but is
// allowed to mutate the provided byte slice before returning the
// string.
//
// For invalid inputs (if a contains spaces or non-token bytes), a
// is unchanged and a string copy is returned.
func canonicalMIMEHeaderKey(a []byte) string {
	// See if a looks like a header key. If not, return it unchanged.
	for _, c := range a {
		if validHeaderFieldByte(c) {
			continue
		}
		// Don't canonicalize.
		// 文档:For invalid inputs (if a contains spaces or non-token bytes), a is unchanged and a string copy is returned.
		return string(a)
	}
	// 现在,a中不存在非法的header字节

	// 当前循环中的rune是否应该是大写; 最开始的字节应该是大写,因此默认为true
	upper := true
	for i, c := range a {
		// Canonicalize: first letter upper case
		// and upper case after each dash.
		// (Host, User-Agent, If-Modified-Since).
		// MIME headers are ASCII only, so no Unicode issues.
		if upper && 'a' <= c && c <= 'z' {
			// 应该是大写,实际是小写
			c -= toLower
		} else if !upper && 'A' <= c && c <= 'Z' {
			// 应该是小写,实际是大写
			c += toLower
		}
		a[i] = c
		upper = c == '-' // for next time
	}
	// The compiler recognizes m[string(byteSlice)] as a special
	// case, so a copy of a's bytes into a new string does not
	// happen in this map lookup:
	if v := commonHeader[string(a)]; v != "" {
		// 此时不会发生从a([]byte)到string的copy
		return v
	}
	return string(a)
}

// commonHeader interns common header strings.
var commonHeader = make(map[string]string)

func init() {
	for _, v := range []string{
		"Accept",
		"Accept-Charset",
		"Accept-Encoding",
		"Accept-Language",
		"Accept-Ranges",
		"Cache-Control",
		"Cc",
		"Connection",
		"Content-Id",
		"Content-Language",
		"Content-Length",
		"Content-Transfer-Encoding",
		"Content-Type",
		"Cookie",
		"Date",
		"Dkim-Signature",
		"Etag",
		"Expires",
		"From",
		"Host",
		"If-Modified-Since",
		"If-None-Match",
		"In-Reply-To",
		"Last-Modified",
		"Location",
		"Message-Id",
		"Mime-Version",
		"Pragma",
		"Received",
		"Return-Path",
		"Server",
		"Set-Cookie",
		"Subject",
		"To",
		"User-Agent",
		"Via",
		"X-Forwarded-For",
		"X-Imforwards",
		"X-Powered-By",
	} {
		commonHeader[v] = v
	}
}

// isTokenTable is a copy of net/http/lex.go's isTokenTable.
// See https://httpwg.github.io/specs/rfc7230.html#rule.token.separators
var isTokenTable = [127]bool{
	'!':  true,
	'#':  true,
	'$':  true,
	'%':  true,
	'&':  true,
	'\'': true,
	'*':  true,
	'+':  true,
	'-':  true,
	'.':  true,
	'0':  true,
	'1':  true,
	'2':  true,
	'3':  true,
	'4':  true,
	'5':  true,
	'6':  true,
	'7':  true,
	'8':  true,
	'9':  true,
	'A':  true,
	'B':  true,
	'C':  true,
	'D':  true,
	'E':  true,
	'F':  true,
	'G':  true,
	'H':  true,
	'I':  true,
	'J':  true,
	'K':  true,
	'L':  true,
	'M':  true,
	'N':  true,
	'O':  true,
	'P':  true,
	'Q':  true,
	'R':  true,
	'S':  true,
	'T':  true,
	'U':  true,
	'W':  true,
	'V':  true,
	'X':  true,
	'Y':  true,
	'Z':  true,
	'^':  true,
	'_':  true,
	'`':  true,
	'a':  true,
	'b':  true,
	'c':  true,
	'd':  true,
	'e':  true,
	'f':  true,
	'g':  true,
	'h':  true,
	'i':  true,
	'j':  true,
	'k':  true,
	'l':  true,
	'm':  true,
	'n':  true,
	'o':  true,
	'p':  true,
	'q':  true,
	'r':  true,
	's':  true,
	't':  true,
	'u':  true,
	'v':  true,
	'w':  true,
	'x':  true,
	'y':  true,
	'z':  true,
	'|':  true,
	'~':  true,
}
