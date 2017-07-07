// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[6-over]]] 2017-6-12 13:29:34 ★★★★★

// Package exec runs external commands. It wraps os.StartProcess to make it
// easier to remap stdin and stdout, connect I/O with pipes, and do other
// adjustments.
//
// Unlike the "system" library call from C and other languages, the
// os/exec package intentionally does not invoke the system shell and
// does not expand any glob patterns or handle other expansions,
// pipelines, or redirections typically done by shells. The package
// behaves more like C's "exec" family of functions. To expand glob
// patterns, either call the shell directly, taking care to escape any
// dangerous input, or use the path/filepath package's Glob function.
// To expand environment variables, use package os's ExpandEnv.
//
// Note that the examples in this package assume a Unix system.
// They may not run on Windows, and they do not run in the Go Playground
// used by golang.org and godoc.org.
package exec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// Error records the name of a binary that failed to be executed
// and the reason it failed.
type Error struct {
	Name string
	Err  error
}

func (e *Error) Error() string {
	// 注意strconv.Quote(e.Name)的使用
	return "exec: " + strconv.Quote(e.Name) + ": " + e.Err.Error()
}

// Cmd represents an external command being prepared or run.
//
// A Cmd cannot be reused after calling its Run, Output or CombinedOutput
// methods.
//
// 在调用Cmd的Run,Output,CombinedOutput方法后,Cmd不能再被重用.
type Cmd struct {
	// Path is the path of the command to run.
	//
	// This is the only field that must be set to a non-zero
	// value. If Path is relative, it is evaluated relative
	// to Dir.
	Path string

	// Args holds command line arguments, including the command as Args[0].
	// If the Args field is empty or nil, Run uses {Path}.
	//
	// In typical use, both Path and Args are set by calling Command.
	//
	// 命令本身是作为 Args[0].
	// 如果 Args 字段是 empty 或 nil, Run 会使用 Path 作为 Args[0].
	// 参考 : 'func Command(name string, arg ...string) *Cmd {' 的实现
	Args []string

	// Env specifies the environment of the process.
	// Each entry is of the form "key=value".
	// If Env is nil, the new process uses the current process's
	// environment.
	// If Env contains duplicate environment keys, only the last
	// value in the slice for each duplicate key is used.
	//
	// 格式参考: go doc os.Environ
	Env []string

	// Dir specifies the working directory of the command.
	// If Dir is the empty string, Run runs the command in the
	// calling process's current directory.
	Dir string

	// Stdin specifies the process's standard input.
	// If Stdin is nil, the process reads from the null device (os.DevNull).
	// If Stdin is an *os.File, the process's standard input is connected
	// directly to that file.
	// Otherwise, during the execution of the command a separate
	// goroutine reads from Stdin and delivers that data to the command
	// over a pipe. In this case, Wait does not complete until the goroutine
	// stops copying, either because it has reached the end of Stdin
	// (EOF or a read error) or because writing to the pipe returned an error.
	Stdin io.Reader

	// Stdout and Stderr specify the process's standard output and error.
	//
	// If either is nil, Run connects the corresponding file descriptor
	// to the null device (os.DevNull).
	//
	// If Stdout and Stderr are the same writer, and have a type that can be compared with ==,
	// at most one goroutine at a time will call Write.
	Stdout io.Writer
	Stderr io.Writer

	// ExtraFiles specifies additional open files to be inherited by the
	// new process. It does not include standard input, standard output, or
	// standard error. If non-nil, entry i becomes file descriptor 3+i.
	ExtraFiles []*os.File

	// SysProcAttr holds optional, operating system-specific attributes.
	// Run passes it to os.StartProcess as the os.ProcAttr's Sys field.
	//
	// 参考: go doc os.ProcAttr
	SysProcAttr *syscall.SysProcAttr

	// Process is the underlying process, once started.
	Process *os.Process

	// ProcessState contains information about an exited process,
	// available after a call to Wait or Run.
	ProcessState *os.ProcessState

	ctx             context.Context // nil means none
	lookPathErr     error           // LookPath error, if any.
	finished        bool            // when Wait was called
	childFiles      []*os.File
	closeAfterStart []io.Closer
	closeAfterWait  []io.Closer
	goroutine       []func() error
	errch           chan error // one send per goroutine
	// 当Cmd.Wait方法调用中的c.Process.Wait()已经完毕,waitDone会被close
	waitDone        chan struct{}
}

