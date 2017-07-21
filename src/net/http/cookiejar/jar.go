// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-7-17 09:33:28

// Package cookiejar implements an in-memory RFC 6265-compliant http.CookieJar.
// Package cookiejar实现了类似浏览器的cookie管理(基于内存实现)
package cookiejar

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// PublicSuffixList provides the public suffix of a domain. For example:
//      - the public suffix of "example.com" is "com",
//      - the public suffix of "foo1.foo2.foo3.co.uk" is "co.uk", and
//      - the public suffix of "bar.pvt.k12.ma.us" is "pvt.k12.ma.us".
//
// Implementations of PublicSuffixList must be safe for concurrent use by
// multiple goroutines.
//
// An implementation that always returns "" is valid and may be useful for
// testing but it is not secure: it means that the HTTP server for foo.com can
// set a cookie for bar.com.
//
// A public suffix list implementation is in the package
// golang.org/x/net/publicsuffix.
type PublicSuffixList interface {
	// PublicSuffix returns the public suffix of domain.
	//
	// PublicSuffix返回域名的公共后缀.
	//
	// TODO: specify which of the caller and callee is responsible for IP
	// addresses, for leading and trailing dots, for case sensitivity, and
	// for IDN/Punycode.
	PublicSuffix(domain string) string

	// String returns a description of the source of this public suffix
	// list. The description will typically contain something like a time
	// stamp or version number.
	String() string
}

// Options are the options for creating a new Jar.
type Options struct {
	// PublicSuffixList is the public suffix list that determines whether
	// an HTTP server can set a cookie for a domain.
	//
	// A nil value is valid and may be useful for testing but it is not
	// secure: it means that the HTTP server for foo.co.uk can set a cookie
	// for bar.co.uk.
	PublicSuffixList PublicSuffixList
}

// Jar implements the http.CookieJar interface from the net/http package.
//
// 参考 go doc http.CookieJar
type Jar struct {
	psList PublicSuffixList

	// mu locks the remaining fields.
	mu sync.Mutex

	// entries is a set of entries, keyed by their eTLD+1 and subkeyed by
	// their name/domain/path.
	entries map[string]map[string]entry

	// nextSeqNum is the next sequence number assigned to a new cookie
	// created SetCookies.
	nextSeqNum uint64
}

// New returns a new cookie jar. A nil *Options is equivalent to a zero
// Options.
func New(o *Options) (*Jar, error) {
	jar := &Jar{
		entries: make(map[string]map[string]entry),
	}
	if o != nil {
		// 文档: A nil *Options is equivalent to a zero Options.
		jar.psList = o.PublicSuffixList
	}
	return jar, nil
}

/**
per se [,pə:'sei; -'si:]
1.自(身)，本来；本质上
2.亲身；切身
3.本身；本来
4.本质上
 */

// entry is the internal representation of a cookie.
//
// This struct type is not used outside of this package per se, but the exported
// fields are those of RFC 6265.
type entry struct {
	Name       string
	Value      string
	Domain     string
	Path       string
	Secure     bool
	HttpOnly   bool
	// 是否是持久cookie
	Persistent bool
	// 参考:
	// https://imququ.com/post/host-only-cookie.html
	// http://www.rfc-editor.org/rfc/rfc6265.txt
	//
	// 举个例子
	// host-only-flag为true时，Domain属性为example.com的Cookie只有在example.com才有可能获取到；
	// host-only-flag为false时，Domain属性为example.com的Cookie，在example.com、
	// www.example.com、sub.example.com等等都可能获取到。
	HostOnly   bool
	Expires    time.Time
	Creation   time.Time
	LastAccess time.Time

	// seqNum is a sequence number so that Cookies returns cookies in a
	// deterministic order, even for cookies that have equal Path length and
	// equal Creation time. This simplifies testing.
	seqNum uint64
}

// id returns the domain;path;name triple of e as an id.
func (e *entry) id() string {
	// e.Domain,e.Path,e.Name的组合可以唯一标识一个cookie
	return fmt.Sprintf("%s;%s;%s", e.Domain, e.Path, e.Name)
}

// shouldSend determines whether e's cookie qualifies to be included in a
// request to host/path. It is the caller's responsibility to check if the
// cookie is expired.
//
// qualifies to be: 有资格成为
// shouldSend决定是否e这个cookie应该在请求中被发送.检查cookie是否过期是调用者的责任.
func (e *entry) shouldSend(https bool, host, path string) bool {
	// 域名匹配 && path匹配 && (如果是https请求 || 是https请求或e.Secure==false)
	return e.domainMatch(host) && e.pathMatch(path) && (https || !e.Secure)
}

