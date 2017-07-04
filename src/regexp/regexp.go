// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] only-export 2017-7-3 17:06:17

// Package regexp implements regular expression search.
//
// The syntax of the regular expressions accepted is the same
// general syntax used by Perl, Python, and other languages.
// More precisely, it is the syntax accepted by RE2 and described at
// https://golang.org/s/re2syntax, except for \C.
// For an overview of the syntax, run
//   go doc regexp/syntax
//
// The regexp implementation provided by this package is
// guaranteed to run in time linear in the size of the input.
// (This is a property not guaranteed by most open source
// implementations of regular expressions.) For more information
// about this property, see
//	http://swtch.com/~rsc/regexp/regexp1.html
// or any book about automata theory.
//
// 本包中的实现保证运行时间只跟input的size有关.
//
// All characters are UTF-8-encoded code points.
//
// There are 16 methods of Regexp that match a regular expression and identify
// the matched text. Their names are matched by this regular expression:
//
//	Find(All)?(String)?(Submatch)?(Index)?
//
// If 'All' is present, the routine matches successive non-overlapping
// matches of the entire expression. Empty matches abutting a preceding
// match are ignored. The return value is a slice containing the successive
// return values of the corresponding non-'All' routine. These routines take
// an extra integer argument, n; if n >= 0, the function returns at most n
// matches/submatches.
//
// 上文中: These routines(指All系列的方法) take an extra integer argument, n
//
// 上文中: Empty matches abutting a preceding match are ignored
// (如果有一个空匹配紧靠之前的匹配,此空匹配会被忽略)
//
// abutting [ə'bʌtɪŋ] adj. 邻接的 v. 靠近；与…邻接（abut的ing形式）
// successive [sək'sesɪv] adj. 连续的；继承的；依次的；接替的
// preceding [prɪ'siːdɪŋ] adj. 在前的；前述的 v. 在...之前（precede的ing形式）
// adjusted [ə'dʒʌstɪd] adj. 调整过的，调节了的 v. 调整；校正（adjust的过去分词）
//
// If 'String' is present, the argument is a string; otherwise it is a slice
// of bytes; return values are adjusted as appropriate.
//
// If 'Submatch' is present, the return value is a slice identifying the
// successive submatches of the expression. Submatches are matches of
// parenthesized subexpressions (also known as capturing groups) within the
// regular expression, numbered from left to right in order of opening
// parenthesis. Submatch 0 is the match of the entire expression, submatch 1
// the match of the first parenthesized subexpression, and so on.
//
// If 'Index' is present, matches and submatches are identified by byte index
// pairs within the input string: result[2*n:2*n+1] identifies the indexes of
// the nth submatch. The pair for n==0 identifies the match of the entire
// expression. If 'Index' is not present, the match is identified by the
// text of the match/submatch. If an index is negative, it means that
// subexpression did not match any string in the input.
//
// 注意:上文中的'by byte index'
//
// 上面这段话怎么理解??
//
// 假设输入是 input
// result[0:1] 是整个匹配 , 也就是 result[2*0:2*0+1], 匹配文本为 input[result[0]:result[1]] 
// result[2:3] 是第1个子表达式匹配 , 也就是 result[2*1:2*1+1], 匹配文本为 input[result[2]:result[3]]
// result[4:5] 是第2个子表达式匹配 , 也就是 result[2*2:2*2+1], 匹配文本为 input[result[4]:result[5]]
// result[6:7] 是第3个子表达式匹配 , 也就是 result[2*3:2*3+1], 匹配文本为 input[result[6]:result[7]]
// 如果 result[-1(负数):-1(负数)], 表示这个子表达式没有匹配到文本. ?????? 是这样理解吗 ??????
//
// There is also a subset of the methods that can be applied to text read
// from a RuneReader:
//
//	MatchReader, FindReaderIndex, FindReaderSubmatchIndex
//
// This set may grow. Note that regular expression matches may need to
// examine text beyond the text returned by a match, so the methods that
// match text from a RuneReader may read arbitrarily far into the input
// before returning.
//
// (There are a few other methods that do not match this pattern.)
//
package regexp

