package jsh

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"
)

func valueToPrintable(value goja.Value) any {
	val := value.Export()
	if obj, ok := val.(*goja.Object); ok {
		toString, ok := goja.AssertFunction(obj.Get("toString"))
		if ok {
			ret, _ := toString(obj)
			return ret
		}
	}
	if m, ok := val.(map[string]any); ok {
		f := []string{}
		for k, v := range m {
			f = append(f, fmt.Sprintf("%s:%v", k, v))
		}
		return fmt.Sprintf("{%s}", strings.Join(f, ", "))
	}
	if a, ok := val.([]any); ok {
		f := []string{}
		for _, v := range a {
			f = append(f, fmt.Sprintf("%v", v))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	}
	if a, ok := val.([]float64); ok {
		f := []string{}
		for _, v := range a {
			f = append(f, fmt.Sprintf("%v", v))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	}
	if a, ok := val.([][]float64); ok {
		f := []string{}
		for _, vv := range a {
			fv := []string{}
			for _, v := range vv {
				fv = append(fv, fmt.Sprintf("%v", v))
			}
			f = append(f, fmt.Sprintf("[%s]", strings.Join(fv, ", ")))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	}
	return fmt.Sprintf("%v", val)
}
