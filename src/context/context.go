// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[6-over]]] 2017-6-25 10:22:56

// Package context defines the Context type, which carries deadlines,
// cancelation signals, and other request-scoped values across API boundaries
// and between processes.
//
// carries v. 携带；传递；运载；怀孕（carry的第三人称单数形式）
// process ['prəuses; 'prɔ-] n.1.过程,进程 2.(时间等的)变化过程 3.(包含许多变化的)连续发展过程 4.步骤；方法；程序；工序 5.加工方法，操作工序；制作法
//
// Incoming requests to a server should create a Context, and outgoing
// calls to servers should accept a Context. The chain of function
// calls between them must propagate the Context, optionally replacing
// it with a derived Context created using WithCancel, WithDeadline,
// WithTimeout, or WithValue. When a Context is canceled, all
// Contexts derived from it are also canceled.
//
// 进入一个server的请求应该创建Context,出去其他servers的调用应该接收Context(比如db查询,api调用).
//
// The WithCancel, WithDeadline, and WithTimeout functions take a
// Context (the parent) and return a derived Context (the child) and a
// CancelFunc. Calling the CancelFunc cancels the child and its
// children, removes the parent's reference to the child, and stops
// any associated timers. Failing to call the CancelFunc leaks the
// child and its children until the parent is canceled or the timer
// fires. The go vet tool checks that CancelFuncs are used on all
// control-flow paths.
//
// Programs that use Contexts should follow these rules to keep interfaces
// consistent across packages and enable static analysis tools to check context
// propagation:
//
// Do not store Contexts inside a struct type; instead, pass a Context
// explicitly to each function that needs it. The Context should be the first
// parameter, typically named ctx:
//
// 	func DoSomething(ctx context.Context, arg Arg) error {
// 		// ... use ctx ...
// 	}
//
// Do not pass a nil Context, even if a function permits it. Pass context.TODO
// if you are unsure about which Context to use.
//
// Use context Values only for request-scoped data that transits processes and
// APIs, not for passing optional parameters to functions.
//
// transit ['trænsɪt; 'trɑːns-; -nz-] n. 运输；经过 vt. 运送 vi. 经过
//
// The same Context may be passed to functions running in different goroutines;
// Contexts are safe for simultaneous use by multiple goroutines.
//
// See https://blog.golang.org/context for example code for a server that uses
// Contexts.
package context

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// A Context carries a deadline, a cancelation signal, and other values across
// API boundaries.
//
// Context's methods may be called by multiple goroutines simultaneously.
//
// 注意: context.Context这个type只是个接口.
type Context interface {
	// Deadline returns the time when work done on behalf of this context
	// should be canceled. Deadline returns ok==false when no deadline is
	// set. Successive calls to Deadline return the same results.
	//
	// on behalf of : 1. 代表…一方；代表(或为)…说话，作为…的代表(或代言人) 2. 为了；为了…的利益
	Deadline() (deadline time.Time, ok bool)

	// Done returns a channel that's closed when work done on behalf of this
	// context should be canceled. Done may return nil if this context can
	// never be canceled. Successive calls to Done return the same value.
	//
	// 注意: nil 是 channel 的 zero value
	//
	// tgpl书中提到这样几句话
	// The zero value for a channel is nil. Perhaps surprisingly, nil channels are sometimes useful.
	// Because send and receive operations on a nil channel block forever, a case in a select statement
	// whose channel is nil is never selected. This lets us use nil to enable or disable cases that correspond
	// to features like handling timeouts or cancellation, responding to other input events, or emitting output.
	// 对一个nil的channel进行接收会阻塞.
	//
	// WithCancel arranges for Done to be closed when cancel is called;
	// WithDeadline arranges for Done to be closed when the deadline
	// expires; WithTimeout arranges for Done to be closed when the timeout
	// elapses.
	//
	// arrange for: 安排；为…做准备
	// WithCancel 函数签名: func WithCancel(parent Context) (ctx Context, cancel CancelFunc)
	// WithCancel返回的CancelFunc被调用的时候,Context.Done()返回的chan会被close.
	//
	// Done is provided for use in select statements:
	//
	//  // Stream generates values with DoSomething and sends them to out
	//  // until DoSomething returns an error or ctx.Done is closed.
	//  func Stream(ctx context.Context, out chan<- Value) error {
	//  	for {
	//  		v, err := DoSomething(ctx)
	//  		if err != nil {
	//  			return err
	//  		}
	//  		select {
	//  		case <-ctx.Done():
	//  			return ctx.Err()
	//  		case out <- v:
	//  		}
	//  	}
	//  }
	//
	// See https://blog.golang.org/pipelines for more examples of how to use
	// a Done channel for cancelation.
	Done() <-chan struct{}

	// If Done is not yet closed, Err returns nil.
	// If Done is closed, Err returns a non-nil error explaining why:
	// Canceled if the context was canceled
	// or DeadlineExceeded if the context's deadline passed.
	// After Err returns a non-nil error, successive calls to Err return the same error.
	//
	//
	// Canceled,DeadlineExceeded 都是 error, 下方有定义
	// var Canceled = errors.New("context canceled")
	// var DeadlineExceeded error = deadlineExceededError{}
	Err() error

	// Value returns the value associated with this context for key, or nil
	// if no value is associated with key. Successive calls to Value with
	// the same key returns the same result.
	//
	// Use context values only for request-scoped data that transits
	// processes and API boundaries, not for passing optional parameters to
	// functions.
	//
	// A key identifies a specific value in a Context. Functions that wish
	// to store values in Context typically allocate a key in a global
	// variable then use that key as the argument to context.WithValue and
	// Context.Value. A key can be any type that supports equality;
	// packages should define keys as an unexported type to avoid
	// collisions.
	//
	// Packages that define a Context key should provide type-safe accessors
	// for the values stored using that key:
	//
	// 	// Package user defines a User type that's stored in Contexts.
	// 	package user
	//
	// 	import "context"
	//
	// 	// User is the type of value stored in the Contexts.
	// 	type User struct {...}
	//
	// 	// key is an unexported type for keys defined in this package.
	// 	// This prevents collisions with keys defined in other packages.
	// 	type key int
	//
	// 	// userKey is the key for user.User values in Contexts. It is
	// 	// unexported; clients use user.NewContext and user.FromContext
	// 	// instead of using this key directly.
	// 	var userKey key = 0
	//
	// 	// NewContext returns a new Context that carries value u.
	// 	func NewContext(ctx context.Context, u *User) context.Context {
	// 		return context.WithValue(ctx, userKey, u)
	// 	}
	//
	// 	// FromContext returns the User value stored in ctx, if any.
	// 	func FromContext(ctx context.Context) (*User, bool) {
	// 		u, ok := ctx.Value(userKey).(*User)
	// 		return u, ok
	// 	}
	Value(key interface{}) interface{}
}

