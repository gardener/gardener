// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"encoding/json"
	"slices"
)

func computeChecksum(data map[string][]byte) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	slices.Sort(keys)

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

// ComputeChecksum computes a SHA256 checksum for the given data.
func ComputeChecksum(data any) string {
	jsonString, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return ComputeSHA256Hex(jsonString)
}
