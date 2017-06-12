// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[6-over]]] 2017-6-9 17:34:39

// General environment variables.

package os

import "syscall"

// Expand replaces ${var} or $var in the string based on the mapping function.
// For example, os.ExpandEnv(s) is equivalent to os.Expand(s, os.Getenv).
// $ go doc os.Getenv
// 本函数的使用参考: func TestExpand(t *testing.T)
func Expand(s string, mapping func(string) string) string {
	// 假设Expand后容量最多翻倍
	// 也可能更多,但那是依赖下方的append进行分配
	buf := make([]byte, 0, 2*len(s))
	// ${} is all ASCII, so bytes are fine for this operation.
	// $ { } 这三个字符全部是 ascii, 因此下面的操作是没有问题的
	// i代表最后循环到的'$'的下一个位置,初始时是0,每轮循环中将i设置为'$'的下一个位置
	i := 0
	// j是$的位移, j+1 < len(s) 代表: 在$不是s中最后一个字符的情况下进行循环
	for j := 0; j < len(s); j++ {
		if s[j] == '$' && j+1 < len(s) {
			// 将$之前的字符追加到buf
			buf = append(buf, s[i:j]...)
			name, w := getShellName(s[j+1:])
			// 应用 mapping 函数之后追加到 buf
			buf = append(buf, mapping(name)...)
			j += w
			// 将i设置为$的下一个位置
			i = j + 1
		}
	}
	// buf + 最后一个$之后的所有内容
	return string(buf) + s[i:]
}

// ExpandEnv replaces ${var} or $var in the string according to the values
// of the current environment variables. References to undefined
// variables are replaced by the empty string.
//
// 使用当前的环境变量进行expand
// 比如,定义了环境变量
// A=1
// B=2
// os.ExpandEnv("${A}${B}")=="12"
func ExpandEnv(s string) string {
	return Expand(s, Getenv)
}

// isShellSpecialVar reports whether the character identifies a special
// shell variable such as $*.
func isShellSpecialVar(c uint8) bool {
	// $ man bash 然后搜索 Special Parameters
	switch c {
	case '*', '#', '$', '@', '!', '?', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	}
	return false
}

// isAlphaNum reports whether the byte is an ASCII letter, number, or underscore
func isAlphaNum(c uint8) bool {
	// 可以将 uint8 与 'x' 进行比较, 因为底层类型都是整型
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// getShellName returns the name that begins the string and the number of bytes
// consumed to extract it. If the name is enclosed in {}, it's part of a ${}
// expansion and two more bytes are needed than the length of the name.
//
// 返回shell变量名和应该往后移动多少字节
// @see
func getShellName(s string) (string, int) {
	// 扫描 ${abc123_} 的情况
	switch {
	case s[0] == '{':
		// 如果第一个字节是'{'
		if len(s) > 2 && isShellSpecialVar(s[1]) && s[2] == '}' {
			// 比如s={x}abc
			// 0,1,2,总共是3个字节
			// 返回 "x",3
			return s[1:2], 3
		}
		// Scan to closing brace
		for i := 1; i < len(s); i++ {
			if s[i] == '}' {
				return s[1:i], i + 1
			}
		}
		return "", 1 // Bad syntax; just eat the brace.
	case isShellSpecialVar(s[0]):
		// 如果第一个字节是shell特殊字符
		return s[0:1], 1
	}
	// Scan alphanumerics.
	// 扫描 $abc123_ 的情况
	// Scan alphanumerics.
	var i int
	for i = 0; i < len(s) && isAlphaNum(s[i]); i++ {
	}
	return s[:i], i
}

// Getenv retrieves the value of the environment variable named by the key.
// It returns the value, which will be empty if the variable is not present.
// To distinguish between an empty value and an unset value, use LookupEnv.
//
// 对比一下本函数和os.LookupEnv的源码
func Getenv(key string) string {
	v, _ := syscall.Getenv(key)
	return v
}

// LookupEnv retrieves the value of the environment variable named
// by the key. If the variable is present in the environment the
// value (which may be empty) is returned and the boolean is true.
// Otherwise the returned value will be empty and the boolean will
// be false.
//
// 对比一下本函数和os.Getenv的源码
func LookupEnv(key string) (string, bool) {
	return syscall.Getenv(key)
}

// Setenv sets the value of the environment variable named by the key.
// It returns an error, if any.
func Setenv(key, value string) error {
	err := syscall.Setenv(key, value)
	if err != nil {
		return NewSyscallError("setenv", err)
	}
	return nil
}

// Unsetenv unsets a single environment variable.
func Unsetenv(key string) error {
	return syscall.Unsetenv(key)
}

// Clearenv deletes all environment variables.
//
// 删除所有环境变量
func Clearenv() {
	syscall.Clearenv()
}

// Environ returns a copy of strings representing the environment,
// in the form "key=value".
//
// windows输出例子:
// []string{
// ...
// "GOPATH=F:/qcpj/gopl",
// "GOROOT=D:/Go",
// "PATH=xyz"
// ...
// }
// fmt.Printf("%#v", os.Environ())
func Environ() []string {
	return syscall.Environ()
}