import (
	"bytes"
	"io"
	"regexp/syntax"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Regexp is the representation of a compiled regular expression.
// A Regexp is safe for concurrent use by multiple goroutines,
// except for configuration methods, such as Longest.
type Regexp struct {
	// read-only after Compile
	regexpRO

	// cache of machines for running regexp
	mu      sync.Mutex
	machine []*machine
}

type regexpRO struct {
	expr           string         // as passed to Compile
	prog           *syntax.Prog   // compiled program
	onepass        *onePassProg   // onepass program or nil
	prefix         string         // required prefix in unanchored matches
	prefixBytes    []byte         // prefix, as a []byte
	prefixComplete bool           // prefix is the entire regexp
	prefixRune     rune           // first rune in prefix
	prefixEnd      uint32         // pc for last rune in prefix
	cond           syntax.EmptyOp // empty-width conditions required at start of match
	numSubexp      int
	subexpNames    []string
	longest        bool
}

// String returns the source text used to compile the regular expression.
// String返回用来编译的正则源文本,也就是传递给Compile函数的字符串
func (re *Regexp) String() string {
	return re.expr
}

// Copy returns a new Regexp object copied from re.
//
// When using a Regexp in multiple goroutines, giving each goroutine
// its own copy helps to avoid lock contention.
//
// 虽然Regexp支持并发使用,但是每个goroutine单独分配一
// 个Regexp会更好,避免lock contention.
func (re *Regexp) Copy() *Regexp {
	// It is not safe to copy Regexp by value
	// since it contains a sync.Mutex.
	return &Regexp{
		regexpRO: re.regexpRO,
	}
}

// Compile parses a regular expression and returns, if successful,
// a Regexp object that can be used to match against text.
//
// When matching against text, the regexp returns a match that
// begins as early as possible in the input (leftmost), and among those
// it chooses the one that a backtracking search would have found first.
// This so-called leftmost-first matching is the same semantics
// that Perl, Python, and other implementations use, although this
// package implements it without the expense of backtracking.
// For POSIX leftmost-longest matching, see CompilePOSIX.
//
//
// backtracking ['bæk,træk] n. [计] 回溯；回溯法 v. 原路返回；跟踪（backtrack的ing形式）
// expense [ɪk'spens; ek-] n. 损失，代价；消费；开支
func Compile(expr string) (*Regexp, error) {
	return compile(expr, syntax.Perl, false)
}

// CompilePOSIX is like Compile but restricts the regular expression
// to POSIX ERE (egrep) syntax and changes the match semantics to
// leftmost-longest.
//
// That is, when matching against text, the regexp returns a match that
// begins as early as possible in the input (leftmost), and among those
// it chooses a match that is as long as possible.
// This so-called leftmost-longest matching is the same semantics
// that early regular expression implementations used and that POSIX
// specifies.
//
// However, there can be multiple leftmost-longest matches, with different
// submatch choices, and here this package diverges from POSIX.
// 然而,可以存在多个leftmost-longest匹配(通过选择不同的submatch),在这点上本包与POSIX不同.
// Among the possible leftmost-longest matches, this package chooses
// the one that a backtracking search would have found first, while POSIX
// specifies that the match be chosen to maximize the length of the first
// subexpression, then the second, and so on from left to right.
// The POSIX rule is computationally prohibitive and not even well-defined.
// See http://swtch.com/~rsc/regexp/regexp2.html#posix for details.
//
// prohibitive [prəu'hibitiv] adj. 1.禁止性的；阻止的；抑制性的
// 2.(指价格、费用等)高得令人望而生畏的[亦作prohibitory ／prəu'hibitəri／]
func CompilePOSIX(expr string) (*Regexp, error) {
	return compile(expr, syntax.POSIX, true)
}

// Longest makes future searches prefer the leftmost-longest match.
// That is, when matching against text, the regexp returns a match that
// begins as early as possible in the input (leftmost), and among those
// it chooses a match that is as long as possible.
// This method modifies the Regexp and may not be called concurrently
// with any other methods.
func (re *Regexp) Longest() {
	re.longest = true
}

func compile(expr string, mode syntax.Flags, longest bool) (*Regexp, error) {
	re, err := syntax.Parse(expr, mode)
	if err != nil {
		return nil, err
	}
	maxCap := re.MaxCap()
	capNames := re.CapNames()

	re = re.Simplify()
	prog, err := syntax.Compile(re)
	if err != nil {
		return nil, err
	}
	regexp := &Regexp{
		regexpRO: regexpRO{
			expr:        expr,
			prog:        prog,
			onepass:     compileOnePass(prog),
			numSubexp:   maxCap,
			subexpNames: capNames,
			cond:        prog.StartCond(),
			longest:     longest,
		},
	}
	if regexp.onepass == notOnePass {
		regexp.prefix, regexp.prefixComplete = prog.Prefix()
	} else {
		regexp.prefix, regexp.prefixComplete, regexp.prefixEnd = onePassPrefix(prog)
	}
	if regexp.prefix != "" {
		// TODO(rsc): Remove this allocation by adding
		// IndexString to package bytes.
		regexp.prefixBytes = []byte(regexp.prefix)
		regexp.prefixRune, _ = utf8.DecodeRuneInString(regexp.prefix)
	}
	return regexp, nil
}

// get returns a machine to use for matching re.
// It uses the re's machine cache if possible, to avoid
// unnecessary allocation.
func (re *Regexp) get() *machine {
	re.mu.Lock()
	if n := len(re.machine); n > 0 {
		z := re.machine[n-1]
		re.machine = re.machine[:n-1]
		re.mu.Unlock()
		return z
	}
	re.mu.Unlock()
	z := progMachine(re.prog, re.onepass)
	z.re = re
	return z
}

// put returns a machine to the re's machine cache.
// There is no attempt to limit the size of the cache, so it will
// grow to the maximum number of simultaneous matches
// run using re.  (The cache empties when re gets garbage collected.)
func (re *Regexp) put(z *machine) {
	re.mu.Lock()
	re.machine = append(re.machine, z)
	re.mu.Unlock()
}

// MustCompile is like Compile but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding compiled regular
// expressions.
//
// @see
func MustCompile(str string) *Regexp {
	regexp, error := Compile(str)
	if error != nil {
		// 无法编译时发生panic, 看看quote的源码
		panic(`regexp: Compile(` + quote(str) + `): ` + error.Error())
	}
	return regexp
}

// MustCompilePOSIX is like CompilePOSIX but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding compiled regular
// expressions.
func MustCompilePOSIX(str string) *Regexp {
	regexp, error := CompilePOSIX(str)
	if error != nil {
		panic(`regexp: CompilePOSIX(` + quote(str) + `): ` + error.Error())
	}
	return regexp
}

// @see
func quote(s string) string {
	if strconv.CanBackquote(s) {
		// 如果能进行Backquote,则进行Backquote.
		return "`" + s + "`"
	}
	// 否则,使用双引号进行quote
	return strconv.Quote(s)
}

// NumSubexp returns the number of parenthesized subexpressions in this Regexp.
// NumSubexp返回正则表达式中有多少括号括起来的子表达式
func (re *Regexp) NumSubexp() int {
	return re.numSubexp
}

// SubexpNames returns the names of the parenthesized subexpressions
// in this Regexp. The name for the first sub-expression is names[1],
// so that if m is a match slice, the name for m[i] is SubexpNames()[i].
// Since the Regexp as a whole cannot be named, names[0] is always
// the empty string. The slice should not be modified.
func (re *Regexp) SubexpNames() []string {
	return re.subexpNames
}

const endOfText rune = -1

// input abstracts different representations of the input text. It provides
// one-character lookahead.
type input interface {
	step(pos int) (r rune, width int) // advance one rune
	canCheckPrefix() bool             // can we look ahead without losing info?
	hasPrefix(re *Regexp) bool
	index(re *Regexp, pos int) int
	context(pos int) syntax.EmptyOp
}

// inputString scans a string.
type inputString struct {
	str string
}

func (i *inputString) step(pos int) (rune, int) {
	if pos < len(i.str) {
		c := i.str[pos]
		if c < utf8.RuneSelf {
			return rune(c), 1
		}
		return utf8.DecodeRuneInString(i.str[pos:])
	}
	return endOfText, 0
}

func (i *inputString) canCheckPrefix() bool {
	return true
}

func (i *inputString) hasPrefix(re *Regexp) bool {
	return strings.HasPrefix(i.str, re.prefix)
}

func (i *inputString) index(re *Regexp, pos int) int {
	return strings.Index(i.str[pos:], re.prefix)
}

func (i *inputString) context(pos int) syntax.EmptyOp {
	r1, r2 := endOfText, endOfText
	// 0 < pos && pos <= len(i.str)
	if uint(pos-1) < uint(len(i.str)) {
		r1 = rune(i.str[pos-1])
		if r1 >= utf8.RuneSelf {
			r1, _ = utf8.DecodeLastRuneInString(i.str[:pos])
		}
	}
	// 0 <= pos && pos < len(i.str)
	if uint(pos) < uint(len(i.str)) {
		r2 = rune(i.str[pos])
		if r2 >= utf8.RuneSelf {
			r2, _ = utf8.DecodeRuneInString(i.str[pos:])
		}
	}
	return syntax.EmptyOpContext(r1, r2)
}

// inputBytes scans a byte slice.
type inputBytes struct {
	str []byte
}

func (i *inputBytes) step(pos int) (rune, int) {
	if pos < len(i.str) {
		c := i.str[pos]
		if c < utf8.RuneSelf {
			return rune(c), 1
		}
		return utf8.DecodeRune(i.str[pos:])
	}
	return endOfText, 0
}

func (i *inputBytes) canCheckPrefix() bool {
	return true
}

func (i *inputBytes) hasPrefix(re *Regexp) bool {
	return bytes.HasPrefix(i.str, re.prefixBytes)
}

func (i *inputBytes) index(re *Regexp, pos int) int {
	return bytes.Index(i.str[pos:], re.prefixBytes)
}

func (i *inputBytes) context(pos int) syntax.EmptyOp {
	r1, r2 := endOfText, endOfText
	// 0 < pos && pos <= len(i.str)
	if uint(pos-1) < uint(len(i.str)) {
		r1 = rune(i.str[pos-1])
		if r1 >= utf8.RuneSelf {
			r1, _ = utf8.DecodeLastRune(i.str[:pos])
		}
	}
	// 0 <= pos && pos < len(i.str)
	if uint(pos) < uint(len(i.str)) {
		r2 = rune(i.str[pos])
		if r2 >= utf8.RuneSelf {
			r2, _ = utf8.DecodeRune(i.str[pos:])
		}
	}
	return syntax.EmptyOpContext(r1, r2)
}

// inputReader scans a RuneReader.
type inputReader struct {
	r     io.RuneReader
	atEOT bool
	pos   int
}

func (i *inputReader) step(pos int) (rune, int) {
	if !i.atEOT && pos != i.pos {
		return endOfText, 0

	}
	r, w, err := i.r.ReadRune()
	if err != nil {
		i.atEOT = true
		return endOfText, 0
	}
	i.pos += w
	return r, w
}

func (i *inputReader) canCheckPrefix() bool {
	return false
}

func (i *inputReader) hasPrefix(re *Regexp) bool {
	return false
}

func (i *inputReader) index(re *Regexp, pos int) int {
	return -1
}

func (i *inputReader) context(pos int) syntax.EmptyOp {
	return 0
}

// LiteralPrefix returns a literal string that must begin any match
// of the regular expression re. It returns the boolean true if the
// literal string comprises the entire regular expression.
//
// 返回正则表达式必须匹配到的字面前缀(不包含可变部分).
// 也就是说,返回的prefix,是re对任意文本都能匹配到的前缀.
// 如果整个正则表达式刚好等于prefix,则complete返回true.
//
// comprise /kəmˈpraɪz/ (comprising,comprised,comprises)
// If you say that something comprises or is comprised of a number of things
// or people, you mean it has them as its parts or members. 包含; 由…组成
func (re *Regexp) LiteralPrefix() (prefix string, complete bool) {
	return re.prefix, re.prefixComplete
}

// MatchReader reports whether the Regexp matches the text read by the
// RuneReader.
//
// 用于测试是否匹配
func (re *Regexp) MatchReader(r io.RuneReader) bool {
	return re.doMatch(r, nil, "")
}

// MatchString reports whether the Regexp matches the string s.
//
// 用于测试是否匹配
func (re *Regexp) MatchString(s string) bool {
	return re.doMatch(nil, nil, s)
}

// Match reports whether the Regexp matches the byte slice b.
//
// 用于测试是否匹配
func (re *Regexp) Match(b []byte) bool {
	return re.doMatch(nil, b, "")
}

// MatchReader checks whether a textual regular expression matches the text
// read by the RuneReader. More complicated queries need to use Compile and
// the full Regexp interface.
func MatchReader(pattern string, r io.RuneReader) (matched bool, err error) {
	re, err := Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchReader(r), nil
}

// MatchString checks whether a textual regular expression
// matches a string. More complicated queries need
// to use Compile and the full Regexp interface.
func MatchString(pattern string, s string) (matched bool, err error) {
	re, err := Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(s), nil
}

// Match checks whether a textual regular expression
// matches a byte slice. More complicated queries need
// to use Compile and the full Regexp interface.
func Match(pattern string, b []byte) (matched bool, err error) {
	re, err := Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.Match(b), nil
}

// ReplaceAllString returns a copy of src, replacing matches of the Regexp
// with the replacement string repl. Inside repl, $ signs are interpreted as
// in Expand, so for instance $1 represents the text of the first submatch.
//
// src: 将要被替换的字符串
// repl: 替换规则,可以使用 $1 等
// 返回: 被替换后的字符串
// 例子
// re := regexp.MustCompile(`(a+)`)
// fmt.Println(re.ReplaceAllString("abaabaaabaaaab", "x")) // 输出: xbxbxbxb
// fmt.Println(re.ReplaceAllString("abaabaaabaaaab", "${1}x")) // 正确写法: 输出: axbaaxbaaaxbaaaaxb
// fmt.Println(re.ReplaceAllString("abaabaaabaaaab", "{$1}x")) // 错误写法: 输出: {a}xb{aa}xb{aaa}xb{aaaa}xb
// fmt.Println(re.ReplaceAllString("abaabaaabaaaab", "$1x")) // 错误写法: 输出: bbbb (因为不存在$1x这个匹配)
func (re *Regexp) ReplaceAllString(src, repl string) string {
	n := 2
	if strings.Contains(repl, "$") {
		n = 2 * (re.numSubexp + 1)
	}
	b := re.replaceAll(nil, src, n, func(dst []byte, match []int) []byte {
		return re.expand(dst, repl, nil, src, match)
	})
	return string(b)
}

// ReplaceAllLiteralString returns a copy of src, replacing matches of the Regexp
// with the replacement string repl. The replacement repl is substituted directly,
// without using Expand.
//
// src: 将要被替换的字符串
// repl: 替换字符串,不会被Expand
// 返回: 被替换后的字符串
// 例子
// re := regexp.MustCompile(`(a+)`)
// fmt.Println(re.ReplaceAllLiteralString("abaabaaabaaaab", "x")) // 输出: xbxbxbxb
func (re *Regexp) ReplaceAllLiteralString(src, repl string) string {
	return string(re.replaceAll(nil, src, 2, func(dst []byte, match []int) []byte {
		return append(dst, repl...)
	}))
}

// ReplaceAllStringFunc returns a copy of src in which all matches of the
// Regexp have been replaced by the return value of function repl applied
// to the matched substring. The replacement returned by repl is substituted
// directly, without using Expand.
//
//
// 例子
// re := regexp.MustCompile(`(a+)`)
// fmt.Println(re.ReplaceAllStringFunc("abaabaaabaaaab", func(x string) string { return "x"})) // 输出: xbxbxbxb
func (re *Regexp) ReplaceAllStringFunc(src string, repl func(string) string) string {
	b := re.replaceAll(nil, src, 2, func(dst []byte, match []int) []byte {
		return append(dst, repl(src[match[0]:match[1]])...)
	})
	return string(b)
}

func (re *Regexp) replaceAll(bsrc []byte, src string, nmatch int, repl func(dst []byte, m []int) []byte) []byte {
	lastMatchEnd := 0 // end position of the most recent match
	searchPos := 0    // position where we next look for a match
	var buf []byte
	var endPos int
	if bsrc != nil {
		endPos = len(bsrc)
	} else {
		endPos = len(src)
	}
	if nmatch > re.prog.NumCap {
		nmatch = re.prog.NumCap
	}

	var dstCap [2]int
	for searchPos <= endPos {
		a := re.doExecute(nil, bsrc, src, searchPos, nmatch, dstCap[:0])
		if len(a) == 0 {
			break // no more matches
		}

		// Copy the unmatched characters before this match.
		if bsrc != nil {
			buf = append(buf, bsrc[lastMatchEnd:a[0]]...)
		} else {
			buf = append(buf, src[lastMatchEnd:a[0]]...)
		}

		// Now insert a copy of the replacement string, but not for a
		// match of the empty string immediately after another match.
		// (Otherwise, we get double replacement for patterns that
		// match both empty and nonempty strings.)
		if a[1] > lastMatchEnd || a[0] == 0 {
			buf = repl(buf, a)
		}
		lastMatchEnd = a[1]

		// Advance past this match; always advance at least one character.
		var width int
		if bsrc != nil {
			_, width = utf8.DecodeRune(bsrc[searchPos:])
		} else {
			_, width = utf8.DecodeRuneInString(src[searchPos:])
		}
		if searchPos+width > a[1] {
			searchPos += width
		} else if searchPos+1 > a[1] {
			// This clause is only needed at the end of the input
			// string. In that case, DecodeRuneInString returns width=0.
			searchPos++
		} else {
			searchPos = a[1]
		}
	}

	// Copy the unmatched characters after the last match.
	if bsrc != nil {
		buf = append(buf, bsrc[lastMatchEnd:]...)
	} else {
		buf = append(buf, src[lastMatchEnd:]...)
	}

	return buf
}

// ReplaceAll returns a copy of src, replacing matches of the Regexp
// with the replacement text repl. Inside repl, $ signs are interpreted as
// in Expand, so for instance $1 represents the text of the first submatch.
//
// 例子
// re := regexp.MustCompile(`(a+)`)
// ret := re.ReplaceAll([]byte("abaabaaabaaaab"), []byte("x"))
// fmt.Println(string(ret)) // xbxbxbxb
func (re *Regexp) ReplaceAll(src, repl []byte) []byte {
	n := 2
	if bytes.IndexByte(repl, '$') >= 0 {
		n = 2 * (re.numSubexp + 1)
	}
	srepl := ""
	b := re.replaceAll(src, "", n, func(dst []byte, match []int) []byte {
		if len(srepl) != len(repl) {
			srepl = string(repl)
		}
		return re.expand(dst, srepl, src, "", match)
	})
	return b
}

// ReplaceAllLiteral returns a copy of src, replacing matches of the Regexp
// with the replacement bytes repl. The replacement repl is substituted directly,
// without using Expand.
//
// 例子
// re := regexp.MustCompile(`(a+)`)
// ret := re.ReplaceAllLiteral([]byte("abaabaaabaaaab"), []byte("${1}x"))
// fmt.Println(string(ret)) // ${1}xb${1}xb${1}xb${1}xb
func (re *Regexp) ReplaceAllLiteral(src, repl []byte) []byte {
	return re.replaceAll(src, "", 2, func(dst []byte, match []int) []byte {
		return append(dst, repl...)
	})
}

// ReplaceAllFunc returns a copy of src in which all matches of the
// Regexp have been replaced by the return value of function repl applied
// to the matched byte slice. The replacement returned by repl is substituted
// directly, without using Expand.
//
//
// re := regexp.MustCompile(`(a+)`)
// ret := re.ReplaceAllFunc([]byte("abaabaaabaaaab"), func(x []byte) []byte { return []byte("x") })
// fmt.Println(string(ret)) // xbxbxbxb
func (re *Regexp) ReplaceAllFunc(src []byte, repl func([]byte) []byte) []byte {
	return re.replaceAll(src, "", 2, func(dst []byte, match []int) []byte {
		return append(dst, repl(src[match[0]:match[1]])...)
	})
}

// Bitmap used by func special to check whether a character needs to be escaped.
var specialBytes [16]byte

// special reports whether byte b needs to be escaped by QuoteMeta.
func special(b byte) bool {
	return b < utf8.RuneSelf && specialBytes[b%16]&(1<<(b/16)) != 0
}

func init() {
	for _, b := range []byte(`\.+*?()|[]{}^$`) {
		specialBytes[b%16] |= 1 << (b / 16)
	}
}

// QuoteMeta returns a string that quotes all regular expression metacharacters
// inside the argument text; the returned string is a regular expression matching
// the literal text. For example, QuoteMeta(`[foo]`) returns `\[foo\]`.
//
// 其实不应该叫QuoteMeta,应该叫SlashMeta
func QuoteMeta(s string) string {
	// A byte loop is correct because all metacharacters are ASCII.
	var i int
	for i = 0; i < len(s); i++ {
		if special(s[i]) {
			break
		}
	}
	// No meta characters found, so return original string.
	if i >= len(s) {
		return s
	}

	b := make([]byte, 2*len(s)-i)
	copy(b, s[:i])
	j := i
	for ; i < len(s); i++ {
		if special(s[i]) {
			b[j] = '\\'
			j++
		}
		b[j] = s[i]
		j++
	}
	return string(b[:j])
}

// The number of capture values in the program may correspond
// to fewer capturing expressions than are in the regexp.
// For example, "(a){0}" turns into an empty program, so the
// maximum capture in the program is 0 but we need to return
// an expression for \1.  Pad appends -1s to the slice a as needed.
func (re *Regexp) pad(a []int) []int {
	if a == nil {
		// No match.
		return nil
	}
	n := (1 + re.numSubexp) * 2
	for len(a) < n {
		a = append(a, -1)
	}
	return a
}

// Find matches in slice b if b is non-nil, otherwise find matches in string s.
func (re *Regexp) allMatches(s string, b []byte, n int, deliver func([]int)) {
	var end int
	if b == nil {
		end = len(s)
	} else {
		end = len(b)
	}

	for pos, i, prevMatchEnd := 0, 0, -1; i < n && pos <= end; {
		matches := re.doExecute(nil, b, s, pos, re.prog.NumCap, nil)
		if len(matches) == 0 {
			break
		}

		accept := true
		if matches[1] == pos {
			// We've found an empty match.
			if matches[0] == prevMatchEnd {
				// We don't allow an empty match right
				// after a previous match, so ignore it.
				accept = false
			}
			var width int
			// TODO: use step()
			if b == nil {
				_, width = utf8.DecodeRuneInString(s[pos:end])
			} else {
				_, width = utf8.DecodeRune(b[pos:end])
			}
			if width > 0 {
				pos += width
			} else {
				pos = end + 1
			}
		} else {
			pos = matches[1]
		}
		prevMatchEnd = matches[1]

		if accept {
			deliver(re.pad(matches))
			i++
		}
	}
}

// Find returns a slice holding the text of the leftmost match in b of the regular expression.
// A return value of nil indicates no match.
//
// 之前match打头的方法判断是否匹配,而find打头的方法用于获取正则匹配的文本(第一个匹配).
//
// re := regexp.MustCompile(`(a+)`)
// ret := re.Find([]byte("abaabaaabaaaab"))
// fmt.Println(ret) // [97]
// ret = re.Find([]byte("222"))
// fmt.Println(ret) // []
func (re *Regexp) Find(b []byte) []byte {
	var dstCap [2]int
	a := re.doExecute(nil, b, "", 0, 2, dstCap[:0])
	if a == nil {
		return nil
	}
	return b[a[0]:a[1]]
}

// FindIndex returns a two-element slice of integers defining the location of
// the leftmost match in b of the regular expression. The match itself is at
// b[loc[0]:loc[1]].
// A return value of nil indicates no match.
//
// 返回b中匹配开始位置和匹配结束位置组成的slice;如果没有匹配,返回nil
// 注意,上面说到的 b[loc[0]:loc[1]] 是左开右闭.
//
// re := regexp.MustCompile(`(a+)`)
// ret := re.FindIndex([]byte("bbaab"))
// fmt.Printf("%#v\n", ret) // []int{2, 4}
// ret = re.FindIndex([]byte("222"))
// fmt.Printf("%#v\n", ret) // []int(nil)
func (re *Regexp) FindIndex(b []byte) (loc []int) {
	a := re.doExecute(nil, b, "", 0, 2, nil)
	if a == nil {
		return nil
	}
	return a[0:2]
}

// FindString returns a string holding the text of the leftmost match in s of the regular
// expression. If there is no match, the return value is an empty string,
// but it will also be empty if the regular expression successfully matches
// an empty string. Use FindStringIndex or FindStringSubmatch if it is
// necessary to distinguish these cases.
//
// 只有当正则可能会匹配空的时候才需要FindStringIndex或FindStringSubmatch来进行区分.
// 大部分情况的正则都不会匹配空,因此需要通过FindStringIndex或FindStringSubmatch来进行区分的情况很少.
//
// 例子
// re := regexp.MustCompile(`(a+)`)
// ret := re.FindString("bbaab")
// fmt.Printf("%#v\n", ret) // "aa"
// ret = re.FindString("222")
// fmt.Printf("%#v\n", ret) // ""
func (re *Regexp) FindString(s string) string {
	var dstCap [2]int
	a := re.doExecute(nil, nil, s, 0, 2, dstCap[:0])
	if a == nil {
		return ""
	}
	return s[a[0]:a[1]]
}

// FindStringIndex returns a two-element slice of integers defining the
// location of the leftmost match in s of the regular expression. The match
// itself is at s[loc[0]:loc[1]].
// A return value of nil indicates no match.
//
// 通过 s[loc[0]:loc[1]] 获取实际匹配的文本,位移是也是左闭右开.
// 例子
// re := regexp.MustCompile(`(a+)`)
// ret := re.FindStringIndex("01aa4")
// fmt.Printf("%#v\n", ret) // []int{2, 4}
// ret = re.FindStringIndex("222")
// fmt.Printf("%#v\n", ret) // []int(nil)
func (re *Regexp) FindStringIndex(s string) (loc []int) {
	a := re.doExecute(nil, nil, s, 0, 2, nil)
	if a == nil {
		return nil
	}
	return a[0:2]
}

// FindReaderIndex returns a two-element slice of integers defining the
// location of the leftmost match of the regular expression in text read from
// the RuneReader. The match text was found in the input stream at
// byte offset loc[0] through loc[1]-1.
// A return value of nil indicates no match.
//
// 本方法跟Regexp.FindStringIndex完全一样,只是输入参数的类型不同
//
// 例子
// r := strings.NewReader("01aa4")
// re := regexp.MustCompile(`(a+)`)
// ret := re.FindReaderIndex(r)
// fmt.Printf("%#v\n", ret) // []int{2, 4}
// ret = re.FindStringIndex("222")
// fmt.Printf("%#v\n", ret) // []int(nil)
func (re *Regexp) FindReaderIndex(r io.RuneReader) (loc []int) {
	a := re.doExecute(r, nil, "", 0, 2, nil)
	if a == nil {
		return nil
	}
	return a[0:2]
}

// FindSubmatch returns a slice of slices holding the text of the leftmost
// match of the regular expression in b and the matches, if any, of its
// subexpressions, as defined by the 'Submatch' descriptions in the package
// comment.
// A return value of nil indicates no match.
//
// 返回类型为 [][]byte, 假设为 a, 则 a[0] 代表re的完整匹配, a[1] 代表第1个子匹配, a[2] 代表第2个子匹配
// 例子
// re := regexp.MustCompile(`(a+)(b+)`)
// ret := re.FindSubmatch([]byte("aaabbbbb"))
// // ascii 中 0x61 对应 a, 0x62 对应 b
// fmt.Printf("%#v\n", ret) // [][]uint8{[]uint8{0x61, 0x61, 0x61, 0x62, 0x62, 0x62, 0x62, 0x62}, []uint8{0x61, 0x61, 0x61}, []uint8{0x62, 0x62, 0x62, 0x62, 0x62}}
// ret = re.FindSubmatch([]byte("222"))
// fmt.Printf("%#v\n", ret) // [][]uint8(nil)
func (re *Regexp) FindSubmatch(b []byte) [][]byte {
	var dstCap [4]int
	a := re.doExecute(nil, b, "", 0, re.prog.NumCap, dstCap[:0])
	if a == nil {
		return nil
	}
	ret := make([][]byte, 1+re.numSubexp)
	for i := range ret {
		if 2*i < len(a) && a[2*i] >= 0 {
			ret[i] = b[a[2*i]:a[2*i+1]]
		}
	}
	return ret
}

// Expand appends template to dst and returns the result; during the
// append, Expand replaces variables in the template with corresponding
// matches drawn from src. The match slice should have been returned by
// FindSubmatchIndex.
//
// In the template, a variable is denoted by a substring of the form
// $name or ${name}, where name is a non-empty sequence of letters,
// digits, and underscores. A purely numeric name like $1 refers to
// the submatch with the corresponding index; other names refer to
// capturing parentheses named with the (?P<name>...) syntax. A
// reference to an out of range or unmatched index or a name that is not
// present in the regular expression is replaced with an empty slice.
//
// In the $name form, name is taken to be as long as possible: $1x is
// equivalent to ${1x}, not ${1}x, and, $10 is equivalent to ${10}, not ${1}0.
//
// To insert a literal $ in the output, use $$ in the template.
func (re *Regexp) Expand(dst []byte, template []byte, src []byte, match []int) []byte {
	return re.expand(dst, string(template), src, "", match)
}

// ExpandString is like Expand but the template and source are strings.
// It appends to and returns a byte slice in order to give the calling
// code control over allocation.
func (re *Regexp) ExpandString(dst []byte, template string, src string, match []int) []byte {
	return re.expand(dst, template, nil, src, match)
}

func (re *Regexp) expand(dst []byte, template string, bsrc []byte, src string, match []int) []byte {
	for len(template) > 0 {
		i := strings.Index(template, "$")
		if i < 0 {
			break
		}
		dst = append(dst, template[:i]...)
		template = template[i:]
		if len(template) > 1 && template[1] == '$' {
			// Treat $$ as $.
			dst = append(dst, '$')
			template = template[2:]
			continue
		}
		name, num, rest, ok := extract(template)
		if !ok {
			// Malformed; treat $ as raw text.
			dst = append(dst, '$')
			template = template[1:]
			continue
		}
		template = rest
		if num >= 0 {
			if 2*num+1 < len(match) && match[2*num] >= 0 {
				if bsrc != nil {
					dst = append(dst, bsrc[match[2*num]:match[2*num+1]]...)
				} else {
					dst = append(dst, src[match[2*num]:match[2*num+1]]...)
				}
			}
		} else {
			for i, namei := range re.subexpNames {
				if name == namei && 2*i+1 < len(match) && match[2*i] >= 0 {
					if bsrc != nil {
						dst = append(dst, bsrc[match[2*i]:match[2*i+1]]...)
					} else {
						dst = append(dst, src[match[2*i]:match[2*i+1]]...)
					}
					break
				}
			}
		}
	}
	dst = append(dst, template...)
	return dst
}

// extract returns the name from a leading "$name" or "${name}" in str.
// If it is a number, extract returns num set to that number; otherwise num = -1.
func extract(str string) (name string, num int, rest string, ok bool) {
	if len(str) < 2 || str[0] != '$' {
		return
	}
	brace := false
	if str[1] == '{' {
		brace = true
		str = str[2:]
	} else {
		str = str[1:]
	}
	i := 0
	for i < len(str) {
		rune, size := utf8.DecodeRuneInString(str[i:])
		if !unicode.IsLetter(rune) && !unicode.IsDigit(rune) && rune != '_' {
			break
		}
		i += size
	}
	if i == 0 {
		// empty name is not okay
		return
	}
	name = str[:i]
	if brace {
		if i >= len(str) || str[i] != '}' {
			// missing closing brace
			return
		}
		i++
	}

	// Parse number.
	num = 0
	for i := 0; i < len(name); i++ {
		if name[i] < '0' || '9' < name[i] || num >= 1e8 {
			num = -1
			break
		}
		num = num*10 + int(name[i]) - '0'
	}
	// Disallow leading zeros.
	if name[0] == '0' && len(name) > 1 {
		num = -1
	}

	rest = str[i:]
	ok = true
	return
}

// FindSubmatchIndex returns a slice holding the index pairs identifying the
// leftmost match of the regular expression in b and the matches, if any, of
// its subexpressions, as defined by the 'Submatch' and 'Index' descriptions
// in the package comment.
// A return value of nil indicates no match.
// 到此
//
// 返回类型是 []int, 假设为 a, a[0:1] 是整个匹配, a[2:3] 是第一个子表达式的匹配 
// 例子
// re := regexp.MustCompile(`(a+)(b+)`)
// ret := re.FindSubmatchIndex([]byte("aaabbbbb"))
// // ascii 中 0x61 对应 a, 0x62 对应 b
// fmt.Printf("%#v\n", ret) // []int{0, 8, 0, 3, 3, 8}
// ret = re.FindSubmatchIndex([]byte("222"))
// fmt.Printf("%#v\n", ret) // []int(nil)
func (re *Regexp) FindSubmatchIndex(b []byte) []int {
	return re.pad(re.doExecute(nil, b, "", 0, re.prog.NumCap, nil))
}

// FindStringSubmatch returns a slice of strings holding the text of the
// leftmost match of the regular expression in s and the matches, if any, of
// its subexpressions, as defined by the 'Submatch' description in the
// package comment.
// A return value of nil indicates no match.
//
// 假设返回结果是a,
// a[0]代表整个正则的匹配,
// a[1]代表第一个正则子表达式的匹配,
// a[2] 代表第二个正则子表达式的匹配
// 例子
// re := regexp.MustCompile(`(a+)(b+)`)
// ret := re.FindStringSubmatch("aaabbbbb")
// // ascii 中 0x61 对应 a, 0x62 对应 b
// fmt.Printf("%#v\n", ret) // []string{"aaabbbbb", "aaa", "bbbbb"}
// ret = re.FindStringSubmatch("222")
// fmt.Printf("%#v\n", ret) // []string(nil)
func (re *Regexp) FindStringSubmatch(s string) []string {
	var dstCap [4]int
	a := re.doExecute(nil, nil, s, 0, re.prog.NumCap, dstCap[:0])
	if a == nil {
		return nil
	}
	ret := make([]string, 1+re.numSubexp)
	for i := range ret {
		if 2*i < len(a) && a[2*i] >= 0 {
			ret[i] = s[a[2*i]:a[2*i+1]]
		}
	}
	return ret
}

// FindStringSubmatchIndex returns a slice holding the index pairs
// identifying the leftmost match of the regular expression in s and the
// matches, if any, of its subexpressions, as defined by the 'Submatch' and
// 'Index' descriptions in the package comment.
// A return value of nil indicates no match.
//
// 假设返回结果是a,
// a[0:1]代表整个正则的匹配位移,
// a[2:3]代表第一个正则子表达式的匹配位移,
// a[4:5] 代表第二个正则子表达式的匹配位移
//
// re := regexp.MustCompile(`(a+)(b+)`)
// ret := re.FindStringSubmatchIndex("aaabbbbb")
// // ascii 中 0x61 对应 a, 0x62 对应 b
// fmt.Printf("%#v\n", ret) // []int{0, 8, 0, 3, 3, 8}
// ret = re.FindStringSubmatchIndex("222")
// fmt.Printf("%#v\n", ret) // []int(nil)
func (re *Regexp) FindStringSubmatchIndex(s string) []int {
	return re.pad(re.doExecute(nil, nil, s, 0, re.prog.NumCap, nil))
}

// FindReaderSubmatchIndex returns a slice holding the index pairs
// identifying the leftmost match of the regular expression of text read by
// the RuneReader, and the matches, if any, of its subexpressions, as defined
// by the 'Submatch' and 'Index' descriptions in the package comment. A
// return value of nil indicates no match.
//
// 与 func (re *Regexp) FindStringSubmatchIndex(s string) []int {
// 一样,只是输入参数类型不同
func (re *Regexp) FindReaderSubmatchIndex(r io.RuneReader) []int {
	return re.pad(re.doExecute(r, nil, "", 0, re.prog.NumCap, nil))
}

const startSize = 10 // The size at which to start a slice in the 'All' routines.

// FindAll is the 'All' version of Find; it returns a slice of all successive
// matches of the expression, as defined by the 'All' description in the
// package comment.
// A return value of nil indicates no match.
//
// 假设结果为a, a[0] 是整个正则的第1个匹配, a[1] 是整个正则的第2个匹配
// 例子
// re := regexp.MustCompile(`(a+)(b+)`)
// ret := re.FindAll([]byte("x"), 0)
// fmt.Printf("%s\n", ret) // []
// ret = re.FindAll([]byte("abaabaaabaaaab"), 0)
// fmt.Printf("%s\n", ret) // []
// ret = re.FindAll([]byte("abaabaaabaaaab"), 1)
// fmt.Printf("%s\n", ret) // [ab]
// ret = re.FindAll([]byte("abaabaaabaaaab"), 2)
// fmt.Printf("%s\n", ret) // [ab aab]
// ret = re.FindAll([]byte("abaabaaabaaaab"), 3)
// fmt.Printf("%s\n", ret) // [ab aab aaab]
// ret = re.FindAll([]byte("abaabaaabaaaab"), 4)
// fmt.Printf("%s\n", ret) // [ab aab aaab aaaab]
// ret = re.FindAll([]byte("abaabaaabaaaab"), 5)
// fmt.Printf("%s\n", ret) // [ab aab aaab aaaab]
func (re *Regexp) FindAll(b []byte, n int) [][]byte {
	if n < 0 {
		n = len(b) + 1
	}
	result := make([][]byte, 0, startSize)
	re.allMatches("", b, n, func(match []int) {
		result = append(result, b[match[0]:match[1]])
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

// FindAllIndex is the 'All' version of FindIndex; it returns a slice of all
// successive matches of the expression, as defined by the 'All' description
// in the package comment.
// A return value of nil indicates no match.
//
// 例子
// re := regexp.MustCompile(`(a+)(b+)`)
// ret := re.FindAllIndex([]byte("x"), 0)
// fmt.Printf("%#v\n", ret) // [][]int(nil)
// ret = re.FindAllIndex([]byte("abaabaaabaaaab"), 0)
// fmt.Printf("%#v\n", ret) // [][]int(nil)
// ret = re.FindAllIndex([]byte("abaabaaabaaaab"), 1)
// fmt.Printf("%#v\n", ret) // [][]int{[]int{0, 2}}
// ret = re.FindAllIndex([]byte("abaabaaabaaaab"), 2)
// fmt.Printf("%#v\n", ret) // [][]int{[]int{0, 2}, []int{2, 5}}
// ret = re.FindAllIndex([]byte("abaabaaabaaaab"), 3)
// fmt.Printf("%#v\n", ret) // [][]int{[]int{0, 2}, []int{2, 5}, []int{5, 9}}
// ret = re.FindAllIndex([]byte("abaabaaabaaaab"), 4)
// fmt.Printf("%#v\n", ret) // [][]int{[]int{0, 2}, []int{2, 5}, []int{5, 9}, []int{9, 14}}
// ret = re.FindAllIndex([]byte("abaabaaabaaaab"), 5)
// fmt.Printf("%#v\n", ret) // [][]int{[]int{0, 2}, []int{2, 5}, []int{5, 9}, []int{9, 14}}
func (re *Regexp) FindAllIndex(b []byte, n int) [][]int {
	if n < 0 {
		n = len(b) + 1
	}
	result := make([][]int, 0, startSize)
	re.allMatches("", b, n, func(match []int) {
		result = append(result, match[0:2])
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

// FindAllString is the 'All' version of FindString; it returns a slice of all
// successive matches of the expression, as defined by the 'All' description
// in the package comment.
// A return value of nil indicates no match.
//
// 例子
// re := regexp.MustCompile("a+")
// fmt.Println(re.FindAllString("abaabaaabaaaab", -1)) // [a aa aaa aaaa]
func (re *Regexp) FindAllString(s string, n int) []string {
	if n < 0 {
		n = len(s) + 1
	}
	result := make([]string, 0, startSize)
	re.allMatches(s, nil, n, func(match []int) {
		result = append(result, s[match[0]:match[1]])
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

// FindAllStringIndex is the 'All' version of FindStringIndex; it returns a
// slice of all successive matches of the expression, as defined by the 'All'
// description in the package comment.
// A return value of nil indicates no match.
//
// 例子
// re := regexp.MustCompile("a+")
// fmt.Println(re.FindAllStringIndex("abaabaaabaaaab", -1)) // [[0 1] [2 4] [5 8] [9 13]]
func (re *Regexp) FindAllStringIndex(s string, n int) [][]int {
	if n < 0 {
		n = len(s) + 1
	}
	result := make([][]int, 0, startSize)
	re.allMatches(s, nil, n, func(match []int) {
		result = append(result, match[0:2])
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

// FindAllSubmatch is the 'All' version of FindSubmatch; it returns a slice
// of all successive matches of the expression, as defined by the 'All'
// description in the package comment.
// A return value of nil indicates no match.
//
// 例子
// re := regexp.MustCompile("a+")
// fmt.Println(re.FindAllSubmatch([]byte("abaabaaabaaaab"), -1)) // [[[97]] [[97 97]] [[97 97 97]] [[97 97 97 97]]]
func (re *Regexp) FindAllSubmatch(b []byte, n int) [][][]byte {
	if n < 0 {
		n = len(b) + 1
	}
	result := make([][][]byte, 0, startSize)
	re.allMatches("", b, n, func(match []int) {
		slice := make([][]byte, len(match)/2)
		for j := range slice {
			if match[2*j] >= 0 {
				slice[j] = b[match[2*j]:match[2*j+1]]
			}
		}
		result = append(result, slice)
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

// FindAllSubmatchIndex is the 'All' version of FindSubmatchIndex; it returns
// a slice of all successive matches of the expression, as defined by the
// 'All' description in the package comment.
// A return value of nil indicates no match.
//
// 例子
// re := regexp.MustCompile("a+")
// fmt.Println(re.FindAllSubmatchIndex([]byte("abaabaaabaaaab"), -1)) // [[0 1] [2 4] [5 8] [9 13]]
func (re *Regexp) FindAllSubmatchIndex(b []byte, n int) [][]int {
	if n < 0 {
		n = len(b) + 1
	}
	result := make([][]int, 0, startSize)
	re.allMatches("", b, n, func(match []int) {
		result = append(result, match)
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

// FindAllStringSubmatch is the 'All' version of FindStringSubmatch; it
// returns a slice of all successive matches of the expression, as defined by
// the 'All' description in the package comment.
// A return value of nil indicates no match.
//
// 例子
// re := regexp.MustCompile("a+")
// fmt.Println(re.FindAllStringSubmatch("abaabaaabaaaab", -1)) // [[a] [aa] [aaa] [aaaa]]
func (re *Regexp) FindAllStringSubmatch(s string, n int) [][]string {
	if n < 0 {
		n = len(s) + 1
	}
	result := make([][]string, 0, startSize)
	re.allMatches(s, nil, n, func(match []int) {
		slice := make([]string, len(match)/2)
		for j := range slice {
			if match[2*j] >= 0 {
				slice[j] = s[match[2*j]:match[2*j+1]]
			}
		}
		result = append(result, slice)
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

// FindAllStringSubmatchIndex is the 'All' version of
// FindStringSubmatchIndex; it returns a slice of all successive matches of
// the expression, as defined by the 'All' description in the package
// comment.
// A return value of nil indicates no match.
//
// 例子
// re := regexp.MustCompile("a+")
// fmt.Println(re.FindAllStringSubmatchIndex("abaabaaabaaaab", -1)) // [[0 1] [2 4] [5 8] [9 13]]
func (re *Regexp) FindAllStringSubmatchIndex(s string, n int) [][]int {
	if n < 0 {
		n = len(s) + 1
	}
	result := make([][]int, 0, startSize)
	re.allMatches(s, nil, n, func(match []int) {
		result = append(result, match)
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

// Split slices s into substrings separated by the expression and returns a slice of
// the substrings between those expression matches.
//
// The slice returned by this method consists of all the substrings of s
// not contained in the slice returned by FindAllString. When called on an expression
// that contains no metacharacters, it is equivalent to strings.SplitN.
//
// Example:
//   s := regexp.MustCompile("a*").Split("abaabaccadaaae", 5)
//   // s: ["", "b", "b", "c", "cadaaae"]
//
// The count determines the number of substrings to return:
//   n > 0: at most n substrings; the last substring will be the unsplit remainder.
//   n == 0: the result is nil (zero substrings)
//   n < 0: all substrings
//
// 注意:当n==1:返回值len=1,并且ret[0]==s,参考:example.
func (re *Regexp) Split(s string, n int) []string {

	if n == 0 {
		return nil
	}

	if len(re.expr) > 0 && len(s) == 0 {
		return []string{""}
	}

	matches := re.FindAllStringIndex(s, n)
	strings := make([]string, 0, len(matches))

	beg := 0
	end := 0
	for _, match := range matches {
		if n > 0 && len(strings) >= n-1 {
			break
		}

		end = match[0]
		if match[1] != 0 {
			strings = append(strings, s[beg:end])
		}
		beg = match[1]
	}

	if end != len(s) {
		strings = append(strings, s[beg:])
	}

	return strings
}
