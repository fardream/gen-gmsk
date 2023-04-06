package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-clang/clang-v15/clang"
)

type MskEnumValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type MskEnum struct {
	Name        string         `json:"name"`
	IntegerType string         `json:"integer_type"`
	Values      []MskEnumValue `json:"values"`
}

func (e *MskEnum) AddValue(name, value string) *MskEnum {
	if e == nil {
		return e
	}

	e.Values = append(e.Values, MskEnumValue{Name: name, Value: value})
	return e
}

type MskFunctionParameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type MskFunction struct {
	Name       string                 `json:"name"`
	Parameters []MskFunctionParameter `json:"parameters"`
	ReturnType string                 `json:"return_type"`
}

type MosekH struct {
	Enums     map[string]*MskEnum `json:"-"` // turn off
	EnumList  []string            `json:"-"` // turn off
	Functions []*MskFunction      `json:"functions"`
	Typedefs  map[string]string   `json:"-"`
}

type Typedef struct {
	Name string `json:"name"`
}

func NewMosekH() *MosekH {
	return &MosekH{
		Enums:    make(map[string]*MskEnum),
		Typedefs: make(map[string]string),
	}
}

func (h *MosekH) AddEnum(name string, integerType string) *MskEnum {
	if name == "" {
		return nil
	}
	if h.Enums == nil {
		h.Enums = make(map[string]*MskEnum)
	}
	e, found := h.Enums[name]
	if !found {
		e = &MskEnum{
			Name:        name,
			IntegerType: integerType,
		}
		h.Enums[name] = e
		h.EnumList = append(h.EnumList, name)
	} else {
		e.Name = name
		e.IntegerType = integerType
	}

	return e
}

func (h *MosekH) AddEnumTypeDef(orig, defto string) {
	if h.Typedefs == nil {
		h.Typedefs = make(map[string]string)
	}

	h.Typedefs[defto] = orig
}

func (h *MosekH) VisitFunc(cursor, parent clang.Cursor) clang.ChildVisitResult {
	if cursor.IsNull() {
		log.Printf("cursor: <none>\n")

		return clang.ChildVisit_Continue
	}
	name := cursor.Spelling()
	if !strings.HasPrefix(name, "MSK") {
		return clang.ChildVisit_Continue
	}

	switch cursor.Kind() {
	case clang.Cursor_EnumDecl:
		h.AddEnum(name, cursor.EnumDeclIntegerType().Spelling())

		return clang.ChildVisit_Recurse

	case clang.Cursor_EnumConstantDecl:
		if parent.IsNull() {
			log.Printf("cannot find corresponding enum for %s", name)
			return clang.ChildVisit_Continue
		}

		e := h.AddEnum(parent.Spelling(), parent.EnumDeclIntegerType().Spelling())
		e.AddValue(name, fmt.Sprintf("%d", cursor.EnumConstantDeclValue()))

		return clang.ChildVisit_Continue

	case clang.Cursor_TypedefDecl:
		underlyingType := cursor.TypedefDeclUnderlyingType()
		h.AddEnumTypeDef(underlyingType.Spelling(), cursor.Spelling())

		return clang.ChildVisit_Recurse

	case clang.Cursor_FunctionDecl:
		f := &MskFunction{Name: name}
		f.ReturnType = cursor.ResultType().Spelling()
		for i := uint32(0); i < uint32(cursor.NumArguments()); i++ {
			arg := cursor.Argument(i)
			f.Parameters = append(f.Parameters, MskFunctionParameter{
				Type: arg.Type().Spelling(),
				Name: arg.Spelling(),
			})
		}
		h.Functions = append(h.Functions, f)
		return clang.ChildVisit_Continue
	default:
		fmt.Printf("%s: %s (%s)\n", cursor.Kind().Spelling(), cursor.Spelling(), cursor.USR())
		return clang.ChildVisit_Continue
	}
}

// Build MosekH from the cursor
func (h *MosekH) Build(cursor clang.Cursor) *MosekH {
	cursor.Visit(h.VisitFunc)
	return h
}
