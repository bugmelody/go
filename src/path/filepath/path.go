// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-14 13:16:47 本包只看导出的

// Package filepath implements utility routines for manipulating filename paths
// in a way compatible with the target operating system-defined file paths.
//
// The filepath package uses either forward slashes or backslashes,
// depending on the operating system. To process paths such as URLs
// that always use forward slashes regardless of the operating
// system, see the path package.
package filepath

import (
	"errors"
	"os"
	"sort"
	"strings"
)

// A lazybuf is a lazily constructed path buffer.
// It supports append, reading previously appended bytes,
// and retrieving the final string. It does not allocate a buffer
// to hold the output until that output diverges from s.
type lazybuf struct {
	path       string
	buf        []byte
	w          int
	volAndPath string
	volLen     int
}

func (b *lazybuf) index(i int) byte {
	if b.buf != nil {
		return b.buf[i]
	}
	return b.path[i]
}

func (b *lazybuf) append(c byte) {
	if b.buf == nil {
		if b.w < len(b.path) && b.path[b.w] == c {
			b.w++
			return
		}
		b.buf = make([]byte, len(b.path))
		copy(b.buf, b.path[:b.w])
	}
	b.buf[b.w] = c
	b.w++
}

func (b *lazybuf) string() string {
	if b.buf == nil {
		return b.volAndPath[:b.volLen+b.w]
	}
	return b.volAndPath[:b.volLen] + string(b.buf[:b.w])
}

const (
	Separator     = os.PathSeparator
	ListSeparator = os.PathListSeparator
)

// Clean returns the shortest path name equivalent to path
// by purely lexical processing. It applies the following rules
// iteratively until no further processing can be done:
//
//	1. Replace multiple Separator elements with a single one.
//	2. Eliminate each . path name element (the current directory).
//	3. Eliminate each inner .. path name element (the parent directory)
//	   along with the non-.. element that precedes it.
//	4. Eliminate .. elements that begin a rooted path:
//	   that is, replace "/.." by "/" at the beginning of a path,
//	   assuming Separator is '/'.
//
// The returned path ends in a slash only if it represents a root directory,
// such as "/" on Unix or `C:\` on Windows.
//
// Finally, any occurrences of slash are replaced by Separator.
//
// If the result of this process is an empty string, Clean
// returns the string ".".
//
// See also Rob Pike, ``Lexical File Names in Plan 9 or
// Getting Dot-Dot Right,''
// https://9p.io/sys/doc/lexnames.html
//
//
// 相比 path.Clean, filepath.Clean 多了下面两个功能
// 1. The returned path ends in a slash only if it represents a root directory,
// such as "/" on Unix or `C:\` on Windows.
// 2. Finally, any occurrences of slash are replaced by Separator.
//
// 本函数不看细节
func Clean(path string) string {
	originalPath := path
	volLen := volumeNameLen(path)
	path = path[volLen:]
	if path == "" {
		if volLen > 1 && originalPath[1] != ':' {
			// should be UNC
			return FromSlash(originalPath)
		}
		return originalPath + "."
	}
	rooted := os.IsPathSeparator(path[0])

	// Invariants:
	//	reading from path; r is index of next byte to process.
	//	writing to buf; w is index of next byte to write.
	//	dotdot is index in buf where .. must stop, either because
	//		it is the leading slash or it is a leading ../../.. prefix.
	n := len(path)
	out := lazybuf{path: path, volAndPath: originalPath, volLen: volLen}
	r, dotdot := 0, 0
	if rooted {
		out.append(Separator)
		r, dotdot = 1, 1
	}

	for r < n {
		switch {
		case os.IsPathSeparator(path[r]):
			// empty path element
			r++
		case path[r] == '.' && (r+1 == n || os.IsPathSeparator(path[r+1])):
			// . element
			r++
		case path[r] == '.' && path[r+1] == '.' && (r+2 == n || os.IsPathSeparator(path[r+2])):
			// .. element: remove to last separator
			r += 2
			switch {
			case out.w > dotdot:
				// can backtrack
				out.w--
				for out.w > dotdot && !os.IsPathSeparator(out.index(out.w)) {
					out.w--
				}
			case !rooted:
				// cannot backtrack, but not rooted, so append .. element.
				if out.w > 0 {
					out.append(Separator)
				}
				out.append('.')
				out.append('.')
				dotdot = out.w
			}
		default:
			// real path element.
			// add slash if needed
			if rooted && out.w != 1 || !rooted && out.w != 0 {
				out.append(Separator)
			}
			// copy element
			for ; r < n && !os.IsPathSeparator(path[r]); r++ {
				out.append(path[r])
			}
		}
	}

	// Turn empty string into "."
	if out.w == 0 {
		out.append('.')
	}

	return FromSlash(out.string())
}

