// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-12 15:46:10

// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package exec

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is the error resulting if a path search failed to find an executable file.
var ErrNotFound = errors.New("executable file not found in $PATH")

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	// linux不像windows只需要判断是否是目录,还需要判断rwx
	if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
		// 0111是8进制,对应二进制是 001001001
		//                      rwxrwxrwx
		// 也就是需要ugo任何一个有x运行权限
		return nil
	}
	return os.ErrPermission
}

// LookPath searches for an executable binary named file
// in the directories named by the PATH environment variable.
// If file contains a slash, it is tried directly and the PATH is not consulted.
// The result may be an absolute path or a path relative to the current directory.
func LookPath(file string) (string, error) {
	// NOTE(rsc): I wish we could use the Plan 9 behavior here
	// (only bypass the path if file begins with / or ./ or ../)
	// but that would not match all the Unix shells.

	if strings.Contains(file, "/") {
		// 文档:If file contains a slash, it is tried directly and the PATH is not consulted.
		err := findExecutable(file)
		if err == nil {
			return file, nil
		}
		return "", &Error{file, err}
	}
	// 文档:searches for an executable binary named file in the directories named by the PATH environment variable.
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			return path, nil
		}
	}
	return "", &Error{file, ErrNotFound}
}
