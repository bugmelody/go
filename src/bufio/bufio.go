// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// [[[5-over]]] 2017-6-8 13:39:29

// Package bufio implements buffered I/O. It wraps an io.Reader or io.Writer
// object, creating another object (Reader or Writer) that also implements
// the interface but provides buffering and some help for textual I/O.
package bufio

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

/**
defaultBufSize: 默认的缓冲size
参考下面三个函数
func NewReader(rd io.Reader) *Reader {
func NewWriter(w io.Writer) *Writer {
func NewWriterSize(w io.Writer, size int) *Writer {
 */
const (
	defaultBufSize = 4096
)

/**
ErrBufferFull: buffer 已经满了,没法向 buffer 中添加更多数据
 */

var (
	ErrInvalidUnreadByte = errors.New("bufio: invalid use of UnreadByte")
	ErrInvalidUnreadRune = errors.New("bufio: invalid use of UnreadRune")
	ErrBufferFull        = errors.New("bufio: buffer full")
	ErrNegativeCount     = errors.New("bufio: negative count")
)

// Buffered input.

// Reader implements buffering for an io.Reader object.
//
//                         rd
//                         ↓
// buf0.......r............w
//            →---缓冲数据--←
//     →-已读-←
//
// rd是数据的来源
// lastByte: 最后读取的那个字节
// lastRuneSize: 最后读取的那个rune有多少字节
// rd的数据读,对应写到buf[w]的位置
// buf[r:w]之间的数据属于是缓冲下来的数据
// buf[0:r]之间的数据属于是缓冲中已经被读取的数据
type Reader struct {
	buf          []byte
	rd           io.Reader // reader provided by the client
	r, w         int       // buf read and write positions
	err          error
	lastByte     int
	lastRuneSize int
}

// Reader.buf分配最小的size
const minReadBufferSize = 16
// 最大连续读取多少次空数据之后报告 io.ErrNoProgress
const maxConsecutiveEmptyReads = 100

// NewReaderSize returns a new Reader whose buffer has at least the specified
// size. If the argument io.Reader is already a Reader with large enough
// size, it returns the underlying Reader.
//
// 上文的Reader指bufio.Reader
func NewReaderSize(rd io.Reader, size int) *Reader {
	// Is it already a Reader?
	// 如果rd已经是一个*bufio.Reader
	b, ok := rd.(*Reader)
	if ok && len(b.buf) >= size {
		// 如果参数 rd 已经是*bufio.Reader, 并且 len(b.buf) 至少有 size 的长度, 直接返回 b
		// 注意,这里是返回的 b, 而不是 rd, b 通过 type assertion 操作后是 bufio.Reader 类型
		// 返回的b类型为*bufio.Reader
		return b
	}
	if size < minReadBufferSize {
		// 修正size
		size = minReadBufferSize
	}
	r := new(Reader)
	r.reset(make([]byte, size), rd)
	return r
}

// NewReader returns a new Reader whose buffer has the default size.
func NewReader(rd io.Reader) *Reader {
	return NewReaderSize(rd, defaultBufSize)
}

// Reset discards any buffered data, resets all state, and switches
// the buffered reader to read from r.
//
// Reset并没有分配新的缓冲内存,仍然使用原来的缓冲
// Reset之后,仍然使用原来的b.buf作为缓冲,但是读取源变为r
func (b *Reader) Reset(r io.Reader) {
	b.reset(b.buf, r)
}

func (b *Reader) reset(buf []byte, r io.Reader) {
	*b = Reader{
		buf:          buf,
		rd:           r,
		lastByte:     -1,
		lastRuneSize: -1,
	}
}

var errNegativeRead = errors.New("bufio: reader returned negative count from Read")

// fill reads a new chunk into the buffer.
// fill 过程中会调用 b.rd.Read 将 b.rd 中的数据读入一块到 b.buf
// 如果 fill 过程出错,会将 error 设置到 b.err
// 如果要获取 fill 过程中的错误,可以调用 'func (b *Reader) readErr() error {'
func (b *Reader) fill() {
	// Slide existing data to beginning.
	if b.r > 0 {
		// r之前的数据已经被读取过了,那段空间已经没有用了
		// b.r > 0, 说明 b.r 之前有未利用的空间. 进行滑动, 将现有数据滑动到 b.buf 的最开始处

		// 数据向左滑动r的距离
		copy(b.buf, b.buf[b.r:b.w])
		// b.w也需要向左滑动r的距离
		b.w -= b.r
		// b.r也需要向左滑动r的距离,b.r - r = 0
		b.r = 0
	}
	// slide 完毕

	if b.w >= len(b.buf) {
		// buffer空间已经满了,没办法再通过fill利用
		panic("bufio: tried to fill full buffer")
	}

	// Read new data: try a limited number of times.
	// 最大连续读取maxConsecutiveEmptyReads次空数据之后报告io.ErrNoProgress
	for i := maxConsecutiveEmptyReads; i > 0; i-- {
		// 将b.rd数据源中的数据读入b.buf中的b.w位置处
		n, err := b.rd.Read(b.buf[b.w:])
		if n < 0 {
			panic(errNegativeRead)
		}
		// 更新 buf write position
		b.w += n
		if err != nil {
			// b.rd.Read 出错
			b.err = err
			return
		}
		if n > 0 {
			// 在 for 循环中,只要有1轮确实成功读取到数据,就返回
			return
		}
	}

	// 上面 for 循环中如果正常读取,会 return; 否则,到了这里,说明读取异常,多次循环读取后仍然无进度
	b.err = io.ErrNoProgress
}

