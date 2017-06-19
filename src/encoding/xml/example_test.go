// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xml_test

import (
	"encoding/xml"
	"fmt"
	"os"
)

func ExampleMarshalIndent() {
	type Address struct {
		City, State string
	}
	type Person struct {
		XMLName   xml.Name `xml:"person"`
		Id        int      `xml:"id,attr"`
		FirstName string   `xml:"name>first"`
		LastName  string   `xml:"name>last"`
		Age       int      `xml:"age"`
		// 注:最终的输出被忽略,因为有omitempty
		Height    float32  `xml:"height,omitempty"`
		Married   bool
		Address
		Comment string `xml:",comment"`
	}

	v := &Person{Id: 13, FirstName: "John", LastName: "Doe", Age: 42}
	v.Comment = " Need more details. "
	// 注:尽管 Address 是匿名字段,最终 City 和 State 是在外层
	v.Address = Address{"Hanga Roa", "Easter Island"}

	output, err := xml.MarshalIndent(v, "  ", "    ")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}

	os.Stdout.Write(output)
	// Output:
	//   <person id="13">
	//       <name>
	//           <first>John</first>
	//           <last>Doe</last>
	//       </name>
	//       <age>42</age>
	//       <Married>false</Married>
	//       <City>Hanga Roa</City>
	//       <State>Easter Island</State>
	//       <!-- Need more details. -->
	//   </person>
}

func ExampleEncoder() {
	type Address struct {
		City, State string
	}
	type Person struct {
		XMLName   xml.Name `xml:"person"`
		Id        int      `xml:"id,attr"`
		FirstName string   `xml:"name>first"`
		LastName  string   `xml:"name>last"`
		Age       int      `xml:"age"`
		Height    float32  `xml:"height,omitempty"`
		Married   bool
		Address
		Comment string `xml:",comment"`
	}

	v := &Person{Id: 13, FirstName: "John", LastName: "Doe", Age: 42}
	v.Comment = " Need more details. "
	v.Address = Address{"Hanga Roa", "Easter Island"}

	enc := xml.NewEncoder(os.Stdout)
	enc.Indent("  ", "    ")
	if err := enc.Encode(v); err != nil {
		fmt.Printf("error: %v\n", err)
	}

	// Output:
	//   <person id="13">
	//       <name>
	//           <first>John</first>
	//           <last>Doe</last>
	//       </name>
	//       <age>42</age>
	//       <Married>false</Married>
	//       <City>Hanga Roa</City>
	//       <State>Easter Island</State>
	//       <!-- Need more details. -->
	//   </person>
}

// This example demonstrates unmarshaling an XML excerpt into a value with
// some preset fields. Note that the Phone field isn't modified and that
// the XML <Company> element is ignored. Also, the Groups field is assigned
// considering the element path provided in its tag.
//
// 注: preset(预先设置的)
// 注: excerpt ['eksɜːpt] n. 摘录，引用 vt. 引用，摘录 vi. 摘录，引用
func ExampleUnmarshal() {
	type Email struct {
		Where string `xml:"where,attr"`
		Addr  string
	}
	type Address struct {
		City, State string
	}
	type Result struct {
		XMLName xml.Name `xml:"Person"`
		Name    string   `xml:"FullName"`
		// 注: Note that the Phone field isn't modified
		Phone   string
		Email   []Email
		// 注: the Groups field is assigned considering the element path provided in its tag.
		Groups  []string `xml:"Group>Value"`
		Address
	}
	// 注: Name字段最后有变化,Phone字段最后没有变化
	v := Result{Name: "none", Phone: "none"}

	// 注: Note that the XML <Company> element is ignored; 由于声明的结构体Result中没有Company的对应字段
	data := `
		<Person>
			<FullName>Grace R. Emlin</FullName>
			<Company>Example Inc.</Company>
			<Email where="home">
				<Addr>gre@example.com</Addr>
			</Email>
			<Email where='work'>
				<Addr>gre@work.com</Addr>
			</Email>
			<Group>
				<Value>Friends</Value>
				<Value>Squash</Value>
			</Group>
			<City>Hanga Roa</City>
			<State>Easter Island</State>
		</Person>
	`
	err := xml.Unmarshal([]byte(data), &v)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	fmt.Printf("XMLName: %#v\n", v.XMLName)
	fmt.Printf("Name: %q\n", v.Name)
	fmt.Printf("Phone: %q\n", v.Phone)
	fmt.Printf("Email: %v\n", v.Email)
	fmt.Printf("Groups: %v\n", v.Groups)
	fmt.Printf("Address: %v\n", v.Address)
	// Output:
	// XMLName: xml.Name{Space:"", Local:"Person"}
	// Name: "Grace R. Emlin"
	// Phone: "none"
	// Email: [{home gre@example.com} {work gre@work.com}]
	// Groups: [Friends Squash]
	// Address: {Hanga Roa Easter Island}
}
