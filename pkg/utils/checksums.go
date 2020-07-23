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

// ComputeSecretCheckSum computes the sha256 checksum of secret data.
func ComputeSecretCheckSum(data map[string][]byte) string {
	var (
		hash string
		keys []string
	)

	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		hash += ComputeSHA256Hex(data[k])
	}

	return ComputeSHA256Hex([]byte(hash))
}

// ComputeChecksum computes a SHA256 checksum for the give map.
func ComputeChecksum(data interface{}) string {
	jsonString, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return ComputeSHA256Hex(jsonString)
}
