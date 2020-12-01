package utils

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func ParseHeaderLabels(h string) map[string]string {
	labels := map[string]string{}
	parts := strings.Split(h, ";")
	for _, p := range parts {
		kvparts := strings.Split(p, "=")
		if len(kvparts) != 2 {
			continue
		}
		k := strings.TrimSpace(kvparts[0])
		v := strings.TrimSpace(kvparts[1])
		labels[k] = v
	}
	return labels
}

func SwapFileExt(path string, ext string) string {
	oldExt := filepath.Ext(path)
	return fmt.Sprintf("%s%s", strings.TrimSuffix(path, oldExt), ext)
}

var normalizeFilenameRegex = regexp.MustCompile("[^-_.a-zA-Z0-9 ]")

func NormalizeFilename(name string) string {
	name = normalizeFilenameRegex.ReplaceAllString(name, "")
	if len(name) > 255 {
		// make sure filename is an acceptable length
		// 255 is a common limit
		return name[:255]
	}
	return name
}