// Canceled is the error returned by Context.Err when the context is canceled.
var Canceled = errors.New("context canceled")

// DeadlineExceeded is the error returned by Context.Err when the context's
// deadline passes.
var DeadlineExceeded error = deadlineExceededError{}

type deadlineExceededError struct{}

// 实现内置的error接口
func (deadlineExceededError) Error() string   { return "context deadline exceeded" }
// 表明是由于超时引发的错误
func (deadlineExceededError) Timeout() bool   { return true }
func (deadlineExceededError) Temporary() bool { return true }

// An emptyCtx is never canceled, has no values, and has no deadline. It is not
// struct{}, since vars of this type must have distinct addresses.
//
// (这句话是什么意思? struct{} 是不占用内存空间的)
//
//
// emptyCtx实现了Context接口
type emptyCtx int

func (*emptyCtx) Deadline() (deadline time.Time, ok bool) {
	// 注: An emptyCtx has no deadline.
	// deadline和ok会返回对应的zero value
	// bool的零值是false, 表明没有deadline
	return
}

// 返回nil channel,造成从返回的channel进行接收时一直阻塞,达到 never canceled 的效果
func (*emptyCtx) Done() <-chan struct{} {
	// qc: An emptyCtx is never canceled

	// The value of an uninitialized channel is nil.
	// 根据 spec: A nil channel is never ready for communication
	// ================
	// tgpl书中提到这样几句话
	// The zero value for a channel is nil. Perhaps surprisingly, nil channels are sometimes useful.
	// Because send and receive operations on a nil channel block forever, a case in a select statement
	// whose channel is nil is never selected. This lets us use nil to enable or disable cases that correspond
	// to features like handling timeouts or cancellation, responding to other input events, or emitting output.
	// 对一个nil的channel进行接收会阻塞.
	// ================
	return nil
}

func (*emptyCtx) Err() error {
	// 永远不返回错误
	return nil
}