// readErr 返回上次 b.rd.Read 读取时发生的错误(同时会清理 b.err)
func (b *Reader) readErr() error {
	err := b.err
	b.err = nil
	return err
}

// Peek returns the next n bytes without advancing the reader. The bytes stop
// being valid at the next read call. If Peek returns fewer than n bytes, it
// also returns an error explaining why the read is short. The error is
// ErrBufferFull if n is larger than b's buffer size.
//
// 上文中的The bytes指返回的 []byte
func (b *Reader) Peek(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrNegativeCount
	}

	b.lastByte = -1
	b.lastRuneSize = -1

	for b.w-b.r < n && b.w-b.r < len(b.buf) && b.err == nil {
		// b.w-b.r 代表当前在没有fill的情况下可以从b.buf中读到多少数据. 也就是当前实际被缓冲了多少字节.
		// b.w-b.r < len(b.buf): buffer is not full, 还能继续从b.rd读取数据放入buffer.
		// b.err == nil 代表上轮 b.rd.Read 读取操作没有错误.
		// b.fill() 内部会调用 b.rd.Read 进行读取操作
		b.fill() // b.w-b.r < len(b.buf) => buffer is not full
	}

	// n是函数参数,代表要peek多少字节
	if n > len(b.buf) {
		// qc: The error is ErrBufferFull if n is larger than b's buffer size
		return b.buf[b.r:b.w], ErrBufferFull
	}

	// 现在, 0 <= n <= len(b.buf)
	// 0 <= n <= len(b.buf)
	// err 是函数最后将要返回的 error, zero value 是 nil
	var err error
	// avail: 代表当前可以从 b.buf 中读到的数据长度.
	if avail := b.w - b.r; avail < n {
		// not enough data in buffer

		// avail < n : 说明 buffer 中的数据不足 n.
		// 根据函数定义: The error is ErrBufferFull if n is larger than b's buffer size

		// 修正n为:buffer中实际有多少数据可供读取
		n = avail
		err = b.readErr()
		if err == nil {
			// The error is ErrBufferFull if n is larger than b's buffer size
			// 没有错误,但是根据函数定义, 修正为有错.
			// ErrBufferFull 的优先级比 b.readErr() 的优先级低
			err = ErrBufferFull
		}
	}
	// 注意在整个函数中没有更新过b.r和b.w
	return b.buf[b.r : b.r+n], err
}

// Discard skips the next n bytes, returning the number of bytes discarded.
//
// If Discard skips fewer than n bytes, it also returns an error.
// If 0 <= n <= b.Buffered(), Discard is guaranteed to succeed without
// reading from the underlying io.Reader.
//
// 如果0 <= n <= b.Buffered(),Discard只会丢弃已缓冲的数据
// 如果n > b.Buffered(), 会循环丢弃缓冲的数据和数据源的数据,直到出错或者已经丢弃够了了n个字节.
//
// --------------------------------------------------
// b.Buffered() 说明: Buffered returns the number of bytes that can be read from the current buffer.
// func (b *Reader) Buffered() int { return b.w - b.r }
// --------------------------------------------------
func (b *Reader) Discard(n int) (discarded int, err error) {
	if n < 0 {
		return 0, ErrNegativeCount
	}
	if n == 0 {
		return
	}
	// remain 代表 : 还剩多少字节需要丢弃, 初始等于 n, 下面的每轮循环中会进行减小
	remain := n
	for {
		skip := b.Buffered()
		if skip == 0 {
			// if skip == 0 表示 没有数据可以从 b.buf 中读取了
			// 这时候需要调用 b.fill() 将 b.rd 的数据读入 b.buf
			b.fill()
			// 填充后,重新调用 b.Buffered() 得到当前可以从缓冲(b.buf)中读取多少字节
			skip = b.Buffered()
		}
		// 现在, skip 代表了本轮循环中当前可以从buffer中读取字节数
		if skip > remain {
			// 当前可以从buffer中读取字节数 > 还剩多少字节需要丢弃
			// 说明当前循环可以完成本函数期望的操作
			// 修正可以读取的字节数
			skip = remain
		}
		// 现在,skip 代表了实际应该 discard 的字节数
		// 增加 b.r 位移,也就是实际进行skip操作
		b.r += skip
		// 更新 还剩多少字节需要丢弃
		remain -= skip
		if remain == 0 {
			// 如果没有字节需要再丢弃了, 说明函数成功, 返回 n 和 err=nil
			return n, nil
		}
		if b.err != nil {
			// 说明在 fill 过程中出现了错误
			// n - remain : 代表已丢弃的字节数, b.readErr() 会返回 fill 过程中的错误
			return n - remain, b.readErr()
		}
	}
}

