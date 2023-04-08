package main

import (
	"fmt"
	"log"
	"strings"
)

type funcType uint

const (
	funcType_NORMAL funcType = iota
	funcType_ENV
	funcType_TASK_PUT
	funcType_TASK_GET
	funcType_TASK_APPEND
	funcType_TASK_OTHER
)

func (t funcType) OutputFile() string {
	switch t {
	case funcType_NORMAL:
		return "other_funcs.go"
	case funcType_ENV:
		return "env_methods.go"
	case funcType_TASK_APPEND:
		return "task_append.go"
	case funcType_TASK_GET:
		return "task_get.go"
	case funcType_TASK_PUT:
		return "task_put.go"
	case funcType_TASK_OTHER:
		return "task_other.go"
	}

	log.Panicf("doesn't recognize %d", t)

	return ""
}

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

func GetFuncName(s string) (action string, mid string, suffix string) {
	var after string
	after, action = GetFuncAction(GetGoName(s))
	mid, suffix = GetFunctionSuffix(after)
	return
}

type ParamConfig struct {
	Name      string `json:"name"`
	OrigCType string `json:"orig_c_type"`
	GoType    string `json:"go_type"`
	CgoType   string `json:"cgo_type"`
	IsPointer bool   `json:"is_pointer"`
	IsConst   bool   `json:"is_const"`
	IsString  bool   `json:"is_string"`
	IsTask    bool   `json:"is_task"`
	IsEnv     bool   `json:"is_env"`
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
	*CommonId `json:",inline"`

	LastParamOutput bool     `json:"last_param_outupt"` // last parameter is output
	FuncType        funcType `json:"func_type"`

	params []*ParamConfig
}

func (fc *FuncConfig) IsEnv() bool {
	return fc.FuncType == funcType_ENV
}

func (fc *FuncConfig) IsTask() bool {
	switch fc.FuncType {
	case funcType_TASK_APPEND,
		funcType_TASK_GET,
		funcType_TASK_PUT,
		funcType_TASK_OTHER:
		return true
	default:
		return false
	}
}

type FuncTmplInput struct {
	*FuncConfig

	CFunc *MskFunction

	config *OutputConfig
}

func (t *FuncTmplInput) CName() string {
	return t.CFunc.Name
}

func (t *FuncTmplInput) GoParams() []string {
	var r []string
	i := 0
	if t.IsEnv() || t.IsTask() {
		i = 1
	}
	n := len(t.CFunc.Parameters)
	if t.LastParamOutput {
		n -= 1
	}

	for _, v := range t.params[i:n] {
		var s string
		switch {
		case v.IsPointer:
			s = fmt.Sprintf("%s *%s", v.Name, v.GoType)
		default:
			s = fmt.Sprintf("%s %s", v.Name, v.GoType)
		}
		r = append(r, s)
	}

	return r
}

func (t *FuncTmplInput) CCallInputs() []string {
	n := len(t.params)
	var r []string
	for i, pc := range t.params {
		var s string
		switch {
		case i == 0 && t.IsEnv():
			s = "env.getEnv()"
		case i == 0 && t.IsTask():
			s = "task.task"
		case i == n-1 && t.LastParamOutput:
			s = fmt.Sprintf("(*C.%s)(&%s)", pc.CgoType, pc.Name)
		case pc.OrigCType == "const char *":
			s = fmt.Sprintf("(*C.char)(unsafe.Pointer(%s))", pc.Name)
		case pc.OrigCType == "char *":
			s = fmt.Sprintf("(*C.char)(unsafe.Pointer(%s))", pc.Name)
		case pc.IsPointer:
			s = fmt.Sprintf("(*C.%s)(%s)", pc.CgoType, pc.Name)
		default:
			s = fmt.Sprintf("C.%s(%s)", pc.CgoType, pc.Name)
		}

		r = append(r, s)
	}

	return r
}

func (t *FuncTmplInput) CReturnMapped() string {
	goTypeForC, found := t.config.TypeToGoType[t.CFunc.ReturnType]
	if !found {
		log.Printf("cannot find mapping for return type %s", t.CFunc.ReturnType)
	}

	return goTypeForC
}

func (t *FuncTmplInput) ReturnType() string {
	var lastparam *ParamConfig
	if t.LastParamOutput {
		lastparam = t.params[len(t.params)-1]
		if !lastparam.IsPointer {
			log.Panicf("last output parameter: %s is not a pointer: %s", lastparam.Name, lastparam.OrigCType)
		}
	}

	if t.LastParamOutput && t.CFunc.ReturnType == "void" {
		return fmt.Sprintf("%s %s", lastparam.Name, lastparam.GoType)
	}

	goTypeForC, found := t.config.TypeToGoType[t.CFunc.ReturnType]
	if !found {
		log.Printf("cannot find mapping for return type %s", t.CFunc.ReturnType)
	}

	if t.LastParamOutput {
		return fmt.Sprintf("(r %s, %s %s)", goTypeForC, lastparam.Name, lastparam.GoType)
	}

	return goTypeForC
}

func normalizeFunction(f *MskFunction, config *OutputConfig) {
	fname := f.Name
	action, mid, suffix := GetFuncName(f.Name)
	canonName := fmt.Sprintf("%s%s%s", action, UpperCaseFirstLetter(mid), suffix)

	fc, found := config.Funcs[fname]

	if !found {
		fc = &FuncConfig{
			CommonId: &CommonId{
				GoName: canonName,
			},
		}
		config.Funcs[f.Name] = fc
	}
	if fc.Skip {
		return
	}
	if fc.GoName == "" {
		fc.GoName = fmt.Sprintf("%s%s%s", action, UpperCaseFirstLetter(mid), suffix)
	}

	IsTask := len(f.Parameters) >= 1 && f.Parameters[0].Type == "MSKtask_t"
	IsEnv := len(f.Parameters) >= 1 && f.Parameters[0].Type == "MSKenv_t"
	if IsEnv {
		fc.FuncType = funcType_ENV
	} else if IsTask {
		switch action {
		case "Put":
			fc.FuncType = funcType_TASK_PUT
		case "Get":
			fc.FuncType = funcType_TASK_GET
		case "Append":
			fc.FuncType = funcType_TASK_APPEND
		default:
			fc.FuncType = funcType_TASK_OTHER
		}
	}

	for i, p := range f.Parameters {
		pc := &ParamConfig{Name: p.Name, OrigCType: p.Type}
		switch {
		case i == 0 && IsEnv:
			pc.IsEnv = true
		case i == 0 && IsTask:
			pc.IsTask = true
		default:
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
		}

		fc.params = append(fc.params, pc)
	}
}