func (*emptyCtx) Value(key interface{}) interface{} {
	// 注: emptyCtx has no values
	return nil
}

func (e *emptyCtx) String() string {
	// background和todo都是*emptyCtx类型的具体值
	switch e {
	case background:
		return "context.Background"
	case todo:
		return "context.TODO"
	}
	return "unknown empty Context"
}

var (
	background = new(emptyCtx)
	todo       = new(emptyCtx)
)

// Background returns a non-nil, empty Context. It is never canceled, has no
// values, and has no deadline. It is typically used by the main function,
// initialization, and tests, and as the top-level Context for incoming
// requests.
func Background() Context {
	return background
}

// TODO returns a non-nil, empty Context. Code should use context.TODO when
// it's unclear which Context to use or it is not yet available (because the
// surrounding function has not yet been extended to accept a Context
// parameter). TODO is recognized by static analysis tools that determine
// whether Contexts are propagated correctly in a program.
func TODO() Context {
	return todo
}

// A CancelFunc tells an operation to abandon its work.
// A CancelFunc does not wait for the work to stop.
// After the first call, subsequent calls to a CancelFunc do nothing.
//
// 上文中:
// A CancelFunc tells(仅仅是告诉,并不会进行实际的取消)
// 也就是说调用CancelFunc是立即返回的,不会阻塞
type CancelFunc func()

// WithCancel returns a copy of parent with a new Done channel. The returned
// context's Done channel is closed when the returned cancel function is called
// or when the parent context's Done channel is closed, whichever happens first.
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this Context complete.
//
// 上文中: Canceling this context(指返回的ctx)
//
// WithCancel会返回parent的一个拷贝ctx,但是Done字段是新的.
// 当cancel被调用,或者parent.Done被关闭,这两种情况下(任意一个先发生),ctx.Done也会被关闭
func WithCancel(parent Context) (ctx Context, cancel CancelFunc) {
	c := newCancelCtx(parent)
	// 注: propagateCancel arranges for child to be canceled when parent is.
	propagateCancel(parent, &c)
	return &c, func() { c.cancel(true, Canceled) }
}

// newCancelCtx returns an initialized cancelCtx.
//
// newCancelCtx根据parent返回cancelCtx
// cancelCtx.Context字段是匿名字段,因此cancelCtx拥有了parent(类型为context.Context)的所有字段和方法
// cancelCtx也满足Context接口
func newCancelCtx(parent Context) cancelCtx {
	// cancelCtx.Context 是匿名字段
	return cancelCtx{Context: parent}
}

// propagateCancel arranges for child to be canceled when parent is.
func propagateCancel(parent Context, child canceler) {
	if parent.Done() == nil {
		// nil channel 会永久阻塞
		// 如果parent会永久阻塞,则不必要进行propagate(本函数余下的代码),直接返回
		// 注意: 在context.Done方法的文档中,说到: Done may return nil if this context can never be canceled.
		return // parent is never canceled
	}
	// 现在,parent不会永久阻塞
	if p, ok := parentCancelCtx(parent); ok {
		// 如果从parent开始往下找,找到了一个 *cancelCtx
		p.mu.Lock()
		if p.err != nil {
			// parent has already been canceled
			// p.err != nil : 表示p已经被取消
			// false代表不从parent中移除
			child.cancel(false, p.err)
		} else {
			// 此分支说明p还未被取消
			if p.children == nil {
				p.children = make(map[canceler]struct{})
			}
			// 这就是所谓的propagate,其实就是在p.children的map中增加一个映射
			// 注意: struct{}是类型,struct{}{}是类型的字面量
			p.children[child] = struct{}{}
		}
		p.mu.Unlock()
	} else {
		// 如果从parent开始,没有找到*cancelCtx,
		// 启动一个goroutine,等待parent和child的Done被close
		go func() {
			select {
			case <-parent.Done():
			// parent被取消
			// false代表不从parent中移除
				child.cancel(false, parent.Err())
			case <-child.Done():
			// child被取消
			}
		}()
	}
}