// Read reads data into p.
// It returns the number of bytes read into p.
// The bytes are taken from at most one Read on the underlying Reader,
// hence n may be less than len(p).
// At EOF, the count will be zero and err will be io.EOF.
//
// 读出来的数据可能来自于buffer,也可能来自underlying Reader.
// Read 会将数据读入 p. 它会返回读入p的字节数.
// 读入的bytes最多从underlying Reader进行一次拿取 (b.rd.Read(p)), 因此 n 可能会小于 len(p)
func (b *Reader) Read(p []byte) (n int, err error) {
	// n 是函数命名返回值,代表实际读取了多少字节. 这里先假设为 p 的长度
	n = len(p)
	if n == 0 {
		// 也就是说len(p) == 0
		// b.readErr() 会返回上次 b.rd.Read 读取时发生的错误, 也就是说,上次 b.rd.Read(p) 的错误可能这次才返回
		return 0, b.readErr()
	}

	// 如果 if b.r == b.w, empty buffer, 说明 buffer 没有数据了, 需要从 b.rd 中读数据到 buffer.
	if b.r == b.w {
		// buffer是空的
		if b.err != nil {
			// 上轮 b.rd.Read 读取时有错误发生, 返回上轮b.rd.Read读取错误
			return 0, b.readErr()
		}

		// 现在, 上轮读取没有错误发生
		if len(p) >= len(b.buf) {
			// Large read, empty buffer.
			// Read directly into p to avoid copy.

			// 需要要读取的数据长度大于buffer的长度,buffer是空的(b.r == b.w),
			// 直接从 b.rd 读入 p,避免 buffer 中转过程的拷贝

			// 直接读入p, 不经过 b.buf
			n, b.err = b.rd.Read(p)
			if n < 0 {
				panic(errNegativeRead)
			}
			if n > 0 {
				// 读到了数据
				// 记录最后读取到的那个字节
				b.lastByte = int(p[n-1])
				// 上一个操作不是ReadRune
				b.lastRuneSize = -1
			}
			// 返回读取到多少字节和读取过程中可能发生的错误
			return n, b.readErr()
		}
		// One read.
		// Do not use b.fill, which will loop.
		// 注意这里仍然在 if b.r == b.w { 条件的控制下
		b.r = 0
		b.w = 0
		n, b.err = b.rd.Read(b.buf)
		if n < 0 {
			panic(errNegativeRead)
		}
		if n == 0 {
			return 0, b.readErr()
		}
		b.w += n
	}

	// 到这里,说明buffer中有数据,需要从buffer copy数据到目标p.
	
	// copy as much as we can
	// copy buffer 数据到目标 p.
	n = copy(p, b.buf[b.r:b.w])
	// b.r += copy字节数
	b.r += n
	// 记录最后读取到的那个字节
	b.lastByte = int(b.buf[b.r-1])
	// 上一个操作不是ReadRune
	b.lastRuneSize = -1
	return n, nil
}

// ReadByte reads and returns a single byte.
// If no byte is available, returns an error.
//
// 上文中的 If no byte is available (buffer和底层数据源都没有数据了)
// 如果 b.r == b.w 的时候, ReadByte内部会调用 b.fill()
func (b *Reader) ReadByte() (byte, error) {
	b.lastRuneSize = -1
	// b.r == b.w ,说明 buffer empty(buffer 没有数据了),需要 fill
	for b.r == b.w {
		if b.err != nil {
			// 上一个read操作有错误
			return 0, b.readErr()
		}
		// 现在, 上一个read操作没有错误
		b.fill() // buffer is empty
	}
	// 现在, b.buf 已经被填充了数据,直接读取 buffer 中第一个字节即可.
	c := b.buf[b.r]
	b.r++
	// 记录最后读取的字节
	b.lastByte = int(c)
	return c, nil
}

// UnreadByte unreads the last byte. Only the most recently read byte can be unread.
//
// 注意: Unread the last byte from any read operation. 这个不需要上一个操作是 ReadByte(),
// 只需是任意一个 read 操作即可.
func (b *Reader) UnreadByte() error {
	if b.lastByte < 0 || b.r == 0 && b.w > 0 {
		// b.lastByte < 0: 说明上一个操作不是 read 相关操作
		// b.r == 0 && b.w > 0 : 此时无法回退,无法进行 unread
		return ErrInvalidUnreadByte
	}
	// b.r > 0 || b.w == 0
	if b.r > 0 {
		// 有回退余地,进行回退
		b.r--
	} else {
		// 没有回退余地
		// b.r == 0 && b.w == 0
		b.w = 1
		// b.w = 1 ?????
	}
	b.buf[b.r] = byte(b.lastByte)
	b.lastByte = -1
	b.lastRuneSize = -1
	return nil
}

// ReadRune reads a single UTF-8 encoded Unicode character and returns the
// rune and its size in bytes. If the encoded rune is invalid, it consumes one byte
// and returns unicode.ReplacementChar (U+FFFD) with a size of 1.
//
// $ go doc io.RuneReader
func (b *Reader) ReadRune() (r rune, size int, err error) {
	// b.r <-> b.w 之间代表 b.buf 中可读的数据
	// b.r+utf8.UTFMax > b.w : 意思是说当前 b.buf 中的可读数据可能放不下一个 UTF8 字符
	// !utf8.FullRune(b.buf[b.r:b.w]): b.buf[b.r:b.w] 不是以一个完整的utf8编码的字符开头.
	// b.err == nil 之前没有读取错误.
	// b.w-b.r < len(b.buf): buffer is not full, buffer 未满, 还可以进行填充.
	for b.r+utf8.UTFMax > b.w && !utf8.FullRune(b.buf[b.r:b.w]) && b.err == nil && b.w-b.r < len(b.buf) {
		// 注意: b.fill()是以字节的概念进行填充的,
		// 因此fill()后, r.....w  , w可能不是以一个完整的rune结尾
		b.fill() // b.w-b.r < len(buf) => buffer is not full
	}
	b.lastRuneSize = -1
	if b.r == b.w {
		// fill 之后 buffer 仍然是 empty
		return 0, 0, b.readErr()
	}
	// 下面开始读取 buffer 中的第一个字符
	// 初始假设是单字节字符情况,因此可以进行 rune(xxx) 转换
	r, size = rune(b.buf[b.r]), 1
	if r >= utf8.RuneSelf {
		// 如果是多字节字符,修正 r 和 size
		r, size = utf8.DecodeRune(b.buf[b.r:b.w])
	}
	b.r += size
	b.lastByte = int(b.buf[b.r-1])
	b.lastRuneSize = size
	return r, size, nil
}

