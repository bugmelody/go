// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[5-over]]] 2017-7-14 13:16:15

// HTTP Request reading and parsing.

package http

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"golang_org/x/net/idna"
)

/**
	1KB等于1024B，B是英文Byte(比特)的缩写,KB即kilobyte,字面意思就是千比特。 byte是文件大小的一个计量
	单位，大家都知道在计算机里面，文件都是以二进制方式存储的，这样一个最小的存储单元（譬如10、11、01、00）叫
	做一个bit(位，位元)，八个字节等于一个比特。
	转换关系：
	8bit=1b
	1024byte=1kb
	1024kb=1mb
	1024mb=1gb
	1024gb=1tb
	以上单位k指千、m指百万、g指10亿，t指万亿，大小写均可。 因为1024≈1000，所以1024b,也称为1k，以下类似。
	 */

/**
用php来描述就是
$bytes_array = array(
			'B' => 1,
			'KB' => 1024,
			'MB' => 1024 * 1024,
			'GB' => 1024 * 1024 * 1024,
			'TB' => 1024 * 1024 * 1024 * 1024,
			'PB' => 1024 * 1024 * 1024 * 1024 * 1024,
		);
 */

const (
	// 十进制 256  => 二进制 100000000
	// 十进制 256  => 二进制 100000000
	// 十进制 512  => 二进制 1000000000
	// 十进制 1024 => 二进制 10000000000
	// 十进制 32 => 二进制 100000
	// --------------
	// 
	// 1 byte        => 1
	// 1 kb          => 1 << 10 => 10000000000
	// 可见, 1 << 10 是 1kb, 1 << 20 是 1mb
	// 二进制左移10位 = 十进制 * 1024
	// 32 << 20 是 32 mb
	defaultMaxMemory = 32 << 20 // 32 MB
)

// ErrMissingFile is returned by FormFile when the provided file field name
// is either not present in the request or not a file field.
var ErrMissingFile = errors.New("http: no such file")

// ProtocolError represents an HTTP protocol error.
//
// Deprecated: Not all errors in the http package related to protocol errors
// are of type ProtocolError.
type ProtocolError struct {
	ErrorString string
}

func (pe *ProtocolError) Error() string { return pe.ErrorString }

var (
	// ErrNotSupported is returned by the Push method of Pusher
	// implementations to indicate that HTTP/2 Push support is not
	// available.
	ErrNotSupported = &ProtocolError{"feature not supported"}

	// ErrUnexpectedTrailer is returned by the Transport when a server
	// replies with a Trailer header, but without a chunked reply.
	ErrUnexpectedTrailer = &ProtocolError{"trailer header without chunked transfer encoding"}

	// ErrMissingBoundary is returned by Request.MultipartReader when the
	// request's Content-Type does not include a "boundary" parameter.
	ErrMissingBoundary = &ProtocolError{"no multipart boundary param in Content-Type"}

	// ErrNotMultipart is returned by Request.MultipartReader when the
	// request's Content-Type is not multipart/form-data.
	ErrNotMultipart = &ProtocolError{"request Content-Type isn't multipart/form-data"}

	// Deprecated: ErrHeaderTooLong is not used.
	ErrHeaderTooLong = &ProtocolError{"header too long"}
	// Deprecated: ErrShortBody is not used.
	ErrShortBody = &ProtocolError{"entity body too short"}
	// Deprecated: ErrMissingContentLength is not used.
	ErrMissingContentLength = &ProtocolError{"missing ContentLength in HEAD response"}
)

type badStringError struct {
	what string
	str  string
}

func (e *badStringError) Error() string { return fmt.Sprintf("%s %q", e.what, e.str) }

// Headers that Request.Write handles itself and should be skipped.
//
// 这些header是Request.Write(写请求数据)内部自己处理的头部字段.
var reqWriteExcludeHeader = map[string]bool{
	"Host":              true, // not in Header map anyway
	"User-Agent":        true,
	"Content-Length":    true,
	"Transfer-Encoding": true,
	"Trailer":           true,
}