// Command returns the Cmd struct to execute the named program with
// the given arguments.
//
// It sets only the Path and Args in the returned structure.
//
// If name contains no path separators, Command uses LookPath to
// resolve name to a complete path if possible. Otherwise it uses name
// directly as Path.
//
// The returned Cmd's Args field is constructed from the command name
// followed by the elements of arg, so arg should not include the
// command name itself. For example, Command("echo", "hello").
// Args[0] is always name, not the possibly resolved Path.
//
// 上文中的so arg(这里指Command函数的arg参数) should not include the command name itself
func Command(name string, arg ...string) *Cmd {
	// 注意下面,可以在Struct的literal中直接使用append
	// 而append中可以直接使用[]string{name}
	cmd := &Cmd{
		Path: name,
		Args: append([]string{name}, arg...),
	}
	if filepath.Base(name) == name {
		// 根据 Command 函数文档: If name contains no path separators, Command uses LookPath to
		// resolve the path to a complete name if possible. Otherwise it uses name directly.
		if lp, err := LookPath(name); err != nil {
			// LookPath出错
			cmd.lookPathErr = err
		} else {
			// LookPath成功
			cmd.Path = lp
		}
	}
	return cmd
}

// CommandContext is like Command but includes a context.
//
// The provided context is used to kill the process (by calling
// os.Process.Kill) if the context becomes done before the command
// completes on its own.
//
// 上文中the process指返回的Cmd进程
func CommandContext(ctx context.Context, name string, arg ...string) *Cmd {
	if ctx == nil {
		panic("nil Context")
	}
	cmd := Command(name, arg...)
	cmd.ctx = ctx
	return cmd
}

// interfaceEqual protects against panics from doing equality tests on
// two interfaces with non-comparable underlying types.
//
// 注意这里的技巧
//
// 当比较两个 interface 的时候(相等性测试), 如果两个 interface 的 underlying types 是 non-comparable
// 会发生 panic
// 这里通过 defer 和 recover 来解决
func interfaceEqual(a, b interface{}) bool {
	defer func() {
		recover()
	}()
	return a == b
}

// 如果 c.Env 不是 nil, 返回 c.Env
// 否则,返回当前进程的环境(os.Environ())
func (c *Cmd) envv() []string {
	if c.Env != nil {
		return c.Env
	}
	return os.Environ()
}

func (c *Cmd) argv() []string {
	if len(c.Args) > 0 {
		return c.Args
	}
	return []string{c.Path}
}

// skipStdinCopyError optionally specifies a function which reports
// whether the provided the stdin copy error should be ignored.
// It is non-nil everywhere but Plan 9, which lacks EPIPE. See exec_posix.go.
var skipStdinCopyError func(error) bool

