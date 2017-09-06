// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-14 15:17:41

/*
	Package flag implements command-line flag parsing.

	Usage:

	Define flags using flag.String(), Bool(), Int(), etc.

	This declares an integer flag, -flagname, stored in the pointer ip, with type *int.
		import "flag"
		var ip = flag.Int("flagname", 1234, "help message for flagname")
	If you like, you can bind the flag to a variable using the Var() functions.
		var flagvar int
		func init() {
			flag.IntVar(&flagvar, "flagname", 1234, "help message for flagname")
		}
	Or you can create custom flags that satisfy the Value interface (with
	pointer receivers) and couple them to flag parsing by
		flag.Var(&flagVal, "name", "help message for flagname")
	For such flags, the default value is just the initial value of the variable.

	After all flags are defined, call
		flag.Parse()
	to parse the command line into the defined flags.

	Flags may then be used directly. If you're using the flags themselves,
	they are all pointers; if you bind to variables, they're values.
		fmt.Println("ip has value ", *ip)
		fmt.Println("flagvar has value ", flagvar)

	After parsing, the arguments following the flags are available as the
	slice flag.Args() or individually as flag.Arg(i).
	The arguments are indexed from 0 through flag.NArg()-1.

	Command line flag syntax:
		-flag
		-flag=x
		-flag x  // non-boolean flags only
	One or two minus signs may be used; they are equivalent.
	The last form is not permitted for boolean flags because the
	meaning of the command
		cmd -x *
	will change if there is a file called 0, false, etc.  You must
	use the -flag=false form to turn off a boolean flag.

	Flag parsing stops just before the first non-flag argument
	("-" is a non-flag argument) or after the terminator "--".

	Integer flags accept 1234, 0664, 0x1234 and may be negative.
	Boolean flags may be:
		1, 0, t, f, T, F, true, false, TRUE, FALSE, True, False
	Duration flags accept any input valid for time.ParseDuration.

	The default set of command-line flags is controlled by
	top-level functions.  The FlagSet type allows one to define
	independent sets of flags, such as to implement subcommands
	in a command-line interface. The methods of FlagSet are
	analogous to the top-level functions for the command-line
	flag set.
*/
package flag

import (
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"time"
)

// ErrHelp is the error returned if the -help or -h flag is invoked
// but no such flag is defined.
//
// 命令发起时调用了 -h 或 -help, 但是没有 -h 或 -help 的 flag 被定义
var ErrHelp = errors.New("flag: help requested")

// -- bool Value
//
// 实现了 Value 和 Getter 接口
type boolValue bool


// val 代表 default value
func newBoolValue(val bool, p *bool) *boolValue {
	*p = val
	return (*boolValue)(p)
}

func (b *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	*b = boolValue(v)
	return err
}

func (b *boolValue) Get() interface{} { return bool(*b) }

// fmt 包的 %v::: the value in a default format when printing structs, the plus flag (%+v) adds field names
// %v 任意一个值的默认格式
func (b *boolValue) String() string { return strconv.FormatBool(bool(*b)) }

// 实现 boolFlag 接口
func (b *boolValue) IsBoolFlag() bool { return true }

// optional interface to indicate boolean flags that can be
// supplied without "=value" text
type boolFlag interface {
	Value
	IsBoolFlag() bool
}

// -- int Value
// 实现了 Value 和 Getter 接口
type intValue int

// val 代表 default value
func newIntValue(val int, p *int) *intValue {
	*p = val
	return (*intValue)(p)
}

func (i *intValue) Set(s string) error {
	// 参数0表示根据字符串前缀来决定base进制
	v, err := strconv.ParseInt(s, 0, strconv.IntSize)
	// 必须转换为intValue后才能赋值给*i
	*i = intValue(v)
	return err
}

// 获取实际存储的int值
func (i *intValue) Get() interface{} { return int(*i) }

func (i *intValue) String() string { return strconv.Itoa(int(*i)) }

// -- int64 Value
type int64Value int64

// val 代表 default value
func newInt64Value(val int64, p *int64) *int64Value {
	*p = val
	return (*int64Value)(p)
	// 现在,参数p和返回的指针*int64Value都指向同一个存储区
	// 也就是指向了同一个数据
	// 通过p和返回的指针进行访问都是相同的,因为底层存储都是int64
}

func (i *int64Value) Set(s string) error {
	// 参数0表示根据字符串前缀来决定base进制
	v, err := strconv.ParseInt(s, 0, 64)
	*i = int64Value(v)
	return err
}

func (i *int64Value) Get() interface{} { return int64(*i) }

func (i *int64Value) String() string { return strconv.FormatInt(int64(*i), 10) }

// -- uint Value
type uintValue uint

// val 代表 default value
func newUintValue(val uint, p *uint) *uintValue {
	*p = val
	return (*uintValue)(p)
}

func (i *uintValue) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, strconv.IntSize)
	*i = uintValue(v)
	return err
}

func (i *uintValue) Get() interface{} { return uint(*i) }

func (i *uintValue) String() string { return strconv.FormatUint(uint64(*i), 10) }

// -- uint64 Value
type uint64Value uint64

