// Package userip provides functions for extracting a user IP address from a
// request and associating it with a Context.
//
// This package is an example to accompany https://blog.golang.org/context.
// It is not intended for use by others.
package userip

import (
	"fmt"
	"net"
	"net/http"
	"context"
)	

// FromRequest extracts the user IP address from req, if present.
func FromRequest(req *http.Request) (net.IP, error) {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		// SplitHostPort error
		return nil, fmt.Errorf("userip: %q is not IP:port", req.RemoteAddr)
	}

	userIP := net.ParseIP(ip)
	if userIP == nil {
		// net.ParseIP error
		return nil, fmt.Errorf("userip: %q is not IP:port", req.RemoteAddr)
	}
	return userIP, nil
}

// The key type is unexported to prevent collisions with context keys defined in
// other packages.
type key int

// userIPkey is the context key for the user IP address.  Its value of zero is
// arbitrary.  If this package defined other context keys, they would have
// different integer values.
const userIPKey key = 0

// NewContext returns a new Context carrying userIP.
func NewContext(ctx context.Context, userIP net.IP) context.Context {
	return context.WithValue(ctx, userIPKey, userIP)
}

// FromContext extracts the user IP address from ctx, if present.
func FromContext(ctx context.Context) (net.IP, bool) {
	// ctx.Value returns nil if ctx has no value for the key;
	// 注意: the net.IP type assertion returns ok=false for nil.

	// ctx.Value 签名: Value(key interface{}) interface{}
	// 因此 ctx.Value(userIPKey) 即使实际值是 nil 也没关系
	userIP, ok := ctx.Value(userIPKey).(net.IP)
	return userIP, ok
}
