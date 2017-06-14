// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [[[5-over]]] 2017-6-14 09:20:04

package driver

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// ValueConverter is the interface providing the ConvertValue method.
//
// Various implementations of ValueConverter are provided by the
// driver package to provide consistent implementations of conversions
// between drivers. The ValueConverters have several uses:
//
//  * converting from the Value types as provided by the sql package
//    into a database table's specific column type and making sure it
//    fits, such as making sure a particular int64 fits in a
//    table's uint16 column.
//
//    就是: driver's Value=>db原始数据
//
//  * converting a value as given from the database into one of the
//    driver Value types.
//
//    就是: db原始数据=>driver's Value
//
//  * by the sql package, for converting from a driver's Value type
//    to a user's type in a scan.
//
//    就是: Scan 的时候将 driver's Value => a user's type
type ValueConverter interface {
	// ConvertValue converts a value to a driver Value.
	// 将v转换为driver.Value
	ConvertValue(v interface{}) (Value, error)
}

// Valuer is the interface providing the Value method.
//
// Types implementing Valuer interface are able to convert
// themselves to a driver Value.
type Valuer interface {
	// Value returns a driver Value.
	// 将自身转换为driver.Value
	Value() (Value, error)
}

// Bool is a ValueConverter that converts input values to bools.
//
// The conversion rules are:
//  - booleans are returned unchanged
//  - for integer types,
//       1 is true
//       0 is false,
//       other integers are an error
//  - for strings and []byte, same rules as strconv.ParseBool
//  - all other types are an error
//
// 仔细读读上面的文档,就能理解下面的'func (boolType) ConvertValue'方法
var Bool boolType

type boolType struct{}

var _ ValueConverter = boolType{}

func (boolType) String() string { return "Bool" }

// 注:ConvertValue converts a value to a driver Value.
// 将v转换为driver.Value
func (boolType) ConvertValue(src interface{}) (Value, error) {
	// 尽量先用 type switch, 能不用反射就不用
	switch s := src.(type) {
	case bool:
		return s, nil
	case string:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("sql/driver: couldn't convert %q into type bool", s)
		}
		return b, nil
	case []byte:
		b, err := strconv.ParseBool(string(s))
		if err != nil {
			return nil, fmt.Errorf("sql/driver: couldn't convert %q into type bool", s)
		}
		return b, nil
	}

	sv := reflect.ValueOf(src)
	switch sv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		iv := sv.Int()
		if iv == 1 || iv == 0 {
			return iv == 1, nil
		}
		return nil, fmt.Errorf("sql/driver: couldn't convert %d into type bool", iv)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uv := sv.Uint()
		if uv == 1 || uv == 0 {
			return uv == 1, nil
		}
		return nil, fmt.Errorf("sql/driver: couldn't convert %d into type bool", uv)
	}

	// 注:all other types are an error
	return nil, fmt.Errorf("sql/driver: couldn't convert %v (%T) into type bool", src, src)
}

// Int32 is a ValueConverter that converts input values to int64,
// respecting the limits of an int32 value.
var Int32 int32Type

type int32Type struct{}

var _ ValueConverter = int32Type{}

func (int32Type) ConvertValue(v interface{}) (Value, error) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// 注意: rv.Int()的返回值是int64
		i64 := rv.Int()
		if i64 > (1<<31)-1 || i64 < -(1<<31) {
			// 超过了int32的限制
			return nil, fmt.Errorf("sql/driver: value %d overflows int32", v)
		}
		return i64, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		// 注意: rv.Uint()返回值是uint64
		u64 := rv.Uint()
		if u64 > (1<<31)-1 {
			// 超过了int32的限制
			return nil, fmt.Errorf("sql/driver: value %d overflows int32", v)
		}
		return int64(u64), nil
	case reflect.String:
		i, err := strconv.Atoi(rv.String())
		if err != nil {
			return nil, fmt.Errorf("sql/driver: value %q can't be converted to int32", v)
		}
		return int64(i), nil
	}
	// 到这里还没有返回,说明src不能被转换为int32
	// %T 是指输出类型
	return nil, fmt.Errorf("sql/driver: unsupported value %v (type %T) converting to int32", v, v)
}

// String is a ValueConverter that converts its input to a string.
// If the value is already a string or []byte, it's unchanged.
// If the value is of another type, conversion to string is done
// with fmt.Sprintf("%v", v).
var String stringType

type stringType struct{}

func (stringType) ConvertValue(v interface{}) (Value, error) {
	switch v.(type) {
	case string, []byte:
		// 注:If the value is already a string or []byte, it's unchanged.
		return v, nil
	}
	// 注:If the value is of another type, conversion to string is done
	// with fmt.Sprintf("%v", v).
	return fmt.Sprintf("%v", v), nil
}

// Null is a type that implements ValueConverter by allowing nil
// values but otherwise delegating to another ValueConverter.
type Null struct {
	Converter ValueConverter
}

