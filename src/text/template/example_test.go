// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[4-over]]] 2017-6-16 11:05:19

package template_test

import (
	"log"
	"os"
	"strings"
	"text/template"
)

func ExampleTemplate() {
	// Define a template.
	// attend [ə'tend] vt. 出席；上（大学等）；照料；招待；陪伴 vi. 出席；致力于；照料；照顾 
	// wedding ['wedɪŋ] n. 婚礼，婚宴；结婚；结合 v. 与…结婚（wed的ing形式）
	const letter = `
Dear {{.Name}},
{{if .Attended}}
It was a pleasure to see you at the wedding.
{{- else}}
It is a shame you couldn't make it to the wedding.
{{- end}}
{{with .Gift -}}
Thank you for the lovely {{.}}.
{{end}}
Best wishes,
Josie
`

	// Prepare some data to insert into the template.
	// recipient [rɪ'sɪpɪənt] n. 容器，接受者；容纳者 adj. 容易接受的，感受性强的
	type Recipient struct {
		Name, Gift string
		Attended   bool
	}
	var recipients = []Recipient{
		{"Aunt Mildred", "bone china tea set", true},
		{"Uncle John", "moleskin pants", false},
		// 由于 Gift 是空字符串, 因此 with 部分没有输出
		{"Cousin Rodney", "", false},
	}

	// Create a new template and parse the letter into it.
	t := template.Must(template.New("letter").Parse(letter))

	// Execute the template for each recipient.
	for _, r := range recipients {
		err := t.Execute(os.Stdout, r)
		if err != nil {
			log.Println("executing template:", err)
		}
	}

	// Output:
	// Dear Aunt Mildred,
	//
	// It was a pleasure to see you at the wedding.
	// Thank you for the lovely bone china tea set.
	//
	// Best wishes,
	// Josie
	//
	// Dear Uncle John,
	//
	// It is a shame you couldn't make it to the wedding.
	// Thank you for the lovely moleskin pants.
	//
	// Best wishes,
	// Josie
	//
	// Dear Cousin Rodney,
	//
	// It is a shame you couldn't make it to the wedding.
	//
	// Best wishes,
	// Josie
}

// The following example is duplicated in html/template; keep them in sync.

func ExampleTemplate_block() {
	// master 中通过 block 定义了 template list 并立即执行
	/**
	什么是block?
	{{block "name" pipeline}} T1 {{end}}
	A block is shorthand for defining a template
		{{define "name"}} T1 {{end}}
	and then executing it in place
		{{template "name" .}}
	The typical use is to define a set of root templates that are
	then customized by redefining the block templates within.
	 */
	const (
		master  = `Names:{{block "list" .}}{{"\n"}}{{range .}}{{println "-" .}}{{end}}{{end}}`
		// overlay [əʊvə'leɪ] n. 覆盖图；覆盖物 vt. 在表面上铺一薄层，镀
		// overlay 中通过重新定义 template list, 覆盖掉了 master 中的 list
		overlay = `{{define "list"}} {{join . ", "}}{{end}} `
	)
	var (
		funcs     = template.FuncMap{"join": strings.Join}
		// guardian ['gɑːdɪən] n. [法] 监护人，保护人；守护者 adj. 守护的
		guardians = []string{"Gamora", "Groot", "Nebula", "Rocket", "Star-Lord"}
	)
	masterTmpl, err := template.New("master").Funcs(funcs).Parse(master)
	if err != nil {
		log.Fatal(err)
	}
	// 注意: masterTmpl.Clone() 克隆出来的模板也拥有之前.Funcs(funcs)添加的函数
	// 在 masterClone 中 parse overlay
	overlayTmpl, err := template.Must(masterTmpl.Clone()).Parse(overlay)
	if err != nil {
		log.Fatal(err)
	}
	if err := masterTmpl.Execute(os.Stdout, guardians); err != nil {
		log.Fatal(err)
	}
	if err := overlayTmpl.Execute(os.Stdout, guardians); err != nil {
		log.Fatal(err)
	}
	// Output:
	// Names:
	// - Gamora
	// - Groot
	// - Nebula
	// - Rocket
	// - Star-Lord
	// Names: Gamora, Groot, Nebula, Rocket, Star-Lord
}
