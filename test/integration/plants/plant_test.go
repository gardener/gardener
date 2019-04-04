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
	"io/ioutil"
	"time"

	"k8s.io/api/core/v1"

	. "github.com/gardener/gardener/test/integration/shoots"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var (
	kubeconfigPath                = flag.String("kubeconfig-path", "", "the path to the kubeconfig path  of the garden cluster that will be used for integration tests")
	kubeconfigPathExternalCluster = flag.String("kubeconfig-path-externalcluster", "", "the path to the kubeconfig path  of the external cluster that will be registered as a plant")
	logLevel                      = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	plantTestYamlPath             = flag.String("plant-path", "", "the path to the plant yaml that will be used for testing")
	plantTestNamespace            = flag.String("plant-test-namespace", "", "the namespace where the plant will be created")
)

const (
	PlantUpdateSecretTimeout = 90 * time.Second
	PlantCreationTimeout     = 60 * time.Second
	InitializationTimeout    = 20 * time.Second

	KubeConfigKey = "kubeconfig"
)

func validateFlags() {
	if !StringSet(*plantTestYamlPath) {
		Fail("You need to set the YAML path to the Plant that should be created")
	} else {
		if !FileExists(*plantTestYamlPath) {
			Fail("plant yaml path is set but invalid")
		}
	}

	if !StringSet(*plantTestNamespace) {
		Fail("you need to specify the namespace where the plant will be created")
	}

	if !StringSet(*kubeconfigPath) {
		Fail("you need to specify the correct path for the kubeconfigpath")
	}

	if !FileExists(*kubeconfigPath) {
		Fail("kubeconfigpath path does not exist")
	}

	if !StringSet(*kubeconfigPathExternalCluster) {
		Fail("you need to specify the correct path for the kubeconfigpath of the external cluster")
	}

	if !FileExists(*kubeconfigPathExternalCluster) {
		Fail("kubeconfigPathExternalCluster path does not exist")
	}
}

func cleanPlant(ctx context.Context, plantTest *PlantTest, secret *v1.Secret) error {
	if err := plantTest.GardenClient.Client().Delete(ctx, secret); err != nil {
		return err
	}
	return plantTest.DeletePlant(ctx)

}

func createPlant(ctx context.Context, plantNamespace string, plantTest *PlantTest, kubeConfigContent []byte) (*v1.Secret, error) {
	plantTest.Plant.Namespace = plantNamespace
	secret, err := plantTest.CreatePlantSecret(ctx, kubeConfigContent)
	if err != nil {
		return nil, err
	}

	if err := plantTest.CreatePlant(ctx, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

var _ = Describe("Plant testing", func() {
	var (
		plantTest              *PlantTest
		plantTestLogger        *logrus.Logger
		validKubeConfigContent []byte
	)

	CBeforeSuite(func(ctx context.Context) {
		validateFlags()
		plantTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		if StringSet(*plantTestYamlPath) {
			// parse plant yaml into plant object and generate random test names for plants
			plantObject, err := CreatePlantTestArtifacts(*plantTestYamlPath)
			Expect(err).NotTo(HaveOccurred())

			plantTest, err = NewPlantTest(*kubeconfigPath, *kubeconfigPathExternalCluster, plantObject, plantTestLogger)
			Expect(err).NotTo(HaveOccurred())
		}

		var err error
		validKubeConfigContent, err = ioutil.ReadFile(*kubeconfigPathExternalCluster)
		Expect(err).NotTo(HaveOccurred())

	}, InitializationTimeout)

	CIt("Should create plant successfully", func(ctx context.Context) {
		secret, err := createPlant(ctx, *plantTestNamespace, plantTest, validKubeConfigContent)
		Expect(err).NotTo(HaveOccurred())

		Expect(plantTest.WaitForPlantToBeReconciledSuccessfully(ctx)).NotTo(HaveOccurred())

		defer func() {
			Expect(cleanPlant(ctx, plantTest, secret)).NotTo(HaveOccurred())
		}()
	}, PlantCreationTimeout)

	CIt("Should update Plant Status to 'unknown' due to updated and invalid Plant Secret (kubeconfig invalid)", func(ctx context.Context) {
		secret, err := createPlant(ctx, *plantTestNamespace, plantTest, validKubeConfigContent)
		Expect(err).NotTo(HaveOccurred())

		Expect(plantTest.WaitForPlantToBeReconciledSuccessfully(ctx)).NotTo(HaveOccurred())

		defer func() {
			Expect(cleanPlant(ctx, plantTest, secret)).NotTo(HaveOccurred())
		}()

		// modify data.kubeconfigpath to update the secret with false information
		source := []byte("Here is a string....")
		base64DummyKubeconfig := base64.StdEncoding.EncodeToString(source)
		secret.Data[KubeConfigKey] = []byte(base64DummyKubeconfig)

		By(fmt.Sprintf("Update Plant secret with invalid kubeconfig"))

		err = plantTest.UpdatePlantSecret(ctx, secret)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Wait for PlantController to update to status 'unknown'"))

		err = plantTest.WaitForPlantToBeReconciledWithUnknownStatus(ctx)
		Expect(err).NotTo(HaveOccurred())

	}, PlantUpdateSecretTimeout)
	CIt("Should reconcile Plant Status to be successful after Plant Secret update", func(ctx context.Context) {
		dummyKubeconfigContent := []byte("Here is a string....")
		secret, err := createPlant(ctx, *plantTestNamespace, plantTest, dummyKubeconfigContent)
		Expect(err).NotTo(HaveOccurred())

		Expect(plantTest.WaitForPlantToBeReconciledWithUnknownStatus(ctx)).NotTo(HaveOccurred())

		defer func() {
			Expect(cleanPlant(ctx, plantTest, secret)).NotTo(HaveOccurred())
		}()

		secret.Data[KubeConfigKey] = validKubeConfigContent
		err = plantTest.UpdatePlantSecret(ctx, secret)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Plant secret updated to contain valid kubeconfig again"))
		err = plantTest.WaitForPlantToBeReconciledSuccessfully(ctx)
		Expect(err).NotTo(HaveOccurred())

		By("Plant reconciled successfully")

	}, PlantUpdateSecretTimeout)
	CIt("Should update Plant Status to 'unknown' due to updated and invalid Plant Secret (kubeconfig not provided)", func(ctx context.Context) {
		secret, err := createPlant(ctx, *plantTestNamespace, plantTest, validKubeConfigContent)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(cleanPlant(ctx, plantTest, secret)).NotTo(HaveOccurred())
		}()

		Expect(plantTest.WaitForPlantToBeReconciledSuccessfully(ctx)).NotTo(HaveOccurred())

		// remove data.kubeconfigpath to update the secret with false information
		secret.Data[KubeConfigKey] = nil
		err = plantTest.UpdatePlantSecret(ctx, secret)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Wait for PlantController to update to status 'unknown'"))
		err = plantTest.WaitForPlantToBeReconciledWithUnknownStatus(ctx)
		Expect(err).NotTo(HaveOccurred())
	}, PlantUpdateSecretTimeout)
})
