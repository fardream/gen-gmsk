package main

import _ "embed"

//go:embed from-rust/funcs.yml
var rustFuncsBytes []byte

//go:embed from-rust/enums.yml
var rustEnumsBytes []byte

type RustEnumConstant struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
	Value   string `json:"value"`
}

type RustEnum struct {
	Name       string             `json:"name"`
	Comment    string             `json:"comment"`
	EnumConsts []RustEnumConstant `json:"enum_consts"`
}

type RustFunc struct {
	Name       string `json:"name"`
	Comment    string `json:"comment"`
	StructName string `json:"struct_name"`
}
