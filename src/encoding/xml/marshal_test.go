// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type DriveType int

const (
	HyperDrive DriveType = iota
	ImprobabilityDrive
)

type Passenger struct {
	Name   []string `xml:"name"`
	Weight float32  `xml:"weight"`
}

type Ship struct {
	XMLName struct{} `xml:"spaceship"`

	Name      string       `xml:"name,attr"`
	// pilot ['pailət] n. 1.舵手，领航员；领港员；引水者 2.飞行员；飞机驾驶员；正驾驶；机长
	Pilot     string       `xml:"pilot,attr"`
	Drive     DriveType    `xml:"drive"`
	Age       uint         `xml:"age"`
	// Passenger n. 旅客；乘客；过路人
	Passenger []*Passenger `xml:"passenger"`
	secret    string
}

type NamedType string

type Port struct {
	XMLName struct{} `xml:"port"`
	Type    string   `xml:"type,attr,omitempty"`
	Comment string   `xml:",comment"`
	Number  string   `xml:",chardata"`
}

type Domain struct {
	XMLName struct{} `xml:"domain"`
	Country string   `xml:",attr,omitempty"`
	Name    []byte   `xml:",chardata"`
	Comment []byte   `xml:",comment"`
}

type Book struct {
	XMLName struct{} `xml:"book"`
	Title   string   `xml:",chardata"`
}

type Event struct {
	XMLName struct{} `xml:"event"`
	Year    int      `xml:",chardata"`
}

type Movie struct {
	XMLName struct{} `xml:"movie"`
	Length  uint     `xml:",chardata"`
}

type Pi struct {
	XMLName       struct{} `xml:"pi"`
	Approximation float32  `xml:",chardata"`
}

type Universe struct {
	XMLName struct{} `xml:"universe"`
	Visible float64  `xml:",chardata"`
}

type Particle struct {
	XMLName struct{} `xml:"particle"`
	HasMass bool     `xml:",chardata"`
}

// Departure: n. 离开；出发；违背
type Departure struct {
	XMLName struct{}  `xml:"departure"`
	When    time.Time `xml:",chardata"`
}

// 注: SecretAgent: 间谍；特务
type SecretAgent struct {
	XMLName   struct{} `xml:"agent"`
	Handle    string   `xml:"handle,attr"`
	Identity  string
	Obfuscate string `xml:",innerxml"`
}

type NestedItems struct {
	XMLName struct{} `xml:"result"`
	// 注: Items字段对应xml中的 result>Items>item
	// ">item" 这种写法是"Items>item"的简略写法
	Items   []string `xml:">item"`
	Item1   []string `xml:"Items>item1"`
}

type NestedOrder struct {
	XMLName struct{} `xml:"result"`
	Field1  string   `xml:"parent>c"`
	Field2  string   `xml:"parent>b"`
	Field3  string   `xml:"parent>a"`
}

/**
只有C,D会共享同一个parent1
A因为没有和C,D挨着,因此不会和C,D共享同一个parent1
 */
type MixedNested struct {
	XMLName struct{} `xml:"result"`
	A       string   `xml:"parent1>a"`
	B       string   `xml:"b"`
	C       string   `xml:"parent1>parent2>c"`
	D       string   `xml:"parent1>d"`
}

type NilTest struct {
	A interface{} `xml:"parent1>parent2>a"`
	B interface{} `xml:"parent1>b"`
	C interface{} `xml:"parent1>parent2>c"`
}

/**
根据结构体可以看出,xml结构如下
<service>
	<host>
		<domain></domain>
		<port></port>
	</host>
	<Extra1>
	</Extra1>
	<host>
		<extra2></extra2>
	</host>
</service>
 */
type Service struct {
	XMLName struct{} `xml:"service"`
	Domain  *Domain  `xml:"host>domain"`
	Port    *Port    `xml:"host>port"`
	Extra1  interface{}
	Extra2  interface{} `xml:"host>extra2"`
}

var nilStruct *Ship

type EmbedA struct {
	EmbedC
	EmbedB EmbedB
	FieldA string
	embedD
}

type EmbedB struct {
	FieldB string
	*EmbedC
}

type EmbedC struct {
	FieldA1 string `xml:"FieldA>A1"`
	FieldA2 string `xml:"FieldA>A2"`
	FieldB  string
	FieldC  string
}

type embedD struct {
	fieldD string
	FieldE string // Promoted and visible when embedD is embedded.
}

type NameCasing struct {
	XMLName struct{} `xml:"casing"`
	Xy      string
	XY      string
	XyA     string `xml:"Xy,attr"`
	XYA     string `xml:"XY,attr"`
}

type NamePrecedence struct {
	XMLName     Name              `xml:"Parent"`
	FromTag     XMLNameWithoutTag `xml:"InTag"`
	FromNameVal XMLNameWithoutTag
	FromNameTag XMLNameWithTag
	InFieldName string
}

type XMLNameWithTag struct {
	XMLName Name   `xml:"InXMLNameTag"`
	Value   string `xml:",chardata"`
}

type XMLNameWithoutTag struct {
	XMLName Name
	Value   string `xml:",chardata"`
}

type NameInField struct {
	Foo Name `xml:"ns foo"`
}

type AttrTest struct {
	Int   int     `xml:",attr"`
	Named int     `xml:"int,attr"`
	Float float64 `xml:",attr"`
	Uint8 uint8   `xml:",attr"`
	Bool  bool    `xml:",attr"`
	Str   string  `xml:",attr"`
	Bytes []byte  `xml:",attr"`
}

type AttrsTest struct {
	// []Attr中,Attr是指xml.Attr
	Attrs []Attr  `xml:",any,attr"`
	Int   int     `xml:",attr"`
	Named int     `xml:"int,attr"`
	Float float64 `xml:",attr"`
	Uint8 uint8   `xml:",attr"`
	Bool  bool    `xml:",attr"`
	Str   string  `xml:",attr"`
	Bytes []byte  `xml:",attr"`
}

type OmitAttrTest struct {
	Int   int     `xml:",attr,omitempty"`
	Named int     `xml:"int,attr,omitempty"`
	Float float64 `xml:",attr,omitempty"`
	Uint8 uint8   `xml:",attr,omitempty"`
	Bool  bool    `xml:",attr,omitempty"`
	Str   string  `xml:",attr,omitempty"`
	Bytes []byte  `xml:",attr,omitempty"`
	PStr  *string `xml:",attr,omitempty"`
}

type OmitFieldTest struct {
	Int   int           `xml:",omitempty"`
	Named int           `xml:"int,omitempty"`
	Float float64       `xml:",omitempty"`
	Uint8 uint8         `xml:",omitempty"`
	Bool  bool          `xml:",omitempty"`
	Str   string        `xml:",omitempty"`
	Bytes []byte        `xml:",omitempty"`
	PStr  *string       `xml:",omitempty"`
	Ptr   *PresenceTest `xml:",omitempty"`
}

type AnyTest struct {
	XMLName  struct{}  `xml:"a"`
	Nested   string    `xml:"nested>value"`
	AnyField AnyHolder `xml:",any"`
}

type AnyOmitTest struct {
	XMLName  struct{}   `xml:"a"`
	Nested   string     `xml:"nested>value"`
	AnyField *AnyHolder `xml:",any,omitempty"`
}

type AnySliceTest struct {
	XMLName  struct{}    `xml:"a"`
	Nested   string      `xml:"nested>value"`
	AnyField []AnyHolder `xml:",any"`
}

type AnyHolder struct {
	XMLName Name
	XML     string `xml:",innerxml"`
}

type RecurseA struct {
	A string
	B *RecurseB
}

type RecurseB struct {
	A *RecurseA
	B string
}

type PresenceTest struct {
	Exists *struct{}
}

type IgnoreTest struct {
	PublicSecret string `xml:"-"`
}

type MyBytes []byte

type Data struct {
	Bytes  []byte
	Attr   []byte `xml:",attr"`
	Custom MyBytes
}

type Plain struct {
	V interface{}
}

type MyInt int

type EmbedInt struct {
	MyInt
}

type Strings struct {
	X []string `xml:"A>B,omitempty"`
}

type PointerFieldsTest struct {
	XMLName  Name    `xml:"dummy"`
	Name     *string `xml:"name,attr"`
	Age      *uint   `xml:"age,attr"`
	Empty    *string `xml:"empty,attr"`
	Contents *string `xml:",chardata"`
}

type ChardataEmptyTest struct {
	XMLName  Name    `xml:"test"`
	Contents *string `xml:",chardata"`
}

type MyMarshalerTest struct {
}

// 注: 编译期验证 *MyMarshalerTest 实现了 Marshaler 接口.
var _ Marshaler = (*MyMarshalerTest)(nil)

// 注: 对 *MyMarshalerTest 实现 Marshaler 接口.
func (m *MyMarshalerTest) MarshalXML(e *Encoder, start StartElement) error {
	e.EncodeToken(start)
	e.EncodeToken(CharData([]byte("hello world")))
	e.EncodeToken(EndElement{start.Name})
	return nil
}

type MyMarshalerAttrTest struct {
}

var _ MarshalerAttr = (*MyMarshalerAttrTest)(nil)

func (m *MyMarshalerAttrTest) MarshalXMLAttr(name Name) (Attr, error) {
	return Attr{name, "hello world"}, nil
}

func (m *MyMarshalerAttrTest) UnmarshalXMLAttr(attr Attr) error {
	return nil
}

type MarshalerStruct struct {
	Foo MyMarshalerAttrTest `xml:",attr"`
}

type InnerStruct struct {
	XMLName Name `xml:"testns outer"`
}

type OuterStruct struct {
	InnerStruct
	IntAttr int `xml:"int,attr"`
}

type OuterNamedStruct struct {
	InnerStruct
	XMLName Name `xml:"outerns test"`
	IntAttr int  `xml:"int,attr"`
}

type OuterNamedOrderedStruct struct {
	XMLName Name `xml:"outerns test"`
	InnerStruct
	IntAttr int `xml:"int,attr"`
}

type OuterOuterStruct struct {
	OuterStruct
}

type NestedAndChardata struct {
	AB       []string `xml:"A>B"`
	Chardata string   `xml:",chardata"`
}

type NestedAndComment struct {
	AB      []string `xml:"A>B"`
	Comment string   `xml:",comment"`
}

type CDataTest struct {
	Chardata string `xml:",cdata"`
}

type NestedAndCData struct {
	AB    []string `xml:"A>B"`
	CDATA string   `xml:",cdata"`
}

func ifaceptr(x interface{}) interface{} {
	return &x
}

func stringptr(x string) *string {
	return &x
}

type T1 struct{}
type T2 struct{}
type T3 struct{}

type IndirComment struct {
	T1      T1
	Comment *string `xml:",comment"`
	T2      T2
}

type DirectComment struct {
	T1      T1
	Comment string `xml:",comment"`
	T2      T2
}

type IfaceComment struct {
	T1      T1
	Comment interface{} `xml:",comment"`
	T2      T2
}

type IndirChardata struct {
	T1       T1
	Chardata *string `xml:",chardata"`
	T2       T2
}

type DirectChardata struct {
	T1       T1
	Chardata string `xml:",chardata"`
	T2       T2
}

type IfaceChardata struct {
	T1       T1
	Chardata interface{} `xml:",chardata"`
	T2       T2
}

type IndirCDATA struct {
	T1    T1
	CDATA *string `xml:",cdata"`
	T2    T2
}

type DirectCDATA struct {
	T1    T1
	CDATA string `xml:",cdata"`
	T2    T2
}

type IfaceCDATA struct {
	T1    T1
	CDATA interface{} `xml:",cdata"`
	T2    T2
}