// A Request represents an HTTP request received by a server
// or to be sent by a client.
//
// The field semantics differ slightly between client and server
// usage. In addition to the notes on the fields below, see the
// documentation for Request.Write and RoundTripper.
type Request struct {
	// Method specifies the HTTP method (GET, POST, PUT, etc.).
	// For client requests an empty string means GET.
	Method string

	// URL specifies either the URI being requested (for server
	// requests) or the URL to access (for client requests).
	//
	// For server requests the URL is parsed from the URI
	// supplied on the Request-Line as stored in RequestURI.  For
	// most requests, fields other than Path and RawQuery will be
	// empty. (See RFC 2616, Section 5.1.2)
	//
	// For client requests, the URL's Host specifies the server to
	// connect to, while the Request's Host field optionally
	// specifies the Host header value to send in the HTTP
	// request.
	//
	// 上文:
	// fields other than Path(指:Request.URL.Path) and RawQuery(指:Request.URL.RawQuery)
	// the URL's Host(指:Request.URL.Host) specifies the server to connect to
	// other than 1.与…不同，非 2.除了
	//
	//
	// URL在服务端表示被请求的URI，在客户端表示要访问的URL.
	//
	// 对于服务端请求来说,URL字段是解析Request-Line的URI(保存在Request.RequestURI字段)得到的,
	// 对大多数请求来说，除了Request.URL.Path和Request.URL.RawQuery之外的字段都是空字符串.
	// （参见RFC 2616, Section 5.1.2）
	//
	// 对于客户端请求来说,Request.URL.Host字段指定了要连接的服务器,
	// 而Request.Host字段（optionally）指定要发送的HTTP请求中的Host头的值.
	// 也就是说, 要连接的服务器和header中的Host头可以不同.
	URL *url.URL

	// The protocol version for incoming server requests.
	//
	// For client requests these fields are ignored. The HTTP
	// client code always uses either HTTP/1.1 or HTTP/2.
	// See the docs on Transport for details.
	Proto      string // "HTTP/1.0"
	ProtoMajor int    // 1
	ProtoMinor int    // 0

	// Header contains the request header fields either received
	// by the server or to be sent by the client.
	//
	// If a server received a request with header lines,
	//
	//	Host: example.com
	//	accept-encoding: gzip, deflate
	//	Accept-Language: en-us
	//	fOO: Bar
	//	foo: two
	//
	// then
	//
	//	Header = map[string][]string{
	//		"Accept-Encoding": {"gzip, deflate"},
	//		"Accept-Language": {"en-us"},
	//		"Foo": {"Bar", "two"},
	//	}
	//
	// For incoming requests, the Host header is promoted to the
	// Request.Host field and removed from the Header map.
	//
	// 对于服务端收到的请求,Host这个header会提升为Request.Host字段,并且从Header map中移除
	//
	// HTTP defines that header names are case-insensitive. The
	// request parser implements this by using CanonicalHeaderKey,
	// making the first character and any characters following a
	// hyphen uppercase and the rest lowercase.
	//
	// For client requests, certain headers such as Content-Length
	// and Connection are automatically written when needed and
	// values in Header may be ignored. See the documentation
	// for the Request.Write method.
	Header Header

	// Body is the request's body.
	//
	// For client requests a nil body means the request has no
	// body, such as a GET request. The HTTP Client's Transport
	// is responsible for calling the Close method.
	//
	// For server requests the Request Body is always non-nil
	// but will return EOF immediately when no body is present.
	// The Server will close the request body. The ServeHTTP
	// Handler does not need to.
	//
	// req.Body总是会被自动关闭
	//   对于client请求,Client's Transport负责关闭.
	//   对于server服务,Server负责关闭,因此ServeHTTP Handler不需要进行关闭.
	Body io.ReadCloser

	// GetBody defines an optional func to return a new copy of
	// Body. It is used for client requests when a redirect requires
	// reading the body more than once. Use of GetBody still
	// requires setting Body.
	//
	// For server requests it is unused.
	GetBody func() (io.ReadCloser, error)

	// ContentLength records the length of the associated content.
	// The value -1 indicates that the length is unknown.
	// Values >= 0 indicate that the given number of bytes may
	// be read from Body.
	// For client requests, a value of 0 with a non-nil Body is
	// also treated as unknown.
	ContentLength int64

	// TransferEncoding lists the transfer encodings from outermost to
	// innermost. An empty list denotes the "identity" encoding.
	// TransferEncoding can usually be ignored; chunked encoding is
	// automatically added and removed as necessary when sending and
	// receiving requests.
	//
	// 传输数据编码：Transfer-Encoding 
	// 数据编码，即表示数据在网络传输当中，使用怎么样的保证方式来保证数据是安全成功地传输处理。
	// 可以是分段传输，也可以是不分段，直接使用原数据进行传输。
	// 有效的值为：Trunked(分段)和Identity(不分段).
	//
	// transfer-encoding:chunked的含义 : http://blog.csdn.net/whatday/article/details/7571451
	TransferEncoding []string

	// Close indicates whether to close the connection after
	// replying to this request (for servers) or after sending this
	// request and reading its response (for clients).
	//
	// 对于server,Close表示reply请求后是否关闭连接.
	// 对于client,Close表示send请求并读取响应后是否关闭连接.
	//
	// For server requests, the HTTP server handles this automatically
	// and this field is not needed by Handlers.
	//
	// 对于服务端请求,HTTP server会自动处理这些关闭逻辑,此时这个字段无用.
	//
	// For client requests, setting this field prevents re-use of
	// TCP connections between requests to the same hosts, as if
	// Transport.DisableKeepAlives were set.
	Close bool

	// For server requests Host specifies the host on which the
	// URL is sought. Per RFC 2616, this is either the value of
	// the "Host" header or the host name given in the URL itself.
	// It may be of the form "host:port". For international domain
	// names, Host may be in Punycode or Unicode form. Use
	// golang.org/x/net/idna to convert it to either format if
	// needed.
	//
	// 关于域名的Punycode: http://tools.jb51.net/punycode/
	// IDNs（国际化域名Internationalized Domain Names）
	//
	// For client requests Host optionally overrides the Host
	// header to send. If empty, the Request.Write method uses
	// the value of URL.Host. Host may contain an international
	// domain name.
	//
	// 上文:For server requests Host specifies the host on which the
	// URL is sought(想象一下一台机器上多个虚拟主机)
	//
	// 对于服务端收到的请求,此Host字段指定在哪个主机上寻找URL.
	// 根据RFC 2616,该值可以是Host头的值，或者URL自身提供的主机名.
	// Host的格式可以是"host:port".
	// 对于客户端发起的请求,此Host字段(可选地)用来重写请求的Host头.
	// 如过该字段为"", Request.Write 方法会使用 Request.URL.Host 的值.
	Host string

	// Form contains the parsed form data, including both the URL
	// field's query parameters and the POST or PUT form data.
	// This field is only available after ParseForm is called.
	// The HTTP client ignores Form and uses Body instead.
	//
	// Request.Form包含了GET和POST
	// 此字段只对于server有用,client会直接使用Body.
	Form url.Values

	// PostForm contains the parsed form data from POST, PATCH,
	// or PUT body parameters.
	//
	// This field is only available after ParseForm is called.
	// The HTTP client ignores PostForm and uses Body instead.
	//
	// Request.PostForm包含POST,不包含url中的query parameters.
	// 此字段只对于server有用,client会直接使用Body.
	PostForm url.Values

	// MultipartForm is the parsed multipart form, including file uploads.
	// This field is only available after ParseMultipartForm is called.
	// The HTTP client ignores MultipartForm and uses Body instead.
	//
	// 此字段只对于server有用,client会直接使用Body.
	MultipartForm *multipart.Form

	// Trailer specifies additional headers that are sent after the request
	// body.
	//
	// For server requests the Trailer map initially contains only the
	// trailer keys, with nil values. (The client declares which trailers it
	// will later send.)  While the handler is reading from Body, it must
	// not reference Trailer. After reading from Body returns EOF, Trailer
	// can be read again and will contain non-nil values, if they were sent
	// by the client.
	//
	// For client requests Trailer must be initialized to a map containing
	// the trailer keys to later send. The values may be nil or their final
	// values. The ContentLength must be 0 or -1, to send a chunked request.
	// After the HTTP request is sent the map values can be updated while
	// the request body is read. Once the body returns EOF, the caller must
	// not mutate Trailer.
	//
	// Few HTTP clients, servers, or proxies support HTTP trailers.
	//
	// HTTP1.1支持chunked transfer，所以可以有Transfer-Encoding头部域:
	// Transfer-Encoding:chunked
	// HTTP1.0则没有。
	// HTTP消息中可以包含任意长度的实体，通常它们使用Content-Length来给出消息结束标志。
	// 但是，对于很多动态产生的响应，只能通过缓冲完整的消息来判断消息的大小，但这样做会加大延迟。
	// 如果不使用长连接，还可以通过连接关闭的信号来判定一个消息的结束。
	// HTTP/1.1中引入了Chunked transfer-coding来解决上面这个问题，发送方将消息分割成若干个任意大小的数据块,每个数据块在发送
	// 时都会附上块的长度,最后用一个零长度的块作为消息结束的标志.这种方法允许发送方只缓冲消息的一个片段,避免缓冲整个消息带来的过载。
	//
	// 在HTTP/1.0中，有一个Content-MD5的头域，要计算这个头域需要发送方缓冲完整个消息后才能进行。而HTTP/1.1中，采用chunked分块传
	// 递的消息在最后一个块（零长度）结束之后会再传递一个拖尾（trailer），它包含一个或多个头域，这些头域是发送方在传递完所有块之后再
	// 计算出值的。发送方会在消息中包含一个Trailer头域告诉接收方这个拖尾的存在。
	Trailer Header

	// RemoteAddr allows HTTP servers and other software to record
	// the network address that sent the request, usually for
	// logging. This field is not filled in by ReadRequest and
	// has no defined format. The HTTP server in this package
	// sets RemoteAddr to an "IP:port" address before invoking a
	// handler.
	// This field is ignored by the HTTP client.
	//
	// 此字段对于client没有意义
	RemoteAddr string

	// RequestURI is the unmodified Request-URI of the
	// Request-Line (RFC 2616, Section 5.1) as sent by the client
	// to a server. Usually the URL field should be used instead.
	// It is an error to set this field in an HTTP client request.
	//
	// RequestURI代表server收到的 Request-Line 中的 Request-URI.
	// 在 HTTP client request 中,不应该设置这个字段
	//
	// 比如: Request.RequestURI == "/favicon.ico"
	RequestURI string

	// TLS allows HTTP servers and other software to record
	// information about the TLS connection on which the request
	// was received. This field is not filled in by ReadRequest.
	// The HTTP server in this package sets the field for
	// TLS-enabled connections before invoking a handler;
	// otherwise it leaves the field nil.
	// This field is ignored by the HTTP client.
	//
	// server使用,client无意义
	TLS *tls.ConnectionState

	// Cancel is an optional channel whose closure indicates that the client
	// request should be regarded as canceled. Not all implementations of
	// RoundTripper may support Cancel.
	//
	// Cancel是一个可选的字段,如果设置了的话,Cancel这
	// 个channel的关闭表示client请求应该被视为已经取消.
	// 不是所有RoundTripper的实现都支持Cancel.
	//
	// For server requests, this field is not applicable.
	// 对于服务端的请求,此字段不适用.
	//
	// Deprecated: Use the Context and WithContext methods
	// instead. If a Request's Cancel field and context are both
	// set, it is undefined whether Cancel is respected.
	Cancel <-chan struct{}

	// Response is the redirect response which caused this request
	// to be created. This field is only populated during client
	// redirects.
	//
	// 引发此Request的响应(通过跳转).
	// 也就是说,对于Request.Response,是Response触发了Request.
	Response *Response

	// ctx is either the client or server context. It should only
	// be modified via copying the whole Request using WithContext.
	// It is unexported to prevent people from using Context wrong
	// and mutating the contexts held by callers of the same request.
	ctx context.Context
}

