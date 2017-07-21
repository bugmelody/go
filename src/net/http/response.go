// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[5-over]]] 2017-7-20 12:33:39

// HTTP Response reading and parsing.

package http

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
)

// Response.Write中会排除的一些header,它们需要有特殊的处理
var respExcludeHeader = map[string]bool{
	"Content-Length":    true,
	"Transfer-Encoding": true,
	"Trailer":           true,
}

// Response represents the response from an HTTP request.
//
type Response struct {
	Status     string // e.g. "200 OK"
	StatusCode int    // e.g. 200
	Proto      string // e.g. "HTTP/1.0"
	ProtoMajor int    // e.g. 1
	ProtoMinor int    // e.g. 0

	// Header maps header keys to values. If the response had multiple
	// headers with the same key, they may be concatenated, with comma
	// delimiters.  (Section 4.2 of RFC 2616 requires that multiple headers
	// be semantically equivalent to a comma-delimited sequence.) When
	// Header values are duplicated by other fields in this struct (e.g.,
	// ContentLength, TransferEncoding, Trailer), the field values are
	// authoritative.
	//
	// Keys in the map are canonicalized (see CanonicalHeaderKey).
	//
	// 下面是RFC 2616的描述
	// Multiple message-header fields with the same field-name MAY be
	// present in a message if and only if the entire field-value for that
	// header field is defined as a comma-separated list [i.e., #(values)].
	// It MUST be possible to combine the multiple header fields into one
	// "field-name: field-value" pair, without changing the semantics of the
	// message, by appending each subsequent field-value to the first, each
	// separated by a comma. The order in which header fields with the same
	// field-name are received is therefore significant to the
	// interpretation of the combined field value, and thus a proxy MUST NOT
	// change the order of these field values when a message is forwarded.
	Header Header

	// Body represents the response body.
	//
	// The http Client and Transport guarantee that Body is always
	// non-nil, even on responses without a body or responses with
	// a zero-length body. It is the caller's responsibility to
	// close Body. The default HTTP client's Transport does not
	// attempt to reuse HTTP/1.0 or HTTP/1.1 TCP connections
	// ("keep-alive") unless the Body is read to completion and is
	// closed.
	//
	// The Body is automatically dechunked if the server replied
	// with a "chunked" Transfer-Encoding.
	//
	// 默认的HTTP client's Transport不会尝试重用 http1.0 或 http1.1 tcp 连接(http长连接), 除非Body被读完并关闭.
	// 这里有个问题,Body读取到一半,就关闭,这样会重用连接吗??
	// chunked v. 分成大块（chunk的过去分词）; dechunked 表示将块组合为一个整体
	Body io.ReadCloser

	// ContentLength records the length of the associated content. The
	// value -1 indicates that the length is unknown. Unless Request.Method
	// is "HEAD", values >= 0 indicate that the given number of bytes may
	// be read from Body.
	ContentLength int64

	// Contains transfer encodings from outer-most to inner-most. Value is
	// nil, means that "identity" encoding is used.
	TransferEncoding []string

	// Close records whether the header directed that the connection be
	// closed after reading Body. The value is advice for clients: neither
	// ReadResponse nor Response.Write ever closes a connection.
	// 
	// 此字段只是server给client的一个建议.
	Close bool

	// Uncompressed reports whether the response was sent compressed but
	// was decompressed by the http package. When true, reading from
	// Body yields the uncompressed content instead of the compressed
	// content actually set from the server, ContentLength is set to -1,
	// and the "Content-Length" and "Content-Encoding" fields are deleted
	// from the responseHeader. To get the original response from
	// the server, set Transport.DisableCompression to true.
	//
	// resp.Uncompressed报告"响应是否以压缩形式发送并且被client解压缩"
	//
	// compressed(压缩)
	// decompressed(解压缩)
	// uncompressed(未压缩的)
	// compressed(压缩的)
	// 这里需要将 decompressed 理解为 解压缩, 将 uncompressed 理解为 未压缩
	// 注意: 前文中的 Uncompressed reports whether ... , 意思是 Uncompressed 仅仅具有报告功能, 而非设置功能.
	Uncompressed bool

	// Trailer maps trailer keys to values in the same
	// format as Header.
	//
	// The Trailer initially contains only nil values, one for
	// each key specified in the server's "Trailer" header
	// value. Those values are not added to Header.
	//
	// Trailer must not be accessed concurrently with Read calls
	// on the Body.
	//
	// After Body.Read has returned io.EOF, Trailer will contain
	// any trailer values sent by the server.
	//
	// with Read calls on the Body(指Response.Body).
	Trailer Header

	// Request is the request that was sent to obtain this Response.
	// Request's Body is nil (having already been consumed).
	// This is only populated for Client requests.
	// Response.Request代表引发Response响应的那个请求
	// 此字段仅适用于client请求server
	Request *Request

	// TLS contains information about the TLS connection on which the
	// response was received. It is nil for unencrypted responses.
	// The pointer is shared between responses and should not be
	// modified.
	TLS *tls.ConnectionState
}