// stdin 会返回一个 *os.File f, 从f可以读取到stdin中的内容
func (c *Cmd) stdin() (f *os.File, err error) {
	if c.Stdin == nil {
		// c.Stdin文档: If Stdin is nil, the process reads from the null device (os.DevNull).
		f, err = os.Open(os.DevNull)
		if err != nil {
			return
		}
		c.closeAfterStart = append(c.closeAfterStart, f)
		return
	}

	if f, ok := c.Stdin.(*os.File); ok {
		// c.Stdin文档:If Stdin is an *os.File, the process's standard input is connected directly to that file.
		return f, nil
	}

	// 根据c.Stdin文档: Otherwise, during the execution of the command a separate
	// goroutine reads from Stdin and delivers that data to the command
	// over a pipe. In this case, Wait does not complete until the goroutine
	// stops copying, either because it has reached the end of Stdin
	// (EOF or a read error) or because writing to the pipe returned an error.
	pr, pw, err := os.Pipe()
	if err != nil {
		return
	}

	c.closeAfterStart = append(c.closeAfterStart, pr)
	c.closeAfterWait = append(c.closeAfterWait, pw)
	c.goroutine = append(c.goroutine, func() error {
		// 注意: Copy copies from src to dst until either EOF is reached on src or an error occurs.
		// 向 pw 写入数据, 由于是 os.Pipe() 创建的, 后续可以从 pr 中读取此时写入到 pw 中的数据.
		_, err := io.Copy(pw, c.Stdin)
		if skip := skipStdinCopyError; skip != nil && skip(err) {
			// 进行skip
			err = nil
		}
		if err1 := pw.Close(); err == nil {
			// err和err1之间优先使用err
			err = err1
		}
		return err
	})
	// 可以从pr读取到写入pw的内容,而写入pw的内容就是来源于c.Stdin的内容
	return pr, nil
}

// 根据 Cmd.Stdout 和 Cmd.Stderr 的文档
// Stdout and Stderr specify the process's standard output and error.
// If either is nil, Run connects the corresponding file descriptor to the null device (os.DevNull).
// If Stdout and Stderr are the same writer, at most one goroutine at a time will call Write.

func (c *Cmd) stdout() (f *os.File, err error) {
	return c.writerDescriptor(c.Stdout)
}

func (c *Cmd) stderr() (f *os.File, err error) {
	if c.Stderr != nil && interfaceEqual(c.Stderr, c.Stdout) {
		return c.childFiles[1], nil
	}
	// 现在,c.Stderr=nil ||  !interfaceEqual(c.Stderr, c.Stdout)
	return c.writerDescriptor(c.Stderr)
}

func (c *Cmd) writerDescriptor(w io.Writer) (f *os.File, err error) {
	if w == nil {
		// 文档:If either is nil, Run connects the corresponding file descriptor to the null device (os.DevNull).
		f, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			return
		}
		c.closeAfterStart = append(c.closeAfterStart, f)
		return
	}

	if f, ok := w.(*os.File); ok {
		return f, nil
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return
	}

	c.closeAfterStart = append(c.closeAfterStart, pw)
	c.closeAfterWait = append(c.closeAfterWait, pr)
	c.goroutine = append(c.goroutine, func() error {
		// 注意: Copy copies from src to dst until either EOF is reached on src or an error occurs.
		_, err := io.Copy(w, pr)
		pr.Close() // in case io.Copy stopped due to write error
		return err
	})
	// 向 pw写入的数据 => pr => w
	return pw, nil
}

func (c *Cmd) closeDescriptors(closers []io.Closer) {
	for _, fd := range closers {
		fd.Close()
	}
}

// Run starts the specified command and waits for it to complete.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the command starts but does not complete successfully, the error is of
// type *ExitError. Other error types may be returned for other situations.
func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		// 启动失败,返回err
		return err
	}
	// 现在,启动成功
	// 等待 Cmd 完成
	return c.Wait()
}

// lookExtensions finds windows executable by its dir and path.
// It uses LookPath to try appropriate extensions.
// lookExtensions does not search PATH, instead it converts `prog` into `.\prog`.
//
// 返回path + ext
func lookExtensions(path, dir string) (string, error) {
	if filepath.Base(path) == path {
		// 如果满足本分支,必然是这样的情况: dir = '/dir1/dir2', path = 'filename.ext'
		// 根据文档:lookExtensions does not search PATH, instead it converts `prog` into `.\prog`.
		path = filepath.Join(".", path)
	}
	if dir == "" {
		// PATH environment variable 中查找
		return LookPath(path)
	}
	// 现在,dir不为空
	if filepath.VolumeName(path) != "" {
		// path以VolumeName开头
		// 可以获取到VolumeName
		return LookPath(path)
	}
	// 现在,path不以VolumeName开头
	if len(path) > 1 && os.IsPathSeparator(path[0]) {
		// path[0]是路径分隔符
		return LookPath(path)
	}
	dirandpath := filepath.Join(dir, path)
	// We assume that LookPath will only add file extension.
	lp, err := LookPath(dirandpath)
	if err != nil {
		return "", err
	}
	ext := strings.TrimPrefix(lp, dirandpath)
	return path + ext, nil
}

