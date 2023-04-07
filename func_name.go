package main

import (
	"fmt"
	"strings"
)

type pair struct {
	MskName string
	GoName  string
}

var funcActions = []pair{
	{"analyze", "Analyze"},
	{"append", "Append"},
	{"make", "Make"},
	{"get", "Get"},
	{"put", "Put"},
}

var funcSuffix = []pair{
	{"sliceconst", "SliceConst"},
	{"listconst", "ListConst"},
	{"slice", "Slice"},
	{"list", "List"},
	{"seq", "Seq"},
}

func GetFuncAction(s string) (string, string) {
	for _, a := range funcActions {
		if strings.HasPrefix(s, a.MskName) {
			return strings.TrimPrefix(s, a.MskName), a.GoName
		}
	}

	return s, ""
}

func GetFunctionSuffix(s string) (string, string) {
	for _, a := range funcSuffix {
		if strings.HasSuffix(s, a.MskName) {
			return strings.TrimSuffix(s, a.MskName), a.GoName
		}
	}

	return s, ""
}

func GetFuncName(s string) string {
	after, action := GetFuncAction(GetGoName(s))
	mid, suffix := GetFunctionSuffix(after)
	mid = UpperCaseFirstLetter(mid)
	return fmt.Sprintf("%s%s%s", action, mid, suffix)
}
