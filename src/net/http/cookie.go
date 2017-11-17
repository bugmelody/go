// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[3-over]]] 2017-3-4 19:14:56

package http

import (
	"bytes"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

// A Cookie represents an HTTP cookie as sent in the Set-Cookie header of an
// HTTP response or the Cookie header of an HTTP request.
//
// See http://tools.ietf.org/html/rfc6265 for details.
//
// Cookie struct代表请求中的单个cookie或响应中的单个cookie
type Cookie struct {
	Name  string
	Value string

	Path       string    // optional
	Domain     string    // optional
	// Expires参考: https://tools.ietf.org/html/rfc6265#section-4.1.2.1,以下是rfc中Expires的说明:
	// rfc原文:The Expires attribute indicates the maximum lifetime of the cookie,
	// represented as the date and time at which the cookie expires.  The
	// user agent is not required to retain the cookie until the specified
	// date has passed.  In fact, user agents often evict cookies due to
	// memory pressure or privacy concerns(在实际中,浏览器经常由于自身的考虑或内存压力,会进行提前清理).
	Expires    time.Time // optional
	// RawExpires和Expires有什么区别?
	RawExpires string    // for reading cookies only

	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'
	// MaxAge>0 means Max-Age attribute present and given in seconds
	// 参考:https://tools.ietf.org/html/rfc6265#section-4.1.2.2,以下是rfc原文:
	// The Max-Age attribute indicates the maximum lifetime of the cookie,
	// represented as the number of seconds until the cookie expires.  The
	// user agent is not required to retain the cookie for the specified
	// duration.  In fact, user agents often evict cookies due to memory
	// pressure or privacy concerns(在实际中,浏览器经常由于自身的考虑或内存压力,会进行提前清理).
	//
	// NOTE: Some existing user agents do not support the Max-Age
	// attribute.  User agents that do not support the Max-Age attribute
	// ignore the attribute.
	//
	// If a cookie has both the Max-Age and the Expires attribute, the Max-
	// Age attribute has precedence and controls the expiration date of the
	// cookie.  If a cookie has neither the Max-Age nor the Expires
	// attribute, the user agent will retain the cookie until "the current
	// session is over" (as defined by the user agent).
	MaxAge   int
	// cookie的secure属性: http://www.cnblogs.com/EX32/p/4398960.html
	// https://tools.ietf.org/html/rfc6265#section-4.1.2.5
	// 当secure属性设置为true时，cookie只有在https协议下才能上传到服务器，而在http协议下是没法上传的，所以也不会被窃听
	Secure   bool
	// 参考:https://tools.ietf.org/html/rfc6265#section-4.1.2.6
	HttpOnly bool
	// ??????
	Raw      string
	// ??????
	Unparsed []string // Raw text of unparsed attribute-value pairs
}

// readSetCookies parses all "Set-Cookie" values from
// the header h and returns the successfully parsed Cookies.
//
// readSetCookies解析header["Set-Cookie"]中的所有值
// 并返回成功解析的cookies.
// 暂时不看函数源码
func readSetCookies(h Header) []*Cookie {
	cookieCount := len(h["Set-Cookie"])
	if cookieCount == 0 {
		return []*Cookie{}
	}
	cookies := make([]*Cookie, 0, cookieCount)
	for _, line := range h["Set-Cookie"] {
		parts := strings.Split(strings.TrimSpace(line), ";")
		if len(parts) == 1 && parts[0] == "" {
			continue
		}
		parts[0] = strings.TrimSpace(parts[0])
		j := strings.Index(parts[0], "=")
		if j < 0 {
			continue
		}
		name, value := parts[0][:j], parts[0][j+1:]
		if !isCookieNameValid(name) {
			continue
		}
		value, ok := parseCookieValue(value, true)
		if !ok {
			continue
		}
		c := &Cookie{
			Name:  name,
			Value: value,
			Raw:   line,
		}
		for i := 1; i < len(parts); i++ {
			parts[i] = strings.TrimSpace(parts[i])
			if len(parts[i]) == 0 {
				continue
			}

			attr, val := parts[i], ""
			if j := strings.Index(attr, "="); j >= 0 {
				attr, val = attr[:j], attr[j+1:]
			}
			lowerAttr := strings.ToLower(attr)
			val, ok = parseCookieValue(val, false)
			if !ok {
				c.Unparsed = append(c.Unparsed, parts[i])
				continue
			}
			switch lowerAttr {
			case "secure":
				c.Secure = true
				continue
			case "httponly":
				c.HttpOnly = true
				continue
			case "domain":
				c.Domain = val
				continue
			case "max-age":
				secs, err := strconv.Atoi(val)
				if err != nil || secs != 0 && val[0] == '0' {
					break
				}
				if secs <= 0 {
					secs = -1
				}
				c.MaxAge = secs
				continue
			case "expires":
				c.RawExpires = val
				exptime, err := time.Parse(time.RFC1123, val)
				if err != nil {
					exptime, err = time.Parse("Mon, 02-Jan-2006 15:04:05 MST", val)
					if err != nil {
						c.Expires = time.Time{}
						break
					}
				}
				c.Expires = exptime.UTC()
				continue
			case "path":
				c.Path = val
				continue
			}
			c.Unparsed = append(c.Unparsed, parts[i])
		}
		cookies = append(cookies, c)
	}
	return cookies
}

// SetCookie adds a Set-Cookie header to the provided ResponseWriter's headers.
// The provided cookie must have a valid Name. Invalid cookies may be
// silently dropped.
//
// ??? 这个函数是设置整个 "Set-Cookie" 头???
// ??? 一个http相应只能有一个"Set-Cookie" 头???
//
// ??? w.Header().Add 是进行了实际的发送还是仅仅是记录了下来 ???
func SetCookie(w ResponseWriter, cookie *Cookie) {
	if v := cookie.String(); v != "" {
		w.Header().Add("Set-Cookie", v)
	}
}

// String returns the serialization of the cookie for use in a Cookie
// header (if only Name and Value are set) or a Set-Cookie response
// header (if other fields are set).
// If c is nil or c.Name is invalid, the empty string is returned.
//
// 上文中: Cookie header(指请求时的Header['cookie'])
// Set-Cookie response header(指响应时的Header['Set-Cookie'])
// @see
func (c *Cookie) String() string {
	if c == nil || !isCookieNameValid(c.Name) {
		return ""
	}
	var b bytes.Buffer
	b.WriteString(sanitizeCookieName(c.Name))
	b.WriteRune('=')
	b.WriteString(sanitizeCookieValue(c.Value))

	if len(c.Path) > 0 {
		b.WriteString("; Path=")
		b.WriteString(sanitizeCookiePath(c.Path))
	}
	if len(c.Domain) > 0 {
		if validCookieDomain(c.Domain) {
			// A c.Domain containing illegal characters is not
			// sanitized but simply dropped which turns the cookie
			// into a host-only cookie. A leading dot is okay
			// but won't be sent.
			d := c.Domain
			if d[0] == '.' {
				d = d[1:]
			}
			b.WriteString("; Domain=")
			b.WriteString(d)
		} else {
			log.Printf("net/http: invalid Cookie.Domain %q; dropping domain attribute", c.Domain)
		}
	}
	if validCookieExpires(c.Expires) {
		b.WriteString("; Expires=")
		b2 := b.Bytes()
		b.Reset()
		b.Write(c.Expires.UTC().AppendFormat(b2, TimeFormat))
	}
	if c.MaxAge > 0 {
		b.WriteString("; Max-Age=")
		b2 := b.Bytes()
		b.Reset()
		b.Write(strconv.AppendInt(b2, int64(c.MaxAge), 10))
	} else if c.MaxAge < 0 {
		b.WriteString("; Max-Age=0")
	}
	if c.HttpOnly {
		b.WriteString("; HttpOnly")
	}
	if c.Secure {
		b.WriteString("; Secure")
	}
	return b.String()
}

// readCookies parses all "Cookie" values from the header h and
// returns the successfully parsed Cookies.
//
// if filter isn't empty, only cookies of that name are returned
//
// 注意,解析的是h["Cookie"],也就是http请求中的Cookie头部
// @see
func readCookies(h Header, filter string) []*Cookie {
	lines, ok := h["Cookie"]
	if !ok {
		// h["Cookie"]不存在
		return []*Cookie{}
	}

	// 初始化最后要返回的值
	cookies := []*Cookie{}
	// lines的类型为[]string
	for _, line := range lines {
		// 使用分号分隔
		parts := strings.Split(strings.TrimSpace(line), ";")
		if len(parts) == 1 && parts[0] == "" {
			// 格式有问题,忽略,进行下轮循环
			continue
		}
		// Per-line attributes
		for i := 0; i < len(parts); i++ {
			parts[i] = strings.TrimSpace(parts[i])
			if len(parts[i]) == 0 {
				continue
			}
			name, val := parts[i], ""
			if j := strings.Index(name, "="); j >= 0 {
				name, val = name[:j], name[j+1:]
			}
			if !isCookieNameValid(name) {
				// cookie name 不合法
				continue
			}
			if filter != "" && filter != name {
				// 被filter参数过滤掉
				continue
			}
			val, ok := parseCookieValue(val, true)
			if !ok {
				// 解析cookie value失败
				continue
			}
			cookies = append(cookies, &Cookie{Name: name, Value: val})
		}
	}
	return cookies
}

// validCookieDomain returns whether v is a valid cookie domain-value.
func validCookieDomain(v string) bool {
	if isCookieDomainName(v) {
		return true
	}
	if net.ParseIP(v) != nil && !strings.Contains(v, ":") {
		return true
	}
	return false
}

// validCookieExpires returns whether v is a valid cookie expires-value.
func validCookieExpires(t time.Time) bool {
	// IETF RFC 6265 Section 5.1.1.5, the year must not be less than 1601
	return t.Year() >= 1601
}

// isCookieDomainName returns whether s is a valid domain name or a valid
// domain name with a leading dot '.'.  It is almost a direct copy of
// package net's isDomainName.
//
// @notsee
func isCookieDomainName(s string) bool {
	if len(s) == 0 {
		return false
	}
	if len(s) > 255 {
		return false
	}

	if s[0] == '.' {
		// A cookie a domain attribute may start with a leading dot.
		s = s[1:]
	}
	last := byte('.')
	ok := false // Ok once we've seen a letter.
	partlen := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		default:
			return false
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
			// No '_' allowed here (in contrast to package net).
			ok = true
			partlen++
		case '0' <= c && c <= '9':
			// fine
			partlen++
		case c == '-':
			// Byte before dash cannot be dot.
			if last == '.' {
				return false
			}
			partlen++
		case c == '.':
			// Byte before dot cannot be dot, dash.
			if last == '.' || last == '-' {
				return false
			}
			if partlen > 63 || partlen == 0 {
				return false
			}
			partlen = 0
		}
		last = c
	}
	if last == '-' || partlen > 63 {
		return false
	}

	return ok
}