// Start starts the specified command but does not wait for it to complete.
//
// The Wait method will return the exit code and release associated resources
// once the command exits.
func (c *Cmd) Start() error {
	if c.lookPathErr != nil {
		c.closeDescriptors(c.closeAfterStart)
		c.closeDescriptors(c.closeAfterWait)
		return c.lookPathErr
	}
	if runtime.GOOS == "windows" {
		lp, err := lookExtensions(c.Path, c.Dir)
		if err != nil {
			c.closeDescriptors(c.closeAfterStart)
			c.closeDescriptors(c.closeAfterWait)
			return err
		}
		c.Path = lp
	}
	if c.Process != nil {
		// 进程已经启动
		return errors.New("exec: already started")
	}
	if c.ctx != nil {
		select {
		case <-c.ctx.Done():
			// c.ctx.Done()返回一个chan,从返回的chan中接收到值说明工作应该结束了
			c.closeDescriptors(c.closeAfterStart)
			c.closeDescriptors(c.closeAfterWait)
			return c.ctx.Err()
		default:
			// 程序往下执行,不阻塞
		}
	}

	type F func(*Cmd) (*os.File, error)
	// (*Cmd).stdin, (*Cmd).stdout, (*Cmd).stderr 都满足 F 的函数签名
	// (*Cmd).stdin 是上面已经定义好的函数
	for _, setupFd := range []F{(*Cmd).stdin, (*Cmd).stdout, (*Cmd).stderr} {
		// setupFd是类型F的值
		// 这里setupFd(c)其实是调用(*Cmd).stdin,(*Cmd).stdout,(*Cmd).stderr
		fd, err := setupFd(c)
		if err != nil {
			c.closeDescriptors(c.closeAfterStart)
			c.closeDescriptors(c.closeAfterWait)
			return err
		}
		// 子进程继承
		c.childFiles = append(c.childFiles, fd)
	}
	// 子进程继承
	c.childFiles = append(c.childFiles, c.ExtraFiles...)

	var err error
	c.Process, err = os.StartProcess(c.Path, c.argv(), &os.ProcAttr{
		Dir:   c.Dir,
		Files: c.childFiles,
		Env:   dedupEnv(c.envv()),
		Sys:   c.SysProcAttr,
	})
	if err != nil {
		c.closeDescriptors(c.closeAfterStart)
		c.closeDescriptors(c.closeAfterWait)
		return err
	}

	// c.closeAfterStart,c.closeAfterWait 在任何出错的情况下都要close
	// c.closeAfterStart在成功的情况下close
	c.closeDescriptors(c.closeAfterStart)

	c.errch = make(chan error, len(c.goroutine))
	for _, fn := range c.goroutine {
		go func(fn func() error) {
			c.errch <- fn()
		}(fn)
	}

	if c.ctx != nil {
		c.waitDone = make(chan struct{})
		go func() {
			select {
			case <-c.ctx.Done():
				// c.ctx.Done()返回一个chan,从返回的chan中接收到值说明工作应该结束了
				c.Process.Kill()
			case <-c.waitDone:
			}
		}()
	}

	return nil
}

// An ExitError reports an unsuccessful exit by a command.
type ExitError struct {
	*os.ProcessState

	// Stderr holds a subset of the standard error output from the
	// Cmd.Output method if standard error was not otherwise being
	// collected.
	//
	// If the error output is long, Stderr may contain only a prefix
	// and suffix of the output, with the middle replaced with
	// text about the number of omitted bytes.
	//
	// Stderr is provided for debugging, for inclusion in error messages.
	// Users with other needs should redirect Cmd.Stderr as needed.
	Stderr []byte
}

