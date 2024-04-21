package main

import (
	"fmt"
	"strings"
)

type enumConfig struct {
	CommonId         `json:",inline"`
	ConstantComments map[string]string `json:"constant_comments"`
	IntegerType      string            `json:"integer_type"`
	IsEqualType      bool              `json:"is_equal_type"`
}

type enumFileInput struct {
	*enumConfig

	CEnum *MskEnum

	PkgName string

	stripPrefix string
}

func (e *enumFileInput) CName() string {
	return e.CEnum.Name
}

func (e *enumFileInput) ConstantValues() []string {
	if len(e.CEnum.Values) == 0 {
		return nil
	}

	var r []string
	for _, ev := range e.CEnum.Values {
		constname := strings.TrimPrefix(ev.Name, e.stripPrefix)
		c, found := e.ConstantComments[ev.Name]
		if found {
			r = append(r, fmt.Sprintf("%s %s = C.%s // %s", constname, e.GoName, ev.Name, c))
		} else {
			r = append(r, fmt.Sprintf("%s %s = C.%s", constname, e.GoName, ev.Name))
		}
	}

	return r
}

func (e *enumFileInput) ConstantMaps() []string {
	if len(e.CEnum.Values) == 0 {
		return nil
	}

	var r []string
	for _, ev := range e.CEnum.Values {
		constname := strings.TrimPrefix(ev.Name, e.stripPrefix)
		r = append(r, fmt.Sprintf("%s: \"%s\",", constname, constname))
	}

	return r
}