// val 代表 default value
func newUint64Value(val uint64, p *uint64) *uint64Value {
	*p = val
	return (*uint64Value)(p)
}

func (i *uint64Value) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	*i = uint64Value(v)
	return err
}

func (i *uint64Value) Get() interface{} { return uint64(*i) }

func (i *uint64Value) String() string { return strconv.FormatUint(uint64(*i), 10) }

// -- string Value
type stringValue string

// val 代表 default value
func newStringValue(val string, p *string) *stringValue {
	*p = val
	return (*stringValue)(p)
}

func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}

// *s 是 stringValue 类型,因此还是得用 string(*s) 进行类型转换得到底层的 string
func (s *stringValue) Get() interface{} { return string(*s) }

func (s *stringValue) String() string { return string(*s) }

// -- float64 Value
type float64Value float64

// val 代表 default value
func newFloat64Value(val float64, p *float64) *float64Value {
	*p = val
	return (*float64Value)(p)
}

func (f *float64Value) Set(s string) error {
	v, err := strconv.ParseFloat(s, 64)
	*f = float64Value(v)
	return err
}

func (f *float64Value) Get() interface{} { return float64(*f) }

func (f *float64Value) String() string { return strconv.FormatFloat(float64(*f), 'g', -1, 64) }

// -- time.Duration Value
type durationValue time.Duration

// val 代表 default value
func newDurationValue(val time.Duration, p *time.Duration) *durationValue {
	*p = val
	return (*durationValue)(p)
}

func (d *durationValue) Set(s string) error {
	v, err := time.ParseDuration(s)
	*d = durationValue(v)
	return err
}

func (d *durationValue) Get() interface{} { return time.Duration(*d) }

// 实际是调用 func (d Duration) String() string , 返回值意义见其文档
// d 类型为 *durationValue, 需要转型为 *time.Duration 后才能使用 Duration.String 方法
func (d *durationValue) String() string { return (*time.Duration)(d).String() }

// Value is the interface to the dynamic value stored in a flag.
// (The default value is represented as a string.)
//
// If a Value has an IsBoolFlag() bool method returning true,
// the command-line parser makes -name equivalent to -name=true
// rather than using the next command-line argument.
//
// Set is called once, in command line order, for each flag present.
// The flag package may call the String method with a zero-valued receiver,
// such as a nil pointer.
type Value interface {
	String() string
	Set(string) error
}

// Getter is an interface that allows the contents of a Value to be retrieved.
// It wraps the Value interface, rather than being part of it, because it
// appeared after Go 1 and its compatibility rules. All Value types provided
// by this package satisfy the Getter interface.
//
// 本包中所有 Value 类型都满足 Getter 接口(历史遗留原因)
type Getter interface {
	// 内嵌匿名interface
	Value
	Get() interface{}
}

// ErrorHandling defines how FlagSet.Parse behaves if the parse fails.
type ErrorHandling int

// These constants cause FlagSet.Parse to behave as described if the parse fails.
const (
	ContinueOnError ErrorHandling = iota // Return a descriptive error.
	ExitOnError                          // Call os.Exit(2).
	PanicOnError                         // Call panic with a descriptive error.
)

// A FlagSet represents a set of defined flags. The zero value of a FlagSet
// has no name and has ContinueOnError error handling.
type FlagSet struct {
	// Usage is the function called when an error occurs while parsing flags.
	// The field is a function (not a method) that may be changed to point to
	// a custom error handler. What happens after Usage is called depends
	// on the ErrorHandling setting; for the command line, this defaults
	// to ExitOnError, which exits the program after calling Usage.
	//
	// Usage 字段类型 是一个函数, 这个函数的签名是 func(), 无参数,无返回值
	// 当 parsing flags 时发生错误,会调用此函数
	Usage func()

	// 参考 func NewFlagSet, 这个name字段实际是NewFlagSet的第一个参数
	// func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	// 本文件中搜索 f.name 可以看到哪些地方在用这个字段
	name          string
	// 是否已经被解析, 是否已经被 func (f *FlagSet) Parse(arguments []string) error 方法调用过
	// 在func (f *FlagSet) Parse方法中,做的第一件事情就是将 parsed 字段标记为 true
	parsed        bool
	// actual 代表实际的,已经被解析的
	actual        map[string]*Flag
	// formal 代表形式上的,所有需要被解析的
	formal        map[string]*Flag
	args          []string // arguments after flags
	// 参考 func NewFlagSet, 这个 errorHandling 字段实际是NewFlagSet的第二个参数
	// func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	errorHandling ErrorHandling
	// 参见 *FlagSet 的 out 方法
	output        io.Writer // nil means stderr; use out() accessor
}

// A Flag represents the state of a flag.
type Flag struct {
	// find usages看看这个字段在哪里显示
	Name     string // name as it appears on command line
	Usage    string // help message
	Value    Value  // value as set
	// usage message 中显示的默认值
	DefValue string // default value (as text); for usage message
}

