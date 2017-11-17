// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[3-over]]] 2017-10-9 16:08:50

package sync

import (
	"sync/atomic"
	"unsafe"
)

// Cond implements a condition variable, a rendezvous point
// for goroutines waiting for or announcing the occurrence
// of an event.
//
// Each Cond has an associated Locker L (often a *Mutex or *RWMutex),
// which must be held when changing the condition and
// when calling the Wait method.
//
// A Cond must not be copied after first use.
//
// rendezvous point: 同步点,汇聚点
//
// 那么condition一般用于什么场景?最多的场景是什么?
// 线程A需要等某个条件成立才能继续往下执行,现在这个条件不成立,线程A就阻塞等待,而线程B在
// 执行过程中使这个条件成立了,就唤醒线程A继续执行.在pthread库中通过条件变量来阻塞等待一
// 个条件,或者唤醒等待这个条件的线程.
// 通俗的讲,生产者,消费者的模型.condition很适合那种主动休眠,被动唤醒的场景
//
// 另外,参考: http://hipercomer.blog.51cto.com/4415661/914841
// 另外,参考: http://blog.csdn.net/erickhuang1989/article/details/8754357
// 另外,参考: http://ju.outofmemory.cn/entry/97991
// 另外,参考: http://www.pydevops.com/2016/12/04/go-cond%E6%BA%90%E7%A0%81%E5%89%96%E6%9E%90-3/
// 另外,参考: http://utahcon.com/broadcast-communication-in-golang/
// 另外,参考: https://lycheng.github.io/2016/10/29/golang-sync-package.html
// 另外,参考: http://www.cnblogs.com/golove/p/5918082.html
// 另外,参考: https://my.oschina.net/xinxingegeya/blog/729197
// 另外,参考: http://www.liguosong.com/2014/05/07/golang-sync-cond/
// 另外,参考: https://github.com/polaris1119/The-Golang-Standard-Library-by-Example/blob/master/chapter16/16.01.md
// 另外,参考: https://kaviraj.me/understanding-condition-variable-in-go/
// 另外,参考: http://nanxiao.me/golang-condition-variable/
// 另外,参考: https://deepzz.com/post/golang-sync-package-usage.html
type Cond struct {
	noCopy noCopy

	// L is held while observing or changing the condition
	L Locker

	notify  notifyList
	checker copyChecker
}

// NewCond returns a new Cond with Locker l.
func NewCond(l Locker) *Cond {
	return &Cond{L: l}
}

// Wait atomically unlocks c.L and suspends execution
// of the calling goroutine. After later resuming execution,
// Wait locks c.L before returning. Unlike in other systems,
// Wait cannot return unless awoken by Broadcast or Signal.
//
// Because c.L is not locked when Wait first resumes, the caller
// typically cannot assume that the condition is true when
// Wait returns. Instead, the caller should Wait in a loop:
//
//    c.L.Lock()
//    for !condition() {
//        c.Wait()
//    }
//    ... make use of condition ...
//    c.L.Unlock()
//
// suspend [sə'spend] vt. 延缓，推迟；使暂停；使悬浮 vi. 悬浮；禁赛
// atomically: 原子地
func (c *Cond) Wait() {
	c.checker.check()
	t := runtime_notifyListAdd(&c.notify)
	c.L.Unlock()
	runtime_notifyListWait(&c.notify, t)
	c.L.Lock()
}

// Signal wakes one goroutine waiting on c, if there is any.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
func (c *Cond) Signal() {
	c.checker.check()
	runtime_notifyListNotifyOne(&c.notify)
}

// Broadcast wakes all goroutines waiting on c.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
func (c *Cond) Broadcast() {
	c.checker.check()
	runtime_notifyListNotifyAll(&c.notify)
}

// copyChecker holds back pointer to itself to detect object copying.
type copyChecker uintptr

func (c *copyChecker) check() {
	if uintptr(*c) != uintptr(unsafe.Pointer(c)) &&
		!atomic.CompareAndSwapUintptr((*uintptr)(c), 0, uintptr(unsafe.Pointer(c))) &&
		uintptr(*c) != uintptr(unsafe.Pointer(c)) {
		panic("sync.Cond is copied")
	}
}

// noCopy may be embedded into structs which must not be copied
// after the first use.
//
// See https://github.com/golang/go/issues/8005#issuecomment-190753527
// for details.
type noCopy struct{}

// Lock is a no-op used by -copylocks checker from `go vet`.
func (*noCopy) Lock() {}
