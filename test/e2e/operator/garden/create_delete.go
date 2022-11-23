// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	. "github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Garden Tests", Label("Garden", "default"), func() {
	var (
		parentCtx     = context.Background()
		runtimeClient client.Client
		garden        *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&componentbaseconfig.ClientConnectionConfiguration{Kubeconfig: os.Getenv("KUBECONFIG")}, nil, kubernetes.AuthTokenFile)
		Expect(err).NotTo(HaveOccurred())

		runtimeClient, err = client.New(restConfig, client.Options{Scheme: operatorclient.RuntimeScheme})
		Expect(err).NotTo(HaveOccurred())

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "garden-",
			},
			Spec: operatorv1alpha1.GardenSpec{
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Provider: operatorv1alpha1.Provider{
						Zones: []string{"0"},
					},
					Settings: &operatorv1alpha1.Settings{
						VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
							Enabled: pointer.Bool(true),
						},
					},
				},
			},
		}
	})

	It("Create, Delete", Label("simple"), func() {
		By("Create Garden")
		ctx, cancel := context.WithTimeout(parentCtx, 2*time.Minute)
		defer cancel()

		Expect(runtimeClient.Create(ctx, garden)).To(Succeed())
		CEventually(ctx, func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			return garden.Status.Conditions
		}).WithPolling(2 * time.Second).Should(ContainCondition(OfType(operatorv1alpha1.GardenReconciled), WithStatus(gardencorev1beta1.ConditionTrue)))

		By("Verify creation")
		CEventually(ctx, func(g Gomega) {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(runtimeClient.List(ctx, managedResourceList, client.InNamespace("garden"))).To(Succeed())
			g.Expect(managedResourceList.Items).To(ConsistOf(
				healthyManagedResource("garden-system"),
				healthyManagedResource("hvpa"),
				healthyManagedResource("vpa"),
				healthyManagedResource("etcd-druid"),
			))
		}).WithPolling(2 * time.Second).Should(Succeed())

		By("Delete Garden")
		ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
		defer cancel()

		Expect(runtimeClient.Delete(ctx, garden)).To(Succeed())
		CEventually(ctx, func() error {
			return runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
		}).WithPolling(2 * time.Second).Should(BeNotFoundError())

		By("Verify deletion")
		secretList := &corev1.SecretList{}
		Expect(runtimeClient.List(ctx, secretList, client.InNamespace("garden"), client.MatchingLabels{
			secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
			secretsmanager.LabelKeyManagerIdentity: operatorv1alpha1.SecretManagerIdentityOperator,
		})).To(Succeed())
		Expect(secretList.Items).To(BeEmpty())

		crdList := &apiextensionsv1.CustomResourceDefinitionList{}
		Expect(runtimeClient.List(ctx, crdList)).To(Succeed())
		Expect(crdList.Items).To(ContainElement(MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardens.operator.gardener.cloud")})})))

		Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: "garden"}, &appsv1.Deployment{})).To(BeNotFoundError())
	})
})

func healthyManagedResource(name string) gomegatypes.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal(name)}),
		"Status": MatchFields(IgnoreExtras, Fields{"Conditions": And(
			ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue)),
			ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionTrue)),
			ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse)),
		)}),
	})
}
