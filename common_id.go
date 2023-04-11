package main

import "strings"

type CommonId struct {
	GoName       string `json:"go_name"`
	Skip         bool   `json:"skip"`
	Comment      string `json:"comment"`
	IsDeprecated bool   `json:"is_deprecated"`
	Url          string `json:"url"`
}

func (c *CommonId) SplitComments() []string {
	if c.Comment == "" {
		return nil
	}
	return strings.Split(c.Comment, "\n")
}