// ToSlash returns the result of replacing each separator character
// in path with a slash ('/') character. Multiple separators are
// replaced by multiple slashes.
//
//
// ToSlash 应该叫 from Separator to Slash
// ToSlash 将 path 中所有 separator 替换为 '/', 多个 separators 被替换为多个 '/'.
// separator 是独立于操作系统
//   windows 是 \ , 见: src\os\path_windows.go
//   linux 是 / , 见: src\os\path_unix.go
// @see
func ToSlash(path string) string {
	if Separator == '/' {
		// 当前系统操作符已经是slash;没有必要进行转换,返回
		return path
	}
	// 现在,需要将Separator替换为slash
	return strings.Replace(path, string(Separator), "/", -1)
}

// FromSlash returns the result of replacing each slash ('/') character
// in path with a separator character. Multiple slashes are replaced
// by multiple separators.
//
//
// FromSlash 应该叫 from Slash to Separator
// 如果当前系统分隔符是'/',path原样返回
// 否则,将path中的'/'替换为当前系统分隔符后返回.
func FromSlash(path string) string {
	if Separator == '/' {
		// 当前系统操作符已经是slash
		// 没有必要进行转换,返回
		return path
	}
	return strings.Replace(path, "/", string(Separator), -1)
}

// SplitList splits a list of paths joined by the OS-specific ListSeparator,
// usually found in PATH or GOPATH environment variables.
// Unlike strings.Split, SplitList returns an empty slice when passed an empty
// string.
//
// 对比下 $ go doc strings.Split
//
// PathListSeparator = ';'  // windows path list separator
// PathListSeparator = ':' // unix path list separator
//
// 例子: filepath.SplitList("/a/b/c:/usr/bin") = [/a/b/c /usr/bin]
func SplitList(path string) []string {
	return splitList(path)
}

// Split splits path immediately following the final Separator,
// separating it into a directory and file name component.
// If there is no Separator in path, Split returns an empty dir
// and file set to path.
// The returned values have the property that path = dir+file.
//
// @see
func Split(path string) (dir, file string) {
	vol := VolumeName(path)
	i := len(path) - 1
	for i >= len(vol) && !os.IsPathSeparator(path[i]) {
		i--
	}
	return path[:i+1], path[i+1:]
}

// Join joins any number of path elements into a single path, adding
// a Separator if necessary. Join calls Clean on the result; in particular,
// all empty strings are ignored.
// On Windows, the result is a UNC path if and only if the first path
// element is a UNC path.
//
//
// 什么是 UNC path: 
// http://baike.baidu.com/link?url=DOT-IdRopiwt3OFOzT2V0HBKRq4fCl0BCUb4vhsMkFkEjmiK02zu7p1s5PlwMgKl04iShNbF0zqSL8bBwzEcE1U8PQeTjMjWl4yOK4kVnO3
func Join(elem ...string) string {
	return join(elem)
}

// Ext returns the file name extension used by path.
// The extension is the suffix beginning at the final dot
// in the final element of path; it is empty if there is
// no dot.
//
//
// 获取 path 中文件名部分的扩展名
// 返回的字符串中以'.'开头,比如:'.jpg','.txt'
// @see
func Ext(path string) string {
	for i := len(path) - 1; i >= 0 && !os.IsPathSeparator(path[i]); i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	// 根据文档: it is empty if there is no dot.
	return ""
}

// EvalSymlinks returns the path name after the evaluation of any symbolic
// links.
// If path is relative the result will be relative to the current directory,
// unless one of the components is an absolute symbolic link.
// EvalSymlinks calls Clean on the result.
func EvalSymlinks(path string) (string, error) {
	return evalSymlinks(path)
}

// Abs returns an absolute representation of path.
// If the path is not absolute it will be joined with the current
// working directory to turn it into an absolute path. The absolute
// path name for a given file is not guaranteed to be unique.
// Abs calls Clean on the result.
func Abs(path string) (string, error) {
	return abs(path)
}