func (n Null) ConvertValue(v interface{}) (Value, error) {
	if v == nil {
		// 允许nil
		return nil, nil
	}
	// 非nil进行委托调用
	return n.Converter.ConvertValue(v)
}

// NotNull is a type that implements ValueConverter by disallowing nil
// values but otherwise delegating to another ValueConverter.
type NotNull struct {
	Converter ValueConverter
}

func (n NotNull) ConvertValue(v interface{}) (Value, error) {
	if v == nil {
		// 不允许nil
		return nil, fmt.Errorf("nil value not allowed")
	}
	// nil进行委托调用
	return n.Converter.ConvertValue(v)
}

// IsValue reports whether v is a valid Value parameter type.
//
// driver.IsValue报告v是否是一个driver.Value
// 参考: go doc driver.Value
func IsValue(v interface{}) bool {
	if v == nil {
		return true
	}
	switch v.(type) {
	case []byte, bool, float64, int64, string, time.Time:
		return true
	}
	return false
}

// IsScanValue is equivalent to IsValue.
// It exists for compatibility.
func IsScanValue(v interface{}) bool {
	return IsValue(v)
}

// DefaultParameterConverter is the default implementation of
// ValueConverter that's used when a Stmt doesn't implement
// ColumnConverter.
//
// DefaultParameterConverter returns its argument directly if
// IsValue(arg). Otherwise, if the argument implements Valuer, its
// Value method is used to return a Value. As a fallback, the provided
// argument's underlying type is used to convert it to a Value:
// underlying integer types are converted to int64, floats to float64,
// bool, string, and []byte to themselves. If the argument is a nil
// pointer, ConvertValue returns a nil Value. If the argument is a
// non-nil pointer, it is dereferenced and ConvertValue is called
// recursively. Other types are an error.
var DefaultParameterConverter defaultConverter

type defaultConverter struct{}

var _ ValueConverter = defaultConverter{}

var valuerReflectType = reflect.TypeOf((*Valuer)(nil)).Elem()

// callValuerValue returns vr.Value(), with one exception:
// If vr.Value is an auto-generated method on a pointer type and the
// pointer is nil, it would panic at runtime in the panicwrap
// method. Treat it like nil instead.
// Issue 8415.
//
// This is so people can implement driver.Value on value types and
// still use nil pointers to those types to mean nil/NULL, just like
// string/*string.
//
// This function is mirrored in the database/sql package.
//
// 函数名的意义是:调用Valuer接口的Value方法
func callValuerValue(vr Valuer) (v Value, err error) {
	if rv := reflect.ValueOf(vr); rv.Kind() == reflect.Ptr &&
		rv.IsNil() &&
		rv.Type().Elem().Implements(valuerReflectType) {
		return nil, nil
	}
	return vr.Value()
}

func (defaultConverter) ConvertValue(v interface{}) (Value, error) {
	if IsValue(v) {
		// 文档:DefaultParameterConverter returns its argument directly if IsValue(arg).
		return v, nil
	}

	if vr, ok := v.(Valuer); ok {
		// 文档:if the argument implements Valuer, its Value method is used to return a Value.
		sv, err := callValuerValue(vr)
		if err != nil {
			// 调用 .Value() 出错
			return nil, err
		}
		if !IsValue(sv) {
			// 判断callValuerValue返回的值是否是 dirver.Value
			return nil, fmt.Errorf("non-Value type %T returned from Value", sv)
		}
		// 现在,sv是一个driver.Value
		return sv, nil
	}

	// 文档:As a fallback, the provided argument's underlying type is used to convert it to a Value:
	// underlying integer types are converted to int64, floats to float64, and strings to []byte.
	// If the argument is a nil pointer, ConvertValue returns a nil Value.
	// If the argument is a non-nil pointer, it is dereferenced and ConvertValue is called recursively.
	// Other types are an error.
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr:
		// indirect pointers
		if rv.IsNil() {
			return nil, nil
		} else {
			// 递归
			return defaultConverter{}.ConvertValue(rv.Elem().Interface())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return int64(rv.Uint()), nil
	case reflect.Uint64:
		u64 := rv.Uint()
		if u64 >= 1<<63 {
			return nil, fmt.Errorf("uint64 values with high bit set are not supported")
		}
		return int64(u64), nil
	case reflect.Float32, reflect.Float64:
		return rv.Float(), nil
	case reflect.Bool:
		return rv.Bool(), nil
	case reflect.Slice:
		ek := rv.Type().Elem().Kind()
		if ek == reflect.Uint8 {
			return rv.Bytes(), nil
		}
		return nil, fmt.Errorf("unsupported type %T, a slice of %s", v, ek)
	case reflect.String:
		return rv.String(), nil
	}
	return nil, fmt.Errorf("unsupported type %T, a %s", v, rv.Kind())
}
