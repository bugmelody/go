// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-7-3 11:12:27

package regexp_test

import (
	"fmt"
	"regexp"
)

func Example() {
	// Compile the expression once, usually at init time.
	// Use raw strings to avoid having to quote the backslashes.
	var validID = regexp.MustCompile(`^[a-z]+\[[0-9]+\]$`)

	fmt.Println(validID.MatchString("adam[23]"))
	fmt.Println(validID.MatchString("eve[7]"))
	// J 为大写,匹配失败
	fmt.Println(validID.MatchString("Job[48]"))
	// 没有方括号部分,匹配失败
	fmt.Println(validID.MatchString("snakey"))
	// Output:
	// true
	// true
	// false
	// false
}

func ExampleMatchString() {
	// foo后面跟任意字符,因此匹配成功,输出: true <nil>
	matched, err := regexp.MatchString("foo.*", "seafood")
	fmt.Println(matched, err)
	// bar后面跟任意字符,因此匹配失败,输出: false <nil>
	matched, err = regexp.MatchString("bar.*", "seafood")
	fmt.Println(matched, err)
	// 正则语法错误,因此输出: false error parsing regexp: missing closing ): `a(b`
	matched, err = regexp.MatchString("a(b", "seafood")
	fmt.Println(matched, err)
	// Output:
	// true <nil>
	// false <nil>
	// false error parsing regexp: missing closing ): `a(b`
}

func ExampleRegexp_FindString() {
	re := regexp.MustCompile("foo.?")
	fmt.Printf("%q\n", re.FindString("seafood fool"))
	fmt.Printf("%q\n", re.FindString("meat"))
	// Output:
	// "food"
	// ""
}

func ExampleRegexp_FindStringIndex() {
	re := regexp.MustCompile("ab?")
	fmt.Println(re.FindStringIndex("tablett"))
	fmt.Println(re.FindStringIndex("foo") == nil)
	// Output:
	// [1 3]
	// true
}

func ExampleRegexp_FindStringSubmatch() {
	re := regexp.MustCompile("a(x*)b(y|z)c")
	// 关于 Submatch
	// If 'Submatch' is present, the return value is a slice identifying the
	// successive submatches of the expression. Submatches are matches of
	// parenthesized subexpressions (also known as capturing groups) within the
	// regular expression, numbered from left to right in order of opening
	// parenthesis. Submatch 0 is the match of the entire expression, submatch 1
	// the match of the first parenthesized subexpression, and so on.
	fmt.Printf("%q\n", re.FindStringSubmatch("-axxxbyc-"))
	fmt.Printf("%q\n", re.FindStringSubmatch("-abzc-"))
	// Output:
	// ["axxxbyc" "xxx" "y"]
	// ["abzc" "" "z"]
}

func ExampleRegexp_FindAllString() {
	re := regexp.MustCompile("a.")

	// 关于 All
	// If 'All' is present, the routine matches successive non-overlapping
	// matches of the entire expression. Empty matches abutting a preceding
	// match are ignored. The return value is a slice containing the successive
	// return values of the corresponding non-'All' routine. These routines take
	// an extra integer argument, n; if n >= 0, the function returns at most n
	// matches/submatches.
	fmt.Println(re.FindAllString("paranormal", -1))
	fmt.Println(re.FindAllString("paranormal", 2))
	fmt.Println(re.FindAllString("graal", -1))
	fmt.Println(re.FindAllString("none", -1))
	// Output:
	// [ar an al]
	// [ar an]
	// [aa]
	// []
}

func ExampleRegexp_FindAllStringSubmatch() {
	// 首先是一个 a, 然后是任意个 x, 然后是 b, 注意 x 可以不出现. 即使 x 没有出现,整个正则也算匹配成功.
	re := regexp.MustCompile("a(x*)b")
	// [["ab" ""]],整个正则匹配1次,子表达式 (x*) 匹配空
	fmt.Printf("%q\n", re.FindAllStringSubmatch("-ab-", -1))
	// [["axxb" "xx"]],整个正则匹配1次,子表达式(x*) 匹配 xx
	fmt.Printf("%q\n", re.FindAllStringSubmatch("-axxb-", -1))
	// [["ab" ""] ["axb" "x"]],整个正则匹配2次,子表达式(x*)第一次匹配空,第二次匹配 x
	fmt.Printf("%q\n", re.FindAllStringSubmatch("-ab-axb-", -1))
	// [["axxb" "xx"] ["ab" ""]],整个正则匹配2次,子表达式(x*)第一次匹配xx,第二次匹配空
	fmt.Printf("%q\n", re.FindAllStringSubmatch("-axxb-ab-", -1))
	// Output:
	// [["ab" ""]]
	// [["axxb" "xx"]]
	// [["ab" ""] ["axb" "x"]]
	// [["axxb" "xx"] ["ab" ""]]
}

