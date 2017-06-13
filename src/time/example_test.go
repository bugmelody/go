// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time_test

import (
	"fmt"
	"time"
)

// 假设 expensiveCall 代表一个非常昂贵的操作
func expensiveCall() {}

func ExampleDuration() {
	t0 := time.Now()
	expensiveCall()
	t1 := time.Now()
	fmt.Printf("The call took %v to run.\n", t1.Sub(t0))
}

var c chan int

func handle(int) {}

func ExampleAfter() {
	// 要么收到c的通知调用handle,要么5分钟超时后输出timed out;也就是说取决于哪个channel的信号先到
	select {
	case m := <-c:
		handle(m)
	case <-time.After(5 * time.Minute):
		fmt.Println("timed out")
	}
}

func ExampleSleep() {
	time.Sleep(100 * time.Millisecond)
}

// 输出当前的status
func statusUpdate() string { return "" }

func ExampleTick() {
	c := time.Tick(1 * time.Minute)
	// 通过 range 从 channel 中读取
	for now := range c {
		// 每隔1分钟输出status的变化
		fmt.Printf("%v %s\n", now, statusUpdate())
	}
}

func ExampleMonth() {
	_, month, day := time.Now().Date()
	if month == time.November && day == 10 {
		// 如果当前是 11月10日
		fmt.Println("Happy Go day!")
	}
}

func ExampleDate() {
	// 使用time.Date构造一个time.Time
	t := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	// 输出本地时间: 23 - 8 = 15
	fmt.Printf("Go launched at %s\n", t.Local())
	// Output: Go launched at 2009-11-10 15:00:00 -0800 PST
}