type IndirInnerXML struct {
	T1       T1
	InnerXML *string `xml:",innerxml"`
	T2       T2
}

type DirectInnerXML struct {
	T1       T1
	InnerXML string `xml:",innerxml"`
	T2       T2
}

type IfaceInnerXML struct {
	T1       T1
	InnerXML interface{} `xml:",innerxml"`
	T2       T2
}

type IndirElement struct {
	T1      T1
	Element *string
	T2      T2
}

type DirectElement struct {
	T1      T1
	Element string
	T2      T2
}

type IfaceElement struct {
	T1      T1
	Element interface{}
	T2      T2
}

type IndirOmitEmpty struct {
	T1        T1
	OmitEmpty *string `xml:",omitempty"`
	T2        T2
}

type DirectOmitEmpty struct {
	T1        T1
	OmitEmpty string `xml:",omitempty"`
	T2        T2
}

type IfaceOmitEmpty struct {
	T1        T1
	OmitEmpty interface{} `xml:",omitempty"`
	T2        T2
}

type IndirAny struct {
	T1  T1
	Any *string `xml:",any"`
	T2  T2
}

type DirectAny struct {
	T1  T1
	Any string `xml:",any"`
	T2  T2
}

type IfaceAny struct {
	T1  T1
	Any interface{} `xml:",any"`
	T2  T2
}

var (
	nameAttr     = "Sarah"
	ageAttr      = uint(12)
	contentsAttr = "lorem ipsum"
	empty        = ""
)