// sortFlags returns the flags as a slice in lexicographical sorted order.
//
// sortFlags会将参数flags按照每个flag的name字段进行字典序排序
func sortFlags(flags map[string]*Flag) []*Flag {
	// 以下,从 flags 这个 map 中构造出 list, StringSlice 的底层类型是 []string
	// 在 sort 包中定义了 type StringSlice []string

	// 首先构造出用于排序的slice. 注意这里make的是sort.StringSlice,是没有问题的
	list := make(sort.StringSlice, len(flags))
	// 将 map flags 中的 f.Name copy 到 list
	i := 0
	for _, f := range flags {
		list[i] = f.Name
		i++
	}
	// list构造完毕,进行排序
	list.Sort()
	// list 现在已经是按照Name字段排序好的

	// 初始化将要返回的 result
	result := make([]*Flag, len(list))
	for i, name := range list {
		result[i] = flags[name]
	}
	return result
}

// output字段的getter
// 这个getter非导出
func (f *FlagSet) out() io.Writer {
	if f.output == nil {
		return os.Stderr
	}
	return f.output
}

// SetOutput sets the destination for usage and error messages.
// If output is nil, os.Stderr is used.
//
// output字段的setter
// 这个setter是导出的
func (f *FlagSet) SetOutput(output io.Writer) {
	f.output = output
}

// VisitAll visits the flags in lexicographical order, calling fn for each.
// It visits all flags, even those not set.
//
// lexicographical order(字典序)
func (f *FlagSet) VisitAll(fn func(*Flag)) {
	// f.formal 代表 all flags, even those not set.
	for _, flag := range sortFlags(f.formal) {
		fn(flag)
	}
}

// VisitAll visits the command-line flags in lexicographical order, calling
// fn for each. It visits all flags, even those not set.
func VisitAll(fn func(*Flag)) {
	CommandLine.VisitAll(fn)
}

// Visit visits the flags in lexicographical order, calling fn for each.
// It visits only those flags that have been set.
func (f *FlagSet) Visit(fn func(*Flag)) {
	// 注: f.actual:flags that have been set
	for _, flag := range sortFlags(f.actual) {
		fn(flag)
	}
}

// Visit visits the command-line flags in lexicographical order, calling fn
// for each. It visits only those flags that have been set.
func Visit(fn func(*Flag)) {
	CommandLine.Visit(fn)
}

// Lookup returns the Flag structure of the named flag, returning nil if none exists.
func (f *FlagSet) Lookup(name string) *Flag {
	// f.formal 代表 all flags, even those not set.
	return f.formal[name]
}

// Lookup returns the Flag structure of the named command-line flag,
// returning nil if none exists.
func Lookup(name string) *Flag {
	return CommandLine.formal[name]
}

// Set sets the value of the named flag.
func (f *FlagSet) Set(name, value string) error {
	// 注意:包名是flag,但是这里仍然可以使用一个叫flag的变量
	flag, ok := f.formal[name]
	if !ok {
		return fmt.Errorf("no such flag -%v", name)
	}
	// 这个flag是指上方的变量,而不是flag包
	err := flag.Value.Set(value)
	if err != nil {
		return err
	}
	if f.actual == nil {
		// 初始化actual容器
		f.actual = make(map[string]*Flag)
	}
	// 已经在FlagSet.Set调用过的并且Set成功,会在actual字段进行记录
	f.actual[name] = flag
	return nil
}

// Set sets the value of the named command-line flag.
func Set(name, value string) error {
	return CommandLine.Set(name, value)
}

// isZeroValue guesses whether the string represents the zero
// value for a flag. It is not accurate but in practice works OK.
//
// isZeroValue猜测字符串value是否代表了flag的zero value
// 它不是理论上精确的但实践中是没有问题的
func isZeroValue(flag *Flag, value string) bool {
	// Build a zero value of the flag's Value type, and see if the
	// result of calling its String method equals the value passed in.
	// This works unless the Value type is itself an interface type.
	// 注意: 这里的flag是变量,而不是包名
	typ := reflect.TypeOf(flag.Value)
	// 声明 z 变量,它代表了我们根据 flag.Value 反射出来的 zero value
	// 这里只是声明, 赋值是在下方的 if else 中
	var z reflect.Value
	// 计算 z, 也就是 flag.Value 反射出来的 zero value
	if typ.Kind() == reflect.Ptr {
		// 如果 flag.Value 实际是指针
		//     typ.Elem() 会得到指针指向的元素对应的 reflect.Type
		//     reflect.New 返回参数对应的零值
		z = reflect.New(typ.Elem())
	} else {
		// 如果 flag.Value 实际不是指针, 直接通过 reflect.Zero 获取对应类型的零值
		z = reflect.Zero(typ)
	}
	// 现在 z 代表了 typ 类型对应的 zero value
	// z类型是 reflect.Value
	// 首先通过 z.Interface() 得到 z 的底层value. (z.Interface() 会返回空接口, interface{}, 因此能容纳任意值)
	// 但是,我们不能直接在z.Interface()返回值上面调用String()方法
	// 必须首先通过 type assertion转换为 flag.Value 后,才能调用 String() 方法
	if value == z.Interface().(Value).String() {
		return true
	}

	switch value {
	case "false":
		// bool的零值
		return true
	case "":
		// string的零值
		return true
	case "0":
		// int的零值
		return true
	}
	return false
}