// Context returns the request's context. To change the context, use
// WithContext.
//
// The returned context is always non-nil; it defaults to the
// background context.
//
// For outgoing client requests, the context controls cancelation.
//
// For incoming server requests, the context is canceled when the
// client's connection closes, the request is canceled (with HTTP/2),
// or when the ServeHTTP method returns.
//
// client requests(客户端请求)
// server requests(服务端服务)
func (r *Request) Context() context.Context {
	// ??为什么与nil作比较??r.ctx类型为context.Context,这是一个interface
	// interface 的 zero value 是 nil
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

// WithContext returns a shallow copy of r with its context changed
// to ctx. The provided ctx must be non-nil.
func (r *Request) WithContext(ctx context.Context) *Request {
	if ctx == nil {
		panic("nil context")
	}
	r2 := new(Request)
	// a shallow copy of r, 浅复制
	*r2 = *r
	// 文档:with its context changed to ctx
	r2.ctx = ctx

	// Deep copy the URL because it isn't
	// a map and the URL is mutable by users
	// of WithContext.
	if r.URL != nil {
		r2URL := new(url.URL)
		*r2URL = *r.URL
		r2.URL = r2URL
	}

	return r2
}

// ProtoAtLeast reports whether the HTTP protocol used
// in the request is at least major.minor.
func (r *Request) ProtoAtLeast(major, minor int) bool {
	return r.ProtoMajor > major ||
		r.ProtoMajor == major && r.ProtoMinor >= minor
}

// UserAgent returns the client's User-Agent, if sent in the request.
func (r *Request) UserAgent() string {
	return r.Header.Get("User-Agent")
}

// Cookies parses and returns the HTTP cookies sent with the request.
//
// Cookies解析并返回此请求发送的所有cookie
func (r *Request) Cookies() []*Cookie {
	// 因为传递了empty的字符串,因此返回了所有的cookie
	return readCookies(r.Header, "")
}

// ErrNoCookie is returned by Request's Cookie method when a cookie is not found.
var ErrNoCookie = errors.New("http: named cookie not present")

// Cookie returns the named cookie provided in the request or
// ErrNoCookie if not found.
// If multiple cookies match the given name, only one cookie will
// be returned.
func (r *Request) Cookie(name string) (*Cookie, error) {
	for _, c := range readCookies(r.Header, name) {
		return c, nil
	}
	return nil, ErrNoCookie
}

// AddCookie adds a cookie to the request. Per RFC 6265 section 5.4,
// AddCookie does not attach more than one Cookie header field. That
// means all cookies, if any, are written into the same line,
// separated by semicolon.
//
// http请求中的Header[Cookie]的格式为:
// 'LAST_LANG=en; LAST_NEWS=1488337564; COUNTRY=NA%2C221.237.152.85'
// 每个cookie之间以'; '分隔
func (r *Request) AddCookie(c *Cookie) {
	s := fmt.Sprintf("%s=%s", sanitizeCookieName(c.Name), sanitizeCookieValue(c.Value))
	// 文档:Get("Cookie"):If there are no values associated with the key, Get returns ""
	if c := r.Header.Get("Cookie"); c != "" {
		// 如果r.Header中已经有Cookie头,进行追加
		r.Header.Set("Cookie", c+"; "+s)
	} else {
		// 否则,是直接设置
		r.Header.Set("Cookie", s)
	}
}

// Referer returns the referring URL, if sent in the request.
//
// Referer is misspelled as in the request itself, a mistake from the
// earliest days of HTTP.  This value can also be fetched from the
// Header map as Header["Referer"]; the benefit of making it available
// as a method is that the compiler can diagnose programs that use the
// alternate (correct English) spelling req.Referrer() but cannot
// diagnose programs that use Header["Referrer"].
//
// Referer其实是一个错误拼写,正确应该写为Referrer,这
// 里提供此方法是为了让编译器检查拼写错误
func (r *Request) Referer() string {
	return r.Header.Get("Referer")
}

// multipartByReader is a sentinel value.
// Its presence in Request.MultipartForm indicates that parsing of the request
// body has been handed off to a MultipartReader instead of ParseMultipartFrom.
//
// 如果Request.MultipartForm==multipartByReader,表
// 示解析请求body的任务已经交给MultipartReader
//
// sentinel value: 标志值；标记值；警示值
// handed off to:交给
var multipartByReader = &multipart.Form{
	Value: make(map[string][]string),
	File:  make(map[string][]*multipart.FileHeader),
}

// MultipartReader returns a MIME multipart reader if this is a
// multipart/form-data POST request, else returns nil and an error.
// Use this function instead of ParseMultipartForm to
// process the request body as a stream.
//
// 如果使用ParseMultipartForm方法则一次性的对表单数据进行解析
// 如果使用MultipartReader返回的multipart.Reader,可以对表单数据进行流式处理.
//
// 什么是 MIME:
// http://zhidao.baidu.com/link?url=YDg8bUoHKRg4B2vOMWIk4jrP3QNjhnzOqIJWZr2eyA8E9kQgxynyWtoHaJzynjV6QaoU7_sM-6dUWSBZBaOGca
//
// 表单中enctype="multipart/form-data"的意思,是设置表单的MIME编码.
// 默认情况，这个编码格式是application/x-www-form-urlencoded,不能用于文件上传.
// 只有使用了multipart/form-data，才能完整的传递文件数据
//
// @see
func (r *Request) MultipartReader() (*multipart.Reader, error) {
	if r.MultipartForm == multipartByReader {
		// if r.MultipartForm==multipartByReader,表示解析请求body的
		// 任务已经转移,因此MultipartReader方法不能调用超过1次
		return nil, errors.New("http: MultipartReader called twice")
	}
	// 现在,r.MultipartForm!=multipartByReader,也就是本方法还没有调用过
	if r.MultipartForm != nil {
		// 此时如果不为nil,说明已经被ParseMultipartForm处理
		return nil, errors.New("http: multipart handled by ParseMultipartForm")
	}
	// 设置已经转移标志
	r.MultipartForm = multipartByReader
	return r.multipartReader()
}

/**
http post 例子:

POST /sso/loginUser/uploadLogo?qcdebug=1 HTTP/1.1
Host: i.ludashi.com
User-Agent: Go-http-client/1.1
Content-Length: 1639
Content-Type: multipart/form-data; boundary=9e4333274ca910d7f21776c84733a592aee6de4a8d848632bc0c5ba42db2
Accept-Encoding: gzip

--9e4333274ca910d7f21776c84733a592aee6de4a8d848632bc0c5ba42db2
Content-Disposition: form-data; name="logo"; filename="upload_logo_test.go"
Content-Type: application/octet-stream

文件内容

--9e4333274ca910d7f21776c84733a592aee6de4a8d848632bc0c5ba42db2
Content-Disposition: form-data; name="reqjson"

reqjson字段的值
--9e4333274ca910d7f21776c84733a592aee6de4a8d848632bc0c5ba42db2--
 */

// 根据http header信息获取一个 multipart.Reader 对象
// 文件上传时,会传递请求头: 'Content-Type: multipart/form-data; boundary=随机数'
// @see
func (r *Request) multipartReader() (*multipart.Reader, error) {
	v := r.Header.Get("Content-Type")
	if v == "" {
		// 文档:request Content-Type isn't multipart/form-data
		return nil, ErrNotMultipart
	}
	d, params, err := mime.ParseMediaType(v)
	if err != nil || d != "multipart/form-data" {
		// 文档:request Content-Type isn't multipart/form-data
		return nil, ErrNotMultipart
	}
	boundary, ok := params["boundary"]
	if !ok {
		// 文档:no multipart boundary param in Content-Type
		return nil, ErrMissingBoundary
	}
	return multipart.NewReader(r.Body, boundary), nil
}

// isH2Upgrade reports whether r represents the http2 "client preface"
// magic string.
func (r *Request) isH2Upgrade() bool {
	return r.Method == "PRI" && len(r.Header) == 0 && r.URL.Path == "*" && r.Proto == "HTTP/2.0"
}

// Return value if nonempty, def otherwise.
//
// def 是 default 的缩写.
// @see
func valueOrDefault(value, def string) string {
	if value != "" {
		return value
	}
	return def
}

// NOTE: This is not intended to reflect the actual Go version being used.
// It was changed at the time of Go 1.1 release because the former User-Agent
// had ended up on a blacklist for some intrusion detection systems.
// See https://codereview.appspot.com/7532043.
const defaultUserAgent = "Go-http-client/1.1"

// Write writes an HTTP/1.1 request, which is the header and body, in wire format.
// This method consults the following fields of the request:
//	Host
//	URL
//	Method (defaults to "GET")
//	Header
//	ContentLength
//	TransferEncoding
//	Body
//
// If Body is present, Content-Length is <= 0 and TransferEncoding
// hasn't been set to "identity", Write adds "Transfer-Encoding:
// chunked" to the header. Body is closed after it is sent.
func (r *Request) Write(w io.Writer) error {
	return r.write(w, false, nil, nil)
}

// WriteProxy is like Write but writes the request in the form
// expected by an HTTP proxy. In particular, WriteProxy writes the
// initial Request-URI line of the request with an absolute URI, per
// section 5.1.2 of RFC 2616, including the scheme and host.
// In either case, WriteProxy also writes a Host header, using
// either r.Host or r.URL.Host.
func (r *Request) WriteProxy(w io.Writer) error {
	return r.write(w, true, nil, nil)
}

// errMissingHost is returned by Write when there is no Host or URL present in
// the Request.
var errMissingHost = errors.New("http: Request.Write on Request with no Host or URL set")

// usingProxy:该请求是否使用代理
// extraHeaders may be nil
// waitForContinue may be nil
func (r *Request) write(w io.Writer, usingProxy bool, extraHeaders Header, waitForContinue func() bool) (err error) {
	trace := httptrace.ContextClientTrace(r.Context())
	if trace != nil && trace.WroteRequest != nil {
		defer func() {
			trace.WroteRequest(httptrace.WroteRequestInfo{
				Err: err,
			})
		}()
	}

	// Find the target host. Prefer the Host: header, but if that
	// is not given, use the host from the request URL.
	//
	// Clean the host, in case it arrives with unexpected stuff in it.
	host := cleanHost(r.Host)
	if host == "" {
		if r.URL == nil {
			return errMissingHost
		}
		host = cleanHost(r.URL.Host)
	}

	// According to RFC 6874, an HTTP client, proxy, or other
	// intermediary must remove any IPv6 zone identifier attached
	// to an outgoing URI.
	host = removeZone(host)

	ruri := r.URL.RequestURI()
	if usingProxy && r.URL.Scheme != "" && r.URL.Opaque == "" {
		ruri = r.URL.Scheme + "://" + host + ruri
	} else if r.Method == "CONNECT" && r.URL.Path == "" {
		// CONNECT requests normally give just the host and port, not a full URL.
		// 重设 ruri
		// http CONNECT:把请求连接转换到透明的TCP/IP通道。
		ruri = host
	}
	// TODO(bradfitz): escape at least newlines in ruri?

	// Wrap the writer in a bufio Writer if it's not already buffered.
	// Don't always call NewWriter, as that forces a bytes.Buffer
	// and other small bufio Writers to have a minimum 4k buffer
	// size.
	var bw *bufio.Writer
	if _, ok := w.(io.ByteWriter); !ok {
		// 如果 w 实现了 io.ByteWriter 接口, 将 w 重新设置为 bufio.Writer

		/**
		注意: bufio.NewWriter
		// NewWriter returns a new Writer whose buffer has the default size.
		func NewWriter(w io.Writer) *Writer {
		这里所谓的 default size 在 bufio 包中定义为 4096字节,也就是 4k
		
		bufio.Writer 实现了 io.ByteWriter
		 */
		bw = bufio.NewWriter(w)
		w = bw
	}
	// 现在 w 肯定实现了 io.ByteWriter 接口, 也就是拥有  WriteByte(c byte) error 方法

	_, err = fmt.Fprintf(w, "%s %s HTTP/1.1\r\n", valueOrDefault(r.Method, "GET"), ruri)
	if err != nil {
		return err
	}

	// ++++写入 Host Header
	// Header lines
	_, err = fmt.Fprintf(w, "Host: %s\r\n", host)
	if err != nil {
		return err
	}

	// Use the defaultUserAgent unless the Header contains one, which
	// may be blank to not send the header.
	userAgent := defaultUserAgent
	if _, ok := r.Header["User-Agent"]; ok {
		userAgent = r.Header.Get("User-Agent")
	}
	if userAgent != "" {
		// 不为空的时候才发送 User-Agent 头, 如果为空,则不发送
		_, err = fmt.Fprintf(w, "User-Agent: %s\r\n", userAgent)
		if err != nil {
			return err
		}
	}

	// Process Body,ContentLength,Close,Trailer
	tw, err := newTransferWriter(r)
	if err != nil {
		return err
	}
	// 写入部分header,这些header是自动计算
	err = tw.WriteHeader(w)
	if err != nil {
		return err
	}

// 写入部分header,这些header是req.Header减去reqWriteExcludeHeader
	err = r.Header.WriteSubset(w, reqWriteExcludeHeader)
	if err != nil {
		return err
	}

	// 写入额外header
	if extraHeaders != nil {
		err = extraHeaders.Write(w)
		if err != nil {
			return err
		}
	}

	// http header写完, 补上与http body之间的分隔符 \r\n
	_, err = io.WriteString(w, "\r\n")
	if err != nil {
		return err
	}

	if trace != nil && trace.WroteHeaders != nil {
		trace.WroteHeaders()
	}

	/**
	8.2.3 Use of the 100 (Continue) Status
The purpose of the 100 (Continue) status (see section 10.1.1) is to allow a client that is sending a request message with a request body to determine if the origin server is willing to accept the request (based on the request headers) before the client sends the request body. In some cases, it might either be inappropriate or highly inefficient for the client to send the body if the server will reject the message without looking at the body.
Requirements for HTTP/1.1 clients:
– If a client will wait for a 100 (Continue) response before
sending the request body, it MUST send an Expect request-header
field (section 14.20) with the “100-continue” expectation.
– A client MUST NOT send an Expect request-header field (section
14.20) with the “100-continue” expectation if it does not intend
to send a request body.

简单翻译一下：
使用100（不中断，继续）状态码的目的是为了在客户端发出请求体之前，让服务器根据客户端发出的请求信息（根据请求的头信息）来决定是否愿意接受来自客户端的包含了请求内容的请求；在某些情况下，在有些情况下，如果服务器拒绝查看消息主体，这时客户端发送消息主体是不合适的或会降低效率

对HTTP/1.1客户端的要求：
-如果客户端在发送请求体之前，想等待服务器返回100状态码，那么客户端必须要发送一个Expect请求头信息，即：”100-continue”请求头信息；

-如果一个客户端不打算发送请求体的时候，一定不要使用“100-continue”发送Expect的请求头信息；
	 */
	
	// Flush and wait for 100-continue if expected.
	if waitForContinue != nil {
		// waitForContinue 是函数参数,类型是函数 waitForContinue func() bool
		if bw, ok := w.(*bufio.Writer); ok {
			err = bw.Flush()
			if err != nil {
				return err
			}
		}
		if trace != nil && trace.Wait100Continue != nil {
			trace.Wait100Continue()
		}
		if !waitForContinue() {
			// 如果waitForContinue返回false,关闭req.Body,函数返回,根本不会写body
			r.closeBody()
			return nil
		}
	}

	if bw, ok := w.(*bufio.Writer); ok && tw.FlushHeaders {
		if err := bw.Flush(); err != nil {
			return err
		}
	}

	// ++++写入 Body 和 Trailer
	// Write body and trailer
	err = tw.WriteBody(w)
	if err != nil {
		if tw.bodyReadError == err {
			err = requestBodyReadError{err}
		}
		return err
	}

	if bw != nil {
		return bw.Flush()
	}
	return nil
}

// requestBodyReadError wraps an error from (*Request).write to indicate
// that the error came from a Read call on the Request.Body.
// This error type should not escape the net/http package to users.
type requestBodyReadError struct{ error }

func idnaASCII(v string) (string, error) {
	// TODO: Consider removing this check after verifying performance is okay.
	// Right now punycode verification, length checks, context checks, and the
	// permissible character tests are all omitted. It also prevents the ToASCII
	// call from salvaging an invalid IDN, when possible. As a result it may be
	// possible to have two IDNs that appear identical to the user where the
	// ASCII-only version causes an error downstream whereas the non-ASCII
	// version does not.
	// Note that for correct ASCII IDNs ToASCII will only do considerably more
	// work, but it will not cause an allocation.
	if isASCII(v) {
		return v, nil
	}
	return idna.Lookup.ToASCII(v)
}

// cleanHost cleans up the host sent in request's Host header.
//
// It both strips anything after '/' or ' ', and puts the value
// into Punycode form, if necessary.
//
// Ideally we'd clean the Host header according to the spec:
//   https://tools.ietf.org/html/rfc7230#section-5.4 (Host = uri-host [ ":" port ]")
//   https://tools.ietf.org/html/rfc7230#section-2.7 (uri-host -> rfc3986's host)
//   https://tools.ietf.org/html/rfc3986#section-3.2.2 (definition of host)
// But practically, what we are trying to avoid is the situation in
// issue 11206, where a malformed Host header used in the proxy context
// would create a bad request. So it is enough to just truncate at the
// first offending character.
//
// @notsee
func cleanHost(in string) string {
	if i := strings.IndexAny(in, " /"); i != -1 {
		in = in[:i]
	}
	host, port, err := net.SplitHostPort(in)
	if err != nil { // input was just a host
		a, err := idnaASCII(in)
		if err != nil {
			return in // garbage in, garbage out
		}
		return a
	}
	a, err := idnaASCII(host)
	if err != nil {
		return in // garbage in, garbage out
	}
	return net.JoinHostPort(a, port)
}

// removeZone removes IPv6 zone identifier from host.
// E.g., "[fe80::1%en0]:8080" to "[fe80::1]:8080"
func removeZone(host string) string {
	if !strings.HasPrefix(host, "[") {
		// 没有以 "[" 开头
		return host
	}
	i := strings.LastIndex(host, "]")
	if i < 0 {
		// 不是以 "]" 结尾
		return host
	}
	// 最后一个 % 的位置
	j := strings.LastIndex(host[:i], "%")
	if j < 0 {
		// 不存在 %
		return host
	}
	return host[:j] + host[i:]
}

// ParseHTTPVersion parses a HTTP version string.
// "HTTP/1.0" returns (1, 0, true).
//
// ok==false表示解析失败
//
// @notsee
func ParseHTTPVersion(vers string) (major, minor int, ok bool) {
	const Big = 1000000 // arbitrary upper bound
	switch vers {
	case "HTTP/1.1":
		return 1, 1, true
	case "HTTP/1.0":
		return 1, 0, true
	}
	if !strings.HasPrefix(vers, "HTTP/") {
		// 不是以 "HTTP/" 开头
		return 0, 0, false
	}
	// 现在， 是以 "HTTP/" 开头
	// dot 代表点号位置
	dot := strings.Index(vers, ".")
	if dot < 0 {
		// 不含点号
		return 0, 0, false
	}
	// 现在， 包含点号
	major, err := strconv.Atoi(vers[5:dot])
	if err != nil || major < 0 || major > Big {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(vers[dot+1:])
	if err != nil || minor < 0 || minor > Big {
		return 0, 0, false
	}
	return major, minor, true
}

// 判断是否是合法的http方法
func validMethod(method string) bool {
	/*
	     Method         = "OPTIONS"                ; Section 9.2
	                    | "GET"                    ; Section 9.3
	                    | "HEAD"                   ; Section 9.4
	                    | "POST"                   ; Section 9.5
	                    | "PUT"                    ; Section 9.6
	                    | "DELETE"                 ; Section 9.7
	                    | "TRACE"                  ; Section 9.8
	                    | "CONNECT"                ; Section 9.9
	                    | extension-method
	   extension-method = token
	     token          = 1*<any CHAR except CTLs or separators>
	*/
	return len(method) > 0 && strings.IndexFunc(method, isNotToken) == -1
}

// NewRequest returns a new Request given a method, URL, and optional body.
//
// If the provided body is also an io.Closer, the returned
// Request.Body is set to body and will be closed by the Client
// methods Do, Post, and PostForm, and Transport.RoundTrip.
//
// 此时自动Close
//
// NewRequest returns a Request suitable for use with Client.Do or
// Transport.RoundTrip. To create a request for use with testing a
// Server Handler, either use the NewRequest function in the
// net/http/httptest package, use ReadRequest, or manually update the
// Request fields. See the Request type's documentation for the
// difference between inbound and outbound request fields.
//
// If body is of type *bytes.Buffer, *bytes.Reader, or
// *strings.Reader, the returned request's ContentLength is set to its
// exact value (instead of -1), GetBody is populated (so 307 and 308
// redirects can replay the body), and Body is set to NoBody if the
// ContentLength is 0.
//
// 根据 Request 的文档,如果 body 参数是 nil,表示请求时不需要 http body
// 比如: req, err := NewRequest("GET", url, nil)
func NewRequest(method, url string, body io.Reader) (*Request, error) {
	if method == "" {
		// We document that "" means "GET" for Request.Method, and people have
		// relied on that from NewRequest, so keep that working.
		// We still enforce validMethod for non-empty methods.
		method = "GET"
	}
	if !validMethod(method) {
		return nil, fmt.Errorf("net/http: invalid method %q", method)
	}
	u, err := parseURL(url) // Just url.Parse (url is shadowed for godoc).
	if err != nil {
		return nil, err
	}
	rc, ok := body.(io.ReadCloser)
	if !ok && body != nil {
		// 如果body参数没有实现ReadCloser接口
		// 由于在函数签名中指定了body是io.Reader类型,因此如果这里不是ReadCloser接口,实际是说明body没有实现Close方法
		// 这里的意思是如果 body 没有实现 Close 方法, 则通过 ioutil.NopCloser 添加 Close 方法
		rc = ioutil.NopCloser(body)
	}
	// The host's colon:port should be normalized. See Issue 14836.
	u.Host = removeEmptyPort(u.Host)
	// 开始构造要返回的 *Request 对象，固定使用http 1.1，
	req := &Request{
		Method:     method,
		URL:        u,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(Header),
		Body:       rc,
		Host:       u.Host,
	}
	// 这个body是整个函数的参数
	// 以下3个case的v.Len,看文档,都是返回未读区域的长度
	if body != nil {
		switch v := body.(type) {
		case *bytes.Buffer:
			req.ContentLength = int64(v.Len())
			buf := v.Bytes()
			req.GetBody = func() (io.ReadCloser, error) {
				r := bytes.NewReader(buf)
				return ioutil.NopCloser(r), nil
			}
		case *bytes.Reader:
			req.ContentLength = int64(v.Len())
			snapshot := *v
			req.GetBody = func() (io.ReadCloser, error) {
				r := snapshot
				return ioutil.NopCloser(&r), nil
			}
		case *strings.Reader:
			req.ContentLength = int64(v.Len())
			snapshot := *v
			req.GetBody = func() (io.ReadCloser, error) {
				r := snapshot
				return ioutil.NopCloser(&r), nil
			}
		default:
			// This is where we'd set it to -1 (at least
			// if body != NoBody) to mean unknown, but
			// that broke people during the Go 1.8 testing
			// period. People depend on it being 0 I
			// guess. Maybe retry later. See Issue 18117.
		}
		// For client requests, Request.ContentLength of 0
		// means either actually 0, or unknown. The only way
		// to explicitly say that the ContentLength is zero is
		// to set the Body to nil. But turns out too much code
		// depends on NewRequest returning a non-nil Body,
		// so we use a well-known ReadCloser variable instead
		// and have the http package also treat that sentinel
		// variable to mean explicitly zero.
		if req.GetBody != nil && req.ContentLength == 0 {
			req.Body = NoBody
			req.GetBody = func() (io.ReadCloser, error) { return NoBody, nil }
		}
	}

	return req, nil
}

// BasicAuth returns the username and password provided in the request's
// Authorization header, if the request uses HTTP Basic Authentication.
// See RFC 2617, Section 2.
//
// 从 Header 中获取认证信息
func (r *Request) BasicAuth() (username, password string, ok bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return
	}
	return parseBasicAuth(auth)
}

// parseBasicAuth parses an HTTP Basic Authentication string.
// "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==" returns ("Aladdin", "open sesame", true).
func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		// 如果不是以 "Basic " 开头
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		// 不存在冒号
		return
	}
	// s现在代表:位移
	return cs[:s], cs[s+1:], true
}

