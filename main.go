package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path"

	"github.com/go-clang/clang-v15/clang"
	"mvdan.cc/gofumpt/format"
)

func orPanic(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func getOrPanic[T any](a T, err error) T {
	orPanic(err)
	return a
}

func main() {
	homeDir := getOrPanic(os.UserHomeDir())

	fileName := path.Join(homeDir, "mosek", "10.0", "tools", "platform", "linux64x86", "h", "mosek.h")
	flag.StringVar(&fileName, "filename", fileName, "path to mosek.h")

	outputFile := ""
	flag.StringVar(&outputFile, "output", outputFile, "dump mosek header parsed into a json")

	outputDir := ""
	flag.StringVar(&outputDir, "gmsk-dir", outputDir, "gmsk package dir to output the code file to")

	flag.Parse()

	idx := clang.NewIndex(0, 1)
	defer idx.Dispose()

	tu := idx.ParseTranslationUnit(fileName, nil, nil, 0)
	defer tu.Dispose()

	diagnostics := tu.Diagnostics()
	for _, d := range diagnostics {
		log.Println("PROBLEM:", d.Spelling())
	}

	cursor := tu.TranslationUnitCursor()

	m := NewMosekH().Build(cursor)

	if len(diagnostics) > 0 {
		log.Println("NOTE: There were problems while analyzing the given file")
	}

	if outputFile != "" {
		b := getOrPanic(json.MarshalIndent(m, "", "  "))

		orPanic(os.WriteFile(outputFile, b, 0o644))
	}

	config := NewOutputConfig()

	orPanic(Normalize(m, config))

	var enumFile bytes.Buffer

	orPanic(BuildEnums(m, config, &enumFile))

	fmtOpts := format.Options{
		LangVersion: "1.20",
		ExtraRules:  true,
		ModulePath:  "github.com/fardream/gmsk",
	}
	formattedEnum := getOrPanic(format.Source(enumFile.Bytes(), fmtOpts))

	if outputDir != "" {
		enumOut := path.Join(outputDir, "enums.go")
		orPanic(os.WriteFile(enumOut, formattedEnum, 0o644))
	} else {
		os.Stdout.Write(formattedEnum)
	}

	var resFile bytes.Buffer
	orPanic(BuildResCode(m, config, &resFile))

	formattedRes := getOrPanic(format.Source(resFile.Bytes(), fmtOpts))

	if outputDir != "" {
		resOut := path.Join(outputDir, "res", "codes.go")
		orPanic(os.WriteFile(resOut, formattedRes, 0o644))
	} else {
		os.Stdout.Write(formattedRes)
	}

	log.Printf("number of functions: %d", len(m.Functions))
}
