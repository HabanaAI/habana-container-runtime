/*
 * Copyright (c) 2021, HabanaLabs Ltd.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
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