func unixAbs(path string) (string, error) {
	if IsAbs(path) {
		return Clean(path), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return Join(wd, path), nil
}

// Rel returns a relative path that is lexically equivalent to targpath when
// joined to basepath with an intervening separator. That is,
// Join(basepath, Rel(basepath, targpath)) is equivalent to targpath itself.
// On success, the returned path will always be relative to basepath,
// even if basepath and targpath share no elements.
// An error is returned if targpath can't be made relative to basepath or if
// knowing the current working directory would be necessary to compute it.
// Rel calls Clean on the result.
//
// 看看 func ExampleRel() {
// 不看源码
func Rel(basepath, targpath string) (string, error) {
	baseVol := VolumeName(basepath)
	targVol := VolumeName(targpath)
	base := Clean(basepath)
	targ := Clean(targpath)
	if sameWord(targ, base) {
		return ".", nil
	}
	base = base[len(baseVol):]
	targ = targ[len(targVol):]
	if base == "." {
		base = ""
	}
	// Can't use IsAbs - `\a` and `a` are both relative in Windows.
	baseSlashed := len(base) > 0 && base[0] == Separator
	targSlashed := len(targ) > 0 && targ[0] == Separator
	if baseSlashed != targSlashed || !sameWord(baseVol, targVol) {
		return "", errors.New("Rel: can't make " + targpath + " relative to " + basepath)
	}
	// Position base[b0:bi] and targ[t0:ti] at the first differing elements.
	bl := len(base)
	tl := len(targ)
	var b0, bi, t0, ti int
	for {
		for bi < bl && base[bi] != Separator {
			bi++
		}
		for ti < tl && targ[ti] != Separator {
			ti++
		}
		if !sameWord(targ[t0:ti], base[b0:bi]) {
			break
		}
		if bi < bl {
			bi++
		}
		if ti < tl {
			ti++
		}
		b0 = bi
		t0 = ti
	}
	if base[b0:bi] == ".." {
		return "", errors.New("Rel: can't make " + targpath + " relative to " + basepath)
	}
	if b0 != bl {
		// Base elements left. Must go up before going down.
		seps := strings.Count(base[b0:bl], string(Separator))
		size := 2 + seps*3
		if tl != t0 {
			size += 1 + tl - t0
		}
		buf := make([]byte, size)
		n := copy(buf, "..")
		for i := 0; i < seps; i++ {
			buf[n] = Separator
			copy(buf[n+1:], "..")
			n += 3
		}
		if t0 != tl {
			buf[n] = Separator
			copy(buf[n+1:], targ[t0:])
		}
		return string(buf), nil
	}
	return targ[t0:], nil
}

// SkipDir is used as a return value from WalkFuncs to indicate that
// the directory named in the call is to be skipped. It is not returned
// as an error by any function.
//
// that the directory named(WalkFunc的path参数)
// in the call(指WalkFunc的调用)
var SkipDir = errors.New("skip this directory")

// WalkFunc is the type of the function called for each file or directory
// visited by Walk. The path argument contains the argument to Walk as a
// prefix; that is, if Walk is called with "dir", which is a directory
// containing the file "a", the walk function will be called with argument
// "dir/a". The info argument is the os.FileInfo for the named path.
//
// If there was a problem walking to the file or directory named by path, the
// incoming error will describe the problem and the function can decide how
// to handle that error (and Walk will not descend into that directory). If
// an error is returned, processing stops. The sole exception is when the function
// returns the special value SkipDir. If the function returns SkipDir when invoked
// on a directory, Walk skips the directory's contents entirely.
// If the function returns SkipDir when invoked on a non-directory file,
// Walk skips the remaining files in the containing directory.
//
// incoming error: 指WalkFunc的err参数
// the function(指WalkFunc) can decide how to handle that error(指err参数)
// If an error is returned(WalkFunc返回一个error), processing stops(整个walk停止)
//
// 总结:
// WalkFunc的函数体内部,处理的是walk过程中遇到的一个文件path.
// err是walk到path的错误,WalkFunc有权根据err决定如何处理这个错误.
// 如果WalkFunc返回err=SkipDir,表示会忽略当前path.
// 如果WalkFunc返回err=非SkipDir error,整个walk结束.
// 如果WalkFunc返回error=nil,表示没有遇到错误.
//
// 以下是粗略的翻译
// Walk函数对每一个文件/目录都会调用WalkFunc函数类型值.
// 调用时path参数会包含Walk的root参数作为前缀;就是说,如果Walk函数的root为"dir",该目录下有文件"a",
// 将会使用"dir/a"作为path参数调用walkFn参数.
// walkFn参数被调用时的info参数是path指定的地址(文件/目录)的文件信息,类型为os.FileInfo.
// 如果遍历path指定的文件或目录时出现了问题,传入的参数err会描述该问题,WalkFunc类型函数可以决定
// 如何去处理该错误(Walk函数将不会深入该目录);如果该函数返回一个错误,Walk函数的执行会中止;只有一个
// 例外,如果Walk的walkFn返回值是SkipDir,将会跳过该目录的内容而Walk函数照常执行处理下一个文件.
type WalkFunc func(path string, info os.FileInfo, err error) error

var lstat = os.Lstat // for testing

// walk recursively descends path, calling w.
//
// @see
func walk(path string, info os.FileInfo, walkFn WalkFunc) error {
	err := walkFn(path, info, nil)
	if err != nil {
		// walkFn返回错误
		if info.IsDir() && err == SkipDir {
			// SkipDir表示忽略当前错误
			return nil
		}
		// 其他错误会终止整个Walk
		return err
	}
	// 现在,walkFn成功

	if !info.IsDir() {
		// 不是目录,说明是文件;当前函数成功返回
		return nil
	}
	// 现在,path是目录

	// 读取path下的子目录列表
	names, err := readDirNames(path)
	if err != nil {
		// 无法读取path下的子目录列表
		// 那就不管path下的子目录,只管path
		return walkFn(path, info, err)
	}
	// 现在,成功读取到了path下的子目录列表

	// 循环每一个子目录
	for _, name := range names {
		// 该文件的完整路径
		filename := Join(path, name)
		// 不跟踪符号链接
		fileInfo, err := lstat(filename)
		if err != nil {
			// os.Lstat出错,通知walkFn
			if err := walkFn(filename, fileInfo, err); err != nil && err != SkipDir {
				// walkFn失败并且不是SkipDir造成的失败
				return err
			}
			// 如果walkFn失败并且是SkipDir造成的失败,则会继续下轮循环
		} else {
			// os.Lstat成功
			// 递归调用walk
			err = walk(filename, fileInfo, walkFn)
			if err != nil {
				// walk出错
				if !fileInfo.IsDir() || err != SkipDir {
					return err
				}
			}
		}
	}
	return nil
}

// Walk walks the file tree rooted at root, calling walkFn for each file or
// directory in the tree, including root. All errors that arise visiting files
// and directories are filtered by walkFn. The files are walked in lexical
// order, which makes the output deterministic but means that for very
// large directories Walk can be inefficient.
// Walk does not follow symbolic links.
//
// lexical order: 词典式序列
// @see
func Walk(root string, walkFn WalkFunc) error {
	// 文档: does not follow symbolic links
	// os.Stat和os.LStat的对比
	// Stat 会跟踪符号链接,最后返回的是真正的 FileInfo
	// LStat 不会跟踪符号链接,最后返回的是符号链接的 FileInfo
	info, err := os.Lstat(root)
	if err != nil {
		// os.Lstat 出错
		err = walkFn(root, nil, err)
	} else {
		err = walk(root, info, walkFn)
	}
	if err == SkipDir {
		return nil
	}
	return err
}

// readDirNames reads the directory named by dirname and returns
// a sorted list of directory entries.
//
// @see
func readDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// Base returns the last element of path.
// Trailing path separators are removed before extracting the last element.
// If the path is empty, Base returns ".".
// If the path consists entirely of separators, Base returns a single separator.
//
// @see
func Base(path string) string {
	if path == "" {
		// 文档:If the path is empty, Base returns ".".
		return "."
	}
	// Strip trailing slashes.
	for len(path) > 0 && os.IsPathSeparator(path[len(path)-1]) {
		// 文档: Trailing path separators are removed before extracting the last element.
		path = path[0 : len(path)-1]
	}
	// Throw away volume name
	path = path[len(VolumeName(path)):]
	// Find the last element
	i := len(path) - 1
	for i >= 0 && !os.IsPathSeparator(path[i]) {
		// i-- 一直到遇到path的最后一个分隔符
		i--
	}
	if i >= 0 {
		// 设置path为最后一个分隔符之后的内容
		path = path[i+1:]
	}
	// If empty now, it had only slashes.
	if path == "" {
		// 文档: If the path consists entirely of separators, Base returns a single separator.
		return string(Separator)
	}
	return path
}

// Dir returns all but the last element of path, typically the path's directory.
// After dropping the final element, Dir calls Clean on the path and trailing
// slashes are removed.
// If the path is empty, Dir returns ".".
// If the path consists entirely of separators, Dir returns a single separator.
// The returned path does not end in a separator unless it is the root directory.
//
// 返回的内容如'a/b',一般情况下结尾的'/'已经被去掉,除非path本身就是'/'
// 不看细节了
func Dir(path string) string {
	vol := VolumeName(path)
	i := len(path) - 1
	for i >= len(vol) && !os.IsPathSeparator(path[i]) {
		i--
	}
	dir := Clean(path[len(vol) : i+1])
	if dir == "." && len(vol) > 2 {
		// must be UNC
		return vol
	}
	return vol + dir
}

// VolumeName returns leading volume name.
// Given "C:\foo\bar" it returns "C:" on Windows.
// Given "\\host\share\foo" it returns "\\host\share".
// On other platforms it returns "".
func VolumeName(path string) string {
	return path[:volumeNameLen(path)]
}