// UnreadRune unreads the last rune. If the most recent read operation on
// the buffer was not a ReadRune, UnreadRune returns an error.  (In this
// regard it is stricter than UnreadByte, which will unread the last byte
// from any read operation.)
//
// 上一个操作必须是 ReadRune()
func (b *Reader) UnreadRune() error {
	if b.lastRuneSize < 0 || b.r < b.lastRuneSize {
		// b.lastRuneSize < 0: 上一个操作不是 ReadRune
		// b.r < b.lastRuneSize:  buf [012r], b.lastRuneSize 居然大于 b.r
		return ErrInvalidUnreadRune
	}
	b.r -= b.lastRuneSize
	b.lastByte = -1
	b.lastRuneSize = -1
	return nil
}

// Buffered returns the number of bytes that can be read from the current buffer.
//
// 已缓冲多少字节
func (b *Reader) Buffered() int { return b.w - b.r }

// ReadSlice reads until the first occurrence of delim in the input,
// returning a slice pointing at the bytes in the buffer.
// The bytes stop being valid at the next read.
// If ReadSlice encounters an error before finding a delimiter,
// it returns all the data in the buffer and the error itself (often io.EOF).
// ReadSlice fails with error ErrBufferFull if the buffer fills without a delim.
// Because the data returned from ReadSlice will be overwritten
// by the next I/O operation, most clients should use
// ReadBytes or ReadString instead.
// ReadSlice returns err != nil if and only if line does not end in delim.
//
// 返回的line,pointing at the bytes in the buffer(注意,是 in the buffer)
// 返回的 line []byte 直到下一次 read 之前都是有效的
func (b *Reader) ReadSlice(delim byte) (line []byte, err error) {
	for {
		// Search buffer.
		// 在当前的buffer中搜索delim
		if i := bytes.IndexByte(b.buf[b.r:b.w], delim); i >= 0 {
			// 如果在当前的buffer中搜索到了delim
			// 返回的 line 末尾包含 delim
			line = b.buf[b.r : b.r+i+1]
			// 下次的读取从 delim 后的字节开始
			b.r += i + 1
			break
		}
		// 到了这里,说明在当前的buffer中没有搜索到delim

		// Pending error?
		if b.err != nil {
			// 如果之前的读取操作有错误

			// 根据文档: If ReadSlice encounters an error before finding a delimiter, it
			// returns all the data in the buffer and the error itself (often io.EOF).
			
			line = b.buf[b.r:b.w]
			b.r = b.w
			err = b.readErr()
			break
		}

		// Buffer full?
		if b.Buffered() >= len(b.buf) {
			// 根据文档: ReadSlice fails with error ErrBufferFull if
			// the buffer fills without a delim.
			b.r = b.w
			line = b.buf
			err = ErrBufferFull
			break
		}

		b.fill() // buffer is not full
		// 多次 fill 之后,可能会造成 buffer 中有效 buffer 不断增
		// 长, b.Buffered() >= len(b.buf),因此上面有个条件专门检查这种情况
	}

	// 参考前面的三个break.
	// 到了这里,要么找到了 delim, 要么由于 buffer 满了跳出循环, 要么出现读取错误

	// Handle last byte, if any.
	if i := len(line) - 1; i >= 0 {
		// 记录lastByte
		b.lastByte = int(line[i])
		// 上一个操作不是ReadRune
		b.lastRuneSize = -1
	}

	// 注意: 根据函数文档: returning a slice pointing at the bytes in the buffer
	// 现在,line都是指向buffer中的slice
	return
}