// Unless explicitly stated as such (or *Plain), all of the
// tests below are two-way tests. When introducing new tests,
// please try to make them two-way as well to ensure that
// marshaling and unmarshaling are as symmetrical as feasible.
//
// @see
var marshalTests = []struct {
	// Marshal或Unmarshal时的go变量
	Value          interface{}
	// Marshal或Unmarshal时的xml字符串
	ExpectXML      string
	// 只在TestMarshal测试中运行
	MarshalOnly    bool
	// Marshal时发生的错误
	MarshalError   string
	// 只在TestUnmarshal测试中运行
	UnmarshalOnly  bool
	// Unmarshal时发生的错误
	UnmarshalError string
}{
	// Test nil marshals to nothing
	{Value: nil, ExpectXML: ``, MarshalOnly: true},
	{Value: nilStruct, ExpectXML: ``, MarshalOnly: true},

	// 注: Value: &Plain 的不会进入 Unmarshal 测试
	// Test value types
	// 注: 下面这一大块只是做marshal测试
	{Value: &Plain{true}, ExpectXML: `<Plain><V>true</V></Plain>`},
	{Value: &Plain{false}, ExpectXML: `<Plain><V>false</V></Plain>`},
	{Value: &Plain{int(42)}, ExpectXML: `<Plain><V>42</V></Plain>`},
	{Value: &Plain{int8(42)}, ExpectXML: `<Plain><V>42</V></Plain>`},
	{Value: &Plain{int16(42)}, ExpectXML: `<Plain><V>42</V></Plain>`},
	{Value: &Plain{int32(42)}, ExpectXML: `<Plain><V>42</V></Plain>`},
	{Value: &Plain{uint(42)}, ExpectXML: `<Plain><V>42</V></Plain>`},
	{Value: &Plain{uint8(42)}, ExpectXML: `<Plain><V>42</V></Plain>`},
	{Value: &Plain{uint16(42)}, ExpectXML: `<Plain><V>42</V></Plain>`},
	{Value: &Plain{uint32(42)}, ExpectXML: `<Plain><V>42</V></Plain>`},
	{Value: &Plain{float32(1.25)}, ExpectXML: `<Plain><V>1.25</V></Plain>`},
	{Value: &Plain{float64(1.25)}, ExpectXML: `<Plain><V>1.25</V></Plain>`},
	{Value: &Plain{uintptr(0xFFDD)}, ExpectXML: `<Plain><V>65501</V></Plain>`},
	{Value: &Plain{"gopher"}, ExpectXML: `<Plain><V>gopher</V></Plain>`},
	{Value: &Plain{[]byte("gopher")}, ExpectXML: `<Plain><V>gopher</V></Plain>`},
	// 注意ExpectXML中进行了实体转换
	{Value: &Plain{"</>"}, ExpectXML: `<Plain><V>&lt;/&gt;</V></Plain>`},
	// 注意ExpectXML中进行了实体转换
	{Value: &Plain{[]byte("</>")}, ExpectXML: `<Plain><V>&lt;/&gt;</V></Plain>`},
	{Value: &Plain{[3]byte{'<', '/', '>'}}, ExpectXML: `<Plain><V>&lt;/&gt;</V></Plain>`},
	// 注意:使用了NamedType的底层string
	{Value: &Plain{NamedType("potato")}, ExpectXML: `<Plain><V>potato</V></Plain>`},
	{Value: &Plain{[]int{1, 2, 3}}, ExpectXML: `<Plain><V>1</V><V>2</V><V>3</V></Plain>`},
	{Value: &Plain{[3]int{1, 2, 3}}, ExpectXML: `<Plain><V>1</V><V>2</V><V>3</V></Plain>`},
	// 注意:Marshal时,标签名使用类型名,xml值是interface包含的值,
	{Value: ifaceptr(true), MarshalOnly: true, ExpectXML: `<bool>true</bool>`},

	// 注: Value: &Plain 的不会进行 Unmarshal 测试
	// Test time.
	{
		Value:     &Plain{time.Unix(1e9, 123456789).UTC()},
		ExpectXML: `<Plain><V>2001-09-09T01:46:40.123456789Z</V></Plain>`,
	},
	// A pointer to struct{} may be used to test for an element's presence.
	{
		// 注:双向测试
		// 注意:这里struct{}是类型
		// new(struct{})返回一个指向struct{}的指针
		Value:     &PresenceTest{new(struct{})},
		ExpectXML: `<PresenceTest><Exists></Exists></PresenceTest>`,
	},
	{
		Value:     &PresenceTest{},
		ExpectXML: `<PresenceTest></PresenceTest>`,
		// 注意此测试和上面的区别,这里是nil指针,上面是非nil指针(指向zero value)
	},

	// A pointer to struct{} may be used to test for an element's presence.
	{
		Value:     &PresenceTest{new(struct{})},
		ExpectXML: `<PresenceTest><Exists></Exists></PresenceTest>`,
	},
	{
		Value:     &PresenceTest{},
		ExpectXML: `<PresenceTest></PresenceTest>`,
	},

	// A []byte field is only nil if the element was not found.
	{
		Value:         &Data{},
		ExpectXML:     `<Data></Data>`,
		UnmarshalOnly: true,
		// 注意:Unmarshal的结果,各字段是nil,因为xml中没有任何标签
	},
	{
		Value:         &Data{Bytes: []byte{}, Custom: MyBytes{}, Attr: []byte{}},
		ExpectXML:     `<Data Attr=""><Bytes></Bytes><Custom></Custom></Data>`,
		UnmarshalOnly: true,
		// 注意和上面的测试的区别,各字段不是nil,而是empty slice
	},

	// Check that []byte works, including named []byte types.
	{
		Value:     &Data{Bytes: []byte("ab"), Custom: MyBytes("cd"), Attr: []byte{'v'}},
		ExpectXML: `<Data Attr="v"><Bytes>ab</Bytes><Custom>cd</Custom></Data>`,
		// 双向测试
	},

	// Test innerxml
	{
		// 根据Marshal文档: a field with tag ",innerxml" is written verbatim, not subject to the usual marshaling procedure.
		Value: &SecretAgent{
			Handle:    "007",
			Identity:  "James Bond",
			Obfuscate: "<redacted/>",
		},
		ExpectXML:   `<agent handle="007"><Identity>James Bond</Identity><redacted/></agent>`,
		MarshalOnly: true,
	},

	{
		Value: &SecretAgent{
			Handle:    "007",
			Identity:  "James Bond",
			Obfuscate: "<Identity>James Bond</Identity><redacted/>",
		},
		ExpectXML:     `<agent handle="007"><Identity>James Bond</Identity><redacted/></agent>`,
		UnmarshalOnly: true,
		// 注:UnmarshalOnly, unmarshal的时候,会把整个<agent>...</agent>之间的内容作为innerxml进行保存
	},

	// Test structs
	{Value: &Port{Type: "ssl", Number: "443"}, ExpectXML: `<port type="ssl">443</port>`},
	// 因为Port定义了"type,attr,omitempty",因此Marshal的时候不会包含type的attr
	{Value: &Port{Number: "443"}, ExpectXML: `<port>443</port>`},
	{Value: &Port{Type: "<unix>"}, ExpectXML: `<port type="&lt;unix&gt;"></port>`},
	// 注: 在Port的定义中,Comment字段先于Number字段,因此 Marshal的时候也是按照这个顺序.
	{Value: &Port{Number: "443", Comment: "https"}, ExpectXML: `<port><!--https-->443</port>`},
	// 注: MarshalOnly, comment是内嵌到<!----> 之间的,如果有空格需要自己添加
	{Value: &Port{Number: "443", Comment: "add space-"}, ExpectXML: `<port><!--add space- -->443</port>`, MarshalOnly: true},
	// 注: Domain.Name字段的定义: Name []byte `xml:",chardata"`
	{Value: &Domain{Name: []byte("google.com&friends")}, ExpectXML: `<domain>google.com&amp;friends</domain>`},
	// 注: 在Domain的定义中,Name字段先于Comment字段
	{Value: &Domain{Name: []byte("google.com"), Comment: []byte(" &friends ")}, ExpectXML: `<domain>google.com<!-- &friends --></domain>`},
	// 注: Book.Title 的定义: Title string `xml:",chardata"` ;  Pride & Prejudice: 《傲慢与偏见》
	{Value: &Book{Title: "Pride & Prejudice"}, ExpectXML: `<book>Pride &amp; Prejudice</book>`},
	{Value: &Event{Year: -3114}, ExpectXML: `<event>-3114</event>`},
	{Value: &Movie{Length: 13440}, ExpectXML: `<movie>13440</movie>`},
	// 注: ?????? 测试是怎么精确比较的 ??????
	{Value: &Pi{Approximation: 3.14159265}, ExpectXML: `<pi>3.1415927</pi>`},
	{Value: &Universe{Visible: 9.3e13}, ExpectXML: `<universe>9.3e+13</universe>`},
	{Value: &Particle{HasMass: true}, ExpectXML: `<particle>true</particle>`},
	{Value: &Departure{When: ParseTime("2013-01-09T00:15:00-09:00")}, ExpectXML: `<departure>2013-01-09T00:15:00-09:00</departure>`},
	{Value: atomValue, ExpectXML: atomXML},
	{
		Value: &Ship{
			Name:  "Heart of Gold",
			Pilot: "Computer",
			Age:   1,
			Drive: ImprobabilityDrive,
			Passenger: []*Passenger{
				{
					Name:   []string{"Zaphod", "Beeblebrox"},
					Weight: 7.25,
				},
				{
					Name:   []string{"Trisha", "McMillen"},
					Weight: 5.5,
				},
				{
					Name:   []string{"Ford", "Prefect"},
					Weight: 7,
				},
				{
					Name:   []string{"Arthur", "Dent"},
					Weight: 6.75,
				},
			},
		},
		ExpectXML: `<spaceship name="Heart of Gold" pilot="Computer">` +
			`<drive>` + strconv.Itoa(int(ImprobabilityDrive)) + `</drive>` +
			`<age>1</age>` +
			`<passenger>` +
			`<name>Zaphod</name>` +
			`<name>Beeblebrox</name>` +
			`<weight>7.25</weight>` +
			`</passenger>` +
			`<passenger>` +
			`<name>Trisha</name>` +
			`<name>McMillen</name>` +
			`<weight>5.5</weight>` +
			`</passenger>` +
			`<passenger>` +
			`<name>Ford</name>` +
			`<name>Prefect</name>` +
			`<weight>7</weight>` +
			`</passenger>` +
			`<passenger>` +
			`<name>Arthur</name>` +
			`<name>Dent</name>` +
			`<weight>6.75</weight>` +
			`</passenger>` +
			`</spaceship>`,
		// 双向测试
		// 注意:secret是非导出,因此输出中不含
	},

	// Test a>b
	{
		Value: &NestedItems{Items: nil, Item1: nil},
		ExpectXML: `<result>` +
			`<Items>` +
			`</Items>` +
			`</result>`,
		// 注意:Items字段生成了
	},
	{
		// 注: 当Marshal的时候,Items字段在xml中生成了,Item1在xml中没有生成
		Value: &NestedItems{Items: []string{}, Item1: []string{}},
		ExpectXML: `<result>` +
			`<Items>` +
			`</Items>` +
			`</result>`,
		MarshalOnly: true,
	},
	{
		Value: &NestedItems{Items: nil, Item1: []string{"A"}},
		ExpectXML: `<result>` +
			`<Items>` +
			`<item1>A</item1>` +
			`</Items>` +
			`</result>`,
		// 注: Item1字段其实是共享了Items在xml中的路径
		// 注: 双向测试
	},
	{
		Value: &NestedItems{Items: []string{"A", "B"}, Item1: nil},
		ExpectXML: `<result>` +
			`<Items>` +
			`<item>A</item>` +
			`<item>B</item>` +
			`</Items>` +
			`</result>`,
	},
	{
		Value: &NestedItems{Items: []string{"A", "B"}, Item1: []string{"C"}},
		ExpectXML: `<result>` +
			`<Items>` +
			`<item>A</item>` +
			`<item>B</item>` +
			`<item1>C</item1>` +
			`</Items>` +
			`</result>`,
	},
	/**
	注: xml.Marshal文档中提到:
	If a field uses a tag "a>b>c", then the element c will be nested inside
	parent elements a and b. Fields that appear next to each other that name
	the same parent will be enclosed in one XML element.
	注意这句话:Fields that appear next to each other that name the same parent will be enclosed in one XML element.
	意思是,如果结构体字段中,通过"a>b>c"表明同一个父元素的,并且是挨着的字段,才会被enclosed in one XML element.
	否则,通过"a>b>c"表明同一个父元素的,但不是挨着的字段,不会被enclosed in one XML element.
	 */
	{
		Value: &NestedOrder{Field1: "C", Field2: "B", Field3: "A"},
		ExpectXML: `<result>` +
			`<parent>` +
			`<c>C</c>` +
			`<b>B</b>` +
			`<a>A</a>` +
			`</parent>` +
			`</result>`,
		// 注:双向测试
		// go结构体中Field1,Field2,Field3在结构体中是三个挨着的字段.
		// xml中a,b,c三个xml标签在一个parent标签下,而不是三个不同的parent标签
	},
	{
		Value: &NilTest{A: "A", B: nil, C: "C"},
		ExpectXML: `<NilTest>` +
			`<parent1>` +
			`<parent2><a>A</a></parent2>` +
			`<parent2><c>C</c></parent2>` +
			`</parent1>` +
			`</NilTest>`,
		MarshalOnly: true, // Uses interface{}
		// Marshal的时候字段可以使用空接口类型
		// 注:MarshalOnly
		// go结构体中,parent1在三个字段中是挨着的,因此xml中共享同一个parent1
		// go结构体中,parent2在A,C字段中不是挨着的,因此xml中没有共享同一个parent2
	},
	{
		Value: &MixedNested{A: "A", B: "B", C: "C", D: "D"},
		ExpectXML: `<result>` +
			`<parent1><a>A</a></parent1>` +
			`<b>B</b>` +
			`<parent1>` +
			`<parent2><c>C</c></parent2>` +
			`<d>D</d>` +
			`</parent1>` +
			`</result>`,
		// 注意: 根据MixedNested结构体的定义,只有C,D会共享同一个parent1(去看看MixedNested是怎么组织字段的)
	},
	{
		Value:     &Service{Port: &Port{Number: "80"}},
		ExpectXML: `<service><host><port>80</port></host></service>`,
		// 注意:marshal时没有出现第二个host标签,因为Service.Extra2是nil
	},
	{
		Value:     &Service{},
		ExpectXML: `<service></service>`,
		// 注意:marshal时2个host标签都没出现
	},
	{
		Value: &Service{Port: &Port{Number: "80"}, Extra1: "A", Extra2: "B"},
		ExpectXML: `<service>` +
			// 第1个host
			`<host><port>80</port></host>` +
			`<Extra1>A</Extra1>` +
			// 第2个host
			`<host><extra2>B</extra2></host>` +
			`</service>`,
		MarshalOnly: true,
	},
	{
		Value: &Service{Port: &Port{Number: "80"}, Extra2: "example"},
		ExpectXML: `<service>` +
			// 第1个host
			`<host><port>80</port></host>` +
			// 第2个host
			`<host><extra2>example</extra2></host>` +
			`</service>`,
		MarshalOnly: true,
	},
	{
		Value: &struct {
			// UnMarshal的时候,不会给XMLName字段赋值,因为是空struct
			XMLName struct{} `xml:"space top"`
			// 可以看出字段A,B,C,C1,D1在同一个x下
			A       string   `xml:"x>a"`
			B       string   `xml:"x>b"`
			// 注: space不是跟x结合, 其实是跟c结合,相当于是 "space (x>c)"
			C       string   `xml:"space x>c"`
			// 注: space1不是跟x结合, 其实是跟c结合
			C1      string   `xml:"space1 x>c"`
			// 注: space1不是跟x结合, 其实是跟d结合
			D1      string   `xml:"space1 x>d"`
		}{
			A:  "a",
			B:  "b",
			C:  "c",
			C1: "c1",
			D1: "d1",
		},
		ExpectXML: `<top xmlns="space">` +
			`<x><a>a</a><b>b</b><c xmlns="space">c</c>` +
			`<c xmlns="space1">c1</c>` +
			`<d xmlns="space1">d1</d>` +
			`</x>` +
			`</top>`,
		// 注意:双向测试
	},
	{
		Value: &struct {
			XMLName Name
			// 可以看出,所有字段在同一个x下
			A       string `xml:"x>a"`
			B       string `xml:"x>b"`
			C       string `xml:"space x>c"`
			C1      string `xml:"space1 x>c"`
			D1      string `xml:"space1 x>d"`
		}{
			XMLName: Name{
				Space: "space0",
				Local: "top",
			},
			A:  "a",
			B:  "b",
			C:  "c",
			C1: "c1",
			D1: "d1",
		},
		ExpectXML: `<top xmlns="space0">` +
			`<x><a>a</a><b>b</b>` +
			`<c xmlns="space">c</c>` +
			`<c xmlns="space1">c1</c>` +
			`<d xmlns="space1">d1</d>` +
			`</x>` +
			`</top>`,
		// 注:所有字段共享同一个x父元素
	},
	{
		Value: &struct {
			XMLName struct{} `xml:"top"`
			// 可以看出,所有字段在同一个x下
			B       string   `xml:"space x>b"`
			B1      string   `xml:"space1 x>b"`
		}{
			B:  "b",
			B1: "b1",
		},
		ExpectXML: `<top>` +
			`<x><b xmlns="space">b</b>` +
			`<b xmlns="space1">b1</b></x>` +
			`</top>`,
		// 注:所有字段共享同一个x父元素
	},

	// Test struct embedding到此
	{
		Value: &EmbedA{
			// EmbedC是匿名字段
			EmbedC: EmbedC{
				FieldA1: "", // Shadowed by A.A
				FieldA2: "", // Shadowed by A.A
				FieldB:  "A.C.B",
				FieldC:  "A.C.C",
			},
			EmbedB: EmbedB{
				FieldB: "A.B.B",
				EmbedC: &EmbedC{
					FieldA1: "A.B.C.A1",
					FieldA2: "A.B.C.A2",
					FieldB:  "", // Shadowed by A.B.B
					FieldC:  "A.B.C.C",
				},
			},
			FieldA: "A.A",
			embedD: embedD{
				FieldE: "A.D.E",
			},
		},
		ExpectXML: `<EmbedA>` +
			`<FieldB>A.C.B</FieldB>` +
			`<FieldC>A.C.C</FieldC>` +
			`<EmbedB>` +
			`<FieldB>A.B.B</FieldB>` +
			`<FieldA>` +
			`<A1>A.B.C.A1</A1>` +
			`<A2>A.B.C.A2</A2>` +
			`</FieldA>` +
			`<FieldC>A.B.C.C</FieldC>` +
			`</EmbedB>` +
			`<FieldA>A.A</FieldA>` +
			`<FieldE>A.D.E</FieldE>` +
			`</EmbedA>`,
	},

	// Test that name casing matters
	{
		// 此测试说明xml的marshal和unmashal是区分大小写的
		Value:     &NameCasing{Xy: "mixed", XY: "upper", XyA: "mixedA", XYA: "upperA"},
		ExpectXML: `<casing Xy="mixedA" XY="upperA"><Xy>mixed</Xy><XY>upper</XY></casing>`,
	},

	// Test the order in which the XML element name is chosen
	{
		Value: &NamePrecedence{
			FromTag:     XMLNameWithoutTag{Value: "A"},
			FromNameVal: XMLNameWithoutTag{XMLName: Name{Local: "InXMLName"}, Value: "B"},
			FromNameTag: XMLNameWithTag{Value: "C"},
			InFieldName: "D",
		},
		ExpectXML: `<Parent>` +
			// 标签名来自 NamePrecedence.FromTag的tag
			`<InTag>A</InTag>` +
			// 标签名来自 NamePrecedence.FromNameVal.XMLName(xml.Name类型)字段的值
			`<InXMLName>B</InXMLName>` +
			// 标签名来自 NamePrecedence.FromNameTag.XMLName(xml.Name类型)字段的tag
			`<InXMLNameTag>C</InXMLNameTag>` +
			// 标签名来自结构体的字段名 NamePrecedence.InFieldName
			`<InFieldName>D</InFieldName>` +
			`</Parent>`,
		MarshalOnly: true,
	},
	{
		// 这个是在测试UnmarshalOnly的时候xml标签名的优先级
		Value: &NamePrecedence{
			XMLName:     Name{Local: "Parent"},
			// 使用tag的xml来匹配标签名,最后标签名信息会写入XMLName.Name字段
			FromTag:     XMLNameWithoutTag{XMLName: Name{Local: "InTag"}, Value: "A"},
			// 使用 XMLName Name字段的值来匹配标签名,最后标签名信息会写入XMLName.Name字段
			FromNameVal: XMLNameWithoutTag{XMLName: Name{Local: "FromNameVal"}, Value: "B"},
			// 使用 XMLName Name字段的tag来匹配标签名,最后标签名信息会写入XMLName.Name字段
			FromNameTag: XMLNameWithTag{XMLName: Name{Local: "InXMLNameTag"}, Value: "C"},
			// 使用结构体字段名来匹配标签名,这时不存在标签名信息的写入,因为没有XMLName.Name字段
			InFieldName: "D",
		},
		ExpectXML: `<Parent>` +
			`<InTag>A</InTag>` +
			`<FromNameVal>B</FromNameVal>` +
			`<InXMLNameTag>C</InXMLNameTag>` +
			`<InFieldName>D</InFieldName>` +
			`</Parent>`,
		UnmarshalOnly: true,
	},

	// xml.Name works in a plain field as well.
	{
		Value:     &NameInField{Name{Space: "ns", Local: "foo"}},
		ExpectXML: `<NameInField><foo xmlns="ns"></foo></NameInField>`,
		// Foo字段类型为Name,虽然定义了标签,但还是可以通过构造Name类型的值设置space和local
		// 注意:结构体名NameInField作为了整个xml的外层元素名
	},
	{
		Value:         &NameInField{Name{Space: "ns", Local: "foo"}},
		ExpectXML:     `<NameInField><foo xmlns="ns"><ignore></ignore></foo></NameInField>`,
		UnmarshalOnly: true,
		// 注意:UnmarshalOnly, <ignore></ignore> 这段被忽略了
	},

	// Marshaling zero xml.Name uses the tag or field name.
	{
		Value:       &NameInField{},
		// 由于NameInField.Foo(类型为xml.Name)是zero value, 因此MarshalOnly后生成的xml标签名是使用tag,而不是NameInField.Foo字段的值
		ExpectXML:   `<NameInField><foo xmlns="ns"></foo></NameInField>`,
		MarshalOnly: true,
	},

	// Test attributes
	{
		Value: &AttrTest{
			Int:   8,
			Named: 9,
			Float: 23.5,
			Uint8: 255,
			Bool:  true,
			Str:   "str",
			Bytes: []byte("byt"),
		},
		ExpectXML: `<AttrTest Int="8" int="9" Float="23.5" Uint8="255"` +
			` Bool="true" Str="str" Bytes="byt"></AttrTest>`,
	},
	{
		// 注: &AttrTest{Bytes: []byte{}} 与 &{Int:0 Named:0 Float:0 Uint8:0 Bool:false Str: Bytes:[]} 的写法是等价的
		// 注: AttrTest{Bytes: []byte{}} 中 Bytes 字段不是 nil, 而是 empty slice
		Value: &AttrTest{Bytes: []byte{}},
		ExpectXML: `<AttrTest Int="0" int="0" Float="0" Uint8="0"` +
			` Bool="false" Str="" Bytes=""></AttrTest>`,
	},
	{
		Value: &AttrsTest{
			Attrs: []Attr{
				{Name: Name{Local: "Answer"}, Value: "42"},
				{Name: Name{Local: "Int"}, Value: "8"},
				{Name: Name{Local: "int"}, Value: "9"},
				{Name: Name{Local: "Float"}, Value: "23.5"},
				{Name: Name{Local: "Uint8"}, Value: "255"},
				{Name: Name{Local: "Bool"}, Value: "true"},
				{Name: Name{Local: "Str"}, Value: "str"},
				{Name: Name{Local: "Bytes"}, Value: "byt"},
			},
		},
		ExpectXML:   `<AttrsTest Answer="42" Int="8" int="9" Float="23.5" Uint8="255" Bool="true" Str="str" Bytes="byt" Int="0" int="0" Float="0" Uint8="0" Bool="false" Str="" Bytes=""></AttrsTest>`,
		MarshalOnly: true,
		// 注意:ExpectXML中属性名有重复.
		// 注意:MarshalOnly
		// 注意:AttrsTest.Attrs中的",any,attr"在Marshal时是没有作用的
	},
	{
		// 标记: any1111
		// 注意在Unmarshal的文档中提到
		// * If the XML element has an attribute not handled by the previous
		//    rule and the struct has a field with an associated tag containing
		//    ",any,attr", Unmarshal records the attribute value in the first such field.
		Value: &AttrsTest{
			Attrs: []Attr{
				{Name: Name{Local: "Answer"}, Value: "42"},
			},
			Int:   8,
			Named: 9,
			Float: 23.5,
			Uint8: 255,
			Bool:  true,
			Str:   "str",
			Bytes: []byte("byt"),
		},
		ExpectXML: `<AttrsTest Answer="42" Int="8" int="9" Float="23.5" Uint8="255" Bool="true" Str="str" Bytes="byt"></AttrsTest>`,
	},
	{
		Value: &AttrsTest{
			Attrs: []Attr{
				{Name: Name{Local: "Int"}, Value: "0"},
				{Name: Name{Local: "int"}, Value: "0"},
				{Name: Name{Local: "Float"}, Value: "0"},
				{Name: Name{Local: "Uint8"}, Value: "0"},
				{Name: Name{Local: "Bool"}, Value: "false"},
				{Name: Name{Local: "Str"}},
				{Name: Name{Local: "Bytes"}},
			},
			Bytes: []byte{},
		},
		ExpectXML:   `<AttrsTest Int="0" int="0" Float="0" Uint8="0" Bool="false" Str="" Bytes="" Int="0" int="0" Float="0" Uint8="0" Bool="false" Str="" Bytes=""></AttrsTest>`,
		MarshalOnly: true,
		// 注意:MarshalOnly
	},
	{
		Value: &OmitAttrTest{
			Int:   8,
			Named: 9,
			Float: 23.5,
			Uint8: 255,
			Bool:  true,
			Str:   "str",
			Bytes: []byte("byt"),
			// 注意: 对空字符串empty取地址,是一个非空指针,因此生成的xml中会包含PStr属性
			PStr:  &empty,
		},
		ExpectXML: `<OmitAttrTest Int="8" int="9" Float="23.5" Uint8="255"` +
			` Bool="true" Str="str" Bytes="byt" PStr=""></OmitAttrTest>`,
	},
	{
		// 字段值全部是empty,因此生成的xml中不会包含任何attr
		Value:     &OmitAttrTest{},
		ExpectXML: `<OmitAttrTest></OmitAttrTest>`,
	},

	// pointer fields
	{
		// Marshal文档中提到
		// Marshal handles a pointer by marshaling the value it points at or, if the
		// pointer is nil, by writing nothing.
		Value:       &PointerFieldsTest{Name: &nameAttr, Age: &ageAttr, Contents: &contentsAttr},
		ExpectXML:   `<dummy name="Sarah" age="12">lorem ipsum</dummy>`,
		MarshalOnly: true,
		// 注意: Empty是空指针,因此ExpectXML中没有生成
	},

	// empty chardata pointer field
	{
		// Marshal文档中提到
		// Marshal handles a pointer by marshaling the value it points at or, if the
		// pointer is nil, by writing nothing.
		Value:       &ChardataEmptyTest{},
		ExpectXML:   `<test></test>`,
		MarshalOnly: true,
	},

	// omitempty on fields
	{
		Value: &OmitFieldTest{
			Int:   8,
			Named: 9,
			Float: 23.5,
			Uint8: 255,
			Bool:  true,
			Str:   "str",
			Bytes: []byte("byt"),
			// 注意: 对空字符串取地址,是一个非空指针,因此Marshal的结果xml会包含PStr标签
			PStr:  &empty,
			// 注意: 对struct取地址,是一个非空指针,因此Marshal的结果xml会包含Ptr标签
			Ptr:   &PresenceTest{},
		},
		ExpectXML: `<OmitFieldTest>` +
			`<Int>8</Int>` +
			`<int>9</int>` +
			`<Float>23.5</Float>` +
			`<Uint8>255</Uint8>` +
			`<Bool>true</Bool>` +
			`<Str>str</Str>` +
			`<Bytes>byt</Bytes>` +
			`<PStr></PStr>` +
			`<Ptr></Ptr>` +
			`</OmitFieldTest>`,
	},
	{
		Value:     &OmitFieldTest{},
		ExpectXML: `<OmitFieldTest></OmitFieldTest>`,
		// 全部是zero value,因此xml中任何字段都没有输出
	},

	// Test ",any"
	{
		// any1112
		// UnMarshal 文档中提到
		//   * If the XML element contains a sub-element that hasn't matched any
		//      of the above rules and the struct has a field with tag ",any",
		//      unmarshal maps the sub-element to that struct field.
		//      此时不会管标签名和字段名或tag是否匹配
		ExpectXML: `<a><nested><value>known</value></nested><other><sub>unknown</sub></other></a>`,
		Value: &AnyTest{
			Nested: "known",
			AnyField: AnyHolder{
				XMLName: Name{Local: "other"},
				XML:     "<sub>unknown</sub>",
			},
		},
		// 注意:双向测试
	},
	{
		// any1112
		// UnMarshal 文档中提到
		//   * If the XML element contains a sub-element that hasn't matched any
		//      of the above rules and the struct has a field with tag ",any",
		//      unmarshal maps the sub-element to that struct field.
		Value: &AnyTest{Nested: "known",
			AnyField: AnyHolder{
				XML:     "<unknown/>",
				XMLName: Name{Local: "AnyField"},
			},
		},
		ExpectXML: `<a><nested><value>known</value></nested><AnyField><unknown/></AnyField></a>`,
		// 注意:双向测试
	},
	{
		ExpectXML: `<a><nested><value>b</value></nested></a>`,
		Value: &AnyOmitTest{
			Nested: "b",
		},
		// 注意:字段定义为 AnyField *AnyHolder `xml:",any,omitempty"`, 因此当 AnyField 不存在,输出时就会忽略 AnyField 字段.
	},
	{
		ExpectXML: `<a><nested><value>b</value></nested><c><d>e</d></c><g xmlns="f"><h>i</h></g></a>`,
		Value: &AnySliceTest{
			Nested: "b",
			AnyField: []AnyHolder{
				{
					XMLName: Name{Local: "c"},
					XML:     "<d>e</d>",
				},
				{
					XMLName: Name{Space: "f", Local: "g"},
					XML:     "<h>i</h>",
				},
			},
		},
		// 注意:双向测试
	},
	{
		// UnMarshal的结果,AnySliceTest.AnyField是nil slice
		ExpectXML: `<a><nested><value>b</value></nested></a>`,
		Value: &AnySliceTest{
			Nested: "b",
		},
	},

	// Test recursive types.
	{
		Value: &RecurseA{
			A: "a1",
			B: &RecurseB{
				A: &RecurseA{"a2", nil},
				B: "b1",
			},
		},
		// <RecurseA>
		// 		<A>a1</A>
		// 		<B>
		// 			<A>
		// 				<A>a2</A>
		// 			</A>
		// 			<B>b1</B>
		// 		</B>
		// </RecurseA>
		ExpectXML: `<RecurseA><A>a1</A><B><A><A>a2</A></A><B>b1</B></B></RecurseA>`,
	},

	// Test ignoring fields via "-" tag
	{
		ExpectXML: `<IgnoreTest></IgnoreTest>`,
		Value:     &IgnoreTest{},
		// 双向测试
	},
	{
		ExpectXML:   `<IgnoreTest></IgnoreTest>`,
		Value:       &IgnoreTest{PublicSecret: "can't tell"},
		MarshalOnly: true,
		// 注意:MarshalOnly
		// 由于定义了`xml:"-"`,因此ExpectXML不会包含PublicSecret
	},
	{
		ExpectXML:     `<IgnoreTest><PublicSecret>ignore me</PublicSecret></IgnoreTest>`,
		Value:         &IgnoreTest{},
		UnmarshalOnly: true,
		// 根据Unmarshal的文档: A struct field with tag "-" is never unmarshaled into.
	},

	// Test escaping.
	{
		ExpectXML: `<a><nested><value>dquote: &#34;; squote: &#39;; ampersand: &amp;; less: &lt;; greater: &gt;;</value></nested><empty></empty></a>`,
		Value: &AnyTest{
			Nested:   `dquote: "; squote: '; ampersand: &; less: <; greater: >;`,
			AnyField: AnyHolder{XMLName: Name{Local: "empty"}},
		},
		// 注意:双向测试
	},
	{
		ExpectXML: `<a><nested><value>newline: &#xA;; cr: &#xD;; tab: &#x9;;</value></nested><AnyField></AnyField></a>`,
		Value: &AnyTest{
			Nested:   "newline: \n; cr: \r; tab: \t;",
			AnyField: AnyHolder{XMLName: Name{Local: "AnyField"}},
		},
		// 注意:双向测试
	},
	{
		ExpectXML: "<a><nested><value>1\r2\r\n3\n\r4\n5</value></nested></a>",
		Value: &AnyTest{
			Nested: "1\n2\n3\n\n4\n5",
		},
		UnmarshalOnly: true,
		// 注意:UnmarshalOnly
		// 注意: 转换规则如下
		// \r   => \n
		// \r\n => \n
		// \n\r => \n\n
	},
	{
		ExpectXML: `<EmbedInt><MyInt>42</MyInt></EmbedInt>`,
		Value: &EmbedInt{
			MyInt: 42,
		},
	},
	// Test outputting CDATA-wrapped text.
	{
		ExpectXML: `<CDataTest></CDataTest>`,
		Value:     &CDataTest{},
	},
	{
		ExpectXML: `<CDataTest><![CDATA[http://example.com/tests/1?foo=1&bar=baz]]></CDataTest>`,
		Value: &CDataTest{
			Chardata: "http://example.com/tests/1?foo=1&bar=baz",
		},
	},
	{
		ExpectXML: `<CDataTest><![CDATA[Literal <![CDATA[Nested]]]]><![CDATA[>!]]></CDataTest>`,
		Value: &CDataTest{
			Chardata: "Literal <![CDATA[Nested]]>!",
		},
		// 参考: http://blog.sina.com.cn/s/blog_53a0f0810100g8i0.html
	},
	{
		ExpectXML: `<CDataTest><![CDATA[<![CDATA[Nested]]]]><![CDATA[> Literal!]]></CDataTest>`,
		Value: &CDataTest{
			Chardata: "<![CDATA[Nested]]> Literal!",
		},
		// 不看
	},
	{
		ExpectXML: `<CDataTest><![CDATA[<![CDATA[Nested]]]]><![CDATA[> Literal! <![CDATA[Nested]]]]><![CDATA[> Literal!]]></CDataTest>`,
		Value: &CDataTest{
			Chardata: "<![CDATA[Nested]]> Literal! <![CDATA[Nested]]> Literal!",
		},
		// 不看
	},
	{
		ExpectXML: `<CDataTest><![CDATA[<![CDATA[<![CDATA[Nested]]]]><![CDATA[>]]]]><![CDATA[>]]></CDataTest>`,
		Value: &CDataTest{
			Chardata: "<![CDATA[<![CDATA[Nested]]>]]>",
		},
		// 不看
	},

	// Test omitempty with parent chain; see golang.org/issue/4168.
	{
		ExpectXML: `<Strings><A></A></Strings>`,
		Value:     &Strings{},
		// 注: Marshal的时候,即使Strings.X为空,但还是生产了A标签,
		// 也就是说,X []string `xml:"A>B,omitempty"`中的omitempty是针对B而言,而不是针对A
	},
	// Custom marshalers.
	{
		ExpectXML: `<MyMarshalerTest>hello world</MyMarshalerTest>`,
		Value:     &MyMarshalerTest{},
		// @see : 示范了如何实现 Marshaler 接口
	},
	{
		ExpectXML: `<MarshalerStruct Foo="hello world"></MarshalerStruct>`,
		Value:     &MarshalerStruct{},
		// @see
	},
	{
		ExpectXML: `<outer xmlns="testns" int="10"></outer>`,
		Value:     &OuterStruct{IntAttr: 10},
		// @see
	},
	{
		ExpectXML: `<test xmlns="outerns" int="10"></test>`,
		Value:     &OuterNamedStruct{XMLName: Name{Space: "outerns", Local: "test"}, IntAttr: 10},
	},
	{
		ExpectXML: `<test xmlns="outerns" int="10"></test>`,
		Value:     &OuterNamedOrderedStruct{XMLName: Name{Space: "outerns", Local: "test"}, IntAttr: 10},
	},
	{
		ExpectXML: `<outer xmlns="testns" int="10"></outer>`,
		Value:     &OuterOuterStruct{OuterStruct{IntAttr: 10}},
	},
	{
		ExpectXML: `<NestedAndChardata><A><B></B><B></B></A>test</NestedAndChardata>`,
		Value:     &NestedAndChardata{AB: make([]string, 2), Chardata: "test"},
	},
	{
		ExpectXML: `<NestedAndComment><A><B></B><B></B></A><!--test--></NestedAndComment>`,
		Value:     &NestedAndComment{AB: make([]string, 2), Comment: "test"},
	},
	{
		ExpectXML: `<NestedAndCData><A><B></B><B></B></A><![CDATA[test]]></NestedAndCData>`,
		Value:     &NestedAndCData{AB: make([]string, 2), CDATA: "test"},
	},
	// Test pointer indirection in various kinds of fields.
	// https://golang.org/issue/19063
	{
		ExpectXML:   `<IndirComment><T1></T1><!--hi--><T2></T2></IndirComment>`,
		Value:       &IndirComment{Comment: stringptr("hi")},
		MarshalOnly: true,
		// 注: pointer indirection
	},
	{
		ExpectXML:   `<IndirComment><T1></T1><T2></T2></IndirComment>`,
		Value:       &IndirComment{Comment: stringptr("")},
		MarshalOnly: true,
		// 注: pointer indirection后是空字符串,因此ExpectXML中没有生成comment
	},
	{
		ExpectXML:    `<IndirComment><T1></T1><T2></T2></IndirComment>`,
		Value:        &IndirComment{Comment: nil},
		MarshalError: "xml: bad type for comment field of xml.IndirComment",
		// 注: Comment字段是空指针会报错
	},
	{
		ExpectXML:     `<IndirComment><T1></T1><!--hi--><T2></T2></IndirComment>`,
		Value:         &IndirComment{Comment: nil},
		UnmarshalOnly: true,
		// 注: 这里Unmarshal的时候,为什么Comment是nil??????
	},
	{
		ExpectXML:   `<IfaceComment><T1></T1><!--hi--><T2></T2></IfaceComment>`,
		Value:       &IfaceComment{Comment: "hi"},
		MarshalOnly: true,
	},
	{
		ExpectXML:     `<IfaceComment><T1></T1><!--hi--><T2></T2></IfaceComment>`,
		Value:         &IfaceComment{Comment: nil},
		UnmarshalOnly: true,
		// 注: 这里Unmarshal的时候,为什么Comment是nil??????
	},
	{
		ExpectXML:    `<IfaceComment><T1></T1><T2></T2></IfaceComment>`,
		Value:        &IfaceComment{Comment: nil},
		MarshalError: "xml: bad type for comment field of xml.IfaceComment",
		// 注:会报错
	},
	{
		ExpectXML:     `<IfaceComment><T1></T1><T2></T2></IfaceComment>`,
		Value:         &IfaceComment{Comment: nil},
		UnmarshalOnly: true,
		// 后面的测试别看了,太烦
	},
	{
		ExpectXML: `<DirectComment><T1></T1><!--hi--><T2></T2></DirectComment>`,
		Value:     &DirectComment{Comment: string("hi")},
	},
	{
		ExpectXML: `<DirectComment><T1></T1><T2></T2></DirectComment>`,
		Value:     &DirectComment{Comment: string("")},
	},
	{
		ExpectXML: `<IndirChardata><T1></T1>hi<T2></T2></IndirChardata>`,
		Value:     &IndirChardata{Chardata: stringptr("hi")},
	},
	{
		ExpectXML:     `<IndirChardata><T1></T1><![CDATA[hi]]><T2></T2></IndirChardata>`,
		Value:         &IndirChardata{Chardata: stringptr("hi")},
		UnmarshalOnly: true, // marshals without CDATA
	},
	{
		ExpectXML: `<IndirChardata><T1></T1><T2></T2></IndirChardata>`,
		Value:     &IndirChardata{Chardata: stringptr("")},
	},
	{
		ExpectXML:   `<IndirChardata><T1></T1><T2></T2></IndirChardata>`,
		Value:       &IndirChardata{Chardata: nil},
		MarshalOnly: true, // unmarshal leaves Chardata=stringptr("")
	},
	{
		ExpectXML:      `<IfaceChardata><T1></T1>hi<T2></T2></IfaceChardata>`,
		Value:          &IfaceChardata{Chardata: string("hi")},
		UnmarshalError: "cannot unmarshal into interface {}",
	},
	{
		ExpectXML:      `<IfaceChardata><T1></T1><![CDATA[hi]]><T2></T2></IfaceChardata>`,
		Value:          &IfaceChardata{Chardata: string("hi")},
		UnmarshalOnly:  true, // marshals without CDATA
		UnmarshalError: "cannot unmarshal into interface {}",
	},
	{
		ExpectXML:      `<IfaceChardata><T1></T1><T2></T2></IfaceChardata>`,
		Value:          &IfaceChardata{Chardata: string("")},
		UnmarshalError: "cannot unmarshal into interface {}",
	},
	{
		ExpectXML:      `<IfaceChardata><T1></T1><T2></T2></IfaceChardata>`,
		Value:          &IfaceChardata{Chardata: nil},
		UnmarshalError: "cannot unmarshal into interface {}",
	},
	{
		ExpectXML: `<DirectChardata><T1></T1>hi<T2></T2></DirectChardata>`,
		Value:     &DirectChardata{Chardata: string("hi")},
	},
	{
		ExpectXML:     `<DirectChardata><T1></T1><![CDATA[hi]]><T2></T2></DirectChardata>`,
		Value:         &DirectChardata{Chardata: string("hi")},
		UnmarshalOnly: true, // marshals without CDATA
	},
	{
		ExpectXML: `<DirectChardata><T1></T1><T2></T2></DirectChardata>`,
		Value:     &DirectChardata{Chardata: string("")},
	},
	{
		ExpectXML: `<IndirCDATA><T1></T1><![CDATA[hi]]><T2></T2></IndirCDATA>`,
		Value:     &IndirCDATA{CDATA: stringptr("hi")},
	},
	{
		ExpectXML:     `<IndirCDATA><T1></T1>hi<T2></T2></IndirCDATA>`,
		Value:         &IndirCDATA{CDATA: stringptr("hi")},
		UnmarshalOnly: true, // marshals with CDATA
	},
	{
		ExpectXML: `<IndirCDATA><T1></T1><T2></T2></IndirCDATA>`,
		Value:     &IndirCDATA{CDATA: stringptr("")},
	},
	{
		ExpectXML:   `<IndirCDATA><T1></T1><T2></T2></IndirCDATA>`,
		Value:       &IndirCDATA{CDATA: nil},
		MarshalOnly: true, // unmarshal leaves CDATA=stringptr("")
	},
	{
		ExpectXML:      `<IfaceCDATA><T1></T1><![CDATA[hi]]><T2></T2></IfaceCDATA>`,
		Value:          &IfaceCDATA{CDATA: string("hi")},
		UnmarshalError: "cannot unmarshal into interface {}",
	},
	{
		ExpectXML:      `<IfaceCDATA><T1></T1>hi<T2></T2></IfaceCDATA>`,
		Value:          &IfaceCDATA{CDATA: string("hi")},
		UnmarshalOnly:  true, // marshals with CDATA
		UnmarshalError: "cannot unmarshal into interface {}",
	},
	{
		ExpectXML:      `<IfaceCDATA><T1></T1><T2></T2></IfaceCDATA>`,
		Value:          &IfaceCDATA{CDATA: string("")},
		UnmarshalError: "cannot unmarshal into interface {}",
	},
	{
		ExpectXML:      `<IfaceCDATA><T1></T1><T2></T2></IfaceCDATA>`,
		Value:          &IfaceCDATA{CDATA: nil},
		UnmarshalError: "cannot unmarshal into interface {}",
	},
	{
		ExpectXML: `<DirectCDATA><T1></T1><![CDATA[hi]]><T2></T2></DirectCDATA>`,
		Value:     &DirectCDATA{CDATA: string("hi")},
	},
	{
		ExpectXML:     `<DirectCDATA><T1></T1>hi<T2></T2></DirectCDATA>`,
		Value:         &DirectCDATA{CDATA: string("hi")},
		UnmarshalOnly: true, // marshals with CDATA
	},
	{
		ExpectXML: `<DirectCDATA><T1></T1><T2></T2></DirectCDATA>`,
		Value:     &DirectCDATA{CDATA: string("")},
	},
	{
		ExpectXML:   `<IndirInnerXML><T1></T1><hi/><T2></T2></IndirInnerXML>`,
		Value:       &IndirInnerXML{InnerXML: stringptr("<hi/>")},
		MarshalOnly: true,
	},
	{
		ExpectXML:   `<IndirInnerXML><T1></T1><T2></T2></IndirInnerXML>`,
		Value:       &IndirInnerXML{InnerXML: stringptr("")},
		MarshalOnly: true,
	},
	{
		ExpectXML: `<IndirInnerXML><T1></T1><T2></T2></IndirInnerXML>`,
		Value:     &IndirInnerXML{InnerXML: nil},
	},
	{
		ExpectXML:     `<IndirInnerXML><T1></T1><hi/><T2></T2></IndirInnerXML>`,
		Value:         &IndirInnerXML{InnerXML: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML:   `<IfaceInnerXML><T1></T1><hi/><T2></T2></IfaceInnerXML>`,
		Value:       &IfaceInnerXML{InnerXML: "<hi/>"},
		MarshalOnly: true,
	},
	{
		ExpectXML:     `<IfaceInnerXML><T1></T1><hi/><T2></T2></IfaceInnerXML>`,
		Value:         &IfaceInnerXML{InnerXML: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML: `<IfaceInnerXML><T1></T1><T2></T2></IfaceInnerXML>`,
		Value:     &IfaceInnerXML{InnerXML: nil},
	},
	{
		ExpectXML:     `<IfaceInnerXML><T1></T1><T2></T2></IfaceInnerXML>`,
		Value:         &IfaceInnerXML{InnerXML: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML:   `<DirectInnerXML><T1></T1><hi/><T2></T2></DirectInnerXML>`,
		Value:       &DirectInnerXML{InnerXML: string("<hi/>")},
		MarshalOnly: true,
	},
	{
		ExpectXML:     `<DirectInnerXML><T1></T1><hi/><T2></T2></DirectInnerXML>`,
		Value:         &DirectInnerXML{InnerXML: string("<T1></T1><hi/><T2></T2>")},
		UnmarshalOnly: true,
	},
	{
		ExpectXML:   `<DirectInnerXML><T1></T1><T2></T2></DirectInnerXML>`,
		Value:       &DirectInnerXML{InnerXML: string("")},
		MarshalOnly: true,
	},
	{
		ExpectXML:     `<DirectInnerXML><T1></T1><T2></T2></DirectInnerXML>`,
		Value:         &DirectInnerXML{InnerXML: string("<T1></T1><T2></T2>")},
		UnmarshalOnly: true,
	},
	{
		ExpectXML: `<IndirElement><T1></T1><Element>hi</Element><T2></T2></IndirElement>`,
		Value:     &IndirElement{Element: stringptr("hi")},
	},
	{
		ExpectXML: `<IndirElement><T1></T1><Element></Element><T2></T2></IndirElement>`,
		Value:     &IndirElement{Element: stringptr("")},
	},
	{
		ExpectXML: `<IndirElement><T1></T1><T2></T2></IndirElement>`,
		Value:     &IndirElement{Element: nil},
	},
	{
		ExpectXML:   `<IfaceElement><T1></T1><Element>hi</Element><T2></T2></IfaceElement>`,
		Value:       &IfaceElement{Element: "hi"},
		MarshalOnly: true,
	},
	{
		ExpectXML:     `<IfaceElement><T1></T1><Element>hi</Element><T2></T2></IfaceElement>`,
		Value:         &IfaceElement{Element: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML: `<IfaceElement><T1></T1><T2></T2></IfaceElement>`,
		Value:     &IfaceElement{Element: nil},
	},
	{
		ExpectXML:     `<IfaceElement><T1></T1><T2></T2></IfaceElement>`,
		Value:         &IfaceElement{Element: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML: `<DirectElement><T1></T1><Element>hi</Element><T2></T2></DirectElement>`,
		Value:     &DirectElement{Element: string("hi")},
	},
	{
		ExpectXML: `<DirectElement><T1></T1><Element></Element><T2></T2></DirectElement>`,
		Value:     &DirectElement{Element: string("")},
	},
	{
		ExpectXML: `<IndirOmitEmpty><T1></T1><OmitEmpty>hi</OmitEmpty><T2></T2></IndirOmitEmpty>`,
		Value:     &IndirOmitEmpty{OmitEmpty: stringptr("hi")},
	},
	{
		// Note: Changed in Go 1.8 to include <OmitEmpty> element (because x.OmitEmpty != nil).
		ExpectXML:   `<IndirOmitEmpty><T1></T1><OmitEmpty></OmitEmpty><T2></T2></IndirOmitEmpty>`,
		Value:       &IndirOmitEmpty{OmitEmpty: stringptr("")},
		MarshalOnly: true,
	},
	{
		ExpectXML:     `<IndirOmitEmpty><T1></T1><OmitEmpty></OmitEmpty><T2></T2></IndirOmitEmpty>`,
		Value:         &IndirOmitEmpty{OmitEmpty: stringptr("")},
		UnmarshalOnly: true,
	},
	{
		ExpectXML: `<IndirOmitEmpty><T1></T1><T2></T2></IndirOmitEmpty>`,
		Value:     &IndirOmitEmpty{OmitEmpty: nil},
	},
	{
		ExpectXML:   `<IfaceOmitEmpty><T1></T1><OmitEmpty>hi</OmitEmpty><T2></T2></IfaceOmitEmpty>`,
		Value:       &IfaceOmitEmpty{OmitEmpty: "hi"},
		MarshalOnly: true,
	},
	{
		ExpectXML:     `<IfaceOmitEmpty><T1></T1><OmitEmpty>hi</OmitEmpty><T2></T2></IfaceOmitEmpty>`,
		Value:         &IfaceOmitEmpty{OmitEmpty: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML: `<IfaceOmitEmpty><T1></T1><T2></T2></IfaceOmitEmpty>`,
		Value:     &IfaceOmitEmpty{OmitEmpty: nil},
	},
	{
		ExpectXML:     `<IfaceOmitEmpty><T1></T1><T2></T2></IfaceOmitEmpty>`,
		Value:         &IfaceOmitEmpty{OmitEmpty: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML: `<DirectOmitEmpty><T1></T1><OmitEmpty>hi</OmitEmpty><T2></T2></DirectOmitEmpty>`,
		Value:     &DirectOmitEmpty{OmitEmpty: string("hi")},
	},
	{
		ExpectXML: `<DirectOmitEmpty><T1></T1><T2></T2></DirectOmitEmpty>`,
		Value:     &DirectOmitEmpty{OmitEmpty: string("")},
	},
	{
		ExpectXML: `<IndirAny><T1></T1><Any>hi</Any><T2></T2></IndirAny>`,
		Value:     &IndirAny{Any: stringptr("hi")},
	},
	{
		ExpectXML: `<IndirAny><T1></T1><Any></Any><T2></T2></IndirAny>`,
		Value:     &IndirAny{Any: stringptr("")},
	},
	{
		ExpectXML: `<IndirAny><T1></T1><T2></T2></IndirAny>`,
		Value:     &IndirAny{Any: nil},
	},
	{
		ExpectXML:   `<IfaceAny><T1></T1><Any>hi</Any><T2></T2></IfaceAny>`,
		Value:       &IfaceAny{Any: "hi"},
		MarshalOnly: true,
	},
	{
		ExpectXML:     `<IfaceAny><T1></T1><Any>hi</Any><T2></T2></IfaceAny>`,
		Value:         &IfaceAny{Any: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML: `<IfaceAny><T1></T1><T2></T2></IfaceAny>`,
		Value:     &IfaceAny{Any: nil},
	},
	{
		ExpectXML:     `<IfaceAny><T1></T1><T2></T2></IfaceAny>`,
		Value:         &IfaceAny{Any: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML: `<DirectAny><T1></T1><Any>hi</Any><T2></T2></DirectAny>`,
		Value:     &DirectAny{Any: string("hi")},
	},
	{
		ExpectXML: `<DirectAny><T1></T1><Any></Any><T2></T2></DirectAny>`,
		Value:     &DirectAny{Any: string("")},
	},
	{
		ExpectXML:     `<IndirFoo><T1></T1><Foo>hi</Foo><T2></T2></IndirFoo>`,
		Value:         &IndirAny{Any: stringptr("hi")},
		UnmarshalOnly: true,
	},
	{
		ExpectXML:     `<IndirFoo><T1></T1><Foo></Foo><T2></T2></IndirFoo>`,
		Value:         &IndirAny{Any: stringptr("")},
		UnmarshalOnly: true,
	},
	{
		ExpectXML:     `<IndirFoo><T1></T1><T2></T2></IndirFoo>`,
		Value:         &IndirAny{Any: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML:     `<IfaceFoo><T1></T1><Foo>hi</Foo><T2></T2></IfaceFoo>`,
		Value:         &IfaceAny{Any: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML:     `<IfaceFoo><T1></T1><T2></T2></IfaceFoo>`,
		Value:         &IfaceAny{Any: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML:     `<IfaceFoo><T1></T1><T2></T2></IfaceFoo>`,
		Value:         &IfaceAny{Any: nil},
		UnmarshalOnly: true,
	},
	{
		ExpectXML:     `<DirectFoo><T1></T1><Foo>hi</Foo><T2></T2></DirectFoo>`,
		Value:         &DirectAny{Any: string("hi")},
		UnmarshalOnly: true,
	},
	{
		ExpectXML:     `<DirectFoo><T1></T1><Foo></Foo><T2></T2></DirectFoo>`,
		Value:         &DirectAny{Any: string("")},
		UnmarshalOnly: true,
	},
}

// @see
func TestMarshal(t *testing.T) {
	for idx, test := range marshalTests {
		if test.UnmarshalOnly {
			continue
		}

		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			data, err := Marshal(test.Value)
			if err != nil {
				if test.MarshalError == "" {
					t.Errorf("marshal(%#v): %s", test.Value, err)
					return
				}
				if !strings.Contains(err.Error(), test.MarshalError) {
					t.Errorf("marshal(%#v): %s, want %q", test.Value, err, test.MarshalError)
				}
				return
			}
			if test.MarshalError != "" {
				t.Errorf("Marshal succeeded, want error %q", test.MarshalError)
				return
			}
			if got, want := string(data), test.ExpectXML; got != want {
				if strings.Contains(want, "\n") {
					t.Errorf("marshal(%#v):\nHAVE:\n%s\nWANT:\n%s", test.Value, got, want)
				} else {
					t.Errorf("marshal(%#v):\nhave %#q\nwant %#q", test.Value, got, want)
				}
			}
		})
	}
}

type AttrParent struct {
	X string `xml:"X>Y,attr"`
}

type BadAttr struct {
	Name map[string]string `xml:"name,attr"`
}

var marshalErrorTests = []struct {
	Value interface{}
	Err   string
	Kind  reflect.Kind
}{
	{
		Value: make(chan bool),
		Err:   "xml: unsupported type: chan bool",
		Kind:  reflect.Chan,
	},
	{
		Value: map[string]string{
			"question": "What do you get when you multiply six by nine?",
			"answer":   "42",
		},
		Err:  "xml: unsupported type: map[string]string",
		Kind: reflect.Map,
	},
	{
		Value: map[*Ship]bool{nil: false},
		Err:   "xml: unsupported type: map[*xml.Ship]bool",
		Kind:  reflect.Map,
	},
	{
		Value: &Domain{Comment: []byte("f--bar")},
		Err:   `xml: comments must not contain "--"`,
	},
	// Reject parent chain with attr, never worked; see golang.org/issue/5033.
	{
		Value: &AttrParent{},
		Err:   `xml: X>Y chain not valid with attr flag`,
	},
	{
		Value: BadAttr{map[string]string{"X": "Y"}},
		Err:   `xml: unsupported type: map[string]string`,
	},
}

var marshalIndentTests = []struct {
	Value     interface{}
	Prefix    string
	Indent    string
	ExpectXML string
}{
	{
		Value: &SecretAgent{
			Handle:    "007",
			Identity:  "James Bond",
			Obfuscate: "<redacted/>",
		},
		Prefix:    "",
		Indent:    "\t",
		ExpectXML: fmt.Sprintf("<agent handle=\"007\">\n\t<Identity>James Bond</Identity><redacted/>\n</agent>"),
	},
}

func TestMarshalErrors(t *testing.T) {
	for idx, test := range marshalErrorTests {
		data, err := Marshal(test.Value)
		if err == nil {
			t.Errorf("#%d: marshal(%#v) = [success] %q, want error %v", idx, test.Value, data, test.Err)
			continue
		}
		if err.Error() != test.Err {
			t.Errorf("#%d: marshal(%#v) = [error] %v, want %v", idx, test.Value, err, test.Err)
		}
		if test.Kind != reflect.Invalid {
			if kind := err.(*UnsupportedTypeError).Type.Kind(); kind != test.Kind {
				t.Errorf("#%d: marshal(%#v) = [error kind] %s, want %s", idx, test.Value, kind, test.Kind)
			}
		}
	}
}

// Do invertibility testing on the various structures that we test
//
// @see
func TestUnmarshal(t *testing.T) {
	for i, test := range marshalTests {
		if test.MarshalOnly {
			continue
		}
		if _, ok := test.Value.(*Plain); ok {
			continue
		}
		if test.ExpectXML == `<top>`+
			`<x><b xmlns="space">b</b>`+
			`<b xmlns="space1">b1</b></x>`+
			`</top>` {
			// TODO(rogpeppe): re-enable this test in
			// https://go-review.googlesource.com/#/c/5910/
			continue
		}

		vt := reflect.TypeOf(test.Value)
		dest := reflect.New(vt.Elem()).Interface()
		err := Unmarshal([]byte(test.ExpectXML), dest)

		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			switch fix := dest.(type) {
			case *Feed:
				fix.Author.InnerXML = ""
				for i := range fix.Entry {
					fix.Entry[i].Author.InnerXML = ""
				}
			}

			if err != nil {
				if test.UnmarshalError == "" {
					t.Errorf("unmarshal(%#v): %s", test.ExpectXML, err)
					return
				}
				if !strings.Contains(err.Error(), test.UnmarshalError) {
					t.Errorf("unmarshal(%#v): %s, want %q", test.ExpectXML, err, test.UnmarshalError)
				}
				return
			}
			// 注: 对于复杂的结构,在测试时可以使用 reflect.DeepEqual 来判断是否相等.
			if got, want := dest, test.Value; !reflect.DeepEqual(got, want) {
				t.Errorf("unmarshal(%q):\nhave %#v\nwant %#v", test.ExpectXML, got, want)
			}
		})
	}
}

func TestMarshalIndent(t *testing.T) {
	for i, test := range marshalIndentTests {
		data, err := MarshalIndent(test.Value, test.Prefix, test.Indent)
		if err != nil {
			t.Errorf("#%d: Error: %s", i, err)
			continue
		}
		if got, want := string(data), test.ExpectXML; got != want {
			t.Errorf("#%d: MarshalIndent:\nGot:%s\nWant:\n%s", i, got, want)
		}
	}
}

type limitedBytesWriter struct {
	w      io.Writer
	remain int // until writes fail
}

func (lw *limitedBytesWriter) Write(p []byte) (n int, err error) {
	if lw.remain <= 0 {
		println("error")
		return 0, errors.New("write limit hit")
	}
	if len(p) > lw.remain {
		p = p[:lw.remain]
		n, _ = lw.w.Write(p)
		lw.remain = 0
		return n, errors.New("write limit hit")
	}
	n, err = lw.w.Write(p)
	lw.remain -= n
	return n, err
}

func TestMarshalWriteErrors(t *testing.T) {
	var buf bytes.Buffer
	const writeCap = 1024
	w := &limitedBytesWriter{&buf, writeCap}
	enc := NewEncoder(w)
	var err error
	var i int
	const n = 4000
	for i = 1; i <= n; i++ {
		err = enc.Encode(&Passenger{
			Name:   []string{"Alice", "Bob"},
			Weight: 5,
		})
		if err != nil {
			break
		}
	}
	if err == nil {
		t.Error("expected an error")
	}
	if i == n {
		t.Errorf("expected to fail before the end")
	}
	if buf.Len() != writeCap {
		t.Errorf("buf.Len() = %d; want %d", buf.Len(), writeCap)
	}
}

func TestMarshalWriteIOErrors(t *testing.T) {
	enc := NewEncoder(errWriter{})

	expectErr := "unwritable"
	err := enc.Encode(&Passenger{})
	if err == nil || err.Error() != expectErr {
		t.Errorf("EscapeTest = [error] %v, want %v", err, expectErr)
	}
}

func TestMarshalFlush(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.EncodeToken(CharData("hello world")); err != nil {
		t.Fatalf("enc.EncodeToken: %v", err)
	}
	if buf.Len() > 0 {
		t.Fatalf("enc.EncodeToken caused actual write: %q", buf.Bytes())
	}
	if err := enc.Flush(); err != nil {
		t.Fatalf("enc.Flush: %v", err)
	}
	if buf.String() != "hello world" {
		t.Fatalf("after enc.Flush, buf.String() = %q, want %q", buf.String(), "hello world")
	}
}

func BenchmarkMarshal(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Marshal(atomValue)
		}
	})
}

func BenchmarkUnmarshal(b *testing.B) {
	b.ReportAllocs()
	xml := []byte(atomXML)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Unmarshal(xml, &Feed{})
		}
	})
}

// golang.org/issue/6556
func TestStructPointerMarshal(t *testing.T) {
	type A struct {
		XMLName string `xml:"a"`
		B       []interface{}
	}
	type C struct {
		XMLName Name
		Value   string `xml:"value"`
	}

	a := new(A)
	a.B = append(a.B, &C{
		XMLName: Name{Local: "c"},
		Value:   "x",
	})

	b, err := Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if x := string(b); x != "<a><c><value>x</value></c></a>" {
		t.Fatal(x)
	}
	var v A
	err = Unmarshal(b, &v)
	if err != nil {
		t.Fatal(err)
	}
}

var encodeTokenTests = []struct {
	desc string
	toks []Token
	want string
	err  string
}{{
	desc: "start element with name space",
	toks: []Token{
		StartElement{Name{"space", "local"}, nil},
	},
	want: `<local xmlns="space">`,
}, {
	desc: "start element with no name",
	toks: []Token{
		StartElement{Name{"space", ""}, nil},
	},
	err: "xml: start tag with no name",
}, {
	desc: "end element with no name",
	toks: []Token{
		EndElement{Name{"space", ""}},
	},
	err: "xml: end tag with no name",
}, {
	desc: "char data",
	toks: []Token{
		CharData("foo"),
	},
	want: `foo`,
}, {
	desc: "char data with escaped chars",
	toks: []Token{
		CharData(" \t\n"),
	},
	want: " &#x9;\n",
}, {
	desc: "comment",
	toks: []Token{
		Comment("foo"),
	},
	want: `<!--foo-->`,
}, {
	desc: "comment with invalid content",
	toks: []Token{
		Comment("foo-->"),
	},
	err: "xml: EncodeToken of Comment containing --> marker",
}, {
	desc: "proc instruction",
	toks: []Token{
		ProcInst{"Target", []byte("Instruction")},
	},
	want: `<?Target Instruction?>`,
}, {
	desc: "proc instruction with empty target",
	toks: []Token{
		ProcInst{"", []byte("Instruction")},
	},
	err: "xml: EncodeToken of ProcInst with invalid Target",
}, {
	desc: "proc instruction with bad content",
	toks: []Token{
		ProcInst{"", []byte("Instruction?>")},
	},
	err: "xml: EncodeToken of ProcInst with invalid Target",
}, {
	desc: "directive",
	toks: []Token{
		Directive("foo"),
	},
	want: `<!foo>`,
}, {
	desc: "more complex directive",
	toks: []Token{
		Directive("DOCTYPE doc [ <!ELEMENT doc '>'> <!-- com>ment --> ]"),
	},
	want: `<!DOCTYPE doc [ <!ELEMENT doc '>'> <!-- com>ment --> ]>`,
}, {
	desc: "directive instruction with bad name",
	toks: []Token{
		Directive("foo>"),
	},
	err: "xml: EncodeToken of Directive containing wrong < or > markers",
}, {
	desc: "end tag without start tag",
	toks: []Token{
		EndElement{Name{"foo", "bar"}},
	},
	err: "xml: end tag </bar> without start tag",
}, {
	desc: "mismatching end tag local name",
	toks: []Token{
		StartElement{Name{"", "foo"}, nil},
		EndElement{Name{"", "bar"}},
	},
	err:  "xml: end tag </bar> does not match start tag <foo>",
	want: `<foo>`,
}, {
	desc: "mismatching end tag namespace",
	toks: []Token{
		StartElement{Name{"space", "foo"}, nil},
		EndElement{Name{"another", "foo"}},
	},
	err:  "xml: end tag </foo> in namespace another does not match start tag <foo> in namespace space",
	want: `<foo xmlns="space">`,
}, {
	desc: "start element with explicit namespace",
	toks: []Token{
		StartElement{Name{"space", "local"}, []Attr{
			{Name{"xmlns", "x"}, "space"},
			{Name{"space", "foo"}, "value"},
		}},
	},
	want: `<local xmlns="space" xmlns:_xmlns="xmlns" _xmlns:x="space" xmlns:space="space" space:foo="value">`,
}, {
	desc: "start element with explicit namespace and colliding prefix",
	toks: []Token{
		StartElement{Name{"space", "local"}, []Attr{
			{Name{"xmlns", "x"}, "space"},
			{Name{"space", "foo"}, "value"},
			{Name{"x", "bar"}, "other"},
		}},
	},
	want: `<local xmlns="space" xmlns:_xmlns="xmlns" _xmlns:x="space" xmlns:space="space" space:foo="value" xmlns:x="x" x:bar="other">`,
}, {
	desc: "start element using previously defined namespace",
	toks: []Token{
		StartElement{Name{"", "local"}, []Attr{
			{Name{"xmlns", "x"}, "space"},
		}},
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"space", "x"}, "y"},
		}},
	},
	want: `<local xmlns:_xmlns="xmlns" _xmlns:x="space"><foo xmlns="space" xmlns:space="space" space:x="y">`,
}, {
	desc: "nested name space with same prefix",
	toks: []Token{
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"xmlns", "x"}, "space1"},
		}},
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"xmlns", "x"}, "space2"},
		}},
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"space1", "a"}, "space1 value"},
			{Name{"space2", "b"}, "space2 value"},
		}},
		EndElement{Name{"", "foo"}},
		EndElement{Name{"", "foo"}},
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"space1", "a"}, "space1 value"},
			{Name{"space2", "b"}, "space2 value"},
		}},
	},
	want: `<foo xmlns:_xmlns="xmlns" _xmlns:x="space1"><foo _xmlns:x="space2"><foo xmlns:space1="space1" space1:a="space1 value" xmlns:space2="space2" space2:b="space2 value"></foo></foo><foo xmlns:space1="space1" space1:a="space1 value" xmlns:space2="space2" space2:b="space2 value">`,
}, {
	desc: "start element defining several prefixes for the same name space",
	toks: []Token{
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"xmlns", "a"}, "space"},
			{Name{"xmlns", "b"}, "space"},
			{Name{"space", "x"}, "value"},
		}},
	},
	want: `<foo xmlns="space" xmlns:_xmlns="xmlns" _xmlns:a="space" _xmlns:b="space" xmlns:space="space" space:x="value">`,
}, {
	desc: "nested element redefines name space",
	toks: []Token{
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"xmlns", "x"}, "space"},
		}},
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"xmlns", "y"}, "space"},
			{Name{"space", "a"}, "value"},
		}},
	},
	want: `<foo xmlns:_xmlns="xmlns" _xmlns:x="space"><foo xmlns="space" _xmlns:y="space" xmlns:space="space" space:a="value">`,
}, {
	desc: "nested element creates alias for default name space",
	toks: []Token{
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"", "xmlns"}, "space"},
		}},
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"xmlns", "y"}, "space"},
			{Name{"space", "a"}, "value"},
		}},
	},
	want: `<foo xmlns="space" xmlns="space"><foo xmlns="space" xmlns:_xmlns="xmlns" _xmlns:y="space" xmlns:space="space" space:a="value">`,
}, {
	desc: "nested element defines default name space with existing prefix",
	toks: []Token{
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"xmlns", "x"}, "space"},
		}},
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"", "xmlns"}, "space"},
			{Name{"space", "a"}, "value"},
		}},
	},
	want: `<foo xmlns:_xmlns="xmlns" _xmlns:x="space"><foo xmlns="space" xmlns="space" xmlns:space="space" space:a="value">`,
}, {
	desc: "nested element uses empty attribute name space when default ns defined",
	toks: []Token{
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"", "xmlns"}, "space"},
		}},
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"", "attr"}, "value"},
		}},
	},
	want: `<foo xmlns="space" xmlns="space"><foo xmlns="space" attr="value">`,
}, {
	desc: "redefine xmlns",
	toks: []Token{
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"foo", "xmlns"}, "space"},
		}},
	},
	want: `<foo xmlns:foo="foo" foo:xmlns="space">`,
}, {
	desc: "xmlns with explicit name space #1",
	toks: []Token{
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"xml", "xmlns"}, "space"},
		}},
	},
	want: `<foo xmlns="space" xmlns:_xml="xml" _xml:xmlns="space">`,
}, {
	desc: "xmlns with explicit name space #2",
	toks: []Token{
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{xmlURL, "xmlns"}, "space"},
		}},
	},
	want: `<foo xmlns="space" xml:xmlns="space">`,
}, {
	desc: "empty name space declaration is ignored",
	toks: []Token{
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"xmlns", "foo"}, ""},
		}},
	},
	want: `<foo xmlns:_xmlns="xmlns" _xmlns:foo="">`,
}, {
	desc: "attribute with no name is ignored",
	toks: []Token{
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"", ""}, "value"},
		}},
	},
	want: `<foo>`,
}, {
	desc: "namespace URL with non-valid name",
	toks: []Token{
		StartElement{Name{"/34", "foo"}, []Attr{
			{Name{"/34", "x"}, "value"},
		}},
	},
	want: `<foo xmlns="/34" xmlns:_="/34" _:x="value">`,
}, {
	desc: "nested element resets default namespace to empty",
	toks: []Token{
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"", "xmlns"}, "space"},
		}},
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"", "xmlns"}, ""},
			{Name{"", "x"}, "value"},
			{Name{"space", "x"}, "value"},
		}},
	},
	want: `<foo xmlns="space" xmlns="space"><foo xmlns="" x="value" xmlns:space="space" space:x="value">`,
}, {
	desc: "nested element requires empty default name space",
	toks: []Token{
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"", "xmlns"}, "space"},
		}},
		StartElement{Name{"", "foo"}, nil},
	},
	want: `<foo xmlns="space" xmlns="space"><foo>`,
}, {
	desc: "attribute uses name space from xmlns",
	toks: []Token{
		StartElement{Name{"some/space", "foo"}, []Attr{
			{Name{"", "attr"}, "value"},
			{Name{"some/space", "other"}, "other value"},
		}},
	},
	want: `<foo xmlns="some/space" attr="value" xmlns:space="some/space" space:other="other value">`,
}, {
	desc: "default name space should not be used by attributes",
	toks: []Token{
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"", "xmlns"}, "space"},
			{Name{"xmlns", "bar"}, "space"},
			{Name{"space", "baz"}, "foo"},
		}},
		StartElement{Name{"space", "baz"}, nil},
		EndElement{Name{"space", "baz"}},
		EndElement{Name{"space", "foo"}},
	},
	want: `<foo xmlns="space" xmlns="space" xmlns:_xmlns="xmlns" _xmlns:bar="space" xmlns:space="space" space:baz="foo"><baz xmlns="space"></baz></foo>`,
}, {
	desc: "default name space not used by attributes, not explicitly defined",
	toks: []Token{
		StartElement{Name{"space", "foo"}, []Attr{
			{Name{"", "xmlns"}, "space"},
			{Name{"space", "baz"}, "foo"},
		}},
		StartElement{Name{"space", "baz"}, nil},
		EndElement{Name{"space", "baz"}},
		EndElement{Name{"space", "foo"}},
	},
	want: `<foo xmlns="space" xmlns="space" xmlns:space="space" space:baz="foo"><baz xmlns="space"></baz></foo>`,
}, {
	desc: "impossible xmlns declaration",
	toks: []Token{
		StartElement{Name{"", "foo"}, []Attr{
			{Name{"", "xmlns"}, "space"},
		}},
		StartElement{Name{"space", "bar"}, []Attr{
			{Name{"space", "attr"}, "value"},
		}},
	},
	want: `<foo xmlns="space"><bar xmlns="space" xmlns:space="space" space:attr="value">`,
}}