// domainMatch implements "domain-match" of RFC 6265 section 5.1.3.
func (e *entry) domainMatch(host string) bool {
	if e.Domain == host {
		// 完全匹配
		return true
	}
	/**
	rfc6265里有这么一段：
	Either: The cookie's host-only-flag is true and the canonicalized request-host
	is identical to the cookie's domain.
	Or:  The cookie's host-only-flag is false and the canonicalized request-host
	domain-matches the cookie's domain.
	
	获取Cookie时,首先要检查Domain匹配性,其次才检查path,secure,httponly等属性的匹配性.
	如果host-only-flag为true时，只有当前域名与该Cookie的Domain属性完全相等才可以进入后续流程;
	host-only-flag为false时，符合域规则（domain-matches）的域名都可以进入后续流程。
	 */
	return !e.HostOnly && hasDotSuffix(host, e.Domain)
}

// pathMatch implements "path-match" according to RFC 6265 section 5.1.4.
func (e *entry) pathMatch(requestPath string) bool {
	if requestPath == e.Path {
		// 完全匹配
		return true
	}
	if strings.HasPrefix(requestPath, e.Path) {
		if e.Path[len(e.Path)-1] == '/' {
			// e.Path最后一个字节是'/'
			return true // The "/any/" matches "/any/path" case.
		} else if requestPath[len(e.Path)] == '/' {
			return true // The "/any" matches "/any/path" case.
		}
	}
	return false
}

// hasDotSuffix reports whether s ends in "."+suffix.
func hasDotSuffix(s, suffix string) bool {
	// 越在前方的条件越容易判断,性能判断也更高
	return len(s) > len(suffix) && s[len(s)-len(suffix)-1] == '.' && s[len(s)-len(suffix):] == suffix
}

// Cookies implements the Cookies method of the http.CookieJar interface.
//
// It returns an empty slice if the URL's scheme is not HTTP or HTTPS.
//
// 以下描述摘自 http.CookieJar interface
// Cookies returns the cookies to send in a request for the given URL.
// It is up to the implementation to honor the standard cookie use
// restrictions such as in RFC 6265.
// 
// Cookies返回发送请求到u时应使用的cookie.
// 本方法有责任遵守RFC 6265规定的标准cookie限制.
// 也就是说,Cookies返回发送http请求时应该发送的cookie.
func (j *Jar) Cookies(u *url.URL) (cookies []*http.Cookie) {
	return j.cookies(u, time.Now())
}

// cookies is like Cookies but takes the current time as a parameter.
//
// u 代表了要请求哪个 url
// 函数内部会用now判断cookie是否过期,是否应该发送; 以及用于设置LastAccess字段
func (j *Jar) cookies(u *url.URL, now time.Time) (cookies []*http.Cookie) {
	if u.Scheme != "http" && u.Scheme != "https" {
		// 文档:It returns an empty slice if the URL's scheme is not HTTP or HTTPS.
		return cookies
	}
	// host代表了请求url的host
	host, err := canonicalHost(u.Host)
	if err != nil {
		return cookies
	}
	key := jarKey(host, j.psList)

	j.mu.Lock()
	defer j.mu.Unlock()

	submap := j.entries[key]
	if submap == nil {
		// j.entries[key]不存在,返回nil slice
		return cookies
	}
	// 现在,submap不是nil

	// 是否是https请求
	https := u.Scheme == "https"
	// [start: 计算 path]
	path := u.Path
	if path == "" {
		path = "/"
	}
	// [end: 计算 path]

	// 是否修改过submap
	modified := false
	// selected:最终选择要在请求中发送哪些cookie
	var selected []entry
	for id, e := range submap {
		// 内存cookie与持久cookie: http://blog.csdn.net/wyb_aa/article/details/17961321
		if e.Persistent && !e.Expires.After(now) {
			// 如果是持久cookie并且已经过期
			// 在submap中删除对应的cookie
			delete(submap, id)
			// 标记submap被修改过
			modified = true
			continue
		}
		if !e.shouldSend(https, host, path) {
			// 不应该发送,continue到下个循环
			continue
		}
		// 设置最后访问时间
		e.LastAccess = now
		// 将修改后的e设置回submap
		submap[id] = e
		// 进入应该发送的cookie集合
		selected = append(selected, e)
		// 标记submap被修改过
		modified = true
	}
	if modified {
		if len(submap) == 0 {
			delete(j.entries, key)
		} else {
			j.entries[key] = submap
		}
	}

	// sort according to RFC 6265 section 5.4 point 2: by longest
	// path and then by earliest creation time.
	// 对应该发送的cookies进行排序
	sort.Slice(selected, func(i, j int) bool {
		// 匿名函数中捕获外层变量
		s := selected
		// 优先比较长度
		if len(s[i].Path) != len(s[j].Path) {
			return len(s[i].Path) > len(s[j].Path)
		}
		// 在比较cookie的创建时间
		if !s[i].Creation.Equal(s[j].Creation) {
			return s[i].Creation.Before(s[j].Creation)
		}
		return s[i].seqNum < s[j].seqNum
	})
	for _, e := range selected {
		// 循环添加到返回值cookies中
		cookies = append(cookies, &http.Cookie{Name: e.Name, Value: e.Value})
	}

	// 返回的cookies代表了应该在请求中发送哪些cookie
	return cookies
}

