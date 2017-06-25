// Package google provides a function to do Google searches using the Google Web
// Search API. See https://developers.google.com/web-search/docs/
//
// This package is an example to accompany https://blog.golang.org/context.
// It is not intended for use by others.
//
// Google has since disabled its search API,
// and so this package is no longer useful.
package google

import (
	"encoding/json"
	"net/http"
	"context/blogarticle/userip"
	"context"
)

// Results is an ordered list of search results.
type Results []Result

// A Result contains the title and URL of a search result.
type Result struct {
	Title, URL string
}

// Search sends query to Google search and returns the results.
func Search(ctx context.Context, query string) (Results, error) {
	// Prepare the Google Search API request.
	req, err := http.NewRequest("GET", "https://ajax.googleapis.com/ajax/services/search/web?v=1.0", nil)
	if err != nil {
		// http.NewRequest 出错
		return nil, err
	}
	// Query parses RawQuery and returns the corresponding values.
	// func (u *URL) Query() Values {
	// q 的类型为 url.Values
	q := req.URL.Query()
	// 添加q参数
	q.Set("q", query)

	// If ctx is carrying the user IP address, forward it to the server.
	// Google APIs use the user IP to distinguish server-initiated requests from end-user requests.
	if userIP, ok := userip.FromContext(ctx); ok {
		q.Set("userip", userIP.String())
	}
	// ?? 为什么要做这一步,需要看看 url 包的源码
	req.URL.RawQuery = q.Encode()

	// Issue the HTTP request and handle the response. The httpDo function
	// cancels the request if ctx.Done is closed.
	var results Results
	err = httpDo(ctx, req, func(resp *http.Response, err error) error {
		if err != nil {
			// http 请求出错了
			return err
		}
		// 函数结束时关闭响应body
		defer resp.Body.Close()

		// Parse the JSON search result.
		// https://developers.google.com/web-search/docs/#fonje
		var data struct {
			ResponseData struct {
										 Results []struct {
											 TitleNoFormatting string
											 URL               string
										 }
									 }
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			// json 解析出错
			return err
		}
		for _, res := range data.ResponseData.Results {
			// results是闭包引用的外层变量
			results = append(results, Result{Title: res.TitleNoFormatting, URL: res.URL})
		}
		return nil
	})
	// httpDo waits for the closure we provided to return, so it's safe to read results here.
	// httpDo会等待提供的回调返回,因此这里读取results变量是安全的
	return results, err
}

// httpDo issues the HTTP request and calls f with the response. If ctx.Done is
// closed while the request or f is running, httpDo cancels the request, waits
// for f to exit, and returns ctx.Err. Otherwise, httpDo returns f's error.
func httpDo(ctx context.Context, req *http.Request, f func(*http.Response, error) error) error {
	// Run the HTTP request in a goroutine and pass the response to f.
	tr := &http.Transport{}
	client := &http.Client{Transport: tr}
	
	// 缓冲长度为1的channel
	c := make(chan error, 1)
	// client.Do(req) 说明: func (c *Client) Do(req *Request) (*Response, error) : 进行 http 请求
	// f(client.Do(req)) : 用函数参数f(也是一个函数)处理 client.Do 的返回结果
	// go func() { c <- f(client.Do(req)) }() : 定义了一个匿名函数,然后通过最后的两个()括号进行立即调用
	// 假设这里启动的是 real_request goroutine, real_request 会进行真正的 http 请求,当请求完毕,会向channel c 发送完成数据
	go func() {
		c <- f(client.Do(req)) // 因为缓冲长度为1,这里一定可以立即发送成功
	}()
	
	select {
	case <-ctx.Done():
		// If ctx.Done is closed while the request or f is running, httpDo cancels the request, waits
		// for f to exit, and returns ctx.Err
	
		// CancelRequest cancels an in-flight request by closing its connection.
		// CancelRequest should only be called after RoundTrip has returned.
		//
		// in-flight ['in'flait] adj. 在飞行中的
		//
		// Deprecated: Use Request.Cancel instead. CancelRequest cannot cancel
		// HTTP/2 requests.
		// func (t *Transport) CancelRequest(req *Request) {
		tr.CancelRequest(req)
		// 等待函数f返回
		<-c // Wait for f to return.
		return ctx.Err()
	case err := <-c:
		// Otherwise, httpDo returns f's error
		// 如果f没有出错,也就是err=nil,返回nil
		// 如果f出错,也就是err!=nil,返回非nil
		return err
	}
}
