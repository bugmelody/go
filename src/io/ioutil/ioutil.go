// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-20 22:11:12

// Package ioutil implements some I/O utility functions.
package ioutil

import (
	"bytes"
	"io"
	"os"
	"sort"
	"sync"
)

// readAll reads from r until an error or EOF and returns the data it read
// from the internal buffer allocated with a specified capacity.
//
// 上文中:internal buffer allocated(本函数内部分配的一个buffer)
// 参数 capacity: 代表了函数内部会分配的内部buffer的字节数
// b: 读取到的数据
func readAll(r io.Reader, capacity int64) (b []byte, err error) {
	// 分配internal buffer,容量为capacity
	buf := bytes.NewBuffer(make([]byte, 0, capacity))
	// If the buffer overflows, we will get bytes.ErrTooLarge.
	// Return that as an error. Any other panic remains.
	defer func() {
		e := recover()
		if e == nil {
			// 没有发生 panic
			return
		}
		// 现在,说明发生了 panic
		if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
			// 如果 panic 出来的属于内置的 error 接口,并且是 bytes.ErrTooLarge
			// 将此 error 返回, 终止 panic
			err = panicErr
		} else {
			// 否则, 是其他类型的, 继续panic
			panic(e)
		}
	}()
	// 将r中的内容全部读入buf
	// buf.ReadFrom可能会panic出一个ErrTooLarge的错误,上
	// 面的defer func中会处理这个panic,将其作为函数错误返回
	_, err = buf.ReadFrom(r)
	// 返回buf中的内容
	return buf.Bytes(), err
}

// ReadAll reads from r until an error or EOF and returns the data it read.
// A successful call returns err == nil, not err == EOF. Because ReadAll is
// defined to read from src until EOF, it does not treat an EOF from Read
// as an error to be reported.
func ReadAll(r io.Reader) ([]byte, error) {
	// ReadAll通过调用readAll,指定capacity为bytes.MinRead,也就是512字节
	// 如果读取超过了512字节会发生什么?????
	// readAll 中的第二个参数: capacity: 代表了函数内部会分配的buffer的字节数
	// 这是buffer的初始容量,如果实际不够的话,bytes.Buffer.ReadFrom内部会增加buffer容量
	return readAll(r, bytes.MinRead)
}

// ReadFile reads the file named by filename and returns the contents.
// A successful call returns err == nil, not err == EOF. Because ReadFile
// reads the whole file, it does not treat an EOF from Read as an error
// to be reported.
func ReadFile(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// It's a good but not certain bet that FileInfo will tell us exactly how much to
	// read, so let's try it but be prepared for the answer to be wrong.
	// 需要读取多少字节
	var n int64

	if fi, err := f.Stat(); err == nil {
		// Don't preallocate a huge buffer, just in case.
		// just in case: 以防万一
		// fi.Size() 返回的是以字节为单位
		/**
		E后的数表示10的多少次方,用指数表示法显示数字,以 E+n 替换部分数字,
		其中E(代表指数)表示将前面的数字乘以10的n次幂.
		例如,用2位小数的"科学记数"格式表示12345678901,结果为 1.23E+10,即1.23乘以10的10次幂.
		您可以指定要使用的小数位数.
		这里: 1e9 = 1 后面带9个0 = 1 000 000 000 ≈ 1G
		 */
		if size := fi.Size(); size < 1e9 {
			// 如果 fi.Size() 统计为 1G 内
			n = size
		}
	}
	// As initial capacity for readAll, use n + a little extra in case Size is zero,
	// and to avoid another allocation after Read has filled the buffer. The readAll
	// call will read into its allocated internal buffer cheaply. If the size was
	// wrong, we'll either waste some space off the end or reallocate as needed, but
	// in the overwhelmingly common case we'll get it just right.
	return readAll(f, n+bytes.MinRead)
}

// WriteFile writes data to a file named by filename.
// If the file does not exist, WriteFile creates it with permissions perm;
// otherwise WriteFile truncates it before writing.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	// err变量是在最开始一行声明的,这里err只是赋值,n是重新声明
	// f.Write(data)要求data写完
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	// 如果之前没有错误,设置 err= f.Close()返回的错误
	// 如果之前有错误,使用之前的错误
	// 这是想优先使用之前的错误,因为之前的错误更能方便诊断错误信息
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

// ReadDir reads the directory named by dirname and returns
// a list of directory entries sorted by filename.
//
// 由于函数不限制File.Readdir的返回数量,因此如果目录下的文件多了会出问题
func ReadDir(dirname string) ([]os.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	// -1 表示不限制返回数量; f.Readdir 返回 ([]FileInfo, error)
	// 由于这里不限制,因此如果目录下的文件多了会出问题
	list, err := f.Readdir(-1)
	// f不会再被用到了,尽早Close
	f.Close()
	if err != nil {
		return nil, err
	}
	// 这里展示了如何对slice进行排序,go1.8以前必须要定义自定义类型并实现sort.Interface接口
	// 1.8之后对slice进行排序只需要调用sort.Slice
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list, nil
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

// NopCloser returns a ReadCloser with a no-op Close method wrapping
// the provided Reader r.
func NopCloser(r io.Reader) io.ReadCloser {
	return nopCloser{r}
}

type devNull int

// devNull implements ReaderFrom as an optimization so io.Copy to
// ioutil.Discard can avoid doing unnecessary work.
//
// 编译期检查devNull实现了io.ReaderFrom接口
var _ io.ReaderFrom = devNull(0)

func (devNull) Write(p []byte) (int, error) {
	return len(p), nil
}

func (devNull) WriteString(s string) (int, error) {
	return len(s), nil
}

var blackHolePool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 8192)
		return &b
	},
}

// 对devNull实现io.ReaderFrom接口
func (devNull) ReadFrom(r io.Reader) (n int64, err error) {
	// 从 Pool 中拿一块 buf 出来用
	bufp := blackHolePool.Get().(*[]byte)
	// readSize 代表下面 for 每次循环时 r.Read 读取的字节数
	readSize := 0
	// 循环,直到EOF或者出错
	for {
		// 将r中的数据读入bufp,读入了之后,下次循环会覆盖bufp的数据
		readSize, err = r.Read(*bufp)
		// 增加总的读取字节数
		n += int64(readSize)
		if err != nil {
			// r.Read出错,bufp归还给Poll
			blackHolePool.Put(bufp)
			if err == io.EOF {
				// 如果 r 已经读取完毕,说明是正常完毕
				return n, nil
			}
			return
		}
	}
}

// Discard is an io.Writer on which all Write calls succeed
// without doing anything.
//
// 全部会写入成功,写入到黑洞
// 上方对devNull实现了Write,WriteString,ReadFrom方法,都是写入后丢弃的操作
//
// Discard是devNull类型的一个具体变量
var Discard io.Writer = devNull(0)