func ExampleTime_Format() {
	// Parse a time value from a string in the standard Unix format.
	// 注意:下面 7 之前是两个空格, UnixDate 定义也反应了同样的格式
	// time.UnixDate = "Mon Jan _2 15:04:05 MST 2006" // Mon 表示周一, _2 表示使用空格将 day 对齐
	t, err := time.Parse(time.UnixDate, "Sat Mar  7 11:06:39 PST 2015")
	if err != nil { // Always check errors even if they should not happen.
		panic(err)
	}

	// time.Time's Stringer method is useful without any format.
	// 输出: default format: 2015-03-07 11:06:39 -0800 PST
	// 参考: $ go doc time.Time.String
	fmt.Println("default format:", t)

	// Predefined constants in the package implement common layouts.
	// 输出: Unix format: Sat Mar  7 11:06:39 PST 2015
	// 注意两个空格是 time.UnixDate 中定义的对齐效果
	// time.UnixDate = "Mon Jan _2 15:04:05 MST 2006" // Mon 表示周一, _2 表示使用空格将 day 对齐
	fmt.Println("Unix format:", t.Format(time.UnixDate))

	// The time zone attached to the time value affects its output.
	// 输出: Same, in UTC: Sat Mar  7 19:06:39 UTC 2015
	// 将t切换到utc之后,输出的时间多了8小时
	fmt.Println("Same, in UTC:", t.UTC().Format(time.UnixDate))

	// The rest of this function demonstrates the properties of the
	// layout string used in the format.

	// The layout string used by the Parse function and Format method
	// shows by example how the reference time should be represented.
	// We stress that one must show how the reference time is formatted,
	// not a time of the user's choosing. Thus each layout string is a
	// representation of the time stamp,
	//	Jan 2 15:04:05 2006 MST
	// An easy way to remember this value is that it holds, when presented
	// in this order, the values (lined up with the elements above):
	//	  1 2  3  4  5    6  -7
	// There are some wrinkles illustrated below.
	
	// wrinkles(皱纹,妙计,好主意,巧技)

	// Most uses of Format and Parse use constant layout strings such as
	// the ones defined in this package, but the interface is flexible,
	// as these examples show.

	// Define a helper function to make the examples' output look nice.
	do := func(name, layout, want string) {
		// t是闭包引用的外层变量
		got := t.Format(layout)
		if want != got {
			fmt.Printf("error: for %q got %q; expected %q\n", layout, got, want)
			return
		}
		// 现在,want==got
		// %-15s 的解释:
		// -: pad with spaces on the right rather than the left (left-justify the field); 左对齐(也就是右补空格)
		// 15是Width
		// Width is specified by an optional decimal number immediately preceding the verb.
		// If absent, the width is whatever is necessary to represent the value.
		fmt.Printf("%-15s %q gives %q\n", name, layout, got)
	}

	// Print a header in our output.
	fmt.Printf("\nFormats:\n\n")

	// A simple starter example.
	do("Basic", "Mon Jan 2 15:04:05 MST 2006", "Sat Mar 7 11:06:39 PST 2015")

	// For fixed-width printing of values, such as the date, that may be one or
	// two characters (7 vs. 07), use an _ instead of a space in the layout string.
	// Here we print just the day, which is 2 in our layout string and 7 in our
	// value.
	// 不对齐
	do("No pad", "<2>", "<7>")

	// An underscore represents a zero pad, if required.
	// 下划线代表使用空格进行对齐
	do("Spaces", "<_2>", "< 7>")

	// Similarly, a 0 indicates zero padding.
	// 0代表使用0进行对齐
	do("Zeros", "<02>", "<07>")

	// If the value is already the right width, padding is not used.
	// For instance, the second (05 in the reference time) in our value is 39,
	// so it doesn't need padding, but the minutes (04, 06) does.
	// 如果value已经是正确的宽度,不会进行padding.
	// 比如,秒数(reference time中的05)value是39,
	// 因此不需要padding.
	// 但分钟(reference time中的04)value是06,因此会使用0进行padding.
	do("Suppressed pad", "04:05", "06:39")

	// The predefined constant Unix uses an underscore to pad the day.
	// Compare with our simple starter example.
	// 包常量: UnixDate = "Mon Jan _2 15:04:05 MST 2006"
	// 注意: 日期是 _2 , 因此实际生成的日期如果是1位,为用空格在左边进行padding
	// 实际效果是在 Mar  7 之间出现两个空格
	do("Unix", time.UnixDate, "Sat Mar  7 11:06:39 PST 2015")

	// The hour of the reference time is 15, or 3PM. The layout can express
	// it either way, and since our value is the morning we should see it as
	// an AM time. We show both in one format string. Lower case too.
	do("AM/PM", "3PM==3pm==15h", "11AM==11am==11h")

	// When parsing, if the seconds value is followed by a decimal point
	// and some digits, that is taken as a fraction of a second even if
	// the layout string does not represent the fractional second.
	// Here we add a fractional second to our time value used above.
	//
	// time.UnixDate = "Mon Jan _2 15:04:05 MST 2006" , 即使 time.UnixDate 这个 layout string 没有 fractional second,
	// time.Parse 仍然会将 11:06:39.1234 中的 1234 作为秒的小数部分解析.
	// 重新设置t,增加秒的小数部分
	t, err = time.Parse(time.UnixDate, "Sat Mar  7 11:06:39.1234 PST 2015")
	if err != nil {
		panic(err)
	}
	// It does not appear in the output if the layout string does not contain
	// a representation of the fractional second.
	// 如果 layout 中不包含 fractional second, t.Format 的输出也不会包含秒的小数部分.
	do("No fraction", time.UnixDate, "Sat Mar  7 11:06:39 PST 2015")

	/**
	He has had a long run of power 掌权已久
	A Run Of Luck 有福同享
	a run of salmons 一群游动的鲑鱼
	after a run of 一连串的
	a run of bad luck 一连串恶运
	A Run Of Wet Weather 阴雨连绵
	having a run of luck 红火
	What a run of luck! 好运连绵
	 */

	// Fractional seconds can be printed by adding a run of 0s or 9s after
	// a decimal point in the seconds value in the layout string.
	// If the layout digits are 0s, the fractional second is of the specified
	// width. Note that the output has a trailing zero.
	//
	// a run of:一连串的
	do("0s for fraction", "15:04:05.00000", "11:06:39.12340")

	// If the fraction in the layout is 9s, trailing zeros are dropped.
	do("9s for fraction", "15:04:05.99999999", "11:06:39.1234")

	// Output:
	// default format: 2015-03-07 11:06:39 -0800 PST
	// Unix format: Sat Mar  7 11:06:39 PST 2015
	// Same, in UTC: Sat Mar  7 19:06:39 UTC 2015
	//
	// Formats:
	//
	// Basic           "Mon Jan 2 15:04:05 MST 2006" gives "Sat Mar 7 11:06:39 PST 2015"
	// No pad          "<2>" gives "<7>"
	// Spaces          "<_2>" gives "< 7>"
	// Zeros           "<02>" gives "<07>"
	// Suppressed pad  "04:05" gives "06:39"
	// Unix            "Mon Jan _2 15:04:05 MST 2006" gives "Sat Mar  7 11:06:39 PST 2015"
	// AM/PM           "3PM==3pm==15h" gives "11AM==11am==11h"
	// No fraction     "Mon Jan _2 15:04:05 MST 2006" gives "Sat Mar  7 11:06:39 PST 2015"
	// 0s for fraction "15:04:05.00000" gives "11:06:39.12340"
	// 9s for fraction "15:04:05.99999999" gives "11:06:39.1234"

}