// UnquoteUsage extracts a back-quoted name from the usage
// string for a flag and returns it and the un-quoted usage.
// Given "a `name` to show" it returns ("name", "a name to show").
// If there are no back quotes, the name is an educated guess of the
// type of the flag's value, or the empty string if the flag is boolean.
//
// educated guess: n. 有根据的推测
func UnquoteUsage(flag *Flag) (name string, usage string) {
	// Look for a back-quoted name, but avoid the strings package.
	usage = flag.Usage
	// 从usage开始处开始循环,找到第一个`,进行第二个for循环,找到第二个`
	// 此时usage[i]代表第一个`,usage[j]代表第二个`
	for i := 0; i < len(usage); i++ {
		// 找到第一个`
		if usage[i] == '`' {
			for j := i + 1; j < len(usage); j++ {
				// 找到第二个`
				if usage[j] == '`' {
					name = usage[i+1 : j]
					usage = usage[:i] + name + usage[j+1:]
					// 如果找到了,整个函数退出
					return name, usage
				}
			}
			// 运行到这里,说明只找到了一个back quote,跳出循环
			break // Only one back quote; use type name.
		}
	}
	// 现在,说明要么没有`,要么只找到一个`
	// No explicit name, so use type if we can find one.
	name = "value"
	switch flag.Value.(type) {
	case boolFlag:
		name = ""
	case *durationValue:
		name = "duration"
	case *float64Value:
		name = "float"
	case *intValue, *int64Value:
		name = "int"
	case *stringValue:
		name = "string"
	case *uintValue, *uint64Value:
		name = "uint"
	}
	return
}

// PrintDefaults prints to standard error the default values of all
// defined command-line flags in the set. See the documentation for
// the global function PrintDefaults for more information.
//
// 输出所有已定义的 flags 的默认值
func (f *FlagSet) PrintDefaults() {
	/**
	以这个为例:
	flag.String("I", "", "search `directory` for include files")
	会输出
	-I directory
		search directory for include files.
	 */
	f.VisitAll(func(flag *Flag) {
		// 输出 '  -I'
		s := fmt.Sprintf("  -%s", flag.Name) // Two spaces before -; see next two comments.
		name, usage := UnquoteUsage(flag)
		if len(name) > 0 {
			// 输出 ' directory'
			s += " " + name
		}
		// Boolean flags of one ASCII letter are so common we
		// treat them specially, putting their usage on the same line.
		if len(s) <= 4 { // space, space, '-', 'x'.
			// 如果是Boolean flags of one ASCII letter,则后续文字在同一行
			s += "\t"
		} else {
			// Four spaces before the tab triggers good alignment
			// for both 4- and 8-space tab stops.
			s += "\n    \t"
		}
		s += usage
		// 根据 func PrintDefaults() { 的文档, The parenthetical default is omitted if the default is the zero value for the type.
		// 因此下面判断, 如果 flag.DefValue 不是 flag 的 zero value, 才输出 '(default xxx)' 区域; 否则, 如果 flag.DefValue 是 flag 的 zero value, 不输出 '(default xxx)' 区域;
		if !isZeroValue(flag, flag.DefValue) {
			// 输出 '(default xxx)' 区域, 如果是字符串,会通过 fmt 的 %q 来用引号围住 xxx
			if _, ok := flag.Value.(*stringValue); ok {
				// put quotes on the value
				s += fmt.Sprintf(" (default %q)", flag.DefValue)
			} else {
				s += fmt.Sprintf(" (default %v)", flag.DefValue)
			}
		}
		// 每个 flag 的信息用 换行分隔
		fmt.Fprint(f.out(), s, "\n")
	})
}

// PrintDefaults prints, to standard error unless configured otherwise,
// a usage message showing the default settings of all defined
// command-line flags.
// For an integer valued flag x, the default output has the form
//	-x int
//		usage-message-for-x (default 7)
// The usage message will appear on a separate line for anything but
// a bool flag with a one-byte name. For bool flags, the type is
// omitted and if the flag name is one byte the usage message appears
// on the same line. The parenthetical default is omitted if the
// default is the zero value for the type. The listed type, here int,
// can be changed by placing a back-quoted name in the flag's usage
// string; the first such item in the message is taken to be a parameter
// name to show in the message and the back quotes are stripped from
// the message when displayed. For instance, given
//	flag.String("I", "", "search `directory` for include files")
// the output will be
//	-I directory
//		search directory for include files.
//
// parenthetical [,pær(ə)n'θetɪk(ə)l] adj. 插句的；附加说明的；放在括号里的
//
// 对于 a bool flag with a one-byte name, usage message 是在同一行中出现. 其他所有情况, usage message 是在单独的行中出现.
//
// 对于 bool flag, type 是被忽略的(上面例子中的int). 并且如果 flag name 是单字节, usage message 会在同一行出现.
func PrintDefaults() {
	CommandLine.PrintDefaults()
}

// defaultUsage is the default function to print a usage message.
func (f *FlagSet) defaultUsage() {
	// f.name 代表了什么
	// 请参考以下两处
	// var CommandLine = NewFlagSet(os.Args[0], ExitOnError)
	// func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	if f.name == "" {
		fmt.Fprintf(f.out(), "Usage:\n")
	} else {
		fmt.Fprintf(f.out(), "Usage of %s:\n", f.name)
	}
	f.PrintDefaults()
}

