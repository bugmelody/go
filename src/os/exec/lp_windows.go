// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-12 16:02:28

package exec

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is the error resulting if a path search failed to find an executable file.
var ErrNotFound = errors.New("executable file not found in %PATH%")

func chkStat(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if d.IsDir() {
		// windows中是判断是否是目录
		return os.ErrPermission
	}
	return nil
}

func hasExt(file string) bool {
	i := strings.LastIndex(file, ".")
	if i < 0 {
		// 未找到
		return false
	}
	return strings.LastIndexAny(file, `:\/`) < i
}

func findExecutable(file string, exts []string) (string, error) {
	if len(exts) == 0 {
		return file, chkStat(file)
	}
	if hasExt(file) {
		if chkStat(file) == nil {
			return file, nil
		}
	}
	for _, e := range exts {
		// 自动组合可能的扩展名
		if f := file + e; chkStat(f) == nil {
			return f, nil
		}
	}
	return "", os.ErrNotExist
}

// LookPath searches for an executable binary named file
// in the directories named by the PATH environment variable.
// If file contains a slash, it is tried directly and the PATH is not consulted.
// LookPath also uses PATHEXT environment variable to match
// a suitable candidate.
// The result may be an absolute path or a path relative to the current directory.
//
// PATHEXT:是Windows中的环境变量,代表Extensions for executable files
// (比如.com,.exe,.bat,or.cmd).
//
// 通过观察源码发现,在windows中,file参数可以不包含扩展名
func LookPath(file string) (string, error) {
	// 当前系统中实际的值
	var exts []string
	// 我的电脑中实际值是这个样子: ".COM;.EXE;.BAT;.CMD;.VBS;.VBE;.JS;.JSE;.WSF;.WSH;.MSC"
	x := os.Getenv(`PATHEXT`)
	if x != "" {
		// PATHEXT环境变量设置了值
		for _, e := range strings.Split(strings.ToLower(x), `;`) {
			// windows的习惯是以分号对环境变量进行分隔
			if e == "" {
				continue
			}
			if e[0] != '.' {
				// 如果不是以'.'开头,强制以'.'开头
				e = "." + e
			}
			exts = append(exts, e)
		}
	} else {
		// 这些是golang默认的windows下的可执行文件扩展名
		exts = []string{".com", ".exe", ".bat", ".cmd"}
	}
	// 现在,exts已经被计算好

	// 文档:If file contains a slash, it is tried directly and the PATH is not consulted.
	if strings.ContainsAny(file, `:\/`) {
		// 如果file变量中包含':','\','/'
		if f, err := findExecutable(file, exts); err == nil {
			return f, nil
		} else {
			return "", &Error{file, err}
		}
	}
	// 根据文档:searches for an executable binary named file in the directories named by the PATH environment variable.

	// ---- 首先尝试当前目录下
	if f, err := findExecutable(filepath.Join(".", file), exts); err == nil {
		return f, nil
	}
	// ---- 再尝试path环境变量
	path := os.Getenv("path")
	for _, dir := range filepath.SplitList(path) {
		if f, err := findExecutable(filepath.Join(dir, file), exts); err == nil {
			return f, nil
		}
	}
	return "", &Error{file, ErrNotFound}
}
