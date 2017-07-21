// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[3-over]]] 2017-7-20 21:50:36

package http

import (
	"io"
	"net/textproto"
	"sort"
	"strings"
	"sync"
	"time"
)

var raceEnabled = false // set by race.go

// A Header represents the key-value pairs in an HTTP header.
type Header map[string][]string

// Add adds the key, value pair to the header.
// It appends to any existing values associated with key.
//
// 内部的操作会有CanonicalMIMEHeaderKey(key)处理
func (h Header) Add(key, value string) {
	// textproto.MIMEHeader(h)是类型转换,将h转换为textproto.MIMEHeader
	// textproto.MIMEHeader和本文件中的Header底层类型都是 map[string][]string
	// 对此类型的实现只在textproto.MIMEHeader
	// 本文件中都是将Header转型为textproto.MIMEHeader,然后去使用textproto.MIMEHeader中的对应方法实现
	textproto.MIMEHeader(h).Add(key, value)
}

// Set sets the header entries associated with key to
// the single element value. It replaces any existing
// values associated with key.
//
// 内部的操作会有CanonicalMIMEHeaderKey(key)处理
func (h Header) Set(key, value string) {
	textproto.MIMEHeader(h).Set(key, value)
}

// Get gets the first value associated with the given key.
// It is case insensitive; textproto.CanonicalMIMEHeaderKey is used
// to canonicalize the provided key.
// If there are no values associated with the key, Get returns "".
// To access multiple values of a key, or to use non-canonical keys,
// access the map directly.
//
// 内部的操作会有CanonicalMIMEHeaderKey(key)处理
func (h Header) Get(key string) string {
	return textproto.MIMEHeader(h).Get(key)
}

