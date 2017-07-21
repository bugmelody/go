// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[5-over]]] 2017-6-12 16:12:04

package exec_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func ExampleLookPath() {
	// 在环境变量path的路径中查找可执行文件fortune
	path, err := exec.LookPath("fortune")
	if err != nil {
		// 未找到,需要安装
		log.Fatal("installing fortune is in your future")
	}
	// 找到了
	fmt.Printf("fortune is available at %s\n", path)
}

func ExampleCommand() {
	/**
	$ tr a-z A-Z
	some input
	SOME INPUT
	 */
	cmd := exec.Command("tr", "a-z", "A-Z")
	cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	// 设置输出到&out
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		// 运行出错
		log.Fatal(err)
	}
	// 运行成功并且exit status为0
	// caps. abbr. capitals (capital letters)
	fmt.Printf("in all caps: %q\n", out.String())
}

func ExampleCommand_environment() {
	cmd := exec.Command("prog")
	// cmd.Env的文档中提到:
	// If Env contains duplicate environment keys, only the last
	// value in the slice for each duplicate key is used.
	cmd.Env = append(os.Environ(),
		"FOO=duplicate_value", // ignored
		"FOO=actual_value",    // this value is used
	)
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}

func ExampleCmd_Output() {
	// Output方法内部调用了Run.
	out, err := exec.Command("date").Output()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("The date is %s\n", out)
}

func ExampleCmd_Run() {
	cmd := exec.Command("sleep", "1")
	log.Printf("Running command and waiting for it to finish...")
	err := cmd.Run()
	log.Printf("Command finished with error: %v", err)
}

func ExampleCmd_Start() {
	cmd := exec.Command("sleep", "5")
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Waiting for command to finish...")
	err = cmd.Wait()
	log.Printf("Command finished with error: %v", err)
}

// ???????? 使用 cmd.StdoutPipe 和 cmd.Output 的场景有什么区别呢???
// ??????她们各有什么优势和劣势??????
func ExampleCmd_StdoutPipe() {
	// -n在man中的说明:
	// -n: do not output the trailing newline
	cmd := exec.Command("echo", "-n", `{"Name": "Bob", "Age": 32}`)
	// StdoutPipe方法应该在子进程启动之前被调用,否则会返回错误.
	stdout, err := cmd.StdoutPipe()
	// 稍后,可以从stdout获取进程的stdout输出
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	var person struct {
		Name string
		Age  int
	}
	// Cmd.StdoutPipe 文档中提到:
	// it is incorrect to call Wait before all reads from the pipe have completed.
	// 因此需要先读取完stdout后才能调用Cmd.Wait
	if err := json.NewDecoder(stdout).Decode(&person); err != nil {
		log.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s is %d years old\n", person.Name, person.Age)
}

func ExampleCmd_StdinPipe() {
	cmd := exec.Command("cat")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, "values written to stdin are passed to cmd's standard input")
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", out)
}

func ExampleCmd_StderrPipe() {
	cmd := exec.Command("sh", "-c", "echo stdout; echo 1>&2 stderr")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	slurp, _ := ioutil.ReadAll(stderr)
	fmt.Printf("%s\n", slurp)

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}

func ExampleCmd_CombinedOutput() {
	cmd := exec.Command("sh", "-c", "echo stdout; echo 1>&2 stderr")
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", stdoutStderr)
}

func ExampleCommandContext() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := exec.CommandContext(ctx, "sleep", "5").Run(); err != nil {
		// This will fail after 100 milliseconds. The 5 second sleep
		// will be interrupted.
	}
}
