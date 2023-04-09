package main

import (
	"fmt"
	"log"
	"strings"
)

type funcType uint

const (
	funcType_NORMAL            funcType = iota // other_funcs
	funcType_ENV                               // env
	funcType_TASK_PUT                          // task_put
	funcType_TASK_GET                          // task_get
	funcType_TASK_GETNUM                       // task_getnum
	funcType_TASK_APPEND                       // task_append
	funcType_TASK_APPENDDOMAIN                 // task_apenddomain
	funcType_TASK_OTHER                        // task_other
	// this is the last so that we can iterate through the types
	funcType_LAST // task_last
)

func (t funcType) OutputFile() string {
	return fmt.Sprintf("%s.go", t.String())
}

type pair struct {
	MskName string
	GoName  string
}

var funcActions = []pair{
	{"checkOut", "CheckOut"},
	{"checkin", "CheckIn"},
	{"analyze", "Analyze"},
	{"getnum", "GetNum"},
	{"append", "Append"},
	{"unlink", "Unlink"},
	{"remove", "Remove"},
	{"check", "Check"},
	{"empty", "Empty"},
	{"print", "Print"},
	{"write", "Write"},
	{"read", "Read"},
	{"make", "Make"},
	{"link", "Link"},
	{"get", "Get"},
	{"put", "Put"},
}

var funcSuffix = []pair{
	{"blocktriplet", "BlockTriplet"},
	{"sliceconst", "SliceConst"},
	{"listconst", "ListConst"},
	{"domain", "Domain"},
	{"slice", "Slice"},
	{"list64", "List64"},
	{"name", "Name"},
	{"list", "List"},
	{"seq", "Seq"},
	{"new", "New"},
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

func splitFuncName(s string) (action string, mid string, suffix string) {
	var after string
	after, action = GetFuncAction(GetGoName(s))
	mid, suffix = GetFunctionSuffix(after)
	return
}

type ParamConfig struct {
	Name      string   `json:"name"`        // name of the parameter
	OrigCType string   `json:"orig_c_type"` // Original C type
	GoType    string   `json:"go_type"`     // Mapped Go Type, without const and *
	CgoType   string   `json:"cgo_type"`    // Mapped CgoType, without *
	IsPointer bool     `json:"is_pointer"`  // is pointer
	IsConst   bool     `json:"is_const"`    // is const
	IsTask    bool     `json:"is_task"`     // task, and first parameter
	IsEnv     bool     `json:"is_env"`      // env, and first parameter
	PkgsUsed  []string `json:"pkgs_used"`   // packaged used
}

type FuncConfig struct {
	*CommonId `json:",inline"`

	LastNParamOutput int      `json:"last_n_param_output"`
	FuncType         funcType `json:"func_type"`

	params []*ParamConfig
}

func (fc *FuncConfig) IsEnv() bool {
	return fc.FuncType == funcType_ENV
}

func (fc *FuncConfig) IsTask() bool {
	i := int(fc.FuncType)
	return i > int(funcType_ENV) && i < int(funcType_LAST)
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
	n := len(t.CFunc.Parameters) - t.LastNParamOutput

	for _, v := range t.params[i:n] {
		var s string
		switch {
		case v.OrigCType == "const char *":
			s = fmt.Sprintf("%s string", v.Name)
		case v.IsPointer:
			s = fmt.Sprintf("%s *%s", v.Name, v.GoType)
		default:
			s = fmt.Sprintf("%s %s", v.Name, v.GoType)
		}
		r = append(r, s)
	}

	return r
}

func (t *FuncTmplInput) ExtraStdPkgs() []string {
	pkgs := make(map[string]struct{})
	for _, pc := range t.params {
		if pc.OrigCType == "const char *" || pc.OrigCType == "char *" {
			pkgs["unsafe"] = struct{}{}
		}
	}

	return keys(pkgs)
}

func keys[T any](m map[string]T) []string {
	var r []string
	for k := range m {
		r = append(r, k)
	}

	return r
}

func (t *FuncTmplInput) InputStrings() []*ParamConfig {
	var r []*ParamConfig
	for _, p := range t.params {
		if p.OrigCType == "const char *" {
			r = append(r, p)
		}
	}

	return r
}

// CCallInputs are the inputs to C function calls from cgo.
func (t *FuncTmplInput) CCallInputs() []string {
	n := len(t.params)
	var r []string
	output_param_n := n - t.LastNParamOutput
	for i, pc := range t.params {
		var s string
		switch {
		case i == 0 && t.IsEnv():
			s = "env.getEnv()"
		case i == 0 && t.IsTask():
			s = "task.task"
		case i >= output_param_n:
			s = fmt.Sprintf("(*C.%s)(&%s)", pc.CgoType, pc.Name)
		case pc.OrigCType == "const char *":
			s = fmt.Sprintf("c_%s", pc.Name)
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
	if t.CFunc.ReturnType == "void" && t.LastNParamOutput == 0 {
		return ""
	}
	goTypeForC, found := t.config.TypeToGoType[t.CFunc.ReturnType]
	if !found {
		log.Panicf("cannot find mapping for return type %s", t.CFunc.ReturnType)
	}
	if t.LastNParamOutput == 0 {
		return goTypeForC
	}

	returnValeus := []string{fmt.Sprintf("r %s", goTypeForC)}
	for _, v := range t.params[(len(t.params) - t.LastNParamOutput):] {
		returnValeus = append(returnValeus, fmt.Sprintf("%s %s", v.Name, v.GoType))
	}

	return fmt.Sprintf("(%s)", strings.Join(returnValeus, ", "))
}

func replacePrefix(s, oldPrefix, newPrefix string) (bool, string) {
	if strings.HasPrefix(s, oldPrefix) {
		return true, fmt.Sprintf("%s%s", newPrefix, UpperCaseFirstLetter(strings.TrimPrefix(s, oldPrefix)))
	}
	return false, s
}

func replaceSuffix(s, oldSuffix, newSuffix string) (bool, string) {
	if strings.HasSuffix(s, oldSuffix) {
		return true, fmt.Sprintf("%s%s", strings.TrimSuffix(s, oldSuffix), newSuffix)
	}
	return false, s
}

func midName(action, mid, suffix string) string {
	switch {
	case action == "Append" && suffix == "Domain":
		s := mid
		_, s = replacePrefix(s, "primal", "Primal")
		_, s = replacePrefix(s, "dual", "Dual")
		_, s = replaceSuffix(s, "cone", "Cone")
		_, s = replacePrefix(s, "r", "R")
		s = UpperCaseFirstLetter(s)
		return s
	default:
		for _, v := range funcVars {
			t, p := replacePrefix(mid, v.MskName, v.GoName)
			if t {
				return p
			}
		}
		return UpperCaseFirstLetter(mid)
	}
}

func normalizeFunction(f *MskFunction, config *OutputConfig) {
	fname := f.Name
	action, mid, suffix := splitFuncName(f.Name)
	canonName := fmt.Sprintf("%s%s%s", action, midName(action, mid, suffix), suffix)

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
		fc.GoName = canonName
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
		case "GetNum":
			fc.FuncType = funcType_TASK_GETNUM
		case "Append":
			if suffix == "Domain" {
				fc.FuncType = funcType_TASK_APPENDDOMAIN
				fc.LastNParamOutput = 1
			} else {
				fc.FuncType = funcType_TASK_APPEND
			}
		default:
			fc.FuncType = funcType_TASK_OTHER
		}
	}

	if action == "GetNum" && fc.LastNParamOutput == 0 {
		fc.LastNParamOutput = 1
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
