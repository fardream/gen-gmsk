package main

import (
	_ "embed"
	"fmt"
	"io"
	"log"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"
)

//go:embed config.yml
var configStr []byte

//go:embed func.tmpl
var funcTmpl string

//go:embed urls.yml
var urlsStr []byte

//go:embed deprecated.yml
var deprecatedStr []byte

type EnumConfig struct {
	CommonId         `json:",inline"`
	ConstantComments map[string]string `json:"constant_comments"`
	IntegerType      string            `json:"integer_type"`
	IsEqualType      bool              `json:"is_equal_type"`
}

type OutputConfig struct {
	Enums           map[string]*EnumConfig `json:"enums"`
	PackageName     string                 `json:"package_name"`
	TypeToGoType    map[string]string      `json:"type_to_go_type"`
	Funcs           map[string]*FuncConfig `json:"funcs"`
	Deprecated      map[string]struct{}    `json:"deprecated"`
	Urls            map[string]string      `json:"urls"`
	RustFuncs       []RustFunc             `json:"rust_funcs"`
	RustEnums       map[string]RustEnum    `json:"rust_enums"`
	mappedRustFuncs map[string]RustFunc    `json:"-"`
}

func NewOutputConfig() *OutputConfig {
	r := &OutputConfig{
		Enums:       make(map[string]*EnumConfig),
		PackageName: "gmsk",
		TypeToGoType: map[string]string{
			"int32_t":      "int32",
			"int64_t":      "int64",
			"int":          "int32",
			"long long":    "int64",
			"size_t":       "uint64",
			"double":       "float64",
			"char":         "byte",
			"MSKrescodee":  "res.Code",
			"unsigned int": "uint32",
			"MSKbooleant":  "bool",
		},
		Deprecated:      make(map[string]struct{}),
		Urls:            make(map[string]string),
		RustEnums:       make(map[string]RustEnum),
		mappedRustFuncs: make(map[string]RustFunc),
	}

	if err := yaml.UnmarshalWithOptions(configStr, r, yaml.Strict()); err != nil {
		log.Panic(err)
	}
	if err := yaml.UnmarshalWithOptions(urlsStr, &r.Urls, yaml.Strict()); err != nil {
		log.Panic(err)
	}
	if err := yaml.UnmarshalWithOptions(deprecatedStr, &r.Deprecated, yaml.Strict()); err != nil {
		log.Panic(err)
	}
	if err := yaml.UnmarshalWithOptions(rustEnumsBytes, &r.RustEnums, yaml.Strict()); err != nil {
		log.Panic(err)
	}
	if err := yaml.UnmarshalWithOptions(rustFuncsBytes, &r.RustFuncs, yaml.Strict()); err != nil {
		log.Panic(err)
	}

	for _, f := range r.RustFuncs {
		mskname := fmt.Sprintf("MSK_%s", strings.ReplaceAll(f.Name, "_", ""))
		r.mappedRustFuncs[mskname] = f
	}

	return r
}

// GetGoName removes the prefix MSK or MSK_
func GetGoName(name string) string {
	if strings.HasPrefix(name, "MSK_") {
		return strings.TrimPrefix(name, "MSK_")
	} else if strings.HasPrefix(name, "MSK") {
		return strings.TrimPrefix(name, "MSK")
	}

	return name
}

func SplitComments(out io.Writer, s string) {
	if s == "" {
		return
	}
	for _, v := range strings.Split(s, "\n") {
		fmt.Fprintf(out, "// %s\n", v)
	}
}

func GetEnumType(underlying string) string {
	switch underlying {
	case "int":
		return "int32"
	case "unsigned int":
		fallthrough
	default:
		return "uint32"
	}
}

func lowerCaseFirstLetter(s string) string {
	b := []byte(s)
	if len(b) == 0 {
		return s
	}
	if b[0] >= 'A' && b[0] <= 'Z' {
		b[0] += byte('a') - byte('A')
	}

	return string(b)
}

