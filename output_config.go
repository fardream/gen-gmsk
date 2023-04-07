package main

import (
	"embed"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed config.yml
var configJson embed.FS

type CommonId struct {
	GoName   string `json:"go_name"`
	Skip     bool   `json:"skip"`
	Comments string `json:"comment"`
}
type EnumConfig struct {
	CommonId         `json:",inline"`
	ConstantComments map[string]string `json:"constant_comments"`
	IntegerType      string            `json:"integer_type"`
	IsEqualType      bool              `json:"is_equal_type"`
}

type ParamConfig struct {
	Name      string `json:"name"`
	GoType    string `json:"go_type"`
	CgoType   string `json:"cgo_type"`
	IsPointer bool   `json:"is_pointer"`
	IsConst   bool   `json:"is_const"`
}

func (c *ParamConfig) GetGoTypeString() string {
	if c.IsPointer {
		return fmt.Sprintf("*%s", c.GoType)
	}
	return c.GoType
}

func (c *ParamConfig) GetCgoTypeCast(withP bool) string {
	cgotype := c.CgoType
	if c.CgoType == "unsigned int" {
		cgotype = "uint"
	}
	if c.IsPointer && c.CgoType == "char" {
		return fmt.Sprintf("(*C.char)(unsafe.Pointer(%s))", c.Name)
	} else if c.IsPointer && withP {
		return fmt.Sprintf("(*C.%s)(&%s)", cgotype, c.Name)
	} else if c.IsPointer {
		return fmt.Sprintf("(*C.%s)(%s)", cgotype, c.Name)
	}
	return fmt.Sprintf("C.%s(%s)", cgotype, c.Name)
}

type FuncConfig struct {
	CommonId `json:",inline"`

	IsTask          bool           `json:"is_task"`           // is task method
	IsEnv           bool           `json:"is_env"`            // is env method
	Params          []*ParamConfig `json:"params"`            // params
	LastParamOutput bool           `json:"last_param_outupt"` // last parameter is output
	ReturnType      string         `json:"return_type"`       // return type
}

type OutputConfig struct {
	Enums        map[string]*EnumConfig `json:"enums"`
	PackageName  string                 `json:"package_name"`
	TypeToGoType map[string]string      `json:"type_to_go_type"`
	Funcs        map[string]*FuncConfig `json:"funcs"`
}

func NewOutputConfig() *OutputConfig {
	data, err := configJson.ReadFile("config.yml")
	if err != nil {
		log.Panic(err)
	}

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
		},
	}

	err = yaml.Unmarshal(data, r)
	if err != nil {
		log.Panic(err)
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

func UpperCaseFirstLetter(s string) string {
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
			goName := UpperCaseFirstLetter(strings.TrimSuffix(GetGoName(enumName), "_enum"))
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
		// first, check if the first argument is MSKent_t or MSKtask_t
		// which we will use to add method to Task or Env
		normalizeFunction(f, config)
	}

	return nil
}