// SetBasicAuth sets the request's Authorization header to use HTTP
// Basic Authentication with the provided username and password.
//
// With HTTP Basic Authentication the provided username and password
// are not encrypted.
func (r *Request) SetBasicAuth(username, password string) {
	r.Header.Set("Authorization", "Basic "+basicAuth(username, password))
}

// parseRequestLine parses "GET /foo HTTP/1.1" into its three parts.
// 将请求行解析为 method, requestURI, proto 三个部分
func parseRequestLine(line string) (method, requestURI, proto string, ok bool) {
	// s1: space 1 position,相对于 "^GET /foo HTTP/1.1"
	s1 := strings.Index(line, " ")
	// s2: space 2 position,相对于 "GET ^/foo HTTP/1.1"
	s2 := strings.Index(line[s1+1:], " ")
	if s1 < 0 || s2 < 0 {
		// 只有一个空格,或者没有空格
		return
	}
	// 现在, line 中有两个空格, s1 和 s2 肯定都 >= 0
	// s1 是第一个空格位置,相对于 "^GET /foo HTTP/1.1"
	// s2 是第二个空格位置,相对于 "GET ^/foo HTTP/1.1"

	// 让 s2 相对于 GET 起点
	s2 += s1 + 1
	// 现在, s1 和 s2 都是相对于 "^GET /foo HTTP/1.1" 的空格1 空格2 位置
	return line[:s1], line[s1+1 : s2], line[s2+1:], true
	//     GET        /foo             HTTP/1.1
}

