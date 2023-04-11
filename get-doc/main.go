package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/gocolly/colly/v2"
)

func writeYaml(name string, d any) {
	b, err := yaml.Marshal(d)
	if err != nil {
		log.Panic(err)
	}

	os.WriteFile(name, b, 0o644)
}

func main() {
	const url = "https://docs.mosek.com/latest/capi/alphabetic-functionalities.html"
	urls := make(map[string]string)
	deprecated := make(map[string]struct{}, 0)
	c := colly.NewCollector()
	c.OnHTML("dl.function.msk", func(h *colly.HTMLElement) {
		fName := h.ChildText("span.sig-name>span.pre")
		flink := h.ChildAttr("a.headerlink", "href")
		if strings.HasPrefix(flink, "#") {
			urls[fName] = fmt.Sprintf("%s%s", url, flink)
		}
		if strings.Contains(h.Text, "Deprecated") {
			deprecated[fName] = struct{}{}
		}
	})

	c.Visit(url)

	writeYaml("urls.yml", urls)
	writeYaml("deprecated.yml", deprecated)
}