func TestEncodeToken(t *testing.T) {
loop:
	for i, tt := range encodeTokenTests {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		var err error
		for j, tok := range tt.toks {
			err = enc.EncodeToken(tok)
			if err != nil && j < len(tt.toks)-1 {
				t.Errorf("#%d %s token #%d: %v", i, tt.desc, j, err)
				continue loop
			}
		}
		errorf := func(f string, a ...interface{}) {
			t.Errorf("#%d %s token #%d:%s", i, tt.desc, len(tt.toks)-1, fmt.Sprintf(f, a...))
		}
		switch {
		case tt.err != "" && err == nil:
			errorf(" expected error; got none")
			continue
		case tt.err == "" && err != nil:
			errorf(" got error: %v", err)
			continue
		case tt.err != "" && err != nil && tt.err != err.Error():
			errorf(" error mismatch; got %v, want %v", err, tt.err)
			continue
		}
		if err := enc.Flush(); err != nil {
			errorf(" %v", err)
			continue
		}
		if got := buf.String(); got != tt.want {
			errorf("\ngot  %v\nwant %v", got, tt.want)
			continue
		}
	}
}

func TestProcInstEncodeToken(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	if err := enc.EncodeToken(ProcInst{"xml", []byte("Instruction")}); err != nil {
		t.Fatalf("enc.EncodeToken: expected to be able to encode xml target ProcInst as first token, %s", err)
	}

	if err := enc.EncodeToken(ProcInst{"Target", []byte("Instruction")}); err != nil {
		t.Fatalf("enc.EncodeToken: expected to be able to add non-xml target ProcInst")
	}

	if err := enc.EncodeToken(ProcInst{"xml", []byte("Instruction")}); err == nil {
		t.Fatalf("enc.EncodeToken: expected to not be allowed to encode xml target ProcInst when not first token")
	}
}

