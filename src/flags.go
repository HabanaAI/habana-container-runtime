package main

import (
	"os"
	"strings"
)

func parseBundle(osArgs []string) (string, error) {
	var bundleDir string
	for i, arg := range osArgs {
		f, val, ok := strings.Cut(arg, "=")
		if !isBundleFlag(f) {
			continue
		}
		if ok {
			bundleDir = val
		} else {
			bundleDir = osArgs[i+1]
		}
		break
	}

	if bundleDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		bundleDir = cwd
	}

	return bundleDir, nil
}

func isBundleFlag(arg string) bool {
	s := strings.TrimLeft(arg, "-")
	return s == "b" || s == "bundle"
}