// ReadLine is a low-level line-reading primitive. Most callers should use
// ReadBytes('\n') or ReadString('\n') instead or use a Scanner.
//
// ReadLine tries to return a single line, not including the end-of-line bytes.
// If the line was too long for the buffer then isPrefix is set and the
// beginning of the line is returned. The rest of the line will be returned
// from future calls. isPrefix will be false when returning the last fragment
// of the line. The returned buffer is only valid until the next call to
// ReadLine. ReadLine either returns a non-nil line or it returns an error,
// never both.
//
// The text returned from ReadLine does not include the line end ("\r\n" or "\n").
// No indication or error is given if the input ends without a final line end.
// Calling UnreadByte after ReadLine will always unread the last byte read
// (possibly a character belonging to the line end) even if that byte is not
// part of the line returned by ReadLine.
//
// 注:
// ReadLine tries to return a single line, not including the
// end-of-line bytes.(也就是不含行结束符号,比如 "\r\n" or "\n")
//
// 由于ReadLine返回值去除了line end,因此整个源文本是否以line end结
// 束,是无法从ReadLine返回值得知的
//
// 一行可能需要调用多次,前几次返回isPrefix为true,最后一次返回isPrefix为false
//
// 注意,返回的line是buffer中的一段
func (b *Reader) ReadLine() (line []byte, isPrefix bool, err error) {
	// 读到 \n 的位置, 根据 ReadSlice 的文档, 如果返回的 err!=nil, 则意味着 line 不是以 delim 结束
	line, err = b.ReadSlice('\n')
	if err == ErrBufferFull {
		// 根据ReadSlice的文档: ReadSlice fails with error ErrBufferFull if the buffer fills without a delim.
		// 如果是由于ReadSlice过程中buffer满了造成的错误,说明此时没有遇到 '\n'
		// 但也可能是刚到 '\r', 马上到 '\n' 之前, buffer 就满了
		
		
		// Handle the case where "\r\n" straddles the buffer.
		// straddle ['stræd(ə)l] vi. 跨坐；两腿叉开坐 vt. 叉开(腿)；骑，跨；跨立于；跨越 n. 跨坐
		if len(line) > 0 && line[len(line)-1] == '\r' {
			// line 的长度大于 0 && line 的最后一个字符是 '\r'

			// 如果刚到'\r',可能下一个字符是'\n',也可能不是
			// Put the '\r' back on buf and drop it from line.
			// Let the next call to ReadLine check for "\r\n".
			if b.r == 0 {
				// should be unreachable
				// 尝试 rewind 越过 buffer 的起始处
				panic("bufio: tried to rewind past start of buffer")
			}
			// 递减一个位置,将\r放回buf, 让下一次ReadLine调用检查\n
			b.r--
			// 修改 line ,去掉最后的 '\r' 这个字节
			line = line[:len(line)-1]
		}
		// true表示返回的是行前缀,不是完整行
		return line, true, nil
	}

	// 现在,b.ReadSlice的返回的err肯定不是ErrBufferFull

	if len(line) == 0 {
		// 如果之前的 b.ReadSlice('\n') 没有读取到任何数据
		
		if err != nil {
			// 根据 ReadSlice 的文档, 如果返回的 err!=nil, 则意味着 line 不是以 delim 结束

			// 根据 ReadLine 文档: ReadLine either returns a non-nil line or it returns an error, never both.
			// 也就是 line 和 err 只能有一个是 nil
			line = nil
		}
		// 如果err!=nil,此时返回 (nil, false, non-nil)
		// 如果err==nil,此时返回 ([]byte, false, nil)
		return
	}

	// 到这里, 说明b.ReadSlice('\n')读取到了数据,len(line)!=0
	// 根据本函数文档: ReadLine either returns a non-nil line or it returns an error, never both.
	err = nil

	if line[len(line)-1] == '\n' {
		// 如果 line 最后一个字节是 '\n',需要丢弃一个字符
		drop := 1
		if len(line) > 1 && line[len(line)-2] == '\r' {
			//如果 line 倒数第二个字节是 '\r', 需要再丢弃一个字符
			drop = 2
		}
		// 丢弃line 结尾可能的 '\n' 或 '\r\n'
		line = line[:len(line)-drop]
	}
	return
}

// ReadBytes reads until the first occurrence of delim in the input,
// returning a slice containing the data up to and including the delimiter.
// If ReadBytes encounters an error before finding a delimiter,
// it returns the data read before the error and the error itself (often io.EOF).
// ReadBytes returns err != nil if and only if the returned data does not end in
// delim.
// For simple uses, a Scanner may be more convenient.
//
// ReadBytes将返回的数据没有以delim结束视为错误
// Scanner比ReadBytes更方便(当然性能也更差).
func (b *Reader) ReadBytes(delim byte) ([]byte, error) {
	// Use ReadSlice to look for array,
	// accumulating full buffers.

	// frag: fragment的缩写
	// 一个frag代表下方for循环中b.ReadSlice(delim)返回的byte slice
	var frag []byte
	// frag通过copy到临时buf后append到full
	var full [][]byte
	// 整个函数的err返回值
	var err error
	for {
		// 下方b.ReadSlice返回的错误
		var e error
		frag, e = b.ReadSlice(delim)
		if e == nil { // got final fragment
			// 根据 ReadSlice 的文档: 如果返回的 err!=nil, 则意味着 line 不是以 delim 结束.
			// 相反,如果返回的 err==nil, 则意味着 line 是以 delim 结束
			// 这里说明已经找到了完整的一行,跳出循环
			break
		}
		// 现在, 还没有 got final fragment,也就是还没有找到完整的一行

		// 根据 ReadSlice 的文档: ReadSlice fails with error ErrBufferFull if the buffer fills without a delim.
		if e != ErrBufferFull { // unexpected error
			err = e
			break
		}

		// 现在, e!=nil && e!=ErrBufferFull, 说明b.ReadSlice读取到了数据,但是不是完整的一行
		// 为什么要使用buf进行中转copy,不直接 full = append(full, frag)
		// 注意: ReadSlice 文档中提到: The bytes(返回的slice) stop being valid at the next read.
		// 因此必须将frag拷贝到buf保证数据的有效性
		// 否则,如果直接 full = append(full, frag),则full中的一段内存其有效性得不到保障.
		
		// Make a copy of the buffer.
		buf := make([]byte, len(frag))
		copy(buf, frag)
		full = append(full, buf)
		//  继续下轮for循环,直到找到完整的一行
	}

	// 注意: full 并非是包含所有,最后一次 ReadSlice 的结果还在 frag 变量中

	// Allocate new buffer to hold the full pieces and the fragment.
	// n代表所有有效数据的字节数,包括(full中的数据, 最后一个frag中的数据)
	n := 0
	for i := range full {
		// 加上所有full中的数据长度
		n += len(full[i])
	}
	// 加上最后一个frag的长度
	n += len(frag)

	// Copy full pieces and fragment in.
	// 整个函数最后要返回的[]byte
	buf := make([]byte, n)
	
	// 现在,n重置为0,下方的n的意义是: 标记每轮循环后,下一轮循环应该在buf的什么位置写入
	n = 0
	for i := range full {
		// 循环将full数据copy到buf
		n += copy(buf[n:], full[i])
	}
	// copy最后一个frag
	copy(buf[n:], frag)
	return buf, err
}

