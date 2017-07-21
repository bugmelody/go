// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-7-7 13:10:19

package http

import (
	"net/url"
)

// A CookieJar manages storage and use of cookies in HTTP requests.
//
// CookieJar是给Client使用的,Server并不会用到.
// CookieJar管理cookie的接收存储和请求中的发送.
// 注意: CookieJar仅仅用于http请求场景(Client).
// 应该把CookieJar想象成浏览器用于保存和发送cookie的一套工具.
//
// Implementations of CookieJar must be safe for concurrent use by multiple
// goroutines.
//
// The net/http/cookiejar package provides a CookieJar implementation.
type CookieJar interface {
	// SetCookies handles the receipt of the cookies in a reply for the
	// given URL.  It may or may not choose to save the cookies, depending
	// on the jar's policy and implementation.
	//
	// receipt [rɪ'siːt] n. 收到；收据；收入 vt. 收到
	// -----------------------------------
	// SetCookies管理从u的响应中收到的cookie
	// 根据其策略和实现,它可以选择是否存储cookie
	// 也就是说,SetCookies用于接收http响应中的cookie,并进行存储.
	SetCookies(u *url.URL, cookies []*Cookie)

	// Cookies returns the cookies to send in a request for the given URL.
	// It is up to the implementation to honor the standard cookie use
	// restrictions such as in RFC 6265.
	// 
	// Cookies返回发送请求到u时应使用的cookie.
	// 本方法有责任遵守RFC 6265规定的标准cookie限制.
	// 也就是说,Cookies返回发送http请求时应该发送的cookie.
	Cookies(u *url.URL) []*Cookie
}