var textprotoReaderPool sync.Pool

func newTextprotoReader(br *bufio.Reader) *textproto.Reader {
	if v := textprotoReaderPool.Get(); v != nil {
		// 从pool中获取到了
		// v的实际底层类型是 *textproto.Reader
		tr := v.(*textproto.Reader)
		tr.R = br
		return tr
	}
	// 没有从pool中获取到
	return textproto.NewReader(br)
}

func putTextprotoReader(r *textproto.Reader) {
	r.R = nil
	textprotoReaderPool.Put(r)
}

// ReadRequest reads and parses an incoming request from b.
func ReadRequest(b *bufio.Reader) (*Request, error) {
	return readRequest(b, deleteHostHeader)
}

// Constants for readRequest's deleteHostHeader parameter.
const (
	deleteHostHeader = true
	keepHostHeader   = false
)

// 如果 deleteHostHeader==true,将会进行 delete(req.Header, "Host") 操作
// @see
func readRequest(b *bufio.Reader, deleteHostHeader bool) (req *Request, err error) {
	// 使用b得到一个textproto.Reader,可能是从pool中得到的
	tp := newTextprotoReader(b)
	// 本函数最后的返回结果
	req = new(Request)

	// First line: GET /index.html HTTP/1.0
	var s string
	if s, err = tp.ReadLine(); err != nil {
		return nil, err
	}
	// 现在,读取first line没有出错
	defer func() {
		// 确保本函数返回时一定将tp归还pool
		putTextprotoReader(tp)
		if err == io.EOF {
			// 修改命名返回值
			err = io.ErrUnexpectedEOF
		}
	}()

	var ok bool
	// 现在, 请求行的数据已经被读取到s,解析请求行
	req.Method, req.RequestURI, req.Proto, ok = parseRequestLine(s)
	if !ok {
		// 解析请求行失败
		return nil, &badStringError{"malformed HTTP request", s}
	}
	if !validMethod(req.Method) {
		// 请求行中的请求方法不合法
		return nil, &badStringError{"invalid method", req.Method}
	}
	// rawurl是根据RequestURI解析出来的
	rawurl := req.RequestURI
	if req.ProtoMajor, req.ProtoMinor, ok = ParseHTTPVersion(req.Proto); !ok {
		// 解析http version失败
		return nil, &badStringError{"malformed HTTP version", req.Proto}
	}

	// CONNECT requests are used two different ways, and neither uses a full URL:
	// The standard use is to tunnel HTTPS through an HTTP proxy.
	// It looks like "CONNECT www.google.com:443 HTTP/1.1", and the parameter is
	// just the authority section of a URL. This information should go in req.URL.Host.
	//
	// The net/rpc package also uses CONNECT, but there the parameter is a path
	// that starts with a slash. It can be parsed with the regular URL parser,
	// and the path will end up in req.URL.Path, where it needs to be in order for
	// RPC to work.
	justAuthority := req.Method == "CONNECT" && !strings.HasPrefix(rawurl, "/")
	if justAuthority {
		rawurl = "http://" + rawurl
	}

	// req.URL是根据请求行解析出来的
	if req.URL, err = url.ParseRequestURI(rawurl); err != nil {
		return nil, err
	}

	if justAuthority {
		// Strip the bogus "http://" back off.
		// bogus ['bəʊgəs] adj. 假的；伪造的 	n. 伪币
		req.URL.Scheme = ""
	}

	// Subsequent lines: Key: value.
	// 读取头部
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	// 底层类型都是map[string][]string,因此可用Header(mimeHeader)进行类型转换
	req.Header = Header(mimeHeader)

	// RFC 2616: Must treat
	//	GET /index.html HTTP/1.1
	//	Host: www.google.com
	// and
	//	GET http://www.google.com/index.html HTTP/1.1
	//	Host: doesntmatter
	// the same. In the second case, any Host line is ignored.
	// req.URL是根据请求行解析出来的
	req.Host = req.URL.Host
	if req.Host == "" {
		// 当设置req.Host的时候,优先使用请求行中的host,如果请求行中不存在host,再考虑使用Header[Host]
		req.Host = req.Header.get("Host")
	}
	if deleteHostHeader {
		delete(req.Header, "Host")
	}

	fixPragmaCacheControl(req.Header)

	req.Close = shouldClose(req.ProtoMajor, req.ProtoMinor, req.Header, false)

	err = readTransfer(req, b)
	if err != nil {
		return nil, err
	}

	if req.isH2Upgrade() {
		// Because it's neither chunked, nor declared:
		req.ContentLength = -1

		// We want to give handlers a chance to hijack the
		// connection, but we need to prevent the Server from
		// dealing with the connection further if it's not
		// hijacked. Set Close to ensure that:
		req.Close = true
	}
	return req, nil
}

