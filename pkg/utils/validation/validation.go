// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

// ShouldEnforceImmutability compares the given slices and returns if a immutability should be enforced.
// It mainly checks if the order of the same elements in `new` and `old` is the same, i.e. only an addition
// of elements to `new` is allowed.
func ShouldEnforceImmutability(new, old []string) bool {
	sizeDelta := len(new) - len(old)
	if sizeDelta > 0 {
		newA := new[:len(new)-sizeDelta]
		if equal(newA, old) {
			return false
		}
		return ShouldEnforceImmutability(newA, old)
	}
	return sizeDelta < 0 || sizeDelta == 0
}

func equal(new, old []string) bool {
	for i := range new {
		if new[i] != old[i] {
			return false
		}
	}
	return true
}
