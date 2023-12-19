// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"net"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// github.com/gardener/gardener/pkg/client/kubernetes aliases
var (
	// NewClientFromSecretObject is an alias to kubernetes.NewClientFromSecretObject which allows it to be mocked for testing.
	NewClientFromSecretObject = kubernetes.NewClientFromSecretObject
)

// github.com/gardener/gardener/pkg/utils/gardener aliases
var (
	// ProjectForNamespaceFromReader is an alias to gardenerutils.ProjectForNamespaceFromReader which allows it to be mocked for testing.
	ProjectForNamespaceFromReader = gardenerutils.ProjectForNamespaceFromReader
)

// net aliases
var (
	// LookupHost is an alias to net.LookupHost which allows it to be mocked for testing.
	LookupHost = net.LookupHost
)