// MaxBytesReader is similar to io.LimitReader but is intended for
// limiting the size of incoming request bodies. In contrast to
// io.LimitReader, MaxBytesReader's result is a ReadCloser, returns a
// non-EOF error for a Read beyond the limit, and closes the
// underlying reader when its Close method is called.
//
// MaxBytesReader prevents clients from accidentally or maliciously
// sending a large request and wasting server resources.
//
// maliciously [mə'lɪʃəsli] adv. 有敌意地，恶意地
// 来对比一下: $ go doc io.LimitReader
// func LimitReader(r Reader, n int64) Reader
// LimitReader returns a Reader that reads from r but stops with EOF after n
// bytes. The underlying implementation is a *LimitedReader.
func MaxBytesReader(w ResponseWriter, r io.ReadCloser, n int64) io.ReadCloser {
	return &maxBytesReader{w: w, r: r, n: n}
}

type maxBytesReader struct {
	w   ResponseWriter
	r   io.ReadCloser // underlying reader
	n   int64         // max bytes remaining
	err error         // sticky error
}

func (l *maxBytesReader) Read(p []byte) (n int, err error) {
	if l.err != nil {
		return 0, l.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	// If they asked for a 32KB byte read but only 5 bytes are
	// remaining, no need to read 32KB. 6 bytes will answer the
	// question of the whether we hit the limit or go past it.
	if int64(len(p)) > l.n+1 {
		p = p[:l.n+1]
	}
	n, err = l.r.Read(p)

	if int64(n) <= l.n {
		// 读取到的字节数 <= 剩余限制字节数
		l.n -= int64(n)
		l.err = err
		return n, err
	}

	// 到了这里,说明l.r.Read(p)超过了希望的限制
	// n是函数返回值,这里修改n为l.n(整个剩余数据字节数),模拟l.r被读取完的效果
	n = int(l.n)
	l.n = 0

	// The server code and client code both use
	// maxBytesReader. This "requestTooLarge" check is
	// only used by the server code. To prevent binaries
	// which only using the HTTP Client code (such as
	// cmd/go) from also linking in the HTTP server, don't
	// use a static type assertion to the server
	// "*response" type. Check this interface instead:
	type requestTooLarger interface {
		requestTooLarge()
	}
	if res, ok := l.w.(requestTooLarger); ok {
		res.requestTooLarge()
	}
	l.err = errors.New("http: request body too large")
	return n, l.err
}

func (l *maxBytesReader) Close() error {
	return l.r.Close()
}

// 复制src到dst
// 注意,并不是覆盖,而是dst.Add
// 这就造成了src中存在,dst中存在,进行Add,其实之后Get是Get到第一个
// 也就是双方都存在是,是dst中的值优先
func copyValues(dst, src url.Values) {
	for k, vs := range src {
		for _, value := range vs {
			dst.Add(k, value)
		}
	}
}

// @see 一定要多看看本函数
func parsePostForm(r *Request) (vs url.Values, err error) {
	if r.Body == nil {
		err = errors.New("missing form body")
		return
	}
	ct := r.Header.Get("Content-Type")
	// RFC 2616, section 7.2.1 - empty type
	//   SHOULD be treated as application/octet-stream
	if ct == "" {
		ct = "application/octet-stream"
	}
	ct, _, err = mime.ParseMediaType(ct)
	switch {
	case ct == "application/x-www-form-urlencoded":
		// 普通表单提交
		var reader io.Reader = r.Body
		maxFormSize := int64(1<<63 - 1)
		if _, ok := r.Body.(*maxBytesReader); !ok {
			maxFormSize = int64(10 << 20) // 10 MB is a lot of text.
			reader = io.LimitReader(r.Body, maxFormSize+1)
		}
		b, e := ioutil.ReadAll(reader)
		if e != nil {
			if err == nil {
				err = e
			}
			break
		}
		if int64(len(b)) > maxFormSize {
			err = errors.New("http: POST too large")
			return
		}
		vs, e = url.ParseQuery(string(b))
		if err == nil {
			err = e
		}
	case ct == "multipart/form-data":
		// 文件表单提交
		// handled by ParseMultipartForm (which is calling us, or should be)
		// TODO(bradfitz): there are too many possible
		// orders to call too many functions here.
		// Clean this up and write more tests.
		// request_test.go contains the start of this,
		// in TestParseMultipartFormOrder and others.
	}
	return
}

// ParseForm populates r.Form and r.PostForm.
//
// For all requests, ParseForm parses the raw query from the URL and updates
// r.Form.
//
// 上文中的For all requests是指所有的http method
//
// For POST, PUT, and PATCH requests, it also parses the request body as a form
// and puts the results into both r.PostForm and r.Form. Request body parameters
// take precedence over URL query string values in r.Form.
//
// For other HTTP methods, or when the Content-Type is not
// application/x-www-form-urlencoded, the request Body is not read, and
// r.PostForm is initialized to a non-nil, empty value.
//
// If the request Body's size has not already been limited by MaxBytesReader,
// the size is capped at 10MB.
//
// ParseMultipartForm calls ParseForm automatically.
// ParseForm is idempotent.
//
// @see
func (r *Request) ParseForm() error {
	// 方法的返回值
	var err error
	// +++ 处理 r.PostForm
	// if r.PostForm == nil: 在没有计算过的情况下才进行计算
	if r.PostForm == nil {
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			// 如果是POST,PUT,PATCH,从body解析出表单数据
			r.PostForm, err = parsePostForm(r)
		}
		if r.PostForm == nil {
			// 确保r.PostForm不是nil map
			r.PostForm = make(url.Values)
		}
	}
	// +++ 处理 r.Form
	// if r.Form == nil: 在没有计算过的情况下才进行计算
	if r.Form == nil {
		if len(r.PostForm) > 0 {
			// 如果r.PostForm有了数据,copy r.PostForm 到 r.Form
			r.Form = make(url.Values)
			copyValues(r.Form, r.PostForm)
		}
		// newValues代表从请求url中的query string解析的数据
		var newValues url.Values
		if r.URL != nil {
			var e error
			// 确保 newValues 不是 nil map
			newValues, e = url.ParseQuery(r.URL.RawQuery)
			if err == nil {
				err = e
			}
		}
		if newValues == nil {
			newValues = make(url.Values)
		}
		if r.Form == nil {
			r.Form = newValues
		} else {
			copyValues(r.Form, newValues)
		}
	}
	return err
}

