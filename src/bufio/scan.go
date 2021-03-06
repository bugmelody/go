// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// [[[5-over]]] 2017-6-8 13:39:29

package bufio

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

// Scanner provides a convenient interface for reading data such as
// a file of newline-delimited lines of text. Successive calls to
// the Scan method will step through the 'tokens' of a file, skipping
// the bytes between the tokens. The specification of a token is
// defined by a split function of type SplitFunc; the default split
// function breaks the input into lines with line termination stripped. Split
// functions are defined in this package for scanning a file into
// lines, bytes, UTF-8-encoded runes, and space-delimited words. The
// client may instead provide a custom split function.
//
// Scanning stops unrecoverably at EOF, the first I/O error, or a token too
// large to fit in the buffer. When a scan stops, the reader may have
// advanced arbitrarily far past the last token. Programs that need more
// control over error handling or large tokens, or must run sequential scans
// on a reader, should use bufio.Reader instead.
//
// 上文的tokens指有效数据
// Scanner虽然方便,但是控制权太少,如果需要更多控制权,应该使用bufio.Reader
// r: 从 r 中读取数据.
// maxTokenSize: 一个token最大可以有多大
// scanCalled: 标记 Scan() 方法被调用过
// done: 如果已经 Scan 完毕, 再次调用 Scan() 只会返回 false
type Scanner struct {
	r            io.Reader // The reader provided by the client.
	split        SplitFunc // The function to split the tokens.
	maxTokenSize int       // Maximum size of a token; modified by tests.
	token        []byte    // Last token returned by split.
	buf          []byte    // Buffer used as argument to split.
	start        int       // First non-processed byte in buf.
	end          int       // End of data in buf.
	err          error     // Sticky error.
	empties      int       // Count of successive empty tokens.
	scanCalled   bool      // Scan has been called; buffer is in use.
	done         bool      // Scan has finished.
}

// SplitFunc is the signature of the split function used to tokenize the
// input. The arguments are an initial substring of the remaining unprocessed
// data and a flag, atEOF, that reports whether the Reader has no more data
// to give. The return values are the number of bytes to advance the input
// and the next token to return to the user, plus an error, if any. If the
// data does not yet hold a complete token, for instance if it has no newline
// while scanning lines, SplitFunc can return (0, nil, nil) to signal the
// Scanner to read more data into the slice and try again with a longer slice
// starting at the same point in the input.
//
// If the returned error is non-nil, scanning stops and the error
// is returned to the client.
//
// The function is never called with an empty data slice unless atEOF
// is true. If atEOF is true, however, data may be non-empty and,
// as always, holds unprocessed text.
//
// atEOF 参数: reports whether the Reader has no more data to give
// (可能是Scan过程出错,也可能是遇到EOF)
//
// 注意: 在 Scanner.Scan 的实现中,只有这样一处调用:
// s.split(s.buf[s.start:s.end], s.err != nil)
// 也就是说, s.split 是作用在 buffer 的有效数据上面, 这里 s.err != nil 表示如果Scan过程出错,或遇到EOF的时候, atEOF=true
type SplitFunc func(data []byte, atEOF bool) (advance int, token []byte, err error)

// Errors returned by Scanner.
var (
	ErrTooLong         = errors.New("bufio.Scanner: token too long")
	ErrNegativeAdvance = errors.New("bufio.Scanner: SplitFunc returns negative advance count")
	ErrAdvanceTooFar   = errors.New("bufio.Scanner: SplitFunc returns advance count beyond input")
)

const (
	// MaxScanTokenSize is the maximum size used to buffer a token
	// unless the user provides an explicit buffer with Scan.Buffer.
	// The actual maximum token size may be smaller as the buffer
	// may need to include, for instance, a newline.
	MaxScanTokenSize = 64 * 1024

	// buffer 初始分配的空间.
	startBufSize = 4096 // Size of initial allocation for buffer.
)

// NewScanner returns a new Scanner to read from r.
// The split function defaults to ScanLines.
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{
		r:            r,
		split:        ScanLines,
		maxTokenSize: MaxScanTokenSize,
	}
}

// Err returns the first non-EOF error that was encountered by the Scanner.
func (s *Scanner) Err() error {
	if s.err == io.EOF {
		// 将数据读完视为非错.
		return nil
	}
	return s.err
}