func normalizeFunction(f *MskFunction, config *OutputConfig) {
	fname := f.Name
	fc, found := config.Funcs[fname]
	if !found {
		fc = &FuncConfig{}
		config.Funcs[f.Name] = fc
	}
	if fc.Skip {
		return
	}
	if fc.GoName == "" {
		fc.GoName = GetFuncName(f.Name)
	}

	fc.IsTask = len(f.Parameters) >= 1 && f.Parameters[0].Type == "MSKtask_t"
	fc.IsEnv = len(f.Parameters) >= 1 && f.Parameters[0].Type == "MSKenv_t"
	firstParam := 0
	if fc.IsEnv || fc.IsTask {
		firstParam = 1
	}

	for i, p := range f.Parameters {
		if i < firstParam {
			continue
		}
		pc := &ParamConfig{Name: p.Name}
		pc.IsPointer = strings.HasSuffix(p.Type, " *")
		ctype := strings.TrimSuffix(p.Type, " *")
		pc.IsConst = strings.HasPrefix(p.Type, "const ")
		ctype = strings.TrimPrefix(ctype, "const ")
		pc.CgoType = ctype
		found = false
		pc.GoType, found = config.TypeToGoType[ctype]
		if !found {
			log.Printf("cannot find func: %s", f.Name)
		}

		fc.Params = append(fc.Params, pc)
	}

	goReturn, found := config.TypeToGoType[f.ReturnType]
	if !found {
		fmt.Printf("cannot find return type for %s: %s\n", f.ReturnType, f.Name)
	}

	fc.ReturnType = goReturn
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
		SplitComments(out, enumConfig.Comments)
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

	fmt.Fprintln(out, "const (")
	for _, v := range rescodeEnum.Values {
		fmt.Fprintf(out, "\t%s Code = %s\n", strings.TrimPrefix(v.Name, "MSK_RES_"), v.Value)
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

func BuildFuncs(h *MosekH, config *OutputConfig, out io.Writer) error {
	fmt.Fprintln(out, "// Automatically generated by github.com/fardream/gen-gmsk")
	fmt.Fprintln(out, "// funcs defitions")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "package %s\n", config.PackageName)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "// #include <mosek.h>")
	fmt.Fprintln(out, "import \"C\"")
	fmt.Fprintln(out, "\nimport \"unsafe\"")
	fmt.Fprintln(out, "\nimport \"github.com/fardream/gmsk/res\"")

	for _, f := range h.Functions {
		fc, ok := config.Funcs[f.Name]
		if !ok {
			return fmt.Errorf("cannot find %s in function configs", f.Name)
		}

		if err := BuildFunction(f, fc, config, out); err != nil {
			return err
		}
	}

	return nil
}

func BuildFunction(f *MskFunction, fc *FuncConfig, config *OutputConfig, out io.Writer) error {
	if fc.Skip {
		return nil
	}
	fmt.Fprintln(out)
	// first, check if the first argument is MSKent_t or MSKtask_t
	// which we will use to add method to Task or Env

	goName := fc.GoName

	s := ","
	if fc.Comments == "" {
		s = ""
	}
	fmt.Fprintf(out, "// %s is wrapping [%s]%s\n", fc.GoName, f.Name, s)
	SplitComments(out, fc.Comments)
	fmt.Fprintln(out, "//")
	fmt.Fprintf(out, "// function %s has following parameters:\n", f.Name)
	for _, fp := range f.Parameters {
		fmt.Fprintf(out, "//   - %s: %s\n", fp.Name, fp.Type)
	}
	fmt.Fprintf(out, "//\n// [%s]: https://docs.mosek.com/latest/capi/alphabetic-functionalities.html\n", f.Name)
	if fc.IsTask {
		fmt.Fprintf(out, "func (task *Task) %s(\n", goName)
	} else if fc.IsEnv {
		fmt.Fprintf(out, "func (env *Env) %s(\n", goName)
	} else {
		fmt.Fprintf(out, "func %s(\n", goName)
	}

	n := len(fc.Params)
	lastParam := &ParamConfig{}
	if fc.LastParamOutput {
		n -= 1
		lastParam = fc.Params[n]
	}
	for _, pc := range fc.Params[:n] {
		fmt.Fprintf(out, "\t%s %s,\n", pc.Name, pc.GetGoTypeString())
	}

	if fc.LastParamOutput {
		fmt.Fprintf(out, ") (r %s, %s %s) {\n", fc.ReturnType, lastParam.Name, lastParam.GoType)
		fmt.Fprintf(out, "\tr = %s(\n", fc.ReturnType)
	} else {
		fmt.Fprintf(out, ") %s {\n", fc.ReturnType)
		fmt.Fprintf(out, "\treturn %s(\n", fc.ReturnType)
	}
	fmt.Fprintf(out, "\t\tC.%s(\n", f.Name)
	if fc.IsTask {
		fmt.Fprintln(out, "\t\t\ttask.task,")
	} else if fc.IsEnv {
		fmt.Fprintln(out, "\t\t\tenv.getEnv(),")
	}
	for _, pc := range fc.Params[:n] {
		fmt.Fprintf(out, "\t\t\t%s,\n", pc.GetCgoTypeCast(false))
	}
	if fc.LastParamOutput {
		fmt.Fprintf(out, "\t\t\t%s,\n", lastParam.GetCgoTypeCast(true))
	}
	fmt.Fprintln(out, "\t\t),")
	fmt.Fprintln(out, "\t)")
	if fc.LastParamOutput {
		fmt.Fprintln(out, "\treturn")
	}
	fmt.Fprintln(out, "}")

	return nil
}