// get is like Get, but key must already be in CanonicalHeaderKey form.
func (h Header) get(key string) string {
	if v := h[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// Del deletes the values associated with key.
// 内部的操作会有CanonicalMIMEHeaderKey(key)处理
func (h Header) Del(key string) {
	textproto.MIMEHeader(h).Del(key)
}

// Write writes a header in wire format.
func (h Header) Write(w io.Writer) error {
	return h.WriteSubset(w, nil)
}

func (h Header) clone() Header {
	// h2代表克隆之后的Header
	h2 := make(Header, len(h))
	// 循环h,赋值到h2
	for k, vv := range h {
		// vv的类型是[]string
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

var timeFormats = []string{
	TimeFormat,
	time.RFC850,
	time.ANSIC,
}

// ParseTime parses a time header (such as the Date: header),
// trying each of the three formats allowed by HTTP/1.1:
// TimeFormat, time.RFC850, and time.ANSIC.
func ParseTime(text string) (t time.Time, err error) {
	for _, layout := range timeFormats {
		t, err = time.Parse(layout, text)
		if err == nil {
			return
		}
	}
	// 到这里,说明三种timeFormats都没能成功解析时
	// 间,此时t和err是循环中最后一次的time.Parse结果
	return
}

// \n 替换为 空格, \r 替换为 空格. ??? 为什么要替换为空格 ???
var headerNewlineToSpace = strings.NewReplacer("\n", " ", "\r", " ")

type writeStringer interface {
	WriteString(string) (int, error)
}

// stringWriter implements WriteString on a Writer.
type stringWriter struct {
	w io.Writer
}

func (w stringWriter) WriteString(s string) (n int, err error) {
	return w.w.Write([]byte(s))
}

type keyValues struct {
	key    string
	values []string
}

// A headerSorter implements sort.Interface by sorting a []keyValues
// by key. It's used as a pointer, so it can fit in a sort.Interface
// interface value without allocation.
// 上面说: It's used as a pointer, so it can fit in a sort.Interface interface value without allocation. 是什么意思 ??
type headerSorter struct {
	kvs []keyValues
}

func (s *headerSorter) Len() int           { return len(s.kvs) }
func (s *headerSorter) Swap(i, j int)      { s.kvs[i], s.kvs[j] = s.kvs[j], s.kvs[i] }
func (s *headerSorter) Less(i, j int) bool { return s.kvs[i].key < s.kvs[j].key }

var headerSorterPool = sync.Pool{
	New: func() interface{} { return new(headerSorter) },
}

// sortedKeyValues returns h's keys sorted in the returned kvs
// slice. The headerSorter used to sort is also returned, for possible
// return to headerSorterCache.
func (h Header) sortedKeyValues(exclude map[string]bool) (kvs []keyValues, hs *headerSorter) {
	// sync.Pool.Get返回类型为interface{},需要使用type assertion转换为*headerSorter
	// 注意,这里得到的hs是可能会在多个headerSorterPool.Get(),headerSorterPool.Put()之间重复利用的
	hs = headerSorterPool.Get().(*headerSorter)
	if cap(hs.kvs) < len(h) {
		// 从pool中拿出来的可能容量不够,需要保证hs.kvs的空间足够
		hs.kvs = make([]keyValues, 0, len(h))
	}
	// kvs是函数命名返回值
	// 下面这条语句让kvs指向hs.kvs的底层数组的起始处
	kvs = hs.kvs[:0]
	for k, vv := range h {
		if !exclude[k] {
			// 如果没有被exclude,才进行append
			kvs = append(kvs, keyValues{k, vv})
		}
	}
	// 经过了上面循环append,kvs可能已经指向了一个新分配的底层数组(和hs.kvs不同了), 因此这里需要设置回来
	hs.kvs = kvs
	sort.Sort(hs)
	// 现在,hs已经排好序(实际是hs.kvs已经排好序)
	// 注意,kvs和hs.kvs 是指向的同一个底层数组
	return kvs, hs
}

// WriteSubset writes a header in wire format.
// If exclude is not nil, keys where exclude[key] == true are not written.
//
// WriteSubset内部通过调用h.sortedKeyValues(其实现通过headerSorterPool来避免每个goroutine一个对象),减少了内存占用
//
// 如果w实现了WriteString(string) (int, error) 方法,会使用w自带的WriteString
// 否则,会使用 stringWriter{w} 构造一个具有 WriteString 方法的对象(此情况下效率比较低,因为内部是通过字符串转换为字节的方式 )
//
// 什么是 wire format ? 个人感觉 wire format 应该是指网络传输时候的格式.
func (h Header) WriteSubset(w io.Writer, exclude map[string]bool) error {
	ws, ok := w.(writeStringer)
	if !ok {
		ws = stringWriter{w}
	}
	// 现在,肯定可以在ws上面调用WriteString(string) (int, error)方法
	kvs, sorter := h.sortedKeyValues(exclude)
	for _, kv := range kvs {
		for _, v := range kv.values {
			v = headerNewlineToSpace.Replace(v)
			v = textproto.TrimString(v)
			for _, s := range []string{kv.key, ": ", v, "\r\n"} {
				if _, err := ws.WriteString(s); err != nil {
					return err
				}
			}
		}
	}
	// 归还给sync.Pool
	headerSorterPool.Put(sorter)
	return nil
}

// CanonicalHeaderKey returns the canonical format of the
// header key s. The canonicalization converts the first
// letter and any letter following a hyphen to upper case;
// the rest are converted to lowercase. For example, the
// canonical key for "accept-encoding" is "Accept-Encoding".
// If s contains a space or invalid header field bytes, it is
// returned without modifications.
func CanonicalHeaderKey(s string) string { return textproto.CanonicalMIMEHeaderKey(s) }

// hasToken reports whether token appears with v, ASCII
// case-insensitive, with space or comma boundaries.
// token must be all lowercase.
// v may contain mixed cased.
func hasToken(v, token string) bool {
	if len(token) > len(v) || token == "" {
		return false
	}
	if v == token {
		return true
	}
	for sp := 0; sp <= len(v)-len(token); sp++ {
		// Check that first character is good.
		// The token is ASCII, so checking only a single byte
		// is sufficient. We skip this potential starting
		// position if both the first byte and its potential
		// ASCII uppercase equivalent (b|0x20) don't match.
		// False positives ('^' => '~') are caught by EqualFold.
		if b := v[sp]; b != token[0] && b|0x20 != token[0] {
			continue
		}
		// Check that start pos is on a valid token boundary.
		if sp > 0 && !isTokenBoundary(v[sp-1]) {
			continue
		}
		// Check that end pos is on a valid token boundary.
		if endPos := sp + len(token); endPos != len(v) && !isTokenBoundary(v[endPos]) {
			continue
		}
		if strings.EqualFold(v[sp:sp+len(token)], token) {
			return true
		}
	}
	return false
}

func isTokenBoundary(b byte) bool {
	return b == ' ' || b == ',' || b == '\t'
}

// @see
func cloneHeader(h Header) Header {
	h2 := make(Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}
