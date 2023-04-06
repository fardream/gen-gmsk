package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path"

	"github.com/go-clang/clang-v15/clang"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Panic(err)
	}

	fileName := path.Join(homeDir, "mosek", "10.0", "tools", "platform", "linux64x86", "h", "mosek.h")
	flag.StringVar(&fileName, "filename", fileName, "path to mosek.h")

	flag.Parse()

	idx := clang.NewIndex(0, 1)
	defer idx.Dispose()

	tu := idx.ParseTranslationUnit(fileName, nil, nil, 0)
	defer tu.Dispose()

	log.Printf("tu: %s\n", tu.Spelling())

	diagnostics := tu.Diagnostics()
	for _, d := range diagnostics {
		log.Println("PROBLEM:", d.Spelling())
	}

	cursor := tu.TranslationUnitCursor()

	log.Printf("cursor: %s\n", cursor.Spelling())
	log.Printf("cursor-kind: %s\n", cursor.Kind().Spelling())
	log.Printf("tu-fname: %s\n", tu.File(fileName).Name())

	m := NewMosekH().Build(cursor)

	if len(diagnostics) > 0 {
		log.Println("NOTE: There were problems while analyzing the given file")
	}

	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		log.Panic(err)
	}

	os.Stdout.Write(b)
	log.Printf("number of functions: %d", len(m.Functions))
	log.Printf(":: bye.\n")
}