// NOTE: Usage is not just defaultUsage(CommandLine)
// because it serves (via godoc flag Usage) as the example
// for how to write your own usage function.

// Usage prints a usage message documenting all defined command-line flags
// to CommandLine's output, which by default is os.Stderr.
// It is called when an error occurs while parsing flags.
// The function is a variable that may be changed to point to a custom function.
// By default it prints a simple header and calls PrintDefaults; for details about the
// format of the output and how to control it, see the documentation for PrintDefaults.
// Custom usage functions may choose to exit the program; by default exiting
// happens anyway as the command line's error handling strategy is set to
// ExitOnError.
var Usage = func() {
	fmt.Fprintf(CommandLine.out(), "Usage of %s:\n", os.Args[0])
	PrintDefaults()
}

// NFlag returns the number of flags that have been set.
//
// 有多少个 flags 已经被 set
func (f *FlagSet) NFlag() int { return len(f.actual) }

// NFlag returns the number of command-line flags that have been set.
func NFlag() int { return len(CommandLine.actual) }

// Arg returns the i'th argument. Arg(0) is the first remaining argument
// after flags have been processed. Arg returns an empty string if the
// requested element does not exist.
func (f *FlagSet) Arg(i int) string {
	if i < 0 || i >= len(f.args) {
		return ""
	}
	return f.args[i]
}

// Arg returns the i'th command-line argument. Arg(0) is the first remaining argument
// after flags have been processed. Arg returns an empty string if the
// requested element does not exist.
func Arg(i int) string {
	return CommandLine.Arg(i)
}

// NArg is the number of arguments remaining after flags have been processed.
func (f *FlagSet) NArg() int { return len(f.args) }

// NArg is the number of arguments remaining after flags have been processed.
func NArg() int { return len(CommandLine.args) }

// Args returns the non-flag arguments.
func (f *FlagSet) Args() []string { return f.args }

// Args returns the non-flag command-line arguments.
func Args() []string { return CommandLine.args }

// BoolVar defines a bool flag with specified name, default value, and usage string.
// The argument p points to a bool variable in which to store the value of the flag.
//
// 使用方式如下:
// var flagvar int
//
// flag.IntVar(&flagvar, "flagname", 1234, "help message for flagname")
//
// 之后可以从变量flagvar获取值.
func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) {
	// f.Var 要求第一个参数满足 flag.Value 接口
	// newBoolValue()的返回值满足 flag.Value 接口
	f.Var(newBoolValue(value, p), name, usage)
}

// BoolVar defines a bool flag with specified name, default value, and usage string.
// The argument p points to a bool variable in which to store the value of the flag.
func BoolVar(p *bool, name string, value bool, usage string) {
	CommandLine.Var(newBoolValue(value, p), name, usage)
}

// Bool defines a bool flag with specified name, default value, and usage string.
// The return value is the address of a bool variable that stores the value of the flag.
func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVar(p, name, value, usage)
	return p
	// 之后可以通过(*p)获取对应值
}

// Bool defines a bool flag with specified name, default value, and usage string.
// The return value is the address of a bool variable that stores the value of the flag.
func Bool(name string, value bool, usage string) *bool {
	return CommandLine.Bool(name, value, usage)
}

// IntVar defines an int flag with specified name, default value, and usage string.
// The argument p points to an int variable in which to store the value of the flag.
func (f *FlagSet) IntVar(p *int, name string, value int, usage string) {
	f.Var(newIntValue(value, p), name, usage)
}

// IntVar defines an int flag with specified name, default value, and usage string.
// The argument p points to an int variable in which to store the value of the flag.
func IntVar(p *int, name string, value int, usage string) {
	CommandLine.Var(newIntValue(value, p), name, usage)
}

// Int defines an int flag with specified name, default value, and usage string.
// The return value is the address of an int variable that stores the value of the flag.
func (f *FlagSet) Int(name string, value int, usage string) *int {
	p := new(int)
	f.IntVar(p, name, value, usage)
	return p
}

// Int defines an int flag with specified name, default value, and usage string.
// The return value is the address of an int variable that stores the value of the flag.
func Int(name string, value int, usage string) *int {
	return CommandLine.Int(name, value, usage)
}

// Int64Var defines an int64 flag with specified name, default value, and usage string.
// The argument p points to an int64 variable in which to store the value of the flag.
func (f *FlagSet) Int64Var(p *int64, name string, value int64, usage string) {
	f.Var(newInt64Value(value, p), name, usage)
}

// Int64Var defines an int64 flag with specified name, default value, and usage string.
// The argument p points to an int64 variable in which to store the value of the flag.
func Int64Var(p *int64, name string, value int64, usage string) {
	CommandLine.Var(newInt64Value(value, p), name, usage)
}

// Int64 defines an int64 flag with specified name, default value, and usage string.
// The return value is the address of an int64 variable that stores the value of the flag.
func (f *FlagSet) Int64(name string, value int64, usage string) *int64 {
	p := new(int64)
	f.Int64Var(p, name, value, usage)
	return p
}

