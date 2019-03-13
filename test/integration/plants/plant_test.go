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

package plants_test

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/test/integration/shoots"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kubeconfig        = flag.String("kubeconfig", "", "the path to the kubeconfig  of the garden cluster that will be used for integration tests")
	shootName         = flag.String("shootName", "", "the name of the shoot we want to test")
	shootNamespace    = flag.String("shootNamespace", "", "the namespace name that the shoot resides in")
	testShootsPrefix  = flag.String("prefix", "", "prefix to use for test shoots")
	logLevel          = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	shootTestYamlPath = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
	plantTestYamlPath = flag.String("plantpath", "", "the path to the shoot yaml that will be used for testing")
	cleanup           = flag.Bool("cleanup", false, "deletes the newly created / existing test shoot after the test suite is done")
)

const (
	PlantUpdateSecretTimeout = 90 * time.Second
	PlantCreationTimeout     = 60 * time.Second
	InitializationTimeout    = 20 * time.Second
	FinalizationTimeout      = 20 * time.Second
)

func validateFlags() {
	if StringSet(*shootTestYamlPath) && StringSet(*shootName) {
		Fail("You can set either the shoot YAML path or specify a shootName to test against")
	}

	if !StringSet(*shootTestYamlPath) && !StringSet(*shootName) {
		Fail("You should either set the shoot YAML path or specify a shootName to test against")
	}

	if StringSet(*shootTestYamlPath) {
		if !FileExists(*shootTestYamlPath) {
			Fail("shoot yaml path is set but invalid")
		}
	}

	if !StringSet(*plantTestYamlPath) {
		Fail("You need to set the YAML path to the Plant that should be created")
	} else {
		if !FileExists(*plantTestYamlPath) {
			Fail("plant yaml path is set but invalid")
		}
	}

	if !StringSet(*kubeconfig) {
		Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(*kubeconfig) {
		Fail("kubeconfig path does not exist")
	}
}

var _ = Describe("Plant testing", func() {
	var (
		shootGardenerTest *ShootGardenerTest
		plantTest         *PlantTest
		plantTestLogger   *logrus.Logger
		targetTestShoot   *v1beta1.Shoot
		targetTestPlant   *gardencorev1alpha1.Plant
		rememberSecret    *v1.Secret
	)

	CBeforeSuite(func(ctx context.Context) {
		// validate flags
		validateFlags()
		plantTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		// check if a shoot spec is provided, if yes create a shoot object from it and use it for testing
		if StringSet(*shootTestYamlPath) {
			*cleanup = true
			// parse shoot yaml into shoot object and generate random test names for shoots
			_, shootObject, err := CreateShootTestArtifacts(*shootTestYamlPath, *testShootsPrefix)
			Expect(err).NotTo(HaveOccurred())

			shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, shootObject, plantTestLogger)
			Expect(err).NotTo(HaveOccurred())

			targetTestShoot, err = shootGardenerTest.CreateShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
		}

		if StringSet(*shootName) {
			var err error
			shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, nil, plantTestLogger)
			Expect(err).NotTo(HaveOccurred())

			targetTestShoot = &v1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
			if err := shootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: targetTestShoot.Namespace, Name: targetTestShoot.Name}, targetTestShoot); err != nil {
				Expect(err).NotTo(HaveOccurred())
			}
		}

		if StringSet(*plantTestYamlPath) {
			// parse plant yaml into plant object and generate random test names for plants
			plantObject, err := CreatePlantTestArtifacts(*plantTestYamlPath)
			Expect(err).NotTo(HaveOccurred())

			plantTest, err = NewPlantTest(*kubeconfig, plantObject, targetTestShoot, plantTestLogger)
			Expect(err).NotTo(HaveOccurred())
		}

	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		// Clean up shoot
		By("Cleaning up plant resources")

		if *cleanup {
			By("Cleaning up test plant and secret")

			err := plantTest.DeletePlant(ctx)
			Expect(err).NotTo(HaveOccurred())

			err = plantTest.DeletePlantSecret(ctx)
			Expect(err).NotTo(HaveOccurred())
		}
	}, FinalizationTimeout)

	CIt("Should create plant successfully", func(ctx context.Context) {

		By(fmt.Sprintf("Create Plant secret from shoot secret"))

		err := plantTest.CreatePlantSecret(ctx)
		Expect(err).NotTo(HaveOccurred())
		*cleanup = true
		rememberSecret = plantTest.PlantSecret.DeepCopy()

		By(fmt.Sprintf("Create Plant"))

		targetTestPlant, err = plantTest.CreatePlant(ctx)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Check if created plant has successful status. Name of plant: %s", targetTestPlant.Name))

		plantTest.WaitForPlantToBeReconciledSuccessfully(ctx)

		Expect(err).NotTo(HaveOccurred())
	}, PlantCreationTimeout)

	CIt("Should update Plant Status to 'unknown' due to updated and invalid Plant Secret (kubeconfig invalid)", func(ctx context.Context) {

		secretToUpdate := plantTest.PlantSecret

		if secretToUpdate == nil {
			plantSecret, err := plantTest.GetPlantSecret(ctx)

			if err != nil {
				Fail("Cannot retrieve Plant Secret")
			}
			secretToUpdate = plantSecret
		}

		// modify data.kubeconfig to update the secret with false information
		dummyKubeconfigContent := "Here is a string...."
		source := []byte(dummyKubeconfigContent)
		base64DummyKubeconfig := base64.StdEncoding.EncodeToString(source)
		secretToUpdate.Data["kubeconfig"] = []byte(base64DummyKubeconfig)

		By(fmt.Sprintf("Update Plant secret with invalid kubeconfig"))

		err := plantTest.UpdatePlantSecret(ctx, secretToUpdate)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Wait for PlantController to update to status 'unknown'"))

		err = plantTest.WaitForPlantToBeReconciledWithUnknownStatus(ctx)
		Expect(err).NotTo(HaveOccurred())
	}, PlantUpdateSecretTimeout)

	CIt("Should reconcile Plant Status to be successful after Plant Secret update", func(ctx context.Context) {

		Expect(rememberSecret.Data).NotTo(Equal(nil))

		plantTest.PlantSecret.Data["kubeconfig"] = rememberSecret.Data["kubeconfig"]
		// Update secret again to contain valid kubeconfig
		err := plantTest.UpdatePlantSecret(ctx, plantTest.PlantSecret)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Plant secret updated to use shoot secret"))

		plantTestLogger.Debugf("Checking if created plant has successful status. Name of plant: %s", targetTestPlant.Name)

		plantTest.WaitForPlantToBeReconciledSuccessfully(ctx)

		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Plant reconciled successfully"))

	}, PlantUpdateSecretTimeout)
	CIt("Should update Plant Status to 'unknown' due to updated and invalid Plant Secret (kubeconfig not provided)", func(ctx context.Context) {

		secretToUpdate := plantTest.PlantSecret

		if secretToUpdate == nil {
			plantSecret, err := plantTest.GetPlantSecret(ctx)

			if err != nil {
				Fail("Cannot retrieve Plant Secret")
			}
			secretToUpdate = plantSecret
		}

		// remove data.kubeconfig to update the secret with false information
		secretToUpdate.Data = nil

		err := plantTest.UpdatePlantSecret(ctx, secretToUpdate)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Wait for PlantController to update to status 'unknown'"))

		err = plantTest.WaitForPlantToBeReconciledWithUnknownStatus(ctx)
		Expect(err).NotTo(HaveOccurred())

	}, PlantUpdateSecretTimeout)
})
