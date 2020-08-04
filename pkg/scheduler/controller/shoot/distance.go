// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"strings"

	"github.com/texttheater/golang-levenshtein/levenshtein"
)

var orientations = []string{"north", "south", "east", "west", "central"}

// orientation extracts an orientation relative to a base from a region name
func orientation(name string) (normalized string, orientation string) {
	for _, o := range orientations {
		if i := strings.Index(name, o); i >= 0 {
			orientation = o
			normalized = name[:i] + ":" + name[i+len(o):]
			return
		}
	}
	return name, ""
}

// distance calculates a formal distance between two region names observing
// some usual orientation keywords. It is based on the levenshtein distance
// of the regions base names plus the difference based on the orientation.
// regions with the same base but different orientations are basically nearer
// to each other than two completely unrelated regions.
func distance(seed, shoot string) int {
	d, _ := distanceValues(seed, shoot)
	return d
}

func distanceValues(seed, shoot string) (int, int) {
	seedBase, seedOrient := orientation(seed)
	shootBase, shootOrient := orientation(shoot)
	dist := levenshtein.DistanceForStrings([]rune(seedBase), []rune(shootBase), levenshtein.DefaultOptionsWithSub)

	if seedOrient != "" || shootOrient != "" {
		if seedOrient == "" || shootOrient == "" {
			return dist*2 + 1, dist
		}
		if seedOrient == shootOrient {
			return dist * 2, dist
		}
		return dist*2 + 2, dist
	}
	return dist * 2, dist
}