// Int64 defines an int64 flag with specified name, default value, and usage string.
// The return value is the address of an int64 variable that stores the value of the flag.
func Int64(name string, value int64, usage string) *int64 {
	return CommandLine.Int64(name, value, usage)
}

// UintVar defines a uint flag with specified name, default value, and usage string.
// The argument p points to a uint variable in which to store the value of the flag.
func (f *FlagSet) UintVar(p *uint, name string, value uint, usage string) {
	f.Var(newUintValue(value, p), name, usage)
}

// UintVar defines a uint flag with specified name, default value, and usage string.
// The argument p points to a uint variable in which to store the value of the flag.
func UintVar(p *uint, name string, value uint, usage string) {
	CommandLine.Var(newUintValue(value, p), name, usage)
}

// Uint defines a uint flag with specified name, default value, and usage string.
// The return value is the address of a uint variable that stores the value of the flag.
func (f *FlagSet) Uint(name string, value uint, usage string) *uint {
	p := new(uint)
	f.UintVar(p, name, value, usage)
	return p
}

// Uint defines a uint flag with specified name, default value, and usage string.
// The return value is the address of a uint variable that stores the value of the flag.
func Uint(name string, value uint, usage string) *uint {
	return CommandLine.Uint(name, value, usage)
}

// Uint64Var defines a uint64 flag with specified name, default value, and usage string.
// The argument p points to a uint64 variable in which to store the value of the flag.
func (f *FlagSet) Uint64Var(p *uint64, name string, value uint64, usage string) {
	f.Var(newUint64Value(value, p), name, usage)
}

// Uint64Var defines a uint64 flag with specified name, default value, and usage string.
// The argument p points to a uint64 variable in which to store the value of the flag.
func Uint64Var(p *uint64, name string, value uint64, usage string) {
	CommandLine.Var(newUint64Value(value, p), name, usage)
}

// Uint64 defines a uint64 flag with specified name, default value, and usage string.
// The return value is the address of a uint64 variable that stores the value of the flag.
func (f *FlagSet) Uint64(name string, value uint64, usage string) *uint64 {
	p := new(uint64)
	f.Uint64Var(p, name, value, usage)
	return p
}

// Uint64 defines a uint64 flag with specified name, default value, and usage string.
// The return value is the address of a uint64 variable that stores the value of the flag.
func Uint64(name string, value uint64, usage string) *uint64 {
	return CommandLine.Uint64(name, value, usage)
}

// StringVar defines a string flag with specified name, default value, and usage string.
// The argument p points to a string variable in which to store the value of the flag.
func (f *FlagSet) StringVar(p *string, name string, value string, usage string) {
	f.Var(newStringValue(value, p), name, usage)
}

// StringVar defines a string flag with specified name, default value, and usage string.
// The argument p points to a string variable in which to store the value of the flag.
func StringVar(p *string, name string, value string, usage string) {
	CommandLine.Var(newStringValue(value, p), name, usage)
}

// String defines a string flag with specified name, default value, and usage string.
// The return value is the address of a string variable that stores the value of the flag.
func (f *FlagSet) String(name string, value string, usage string) *string {
	p := new(string)
	f.StringVar(p, name, value, usage)
	return p
}

// String defines a string flag with specified name, default value, and usage string.
// The return value is the address of a string variable that stores the value of the flag.
func String(name string, value string, usage string) *string {
	return CommandLine.String(name, value, usage)
}

// Float64Var defines a float64 flag with specified name, default value, and usage string.
// The argument p points to a float64 variable in which to store the value of the flag.
func (f *FlagSet) Float64Var(p *float64, name string, value float64, usage string) {
	f.Var(newFloat64Value(value, p), name, usage)
}

// Float64Var defines a float64 flag with specified name, default value, and usage string.
// The argument p points to a float64 variable in which to store the value of the flag.
func Float64Var(p *float64, name string, value float64, usage string) {
	CommandLine.Var(newFloat64Value(value, p), name, usage)
}

// Float64 defines a float64 flag with specified name, default value, and usage string.
// The return value is the address of a float64 variable that stores the value of the flag.
func (f *FlagSet) Float64(name string, value float64, usage string) *float64 {
	p := new(float64)
	f.Float64Var(p, name, value, usage)
	return p
}

// Float64 defines a float64 flag with specified name, default value, and usage string.
// The return value is the address of a float64 variable that stores the value of the flag.
func Float64(name string, value float64, usage string) *float64 {
	return CommandLine.Float64(name, value, usage)
}

// DurationVar defines a time.Duration flag with specified name, default value, and usage string.
// The argument p points to a time.Duration variable in which to store the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func (f *FlagSet) DurationVar(p *time.Duration, name string, value time.Duration, usage string) {
	f.Var(newDurationValue(value, p), name, usage)
}

// DurationVar defines a time.Duration flag with specified name, default value, and usage string.
// The argument p points to a time.Duration variable in which to store the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func DurationVar(p *time.Duration, name string, value time.Duration, usage string) {
	CommandLine.Var(newDurationValue(value, p), name, usage)
}

// Duration defines a time.Duration flag with specified name, default value, and usage string.
// The return value is the address of a time.Duration variable that stores the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func (f *FlagSet) Duration(name string, value time.Duration, usage string) *time.Duration {
	p := new(time.Duration)
	f.DurationVar(p, name, value, usage)
	return p
}

