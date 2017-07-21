// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[1-over]]] 2017-7-17 09:21:32

package cookiejar_test

import (
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
)

func ExampleNew() {
	// Start a server to give us cookies.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("Flavor"); err != nil {
			// 如果请求中不存在名为Flavor的cookie,说明是第一次请求
			http.SetCookie(w, &http.Cookie{Name: "Flavor", Value: "Chocolate Chip"})
		} else {
			// 不是第一次请求
			cookie.Value = "Oatmeal Raisin"
			http.SetCookie(w, cookie)
		}
	}))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		log.Fatal(err)
	}

	// All users of cookiejar should import "golang.org/x/net/publicsuffix"
	// 这里并没有实际导入"golang.org/x/net/publicsuffix",而是在测试中定义了一个publicsuffix的简单实现
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{
		Jar: jar,
	}

	// 进行请求
	if _, err = client.Get(u.String()); err != nil {
		log.Fatal(err)
	}

	fmt.Println("After 1st request:")
	for _, cookie := range jar.Cookies(u) {
		// 第1次请求返回的响应给jar注入的cookie
		fmt.Printf("  %s: %s\n", cookie.Name, cookie.Value)
	}

	if _, err = client.Get(u.String()); err != nil {
		log.Fatal(err)
	}

	fmt.Println("After 2nd request:")
	for _, cookie := range jar.Cookies(u) {
		// 第2次请求返回的响应给jar注入的cookie
		fmt.Printf("  %s: %s\n", cookie.Name, cookie.Value)
	}
	// Output:
	// After 1st request:
	//   Flavor: Chocolate Chip
	// After 2nd request:
	//   Flavor: Oatmeal Raisin
}