// 这里是将 "\n" 替换为 "-", 将 "\r" 替换为 "-"
var cookieNameSanitizer = strings.NewReplacer("\n", "-", "\r", "-")

// @notsee
func sanitizeCookieName(n string) string {
	return cookieNameSanitizer.Replace(n)
}

// http://tools.ietf.org/html/rfc6265#section-4.1.1
// cookie-value      = *cookie-octet / ( DQUOTE *cookie-octet DQUOTE )
// cookie-octet      = %x21 / %x23-2B / %x2D-3A / %x3C-5B / %x5D-7E
//           ; US-ASCII characters excluding CTLs,
//           ; whitespace DQUOTE, comma, semicolon,
//           ; and backslash
// We loosen this as spaces and commas are common in cookie values
// but we produce a quoted cookie-value in when value starts or ends
// with a comma or space.
// See https://golang.org/issue/7243 for the discussion.
//
// @notsee
func sanitizeCookieValue(v string) string {
	v = sanitizeOrWarn("Cookie.Value", validCookieValueByte, v)
	if len(v) == 0 {
		// 如果sanitizeOrWarn之后的v长度为0,返回空字符串
		return v
	}
	if strings.IndexByte(v, ' ') >= 0 || strings.IndexByte(v, ',') >= 0 {
		// 如果发现 空格 或 逗号, 使用双引号包围
		return `"` + v + `"`
	}
	return v
}