// ReadString reads until the first occurrence of delim in the input,
// returning a string containing the data up to and including the delimiter.
// If ReadString encounters an error before finding a delimiter,
// it returns the data read before the error and the error itself (often io.EOF).
// ReadString returns err != nil if and only if the returned data does not end in
// delim.
// For simple uses, a Scanner may be more convenient.
//
// ReadString其实是对ReadBytes的封装
func (b *Reader) ReadString(delim byte) (string, error) {
	bytes, err := b.ReadBytes(delim)
	return string(bytes), err
}

// WriteTo implements io.WriterTo.
// This may make multiple calls to the Read method of the underlying Reader.
//
// 将 b(bufio.Reader) 中的数据读出来写入 w
// 注意: io.WriterTo期望将源数据写完
func (b *Reader) WriteTo(w io.Writer) (n int64, err error) {
	// b.writeBuf(w)说明:
	// writeBuf writes the Reader's buffer to the writer.
	// writeBuf 将 Reader's buffer 中的数据(b.buf) 写入 w
	n, err = b.writeBuf(w)
	if err != nil {
		// b.writeBuf(w)失败
		return
	}

	// 现在,b.writeBuf(w)成功
	if r, ok := b.rd.(io.WriterTo); ok {
		// 如果b.rd实现了io.WriterTo接口
		// 直接将r写入w,避免经过buffer中转
		// 注意: io.WriterTo期望将源数据写完
		m, err := r.WriteTo(w)
		n += m
		// 整个函数返回
		return n, err
	}

	if w, ok := w.(io.ReaderFrom); ok {
		m, err := w.ReadFrom(b.rd)
		n += m
		return n, err
	}
	// 现在,b.rd没有实现了io.WriterTo接口,w没有实现io.ReaderFrom接口
	// 必须的经过buf进行中转了

	if b.w-b.r < len(b.buf) {
		// b.w-b.r 代表被缓冲下来的数据长度
		// len(b.buf) 代表可以缓冲的最大长度
		// buffer not full, 还能继续向 buffer 中填充数据
		b.fill() // buffer not full
	}

	for b.r < b.w {
		// 在buffer中有可读数据的条件下进行循环
		// b.r < b.w => buffer is not empty
		// 将 buffer 中的数据写入 w
		m, err := b.writeBuf(w)
		n += m
		if err != nil {
			// 出错了,整个函数返回
			return n, err
		}
		// 上面 b.writeBuf(w) 的作用是将 buffer 中的数据写入 w
		// 因此到这里的时候,buffer已经空了,需要调用b.fill填充数据
		b.fill() // buffer is empty
	}

	// 根据io.WriterTo文档: WriteTo writes data to w until there's no more data to write or when an error occurs.
	// 因此,如果遇到io.EOF,不应该当做本函数出错.
	if b.err == io.EOF {
		b.err = nil
	}

	// readErr 返回上次 b.rd.Read 读取时发生的错误(同时会清理 b.err)
	return n, b.readErr()
}

var errNegativeWrite = errors.New("bufio: writer returned negative count from Write")

// writeBuf writes the Reader's buffer to the writer.
//
// writeBuf 将 Reader's buffer 中的数据(b.buf) 写入 w
func (b *Reader) writeBuf(w io.Writer) (int64, error) {
	// io.Writer接口期望将数据写完
	n, err := w.Write(b.buf[b.r:b.w])
	if n < 0 {
		panic(errNegativeWrite)
	}
	b.r += n
	return int64(n), err
}

// buffered output

// Writer implements buffering for an io.Writer object.
// If an error occurs writing to a Writer, no more data will be
// accepted and all subsequent writes, and Flush, will return the error.
// After all data has been written, the client should call the
// Flush method to guarantee all data has been forwarded to
// the underlying io.Writer.
//
// 可以把 bufio.Writer 想象成这样的场景:
// Writer.wr 是操作系统的一个文件句柄,如果每次直接向文件写入数据,会搞得IO过于频繁.
// 因此通过 Writer.buf 进行中转,直到 Writer.buf 满了,或者是调用了 Writer.Flush,
// 缓冲中的数据才会被真正写入Writer.wr.
//
// 也就是
// 数据......Writer.buf.......Wirter.wr
//                    buf满了
//                    或者Flush被调用
//
// buf字段: 中间的缓冲区(数据源 => buf => wr)
// err字段: If an error occurs writing to a Writer, no more data will be accepted and all subsequent writes will return the error.
// n字段: the number of bytes that have been written into the current buffer. 这是当前值, 并非是积累值
// wr字段: the underlying io.Writer
type Writer struct {
	err error
	buf []byte
	n   int
	wr  io.Writer
}

// NewWriterSize returns a new Writer whose buffer has at least the specified
// size. If the argument io.Writer is already a Writer with large enough
// size, it returns the underlying Writer.
//
// 上文总的Writer指bufio.Writer
func NewWriterSize(w io.Writer, size int) *Writer {
	// Is it already a Writer?
	b, ok := w.(*Writer)
	if ok && len(b.buf) >= size {
		// 如果w已经是bufio.Writer,并且w.buf已经满足size的长度,原封不动返回底层的bufio.Writer
		return b
	}
	if size <= 0 {
		size = defaultBufSize
	}
	return &Writer{
		buf: make([]byte, size),
		wr:  w,
	}
}