// Duration defines a time.Duration flag with specified name, default value, and usage string.
// The return value is the address of a time.Duration variable that stores the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func Duration(name string, value time.Duration, usage string) *time.Duration {
	return CommandLine.Duration(name, value, usage)
}

// Var defines a flag with the specified name and usage string. The type and
// value of the flag are represented by the first argument, of type Value, which
// typically holds a user-defined implementation of Value. For instance, the
// caller could create a flag that turns a comma-separated string into a slice
// of strings by giving the slice the methods of Value; in particular, Set would
// decompose the comma-separated string into the slice.
//
//
// Var 其实是在 f.formal 新增记录, 如果 f.formal 中之前已经定义了,会报告错误,提示重复定义
//
// decompose [diːkəm'pəʊz] vt. 分解；使腐烂 vi. 分解；腐烂
// 这里列一下 Value 的定义
// type Value interface {
// 	String() string
// 	Set(string) error
// }
func (f *FlagSet) Var(value Value, name string, usage string) {
	// Remember the default value as a string; it won't change.
	// 注意: value.String()是flag默认值的来源
	flag := &Flag{name, usage, value, value.String()}
	_, alreadythere := f.formal[name]
	if alreadythere {
		// 此时应该 panic 告诉用户 flag 重复定义, 下面计算应该提示用户的 msg
		var msg string
		if f.name == "" {
			msg = fmt.Sprintf("flag redefined: %s", name)
		} else {
			msg = fmt.Sprintf("%s flag redefined: %s", f.name, name)
		}
		fmt.Fprintln(f.out(), msg)
		panic(msg) // Happens only if flags are declared with identical names
	}
	if f.formal == nil {
		// 确保f.formal这个map已经被初始化
		f.formal = make(map[string]*Flag)
	}
	f.formal[name] = flag
}

// Var defines a flag with the specified name and usage string. The type and
// value of the flag are represented by the first argument, of type Value, which
// typically holds a user-defined implementation of Value. For instance, the
// caller could create a flag that turns a comma-separated string into a slice
// of strings by giving the slice the methods of Value; in particular, Set would
// decompose the comma-separated string into the slice.
func Var(value Value, name string, usage string) {
	CommandLine.Var(value, name, usage)
}

// failf prints to standard error a formatted error and usage message and
// returns the error.
func (f *FlagSet) failf(format string, a ...interface{}) error {
	err := fmt.Errorf(format, a...)
	fmt.Fprintln(f.out(), err)
	f.usage()
	return err
}

// usage calls the Usage method for the flag set if one is specified,
// or the appropriate default usage function otherwise.
func (f *FlagSet) usage() {
	if f.Usage == nil {
		// 默认情况下, f.Usage 是 nil
		f.defaultUsage()
	} else {
		// 如果 f.Usage 不是 nil, 也就是用户自己设置了 f.Usage 字段(函数类型)
		f.Usage()
	}
}

