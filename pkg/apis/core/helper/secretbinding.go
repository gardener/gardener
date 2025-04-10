// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
)

// GetSecretBindingTypes returns the SecretBinding provider types.
func GetSecretBindingTypes(secretBinding *core.SecretBinding) []string {
	if secretBinding.Provider == nil {
		return []string{}
	}
	return strings.Split(secretBinding.Provider.Type, ",")
}
