// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[6-over]]] 2017-7-6 13:37:21

package textproto

// A MIMEHeader represents a MIME-style header mapping
// keys to sets of values.
type MIMEHeader map[string][]string

// Add adds the key, value pair to the header.
// It appends to any existing values associated with key.
func (h MIMEHeader) Add(key, value string) {
	key = CanonicalMIMEHeaderKey(key)
	// append之前,h[key]可能是nil,append可以作用在nil slice上
	h[key] = append(h[key], value)
}

// Set sets the header entries associated with key to
// the single element value. It replaces any existing
// values associated with key.
func (h MIMEHeader) Set(key, value string) {
	h[CanonicalMIMEHeaderKey(key)] = []string{value}
}

// Get gets the first value associated with the given key.
// It is case insensitive; CanonicalMIMEHeaderKey is used
// to canonicalize the provided key.
// If there are no values associated with the key, Get returns "".
// To access multiple values of a key, or to use non-canonical keys,
// access the map directly.
//
// Get方法内部会调用CanonicalMIMEHeaderKey,因此key大小写可以随意
func (h MIMEHeader) Get(key string) string {
	if h == nil {
		// 语法上,receiver可以是nil
		// 此时h是nil map,nil map是不能进行map查找操作的
		return ""
	}
	// 现在,h不为nil
	v := h[CanonicalMIMEHeaderKey(key)]
	// len作用在Slice,map的,返回: the number of elements in v;
	// if v is nil, len(v) is zero.
	if len(v) == 0 {
		return ""
	}
	// 现在,v这个slice至少有一个元素
	return v[0]
}

// Del deletes the values associated with key.
func (h MIMEHeader) Del(key string) {
	delete(h, CanonicalMIMEHeaderKey(key))
}