// Bytes returns the most recent token generated by a call to Scan.
// The underlying array may point to data that will be overwritten
// by a subsequent call to Scan. It does no allocation.
func (s *Scanner) Bytes() []byte {
	return s.token
}

// Text returns the most recent token generated by a call to Scan
// as a newly allocated string holding its bytes.
func (s *Scanner) Text() string {
	return string(s.token)
}

// ErrFinalToken is a special sentinel error value. It is intended to be
// returned by a Split function to indicate that the token being delivered
// with the error is the last token and scanning should stop after this one.
// After ErrFinalToken is received by Scan, scanning stops with no error.
// The value is useful to stop processing early or when it is necessary to
// deliver a final empty token. One could achieve the same behavior
// with a custom error value but providing one here is tidier.
// See the emptyFinalToken example for a use of this value.
var ErrFinalToken = errors.New("final token")

// Scan advances the Scanner to the next token, which will then be
// available through the Bytes or Text method. It returns false when the
// scan stops, either by reaching the end of the input or an error.
// After Scan returns false, the Err method will return any error that
// occurred during scanning, except that if it was io.EOF, Err
// will return nil.
// Scan panics if the split function returns 100 empty tokens without
// advancing the input. This is a common error mode for scanners.
//
// 方法内部会不停的循环直到找到可用token,或失败,或完毕.
func (s *Scanner) Scan() bool {
	if s.done {
		// qc: It returns false when the scan stops, either by reaching the end of the input or an error.
		return false
	}
	// 标记Scan方法被调用过
	s.scanCalled = true
	// Loop until we have a token.
	// 无限循环,直到发现一个完整的token. 这个 for 循环也是本函数的最后一条语句.
	// 一旦找到了一个 token ,就会 return. 或者在发生错误的情况下, 也 return.
	for {
		// See if we can get a token with what we already have.
		// If we've run out of data but have an error, give the split function
		// a chance to recover any remaining, possibly empty token.
		if s.end > s.start || s.err != nil {
			// 如果buffer还有数据 || buffer没有数据并且之前出现了错误

			// s.end > s.start: 说明 buffer 中还有数据;
			// s.start: First non-processed byte in buf.
			// s.end: End of data in buf. 
			// s.err != nil: 之前有错误

			// 对 buffer 中的数据应用 split 回调, 如果 s.err != nil, 说明之前出了问题,数据应该结束
			advance, token, err := s.split(s.buf[s.start:s.end], s.err != nil)
			if err != nil {
				// 根据ErrFinalToken文档: s.split返回err!=nil:此时可能是真的出错,也可能是ErrFinalToken,需要区分
				if err == ErrFinalToken {
					// split返回的token是最后一个token
					// s.token 用于存储: Last token returned by split.
					s.token = token
					// 标记整个scan结束
					s.done = true
					// 返回true表示找到了token
					return true
				}
				// 现在,err != ErrFinalToken, 说明确实出错了
				s.setErr(err)
				// 根据文档:
				// It returns false when the scan stops, either by reaching the end of the input or an error.
				// After Scan returns false, the Err method will return any error that occurred during
				// scanning, except that if it was io.EOF, Err will return nil.
				return false
			}
			// 现在,s.split返回的err == nil. 说明split函数正常返回.
			if !s.advance(advance) {
				// 尝试进行advance,advance失败返回false.
				return false
			}
			// 现在, 已经进行了 advance.
			// 保存 split 返回的 token
			s.token = token
			// 处理 s.empties
			if token != nil {
				// token != nil: 说明确实读取到了一个 token
				if s.err == nil || advance > 0 {
					// 读取到了token,将 s.empties 重置为 0. s.empties 代表 Count of successive empty tokens.
					s.empties = 0
				} else {
					// Returning tokens not advancing input at EOF.
					// 返回的 token 并没有 advancing input 到 EOF.
					s.empties++
					if s.empties > 100 {
						// 连续 100 次获取到空 token 而没有进行 advance.
						panic("bufio.Scan: 100 empty tokens without progressing")
					}
				}
				return true
			}
		}
		// 上面整个这个 if 分支是从 buf 中进行 split.
		// We cannot generate a token with what we are holding.
		// If we've already hit EOF or an I/O error, we are done.
		// 现在, 我们无法从 buf 中得到一个 token.如果此时已经出现了 EOF or an I/O error, 说明工作完成, 返回 false.
		if s.err != nil {
			// Shut it down.
			s.start = 0
			s.end = 0
			return false
		}
		// Must read more data.
		// First, shift data to beginning of buffer if there's lots of empty space
		// or space is needed.
		// 现在, buffer 中没有找到 token, 工作未完成, 需要源读取更多数据.
		// 如果 buf 中没有空间了, 或者是需要更多的空间, 需要 shift buffer 中的数据到最开始处.(实际是尽量重复利用 buf 这块空间)
		if s.start > 0 && (s.end == len(s.buf) || s.start > len(s.buf)/2) {
			// 现在,该整理 buf 中的空间了, 需要利用那些未利用的空间.
			// s.start>0&&(s.end==len(s.buf): start 之前有未利用的空间 && s.end 已经到了 buf 的末尾
			// s.start > len(s.buf)/2): start 之前未利用的空间超过了 buf 的一半长度
			copy(s.buf, s.buf[s.start:s.end])
			s.end -= s.start
			s.start = 0
		}
		// buf空间整理完成.

		// 如果buf空间够用,此时应该 s.end != len(s.buf)
		// 反之,如果出现 s.end == len(s.buf), 说明 buf 已经被利用完,应当通过resize分配更多的内存.
		
		// Is the buffer full? If so, resize.
		if s.end == len(s.buf) {
			// Guarantee no overflow in the multiplication below.
			// 可以通过const在函数内部if条件下定义常量
			const maxInt = int(^uint(0) >> 1)
			if len(s.buf) >= s.maxTokenSize || len(s.buf) > maxInt/2 {
				// 确保 len(s.buf) 不会太大
				s.setErr(ErrTooLong)
				return false
			}
			// newSize: 下方 newBuf 需要的大小
			// [start: 计算 newSize]
			// 新的 buf 的空间,空间长度设置为当前 buf 长度的两倍
			newSize := len(s.buf) * 2
			if newSize == 0 {
				// * 2 还是0, 说明是初次为 buf 分配空间
				// startBufSize 代表 buffer 初始分配的空间大小.
				newSize = startBufSize
			}
			if newSize > s.maxTokenSize {
				// token 最大不能超过 s.maxTokenSize, 如果超过了, 进行修正
				newSize = s.maxTokenSize
			}
			// [end: 计算 newSize]
			// 分配新空间
			newBuf := make([]byte, newSize)
			// 将老buf空间的内容cp到新buf空间
			copy(newBuf, s.buf[s.start:s.end])
			// 设置 s.buf 为新分配的空间(老的空间之后就没有用了,会被回收掉)
			s.buf = newBuf
			// ???????什么意思
			s.end -= s.start
			s.start = 0
		}
		// 现在, buf 空间足够.
		// Finally we can read some input. Make sure we don't get stuck with
		// a misbehaving Reader. Officially we don't need to do this, but let's
		// be extra careful: Scanner is for safe, simple jobs.
		// misbehave [mɪsbɪ'heɪv] vi. 作弊；行为不礼貌 vt. 使举止失礼；使行为不端
		// officially [ə'fɪʃəlɪ] adv. 正式地；官方地；作为公务员
		// loop 代表读取到了空数据的次数
		for loop := 0; ; {
			// 从源中读取数据到 buffer
			n, err := s.r.Read(s.buf[s.end:len(s.buf)])
			// 更新 buffer 的结束位置
			s.end += n
			if err != nil {
				s.setErr(err)
				break
			}
			if n > 0 {
				// 读取到了数据
				s.empties = 0
				break
			}
			// 现在,读取到了空数据
			loop++
			if loop > maxConsecutiveEmptyReads {
				// 如果连续读取空数据达到 maxConsecutiveEmptyReads 次
				s.setErr(io.ErrNoProgress)
				break
			}
		}
	}
}

