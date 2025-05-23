// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the Care Controller in the Gardenlet

	Prerequisite
		- Shoot Cluster with  Condition "APIServerAvailable" equals true

	Test: Scale down API Server deployment of the Shoot in the Seed
	Expected Output
		- Shoot Condition "APIServerAvailable" becomes unhealthy
 **/

package care

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/test/framework"
)

const (
	timeout = 10 * time.Minute
)

var _ = ginkgo.Describe("Shoot Care testing", func() {
	var (
		f            = framework.NewShootFramework(nil)
		origReplicas *int32
		err          error
	)

	f.Default().Serial().CIt("Should observe failed health condition in the Shoot when scaling down the API Server of the Shoot", func(ctx context.Context) {
		cond := helper.GetCondition(f.Shoot.Status.Conditions, gardencorev1beta1.ShootAPIServerAvailable)
		gomega.Expect(cond).ToNot(gomega.BeNil())
		gomega.Expect(cond.Status).To(gomega.Equal(gardencorev1beta1.ConditionTrue))

		zero := int32(0)
		origReplicas, err = framework.ScaleDeployment(ctx, f.SeedClient.Client(), &zero, v1beta1constants.DeploymentNameKubeAPIServer, f.ShootSeedNamespace())
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// wait for unhealthy condition
		err = f.WaitForShootCondition(ctx, 20*time.Second, 5*time.Minute, gardencorev1beta1.ShootAPIServerAvailable, gardencorev1beta1.ConditionFalse)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}, timeout, framework.WithCAfterTest(func(ctx context.Context) {
		if origReplicas != nil {
			f.Logger.Info("Test cleanup, scale kube-apiserver", "replicas", *origReplicas)
			origReplicas, err = framework.ScaleDeployment(ctx, f.SeedClient.Client(), origReplicas, v1beta1constants.DeploymentNameKubeAPIServer, f.ShootSeedNamespace())
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// wait for healthy condition
			f.Logger.Info("Test cleanup, waiting for shoot health condition to be become healthy", "conditionType", gardencorev1beta1.ShootAPIServerAvailable)
			err = f.WaitForShootCondition(ctx, 20*time.Second, 5*time.Minute, gardencorev1beta1.ShootAPIServerAvailable, gardencorev1beta1.ConditionTrue)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			f.Logger.Info("Test cleanup successful")
		}
	}, timeout))
})
