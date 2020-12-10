package utils

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ContextWithCancelChan returns a new context derived from the given parent
// that is canceled when the given chan receives
func ContextWithCancelChan(parent context.Context, done <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-done:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}

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