// SetCookies implements the SetCookies method of the http.CookieJar interface.
//
// It does nothing if the URL's scheme is not HTTP or HTTPS.
//
// 把 SetCookies 想象成浏览器接受 Set-Cookie 头部的动作
//
// $ go doc http.CookieJar
// 以下描述摘自 http.CookieJar interface
//
// SetCookies handles the receipt of the cookies in a reply for the
// given URL.  It may or may not choose to save the cookies, depending
// on the jar's policy and implementation.
// receipt [rɪ'siːt] n. 收到；收据；收入 vt. 收到
// -----------------------------------
// SetCookies管理从u的响应中收到的cookie
// 根据其策略和实现,它可以选择是否存储cookie
// 也就是说,SetCookies用于接收http响应中的cookie,并进行存储.
func (j *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.setCookies(u, cookies, time.Now())
}

// setCookies is like SetCookies but takes the current time as parameter.
func (j *Jar) setCookies(u *url.URL, cookies []*http.Cookie, now time.Time) {
	if len(cookies) == 0 {
		// 没有cookies需要接受
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		// 文档:It does nothing if the URL's scheme is not HTTP or HTTPS.
		return
	}
	host, err := canonicalHost(u.Host)
	if err != nil {
		return
	}
	key := jarKey(host, j.psList)
	// 返回the directory part of an URL's path
	defPath := defaultPath(u.Path)

	j.mu.Lock()
	defer j.mu.Unlock()

	submap := j.entries[key]

	// 是否修改过submap
	modified := false
	for _, cookie := range cookies {
		// remove代表此cookie是否应该被移除
		e, remove, err := j.newEntry(cookie, now, defPath, host)
		if err != nil {
			// 如果出错,进行下轮循环
			continue
		}
		id := e.id()
		if remove {
			// 如果需要删除这个cookie
			if submap != nil {
				// 只在 if submap != nil { 的情况下进行删除
				if _, ok := submap[id]; ok {
					// 如果submap[id]存在
					delete(submap, id)
					// 标记修改过
					modified = true
				}
			}
			// 进行下轮循环
			continue
		}
		// 现在,说明当前的cookie不应该删除,应该保存
		if submap == nil {
			// 确保map不是nil
			submap = make(map[string]entry)
		}

		if old, ok := submap[id]; ok {
			// 之前存在,更新
			e.Creation = old.Creation
			e.seqNum = old.seqNum
		} else {
			// 之前不存在,创建
			e.Creation = now
			e.seqNum = j.nextSeqNum
			j.nextSeqNum++
		}
		e.LastAccess = now
		// 设置e到submap
		submap[id] = e
		// 标记submap被修改过
		modified = true
	}

	if modified {
		// 如果submap被修改过
		if len(submap) == 0 {
			// 如果submap被修改到里面没有元素,需要删除
			delete(j.entries, key)
		} else {
			// 否则,保存submap到j.entries
			j.entries[key] = submap
		}
	}
}

