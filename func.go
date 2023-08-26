package main

import (
	"fmt"
	"log"
	"strings"
)

type funcType uint

const (
	// first one
	funcType_NORMAL funcType = iota // other_funcs
	// env
	funcType_ENV // env
	// task
	funcType_TASK_PUT              // task_put
	funcType_TASK_GET              // task_get
	funcType_TASK_NAME             // task_name
	funcType_TASK_GETNUM           // task_getnum
	funcType_TASK_GETNUMNZ         // task_getnumnz
	funcType_TASK_SLICETRIP        // task_slicetrip
	funcType_TASK_APPEND           // task_append
	funcType_TASK_APPENDDOMAIN     // task_apenddomain
	funcType_TASK_GETLIST_OR_SLICE // task_getlist_or_slice
	funcType_TASK_PUTLIST_OR_SLICE // task_putlist_or_slice
	funcType_TASK_PUTMAXNUM        // task_putmaxnum
	funcType_TASK_OTHER            // task_other
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
	{"getmaxnum", "GetMaxNum"},
	{"putmaxnum", "PutMaxNum"},
	{"checkOut", "CheckOut"},
	{"evaluate", "Evaluate"},
	{"checkin", "CheckIn"},
	{"analyze", "Analyze"},
	{"getnum", "GetNum"},
	{"append", "Append"},
	{"unlink", "Unlink"},
	{"delete", "Delete"},
	{"remove", "Remove"},
	{"check", "Check"},
	{"empty", "Empty"},
	{"print", "Print"},
	{"write", "Write"},
	{"read", "Read"},
	{"make", "Make"},
	{"link", "Link"},
	{"get", "Get"},
	{"set", "Set"},
	{"put", "Put"},
}

