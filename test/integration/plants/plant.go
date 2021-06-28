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
		- Tests the Gardener Controller Manager Plants Controller.

	Prerequisites
		- Kubeconfig to external cluster available to register as Plant

	BeforeSuite
		- Parse valid Plant from example folder and flags.

	Test: Create Plant
	Expected Output
	- should create & reconcile the plant successfully.

	Test: Update the Plant's secret with an invalid kubeconfig
	Expected Output
	- Should update Plant Status to 'unknown'.

	Test: Update the Plant's secret with a valid kubeconfig
	Expected Output
	- Should reconcile the plant successfully.

	Test: Update the Plant's secret & removing the kubeconfig
	Expected Output
	- Should update Plant Status to 'unknown'.

 **/

package plants

import (
	"context"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/test/framework"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	plantUpdateSecretTimeout = 90 * time.Second
	plantCreationTimeout     = 60 * time.Second
)

func cleanPlant(ctx context.Context, f *framework.ShootFramework, plant *gardencorev1beta1.Plant, secret *corev1.Secret) error {
	if err := f.GardenClient.Client().Delete(ctx, secret); err != nil {
		return err
	}
	return f.DeletePlant(ctx, plant)

}

func createPlant(ctx context.Context, f *framework.ShootFramework, kubeConfigContent []byte) (*gardencorev1beta1.Plant, *corev1.Secret, error) {
	secret, err := f.CreatePlantSecret(ctx, f.ProjectNamespace, kubeConfigContent)
	if err != nil {
		return nil, nil, err
	}

	plant := &gardencorev1beta1.Plant{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "e2e-",
			Namespace:    f.ProjectNamespace,
		},
		Spec: gardencorev1beta1.PlantSpec{
			SecretRef: corev1.LocalObjectReference{Name: secret.Name},
		},
	}

	if err := f.CreatePlant(ctx, plant); err != nil {
		return nil, nil, err
	}
	return plant, secret, nil
}

var _ = ginkgo.Describe("Plant testing", func() {

	f := framework.NewShootFramework(nil)

	var (
		validKubeConfigContent []byte
		dummyKubeConfigContent = []byte(`---
apiVersion: v1
kind: Config
clusters:
- name: dummy
  cluster:
    server: https://some-endpoint-that-does-not-exist
    insecure-skip-tls-verify: true
contexts:
  - name: dummy
    context:
      cluster: dummy
      user: dummy
current-context: dummy
users:
  - name: dummy
    user:
      token: AAAA`)
	)

	framework.CBeforeEach(func(ctx context.Context) {
		if f.Config.Fenced {
			ginkgo.Skip("Shoot skipped as it cannot be reached by gardener")
		}
		// read a valid kubeconfig form the given shoot
		kubeconfig, err := framework.GetObjectFromSecret(ctx, f.GardenClient, f.ProjectNamespace, f.ShootKubeconfigSecretName(), framework.KubeconfigSecretKeyName)
		framework.ExpectNoError(err)
		validKubeConfigContent = []byte(kubeconfig)
	}, 1*time.Minute)

	f.Default().CIt("Should create plant successfully", func(ctx context.Context) {
		plant, secret, err := createPlant(ctx, f, validKubeConfigContent)
		framework.ExpectNoError(err)

		framework.ExpectNoError(f.WaitForPlantToBeReconciledSuccessfully(ctx, plant))

		// cross-check discovered plant cluster info with shoot spec
		// unfortunately we cannot cross-check Cloud.Type in a similar fashion, as the nodes' providerID don't have the
		// same pattern on all cloud providers and also don't necessarily map to the cloud provider types known to
		// Gardener (e.g. GCP nodes have `gce://` as a prefix in their providerID, but the corresponding provider type
		// in Gardener is `gcp`)
		gomega.Expect(plant.Status.ClusterInfo.Cloud.Region).To(gomega.Equal(f.Shoot.Spec.Region))
		gomega.Expect(plant.Status.ClusterInfo.Kubernetes.Version).To(gomega.Equal("v" + f.Shoot.Spec.Kubernetes.Version))

		defer func() {
			framework.ExpectNoError(cleanPlant(ctx, f, plant, secret))
		}()
	}, plantCreationTimeout)

	ginkgo.Context("", func() {

		var (
			plant  *gardencorev1beta1.Plant
			secret *corev1.Secret
			err    error
		)

		framework.CBeforeEach(func(ctx context.Context) {
			plant, secret, err = createPlant(ctx, f, validKubeConfigContent)
			framework.ExpectNoError(err)
		}, 5*time.Minute)

		framework.CAfterEach(func(ctx context.Context) {
			framework.ExpectNoError(cleanPlant(ctx, f, plant, secret))
		}, 1*time.Minute)

		f.Default().CIt("Should update Plant Status to 'unknown' due to updated and invalid Plant Secret (kubeconfig invalid)", func(ctx context.Context) {
			framework.ExpectNoError(f.WaitForPlantToBeReconciledSuccessfully(ctx, plant))

			// modify data.kubeconfigpath to update the secret with false information
			secret.Data[framework.KubeconfigSecretKeyName] = dummyKubeConfigContent

			ginkgo.By("Update Plant secret with invalid kubeconfig")

			err = framework.PatchSecret(ctx, f.GardenClient.Client(), secret)
			framework.ExpectNoError(err)

			ginkgo.By("Wait for PlantController to update to status 'unknown'")

			err = f.WaitForPlantToBeReconciledWithUnknownStatus(ctx, plant)
			framework.ExpectNoError(err)
		}, plantUpdateSecretTimeout)

		f.Default().CIt("Should update Plant Status to 'unknown' due to updated and invalid Plant Secret (kubeconfig not provided)", func(ctx context.Context) {

			framework.ExpectNoError(f.WaitForPlantToBeReconciledSuccessfully(ctx, plant))

			// remove data.kubeconfigpath to update the secret with false information
			secret.Data[framework.KubeconfigSecretKeyName] = nil
			err = framework.PatchSecret(ctx, f.GardenClient.Client(), secret)
			framework.ExpectNoError(err)

			ginkgo.By("Wait for PlantController to update to status 'unknown'")
			err = f.WaitForPlantToBeReconciledWithUnknownStatus(ctx, plant)
			framework.ExpectNoError(err)
		}, plantUpdateSecretTimeout)
	})

	f.Default().CIt("Should reconcile Plant Status to be successful after Plant Secret update", func(ctx context.Context) {
		plant, secret, err := createPlant(ctx, f, dummyKubeConfigContent)
		framework.ExpectNoError(err)

		defer func() {
			framework.ExpectNoError(cleanPlant(ctx, f, plant, secret))
		}()

		framework.ExpectNoError(f.WaitForPlantToBeReconciledWithUnknownStatus(ctx, plant))

		secret.Data[framework.KubeconfigSecretKeyName] = validKubeConfigContent
		err = framework.PatchSecret(ctx, f.GardenClient.Client(), secret)
		framework.ExpectNoError(err)

		ginkgo.By("Plant secret updated to contain valid kubeconfig again")
		err = f.WaitForPlantToBeReconciledSuccessfully(ctx, plant)
		framework.ExpectNoError(err)

		ginkgo.By("Plant reconciled successfully")
	}, plantUpdateSecretTimeout)
})
