// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[5-over]]] 2017-6-20 11:01:35

package io

type eofReader struct{}

func (eofReader) Read([]byte) (int, error) {
	return 0, EOF
}

type multiReader struct {
	// 一个 multiReader 是由多个 Reader 组成的
	readers []Reader
}


func (mr *multiReader) Read(p []byte) (n int, err error) {
	// 循环读取每一个reader,每读完一个进行一次reslice
	for len(mr.readers) > 0 {
		// Optimization to flatten nested multiReaders (Issue 13558).
		if len(mr.readers) == 1 {
			// 只有在len(mr.readers) == 1的条件下才能使用flatten这种优化,因为需要flatten需要修改mr.readers
			// flatten后可以避免函数调用
			if r, ok := mr.readers[0].(*multiReader); ok {
				mr.readers = r.readers
				continue
			}
		}
		// 实际的读取操作
		n, err = mr.readers[0].Read(p)
		if err == EOF {
			// Use eofReader instead of nil to avoid nil panic
			// after performing flatten (Issue 18232).
			mr.readers[0] = eofReader{} // permit earlier GC
			mr.readers = mr.readers[1:]
		}
		if n > 0 || err != EOF {
			if err == EOF && len(mr.readers) > 0 {
				// Don't return EOF yet. More readers remain.
				// 还有数据需要读取
				err = nil
			}
			return
		}
	}
	// 只有这里才会返回EOF // Once all inputs have returned EOF, Read will return EOF.
	return 0, EOF
}

// MultiReader returns a Reader that's the logical concatenation of
// the provided input readers. They're read sequentially. Once all
// inputs have returned EOF, Read will return EOF.  If any of the readers
// return a non-nil, non-EOF error, Read will return that error.
//
// 注意,MultiReader.Read因为使用了多个reader,因此内部会用for循环读取.
// 当每个单独的reader读取没有出错的时候,或者每个reader读完后,并不会返回EOF,而是返回nil
// 直到for循环完所有reader后(mr.readers长度为0),再读取时才会返回EOF
//
// 注意,在Read的时候如果读取为空(当前reader没有数据),并不会return,而是会进行下个循环进行读取下个reader
func MultiReader(readers ...Reader) Reader {
	// ????????????为什么不能直接将readers赋值给目标字段,而必须经过copy
	// 参考测试 func TestMultiReaderCopy, func TestMultiWriterCopy
	r := make([]Reader, len(readers))
	// 变参在函数内部实际是slice
	copy(r, readers)
	return &multiReader{r}
}

type multiWriter struct {
	writers []Writer
}

// multiWriter是指将数据p同时写入多份Writer.
// io.Writer要求一次性写完
func (t *multiWriter) Write(p []byte) (n int, err error) {
	// 循环每一个writer w,将p写入w
	for _, w := range t.writers {
		// io.Writer要求一次性写完
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			// 如果写入的字节数!=p的长度,说明w的空间太小
			err = ErrShortWrite
			return
		}
	}
	// 全部写入成功,返回写入的字节数是p的长度
	return len(p), nil
}

// 编译期检查,确保 *multiWriter 实现了 stringWriter 接口
var _ stringWriter = (*multiWriter)(nil)

// 同时将string s的值写入多份Writer
func (t *multiWriter) WriteString(s string) (n int, err error) {
	var p []byte // lazily initialized if/when needed
	for _, w := range t.writers {
		if sw, ok := w.(stringWriter); ok {
			// 如果w实现了stringWriter接口,也就是实现了WriteString方法, 使用WriteString方法来写入
			n, err = sw.WriteString(s)
		} else {
			// 否则,使用Write方法来写入
			if p == nil {
				// 延迟初始化. 这样,即使所有w不满足stringWriter,也仅仅需要进行一次 []byte(s) 操作.
				p = []byte(s)
			}
			n, err = w.Write(p)
		}
		if err != nil {
			return
		}
		if n != len(s) {
			// 如果写入的字节数!=p的长度,说明w的空间太小
			err = ErrShortWrite
			return
		}
	}
	return len(s), nil
}

// MultiWriter creates a writer that duplicates its writes to all the
// provided writers, similar to the Unix tee(1) command.
//
// 注意: multiWriter.WriteString calls results in at most 1 allocation,
// even if multiple targets don't support WriteString.(参考multiWriter.WriteString中的延迟初始化技巧)
func MultiWriter(writers ...Writer) Writer {
	// ????????????为什么不能直接将writers赋值给目标字段,而必须经过copy
	// 参考测试 func TestMultiReaderCopy, func TestMultiWriterCopy
	w := make([]Writer, len(writers))
	// 变参在函数内部实际是slice
	copy(w, writers)
	return &multiWriter{w}
}
