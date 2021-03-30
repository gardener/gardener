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

package internal

import (
	"net"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
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

// github.com/gardener/gardener/pkg/utils/gardener aliases
var (
	// ProjectForNamespaceFromReader is an alias to gutil.ProjectForNamespaceFromReader which allows it to be mocked for testing.
	ProjectForNamespaceFromReader = gutil.ProjectForNamespaceFromReader
)

// net aliases
var (
	// LookupHost is an alias to net.LookupHost which allows it to be mocked for testing.
	LookupHost = net.LookupHost
)
