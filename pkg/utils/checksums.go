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

package utils

import (
	"encoding/json"
	"sort"
)

func computeChecksum(data map[string][]byte) string {
	var keys []string
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var hash string
	for _, k := range keys {
		hash += ComputeSHA256Hex(data[k])
	}

	return ComputeSHA256Hex([]byte(hash))
}

// ComputeSecretChecksum computes the sha256 checksum of secret data.
func ComputeSecretChecksum(data map[string][]byte) string {
	return computeChecksum(data)
}

// ComputeConfigMapChecksum computes the sha256 checksum of configmap data.
func ComputeConfigMapChecksum(data map[string]string) string {
	out := make(map[string][]byte, len(data))

	for k, v := range data {
		out[k] = []byte(v)
	}

	return computeChecksum(out)
}

// ComputeChecksum computes a SHA256 checksum for the give map.
func ComputeChecksum(data interface{}) string {
	jsonString, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return ComputeSHA256Hex(jsonString)
}
