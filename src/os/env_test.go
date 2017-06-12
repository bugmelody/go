// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[2-over]]] 2017-3-22 21:08:14

package os_test

import (
	. "os"
	"reflect"
	"strings"
	"testing"
)

// testGetenv gives us a controlled set of variables for testing Expand.
func testGetenv(s string) string {
	switch s {
	case "*":
		return "all the args"
	case "#":
		return "NARGS"
	case "$":
		return "PID"
	case "1":
		return "ARGUMENT1"
	case "HOME":
		return "/usr/gopher"
	case "H":
		return "(Value of H)"
	case "home_1":
		return "/usr/foo"
	case "_":
		return "underscore"
	}
	return ""
}

var expandTests = []struct {
	in, out string
}{
	{"", ""},
	{"$*", "all the args"},
	{"$$", "PID"},
	{"${*}", "all the args"},
	{"$1", "ARGUMENT1"},
	{"${1}", "ARGUMENT1"},
	{"now is the time", "now is the time"},
	{"$HOME", "/usr/gopher"},
	{"$home_1", "/usr/foo"},
	{"${HOME}", "/usr/gopher"},
	{"${H}OME", "(Value of H)OME"},
	{"A$$$#$1$H$home_1*B", "APIDNARGSARGUMENT1(Value of H)/usr/foo*B"},
}

func TestExpand(t *testing.T) {
	for _, test := range expandTests {
		// test.in是输入
		// test.out是期望的输出
		// testGetenv 是 Expand 需要的 mapping 函数
		result := Expand(test.in, testGetenv)
		if result != test.out {
			t.Errorf("Expand(%q)=%q; expected %q", test.in, result, test.out)
		}
	}
}

// 确保多次调用Environ()返回的结果是DeepEqual
func TestConsistentEnviron(t *testing.T) {
	e0 := Environ()
	for i := 0; i < 10; i++ {
		e1 := Environ()
		if !reflect.DeepEqual(e0, e1) {
			t.Fatalf("environment changed")
		}
	}
}

func TestUnsetenv(t *testing.T) {
	const testKey = "GO_TEST_UNSETENV"
	// set返回环境变量中是否设置了"GO_TEST_UNSETENV"
	set := func() bool {
		prefix := testKey + "="
		for _, key := range Environ() {
			if strings.HasPrefix(key, prefix) {
				return true
			}
		}
		return false
	}
	if err := Setenv(testKey, "1"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	if !set() {
		t.Error("Setenv didn't set TestUnsetenv")
	}
	if err := Unsetenv(testKey); err != nil {
		t.Fatalf("Unsetenv: %v", err)
	}
	if set() {
		t.Fatal("Unsetenv didn't clear TestUnsetenv")
	}
}

func TestClearenv(t *testing.T) {
	const testKey = "GO_TEST_CLEARENV"
	const testValue = "1"

	// reset env
	// 传递给匿名函数的Environ()是当前状态的环境变量
	// 这里通过defer func在函数退出时候还原环境变量设置
	defer func(origEnv []string) {
		for _, pair := range origEnv {
			// Environment variables on Windows can begin with =
			// http://blogs.msdn.com/b/oldnewthing/archive/2010/05/06/10008132.aspx
			i := strings.Index(pair[1:], "=") + 1
			if err := Setenv(pair[:i], pair[i+1:]); err != nil {
				t.Errorf("Setenv(%q, %q) failed during reset: %v", pair[:i], pair[i+1:], err)
			}
		}
	}(Environ())

	if err := Setenv(testKey, testValue); err != nil {
		t.Fatalf("Setenv(%q, %q) failed: %v", testKey, testValue, err)
	}
	if _, ok := LookupEnv(testKey); !ok {
		t.Errorf("Setenv(%q, %q) didn't set $%s", testKey, testValue, testKey)
	}
	Clearenv()
	if val, ok := LookupEnv(testKey); ok {
		t.Errorf("Clearenv() didn't clear $%s, remained with value %q", testKey, val)
	}
}

func TestLookupEnv(t *testing.T) {
	// No one has smallpox(n. [内科] 天花).
	const smallpox = "SMALLPOX"      // No one has smallpox.
	// ok代表是否存在SMALLPOX这个环境变量
	value, ok := LookupEnv(smallpox) // Should not exist.
	if ok || value != "" {
		t.Fatalf("%s=%q", smallpox, value)
	}
	defer Unsetenv(smallpox)
	// virus ['vaɪrəs] n. [病毒] 病毒；恶毒；毒害
	err := Setenv(smallpox, "virus")
	if err != nil {
		t.Fatalf("failed to release smallpox virus")
	}
	value, ok = LookupEnv(smallpox)
	if !ok {
		t.Errorf("smallpox release failed; world remains safe but LookupEnv is broken")
	}
}
