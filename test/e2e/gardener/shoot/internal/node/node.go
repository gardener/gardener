// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package node

import (
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/shoots/access"
)

const name = "e2e-test-node-critical"

// VerifyNodeCriticalComponentsBootstrapping tests the node readiness feature (see docs/usage/node-readiness.md).
func VerifyNodeCriticalComponentsBootstrapping(ctx context.Context, f *framework.ShootFramework) {
	shootClientSet, err := access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
	Expect(err).To(Succeed())

	var (
		seedClient  = f.SeedClient.Client()
		shootClient = shootClientSet.Client()
		technicalID = f.Shoot.Status.TechnicalID
	)

	By("Create ManagedResource for shoot with broken node-critical component")
	createOrUpdateNodeCriticalManagedResource(ctx, seedClient, shootClient, technicalID, "non-existing")
	DeferCleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		cleanupNodeCriticalManagedResource(cleanupCtx, seedClient, technicalID)
	})

	By("Delete Nodes and Machines to trigger new Node bootstrap")
	Eventually(func(g Gomega) {
		g.Expect(seedClient.DeleteAllOf(ctx, &machinev1alpha1.Machine{}, client.InNamespace(technicalID))).To(Succeed())
		g.Expect(shootClient.DeleteAllOf(ctx, &corev1.Node{})).To(Succeed())
	}).Should(Succeed())

	By("Wait for new Node to be created")
	var node *corev1.Node
	Eventually(func(g Gomega) {
		nodeList := &corev1.NodeList{}
		g.Expect(shootClient.List(ctx, nodeList)).To(Succeed())
		g.Expect(nodeList.Items).NotTo(BeEmpty(), "new Node should be created")
		node = &nodeList.Items[0]
	}).WithContext(ctx).WithTimeout(10 * time.Minute).Should(Succeed())

	By("Verify node-critical components not ready taint is present")
	Consistently(func(g Gomega) []corev1.Taint {
		g.Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		return node.Spec.Taints
	}).Should(ContainElement(corev1.Taint{
		Key:    v1beta1constants.TaintNodeCriticalComponentsNotReady,
		Effect: corev1.TaintEffectNoSchedule,
	}))

	By("Update ManagedResource for shoot with working node-critical component")
	createOrUpdateNodeCriticalManagedResource(ctx, seedClient, shootClient, technicalID, "nginx")

	By("Verify node-critical components not ready taint is removed")
	Eventually(func(g Gomega) []corev1.Taint {
		g.Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		return node.Spec.Taints
	}).WithContext(ctx).WithTimeout(10 * time.Minute).ShouldNot(ContainElement(corev1.Taint{
		Key:    v1beta1constants.TaintNodeCriticalComponentsNotReady,
		Effect: corev1.TaintEffectNoSchedule,
	}))

	cleanupNodeCriticalManagedResource(ctx, seedClient, technicalID)
}

func getLabels() map[string]string {
	return map[string]string{
		"e2e-test": "node-critical",
	}
}

func createOrUpdateNodeCriticalManagedResource(ctx context.Context, seedClient, shootClient client.Client, namespace string, image string) {
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "kube-system",
			Labels: map[string]string{
				v1beta1constants.LabelNodeCriticalComponent: "true",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(
						getLabels(),
						map[string]string{v1beta1constants.LabelNodeCriticalComponent: "true"},
					),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  name,
						Image: image,
					}},
					Tolerations: []corev1.Toleration{{
						Key:    v1beta1constants.TaintNodeCriticalComponentsNotReady,
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
			},
		},
	}

	data, err := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).AddAllAndSerialize(daemonSet)
	Expect(err).NotTo(HaveOccurred())

	secretName, secret := managedresources.NewSecret(seedClient, namespace, name, data, true)
	managedResource := managedresources.NewForShoot(seedClient, namespace, name, managedresources.LabelValueGardener, false).WithSecretRef(secretName)

	Expect(secret.WithLabels(getLabels()).Reconcile(ctx)).To(Succeed())
	Expect(managedResource.WithLabels(getLabels()).Reconcile(ctx)).To(Succeed())

	By("Wait for DaemonSet to be applied in shoot")
	Eventually(func(g Gomega) string {
		g.Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(daemonSet), daemonSet)).To(Succeed())
		return daemonSet.Spec.Template.Spec.Containers[0].Image
	}).WithContext(ctx).WithTimeout(5 * time.Minute).Should(Equal(image))
}

func cleanupNodeCriticalManagedResource(ctx context.Context, seedClient client.Client, namespace string) {
	By("Cleanup ManagedResource for shoot with node-critical component")
	Expect(seedClient.DeleteAllOf(ctx, &resourcesv1alpha1.ManagedResource{}, client.InNamespace(namespace), client.MatchingLabels(getLabels()))).To(Succeed())
	Expect(seedClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(namespace), client.MatchingLabels(getLabels()))).To(Succeed())
}
