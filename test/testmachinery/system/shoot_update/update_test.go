// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the update of a Shoot's Kubernetes version to the next minor version

	Prerequisites
		- A Shoot exists.

	Test: Update the Shoot's Kubernetes version to the next minor version
	Expected Output
		- Successful reconciliation of the Shoot after the Kubernetes Version update.
 **/

package shootupdate_test

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