// ParseMultipartForm parses a request body as multipart/form-data.
// The whole request body is parsed and up to a total of maxMemory bytes of
// its file parts are stored in memory, with the remainder stored on
// disk in temporary files.
// ParseMultipartForm calls ParseForm if necessary.
// After one call to ParseMultipartForm, subsequent calls have no effect.
//
// multipart/form-data(文件上传)
//
// @see
func (r *Request) ParseMultipartForm(maxMemory int64) error {
	/**
	发送post请求时候，表单<form>属性enctype共有二个值可选，这个属性管理的是表单的MIME编码：
	application/x-www-form-urlencoded(默认值)
	multipart/form-data
	其实form表单在你不写enctype属性时，也默认为其添加了enctype属性值，默认值是enctype="application/x- www-form-urlencoded"
	 */
	if r.MultipartForm == multipartByReader {
		return errors.New("http: multipart handled by MultipartReader")
	}
	if r.Form == nil {
		err := r.ParseForm()
		if err != nil {
			return err
		}
	}
	if r.MultipartForm != nil {
		// 之前已经调用过本方法
		return nil
	}

	// 根据http header信息获取一个multipart.Reader对象,文件上传时,
	// 会传递请求头: 'Content-Type: multipart/form-data; boundary=随机数'
	mr, err := r.multipartReader()
	if err != nil {
		return err
	}

	// f类型为multipart.Form
	f, err := mr.ReadForm(maxMemory)
	if err != nil {
		return err
	}

	if r.PostForm == nil {
		// 确保r.PostForm不是nil map
		r.PostForm = make(url.Values)
	}
	for k, v := range f.Value {
		r.Form[k] = append(r.Form[k], v...)
		// r.PostForm should also be populated. See Issue 9305.
		r.PostForm[k] = append(r.PostForm[k], v...)
	}

	r.MultipartForm = f

	return nil
}

