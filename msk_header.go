package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"modernc.org/cc/v4"
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

type ParamDecl struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type MskFunction struct {
	Name       string      `json:"name"`
	Parameters []ParamDecl `json:"parameters"`
	ReturnType string      `json:"return_type"`
}

type MosekH struct {
	Enums     map[string]*MskEnum `json:"enums"`
	EnumList  []string            `json:"enum_list"`
	Functions []*MskFunction      `json:"functions"`
	Typedefs  map[string]string   `json:"typedefs"`
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

func cSpelling(t cc.Type, ignoreTypedef bool) string {
	if t == nil {
		return ""
	}
	var prefix string
	if t.Attributes().IsConst() {
		prefix = "const "
	}
	if t.Attributes().IsVolatile() {
		prefix += "volatile "
	}

	if !ignoreTypedef {
		if td := t.Typedef(); td != nil {
			return prefix + td.Name()
		}
	}

	switch t.Kind() {
	case cc.Void:
		return prefix + "void"
	case cc.Bool:
		return prefix + "_Bool"
	case cc.Char:
		return prefix + "char"
	case cc.SChar:
		return prefix + "signed char"
	case cc.UChar:
		return prefix + "unsigned char"
	case cc.Short:
		return prefix + "short"
	case cc.UShort:
		return prefix + "unsigned short"
	case cc.Int:
		return prefix + "int"
	case cc.UInt:
		return prefix + "unsigned int"
	case cc.Long:
		return prefix + "long"
	case cc.ULong:
		return prefix + "unsigned long"
	case cc.LongLong:
		return prefix + "long long"
	case cc.ULongLong:
		return prefix + "unsigned long long"
	case cc.Float:
		return prefix + "float"
	case cc.Double:
		return prefix + "double"
	case cc.LongDouble:
		return prefix + "long double"
	case cc.Ptr:
		elem := t.(*cc.PointerType).Elem()
		return cSpelling(elem, false) + " *"
	case cc.Array:
		elem := t.(*cc.ArrayType).Elem()
		length := t.(*cc.ArrayType).Len()
		if length <= 0 {
			return cSpelling(elem, false) + "[]"
		}
		return fmt.Sprintf("%s[%d]", cSpelling(elem, false), length)
	case cc.Struct:
		tagTok := t.(*cc.StructType).Tag()
		tag := tagTok.SrcStr()
		if tag == "" {
			return prefix + "struct"
		}
		return prefix + "struct " + tag
	case cc.Union:
		tagTok := t.(*cc.UnionType).Tag()
		tag := tagTok.SrcStr()
		if tag == "" {
			return prefix + "union"
		}
		return prefix + "union " + tag
	case cc.Enum:
		tagTok := t.(*cc.EnumType).Tag()
		tag := tagTok.SrcStr()
		if tag == "" {
			return prefix + "enum"
		}
		return prefix + "enum " + tag
	default:
		return prefix + t.String()
	}
}

// Build MosekH from the AST
func (h *MosekH) Build(ast *cc.AST, fileName string) *MosekH {
	var enums []*cc.EnumSpecifier
	var functions []*cc.Declarator
	var typedefs []*cc.Declarator

	for _, nodes := range ast.Scope.Nodes {
		for _, node := range nodes {
			pos := node.Position()
			if pos.Filename != fileName {
				continue
			}

			switch n := node.(type) {
			case *cc.EnumSpecifier:
				enums = append(enums, n)
			case *cc.Declarator:
				if n.IsTypename() {
					typedefs = append(typedefs, n)
				} else if n.Type().Kind() == cc.Function {
					functions = append(functions, n)
				}
			}
		}
	}

	// Sort by offset to preserve file order
	slices.SortFunc(enums, func(a, b *cc.EnumSpecifier) int {
		return cmp.Compare(a.Position().Offset, b.Position().Offset)
	})
	slices.SortFunc(functions, func(a, b *cc.Declarator) int {
		return cmp.Compare(a.Position().Offset, b.Position().Offset)
	})
	slices.SortFunc(typedefs, func(a, b *cc.Declarator) int {
		return cmp.Compare(a.Position().Offset, b.Position().Offset)
	})

	// Process Enums
	for _, e := range enums {
		et := e.Type().(*cc.EnumType)
		tagTok := et.Tag()
		tag := tagTok.SrcStr()
		if !strings.HasPrefix(tag, "MSK") {
			continue
		}
		integerType := cSpelling(et.UnderlyingType(), false)
		me := h.AddEnum(tag, integerType)
		for _, ev := range et.Enumerators() {
			var valStr string
			switch v := ev.Value().(type) {
			case cc.Int64Value:
				valStr = fmt.Sprintf("%d", v)
			case cc.UInt64Value:
				valStr = fmt.Sprintf("%d", v)
			default:
				valStr = fmt.Sprintf("%v", ev.Value())
			}
			me.AddValue(ev.Token.SrcStr(), valStr)
		}
	}

	// Process Typedefs
	for _, td := range typedefs {
		if !strings.HasPrefix(td.Name(), "MSK") {
			continue
		}
		h.AddEnumTypeDef(cSpelling(td.Type(), true), td.Name())
	}

	// Process Functions
	for _, f := range functions {
		if !strings.HasPrefix(f.Name(), "MSK") {
			continue
		}
		ft := f.Type().(*cc.FunctionType)
		mf := &MskFunction{
			Name:       f.Name(),
			ReturnType: cSpelling(ft.Result(), false),
		}
		for _, param := range ft.Parameters() {
			if param.Type().Kind() == cc.Void {
				continue
			}
			mf.Parameters = append(mf.Parameters, ParamDecl{
				Name: param.Name(),
				Type: cSpelling(param.Type(), false),
			})
		}
		h.Functions = append(h.Functions, mf)
	}

	return h
}
