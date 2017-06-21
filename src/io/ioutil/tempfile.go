// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-21 12:53:37

package ioutil

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Random number state.
// We generate random temporary file names so that there's a good
// chance the file doesn't exist yet - keeps the number of tries in
// TempFile to a minimum.
//
// 临时文件使用随机名字,尽量让TempFile函数中的尝试次数最少
var rand uint32
// randmu 用于保护上面声明的 rand 全局变量
var randmu sync.Mutex

// 使用 时间戳+进程id 的方式作为随机数种子
func reseed() uint32 {
	// os.Getpid 返回调用者的进程 id
	return uint32(time.Now().UnixNano() + int64(os.Getpid()))
}

func nextSuffix() string {
	// 在 randmu 的保护期间, rand 这个全局变量不应该被其他 goroutine 使用
	randmu.Lock()
	// 局部变量 r 设置为全局变量 rand 的值
	r := rand
	if r == 0 {
		// 如果还没有进行过seed
		r = reseed()
	}
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	rand = r
	// 解除对全局变量 rand 的保护
	randmu.Unlock()
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}

// TempFile creates a new temporary file in the directory dir
// with a name beginning with prefix, opens the file for reading
// and writing, and returns the resulting *os.File.
// If dir is the empty string, TempFile uses the default directory
// for temporary files (see os.TempDir).
// Multiple programs calling TempFile simultaneously
// will not choose the same file. The caller can use f.Name()
// to find the pathname of the file. It is the caller's responsibility
// to remove the file when no longer needed.
func TempFile(dir, prefix string) (f *os.File, err error) {
	if dir == "" {
		// 文档:If dir is the empty string, TempFile uses the default directory
		// for temporary files (see os.TempDir).
		dir = os.TempDir()
	}

	// nconflict代表了文件名冲突的次数
	nconflict := 0
	// 如果总循环次数i大于1w,会返回最后一次错误调用os.OpenFile的结果
	for i := 0; i < 10000; i++ {
		name := filepath.Join(dir, prefix+nextSuffix())
		// 现在, name 代表了准备尝试创建的临时文件完整路径
		f, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			// 如果os.OpenFile报告文件已经存在
			if nconflict++; nconflict > 10 {
				// 如果文件名冲突次数在10以内,会一直使用同一个rand;如果文件名冲突次数大于10,每次会重新reseed出新的rand
				randmu.Lock()
				// 重新生成rand变量,而在nextSuffix函数中,会使用rand变量
				// 全局变量rand被修改后,会影响之后的 nextSuffix 函数
				// 下行的rand是指全局变量
				rand = reseed()
				randmu.Unlock()
			}
			continue
		}
		// 文件创建成功,跳出循环
		break
	}
	return
}

// TempDir creates a new temporary directory in the directory dir
// with a name beginning with prefix and returns the path of the
// new directory. If dir is the empty string, TempDir uses the
// default directory for temporary files (see os.TempDir).
// Multiple programs calling TempDir simultaneously
// will not choose the same directory. It is the caller's responsibility
// to remove the directory when no longer needed.
func TempDir(dir, prefix string) (name string, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	// nconflict代表了文件名冲突的次数
	nconflict := 0
	for i := 0; i < 10000; i++ {
		// try代表了要尝试创建的目录的完整路径
		try := filepath.Join(dir, prefix+nextSuffix())
		err = os.Mkdir(try, 0700)
		if os.IsExist(err) {
			// 如果err是文件已存在的错误
			if nconflict++; nconflict > 10 {
				// 自增冲突次数,如果冲突次数大于10,需要重新seed一个新的rand, nextSuffix函数会使用rand变量
				randmu.Lock()
				rand = reseed()
				randmu.Unlock()
			}
			continue
		}
		if os.IsNotExist(err) {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				// dir不存在
				return "", err
			}
		}
		if err == nil {
			// name是本函数的命名返回值
			name = try
		}
		break
	}
	return
}
