// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
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

package shootupdate

import (
	"context"
	"flag"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var kubernetesVersion = flag.String("version", "", "the version to update the shoot")

const UpdateKubernetesVersionTimeout = 45 * time.Minute

func init() {
	framework.RegisterShootFrameworkFlags()
}

var _ = Describe("Shoot update testing", func() {

	f := framework.NewShootFramework(nil)

	framework.CIt("should update the kubernetes version of the shoot to the next version", func(ctx context.Context) {
		currentVersion := f.Shoot.Spec.Kubernetes.Version
		newVersion := *kubernetesVersion
		if currentVersion == newVersion {
			Skip("shoot already has the desired kubernetes version")
		}
		if newVersion == "" {
			var (
				err                       error
				ok                        bool
				consecutiveMinorAvailable bool
			)
			cloudprofile, err := f.GetCloudProfile(ctx)
			Expect(err).ToNot(HaveOccurred())
			consecutiveMinorAvailable, newVersion, err = gardencorev1beta1helper.GetKubernetesVersionForMinorUpdate(cloudprofile, currentVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(consecutiveMinorAvailable).To(BeTrue())
			if !ok {
				Skip("no new version found")
			}
		}

		By(fmt.Sprintf("updating shoot %s to version %s", f.Shoot.GetName(), newVersion))
		err := f.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Spec.Kubernetes.Version = newVersion
			return nil
		})
		Expect(err).ToNot(HaveOccurred())

	}, UpdateKubernetesVersionTimeout)

})
