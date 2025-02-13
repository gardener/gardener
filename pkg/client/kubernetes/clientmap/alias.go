// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clientmap

import (
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// github.com/gardener/gardener/pkg/client/kubernetes aliases
var (
	// NewClientFromSecretObject is an alias to kubernetes.NewClientFromSecretObject which allows it to be mocked for testing.
	NewClientFromSecretObject = kubernetes.NewClientFromSecretObject
)
