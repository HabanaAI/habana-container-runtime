/*
 * Copyright (c) 2022, HabanaLabs Ltd.  All rights reserved.
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
	"log"
)

func capabilityToCLI(cap string) string {
	switch cap {
	case "compute":
		return "--compute"
	case "compat32":
		return "--compat32"
	case "graphics":
		return "--graphics"
	case "utility":
		return "--utility"
	case "video":
		return "--video"
	case "display":
		return "--display"
	case "ngx":
		return "--ngx"
	default:
		log.Panicln("unknown driver capability:", cap)
	}
	return ""
}
