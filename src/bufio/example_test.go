// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// [[[5-over]]] 2017-6-8 16:48:30

package bufio_test

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func ExampleWriter() {
	// 写入目标是 os.Stdout, bufio.Writer 提供了中间缓冲, 避免频繁刷新到 os.Stdout.
	w := bufio.NewWriter(os.Stdout)
	fmt.Fprint(w, "Hello, ")
	fmt.Fprint(w, "world!")
	// 现在, 写入的数据<可能>都在 buffer 中.
	w.Flush() // Don't forget to flush!
	// Output: Hello, world!
}

// The simplest use of a Scanner, to read standard input as a set of lines.
func ExampleScanner_lines() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fmt.Println(scanner.Text()) // Println will add back the final '\n'
	}
	// 错误处理
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}

// Use a Scanner to implement a simple word-count utility by scanning the
// input as a sequence of space-delimited tokens.
func ExampleScanner_words() {
	// An artificial input source.
	// 共15个单词
	const input = "Now is the winter of our discontent,\nMade glorious summer by this sun of York.\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	// Set the split function for the scanning operation.
	scanner.Split(bufio.ScanWords)
	// Count the words.
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading input:", err)
	}
	fmt.Printf("%d\n", count)
	// Output: 15
}

// Use a Scanner with a custom split function (built by wrapping ScanWords) to validate
// 32-bit decimal input.
func ExampleScanner_custom() {
	// An artificial input source.
	// artificial: adj. 人造的；仿造的；虚伪的；非原产地的；武断的
	const input = "1234 5678 1234567901234567890"
	scanner := bufio.NewScanner(strings.NewReader(input))
	// Create a custom split function by wrapping the existing ScanWords function.
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// 委托给 bufio.ScanWords
		advance, token, err = bufio.ScanWords(data, atEOF)
		if err == nil && token != nil {
			// 判断能否成功调用 strconv.ParseInt 容纳到一个 int32 中
			// 修正err
			_, err = strconv.ParseInt(string(token), 10, 32)
		}
		return
	}
	// Set the split function for the scanning operation.
	scanner.Split(split)
	// Validate the input
	for scanner.Scan() {
		fmt.Printf("%s\n", scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		// 1234567901234567890 无法容纳到一个 int32 中
		fmt.Printf("Invalid input: %s", err)
	}
	// Output:
	// 1234
	// 5678
	// Invalid input: strconv.ParseInt: parsing "1234567901234567890": value out of range
}

// Use a Scanner with a custom split function to parse a comma-separated
// list with an empty final value.
func ExampleScanner_emptyFinalToken() {
	// Comma-separated list; last entry is empty.
	const input = "1,2,3,4,"
	scanner := bufio.NewScanner(strings.NewReader(input))
	// Define a split function that separates on commas.
	onComma := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		for i := 0; i < len(data); i++ {
			// 循环 data 中每个字节
			if data[i] == ',' {
				// 如果发现了逗号
				return i + 1, data[:i], nil
			}
		}
		// There is one final token to be delivered, which may be the empty string.
		// Returning bufio.ErrFinalToken here tells Scan there are no more tokens after this
		// but does not trigger an error to be returned from Scan itself.
		return 0, data, bufio.ErrFinalToken
	}
	scanner.Split(onComma)
	// Scan.
	for scanner.Scan() {
		fmt.Printf("%q ", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading input:", err)
	}
	// Output: "1" "2" "3" "4" ""
}