func upperCaseFirstLetter(s string) string {
	b := []byte(s)
	if len(b) == 0 {
		return s
	}
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= byte('a') - byte('A')
	}

	return string(b)
}

// StripCTypePrefix removes enum/struct etc from the type of C types.
func StripCTypePrefix(s string) string {
	if strings.HasPrefix(s, "enum ") {
		return strings.TrimPrefix(s, "enum ")
	} else if strings.HasPrefix(s, "struct ") {
		return strings.TrimPrefix(s, "struct ")
	}
	return s
}

func Normalize(h *MosekH, config *OutputConfig) error {
	if config.Enums == nil {
		config.Enums = make(map[string]*EnumConfig)
	}
	if config.TypeToGoType == nil {
		config.TypeToGoType = make(map[string]string)
	}
	for _, enumName := range h.EnumList {
		v, ok := h.Enums[enumName]
		if !ok {
			return fmt.Errorf("enum %s is not found in parsed mosek.h", enumName)
		}
		enumConfig, found := config.Enums[enumName]
		if !found {
			goName := upperCaseFirstLetter(strings.TrimSuffix(GetGoName(enumName), "_enum"))
			enumConfig = &EnumConfig{CommonId: CommonId{GoName: goName, Skip: false}}
			config.Enums[enumName] = enumConfig
		}
		if enumConfig.ConstantComments == nil {
			enumConfig.ConstantComments = make(map[string]string)
		}
		if enumConfig.IntegerType == "" {
			enumConfig.IntegerType = GetEnumType(v.IntegerType)
		}
		_, found = config.TypeToGoType[enumName]
		if !found {
			config.TypeToGoType[enumName] = enumConfig.GoName
		}
	}

	for k, v := range config.RustEnums {
		cname := fmt.Sprintf("MSK%s_enum", lowerCaseFirstLetter(k))
		e, found := h.Enums[cname]
		if !found {
			continue
		}
		ec, found := config.Enums[cname]
		if !found {
			continue
		}
		if ec.Comment == "" {
			ec.Comment = v.Comment
		}
		for _, recconst := range v.EnumConsts {
			for _, ev := range e.Values {
				if ev.Value == recconst.Value {
					_, found := ec.ConstantComments[ev.Name]
					if !found {
						ec.ConstantComments[ev.Name] = recconst.Comment
					}
				}
			}
		}
	}

	for fromType, toType := range h.Typedefs {
		toType := StripCTypePrefix(toType)
		_, found := config.TypeToGoType[fromType]
		if found {
			continue
		}
		mappedTo, found := config.TypeToGoType[toType]
		if found {
			config.TypeToGoType[fromType] = mappedTo
			continue
		}
		log.Printf("cannot find mapping for %s -> %s", fromType, toType)
	}

	if config.Funcs == nil {
		config.Funcs = make(map[string]*FuncConfig)
	}

	for _, f := range h.Functions {
		normalizeFunction(f, config)
	}

	return nil
}

