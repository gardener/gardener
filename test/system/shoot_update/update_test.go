// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
