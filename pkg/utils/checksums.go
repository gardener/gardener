// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