// canonicalHost strips port from host if present and returns the canonicalized
// host name.
//
// canonicalHost会去除端口并返回 canonicalized host name
func canonicalHost(host string) (string, error) {
	var err error
	// 将host转换为小写
	host = strings.ToLower(host)
	if hasPort(host) {
		// 如果存在端口信息,去掉
		host, _, err = net.SplitHostPort(host)
		if err != nil {
			return "", err
		}
	}
	if strings.HasSuffix(host, ".") {
		// Strip trailing dot from fully qualified domain names.
		// 参考: 什么是完全合格域名:
		// http://blog.sina.com.cn/s/blog_7097b40d0100pvog.html
		// http://www.cnblogs.com/zhenyuyaodidiao/p/4947930.html
		// 如果结尾有点号,去掉它
		host = host[:len(host)-1]
	}
	return toASCII(host)
}

// hasPort reports whether host contains a port number. host may be a host
// name, an IPv4 or an IPv6 address.
func hasPort(host string) bool {
	// colons代表了冒号的数量
	colons := strings.Count(host, ":")
	if colons == 0 {
		// 如果不存在:
		return false
	}
	if colons == 1 {
		// 如果:数量为1
		return true
	}
	// 是否以'['开头,并且包含']:'
	return host[0] == '[' && strings.Contains(host, "]:")
}

// jarKey returns the key to use for a jar.
//
// 比如:'aa.bb.cc.163.com',实际应该返回 'aa.bb.cc'
func jarKey(host string, psl PublicSuffixList) string {
	if isIP(host) {
		// host如果是ip,直接返回ip.因为不涉及域名的PublicSuffix
		return host
	}
	// 现在,host不是ip

	var i int
	// psl 代表 PublicSuffixList
	if psl == nil {
		// psl == nil, 通过字符串操作来计算
		i = strings.LastIndex(host, ".")
		if i <= 0 {
			// host中没有找到'.'
			return host
		}
	} else {
		// psl != nil, 通过 psl.PublicSuffix(host) 来计算
		// psl.PublicSuffix 返回域名的公共后缀
		suffix := psl.PublicSuffix(host)
		if suffix == host {
			return host
		}
		i = len(host) - len(suffix)
		if i <= 0 || host[i-1] != '.' {
			// The provided public suffix list psl is broken.
			// Storing cookies under host is a safe stopgap.
			// stopgap ['stɒpgæp] adj. 权宜的；暂时的 n. 权宜之计；补缺者
			return host
		}
		// Only len(suffix) is used to determine the jar key from
		// here on, so it is okay if psl.PublicSuffix("www.buggy.psl")
		// returns "com" as the jar key is generated from host.
	}
	// 比如:'aa.bb.cc.163.com',实际应该返回 'aa.bb.cc'
	prevDot := strings.LastIndex(host[:i-1], ".")
	return host[prevDot+1:]
}

// isIP reports whether host is an IP address.
// @see
func isIP(host string) bool {
	return net.ParseIP(host) != nil
}

// defaultPath returns the directory part of an URL's path according to
// RFC 6265 section 5.1.4.
func defaultPath(path string) string {
	if len(path) == 0 || path[0] != '/' {
		// 如果path长度为0,或者path不是以'/'开头
		return "/" // Path is empty or malformed.
	}
	// 现在,path肯定是以'/'开头

	// i肯定不会是-1
	i := strings.LastIndex(path, "/") // Path starts with "/", so i != -1.
	if i == 0 {
		return "/" // Path has the form "/abc".
	}
	// 现在,i肯定大于0,取目录部分返回
	return path[:i] // Path is either of form "/abc/xyz" or "/abc/xyz/".
}

