// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-20 19:12:21

package io_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

func ExampleCopy() {
	// 首先 *strings.Reader 满足 io.Reader 接口; 其次 *strings.Reader 还满足 io.WriterTo 接口
	// 因此下面 io.Copy(os.Stdout, r) 会直接调用 r.WriteTo . 达到最高效的使用
	// io.Copy 文档中提到
	// If src implements the WriterTo interface,
	// the copy is implemented by calling src.WriteTo(dst).
	// Otherwise, if dst implements the ReaderFrom interface,
	// the copy is implemented by calling dst.ReadFrom(src).
	// func Copy(dst Writer, src Reader) (written int64, err error) {
	r := strings.NewReader("some io.Reader stream to be read\n")

	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Fatal(err)
	}

	// Output:
	// some io.Reader stream to be read
}

func ExampleCopyBuffer() {
	r1 := strings.NewReader("first reader\n")
	r2 := strings.NewReader("second reader\n")
	// 通过8字节的buffer进行io.CopyBuffer的中转
	buf := make([]byte, 8)

	// buf is used here...
	if _, err := io.CopyBuffer(os.Stdout, r1, buf); err != nil {
		log.Fatal(err)
	}

	// ... reused here also. No need to allocate an extra buffer.
	// 在这里继续使用 buf, 避免了不停的进行内存分配
	if _, err := io.CopyBuffer(os.Stdout, r2, buf); err != nil {
		log.Fatal(err)
	}

	// Output:
	// first reader
	// second reader
}

func ExampleCopyN() {
	r := strings.NewReader("some io.Reader stream to be read")

	if _, err := io.CopyN(os.Stdout, r, 5); err != nil {
		// 实际copy了'some '
		log.Fatal(err)
	}

	// Output:
	// some
}

func ExampleReadAtLeast() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	// 注意:上面r中刚好33个字节

	buf := make([]byte, 33)
	if _, err := io.ReadAtLeast(r, buf, 4); err != nil {
		// 4表示至少要读取4字节,因为buf分配了33个字节长度,因此这是没有问题的
		log.Fatal(err)
	}
	// 输出 'some io.Reader stream to be read\n', 刚好读完(刚好读完的原因是buf也是33个字节)
	fmt.Printf("%s\n", buf)

	// buffer smaller than minimal read size.
	shortBuf := make([]byte, 3)
	if _, err := io.ReadAtLeast(r, shortBuf, 4); err != nil {
		// 4表示至少要读取4字节,因为shortBuf分配了3个字节长度,因此会报错说buffer太小
		fmt.Println("error:", err)
	}

	// minimal read size bigger than io.Reader stream
	longBuf := make([]byte, 64)
	if _, err := io.ReadAtLeast(r, longBuf, 64); err != nil {
		// 64表示至少要读取64字节,因为longBuf分配了64个字节长度,buffer的容量是够了,但是r中内容太少肯定读不到64个字节
		// ???????? 为什么下面会输出 // error: EOF ??????????

		/**
		根据 io.ReadAtLeast 的文档:
		It returns the number of bytes copied and an error if fewer bytes were read.
		The error is EOF only if no bytes were read.
		If an EOF happens after reading fewer than min bytes,
		ReadAtLeast returns ErrUnexpectedEOF.
		??????????
		 */
		fmt.Println("error:", err)
	}

	// Output:
	// some io.Reader stream to be read
	//
	// error: short buffer
	// error: EOF
}

func ExampleReadFull() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		// 可以读满,因此不会出错
		log.Fatal(err)
	}
	// 返回读满的 buf 中的内容
	fmt.Printf("%s\n", buf)

	// minimal read size bigger than io.Reader stream
	longBuf := make([]byte, 64)
	if _, err := io.ReadFull(r, longBuf); err != nil {
		// 读不满,因此一定会出错
		// 根据 io.ReadFull 的文档: If an EOF happens after reading some but not all the bytes, ReadFull returns ErrUnexpectedEOF.
		fmt.Println("error:", err)
	}

	// Output:
	// some
	// error: unexpected EOF
}

func ExampleWriteString() {
	io.WriteString(os.Stdout, "Hello World")

	// Output: Hello World
}

func ExampleLimitReader() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	// 设置为最多读取4字节
	lr := io.LimitReader(r, 4)

	if _, err := io.Copy(os.Stdout, lr); err != nil {
		log.Fatal(err)
	}

	// Output:
	// some
}

func ExampleMultiReader() {
	r1 := strings.NewReader("first reader ")
	r2 := strings.NewReader("second reader ")
	r3 := strings.NewReader("third reader\n")
	// 读完r1,再读r2,再读r3
	r := io.MultiReader(r1, r2, r3)

	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Fatal(err)
	}

	// Output:
	// first reader second reader third reader
}

func ExampleTeeReader() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	var buf bytes.Buffer
	// 从r中读取的数据将会被写入buf
	tee := io.TeeReader(r, &buf)

	printall := func(r io.Reader) {
		b, err := ioutil.ReadAll(r)
		if err != nil {
			log.Fatal(err)
		}

		// 打印从 r 中读取到的内容
		fmt.Printf("%s", b)
	}

	// 从tee这个Reader中读取数据,同时读取的数据会被写入buf
	printall(tee)
	// 因为上面通过printall(tee)对tee进行了读取,因此buf现在已经包含了从tee读取的数据
	// bytes.Buffer也实现了Read方法,因此也是io.Reader
	printall(&buf)
	// 上面两个 printall 调用会在屏幕上输出同样的内容

	// Output:
	// some io.Reader stream to be read
	// some io.Reader stream to be read
}

func ExampleSectionReader() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	// r从offset位置5开始,读取17个字节后结束, 因此读取到的应该是 'io.Reader stream '
	s := io.NewSectionReader(r, 5, 17)

	if _, err := io.Copy(os.Stdout, s); err != nil {
		log.Fatal(err)
	}

	// Output:
	// io.Reader stream
}

func ExampleSectionReader_ReadAt() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	// 从5开始,读取16个字节,也即是 'io.Reader stream'
	s := io.NewSectionReader(r, 5, 16)
	// 现在,s代表 'io.Reader stream'

	buf := make([]byte, 6)
	// 将s中的数据读入buf,从offset为10的位置开始
	// s之前已经被设置为'io.Reader stream', 从10开始是'stream'
	if _, err := s.ReadAt(buf, 10); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", buf)

	// Output:
	// stream
}

func ExampleSectionReader_Seek() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	// s : 'io.Reader stream'
	s := io.NewSectionReader(r, 5, 16)

	// r"some io.Reader stream to be read\n"
	//        |------s-------|

	// Seek是io.Seeker接口要求实现的方法
	if _, err := s.Seek(10, io.SeekStart); err != nil {
		log.Fatal(err)
	}
	// r"some io.Reader stream to be read\n"
	//        |------s-------|
	//                  |seek到这里

	// Seek之后,下次Read或Write会从'stream'的位置进行

	buf := make([]byte, 6)
	if _, err := s.Read(buf); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", buf)

	// Output:
	// stream
}

func ExampleMultiWriter() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	var buf1, buf2 bytes.Buffer
	// 通过io.MultiWriter返回的Writer,写入的时候,会同时写入多个writer中
	w := io.MultiWriter(&buf1, &buf2)

	if _, err := io.Copy(w, r); err != nil {
		log.Fatal(err)
	}

	// 以上通过往w写入数据,实际是buf1和buf2被同时写入了
	fmt.Print(buf1.String())
	fmt.Print(buf2.String())

	// Output:
	// some io.Reader stream to be read
	// some io.Reader stream to be read
}