// advance consumes n bytes of the buffer. It reports whether the advance was legal.
func (s *Scanner) advance(n int) bool {
	if n < 0 {
		s.setErr(ErrNegativeAdvance)
		return false
	}
	// 确认 n 是否合法
	if n > s.end-s.start {
		// 要 advance 的距离已经超过了 buffer 有效数据的长度
		s.setErr(ErrAdvanceTooFar)
		return false
	}
	// 现在, n 值合法
	// 更新 s.start, 代表: First non-processed byte in buf. 也即是 buffer 的开始偏移量.
	s.start += n
	return true
}

// setErr records the first error encountered.
func (s *Scanner) setErr(err error) {
	// io.EOF 视为非错
	if s.err == nil || s.err == io.EOF {
		// 如果之前没有发生错误, 才将 err 设置给 s.err.
		s.err = err
	}
}

// Buffer sets the initial buffer to use when scanning and the maximum
// size of buffer that may be allocated during scanning. The maximum
// token size is the larger of max and cap(buf). If max <= cap(buf),
// Scan will use this buffer only and do no allocation.
//
// By default, Scan uses an internal buffer and sets the
// maximum token size to MaxScanTokenSize.
//
// Buffer panics if it is called after scanning has started.
func (s *Scanner) Buffer(buf []byte, max int) {
	if s.scanCalled {
		// Scan 方法已经被调用过
		panic("Buffer called after Scan")
	}
	// 扩充buf的length到cap
	s.buf = buf[0:cap(buf)]
	s.maxTokenSize = max
}

