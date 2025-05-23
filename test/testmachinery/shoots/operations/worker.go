// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

	Test: Shoot nodes should have different Image Versions
	Expected Output
		- Shoot has nodes with the machine image names specified in the shoot spec (node.Status.NodeInfo.OSImage)

 **/

package operations

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/test/framework"
)

const (
	scaleWorkerTimeout = 15 * time.Minute
)

var _ = ginkgo.Describe("Shoot worker operation testing", func() {

	f := framework.NewShootFramework(nil)

	f.Default().Serial().CIt("should add one machine to the worker pool and remove it again", func(ctx context.Context) {
		shoot := f.Shoot
		if v1beta1helper.IsWorkerless(shoot) {
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

		ginkgo.By("Scale down worker")

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

	f.Beta().CIt("Shoot node's operating systems should differ if the specified workers are different", func(ctx context.Context) {
		ginkgo.By("Check if shoot is compatible for testing")

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

		nodeList := &corev1.NodeList{}
		err := f.ShootClient.Client().List(ctx, nodeList)
		framework.ExpectNoError(err)

		nodeImages := map[string]bool{}
		for _, node := range nodeList.Items {
			if _, ok := nodeImages[node.Status.NodeInfo.OSImage]; !ok {
				nodeImages[node.Status.NodeInfo.OSImage] = true
			}
		}

		g.Expect(nodeImages).To(g.HaveLen(len(workerImages)))
	}, 1*time.Minute)

})
