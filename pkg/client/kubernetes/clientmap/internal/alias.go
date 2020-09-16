// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"net"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
)

// github.com/gardener/gardener/pkg/client/kubernetes aliases
var (
	// NewClientFromFile is an alias to kubernetes.NewClientFromFile which allows it to be mocked for testing.
	NewClientFromFile = kubernetes.NewClientFromFile
	// NewClientFromSecret is an alias to kubernetes.NewClientFromSecret which allows it to be mocked for testing.
	NewClientFromSecret = kubernetes.NewClientFromSecret
	// NewClientSetWithConfig is an alias to kubernetes.NewWithConfig which allows it to be mocked for testing.
	NewClientSetWithConfig = kubernetes.NewWithConfig
)

// github.com/gardener/gardener/pkg/operation/common aliases
var (
	// ProjectForNamespaceWithClient is an alias to common.ProjectForNamespaceWithClient which allows it to be mocked for testing.
	ProjectForNamespaceWithClient = common.ProjectForNamespaceWithClient
)

// net aliases
var (
	// LookupHost is an alias to net.LookupHost which allows it to be mocked for testing.
	LookupHost = net.LookupHost
)