// Cookies parses and returns the cookies set in the Set-Cookie headers.
//
// Cookies解析并返回响应中的Set-Cookie headers.
func (r *Response) Cookies() []*Cookie {
	return readSetCookies(r.Header)
}

// ErrNoLocation is returned by Response's Location method
// when no Location header is present.
var ErrNoLocation = errors.New("http: no Location header in response")

// Location returns the URL of the response's "Location" header,
// if present. Relative redirects are resolved relative to
// the Response's Request. ErrNoLocation is returned if no
// Location header is present.
func (r *Response) Location() (*url.URL, error) {
	lv := r.Header.Get("Location")
	if lv == "" {
		return nil, ErrNoLocation
	}
	if r.Request != nil && r.Request.URL != nil {
		// r.Request: 引发r这个响应的请求
		// r.Request != nil : 说明r是客户端发起请求后收到的响应(而不是server发送给client的resp)
		// r.Request.URL != nil: 可以拿到请求的url
		// 根据Response文档,r.Request字段is only populated for Client requests.
		// r.Request.URL.Parse(lv):基于r.Request.URL对lv进行解析(将相对地址解析为绝对地址)
		return r.Request.URL.Parse(lv)
	}
	return url.Parse(lv)
}

// ReadResponse reads and returns an HTTP response from r.
// The req parameter optionally specifies the Request that corresponds
// to this Response. If nil, a GET request is assumed.
// Clients must call resp.Body.Close when finished reading resp.Body.
// After that call, clients can inspect resp.Trailer to find key/value
// pairs included in the response trailer.
//
// req是指引发Response的请求,可以是nil
// After that call(这个that call是指resp.Body.Close)
func ReadResponse(r *bufio.Reader, req *Request) (*Response, error) {
	tp := textproto.NewReader(r)
	// resp是函数最后要返回的*Response
	resp := &Response{
		// 文档:The req parameter optionally specifies the Request that corresponds
		// to this Response. If nil, a GET request is assumed.
		// 参考Response.Request文档,Response.Request代表引发Response响应的那个请求
		Request: req,
	}

	// Parse the first line of the response.
	// 开始处理请求行
	line, err := tp.ReadLine()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	// 比如: 'HTTP/1.1 200 OK',其中 OK 可能不存在
	if i := strings.IndexByte(line, ' '); i == -1 {
		// 请求行中不存在空格
		return nil, &badStringError{"malformed HTTP response", line}
	} else {
		resp.Proto = line[:i]
		resp.Status = strings.TrimLeft(line[i+1:], " ")
	}
	statusCode := resp.Status
	if i := strings.IndexByte(resp.Status, ' '); i != -1 {
		statusCode = resp.Status[:i]
	}
	if len(statusCode) != 3 {
		// http status code都是3位数字的状态码
		return nil, &badStringError{"malformed HTTP status code", statusCode}
	}
	resp.StatusCode, err = strconv.Atoi(statusCode)
	if err != nil || resp.StatusCode < 0 {
		return nil, &badStringError{"malformed HTTP status code", statusCode}
	}
	var ok bool
	if resp.ProtoMajor, resp.ProtoMinor, ok = ParseHTTPVersion(resp.Proto); !ok {
		return nil, &badStringError{"malformed HTTP version", resp.Proto}
	}

	// Parse the response headers.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	resp.Header = Header(mimeHeader)

	fixPragmaCacheControl(resp.Header)

	err = readTransfer(resp, r)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// RFC 2616: Should treat
//	Pragma: no-cache
// like
//	Cache-Control: no-cache
func fixPragmaCacheControl(header Header) {
	// Cache-Control: no-cache：这个很容易让人产生误解，使人误以为是响应不被缓存。
	// 实际上Cache-Control: no-cache是会被缓存的，只不过每次在向客户端（浏览器）提供响应数据时，缓存都要向服务器评估缓存响应的有效性。
	// Pragma: no-cache：跟Cache-Control: no-cache相同，Pragma: no-cache兼容http 1.0 ，Cache-Control: no-cache是http 1.1提供的。
	// 因此，Pragma: no-cache可以应用到http 1.0 和http 1.1,而Cache-Control: no-cache只能应用于http 1.1.

	// 如果存在 header["Pragma"] 并且值为 "no-cache"
	//    此时如果 header["Cache-Control"] 不存在,则自动设置  header["Cache-Control"] 为 "no-cache"
	//    这样就能同时适应http1.1和1.0
	if hp, ok := header["Pragma"]; ok && len(hp) > 0 && hp[0] == "no-cache" {
		if _, presentcc := header["Cache-Control"]; !presentcc {
			header["Cache-Control"] = []string{"no-cache"}
		}
	}
}

// ProtoAtLeast reports whether the HTTP protocol used
// in the response is at least major.minor.
func (r *Response) ProtoAtLeast(major, minor int) bool {
	return r.ProtoMajor > major ||
		r.ProtoMajor == major && r.ProtoMinor >= minor
}

// Write writes r to w in the HTTP/1.x server response format,
// including the status line, headers, body, and optional trailer.
//
// This method consults the following fields of the response r:
//
//  StatusCode
//  ProtoMajor
//  ProtoMinor
//  Request.Method
//  TransferEncoding
//  Trailer
//  Body
//  ContentLength
//  Header, values for non-canonical keys will have unpredictable behavior
//
// The Response Body is closed after it is sent.
func (r *Response) Write(w io.Writer) error {
	// Status line
	text := r.Status
	if text == "" {
		var ok bool
		text, ok = statusText[r.StatusCode]
		if !ok {
			text = "status code " + strconv.Itoa(r.StatusCode)
		}
	} else {
		// Just to reduce stutter, if user set r.Status to "200 OK" and StatusCode to 200.
		// Not important.
		text = strings.TrimPrefix(text, strconv.Itoa(r.StatusCode)+" ")
	}
	// text现在代表了code描述,如果statusCode是301,则text是"Moved Permanently"

	// %03d:用0进行填充,宽度为3
	if _, err := fmt.Fprintf(w, "HTTP/%d.%d %03d %s\r\n", r.ProtoMajor, r.ProtoMinor, r.StatusCode, text); err != nil {
		return err
	}
	// status line 写入完毕

	// Clone it, so we can modify r1 as needed.
	r1 := new(Response)
	*r1 = *r
	if r1.ContentLength == 0 && r1.Body != nil {
		// Is it actually 0 length? Or just unknown?
		// 尝试只读取一个字节,看看是否 http body 真是长度为0, 还是说处于未知长度的状态
		var buf [1]byte
		n, err := r1.Body.Read(buf[:])
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			// Reset it to a known zero reader, in case underlying one
			// is unhappy being read repeatedly.
			// if n == 0: 说明body确实没有数据
			r1.Body = NoBody
		} else {
			// body有数据,只是长度未知
			// -1 表示长度未知
			r1.ContentLength = -1
			// 重新封装r1.Body,对外之暴露io.Reader,io.Closer两个接口
			r1.Body = struct {
				io.Reader
				io.Closer
			}{
				// body之前已经被读取了1字节,如果再对r.Body进行Read操作是从最老数据的第一个字节后开始
				// 这里使用 io.MultiReader(bytes.NewReader(buf[:1]), r.Body)重新构造完整的数据
				io.MultiReader(bytes.NewReader(buf[:1]), r.Body), // 使用它的Read方法
				r.Body, // 使用它的Close方法
			}
		}
	}
	// If we're sending a non-chunked HTTP/1.1 response without a
	// content-length, the only way to do that is the old HTTP/1.0
	// way, by noting the EOF with a connection close, so we need
	// to set Close.
	if r1.ContentLength == -1 && !r1.Close && r1.ProtoAtLeast(1, 1) && !chunked(r1.TransferEncoding) && !r1.Uncompressed {
		r1.Close = true
	}

	// Process Body,ContentLength,Close,Trailer
	tw, err := newTransferWriter(r1)
	if err != nil {
		return err
	}
	err = tw.WriteHeader(w)
	if err != nil {
		return err
	}

	// Rest of header
	err = r.Header.WriteSubset(w, respExcludeHeader)
	if err != nil {
		return err
	}

	// contentLengthAlreadySent may have been already sent for
	// POST/PUT requests, even if zero length. See Issue 8180.
	contentLengthAlreadySent := tw.shouldSendContentLength()
	if r1.ContentLength == 0 && !chunked(r1.TransferEncoding) && !contentLengthAlreadySent && bodyAllowedForStatus(r.StatusCode) {
		if _, err := io.WriteString(w, "Content-Length: 0\r\n"); err != nil {
			return err
		}
	}

	// End-of-header
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return err
	}

	// Write body and trailer
	err = tw.WriteBody(w)
	if err != nil {
		return err
	}

	// Success
	return nil
}

func (r *Response) closeBody() {
	if r.Body != nil {
		r.Body.Close()
	}
}
