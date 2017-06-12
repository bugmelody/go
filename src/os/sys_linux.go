// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[3-over]]] 2017-6-12 17:31:46

// Linux-specific

package os

func hostname() (name string, err error) {
	f, err := Open("/proc/sys/kernel/hostname")
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf [512]byte // Enough for a DNS name.
	// buf是数组,这里必须通过buf[0:]转换为slice才能传给f.Read
	n, err := f.Read(buf[0:])
	if err != nil {
		return "", err
	}

	if n > 0 && buf[n-1] == '\n' {
		// 如果读取到了数据 && 数据的最后一个字节是"\n"
		// 去除结尾的\n
		n--
	}
	return string(buf[0:n]), nil
}