// @notsee
func validCookieValueByte(b byte) bool {
	return 0x20 <= b && b < 0x7f && b != '"' && b != ';' && b != '\\'
}

// path-av           = "Path=" path-value
// path-value        = <any CHAR except CTLs or ";">
// @notsee
func sanitizeCookiePath(v string) string {
	return sanitizeOrWarn("Cookie.Path", validCookiePathByte, v)
}

// @notsee
func validCookiePathByte(b byte) bool {
	return 0x20 <= b && b < 0x7f && b != ';'
}

// @notsee
func sanitizeOrWarn(fieldName string, valid func(byte) bool, v string) string {
	// 开始假设全ok
	ok := true
	for i := 0; i < len(v); i++ {
		if valid(v[i]) {
			continue
		}
		// 如果有不合法的,log.Printf会记录
		log.Printf("net/http: invalid byte %q in %s; dropping invalid bytes", v[i], fieldName)
		// 设置为不ok
		ok = false
		break
	}
	if ok {
		// 如果全部 ok,直接返回
		return v
	}
	// 现在 ,有不合法的
	// 初始化一个 []byte, cap 为 v 的长度
	buf := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		// 循环v,如果合法,append到 buf
		if b := v[i]; valid(b) {
			buf = append(buf, b)
		}
	}
	// 将 []byte 转换为 string 后返回
	return string(buf)
}

// @notsee
func parseCookieValue(raw string, allowDoubleQuote bool) (string, bool) {
	// Strip the quotes, if present.
	if allowDoubleQuote && len(raw) > 1 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		raw = raw[1 : len(raw)-1]
	}
	for i := 0; i < len(raw); i++ {
		if !validCookieValueByte(raw[i]) {
			return "", false
		}
	}
	return raw, true
}

// @notsee
func isCookieNameValid(raw string) bool {
	if raw == "" {
		return false
	}
	return strings.IndexFunc(raw, isNotToken) < 0
}