func BuildEnums(h *MosekH, config *OutputConfig, out io.Writer) error {
	fmt.Fprintln(out, "// Automatically generated by github.com/fardream/gen-gmsk")
	fmt.Fprintln(out, "// There are many enums in MOSEK, this consolidate everything here.")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "package %s\n", config.PackageName)

	for _, enumName := range h.EnumList {
		if enumName == "MSKrescode_enum" {
			continue
		}
		fmt.Fprintln(out)

		enumData, ok := h.Enums[enumName]
		if !ok {
			return fmt.Errorf("enum %s is not found in parsed mosek.h", enumName)
		}
		enumConfig, found := config.Enums[enumName]
		if !found {
			log.Panicf("failed to find confing for enum: %s", enumName)
		}
		if enumConfig.Skip {
			continue
		}
		if enumConfig.ConstantComments == nil {
			enumConfig.ConstantComments = make(map[string]string)
		}
		fmt.Fprintf(out, "// %s is %s\n//\n", enumConfig.GoName, enumName)
		SplitComments(out, enumConfig.Comment)
		equalstr := " "
		if enumConfig.IsEqualType {
			equalstr = " = "
		}
		fmt.Fprintf(out, "type %s%s%s\n", enumConfig.GoName, equalstr, enumConfig.IntegerType)
		if len(enumData.Values) == 0 {
			continue
		}

		fmt.Fprintln(out, "const (")
		for _, enumValue := range enumData.Values {
			valueComment := ""
			if enumConfig.ConstantComments != nil {
				v, found := enumConfig.ConstantComments[enumValue.Name]
				if found {
					valueComment = fmt.Sprintf("// %s", v)
				}
			}
			fmt.Fprintf(out, "\t%s %s = %s%s\n", GetGoName(enumValue.Name), enumConfig.GoName, enumValue.Value, valueComment)
		}
		fmt.Fprintln(out, ")")
	}

	return nil
}

func BuildResCode(h *MosekH, config *OutputConfig, out io.Writer) error {
	rescodeEnum, ok := h.Enums["MSKrescode_enum"]
	if !ok {
		return fmt.Errorf("failed to find MSKrescode_enum from parsed mosek header")
	}

	fmt.Fprintln(out, "// Automatically generated by github.com/fardream/gen-gmsk")
	fmt.Fprintln(out, "// response codes")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "package res")

	enumConfig, found := config.Enums["MSKrescode_enum"]
	if !found {
		enumConfig = &EnumConfig{
			CommonId: CommonId{},
		}
	}
	fmt.Fprintln(out, "const (")
	for _, v := range rescodeEnum.Values {
		valueComment := ""
		if enumConfig.ConstantComments != nil {
			v, found := enumConfig.ConstantComments[v.Name]
			if found {
				valueComment = fmt.Sprintf("// %s", v)
			}
		}
		fmt.Fprintf(out, "\t%s Code = %s%s\n", strings.TrimPrefix(v.Name, "MSK_RES_"), v.Value, valueComment)

	}
	fmt.Fprintln(out, ")")

	fmt.Fprintln(out, "var resCodeMsg map[Code]string = make(map[Code]string)")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "func init() {")
	for _, v := range rescodeEnum.Values {
		fmt.Fprintf(out, "resCodeMsg[%s] = \"%s (%s)\"\n", strings.TrimPrefix(v.Name, "MSK_RES_"), v.Name, v.Value)
	}
	fmt.Fprintln(out, "}")

	return nil
}

type FileTmplInput struct {
	*OutputConfig
	Funcs []*FuncTmplInput

	Desc string
}

func (fi *FileTmplInput) ExtraStdPkgs() []string {
	pkgs := make(map[string]struct{})
	for _, f := range fi.Funcs {
		for _, p := range f.ExtraStdPkgs() {
			pkgs[p] = struct{}{}
		}
	}

	return keys(pkgs)
}

func BuildFuncs(h *MosekH, config *OutputConfig, funcTypeFilter funcType, out io.Writer) error {
	input := &FileTmplInput{
		Desc:         "function deinitions",
		OutputConfig: config,
	}

	for _, f := range h.Functions {
		fc, ok := config.Funcs[f.Name]
		if !ok {
			return fmt.Errorf("cannot find %s in function configs", f.Name)
		}
		if fc.Skip || fc.FuncType != funcTypeFilter {
			continue
		}
		input.Funcs = append(input.Funcs, &FuncTmplInput{
			FuncConfig: fc,
			CFunc:      f,
			config:     config,
		})
	}

	return tmpl.Execute(out, &input)
}

var tmpl *template.Template

func init() {
	var err error
	tmpl, err = template.New("func-tmpl").Parse(funcTmpl)
	if err != nil {
		log.Panic(err)
	}
}
