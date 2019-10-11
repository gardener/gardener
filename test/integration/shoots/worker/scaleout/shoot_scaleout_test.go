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

package shootscaleout_test

import (
	"context"
	"flag"
	"fmt"
	"time"

	. "github.com/gardener/gardener/test/integration/shoots"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kubeconfig        = flag.String("kubecfg", "", "the path to the kubeconfig  of the garden cluster that will be used for integration tests")
	shootName         = flag.String("shoot-name", "", "the name of the shoot we want to test")
	shootNamespace    = flag.String("shoot-namespace", "", "the namespace name that the shoot resides in")
	logLevel          = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
)

const (
	UpdateKubernetesVersionTimeout = 600 * time.Second
	InitializationTimeout          = 600 * time.Second
	DumpStateTimeout               = 5 * time.Minute
)

func validateFlags() {
	if !StringSet(*shootName) {
		Fail("You should specify a shootName to test against")
	}

	if !StringSet(*kubeconfig) {
		Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(*kubeconfig) {
		Fail("kubeconfig path does not exist")
	}
}

var _ = Describe("Shoot update testing", func() {
	var (
		shootGardenerTest   *ShootGardenerTest
		shootTestOperations *GardenerTestOperation
		shootTestLogger     *logrus.Logger
	)

	CBeforeSuite(func(ctx context.Context) {
		// validate flags
		validateFlags()
		shootTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		var err error
		shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, nil, shootTestLogger)
		Expect(err).NotTo(HaveOccurred())

		shoot := &gardencorev1alpha1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
		shootTestOperations, err = NewGardenTestOperationWithShoot(ctx, shootGardenerTest.GardenClient, shootTestLogger, shoot)
		Expect(err).NotTo(HaveOccurred())

	}, InitializationTimeout)

	CAfterEach(func(ctx context.Context) {
		shootTestOperations.AfterEach(ctx)
	}, DumpStateTimeout)

	CIt("should add one machine to the worker pool", func(ctx context.Context) {
		if len(shootTestOperations.Shoot.Spec.Provider.Workers) == 0 {
			Skip("no workers defined")
		}

		shootTestOperations.Shoot.Spec.Provider.Workers[0].Minimum = shootTestOperations.Shoot.Spec.Provider.Workers[0].Minimum + 1
		if shootTestOperations.Shoot.Spec.Provider.Workers[0].Maximum < shootTestOperations.Shoot.Spec.Provider.Workers[0].Maximum {
			shootTestOperations.Shoot.Spec.Provider.Workers[0].Maximum = shootTestOperations.Shoot.Spec.Provider.Workers[0].Minimum
		}

		By(fmt.Sprintf("updating shoot worker to min of %d machines", shootTestOperations.Shoot.Spec.Provider.Workers[0].Minimum))
		_, err := shootGardenerTest.UpdateShoot(ctx, shootTestOperations.Shoot)
		Expect(err).ToNot(HaveOccurred())

	}, UpdateKubernetesVersionTimeout)

})
