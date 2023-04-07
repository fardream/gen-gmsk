package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
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

func builderToFile(outputDir, outFile string, m *MosekH, c *OutputConfig, buildFunc func(*MosekH, *OutputConfig, io.Writer) error) {
	var fileContent bytes.Buffer
	orPanic(buildFunc(m, c, &fileContent))
	formattedContent, err := format.Source(fileContent.Bytes(), format.Options{
		LangVersion: "1.20",
		ExtraRules:  true,
		ModulePath:  "github.com/fardream/gmsk",
	})
	if err != nil {
		os.Stdout.Write(fileContent.Bytes())
		orPanic(err)
	}

	if outputDir != "" {
		fullPath := path.Join(outputDir, outFile)
		orPanic(os.WriteFile(fullPath, formattedContent, 0o644))
	} else {
		os.Stdout.Write(formattedContent)
	}
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

	builderToFile(outputDir, "enums.go", m, config, BuildEnums)
	builderToFile(outputDir, path.Join("res", "codes.go"), m, config, BuildResCode)
	builderToFile(outputDir, "funcs.go", m, config, BuildFuncs)

	log.Printf("number of functions: %d", len(m.Functions))
}