var funcSuffix = []pair{
	{"blocktriplets", "BlockTriplets"},
	{"blocktriplet", "BlockTriplet"},
	{"sliceconst", "SliceConst"},
	{"slicetrip", "SliceTrip"},
	{"listconst", "ListConst"},
	{"summary", "Summary"},
	{"namelen", "NameLen"},
	{"numnz64", "NumNz64"},
	{"numnz", "NumNz"},
	{"tostr", "ToStr"},
	{"list64", "List64"},
	{"domain", "Domain"},
	{"slice", "Slice"},
	{"dotys", "DotYs"},
	{"doty", "DotY"},
	{"info", "Info"},
	{"name", "Name"},
	{"list", "List"},
	{"file", "File"},
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
	Name      string `json:"name"`        // name of the parameter
	OrigCType string `json:"orig_c_type"` // Original C type
	GoType    string `json:"go_type"`     // Mapped Go Type, without const and *
	CgoType   string `json:"cgo_type"`    // Mapped CgoType, without *
	IsPointer bool   `json:"is_pointer"`  // is pointer
	IsConst   bool   `json:"is_const"`    // is const
	IsTask    bool   `json:"is_task"`     // task, and first parameter
	IsEnv     bool   `json:"is_env"`      // env, and first parameter
	IsStrOut  bool   `json:"is_str_out"`  // char * type, is output string
	IsBoolOut bool   `json:"is_bool_out"` // bool * type, is output bool
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

func (t *FuncTmplInput) OutputStrings() []string {
	var r []string
	for i := len(t.params) - t.LastNParamOutput; i < len(t.params); i++ {
		if t.params[i].IsStrOut {
			r = append(r, t.params[i].Name)
		}
	}

	return r
}

func (t *FuncTmplInput) OutputBools() []string {
	var r []string
	for i := len(t.params) - t.LastNParamOutput; i < len(t.params); i++ {
		if t.params[i].IsBoolOut {
			r = append(r, t.params[i].Name)
		}
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
		case i < output_param_n && pc.OrigCType == "MSKbooleant":
			s = fmt.Sprintf("boolToInt(%s)", pc.Name)
		case pc.IsStrOut:
			s = fmt.Sprintf("c_%s", pc.Name)
		case pc.IsBoolOut:
			s = fmt.Sprintf("&c_%s", pc.Name)
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
	if t.CFunc.ReturnType == "MSKbooleant" {
		return "intToBool"
	}

	goTypeForC, found := t.config.TypeToGoType[t.CFunc.ReturnType]
	if !found {
		log.Printf("cannot find mapping for return type %s", t.CFunc.ReturnType)
	}

	return goTypeForC
}

func (t *FuncTmplInput) ReturnValueName() string {
	for _, v := range t.params {
		if v.Name == "r" {
			return "rescode"
		}
	}

	return "r"
}

func (t *FuncTmplInput) MapResToError() string {
	if t.CFunc.ReturnType == "MSKrescodee" {
		return ".ToError()"
	} else {
		return ""
	}
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
		if goTypeForC == "res.Code" {
			return "error"
		} else {
			return goTypeForC
		}
	}

	returnValeus := []string{}
	returnValueName := t.ReturnValueName()

	for _, v := range t.params[len(t.params)-t.LastNParamOutput:] {
		thisr := fmt.Sprintf("%s %s", v.Name, v.GoType)
		switch {
		case v.IsStrOut:
			thisr = fmt.Sprintf("%s string", v.Name)
		case v.IsBoolOut:
			thisr = fmt.Sprintf("%s bool", v.Name)
		}

		returnValeus = append(returnValeus, thisr)
	}

	return fmt.Sprintf("(%s, %s error)", strings.Join(returnValeus, ", "), returnValueName)
}

func replacePrefix(s, oldPrefix, newPrefix string) (bool, string) {
	if strings.HasPrefix(s, oldPrefix) {
		return true, fmt.Sprintf("%s%s", newPrefix, upperCaseFirstLetter(strings.TrimPrefix(s, oldPrefix)))
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
		s = upperCaseFirstLetter(s)
		return s
	default:
		for _, v := range funcVars {
			t, p := replacePrefix(mid, v.MskName, v.GoName)
			if t {
				return p
			}
		}
		return upperCaseFirstLetter(mid)
	}
}

func snakeToCamel(s string) string {
	b := strings.Builder{}
	for _, v := range strings.Split(s, "_") {
		fmt.Fprint(&b, upperCaseFirstLetter(v))
	}

	return b.String()
}

func processRustComment(rustcomment string, config *OutputConfig) string {
	r := make([]string, 0)
	for _, v := range strings.Split(rustcomment, "\n") {
		switch {
		case strings.Contains(v, "# Argument"):
			r = append(r, "\nArguments: ")
		case strings.Contains(v, "_`") && strings.HasPrefix(v, "- `"):
			r = append(r, fmt.Sprintf("  %s", strings.Replace(v, "_`", "`", len(v))))
		case strings.HasPrefix(v, "- `"):
			r = append(r, fmt.Sprintf("  %s", strings.Replace(v, "_`", "`", len(v))))
		case strings.Contains(v, "# Returns"):
			r = append(r, "\nReturns:")
		case strings.HasPrefix(v, "See ["):
			if len(r) > 0 && r[len(r)-1] == "" {
				r = r[:len(r)-1]
			}
			continue
		case strings.Contains(v, "Full documentation"):
			continue
		default:
			r = append(r, v)
		}
	}

	return strings.Join(r, "\n")
}

func setGoNameAndCommentFromRust(f *MskFunction, fc *FuncConfig, config *OutputConfig) {
	if fc.GoName != "" && fc.Comment != "" {
		return
	}

	rustfunc, found := config.mappedRustFuncs[f.Name]
	if !found {
		return
	}

	if fc.Comment == "" {
		fc.Comment = processRustComment(rustfunc.Comment, config)
	}

	if fc.GoName == "" {
		fc.GoName = snakeToCamel(rustfunc.Name)
	}
}

func normalizeFunction(f *MskFunction, config *OutputConfig) {
	fname := f.Name
	action, mid, suffix := splitFuncName(f.Name)
	canonName := fmt.Sprintf("%s%s%s", action, midName(action, mid, suffix), suffix)

	fc, found := config.Funcs[fname]
	if !found {
		fc = &FuncConfig{
			CommonId: &CommonId{},
		}
		config.Funcs[f.Name] = fc
	}

	if fc.Skip {
		return
	}

	setGoNameAndCommentFromRust(f, fc, config)

	if fc.GoName == "" {
		fc.GoName = canonName
	}
	if !fc.IsDeprecated {
		_, isdes := config.Deprecated[fname]
		fc.IsDeprecated = isdes
	}
	if fc.Url == "" {
		url, found := config.Urls[fname]
		if found {
			fc.Url = url
		} else {
			fc.Url = "https://docs.mosek.com/latest/capi/alphabetic-functionalities.html"
		}
	}

	IsTask := len(f.Parameters) >= 1 && f.Parameters[0].Type == "MSKtask_t"
	IsEnv := len(f.Parameters) >= 1 && f.Parameters[0].Type == "MSKenv_t"
	switch {
	case IsEnv:
		fc.FuncType = funcType_ENV
	case IsTask && action == "PutMaxNum":
		fc.FuncType = funcType_TASK_PUTMAXNUM
	case IsTask && suffix == "SliceTrip":
		fc.FuncType = funcType_TASK_SLICETRIP
	case IsTask && (suffix == "Name" || suffix == "NameLen"):
		fc.FuncType = funcType_TASK_NAME
	case IsTask && (suffix == "NumNz" || suffix == "NumNz64") && action == "Get":
		fc.FuncType = funcType_TASK_GETNUMNZ
	case IsTask && action == "Append" && suffix == "Domain":
		fc.FuncType = funcType_TASK_APPENDDOMAIN
	case IsTask && action == "Append":
		fc.FuncType = funcType_TASK_APPEND
	case IsTask && action == "Get" && (suffix == "List" ||
		suffix == "List64" || suffix == "Slice" || suffix == "SliceConst"):
		fc.FuncType = funcType_TASK_GETLIST_OR_SLICE
	case IsTask && action == "Get":
		fc.FuncType = funcType_TASK_GET
	case IsTask && action == "GetNum":
		fc.FuncType = funcType_TASK_GETNUM
	case IsTask && action == "Put" && (suffix == "List" ||
		suffix == "List64" || suffix == "Slice" || suffix == "SliceConst"):
		fc.FuncType = funcType_TASK_PUTLIST_OR_SLICE
	case IsTask && action == "Put":
		fc.FuncType = funcType_TASK_PUT
	case IsTask:
		fc.FuncType = funcType_TASK_OTHER
	default:
		fc.FuncType = funcType_NORMAL
	}

	if action == "GetNum" && fc.LastNParamOutput == 0 {
		fc.LastNParamOutput = 1
	}
	nparams := len(f.Parameters)
	last_n_params := nparams - fc.LastNParamOutput
	for i, p := range f.Parameters {
		pc := &ParamConfig{Name: p.Name, OrigCType: p.Type}
		switch {
		case i == 0 && IsEnv:
			pc.IsEnv = true

		case i == 0 && IsTask:
			pc.IsTask = true

		case i == nparams-1 && action == "GetMaxNum" && fc.LastNParamOutput == 0:
			fallthrough
		case i == nparams-1 && action == "GetNum" && fc.LastNParamOutput == 0:
			fallthrough
		case i == nparams-1 && fc.FuncType == funcType_TASK_APPENDDOMAIN && fc.LastNParamOutput == 0:
			fallthrough
		case i == nparams-1 && action == "Get" && (suffix == "NumNz" || suffix == "NumNz64") && fc.LastNParamOutput == 0:
			fallthrough
		case i == nparams-1 && action == "Get" && suffix == "NameLen" && fc.LastNParamOutput == 0:
			fc.LastNParamOutput = 1
			processParam(pc, p, config, f)

		case i == nparams-1 && action == "Get" && suffix == "Name" && fc.LastNParamOutput == 0:
			fc.LastNParamOutput = 1
			pc.IsStrOut = true

		case i == nparams-1 && action == "" && suffix == "ToStr" && fc.LastNParamOutput == 0 && p.Type == "char *":
			fc.LastNParamOutput = 1
			pc.IsStrOut = true

		case i >= last_n_params && p.Type == "char *":
			pc.IsStrOut = true

		case i >= last_n_params && p.Type == "MSKbooleant *":
			pc.IsBoolOut = true

		default:
			processParam(pc, p, config, f)
		}

		fc.params = append(fc.params, pc)
	}
}

func processParam(pc *ParamConfig, p ParamDecl, config *OutputConfig, f *MskFunction) {
	pc.IsPointer = strings.HasSuffix(p.Type, " *")
	ctype := strings.TrimSuffix(p.Type, " *")
	pc.IsConst = strings.HasPrefix(p.Type, "const ")
	ctype = strings.TrimPrefix(ctype, "const ")
	pc.CgoType = ctype
	found := false
	pc.GoType, found = config.TypeToGoType[ctype]
	if !found {
		log.Printf("cannot find func: %s", f.Name)
	}
}