// newEntry creates an entry from a http.Cookie c. now is the current time and
// is compared to c.Expires to determine deletion of c. defPath and host are the
// default-path and the canonical host name of the URL c was received from.
//
// remove records whether the jar should delete this cookie, as it has already
// expired with respect to now. In this case, e may be incomplete, but it will
// be valid to call e.id (which depends on e's Name, Domain and Path).
//
// A malformed c.Domain will result in an error.
func (j *Jar) newEntry(c *http.Cookie, now time.Time, defPath, host string) (e entry, remove bool, err error) {
	// e 是函数返回值,此时是zero value
	e.Name = c.Name

	if c.Path == "" || c.Path[0] != '/' {
		// c这个Cookie未设置path,或者设置了path但不是绝对路径
		e.Path = defPath
	} else {
		// c这个Cookie设置了path,使用之
		e.Path = c.Path
	}

	e.Domain, e.HostOnly, err = j.domainAndType(host, c.Domain)
	if err != nil {
		return e, false, err
	}

	/**
	Expires 属性: Optional. This attribute specifies a date string that defines the valid lifetime of that cookie. Once the expiration date has been reached, will no longer be stored or given out. The date is formatted as:
Weekday, DD-Mon-YY HH:MM:SS GMT
The only legal time zone is GMT, and the separators between the elements of the date must be dashes. If Expires is not specified, the will expire when the user's session ends.
Set-Cookie: foo=bar; expires=Wednesday, 09-Nov-99 23:12:40 GMT
	 */

	/**
	Version 1 (RFC 2965) Cookies
	Max-Age:Optional. The value of this attribute is an integer that sets the lifetime of the cookie in seconds. Clients should calculate the age of the according to the HTTP/1.1 age-calculation rules. When a cookie's age becomes greater than the Max-Age, the client should discard the value of zero means the cookie with that name should be discarded immediately.
	 */

	/**
	关于 c.MaxAge: go doc http.Cookie
	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'
	// MaxAge>0 means Max-Age attribute present and given in seconds
	 */
	
	// MaxAge takes precedence over Expires.
	// maxAge和Expires是cookie两个版本rfc中定义的,都跟生存期有关
	if c.MaxAge < 0 {
		// true表示应该删除
		return e, true, nil
	} else if c.MaxAge > 0 {
		e.Expires = now.Add(time.Duration(c.MaxAge) * time.Second)
		// 持久cookie
		e.Persistent = true
	} else {
		if c.Expires.IsZero() {
			e.Expires = endOfTime
			e.Persistent = false
		} else {
			if !c.Expires.After(now) {
				// 过期的情况
				return e, true, nil
			}
			e.Expires = c.Expires
			e.Persistent = true
		}
	}

	e.Value = c.Value
	e.Secure = c.Secure
	e.HttpOnly = c.HttpOnly

	return e, false, nil
}

var (
	errIllegalDomain   = errors.New("cookiejar: illegal cookie domain attribute")
	errMalformedDomain = errors.New("cookiejar: malformed cookie domain attribute")
	errNoHostname      = errors.New("cookiejar: no host name available (IP only)")
)

// endOfTime is the time when session (non-persistent) cookies expire.
// This instant is representable in most date/time formats (not just
// Go's time.Time) and should be far enough in the future.
//
// 注意两种cookie的区别: non-persistent cookies, persistent cookies
var endOfTime = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

// domainAndType determines the cookie's domain and hostOnly attribute.
// @notsee
func (j *Jar) domainAndType(host, domain string) (string, bool, error) {
	if domain == "" {
		// No domain attribute in the SetCookie header indicates a
		// host cookie.
		return host, true, nil
	}

	if isIP(host) {
		// According to RFC 6265 domain-matching includes not being
		// an IP address.
		// TODO: This might be relaxed as in common browsers.
		return "", false, errNoHostname
	}

	// From here on: If the cookie is valid, it is a domain cookie (with
	// the one exception of a public suffix below).
	// See RFC 6265 section 5.2.3.
	if domain[0] == '.' {
		domain = domain[1:]
	}

	if len(domain) == 0 || domain[0] == '.' {
		// Received either "Domain=." or "Domain=..some.thing",
		// both are illegal.
		return "", false, errMalformedDomain
	}
	domain = strings.ToLower(domain)

	if domain[len(domain)-1] == '.' {
		// We received stuff like "Domain=www.example.com.".
		// Browsers do handle such stuff (actually differently) but
		// RFC 6265 seems to be clear here (e.g. section 4.1.2.3) in
		// requiring a reject.  4.1.2.3 is not normative, but
		// "Domain Matching" (5.1.3) and "Canonicalized Host Names"
		// (5.1.2) are.
		return "", false, errMalformedDomain
	}

	// See RFC 6265 section 5.3 #5.
	if j.psList != nil {
		if ps := j.psList.PublicSuffix(domain); ps != "" && !hasDotSuffix(domain, ps) {
			if host == domain {
				// This is the one exception in which a cookie
				// with a domain attribute is a host cookie.
				return host, true, nil
			}
			return "", false, errIllegalDomain
		}
	}

	// The domain must domain-match host: www.mycompany.com cannot
	// set cookies for .ourcompetitors.com.
	if host != domain && !hasDotSuffix(host, domain) {
		return "", false, errIllegalDomain
	}

	return domain, false, nil
}