// NewWriter returns a new Writer whose buffer has the default size.
func NewWriter(w io.Writer) *Writer {
	return NewWriterSize(w, defaultBufSize)
}

// Reset discards any unflushed buffered data, clears any error, and
// resets b to write its output to w.
// 注意: b.buf 仍然没变,还是原来的b.buf,只是b.n有变化.
func (b *Writer) Reset(w io.Writer) {
	b.err = nil
	b.n = 0
	b.wr = w
}

// Flush writes any buffered data to the underlying io.Writer.
//
// 将 buffered data 写入 the underlying io.Writer.
func (b *Writer) Flush() error {
	if b.err != nil {
		// qc: If an error occurs writing to a Writer, no more data will be
		// accepted and all subsequent writes will return the error.
		return b.err
	}
	if b.n == 0 {
		// b.n 说明: the number of bytes that have been written into the current buffer.
		// b.n == 0, 也就是说, 当前 buffer 中没有数据需要 flush,因此直接返回 nil.
		return nil
	}
	// 将buffer中的数据进行实际写入, 0:b.n之间的数据也就是缓冲的数据
	n, err := b.wr.Write(b.buf[0:b.n])
	if n < b.n && err == nil {
		// io.ErrShortWrite: 请求写入x字节的数据,实际写入的数据小于x
		err = io.ErrShortWrite
	}
	if err != nil {
		if n > 0 && n < b.n {
			// 如果 b.wr.Write 写入了字节但是没有 buffer 中的数据多
			// 也就是说, buffer 中的数据只写了一部分到 b.wr 中, 没有全部 flush 掉 buffer 中的数据.
			// 这里使用 copy 将 b.buf 中没有 flush 掉的那部分数据移动到 buffer(b.buf) 的最开始处
			copy(b.buf[0:b.n-n], b.buf[n:b.n])
		}
		// 重设 b.n
		b.n -= n
		// qc:If an error occurs writing to a Writer, no more data will be
		// accepted and all subsequent writes will return the error.
		// 设置b.err,以后即使再次调用本函数,会直接返回b.err(见本函数最开始处)
		b.err = err
		return err
	}
	// 现在,说明 b.wr.Write 成功, 也就是成功地 flush 掉了 buffer 中的数据到 b.wr
	b.n = 0
	return nil
}

// Available returns how many bytes are unused in the buffer.
//
// Available返回buffer还能继续缓冲多少个字节
// 假设 0123456789 是 buffer
//     |||||*****
// 假设 b.n = 4
// 则竖线是buffer的部分,星号是未使用的部分(也就是buffer中还可缓冲的字节数)
func (b *Writer) Available() int { return len(b.buf) - b.n }

// Buffered returns the number of bytes that have been written into the current buffer.
//
// Buffered返回已缓冲多少字节
// b.n: 是当前值, 并非是积累值
func (b *Writer) Buffered() int { return b.n }

// Write writes the contents of p into the buffer.
// It returns the number of bytes written.
// If nn < len(p), it also returns an error explaining
// why the write is short.
//
// 可能会触发flush,也可能不会.
func (b *Writer) Write(p []byte) (nn int, err error) {
	// len(p): 还剩多少字节需要写入; 每轮循环末尾会对p进行reslice, p = p[n:]
	// b.Available(): buffer 中还剩多少字节可供使用 (有多少空间可供缓冲)
	// ------------------------------------------------------
	// len(p) > b.Available(): 可供缓冲的空间不足以存放待写的数据. 一轮 copy 处理不完,需要循环copy
	// b.err == nil : 上一轮写入没有出错
	for len(p) > b.Available() && b.err == nil {
		// n: 本轮for循环写入的字节数
		var n int
		if b.Buffered() == 0 {
			// Large write, empty buffer.
			// Write directly from p to avoid copy.
			// 缓冲中没有数据, 直接使用 b.wr.Write 避免 copy 的二次中转.
			n, b.err = b.wr.Write(p)
		} else {
			// 缓冲中有数据, 写到缓冲中
			n = copy(b.buf[b.n:], p)
			b.n += n
			b.Flush()
		}
		// nn 是函数命名返回值,代表整个函数写入多少字节
		nn += n
		// 重设变量p
		p = p[n:]
	}
	// 到这里,说明: 一轮 copy 可以处理完毕
	if b.err != nil {
		// 上轮 write 出错了
		return nn, b.err
	}
	// 将最后还没有写的数据 copy 到 buffer
	n := copy(b.buf[b.n:], p)
	b.n += n
	nn += n
	return nn, nil
}

// WriteByte writes a single byte.
//
// 方法内部会自动判断buffer是否满了(满了的时候调用flush)
func (b *Writer) WriteByte(c byte) error {
	if b.err != nil {
		return b.err
	}
	if b.Available() <= 0 && b.Flush() != nil {
		// b.Available() <= 0: buffer 满了,没有空间了,应该进行 flush 清理 buffer 了
		// 如果应该清理 buffer 了, 于是去调用 b.flush(), 但是 b.flush() 出错
		return b.err
	}
	// 写入 c 这个 byte 到 buffer
	b.buf[b.n] = c
	b.n++
	return nil
}