// Split sets the split function for the Scanner.
// The default split function is ScanLines.
//
// Split panics if it is called after scanning has started.
func (s *Scanner) Split(split SplitFunc) {
	if s.scanCalled {
		// Scan 方法已经被调用过
		panic("Split called after Scan")
	}
	s.split = split
}

// Split functions

// ScanBytes is a split function for a Scanner that returns each byte as a token.
func ScanBytes(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		// 根据文档: SplitFunc can return (0, nil, nil) to signal the Scanner
		// to read more data into the slice and try again with a longer slice
		// starting at the same point in the input.
		return 0, nil, nil
	}
	return 1, data[0:1], nil
}

// rune 转型为 string, 再转型为 []byte
// 为什么要两次转型
// 经过测试, rune 不能直接转型为 []byte
// var testRune2 = []byte('a') // 报错: cannot convert 'a' (type rune) to type []byte
var errorRune = []byte(string(utf8.RuneError))

// ScanRunes is a split function for a Scanner that returns each
// UTF-8-encoded rune as a token. The sequence of runes returned is
// equivalent to that from a range loop over the input as a string, which
// means that erroneous UTF-8 encodings translate to U+FFFD = "\xef\xbf\xbd".
// Because of the Scan interface, this makes it impossible for the client to
// distinguish correctly encoded replacement runes from encoding errors.
func ScanRunes(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		// 根据文档: SplitFunc can return (0, nil, nil) to signal the Scanner
		// to read more data into the slice and try again with a longer slice
		// starting at the same point in the input.
		return 0, nil, nil
	}

	// Fast path 1: ASCII.
	if data[0] < utf8.RuneSelf {
		//  data 是以单字节字符开头.
		return 1, data[0:1], nil
	}
	// 到这里,说明 data 是以多字节字符开头
	// Fast path 2: Correct UTF-8 decode without error.
	_, width := utf8.DecodeRune(data)
	if width > 1 {
		// It's a valid encoding. Width cannot be one for a correctly encoded
		// non-ASCII rune.
		// 上面 Fast path 1 已经处理了单字节字符,因此这里一定是多字节字符. 除非遇到了 erroneous UTF-8 encodings.
		return width, data[0:width], nil
	}
	// 现在, 遇到了 erroneous UTF-8 encodings.

	// We know it's an error: we have width==1 and implicitly r==utf8.RuneError.
	// Is the error because there wasn't a full rune to be decoded?
	// FullRune distinguishes correctly between erroneous and incomplete encodings.
	if !atEOF && !utf8.FullRune(data) {
		// Incomplete; get more bytes.
		// 说明 data 中的数据不完整
		return 0, nil, nil
	}
	// 到这里,说明data是以一个完整的 UTF-8 编码字符开头,只是这个字符是 UTF-8 encoding error.
	// 现在才是真正遇到了 UTF-8 encoding error.

	// We have a real UTF-8 encoding error. Return a properly encoded error rune
	// but advance only one byte. This matches the behavior of a range loop over
	// an incorrectly encoded string.
	return 1, errorRune, nil
}

