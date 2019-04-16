/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package utils

import "strings"

func IsEmptyString(s *string) bool {
	return s == nil || *s == ""
}

func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func Int64Value(v *int64, def int64) int64 {
	if v == nil {
		return def
	}
	return *v
}

func StringEqual(a, b *string) bool {
	return a == b || (a != nil && b != nil && *a == *b)
}
func IntEqual(a, b *int) bool {
	return a == b || (a != nil && b != nil && *a == *b)
}
func Int64Equal(a, b *int64) bool {
	return a == b || (a != nil && b != nil && *a == *b)
}

func Strings(s ...string) string {
	return "[" + strings.Join(s, ", ") + "]"
}
