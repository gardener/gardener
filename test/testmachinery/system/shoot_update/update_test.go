// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

/**
	Overview
		- Tests the update of a Shoot's Kubernetes version to the next minor version

	Prerequisites
		- A Shoot exists.

	Test: Update the Shoot's Kubernetes version to the next minor version
	Expected Output
		- Successful reconciliation of the Shoot after the Kubernetes Version update.
 **/

package shootupdate

import (
	"context"
	"flag"
	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/test/framework"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
)

var (
	newControlPlaneKubernetesVersion = flag.String("version", "", "the version to use for .spec.kubernetes.version and .spec.provider.workers[].kubernetes.version (only when nil or equal to .spec.kubernetes.version)")
	newWorkerPoolKubernetesVersion   = flag.String("version-worker-pools", "", "the version to use for .spec.provider.workers[].kubernetes.version (only when not equal to .spec.kubernetes.version)")
)

const UpdateKubernetesVersionTimeout = 45 * time.Minute

func init() {
	framework.RegisterShootFrameworkFlags()
}

var _ = Describe("Shoot update testing", func() {
	f := framework.NewShootFramework(nil)

	framework.CIt("should update the kubernetes version of the shoot and its worker pools to the respective next versions", func(ctx context.Context) {
		shootupdatesuite.RunTest(ctx, f, newControlPlaneKubernetesVersion, newWorkerPoolKubernetesVersion)
	}, UpdateKubernetesVersionTimeout)
})