func TestDecodeEncode(t *testing.T) {
	var in, out bytes.Buffer
	in.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<?Target Instruction?>
<root>
</root>
`)
	dec := NewDecoder(&in)
	enc := NewEncoder(&out)
	for tok, err := dec.Token(); err == nil; tok, err = dec.Token() {
		err = enc.EncodeToken(tok)
		if err != nil {
			t.Fatalf("enc.EncodeToken: Unable to encode token (%#v), %v", tok, err)
		}
	}
}

// Issue 9796. Used to fail with GORACE="halt_on_error=1" -race.
func TestRace9796(t *testing.T) {
	type A struct{}
	type B struct {
		C []A `xml:"X>Y"`
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			Marshal(B{[]A{{}}})
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestIsValidDirective(t *testing.T) {
	testOK := []string{
		"<>",
		"< < > >",
		"<!DOCTYPE '<' '>' '>' <!--nothing-->>",
		"<!DOCTYPE doc [ <!ELEMENT doc ANY> <!ELEMENT doc ANY> ]>",
		"<!DOCTYPE doc [ <!ELEMENT doc \"ANY> '<' <!E\" LEMENT '>' doc ANY> ]>",
		"<!DOCTYPE doc <!-- just>>>> a < comment --> [ <!ITEM anything> ] >",
	}
	testKO := []string{
		"<",
		">",
		"<!--",
		"-->",
		"< > > < < >",
		"<!dummy <!-- > -->",
		"<!DOCTYPE doc '>",
		"<!DOCTYPE doc '>'",
		"<!DOCTYPE doc <!--comment>",
	}
	for _, s := range testOK {
		if !isValidDirective(Directive(s)) {
			t.Errorf("Directive %q is expected to be valid", s)
		}
	}
	for _, s := range testKO {
		if isValidDirective(Directive(s)) {
			t.Errorf("Directive %q is expected to be invalid", s)
		}
	}
}

// Issue 11719. EncodeToken used to silently eat tokens with an invalid type.
func TestSimpleUseOfEncodeToken(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.EncodeToken(&StartElement{Name: Name{"", "object1"}}); err == nil {
		t.Errorf("enc.EncodeToken: pointer type should be rejected")
	}
	if err := enc.EncodeToken(&EndElement{Name: Name{"", "object1"}}); err == nil {
		t.Errorf("enc.EncodeToken: pointer type should be rejected")
	}
	if err := enc.EncodeToken(StartElement{Name: Name{"", "object2"}}); err != nil {
		t.Errorf("enc.EncodeToken: StartElement %s", err)
	}
	if err := enc.EncodeToken(EndElement{Name: Name{"", "object2"}}); err != nil {
		t.Errorf("enc.EncodeToken: EndElement %s", err)
	}
	if err := enc.EncodeToken(Universe{}); err == nil {
		t.Errorf("enc.EncodeToken: invalid type not caught")
	}
	if err := enc.Flush(); err != nil {
		t.Errorf("enc.Flush: %s", err)
	}
	if buf.Len() == 0 {
		t.Errorf("enc.EncodeToken: empty buffer")
	}
	want := "<object2></object2>"
	if buf.String() != want {
		t.Errorf("enc.EncodeToken: expected %q; got %q", want, buf.String())
	}
}

// Issue 16158. Decoder.unmarshalAttr ignores the return value of copyValue.
func TestIssue16158(t *testing.T) {
	const data = `<foo b="HELLOWORLD"></foo>`
	err := Unmarshal([]byte(data), &struct {
		B byte `xml:"b,attr,omitempty"`
	}{})
	if err == nil {
		t.Errorf("Unmarshal: expected error, got nil")
	}
}