func (e *ExitError) Error() string {
	return e.ProcessState.String()
}

// Wait waits for the command to exit and waits for any copying to
// stdin or copying from stdout or stderr to complete.
//
// The command must have been started by Start.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the command fails to run or doesn't complete successfully, the
// error is of type *ExitError. Other error types may be
// returned for I/O problems.
//
// If c.Stdin is not an *os.File, Wait also waits for the I/O loop
// copying from c.Stdin into the process's standard input
// to complete.
//
// Wait releases any resources associated with the Cmd.
func (c *Cmd) Wait() error {
	if c.Process == nil {
		// 进程还未启动(Start还未调用过)
		return errors.New("exec: not started")
	}
	if c.finished {
		return errors.New("exec: Wait was already called")
	}
	// 标记Wait已经被调用
	c.finished = true

	state, err := c.Process.Wait()
	if c.waitDone != nil {
		// 标记Cmd.Wait方法调用中的c.Process.Wait()已经完毕
		close(c.waitDone)
	}
	c.ProcessState = state

	var copyError error
	for range c.goroutine {
		if err := <-c.errch; err != nil && copyError == nil {
			copyError = err
		}
	}

	c.closeDescriptors(c.closeAfterWait)

	if err != nil {
		return err
	} else if !state.Success() {
		return &ExitError{ProcessState: state}
	}

	// err比copyError优先级高
	return copyError
}

// Output runs the command and returns its standard output.
// Any returned error will usually be of type *ExitError.
// If c.Stderr was nil, Output populates ExitError.Stderr.
//
// 此方法内部调用了Run.
// 观察源码,此方法不应该被重复调用.
func (c *Cmd) Output() ([]byte, error) {
	if c.Stdout != nil {
		// 不应该重复调用此方法
		return nil, errors.New("exec: Stdout already set")
	}
	var stdout bytes.Buffer
	// *bytes.Buffer实现了io.Writer
	c.Stdout = &stdout

	captureErr := c.Stderr == nil
	if captureErr {
		c.Stderr = &prefixSuffixSaver{N: 32 << 10}
	}

	err := c.Run()
	if err != nil && captureErr {
		if ee, ok := err.(*ExitError); ok {
			// 根据文档:If c.Stderr was nil, Output populates ExitError.Stderr.
			ee.Stderr = c.Stderr.(*prefixSuffixSaver).Bytes()
		}
	}
	return stdout.Bytes(), err
}

// CombinedOutput runs the command and returns its combined standard
// output and standard error.
//
// 此方法内部调用了Run.
// 观察源码,此方法不应该被重复调用.
func (c *Cmd) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var b bytes.Buffer
	// 注意: c.Stdout , c.Stderr 文档中提到
	// If Stdout and Stderr are the same writer, at most one goroutine at a time will call Write.
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	return b.Bytes(), err
}