// parentCancelCtx follows a chain of parent references until it finds a
// *cancelCtx. This function understands how each of the concrete types in this
// package represents its parent.
//
// 从parent开始,查找*cancelCtx
//     如果找到了,返回 *cancelCtx, true
//     如果没找到,返回 nil, false
func parentCancelCtx(parent Context) (*cancelCtx, bool) {
	// 不停的循环查找
	for {
		switch c := parent.(type) {
		case *cancelCtx:
			// 如果当前循环的parent就是*cancelCtx
			return c, true
		case *timerCtx:
			// timerCtx结构体内嵌了匿名的cancelCtx,因此cancelCtx的方法对timerCtx可用;因此,timerCtx IS A cancelCtx
			// timerCtx也属于是cancelCtx(也属于context.Context)
			return &c.cancelCtx, true
		case *valueCtx:
			// valueCtx结构体内嵌Context接口,因此valueCtx IS A Context
			// 将parent赋值为子Context,下轮循环使用
			parent = c.Context
		default:
			// 未找到
			return nil, false
		}
	}
}

// removeChild removes a context from its parent.
//
// removeChild会将child从parent中移除
func removeChild(parent Context, child canceler) {
	p, ok := parentCancelCtx(parent)
	if !ok {
		// 如果从parent开始,没有找到*cancelCtx
		return
	}
	// 现在,从parent开始,找到了一个 *cancelCtx
	p.mu.Lock()
	if p.children != nil {
		delete(p.children, child)
	}
	p.mu.Unlock()
}

// A canceler is a context type that can be canceled directly. The
// implementations are *cancelCtx and *timerCtx.
//
// canceler是一个接口,由*cancelCtx和*timerCtx这两个具体的struct进行实现
type canceler interface {
	cancel(removeFromParent bool, err error)
	Done() <-chan struct{}
}

// closedchan is a reusable closed channel.
var closedchan = make(chan struct{})

func init() {
	close(closedchan)
}

// A cancelCtx can be canceled. When canceled, it also cancels any children
// that implement canceler.
type cancelCtx struct {
	// 匿名接口, 因此, cancelCtx IS A Context; 这个字段可以看做parent
	Context

	mu       sync.Mutex            // protects following fields
	done     chan struct{}         // created lazily, closed by first cancel call
	children map[canceler]struct{} // set to nil by the first cancel call
	// err如果是non-nil,表示此cancelCtx已经被cancel
	err      error                 // set to non-nil by the first cancel call
}

// 返回的chan,是cancelCtx.done的值
func (c *cancelCtx) Done() <-chan struct{} {
	c.mu.Lock()
	if c.done == nil {
		c.done = make(chan struct{})
	}
	d := c.done
	c.mu.Unlock()
	return d
}

func (c *cancelCtx) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *cancelCtx) String() string {
	return fmt.Sprintf("%v.WithCancel", c.Context)
}

// cancel closes c.done, cancels each of c's children, and, if
// removeFromParent is true, removes c from its parent's children.
func (c *cancelCtx) cancel(removeFromParent bool, err error) {
	if err == nil {
		panic("context: internal error: missing cancel error")
	}
	c.mu.Lock()
	if c.err != nil {
		// if c.err != nil: 表示已经被取消
		c.mu.Unlock()
		return // already canceled
	}
	// 现在, c还未被取消, 取消它
	c.err = err
	if c.done == nil {
		c.done = closedchan
	} else {
		close(c.done)
	}
	for child := range c.children {
		// NOTE: acquiring the child's lock while holding parent's lock.
		// cancel child 但是不移除 c 和 child 的映射关系
		// 这里其实是个递归调用???
		child.cancel(false, err)
	}
	c.children = nil
	c.mu.Unlock()

	if removeFromParent {
		removeChild(c.Context, c)
	}
}

// WithDeadline returns a copy of the parent context with the deadline adjusted
// to be no later than d. If the parent's deadline is already earlier than d,
// WithDeadline(parent, d) is semantically equivalent to parent. The returned
// context's Done channel is closed when the deadline expires, when the returned
// cancel function is called, or when the parent context's Done channel is
// closed, whichever happens first.
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this Context complete.
//
// semantic /sɪˈmæntɪk/ ADJ Semantic is used to describe things that deal with the meanings of words and sentences. 语义的
// 例： He did not want to enter into a semantic debate. 他不想卷入一场关于语义的争论。
//
// semantically [si'mæntikəli] adv. with regard to meaning 跟意义相关的(跟结构和语法之类的无关)
func WithDeadline(parent Context, deadline time.Time) (Context, CancelFunc) {
	if cur, ok := parent.Deadline(); ok && cur.Before(deadline) {
		// cur:panrent中设置的deadline
		// ok:parent是否设置了deadline
		// 整个分支是是说:如果parent设置了deadline并且在传入
		// 的deadline参数之前,那么本函数返回的Context的deadline
		// 还是以panrent为准
		// The current deadline is already sooner than the new one.
		return WithCancel(parent)
	}
	c := &timerCtx{
		cancelCtx: newCancelCtx(parent),
		deadline:  deadline,
	}
	// propagateCancel arranges for child to be canceled when parent is. 安排当parent被取消时,c也被取消
	propagateCancel(parent, c)
	// d代表还剩余多少时间到deadline
	d := time.Until(deadline)
	if d <= 0 {
		// d <= 0: 说明早已经过了deadline
		c.cancel(true, DeadlineExceeded) // deadline has already passed
		return c, func() { c.cancel(true, Canceled) }
	}
	// 现在,还么有到deadline
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		// 设置定时器,当定时器发生时,进行cancel
		c.timer = time.AfterFunc(d, func() {
			c.cancel(true, DeadlineExceeded)
		})
	}
	return c, func() { c.cancel(true, Canceled) }
}