// dropCR drops a terminal \r from the data.
//
// CR: \r : carriage return
// terminal ['tɜːmɪn(ə)l] n. 末端；终点；终端机；极限 adj. 末端的；终点的；晚期的
// 丢弃 data 中末尾的 \r
func dropCR(data []byte) []byte {
	// 为什么这里可以使用 rune 和 byte 进行比较 ? data[len(data)-1] == '\r' ?
	// 参考: http://stackoverflow.com/questions/19310700/what-is-a-rune
	// rune 和 byte 底层都是整数类型, 当字符是 单字节字符的时候, 可以直接比较 rune 和 byte.
	if len(data) > 0 && data[len(data)-1] == '\r' {
		// 如果 data 长度大于 0, 并且最后一个byte是 '\r'
		// 返回的数据中丢弃掉 \r
		return data[0 : len(data)-1]
	}
	// 否则,将data原样返回
	return data
}

// ScanLines is a split function for a Scanner that returns each line of
// text, stripped of any trailing end-of-line marker. The returned line may
// be empty. The end-of-line marker is one optional carriage return followed
// by one mandatory newline. In regular expression notation, it is `\r?\n`.
// The last non-empty line of input will be returned even if it has no
// newline.
func ScanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	// 有数据, 存在 \n
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		// dropCR(data[0:i]): \n 之前可能是 \r
		return i + 1, dropCR(data[0:i]), nil
	}
	// 有数据, 不存在 \n
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	// Request more data.
	return 0, nil, nil
}

// isSpace reports whether the character is a Unicode white space character.
// We avoid dependency on the unicode package, but check validity of the implementation
// in the tests.
func isSpace(r rune) bool {
	// 0000-007F：C0控制符及基本拉丁文 (C0 Control and Basic Latin)
	// 0080-00FF：C1控制符及拉丁文补充-1 (C1 Control and Latin 1 Supplement)
	// 0100-017F：拉丁文扩展-A (Latin Extended-A)
	// 0180-024F：拉丁文扩展-B (Latin Extended-B)
	// 0250-02AF：国际音标扩展 (IPA Extensions)
	// 02B0-02FF：空白修饰字母 (Spacing Modifiers)
	// 0300-036F：结合用读音符号 (Combining Diacritics Marks)

	// 16进制 FF = 2进制 11111111 = 十进制 255
	if r <= '\u00FF' {
		// Obvious ASCII ones: \t through \r plus space. Plus two Latin-1 oddballs.
		switch r {
		case ' ', '\t', '\n', '\v', '\f', '\r':
			return true
		case '\u0085', '\u00A0':
			return true
		}
		return false
	}
	// High-valued ones.
	if '\u2000' <= r && r <= '\u200a' {
		return true
	}
	switch r {
	case '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000':
		return true
	}
	return false
}

// ScanWords is a split function for a Scanner that returns each
// space-separated word of text, with surrounding spaces deleted. It will
// never return an empty string. The definition of space is set by
// unicode.IsSpace.
func ScanWords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Skip leading spaces.
	// [start: 计算 start 变量]
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		// isSpace 是本包中实现的, 模拟了 unicode.IsSpace
		if !isSpace(r) {
			break
		}
	}
	// 现在, start 变量已经被计算好,代表 start 之前是 space, start 开始就不是
	// [end: 计算 start 变量]
	// Scan until space, marking end of word.
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		// isSpace 是本包中实现的, 模拟了 unicode.IsSpace
		if isSpace(r) {
			// 结束点 找到了
			return i + width, data[start:i], nil
		}
	}
	// 现在,没有找到 end of word.
	// If we're at EOF, we have a final, non-empty, non-terminated word. Return it.
	if atEOF && len(data) > start {
		// len(data) > start: data的结束位置 > 之前计算的start
		return len(data), data[start:], nil
	}
	// Request more data.
	// 注意, 这里是 start, nil, nil, 表示希望请求更多数据,但是需要 advance start 个位置.
	return start, nil, nil
}
