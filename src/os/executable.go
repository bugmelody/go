// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[1-over]]] 2017-6-12 10:00:45

package os

// Executable returns the path name for the executable that started
// the current process. There is no guarantee that the path is still
// pointing to the correct executable. If a symlink was used to start
// the process, depending on the operating system, the result might
// be the symlink or the path it pointed to. If a stable result is
// needed, path/filepath.EvalSymlinks might help.
//
// Executable returns an absolute path unless an error occurred.
//
// The main use case is finding resources located relative to an
// executable.
//
// Executable is not supported on nacl.
//
// 参考: $ go doc filepath.EvalSymlinks
func Executable() (string, error) {
	return executable()
}
