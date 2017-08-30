// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-26 09:35:33 本包只看doc.go和导出函数,细节太繁琐了

/*
	Package fmt implements formatted I/O with functions analogous
	to C's printf and scanf.  The format 'verbs' are derived from C's but
	are simpler.
	
	analogous(类似的，相似的；可比拟的)
	verbs(动词)
	


	Printing

	The verbs:

	General:
		%v	the value in a default format
			when printing structs, the plus flag (%+v) adds field names
		%#v	a Go-syntax representation of the value
		%T	a Go-syntax representation of the type of the value
		%%	a literal percent sign; consumes no value

	Boolean:
		%t	the word true or false
	Integer:
		%b	base 2
		将整数作为2进制输出
		%c	the character represented by the corresponding Unicode code point
		将整数作为Unicode code point,输出此Unicode code point的对应字符
		%d	base 10
		将整数作为10进制输出
		%o	base 8
		将整数作为8进制输出
		%q	a single-quoted character literal safely escaped with Go syntax.
		%x	base 16, with lower-case letters for a-f
		%X	base 16, with upper-case letters for A-F
		%U	Unicode format: U+1234; same as "U+%04X"
		%U 相当于 U+%04X
	Floating-point and complex constituents:
		constituents(成分)
		exponent(指数)
		%b	decimalless scientific notation with exponent a power of two,
			in the manner of strconv.FormatFloat with the 'b' format,
			e.g. -123456p-78
		%e	scientific notation, e.g. -1.234456e+78
		%E	scientific notation, e.g. -1.234456E+78
		%f	decimal point but no exponent, e.g. 123.456
		非指数的小数点格式.
		%F	synonym for %f
		%g	%e for large exponents, %f otherwise. Precision is discussed below.
		%G	%E for large exponents, %F otherwise
	String and slice of bytes (treated equivalently with these verbs):
		%s	the uninterpreted bytes of the string or slice
		%q	a double-quoted string safely escaped with Go syntax
		%x	base 16, lower-case, two characters per byte
		(two characters:指输出中的两个字符是一个字节)
		%X	base 16, upper-case, two characters per byte
	Pointer:
		%p	base 16 notation, with leading 0x
		The %b, %d, %o, %x and %X verbs also work with pointers,
		formatting the value exactly as if it were an integer.

	The default format for %v is:
		bool:                    %t
		int, int8 etc.:          %d
		uint, uint8 etc.:        %d, %#x if printed with %#v
		float32, complex64, etc: %g
		string:                  %s
		chan:                    %p
		pointer:                 %p
	For compound objects, the elements are printed using these rules, recursively,
	laid out like this:
		struct:             {field0 field1 ...}
		array, slice:       [elem0 elem1 ...]
		maps:               map[key1:value1 key2:value2]
		pointer to above:   &{}, &[], &map[]

	Width is specified by an optional decimal number immediately preceding the verb.
	If absent, the width is whatever is necessary to represent the value.
	Precision is specified after the (optional) width by a period followed by a
	decimal number. If no period is present, a default precision is used.
	A period with no following number specifies a precision of zero.
	
	上文: optional decimal number (可选的十进制数)
	
	Examples:
		%f     default width, default precision
		%9f    width 9, default precision
		%.2f   default width, precision 2
		%9.2f  width 9, precision 2
		%9.f   width 9, precision 0
		根据文档: (A period with no following number specifies a precision of zero.)

	Width and precision are measured in units of Unicode code points,
	that is, runes. (This differs from C's printf where the
	units are always measured in bytes.) Either or both of the flags
	may be replaced with the character '*', causing their values to be
	obtained from the next operand, which must be of type int.
	
	上文中: Either or both of the flags 指宽度和精度这两个flag
	宽度和精度都能使用'*'代替,此时值是从下一个operand获取.

	For most values, width is the minimum number of runes to output,
	padding the formatted form with spaces if necessary.

	For strings, byte slices and byte arrays, however, precision
	limits the length of the input to be formatted (not the size of
	the output), truncating if necessary. Normally it is measured in
	runes, but for these types when formatted with the %x or %X format
	it is measured in bytes.
	
	然而, 对于 strings, byte slices and byte arrays,precision限制了input中将要被格式化
	的长度(不是output的size),必要时会进行截断.通常以rune为单位,但对于使用 x% 或 X% 的时候是以字节为单位.
	
	significant digit: 有效数字，有效位.
	number of decimal places: 小数位数

	For floating-point values, width sets the minimum width of the field and
	precision sets the number of places after the decimal, if appropriate,
	except that for %g/%G precision sets the total number of significant
	digits. For example, given 12.345 the format %6.3f prints 12.345 while
	%.3g prints 12.3. The default precision for %e, %f and %#g is 6; for %g it
	is the smallest number of digits necessary to identify the value uniquely.
	
	上文中: precision sets the number of places after the decimal(小数点后的位数)
	given 12.345 the format %6.3f(小数点后保留三位)
	

	For complex numbers, the width and precision apply to the two
	components independently and the result is parenthesized, so %f applied
	to 1.2+3.4i produces (1.200000+3.400000i).
	
	如之前提到的 : 对于浮点数,The default precision for %e and %f is 6, 因此这里会保留小数点后六位

	Other flags:
		+	always print a sign for numeric values;
			guarantee ASCII-only output for %q (%+q)
			对于数字总是会打印该数字的符号;或者,保证使用q+%的时候仅仅输出ASCII
		-	pad with spaces on the right rather than the left (left-justify the field)
			右边pad空格;而在默认情况下,是左边pad空格
		#	alternate format: add leading 0 for octal (%#o), 0x for hex (%#x);
			0X for hex (%#X); suppress 0x for %p (%#p);
			for %q, print a raw (backquoted) string if strconv.CanBackquote
			returns true;
			
			always print a decimal point for %e, %E, %f, %F, %g and %G;
			do not remove trailing zeros for %g and %G;
			write e.g. U+0078 'x' if the character is printable for %U (%#U).
			
			也就是当使用 %#q 的时候,如果 strconv.CanBackquote 为 true, 会输出 raw (backquoted) string
			测试如下:
			fmt.Printf("%#U", 0x0078) // U+0078 'x'
			fmt.Println()
			fmt.Printf("%#U", rune(0x0078)) // U+0078 'x'
		' '	(space) leave a space for elided sign in numbers (% d);
			put spaces between bytes printing strings or slices in hex (% x, % X)
			
			测试如下:
			fmt.Printf("%d,%d,%d,%d", -11, -12, -13, -14)
			fmt.Println()
			fmt.Printf("% d,% d,% d,% d", -11, -12, -13, -14)
			fmt.Println()
			fmt.Printf("%d,%d,%d,%d", 11, 12, 13, 14)
			fmt.Println()
			fmt.Printf("% d,% d,% d,% d", 11, 12, 13, 14)
			输出:
			-11,-12,-13,-14
			-11,-12,-13,-14
			11,12,13,14
			 11, 12, 13, 14(看出逗号之间有空格)
			 
			比如:
			fmt.Println()
			fmt.Printf("% x,% x,% x", "aBC","aBC","aBC")
			fmt.Println()
			fmt.Printf("% X,% X,% X", "aBC","aBC","aBC")
			61 42 43,61 42 43,61 42 43
			61 42 43,61 42 43,61 42 43
			
		0	pad with leading zeros rather than spaces;
			for numbers, this moves the padding after the sign
			
			0这个flag: 使用0而不是空格进行pad;对于数字,会将padding移动到符号(正负号)之后

	Flags are ignored by verbs that do not expect them.
	For example there is no alternate decimal format, so %#d and %d
	behave identically.

	For each Printf-like function, there is also a Print function
	that takes no format and is equivalent to saying %v for every
	operand.  Another variant Println inserts blanks between
	operands and appends a newline.

	Regardless of the verb, if an operand is an interface value,
	the internal concrete value is used, not the interface itself.
	Thus:
		var i interface{} = 23
		fmt.Printf("%v\n", i)
	will print 23.

	Except when printed using the verbs %T and %p, special
	formatting considerations apply for operands that implement
	certain interfaces. In order of application:

	1. If the operand is a reflect.Value, the operand is replaced by the
	concrete value that it holds, and printing continues with the next rule.

	2. If an operand implements the Formatter interface, it will
	be invoked. Formatter provides fine control of formatting.

	3. If the %v verb is used with the # flag (%#v) and the operand
	implements the GoStringer interface, that will be invoked.

	If the format (which is implicitly %v for Println etc.) is valid
	for a string (%s %q %v %x %X), the following two rules apply:

	4. If an operand implements the error interface, the Error method
	will be invoked to convert the object to a string, which will then
	be formatted as required by the verb (if any).

	5. If an operand implements method String() string, that method
	will be invoked to convert the object to a string, which will then
	be formatted as required by the verb (if any).

	For compound operands such as slices and structs, the format
	applies to the elements of each operand, recursively, not to the
	operand as a whole. Thus %q will quote each element of a slice
	of strings, and %6.2f will control formatting for each element
	of a floating-point array.

	However, when printing a byte slice with a string-like verb
	(%s %q %x %X), it is treated identically to a string, as a single item.

	To avoid recursion in cases such as
		type X string
		func (x X) String() string { return Sprintf("<%s>", x) }
	convert the value before recurring:
		func (x X) String() string { return Sprintf("<%s>", string(x)) }
		// 直接使用string(x)进行手动转换,而不是依赖Sprintf自动调用X.String方法
	Infinite recursion can also be triggered by self-referential data
	structures, such as a slice that contains itself as an element, if
	that type has a String method. Such pathologies are rare, however,
	and the package does not protect against them.
	
	pathologies(病理；病状): 意思是self-referential data structures本身是不合理的设计.

	When printing a struct, fmt cannot and therefore does not invoke
	formatting methods such as Error or String on unexported fields.

	Explicit argument indexes:

	In Printf, Sprintf, and Fprintf, the default behavior is for each
	formatting verb to format successive arguments passed in the call.
	However, the notation [n] immediately before the verb indicates that the
	nth one-indexed argument is to be formatted instead. The same notation
	before a '*' for a width or precision selects the argument index holding
	the value. After processing a bracketed expression [n], subsequent verbs
	will use arguments n+1, n+2, etc. unless otherwise directed.

	For example,
		fmt.Sprintf("%[2]d %[1]d\n", 11, 22)
	will yield "22 11", while
		fmt.Sprintf("%[3]*.[2]*[1]f", 12.0, 2, 6)
	equivalent to
		fmt.Sprintf("%6.2f", 12.0)
	will yield " 12.00". Because an explicit index affects subsequent verbs,
	this notation can be used to print the same values multiple times
	by resetting the index for the first argument to be repeated:
		fmt.Sprintf("%d %d %#[1]x %#x", 16, 17)
	will yield "16 17 0x10 0x11".
	// 0x10 是 10进制的16, 0x11 是10进制的17

	Format errors:

	If an invalid argument is given for a verb, such as providing
	a string to %d, the generated string will contain a
	description of the problem, as in these examples:

		Wrong type or unknown verb: %!verb(type=value)
			Printf("%d", hi):          %!d(string=hi)
		Too many arguments: %!(EXTRA type=value)
			Printf("hi", "guys"):      hi%!(EXTRA string=guys)
		Too few arguments: %!verb(MISSING)
			Printf("hi%d"):            hi%!d(MISSING)
		Non-int for width or precision: %!(BADWIDTH) or %!(BADPREC)
			Printf("%*s", 4.5, "hi"):  %!(BADWIDTH)hi
			上面的例子, 宽度只能为整数
			Printf("%.*s", 4.5, "hi"): %!(BADPREC)hi
			上面的例子, 精度只能为整数
		Invalid or invalid use of argument index: %!(BADINDEX)
			Printf("%*[2]d", 7):       %!d(BADINDEX)
			指定第二个参数,但是不存在
			Printf("%.[2]d", 7):       %!d(BADINDEX)
			指定第二个参数,但是不存在

	All errors begin with the string "%!" followed sometimes
	by a single character (the verb) and end with a parenthesized
	description.

	If an Error or String method triggers a panic when called by a
	print routine, the fmt package reformats the error message
	from the panic, decorating it with an indication that it came
	through the fmt package.  For example, if a String method
	calls panic("bad"), the resulting formatted message will look
	like
		%!s(PANIC=bad)

	The %!s just shows the print verb in use when the failure
	occurred. If the panic is caused by a nil receiver to an Error
	or String method, however, the output is the undecorated
	string, "<nil>".

	Scanning

	An analogous set of functions scans formatted text to yield
	values.  Scan, Scanf and Scanln read from os.Stdin; Fscan,
	Fscanf and Fscanln read from a specified io.Reader; Sscan,
	Sscanf and Sscanln read from an argument string.

	Scan, Fscan, Sscan treat newlines in the input as spaces.

	Scanln, Fscanln and Sscanln stop scanning at a newline and
	require that the items be followed by a newline or EOF.
	
	以下摘自文档
	$ go doc fmt.Scanln
func Scanln(a ...interface{}) (n int, err error)
    Scanln is similar to Scan, but stops scanning at a newline(指输入) and after the
    final item(指函数参数) there must be a newline or EOF.
	$ go doc fmt.Fscanln
func Fscanln(r io.Reader, a ...interface{}) (n int, err error)
    Fscanln is similar to Fscan, but stops scanning at a newline(指输入) and after the
    final item(指函数参数) there must be a newline or EOF.
	$ go doc fmt.Sscanln
func Sscanln(str string, a ...interface{}) (n int, err error)
    Sscanln is similar to Sscan, but stops scanning at a newline(指输入) and after the
    final item(指函数参数) there must be a newline or EOF.
    

	Scanf, Fscanf, and Sscanf parse the arguments according to a
	format string, analogous to that of Printf. In the text that
	follows, 'space' means any Unicode whitespace character
	except newline.
	
	在下方描述format的内容中:'space' means any Unicode whitespace character except newline.

	In the format string, a verb introduced by the % character
	consumes and parses input; these verbs are described in more
	detail below. A character other than %, space, or newline in
	the format consumes exactly that input character, which must
	be present. A newline with zero or more spaces before it in
	the format string consumes zero or more spaces in the input
	followed by a single newline or the end of the input. A space
	following a newline in the format string consumes zero or more
	spaces in the input. Otherwise, any run of one or more spaces
	in the format string consumes as many spaces as possible in
	the input. Unless the run of spaces in the format string
	appears adjacent to a newline, the run must consume at least
	one space from the input or find the end of the input.

	The handling of spaces and newlines differs from that of C's
	scanf family: in C, newlines are treated as any other space,
	and it is never an error when a run of spaces in the format
	string finds no spaces to consume in the input.
	
	golang对于spaces and newlines的处理和C语言不同:
	在C中,newlines跟其他space作为同样的东西对待;
	并且在C中,如果format中的一串spaces没有在input中找到对应的spaces,不算是个错误.

	The verbs behave analogously to those of Printf.
	For example, %x will scan an integer as a hexadecimal number,
	and %v will scan the default representation format for the value.
	The Printf verbs %p and %T and the flags # and + are not implemented,
	and the verbs %e %E %f %F %g and %G are all equivalent and scan any
	floating-point or complex value.

	Input processed by verbs is implicitly space-delimited: the
	implementation of every verb except %c starts by discarding
	leading spaces from the remaining input, and the %s verb
	(and %v reading into a string) stops consuming input at the first
	space or newline character.
	
	由verb处理的input默认是按照space-delimited:所有的verb(除了%c),
	最开始时会将input(当前剩余的input)中的leading spaces进行丢弃.
	而且%s和(%v读入一个字符串)会在消费到首个space或newline时停止.

	The familiar base-setting prefixes 0 (octal) and 0x
	(hexadecimal) are accepted when scanning integers without
	a format or with the %v verb.

	Width is interpreted in the input text but there is no
	syntax for scanning with a precision (no %5.2f, just %5f).
	If width is provided, it applies after leading spaces are
	trimmed and specifies the maximum number of runes to read
	to satisfy the verb. For example,
	   Sscanf(" 1234567 ", "%5s%d", &s, &i)
	   
	   1.trim掉开头的空格.
	   2.根据%5s决定使用s读取input中的5个rune.
	   
	will set s to "12345" and i to 67 while
	   Sscanf(" 12 34 567 ", "%5s%d", &s, &i)
	   
	   1.trim掉开头的空格
	   2.根据%5s决定要使用s读取input中的5个rune,但由于遇到space,s只能读取到12.
	   3.继续trim掉剩余input开头的空格.
	   4.根据%d决定要读入之后的整数.
	   
	   
	will set s to "12" and i to 34.

	In all the scanning functions, a carriage return followed
	immediately by a newline is treated as a plain newline
	(\r\n means the same as \n).
	
	上文中\r\n是指在input还是format中??

	In all the scanning functions, if an operand implements method
	Scan (that is, it implements the Scanner interface) that
	method will be used to scan the text for that operand.  Also,
	if the number of arguments scanned is less than the number of
	arguments provided, an error is returned.

	All arguments to be scanned must be either pointers to basic
	types or implementations of the Scanner interface.

	Like Scanf and Fscanf, Sscanf need not consume its entire input.
	There is no way to recover how much of the input string Sscanf used.

	Note: Fscan etc. can read one character (rune) past the input
	they return, which means that a loop calling a scan routine
	may skip some of the input.  This is usually a problem only
	when there is no space between input values.  If the reader
	provided to Fscan implements ReadRune, that method will be used
	to read characters.  If the reader also implements UnreadRune,
	that method will be used to save the character and successive
	calls will not lose data.  To attach ReadRune and UnreadRune
	methods to a reader without that capability, use
	bufio.NewReader.
*/
package fmt
