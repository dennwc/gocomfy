package classes

import (
	"strings"
	"unicode"
)

func GoNodeType(name string) string {
	return exportedName(formatName(name))
}

func GoLinkType(name string) string {
	return formatName(name)
}

func GoArgName(name string) string {
	name = strings.ToLower(name)
	switch name {
	case "type":
		return "typ"
	case "switch":
		return "sw"
	case "string":
		return "str"
	}
	name = formatName(name)
	return name
}

var nameReplacer = strings.NewReplacer(
	".", "_",
	":", "_",
	" ", "_",
	"(", "",
	")", "",
	"+", "Plus",
)

func formatName(name string) string {
	name = nameReplacer.Replace(name)
	if len(name) != 0 {
		if unicode.IsDigit(rune(name[0])) {
			name = "_" + name
		}
	}
	return name
}

func exportedName(name string) string {
	r := rune(name[0])
	if unicode.IsUpper(r) {
		return name
	}
	return string(unicode.ToUpper(r)) + name[1:]
}
