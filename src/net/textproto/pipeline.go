// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[5-over]]] 2017-7-4 14:06:07

package textproto

import (
	"sync"
)

// A Pipeline manages a pipelined in-order request/response sequence.
//
// in-order: 按次序的,有序的
//
// To use a Pipeline p to manage multiple clients on a connection,
// each client should run:
//
//	id := p.Next()	// take a number
//
//	p.StartRequest(id)	// wait for turn to send request
//	«send request»
//	p.EndRequest(id)	// notify Pipeline that request is sent
//
//	p.StartResponse(id)	// wait for turn to read response
//	«read response»
//	p.EndResponse(id)	// notify Pipeline that response is read
//
// A pipelined server can use the same calls to ensure that
// responses computed in parallel are written in the correct order.
type Pipeline struct {
	mu       sync.Mutex
	id       uint
	request  sequencer
	response sequencer
}

// Next returns the next id for a request/response pair.
func (p *Pipeline) Next() uint {
	p.mu.Lock()
	id := p.id
	p.id++
	p.mu.Unlock()
	return id
}

// StartRequest blocks until it is time to send (or, if this is a server, receive)
// the request with the given id.
//
// 方法名虽然叫StartRequest,其实并不会StartRequest.
// 它只是阻塞,直到id对应的Request可以发送的时候返回.
func (p *Pipeline) StartRequest(id uint) {
	p.request.Start(id)
}

// EndRequest notifies p that the request with the given id has been sent
// (or, if this is a server, received).
//
// 方法名虽然叫EndRequest,其实并不会EndRequest.
// 它只是通知p,id对应的Request已经被发送.
func (p *Pipeline) EndRequest(id uint) {
	p.request.End(id)
}

// StartResponse blocks until it is time to receive (or, if this is a server, send)
// the request with the given id.
func (p *Pipeline) StartResponse(id uint) {
	p.response.Start(id)
}

// EndResponse notifies p that the response with the given id has been received
// (or, if this is a server, sent).
func (p *Pipeline) EndResponse(id uint) {
	p.response.End(id)
}

// A sequencer schedules a sequence of numbered events that must
// happen in order, one after the other. The event numbering must start
// at 0 and increment without skipping. The event number wraps around
// safely as long as there are not 2^32 simultaneous events pending.
type sequencer struct {
	//保护id和wait字段
	mu   sync.Mutex
	// 当前可以Start的id
	id   uint
	// 当前等待的id, map key 是当前等待的id,value是一个chan
	wait map[uint]chan uint
}

// Start waits until it is time for the event numbered id to begin.
// That is, except for the first event, it waits until End(id-1) has
// been called.
func (s *sequencer) Start(id uint) {
	s.mu.Lock()
	if s.id == id {
		// 可以start了,无需阻塞,unlock后函数返回
		s.mu.Unlock()
		return
	}
	// 还未start,需要进行阻塞
	c := make(chan uint)
	if s.wait == nil {
		// 确保map非nil
		s.wait = make(map[uint]chan uint)
	}
	// 设置id和c的对应关系
	s.wait[id] = c
	s.mu.Unlock()
	// 阻塞,直到从chan c中收到可以start的通知(通知是在下面的End方法中发送)
	<-c
}

// End notifies the sequencer that the event numbered id has completed,
// allowing it to schedule the event numbered id+1.  It is a run-time error
// to call End with an id that is not the number of the active event.
func (s *sequencer) End(id uint) {
	// 假设id=2,即想通知2已经结束
	s.mu.Lock()
	if s.id != id {
		// 当前的s.id应该为2
		panic("out of sync")
	}
	id++
	// 现在,id变为3,设置id为3的任务可以Start
	s.id = id
	if s.wait == nil {
		// 确保map非nil
		s.wait = make(map[uint]chan uint)
	}
	// 获取id为3的对应的chan
	c, ok := s.wait[id]
	if ok {
		// 删除id为3和c的对应关系
		delete(s.wait, id)
	}
	s.mu.Unlock()
	if ok {
		// 通知id3可以解除Start中的阻塞
		c <- 1
	}
}
