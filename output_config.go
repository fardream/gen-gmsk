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

//go:embed enums.tmpl
var enumTmpl string

type OutputConfig struct {
	Enums           map[string]*enumConfig `json:"enums"`
	PackageName     string                 `json:"package_name"`
	TypeToGoType    map[string]string      `json:"type_to_go_type"`
	Funcs           map[string]*FuncConfig `json:"funcs"`
	Deprecated      map[string]struct{}    `json:"deprecated"`
	Urls            map[string]string      `json:"urls"`
	RustFuncs       []RustFunc             `json:"rust_funcs"`
	RustEnums       map[string]RustEnum    `json:"rust_enums"`
	mappedRustFuncs map[string]RustFunc    `json:"-"`
}

func newOutputConfig() *OutputConfig {
	r := &OutputConfig{
		Enums:       make(map[string]*enumConfig),
		PackageName: "gmsk",
		TypeToGoType: map[string]string{
			"int32_t":      "int32",
			"int64_t":      "int64",
			"int":          "int32",
			"long long":    "int64",
			"size_t":       "uint64",
			"double":       "float64",
			"char":         "byte",
			"MSKrescodee":  "ResCode",
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

// stripCTypePrefix removes enum/struct etc from the type of C types.
func stripCTypePrefix(s string) string {
	if strings.HasPrefix(s, "enum ") {
		return strings.TrimPrefix(s, "enum ")
	} else if strings.HasPrefix(s, "struct ") {
		return strings.TrimPrefix(s, "struct ")
	}
	return s
}

func normalize(h *MosekH, config *OutputConfig) error {
	if config.Enums == nil {
		config.Enums = make(map[string]*enumConfig)
	}
	if config.TypeToGoType == nil {
		config.TypeToGoType = make(map[string]string)
	}
	for _, enumName := range h.EnumList {
		v, ok := h.Enums[enumName]
		if !ok {
			return fmt.Errorf("enum %s is not found in parsed mosek.h", enumName)
		}
		ec, found := config.Enums[enumName]
		if !found {
			goName := upperCaseFirstLetter(strings.TrimSuffix(GetGoName(enumName), "_enum"))
			ec = &enumConfig{CommonId: CommonId{GoName: goName, Skip: false}}
			config.Enums[enumName] = ec
		}
		if ec.ConstantComments == nil {
			ec.ConstantComments = make(map[string]string)
		}
		if ec.IntegerType == "" {
			ec.IntegerType = GetEnumType(v.IntegerType)
		}
		_, found = config.TypeToGoType[enumName]
		if !found {
			config.TypeToGoType[enumName] = ec.GoName
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
		toType := stripCTypePrefix(toType)
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

type funcFileTmplInput struct {
	*OutputConfig
	Funcs []*FuncTmplInput

	Desc string
}

func (fi *funcFileTmplInput) ExtraStdPkgs() []string {
	pkgs := make(map[string]struct{})
	for _, f := range fi.Funcs {
		for _, p := range f.ExtraStdPkgs() {
			pkgs[p] = struct{}{}
		}
	}

	return keys(pkgs)
}

func BuildFuncs(h *MosekH, config *OutputConfig, funcTypeFilter funcType, out io.Writer) error {
	input := &funcFileTmplInput{
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

	return funcFileTmpl.Execute(out, &input)
}

var (
	funcFileTmpl *template.Template
	enumFileTmpl *template.Template
)

func init() {
	var err error
	funcFileTmpl, err = template.New("func-tmpl").Parse(funcTmpl)
	if err != nil {
		log.Panic(err)
	}
	enumFileTmpl, err = template.New("enum-tmpl").Parse(enumTmpl)
	if err != nil {
		log.Panic(err)
	}
}