// FormValue returns the first value for the named component of the query.
// POST and PUT body parameters take precedence over URL query string values.
// FormValue calls ParseMultipartForm and ParseForm if necessary and ignores
// any errors returned by these functions.
// If key is not present, FormValue returns the empty string.
// To access multiple values of the same key, call ParseForm and
// then inspect Request.Form directly.
//
// FormValue只与r.Form相关
// @see
func (r *Request) FormValue(key string) string {
	if r.Form == nil {
		r.ParseMultipartForm(defaultMaxMemory)
	}
	if vs := r.Form[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// PostFormValue returns the first value for the named component of the POST
// or PUT request body. URL query parameters are ignored.
// PostFormValue calls ParseMultipartForm and ParseForm if necessary and ignores
// any errors returned by these functions.
// If key is not present, PostFormValue returns the empty string.
//
// PostFormValue只与r.PostForm相关
// @see
func (r *Request) PostFormValue(key string) string {
	if r.PostForm == nil {
		r.ParseMultipartForm(defaultMaxMemory)
	}
	if vs := r.PostForm[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// FormFile returns the first file for the provided form key.
// FormFile calls ParseMultipartForm and ParseForm if necessary.
//
// multipart.File和multipart.FileHeader是什么关系???
// *multipart.FileHeader拥有Open方法,该方法返回multipart.File
// func (fh *FileHeader) Open() (File, error) {
//
// @see
func (r *Request) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	if r.MultipartForm == multipartByReader {
		return nil, nil, errors.New("http: multipart handled by MultipartReader")
	}
	if r.MultipartForm == nil {
		// 之前没有调用过ParseMultipartForm
		err := r.ParseMultipartForm(defaultMaxMemory)
		if err != nil {
			return nil, nil, err
		}
	}
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		if fhs := r.MultipartForm.File[key]; len(fhs) > 0 {
			f, err := fhs[0].Open()
			return f, fhs[0], err
		}
	}
	return nil, nil, ErrMissingFile
}

func (r *Request) expectsContinue() bool {
	return hasToken(r.Header.get("Expect"), "100-continue")
}

func (r *Request) wantsHttp10KeepAlive() bool {
	if r.ProtoMajor != 1 || r.ProtoMinor != 0 {
		return false
	}
	return hasToken(r.Header.get("Connection"), "keep-alive")
}

func (r *Request) wantsClose() bool {
	return hasToken(r.Header.get("Connection"), "close")
}

func (r *Request) closeBody() {
	if r.Body != nil {
		r.Body.Close()
	}
}

func (r *Request) isReplayable() bool {
	if r.Body == nil || r.Body == NoBody || r.GetBody != nil {
		switch valueOrDefault(r.Method, "GET") {
		case "GET", "HEAD", "OPTIONS", "TRACE":
			return true
		}
	}
	return false
}

// outgoingLength reports the Content-Length of this outgoing (Client) request.
// It maps 0 into -1 (unknown) when the Body is non-nil.
func (r *Request) outgoingLength() int64 {
	if r.Body == nil || r.Body == NoBody {
		return 0
	}
	if r.ContentLength != 0 {
		return r.ContentLength
	}
	return -1
}

// requestMethodUsuallyLacksBody reports whether the given request
// method is one that typically does not involve a request body.
// This is used by the Transport (via
// transferWriter.shouldSendChunkedRequestBody) to determine whether
// we try to test-read a byte from a non-nil Request.Body when
// Request.outgoingLength() returns -1. See the comments in
// shouldSendChunkedRequestBody.
func requestMethodUsuallyLacksBody(method string) bool {
	switch method {
	case "GET", "HEAD", "DELETE", "OPTIONS", "PROPFIND", "SEARCH":
		// 这些http方法都无需body
		return true
	}
	return false
}
