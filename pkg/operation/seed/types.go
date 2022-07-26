// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"
	"sync"
	"sync/atomic"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

// Builder is an object that builds Seed objects.
type Builder struct {
	seedObjectFunc func(context.Context) (*gardencorev1beta1.Seed, error)
}

// Seed is an object containing information about a Seed cluster.
type Seed struct {
	info      atomic.Value
	infoMutex sync.Mutex

	LoadBalancerServiceAnnotations map[string]string

	components *Components
}

// Components contains different components deployed in the Seed cluster.
type Components struct {
	dnsRecord component.DeployMigrateWaiter
}
