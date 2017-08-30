// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[3-over]]] 2017-6-13 09:03:38

package sort_test

import (
	"fmt"
	"sort"
)

/**
mass [mæs] n.
1.(聚成一体的)团，块，堆，群，片
2.大量，众多，大宗
3.体积，容积，大小，量
4.大多数，大部分，主要部分，主体
5.【绘画】 同色(或阴影部分的)扩散形状，一大片，块面
6.【药剂学】 制药丸的浆状物，炼药，丸块
7.【物理学】 质量
 */

/**
earth [ɜːθ]
n. 地球；地表，陆地；土地，土壤；尘事，俗事；兽穴
vt. 把（电线）[电] 接地；盖（土）；追赶入洞穴
vi. 躲进地洞
 */

// A couple of type definitions to make the units clear.
type earthMass float64
type au float64

/**
solar energy太阳能
solar system太阳系
solar power太阳能；太阳能动力
 */

// A Planet defines the properties of a solar system object.
type Planet struct {
	// 星球名
	name     string
	// 质量
	mass     earthMass
	distance au
}

// By is the type of a "less" function that defines the ordering of its Planet arguments.
// 下面的代码中,会将星球的各种参数作为比较依据,因此大小判断依据是不固定的
// 这里通过定义一个函数类型来进行Less方法的变化
type By func(p1, p2 *Planet) bool

// Sort is a method on the function type, By, that sorts the argument slice according to the function.
// 这是一个定义在函数上的方法.
func (by By) Sort(planets []Planet) {
	ps := &planetSorter{
		planets: planets,
		by:      by, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(ps)
}

// planetSorter joins a By function and a slice of Planets to be sorted.
// planetSorter实现了sort.Interface接口
type planetSorter struct {
	planets []Planet
	by      func(p1, p2 *Planet) bool // Closure used in the Less method.
}

// Len is part of sort.Interface.
func (s *planetSorter) Len() int {
	return len(s.planets)
}

// Swap is part of sort.Interface.
func (s *planetSorter) Swap(i, j int) {
	s.planets[i], s.planets[j] = s.planets[j], s.planets[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *planetSorter) Less(i, j int) bool {
	return s.by(&s.planets[i], &s.planets[j])
}

var planets = []Planet{
	{"Mercury", 0.055, 0.4},
	{"Venus", 0.815, 0.7},
	{"Earth", 1.0, 1.0},
	{"Mars", 0.107, 1.5},
}

// ExampleSortKeys demonstrates a technique for sorting a struct type using programmable sort criteria.
func Example_sortKeys() {
	// Closures that order the Planet structure.
	// 按照 name 字段排序
	name := func(p1, p2 *Planet) bool {
		return p1.name < p2.name
	}
	// 按照 mass 字段排序
	mass := func(p1, p2 *Planet) bool {
		return p1.mass < p2.mass
	}
	// 按照 distance 字段排序
	distance := func(p1, p2 *Planet) bool {
		return p1.distance < p2.distance
	}
	// 按照 distance 字段 降序 排序
	decreasingDistance := func(p1, p2 *Planet) bool {
		return distance(p2, p1)
	}

	// Sort the planets by the various criteria.
	// By(name): 这是在进行类型转换,类型转换之后的值才实现了 sort.Interface 接口
	By(name).Sort(planets)
	fmt.Println("By name:", planets)

	// By(mass): 这是在进行类型转换,类型转换之后的值才实现了 sort.Interface 接口
	By(mass).Sort(planets)
	fmt.Println("By mass:", planets)

	// By(distance): 这是在进行类型转换,类型转换之后的值才实现了 sort.Interface 接口
	By(distance).Sort(planets)
	fmt.Println("By distance:", planets)

	// By(decreasingDistance): 这是在进行类型转换,类型转换之后的值才实现了 sort.Interface 接口
	By(decreasingDistance).Sort(planets)
	fmt.Println("By decreasing distance:", planets)

	// Output: By name: [{Earth 1 1} {Mars 0.107 1.5} {Mercury 0.055 0.4} {Venus 0.815 0.7}]
	// By mass: [{Mercury 0.055 0.4} {Mars 0.107 1.5} {Venus 0.815 0.7} {Earth 1 1}]
	// By distance: [{Mercury 0.055 0.4} {Venus 0.815 0.7} {Earth 1 1} {Mars 0.107 1.5}]
	// By decreasing distance: [{Mars 0.107 1.5} {Earth 1 1} {Venus 0.815 0.7} {Mercury 0.055 0.4}]
}
