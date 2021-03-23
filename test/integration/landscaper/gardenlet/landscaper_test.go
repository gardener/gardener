// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenlet

import (
	"context"
	"time"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/framework"
	landscaperframework "github.com/gardener/gardener/test/framework/landscaper"

	. "github.com/onsi/ginkgo"
)

const (
	CreateAndReconcileTimeout = 30 * time.Minute
	DeletionTimeout           = 15 * time.Minute
)

func init() {
	landscaperframework.RegisterGardenletFrameworkFlags()
}

var _ = Describe("Gardenlet landscaper testing", func() {

	f := landscaperframework.NewGardenletFramework(&landscaperframework.GardenletConfig{
		GardenerConfig: &framework.GardenerConfig{
			CommonConfig: &framework.CommonConfig{
				ResourceDir: "../../framework/resources",
			},
		},
		LandscaperCommonConfig: nil,
	})

	f.CIt("Should successfully install the Gardenlet via the Gardenlet landscaper component", func(ctx context.Context) {
		// create required resources to satisfy the imports in the installation resource
		framework.ExpectNoError(f.CreateInstallationImports(ctx))

		// create the installation resource and wait until it was successfully reconciled
		installation, err := f.CreateInstallation(ctx)
		framework.ExpectNoError(err)
		gomega.Expect(installation).ToNot(gomega.BeNil())

		// check if the Seed cluster has the Gardenlet deployment
		// just check for existence, because we expect the deployment to fail
		// as the Garden cluster in the Test machinery does not have the Gardener API yet
		gardenletDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardenlet",
				Namespace: gardencorev1beta1constants.GardenNamespace,
			},
		}

		framework.ExpectNoError(retry.UntilTimeout(ctx, 10*time.Second, 1*time.Minute, func(ctx context.Context) (done bool, err error) {
			err = f.SeedClient.Get(ctx, client.ObjectKeyFromObject(gardenletDeployment), gardenletDeployment)
			if err != nil {
				f.Logger.Debugf("Could not find Gardenlet deployment (%s/%s): %s", gardenletDeployment.Namespace, gardenletDeployment.Name, err.Error())
				return retry.MinorError(err)
			}
			return retry.Ok()
		}))

		// Check that the Gardenlet landscaper deployed the
		// secret containing the kubeconfig of the Seed cluster to the Garden cluster
		// Secret is specified in Gardenlet Landscaper import configuration
		// componentConfiguration.seedConfig.secretRef
		gomega.Expect(f.ComponentConfiguration).ToNot(gomega.BeNil())
		gomega.Expect(f.ComponentConfiguration.SeedConfig).ToNot(gomega.BeNil())
		gomega.Expect(f.ComponentConfiguration.SeedConfig.Spec.SecretRef).ToNot(gomega.BeNil())

		kubeconfigSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      f.ComponentConfiguration.SeedConfig.Spec.SecretRef.Name,
			Namespace: f.ComponentConfiguration.SeedConfig.Spec.SecretRef.Namespace,
		}}

		framework.ExpectNoError(retry.UntilTimeout(ctx, 10*time.Second, 1*time.Minute, func(ctx context.Context) (done bool, err error) {
			err = f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(kubeconfigSecret), kubeconfigSecret)
			if err != nil {
				f.Logger.Debugf("Could not find secret containing the kubeconfig of the Seed cluster to be deployed in the Garden cluster (%s/%s): %s", kubeconfigSecret.Namespace, kubeconfigSecret.Name, err.Error())
				return retry.MinorError(err)
			}
			return retry.Ok()
		}))

		framework.ExpectNoError(nil)
	}, CreateAndReconcileTimeout)

	// in the after suite, we wait for the deletion of the Installation CRD
	// and all of its imported resources
	// checks if the Gardenlet deployment in the Seed is gone + the kubeconfig secret
	// in the Garden cluster is deleted
	framework.CAfterSuite(func(ctx context.Context) {
		framework.ExpectNoError(f.DeleteInstallationResources(ctx))
	}, DeletionTimeout)
})