// parseOne parses one flag. It reports whether a flag was seen.
//
// 每当一个flag被parse之后,f.actual[name]会被设置
// 返回的bool: 是否解析出来一个
// 返回的error: 解析时发生的错误
func (f *FlagSet) parseOne() (bool, error) {
	if len(f.args) == 0 {
		// 后面已经没有需要解析的了
		return false, nil
	}
	// 后面还有需要进行解析的,准备开始解析,这里通过f.args[0]取出一个来进行解析
	s := f.args[0]
	if len(s) < 2 || s[0] != '-' {
		// 如果s长度为0,说明后面没有参数了
		// 注意在flag包开始处提到: Flag parsing stops just before the first non-flag
		// argument ("-" is a non-flag argument) or after the terminator "--".
		return false, nil
	}
	// 现在, s的长度至少大于1,并且以 '-' 开头

	// numMinuses代表了有多少个减号
	// 初始情况下,有一个减号
	numMinuses := 1
	if s[1] == '-' {
		// 如果s的第二个字符也是减号
		numMinuses++
		if len(s) == 2 { // "--" terminates the flags
			// 注意在flag包开始处提到: Flag parsing stops just before the first non-flag
			// argument ("-" is a non-flag argument) or after the terminator "--".
			// 修正f.args
			f.args = f.args[1:]
			return false, nil
		}
	}
	// numMinuses代表s以几个连续的减号开头,要么是1,要么是2, 比如 -flag, --flag
	// name代表了减号后面的字符串
	name := s[numMinuses:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		// 减号后面没有其他字符 || 减号后面又是减号 || 减号后面是等号
		return false, f.failf("bad flag syntax: %s", s)
	}

	// 现在, name 是 - 之后的字符串, flag 的语法设置也没问题

	// 注:Command line flag syntax:
	// -flag
	// -flag=x
	// -flag x  // non-boolean flags only

	// it's a flag. does it have an argument?
	f.args = f.args[1:]
	// hasValue 代表此 flag 是否有 argument, 初始为 false
	hasValue := false
	value := ""
	// =号不可能是name的首字符(上方已经判断),因此从i=1开始循环
	for i := 1; i < len(name); i++ { // equals cannot be first
		if name[i] == '=' {
			// 如果当前循环是等号, value是等号之后的部分, name修正为等号之前的部分
			value = name[i+1:]
			hasValue = true
			name = name[0:i]
			break
		}
	}
	// 上面这段代码,根据 name中是否有等号来决定 hasValue
	// 但是命令行调用还有另一种格式: -flag x  // non-boolean flags only
	// 这是在下方处理的

	// formal 代表形式上的,所有需要被解析的 flags
	m := f.formal
	flag, alreadythere := m[name] // BUG
	if !alreadythere {
		// name 不存在
		if name == "help" || name == "h" { // special case for nice help message.
			// 如果name是 "help"
			f.usage()
			return false, ErrHelp
		}
		// parse的时候遇到了未定义的flag
		return false, f.failf("flag provided but not defined: -%s", name)
	}

	// boolFlag 接口定义了 IsBoolFlag() bool 方法
	// 注意下行的flag是变量而不是包名
	if fv, ok := flag.Value.(boolFlag); ok && fv.IsBoolFlag() { // special case: doesn't need an arg
		// 此 flag 不需要 argument
		if hasValue {
			// 如果 对 name 这个 flag 提供了值 value
			if err := fv.Set(value); err != nil {
				// 尝试设置 value 给 fv 这个 flag 失败
				return false, f.failf("invalid boolean value %q for -%s: %v", value, name, err)
			}
		} else {
			// 如果 对 name 这个 flag 没有提供值
			if err := fv.Set("true"); err != nil {
				// 尝试用 true 来设置 fv 这个 flag 失败
				return false, f.failf("invalid boolean flag %s: %v", name, err)
			}
			// 如果 尝试用 true 来设置 fv 这个 flag 成功的情况下, 也就是说 bool 的 flag, 如果没有提供值, 默认为 true
			// 比如: './req -https http://www.163.com/', 此时-https会默认为true
		}
	} else {
		// 处理  '-flag x' 这种格式的 flag
		// It must have a value, which might be the next argument.
		if !hasValue && len(f.args) > 0 {
			// value is the next arg
			hasValue = true
			value, f.args = f.args[0], f.args[1:]
		}
		if !hasValue {
			return false, f.failf("flag needs an argument: -%s", name)
		}
		if err := flag.Value.Set(value); err != nil {
			return false, f.failf("invalid value %q for flag -%s: %v", value, name, err)
		}
	}
	if f.actual == nil {
		// 确保f.actual可以使用
		f.actual = make(map[string]*Flag)
	}
	// parse 成功, 设置到 f.actual
	f.actual[name] = flag
	return true, nil
}

// Parse parses flag definitions from the argument list, which should not
// include the command name. Must be called after all flags in the FlagSet
// are defined and before flags are accessed by the program.
// The return value will be ErrHelp if -help or -h were set but not defined.
func (f *FlagSet) Parse(arguments []string) error {
	// 标记已Parse状态
	f.parsed = true
	// f.args : arguments after flags
	f.args = arguments
	for {
		seen, err := f.parseOne()
		if seen {
			// 如果成功解析出一个
			continue
		}
		// 现在,没有解析出
		if err == nil {
			// 没有解析出 && err == nil : 说明已经全部解析完毕
			break
		}
		// 现在,有错, 根据 f.errorHandling 的设置进行相应错误处理
		switch f.errorHandling {
		case ContinueOnError:
			return err
		case ExitOnError:
			os.Exit(2)
		case PanicOnError:
			panic(err)
		}
	}
	return nil
}

// Parsed reports whether f.Parse has been called.
func (f *FlagSet) Parsed() bool {
	return f.parsed
}

// Parse parses the command-line flags from os.Args[1:]. Must be called
// after all flags are defined and before flags are accessed by the program.
func Parse() {
	// Ignore errors; CommandLine is set for ExitOnError.
	CommandLine.Parse(os.Args[1:])
}

// Parsed reports whether the command-line flags have been parsed.
func Parsed() bool {
	return CommandLine.Parsed()
}

// CommandLine is the default set of command-line flags, parsed from os.Args.
// The top-level functions such as BoolVar, Arg, and so on are wrappers for the
// methods of CommandLine.
//
// 注意 os.Args[0], 代表了 'program name'
var CommandLine = NewFlagSet(os.Args[0], ExitOnError)

func init() {
	// Override generic FlagSet default Usage with call to global Usage.
	// Note: This is not CommandLine.Usage = Usage,
	// because we want any eventual call to use any updated value of Usage,
	// not the value it has when this line is run.
	CommandLine.Usage = commandLineUsage
}

func commandLineUsage() {
	Usage()
}

// NewFlagSet returns a new, empty flag set with the specified name and
// error handling property.
func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	f := &FlagSet{
		name:          name,
		errorHandling: errorHandling,
	}
	f.Usage = f.defaultUsage
	return f
}

// Init sets the name and error handling property for a flag set.
// By default, the zero FlagSet uses an empty name and the
// ContinueOnError error handling policy.
func (f *FlagSet) Init(name string, errorHandling ErrorHandling) {
	f.name = name
	f.errorHandling = errorHandling
}