func ExampleParse() {
	// See the example for time.Format for a thorough description of how
	// to define the layout string to parse a time.Time value; Parse and
	// Format use the same model to describe their input and output.

	// longForm shows by example how the reference time would be represented in
	// the desired layout.
	const longForm = "Jan 2, 2006 at 3:04pm (MST)"
	t, _ := time.Parse(longForm, "Feb 3, 2013 at 7:54pm (PST)")
	// Parse后使用Time.String输出
	fmt.Println(t)

	// shortForm is another way the reference time would be represented
	// in the desired layout; it has no time zone present.
	// Note: without explicit zone, returns time in UTC.
	// 注意: 当被 parse 的时间字符串中不存在时区信息的时候,返回的时间是 UTC 作为时区
	const shortForm = "2006-Jan-02"
	t, _ = time.Parse(shortForm, "2013-Feb-03")
	// Parse后使用Time.String输出
	fmt.Println(t)

	// 注意,在time.Parse的文档中提到
	// Elements omitted from the value are assumed to be zero or, when zero is impossible, one

	// Output:
	// 2013-02-03 19:54:00 -0800 PST
	// 2013-02-03 00:00:00 +0000 UTC
}

func ExampleParseInLocation() {
	/**
	北京时间(或者重庆)是GMT/UTC +8 
	而CEST是GMT/UTC +2 
	*/

	// 见: http://www.timeanddate.com/time/zone/germany/berlin
	// "Europe/Berlin" 其实就是 CEST
	// 那么,这两种表述方法有什么区别呢???
	loc, _ := time.LoadLocation("Europe/Berlin")

	const longForm = "Jan 2, 2006 at 3:04pm (MST)"
	t, _ := time.ParseInLocation(longForm, "Jul 9, 2012 at 5:02am (CEST)", loc)
	fmt.Println(t)

	// Note: without explicit zone, returns time in given location.
	const shortForm = "2006-Jan-02"
	t, _ = time.ParseInLocation(shortForm, "2012-Jul-09", loc)
	fmt.Println(t)

	// Output:
	// 2012-07-09 05:02:00 +0200 CEST
	// 2012-07-09 00:00:00 +0200 CEST
}

func ExampleTime_Round() {
	t := time.Date(0, 0, 0, 12, 15, 30, 918273645, time.UTC)
	round := []time.Duration{
		time.Nanosecond,
		time.Microsecond,
		time.Millisecond,
		time.Second,
		2 * time.Second,
		time.Minute,
		10 * time.Minute,
		time.Hour,
	}

	for _, d := range round {
		fmt.Printf("t.Round(%6s) = %s\n", d, t.Round(d).Format("15:04:05.999999999"))
	}
	// Output:
	// t.Round(   1ns) = 12:15:30.918273645
	// t.Round(   1µs) = 12:15:30.918274
	// t.Round(   1ms) = 12:15:30.918
	// t.Round(    1s) = 12:15:31
	// t.Round(    2s) = 12:15:30
	// t.Round(  1m0s) = 12:16:00
	// t.Round( 10m0s) = 12:20:00
	// t.Round(1h0m0s) = 12:00:00
}

func ExampleTime_Truncate() {
	t, _ := time.Parse("2006 Jan 02 15:04:05", "2012 Dec 07 12:15:30.918273645")
	trunc := []time.Duration{
		time.Nanosecond,
		time.Microsecond,
		time.Millisecond,
		time.Second,
		2 * time.Second,
		time.Minute,
		10 * time.Minute,
	}

	for _, d := range trunc {
		fmt.Printf("t.Truncate(%5s) = %s\n", d, t.Truncate(d).Format("15:04:05.999999999"))
	}

	// Output:
	// t.Truncate(  1ns) = 12:15:30.918273645
	// t.Truncate(  1µs) = 12:15:30.918273
	// t.Truncate(  1ms) = 12:15:30.918
	// t.Truncate(   1s) = 12:15:30
	// t.Truncate(   2s) = 12:15:30
	// t.Truncate( 1m0s) = 12:15:00
	// t.Truncate(10m0s) = 12:10:00
}