// StdinPipe returns a pipe that will be connected to the command's
// standard input when the command starts.
// The pipe will be closed automatically after Wait sees the command exit.
// A caller need only call Close to force the pipe to close sooner.
// For example, if the command being run will not exit until standard input
// is closed, the caller must close the pipe.
//
// 向返回的io.WriteCloser写入数据,之后会从c.Stdin被读取到.
// 观察源码,此方法不应该被重复调用.
// 观察源码,此方法应该在子进程启动之前被调用,否则会返回错误.
func (c *Cmd) StdinPipe() (io.WriteCloser, error) {
	if c.Stdin != nil {
		return nil, errors.New("exec: Stdin already set")
	}
	if c.Process != nil {
		return nil, errors.New("exec: StdinPipe after process started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stdin = pr
	c.closeAfterStart = append(c.closeAfterStart, pr)
	wc := &closeOnce{File: pw}
	c.closeAfterWait = append(c.closeAfterWait, closerFunc(wc.safeClose))
	return wc, nil
}

// closeOnce.Close 即使多次被调用只会有一次有效
type closeOnce struct {
	*os.File

	writers sync.RWMutex // coordinate safeClose and Write
	once    sync.Once
	err     error
}

func (c *closeOnce) Close() error {
	c.once.Do(c.close)
	return c.err
}

func (c *closeOnce) close() {
	c.err = c.File.Close()
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// safeClose closes c being careful not to race with any calls to c.Write.
// See golang.org/issue/9307 and TestEchoFileRace in exec_test.go.
// In theory other calls could also be excluded (by writing appropriate
// wrappers like c.Write's implementation below), but since c is most
// commonly used as a WriteCloser, Write is the main one to worry about.
// See also #7970, for which this is a partial fix for this specific instance.
// The idea is that we return a WriteCloser, and so the caller can be
// relied upon not to call Write and Close simultaneously, but it's less
// obvious that cmd.Wait calls Close and that the caller must not call
// Write and cmd.Wait simultaneously. In fact that seems too onerous.
// So we change the use of Close in cmd.Wait to use safeClose, which will
// synchronize with any Write.
//
// It's important that we know this won't block forever waiting for the
// operations being excluded. At the point where this is called,
// the invoked command has exited and the parent copy of the read side
// of the pipe has also been closed, so there should really be no read side
// of the pipe left. Any active writes should return very shortly with an EPIPE,
// making it reasonable to wait for them.
// Technically it is possible that the child forked a sub-process or otherwise
// handed off the read side of the pipe before exiting and the current holder
// is not reading from the pipe, and the pipe is full, in which case the close here
// might block waiting for the write to complete. That's probably OK.
// It's a small enough problem to be outweighed by eliminating the race here.
func (c *closeOnce) safeClose() error {
	c.writers.Lock()
	err := c.Close()
	c.writers.Unlock()
	return err
}

func (c *closeOnce) Write(b []byte) (int, error) {
	c.writers.RLock()
	n, err := c.File.Write(b)
	c.writers.RUnlock()
	return n, err
}

func (c *closeOnce) WriteString(s string) (int, error) {
	c.writers.RLock()
	n, err := c.File.WriteString(s)
	c.writers.RUnlock()
	return n, err
}

// StdoutPipe returns a pipe that will be connected to the command's
// standard output when the command starts.
//
// Wait will close the pipe after seeing the command exit, so most callers
// need not close the pipe themselves; however, an implication is that
// it is incorrect to call Wait before all reads from the pipe have completed.
// For the same reason, it is incorrect to call Run when using StdoutPipe.
// See the example for idiomatic usage.
//
// 从返回的io.ReadCloser读取数据,会读取到写入c.Stdout的数据(数据=>c.Stdout=>io.ReadCloser)
// 观察源码,此方法不应该被重复调用.
// 观察源码,此方法应该在子进程启动之前被调用,否则会返回错误.
func (c *Cmd) StdoutPipe() (io.ReadCloser, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Process != nil {
		return nil, errors.New("exec: StdoutPipe after process started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stdout = pw
	c.closeAfterStart = append(c.closeAfterStart, pw)
	c.closeAfterWait = append(c.closeAfterWait, pr)
	return pr, nil
}

// StderrPipe returns a pipe that will be connected to the command's
// standard error when the command starts.
//
// Wait will close the pipe after seeing the command exit, so most callers
// need not close the pipe themselves; however, an implication is that
// it is incorrect to call Wait before all reads from the pipe have completed.
// For the same reason, it is incorrect to use Run when using StderrPipe.
// See the StdoutPipe example for idiomatic usage.
//
// 从返回的io.ReadCloser读取数据,会读取到写入c.Stderr的数据(数据=>c.Stderr=>io.ReadCloser)
// 观察源码,此方法不应该被重复调用.
// 观察源码,此方法应该在子进程启动之前被调用,否则会返回错误.
func (c *Cmd) StderrPipe() (io.ReadCloser, error) {
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	if c.Process != nil {
		return nil, errors.New("exec: StderrPipe after process started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stderr = pw
	c.closeAfterStart = append(c.closeAfterStart, pw)
	c.closeAfterWait = append(c.closeAfterWait, pr)
	return pr, nil
}

// prefixSuffixSaver相关的暂时不看

// prefixSuffixSaver is an io.Writer which retains the first N bytes
// and the last N bytes written to it. The Bytes() methods reconstructs
// it with a pretty error message.
type prefixSuffixSaver struct {
	N         int // max size of prefix or suffix
	prefix    []byte
	suffix    []byte // ring buffer once len(suffix) == N
	suffixOff int    // offset to write into suffix
	skipped   int64

	// TODO(bradfitz): we could keep one large []byte and use part of it for
	// the prefix, reserve space for the '... Omitting N bytes ...' message,
	// then the ring buffer suffix, and just rearrange the ring buffer
	// suffix when Bytes() is called, but it doesn't seem worth it for
	// now just for error messages. It's only ~64KB anyway.
}

func (w *prefixSuffixSaver) Write(p []byte) (n int, err error) {
	lenp := len(p)
	p = w.fill(&w.prefix, p)

	// Only keep the last w.N bytes of suffix data.
	if overage := len(p) - w.N; overage > 0 {
		p = p[overage:]
		w.skipped += int64(overage)
	}
	p = w.fill(&w.suffix, p)

	// w.suffix is full now if p is non-empty. Overwrite it in a circle.
	for len(p) > 0 { // 0, 1, or 2 iterations.
		n := copy(w.suffix[w.suffixOff:], p)
		p = p[n:]
		w.skipped += int64(n)
		w.suffixOff += n
		if w.suffixOff == w.N {
			w.suffixOff = 0
		}
	}
	return lenp, nil
}

// fill appends up to len(p) bytes of p to *dst, such that *dst does not
// grow larger than w.N. It returns the un-appended suffix of p.
func (w *prefixSuffixSaver) fill(dst *[]byte, p []byte) (pRemain []byte) {
	if remain := w.N - len(*dst); remain > 0 {
		add := minInt(len(p), remain)
		*dst = append(*dst, p[:add]...)
		p = p[add:]
	}
	return p
}

func (w *prefixSuffixSaver) Bytes() []byte {
	if w.suffix == nil {
		return w.prefix
	}
	if w.skipped == 0 {
		return append(w.prefix, w.suffix...)
	}
	var buf bytes.Buffer
	buf.Grow(len(w.prefix) + len(w.suffix) + 50)
	buf.Write(w.prefix)
	buf.WriteString("\n... omitting ")
	buf.WriteString(strconv.FormatInt(w.skipped, 10))
	buf.WriteString(" bytes ...\n")
	buf.Write(w.suffix[w.suffixOff:])
	buf.Write(w.suffix[:w.suffixOff])
	return buf.Bytes()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// dedupEnv returns a copy of env with any duplicates removed, in favor of
// later values.
// Items not of the normal environment "key=value" form are preserved unchanged.
//
// @see
func dedupEnv(env []string) []string {
	return dedupEnvCase(runtime.GOOS == "windows", env)
}

// dedupEnvCase is dedupEnv with a case option for testing.
// If caseInsensitive is true, the case of keys is ignored.
//
// @see
func dedupEnvCase(caseInsensitive bool, env []string) []string {
	// out:函数最终的返回值
	out := make([]string, 0, len(env))
	// saw:见过的
	saw := map[string]int{} // key => index into out
	for _, kv := range env {
		// eq代表=号的位置
		eq := strings.Index(kv, "=")
		if eq < 0 {
			// 不存在=号
			out = append(out, kv)
			continue
		}
		// 现在,找到了=号
		k := kv[:eq]
		if caseInsensitive {
			k = strings.ToLower(k)
		}
		if dupIdx, isDup := saw[k]; isDup {
			// 如果重复,使用最后出现的值
			out[dupIdx] = kv
			continue
		}
		// 记录见过
		saw[k] = len(out)
		// append到结果
		out = append(out, kv)
	}
	return out
}
