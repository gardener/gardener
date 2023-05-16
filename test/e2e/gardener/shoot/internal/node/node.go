// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
)

const nodeCriticalDaemonSetName = "e2e-test-node-critical"
const csiNodeDaemonSetName = "e2e-test-csi-node"
const waitForCSINodeAnnotation = v1beta1constants.AnnotationPrefixWaitForCSINode + "driver"
const driverName = "foo.driver.example.org"

// VerifyNodeCriticalComponentsBootstrapping tests the node readiness feature (see docs/usage/node-readiness.md).
func VerifyNodeCriticalComponentsBootstrapping(ctx context.Context, f *framework.ShootFramework) {
	shootClientSet, err := access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
	Expect(err).To(Succeed())

	var (
		seedClient  = f.SeedClient.Client()
		shootClient = shootClientSet.Client()
		technicalID = f.Shoot.Status.TechnicalID
	)

	By("Create ManagedResources for shoot with broken node-critical components")
	createOrUpdateNodeCriticalManagedResource(ctx, seedClient, shootClient, technicalID, nodeCriticalDaemonSetName, "non-existing", false)
	createOrUpdateNodeCriticalManagedResource(ctx, seedClient, shootClient, technicalID, csiNodeDaemonSetName, "non-existing", true)
	DeferCleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		cleanupNodeCriticalManagedResource(cleanupCtx, seedClient, technicalID, nodeCriticalDaemonSetName)
		cleanupNodeCriticalManagedResource(cleanupCtx, seedClient, technicalID, csiNodeDaemonSetName)
	})

	By("Delete Nodes and Machines to trigger new Node bootstrap")
	Eventually(func(g Gomega) {
		g.ExpectWithOffset(1, seedClient.DeleteAllOf(ctx, &machinev1alpha1.Machine{}, client.InNamespace(technicalID))).To(Succeed())
		g.ExpectWithOffset(1, shootClient.DeleteAllOf(ctx, &corev1.Node{})).To(Succeed())
	}).Should(Succeed())

	By("Wait for new Node to be created")
	var node *corev1.Node
	Eventually(func(g Gomega) {
		nodeList := &corev1.NodeList{}
		g.ExpectWithOffset(1, shootClient.List(ctx, nodeList)).To(Succeed())
		g.ExpectWithOffset(1, nodeList.Items).NotTo(BeEmpty(), "new Node should be created")
		node = &nodeList.Items[0]
	}).WithContext(ctx).WithTimeout(10 * time.Minute).Should(Succeed())

	By("Verify node-critical components not ready taint is present")
	Consistently(func(g Gomega) []corev1.Taint {
		g.ExpectWithOffset(1, shootClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		return node.Spec.Taints
	}).Should(ContainElement(corev1.Taint{
		Key:    v1beta1constants.TaintNodeCriticalComponentsNotReady,
		Effect: corev1.TaintEffectNoSchedule,
	}))

	By("Update ManagedResources for shoot with working node-critical component")
	createOrUpdateNodeCriticalManagedResource(ctx, seedClient, shootClient, technicalID, nodeCriticalDaemonSetName, "nginx", false)
	createOrUpdateNodeCriticalManagedResource(ctx, seedClient, shootClient, technicalID, csiNodeDaemonSetName, "nginx", true)

	By("Patch CSINode object to contain required driver")
	patchCSINodeObjectWithRequiredDriver(ctx, shootClient)

	By("Verify node-critical components not ready taint is removed")
	Eventually(func(g Gomega) []corev1.Taint {
		g.ExpectWithOffset(1, shootClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		return node.Spec.Taints
	}).WithContext(ctx).WithTimeout(10 * time.Minute).ShouldNot(ContainElement(corev1.Taint{
		Key:    v1beta1constants.TaintNodeCriticalComponentsNotReady,
		Effect: corev1.TaintEffectNoSchedule,
	}))

	cleanupNodeCriticalManagedResource(ctx, seedClient, technicalID, nodeCriticalDaemonSetName)
	cleanupNodeCriticalManagedResource(ctx, seedClient, technicalID, csiNodeDaemonSetName)
}

func getLabels(name string) map[string]string {
	return map[string]string{
		"e2e-test": name,
	}
}

func createOrUpdateNodeCriticalManagedResource(ctx context.Context, seedClient, shootClient client.Client, namespace, name, image string, annotateAsCSINodePod bool) {
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
				MatchLabels: getLabels(name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(
						getLabels(name),
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

	if annotateAsCSINodePod {
		daemonSet.Spec.Template.ObjectMeta.Annotations = map[string]string{
			waitForCSINodeAnnotation: driverName,
		}
	}

	data, err := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).AddAllAndSerialize(daemonSet)
	Expect(err).NotTo(HaveOccurred())

	secretName, secret := managedresources.NewSecret(seedClient, namespace, name, data, true)
	managedResource := managedresources.NewForShoot(seedClient, namespace, name, managedresources.LabelValueGardener, false).WithSecretRef(secretName)

	Expect(secret.WithLabels(getLabels(name)).Reconcile(ctx)).To(Succeed())
	Expect(managedResource.WithLabels(getLabels(name)).Reconcile(ctx)).To(Succeed())

	By("Wait for DaemonSet to be applied in shoot")
	Eventually(func(g Gomega) string {
		g.ExpectWithOffset(2, shootClient.Get(ctx, client.ObjectKeyFromObject(daemonSet), daemonSet)).To(Succeed())
		return daemonSet.Spec.Template.Spec.Containers[0].Image
	}).WithContext(ctx).WithTimeout(5 * time.Minute).Should(Equal(image))
}

func patchCSINodeObjectWithRequiredDriver(ctx context.Context, shootClient client.Client) {
	csiNodeList := &storagev1.CSINodeList{}
	Expect(shootClient.List(ctx, csiNodeList)).To(Succeed())
	Expect(csiNodeList.Items).To(HaveLen(1))

	csiNode := csiNodeList.Items[0].DeepCopy()
	csiNode.Spec.Drivers = []storagev1.CSINodeDriver{
		{
			Name:   driverName,
			NodeID: string(uuid.NewUUID()),
		},
	}

	ExpectWithOffset(2, shootClient.Update(ctx, csiNode)).To(Succeed())
}

func cleanupNodeCriticalManagedResource(ctx context.Context, seedClient client.Client, namespace, name string) {
	By("Cleanup ManagedResource for shoot with node-critical component")
	ExpectWithOffset(2, seedClient.DeleteAllOf(ctx, &resourcesv1alpha1.ManagedResource{}, client.InNamespace(namespace), client.MatchingLabels(getLabels(name)))).To(Succeed())
	ExpectWithOffset(2, seedClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(namespace), client.MatchingLabels(getLabels(name)))).To(Succeed())
}