// A timerCtx carries a timer and a deadline. It embeds a cancelCtx to
// implement Done and Err. It implements cancel by stopping its timer then
// delegating to cancelCtx.cancel.
type timerCtx struct {
	// 匿名struct; 因此, timerCtx IS A cancelCtx, cancelCtx IS A Context 
	cancelCtx
	// *****注意这里源码中的描述*****: Under cancelCtx.mu.
	timer *time.Timer // Under cancelCtx.mu.

	deadline time.Time
}

// 以下是 Context 接口的文档
// Deadline returns the time when work done on behalf of this context
// should be canceled. Deadline returns ok==false when no deadline is
// set. Successive calls to Deadline return the same results.
func (c *timerCtx) Deadline() (deadline time.Time, ok bool) {
	return c.deadline, true
}

func (c *timerCtx) String() string {
	// c.cancelCtx.Context 对于 c 才是 parent
	return fmt.Sprintf("%v.WithDeadline(%s [%s])", c.cancelCtx.Context, c.deadline, time.Until(c.deadline))
}

func (c *timerCtx) cancel(removeFromParent bool, err error) {
	c.cancelCtx.cancel(false, err)
	if removeFromParent {
		// Remove this timerCtx from its parent cancelCtx's children.
		// 对于c来说,parent是c.cancelCtx.Context
		removeChild(c.cancelCtx.Context, c)
	}
	c.mu.Lock()
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.mu.Unlock()
}

// WithTimeout returns WithDeadline(parent, time.Now().Add(timeout)).
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this Context complete:
//
// 	func slowOperationWithTimeout(ctx context.Context) (Result, error) {
// 		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
// 		defer cancel()  // releases resources if slowOperation completes before timeout elapses
// 		return slowOperation(ctx)
// 	}
//
// WithDeadline 是设置一个时间点, WithTimeout 是设置一个时间段,除此之外无区别
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
	return WithDeadline(parent, time.Now().Add(timeout))
}

// WithValue returns a copy of parent in which the value associated with key is
// val.
//
// Use context Values only for request-scoped data that transits processes and
// APIs, not for passing optional parameters to functions.
//
// The provided key must be comparable and should not be of type
// string or any other built-in type to avoid collisions between
// packages using context. Users of WithValue should define their own
// types for keys. To avoid allocating when assigning to an
// interface{}, context keys often have concrete type
// struct{}. Alternatively, exported context key variables' static
// type should be a pointer or interface.
func WithValue(parent Context, key, val interface{}) Context {
	if key == nil {
		panic("nil key")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}
	return &valueCtx{parent, key, val}
}

// A valueCtx carries a key-value pair. It implements Value for that key and
// delegates all other calls to the embedded Context.
//
// valueCtx是根据Context派生出来的
// 其实valueCtx也实现了Context接口,只是Value方法是定义在valueCtx上,其余方法都来源于valueCtx.Context
type valueCtx struct {
	// 内嵌 Context interface, 对应 context.WithValue 函数的 parent 参数
	// 这是 parent
	Context
	key, val interface{}
}

func (c *valueCtx) String() string {
	return fmt.Sprintf("%v.WithValue(%#v, %#v)", c.Context, c.key, c.val)
}

func (c *valueCtx) Value(key interface{}) interface{} {
	if c.key == key {
		// 首先看 child 里面是否有对应设置
		return c.val
	}
	// 再去 parent 中看是否有对应设置
	return c.Context.Value(key)
}