// WriteRune writes a single Unicode code point, returning
// the number of bytes written and any error.
//
// 方法内部会自动判断buffer是否满了(满了的时候调用flush)
func (b *Writer) WriteRune(r rune) (size int, err error) {
	if r < utf8.RuneSelf {
		// 如果是单字节字符
		// 由于已经确定了是单字节字符,因此可以安全的用byte(r)进行类型转换
		err = b.WriteByte(byte(r))
		if err != nil {
			// b.WriteByte 出错
			return 0, err
		}
		// b.WriteByte 成功
		return 1, nil
	}
	if b.err != nil {
		// If an error occurs writing to a Writer, no more data will be accepted and all subsequent writes will return the error.
		return 0, b.err
	}
	// 现在,需要处理多字节字符情况
	n := b.Available()
	// utf8.UTFMax: UTFMax=4: 一个UTF8编码的字符最大有4个字节
	if n < utf8.UTFMax {
		// 如果buffer的可用空间不足以容纳一个较长的UTF8编码的字符
		if b.Flush(); b.err != nil {
			return 0, b.err
		}
		// 再次获取buffer的可用空间
		n = b.Available()
		if n < utf8.UTFMax {
			// Can only happen if buffer is silly small.
			// 此情形只可能在 buffer 被设置的变态的小的情况下发生.
			// 仔细看看 WriteString 的源码, 看看为什么这种情况下(n < utf8.UTFMax) 可
			// 以通过 b.WriteString 成功写入
			return b.WriteString(string(r))
		}
	}
	// 现在, buffer 中的空间一定可以容纳一个UTF8编码的字符
	// 将 r 写入 buffer
	size = utf8.EncodeRune(b.buf[b.n:], r)
	b.n += size
	return size, nil
}

// WriteString writes a string.
// It returns the number of bytes written.
// If the count is less than len(s), it also returns an error explaining
// why the write is short.
func (b *Writer) WriteString(s string) (int, error) {
	/** 注意: io 包中定义了一个非导出的 stringWriter interface
	type stringWriter interface {
		WriteString(s string) (n int, err error)
	}
	
	
	并且 io 包中定义了一个导出的 WriteString 函数
	$ go doc io.WriteString
	func WriteString(w Writer, s string) (n int, err error)
	WriteString writes the contents of the string s to w, which accepts a slice
	of bytes. If w implements a WriteString method, it is invoked directly.
	Otherwise, w.Write is called exactly once.
	
	注意: stringWriter interface 和 io.WriteString 函数签名是不同的
	 */

	// nn 是函数返回值, 这里先初始化为 0
	nn := 0
	// len(s) > b.Available(): 待写的字节数 > buffer中可用空间
	for len(s) > b.Available() && b.err == nil {
		// 这个 copy 操作不一定能 copy 完 s
		n := copy(b.buf[b.n:], s)
		// copy调用之后,buffer已经处于满了的状态,需要进行flush,因此,下面会调用flush.
		b.n += n
		nn += n
		// 重新设置 s
		s = s[n:]
		// 将缓冲中的数据flush
		b.Flush()
	}
	if b.err != nil {
		return nn, b.err
	}
	// 进行最后一次 copy
	n := copy(b.buf[b.n:], s)
	b.n += n
	nn += n
	// 此时没有必要进行flush,数据在缓冲中存在,无需落地
	return nn, nil
}

// ReadFrom implements io.ReaderFrom.
//
// go doc io.ReaderFrom
func (b *Writer) ReadFrom(r io.Reader) (n int64, err error) {
	if b.Buffered() == 0 { 
		if w, ok := b.wr.(io.ReaderFrom); ok {
			// 如果缓冲中没已缓冲的数据 && b.wr实现了io.ReaderFrom
			return w.ReadFrom(r)
		}
	}
	// m:下方for循环中每次调用 r.Read 每次返回读取多少字节, 这里仅仅是
	// 声明, 具体赋值是在循环中的每次 r.Read 中赋值的.
	var m int
	for {
		if b.Available() == 0 {
			// 缓冲空间满了,该flush了
			if err1 := b.Flush(); err1 != nil {
				return n, err1
			}
		}
		// 读取次数, 这个 nr 在每次外层 for 中清零(就是此处).
		// 也就是说,代表了在一个外层 for 循环中连续读取多少次没有读到数据.
		nr := 0
		for nr < maxConsecutiveEmptyReads {
			m, err = r.Read(b.buf[b.n:])
			if m != 0 || err != nil {
				// 读到了数据 || 发生错误, 跳出内层循环
				break
			}
			// 现在,没有读取到数据&&没有发生错误
			// 连续未读取到数据的次数
			nr++
		}
		if nr == maxConsecutiveEmptyReads {
			// 循环读取 maxConsecutiveEmptyReads 次后却没有进度
			return n, io.ErrNoProgress
		}
		// 现在, 读取到了数据, 或者是读取过程出现错误
		b.n += m
		// n 是函数的命名返回值
		n += int64(m)
		if err != nil {
			// 出错了, 跳出整个外层 for 循环
			break
		}
	}
	if err == io.EOF {
		// 如果是 io.EOF 错误, 修改为非错
		// 根据 ReadFrom 文档, Any error except io.EOF encountered during the read is also returned.

		// preemptive [pri'emptiv; ,pri:-] adj. 1.优先购买权的；具有优先购买权的 3. 抢先的；先发制人的
		// If we filled the buffer exactly, flush preemptively.
		if b.Available() == 0 {
			// 读取的数据刚好填满缓冲
			err = b.Flush()
		} else {
			err = nil
		}
	}
	return n, err
}

// buffered input and output

// ReadWriter stores pointers to a Reader and a Writer.
// It implements io.ReadWriter.
type ReadWriter struct {
	*Reader
	*Writer
}

// NewReadWriter allocates a new ReadWriter that dispatches to r and w.
func NewReadWriter(r *Reader, w *Writer) *ReadWriter {
	return &ReadWriter{r, w}
}