func ExampleRegexp_FindAllStringSubmatchIndex() {
	// 首先是一个 a, 然后是任意个 x, 然后是 b, 注意 x 可以不出现. 即使 x 没有出现,整个正则也算匹配成功.
	re := regexp.MustCompile("a(x*)b")
	// Indices:
	//    01234567   012345678
	//    -ab-axb-   -axxb-ab-

	// [[1 3 2 2]] 总共一次匹配;第一次整体匹配是 1,3, 分组 (x*) 匹配 2,2 (空匹配)
	fmt.Println(re.FindAllStringSubmatchIndex("-ab-", -1))
	// [[1 5 2 4]] 总共一次匹配;第一次整体匹配是 1,5, 分组 (x*) 匹配 2,4 (匹配xx)
	fmt.Println(re.FindAllStringSubmatchIndex("-axxb-", -1))
	// [[1 3 2 2] [4 7 5 6]] 总共两次匹配;第一次整体匹配是 1,3(ab), 分组 (x*) 匹配 2,2 (空匹配); 第二次整体匹配是 4,7(axb), 分组 (x*) 匹配 5,6 (匹配x);
	fmt.Println(re.FindAllStringSubmatchIndex("-ab-axb-", -1))
	// [[1 5 2 4] [6 8 7 7]] 总共两次匹配;第一次整体匹配是 1,5(axxb), 分组 (x*) 匹配 2,4 (匹配xx); 第二次整体匹配是 6,8(ab), 分组 (x*) 匹配 7,7 (空匹配);
	fmt.Println(re.FindAllStringSubmatchIndex("-axxb-ab-", -1))
	// [] 总共匹配零次
	fmt.Println(re.FindAllStringSubmatchIndex("-foo-", -1))
	// Output:
	// [[1 3 2 2]]
	// [[1 5 2 4]]
	// [[1 3 2 2] [4 7 5 6]]
	// [[1 5 2 4] [6 8 7 7]]
	// []
}

func ExampleRegexp_MatchString() {
	re := regexp.MustCompile("(gopher){2}")
	fmt.Println(re.MatchString("gopher"))
	fmt.Println(re.MatchString("gophergopher"))
	fmt.Println(re.MatchString("gophergophergopher"))
	// Output:
	// false
	// true
	// true
}

func ExampleRegexp_ReplaceAllLiteralString() {
	// 首先是一个 a, 然后是任意个 x, 然后是 b, 注意 x 可以不出现. 即使 x 没有出现,整个正则也算匹配成功.
	re := regexp.MustCompile("a(x*)b")
	fmt.Println(re.ReplaceAllLiteralString("-ab-axxb-", "T"))
	fmt.Println(re.ReplaceAllLiteralString("-ab-axxb-", "$1"))
	fmt.Println(re.ReplaceAllLiteralString("-ab-axxb-", "${1}"))
	// Output:
	// -T-T-
	// -$1-$1-
	// -${1}-${1}-
}

func ExampleRegexp_ReplaceAllString() {
	// 首先是一个 a, 然后是任意个 x, 然后是 b, 注意 x 可以不出现. 即使 x 没有出现,整个正则也算匹配成功.
	re := regexp.MustCompile("a(x*)b")
	// -T-T-
	fmt.Println(re.ReplaceAllString("-ab-axxb-", "T"))
	// "$1"的意思是把整个匹配替换为第一个分组
	// 字符串"-ab-axxb-"中总共有两次匹配
	// 第一个匹配匹配到的字符串是'ab',未发生(x*)的匹配,因此替换字符串是空,整个字符串变为变为 "--axxb-"
	// 现在,继续进行第二次匹配位置的替换.
	// 第二个匹配匹配到的字符串是'axxb',$1对应xx,因此变为 "--xx-"
	fmt.Println(re.ReplaceAllString("-ab-axxb-", "$1"))
	// 根据 go doc regexp.Expand 中的描述:
	// In the $name form, name is taken to be as long as possible: $1x is equivalent to ${1x}, not ${1}x, and, $10 is equivalent to ${10}, not ${1}0.
	// 因此,这里实际使用 $1W 进行替换,但是不存在 $1W 这个分组
	// 当分组不存在的时候,根据 regexp.Expand 的描述: A reference to an out of range or unmatched index or a name that is not present in the regular expression is replaced with an empty slice.
	// 因此 $1W 最终是被空字符串替换
	// ---
	fmt.Println(re.ReplaceAllString("-ab-axxb-", "$1W"))
	// 第一个匹配: ab , 子匹配 ${1} 为空 ,结果变为 "-W-axxb-"
	// 第二个匹配: axxb, 子匹配 ${1} 为 'xx', 结果变为 "-W-xxW-"
	fmt.Println(re.ReplaceAllString("-ab-axxb-", "${1}W"))
	// Output:
	// -T-T-
	// --xx-
	// ---
	// -W-xxW-
}

func ExampleRegexp_SubexpNames() {
	re := regexp.MustCompile("(?P<first>[a-zA-Z]+) (?P<last>[a-zA-Z]+)")
	fmt.Println(re.MatchString("Alan Turing"))
	fmt.Printf("%q\n", re.SubexpNames())
	reversed := fmt.Sprintf("${%s} ${%s}", re.SubexpNames()[2], re.SubexpNames()[1])
	fmt.Println(reversed)
	fmt.Println(re.ReplaceAllString("Alan Turing", reversed))
	// Output:
	// true
	// ["" "first" "last"]
	// ${last} ${first}
	// Turing Alan
}

func ExampleRegexp_Split() {
	a := regexp.MustCompile("a")
	fmt.Println(a.Split("banana", -1))
	fmt.Println(a.Split("banana", 0))
	fmt.Println(a.Split("banana", 1))
	fmt.Println(a.Split("banana", 2))
	zp := regexp.MustCompile("z+")
	fmt.Println(zp.Split("pizza", -1))
	fmt.Println(zp.Split("pizza", 0))
	fmt.Println(zp.Split("pizza", 1))
	fmt.Println(zp.Split("pizza", 2))
	// Output:
	// [b n n ]
	// []
	// [banana]
	// [b nana]
	// [pi a]
	// []
	// [pizza]
	// [pi a]
}
