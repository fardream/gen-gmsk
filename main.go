package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

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

func builderToFile(outputDir, outFile string, h *MosekH, config *OutputConfig, buildFunc func(*MosekH, *OutputConfig, io.Writer) error) {
	var fileContent bytes.Buffer
	orPanic(buildFunc(h, config, &fileContent))
	formattedContent, err := format.Source(fileContent.Bytes(), format.Options{
		LangVersion: "1.21",
		ExtraRules:  true,
		ModulePath:  "github.com/fardream/gmsk",
	})
	if err != nil {
		fullPath := path.Join(outputDir, outFile)
		fmt.Fprintf(os.Stderr, "failed to format %s due to %s", fullPath, err.Error())
		orPanic(os.WriteFile(fullPath, fileContent.Bytes(), 0o644))
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

	fileName := path.Join(homeDir, "mosek", "10.1", "tools", "platform", "linux64x86", "h", "mosek.h")
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

	config := newOutputConfig()

	orPanic(normalize(m, config))

	for _, enumName := range m.EnumList {
		if enumName == "MSKrescode_enum" {
			continue
		}
		enumData, ok := m.Enums[enumName]
		if !ok {
			log.Panicf("enum %s is not found in parsed mosek.h", enumName)
		}
		ec, found := config.Enums[enumName]
		if !found {
			log.Panicf("failed to find confing for enum: %s", enumName)
		}
		if ec.Skip {
			continue
		}
		fileName := fmt.Sprintf("%s.go", strings.TrimPrefix(enumName, "MSK"))
		builderToFile(outputDir, fileName, m, config, func(h *MosekH, config *OutputConfig, out io.Writer) error {
			return enumFileTmpl.Execute(out, &enumFileInput{
				enumConfig:  ec,
				CEnum:       enumData,
				PkgName:     "gmsk",
				stripPrefix: "MSK_",
			})
		})

	}

	builderToFile(outputDir, "rescodes.go", m, config, func(mh *MosekH, oc *OutputConfig, w io.Writer) error {
		rescodeEnum, ok := mh.Enums["MSKrescode_enum"]
		if !ok {
			return fmt.Errorf("failed to find MSKrescode_enum from parsed mosek header")
		}

		rc, found := config.Enums["MSKrescode_enum"]
		if !found {
			rc = &enumConfig{
				CommonId: CommonId{},
			}
		}
		rc.GoName = "ResCode"
		return enumFileTmpl.Execute(w, &enumFileInput{
			enumConfig:  rc,
			CEnum:       rescodeEnum,
			PkgName:     "gmsk",
			stripPrefix: "MSK_",
		})
	})

	for i := 0; i < int(funcType_LAST); i++ {
		t := funcType(i)
		builderToFile(outputDir, t.OutputFile(), m, config, func(mh *MosekH, oc *OutputConfig, w io.Writer) error {
			return BuildFuncs(mh, oc, t, w)
		})
	}

	log.Printf("number of functions: %d", len(m.Functions))
}
