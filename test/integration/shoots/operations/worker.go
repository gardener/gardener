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

	Test: Shoot nodes should have different Image Versions
	Expected Output
		- Shoot has nodes with the machine image names specified in the shoot spec (node.Status.NodeInfo.OSImage)

 **/

package operations

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/test/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/onsi/ginkgo"
	g "github.com/onsi/gomega"
)

const (
	scaleWorkerTimeout = 15 * time.Minute
)

var _ = ginkgo.Describe("Shoot worker operation testing", func() {

	f := framework.NewShootFramework(nil)

	f.Beta().Serial().CIt("should add one machine to the worker pool and remove it again", func(ctx context.Context) {
		shoot := f.Shoot
		if len(shoot.Spec.Provider.Workers) == 0 {
			ginkgo.Skip("no workers defined")
		}
		var (
			min = shoot.Spec.Provider.Workers[0].Minimum + 1
			max = shoot.Spec.Provider.Workers[0].Maximum
		)
		if shoot.Spec.Provider.Workers[0].Maximum < min {
			max = min
		}

		ginkgo.By(fmt.Sprintf("updating shoot worker to min of %d machines", min))
		err := f.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Spec.Provider.Workers[0].Minimum = min
			shoot.Spec.Provider.Workers[0].Maximum = max
			return nil
		})
		framework.ExpectNoError(err)

		ginkgo.By("scale down worker")

		min = shoot.Spec.Provider.Workers[0].Minimum - 1
		max = shoot.Spec.Provider.Workers[0].Maximum - 1

		ginkgo.By(fmt.Sprintf("updating shoot worker to min of %d machines", min))
		err = f.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Spec.Provider.Workers[0].Minimum = min
			shoot.Spec.Provider.Workers[0].Maximum = max
			return nil
		})
		framework.ExpectNoError(err)

	}, scaleWorkerTimeout)

	f.Beta().CIt("Shoot node's operating systems should differ if the the specified workers are different", func(ctx context.Context) {
		ginkgo.By("Checking if shoot is compatible for testing")

		if len(f.Shoot.Spec.Provider.Workers) >= 1 {
			ginkgo.Skip("the test requires at least 2 worker groups")
		}

		workerImages := map[string]bool{}
		for _, worker := range f.Shoot.Spec.Provider.Workers {
			if worker.Minimum == 0 {
				continue
			}
			imageName := fmt.Sprintf("%s/%s", worker.Machine.Image.Name, *worker.Machine.Image.Version)
			if _, ok := workerImages[imageName]; !ok {
				workerImages[imageName] = true
			}
		}

		if len(workerImages) >= 1 {
			ginkgo.Skip("the test requires at least 2 different worker os images")
		}

		nodesList, err := f.ShootClient.Kubernetes().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err)

		nodeImages := map[string]bool{}
		for _, node := range nodesList.Items {
			if _, ok := nodeImages[node.Status.NodeInfo.OSImage]; !ok {
				nodeImages[node.Status.NodeInfo.OSImage] = true
			}
		}

		g.Expect(nodeImages).To(g.HaveLen(len(workerImages)))
	}, 1*time.Minute)

})
