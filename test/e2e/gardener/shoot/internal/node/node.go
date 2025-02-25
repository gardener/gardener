// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

const nodeCriticalDaemonSetName = "e2e-test-node-critical"
const csiNodeDaemonSetName = "e2e-test-csi-node"
const waitForCSINodeAnnotation = v1beta1constants.AnnotationPrefixWaitForCSINode + "driver"
const driverName = "foo.driver.example.org"

// VerifyNodeCriticalComponentsBootstrapping tests the node readiness feature (see docs/usage/advanced/node-readiness.md).
func VerifyNodeCriticalComponentsBootstrapping(s *ShootContext) {
	GinkgoHelper()

	Describe("Verify node-critical components", func() {
		var seedNamespace string

		BeforeAll(func() {
			DeferCleanup(func(ctx SpecContext) {
				cleanupNodeCriticalManagedResource(ctx, s.SeedClient, seedNamespace, nodeCriticalDaemonSetName)
				cleanupNodeCriticalManagedResource(ctx, s.SeedClient, seedNamespace, csiNodeDaemonSetName)
			}, NodeTimeout(time.Minute))
		})

		It("Create ManagedResources for shoot with broken node-critical components", func(ctx SpecContext) {
			seedNamespace = s.Shoot.Status.TechnicalID
			createOrUpdateNodeCriticalManagedResource(ctx, s.SeedClient, s.ShootClient, seedNamespace, nodeCriticalDaemonSetName, "non-existing", false)
			createOrUpdateNodeCriticalManagedResource(ctx, s.SeedClient, s.ShootClient, seedNamespace, csiNodeDaemonSetName, "non-existing", true)
		}, SpecTimeout(time.Minute))

		It("Delete Nodes and Machines to trigger new Node bootstrap", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(s.SeedClient.DeleteAllOf(ctx, &machinev1alpha1.Machine{}, client.InNamespace(seedNamespace))).To(Succeed())
				g.Expect(s.ShootClient.DeleteAllOf(ctx, &corev1.Node{})).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		var node *corev1.Node
		It("Wait for new Node to be created", func(ctx SpecContext) {
			nodeList := &corev1.NodeList{}
			Eventually(ctx, s.ShootKomega.ObjectList(nodeList)).Should(
				HaveField("Items", Not(BeEmpty())), "new Node should be created",
			)
			node = &nodeList.Items[0]
		}, SpecTimeout(10*time.Minute))

		It("Verify node-critical components not ready taint is present", func(ctx SpecContext) {
			Eventually(ctx, s.ShootKomega.Object(node)).MustPassRepeatedly(3).WithPolling(2 * time.Second).Should(
				HaveField("Spec.Taints", ContainElement(corev1.Taint{
					Key:    v1beta1constants.TaintNodeCriticalComponentsNotReady,
					Effect: corev1.TaintEffectNoSchedule,
				})),
			)
		}, SpecTimeout(time.Minute))

		var nodeCriticalDaemonSet, csiNodeDaemonSet *appsv1.DaemonSet

		It("Update ManagedResources for shoot with working node-critical component", func(ctx SpecContext) {
			// Use a container image that is already cached
			image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNamePauseContainer)
			Expect(err).To(Succeed())

			nodeCriticalDaemonSet = createOrUpdateNodeCriticalManagedResource(ctx, s.SeedClient, s.ShootClient, seedNamespace, nodeCriticalDaemonSetName, image.String(), false)
			csiNodeDaemonSet = createOrUpdateNodeCriticalManagedResource(ctx, s.SeedClient, s.ShootClient, seedNamespace, csiNodeDaemonSetName, image.String(), true)
		}, SpecTimeout(time.Minute))

		It("Wait for node-critical components to become healthy", func(ctx SpecContext) {
			waitForDaemonSetToBecomeHealthy(ctx, s.ShootClient, nodeCriticalDaemonSet)
			waitForDaemonSetToBecomeHealthy(ctx, s.ShootClient, csiNodeDaemonSet)
		})

		var csiNodeObject *storagev1.CSINode

		It("Wait for CSINode object", func(ctx SpecContext) {
			csiNodeObject = waitForCSINodeObject(ctx, s.ShootClient)
		}, SpecTimeout(time.Minute))

		It("Patch CSINode object to contain required driver", func(ctx SpecContext) {
			patchCSINodeObjectWithRequiredDriver(ctx, s.ShootClient, csiNodeObject)
		}, SpecTimeout(time.Minute))

		It("Verify node-critical components not ready taint is removed", func(ctx SpecContext) {
			Eventually(ctx, s.ShootKomega.Object(node)).WithPolling(2 * time.Second).Should(
				HaveField("Spec.Taints", Not(ContainElement(corev1.Taint{
					Key:    v1beta1constants.TaintNodeCriticalComponentsNotReady,
					Effect: corev1.TaintEffectNoSchedule,
				}))),
			)
		}, SpecTimeout(time.Minute))
	})
}

func getLabels(name string) map[string]string {
	return map[string]string{
		"e2e-test": name,
	}
}

func createOrUpdateNodeCriticalManagedResource(ctx context.Context, seedClient, shootClient client.Client, namespace, name, image string, annotateAsCSINodePod bool) *appsv1.DaemonSet {
	GinkgoHelper()

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

	Expect(secret.AddLabels(getLabels(name)).Reconcile(ctx)).To(Succeed())
	Expect(managedResource.WithLabels(getLabels(name)).Reconcile(ctx)).To(Succeed())

	By("Wait for DaemonSet to be applied in shoot")
	Eventually(ctx, func(g Gomega) string {
		g.Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(daemonSet), daemonSet)).To(Succeed())
		return daemonSet.Spec.Template.Spec.Containers[0].Image
	}).Should(Equal(image))

	return daemonSet
}

func waitForDaemonSetToBecomeHealthy(ctx context.Context, shootClient client.Client, daemonSet *appsv1.DaemonSet) {
	GinkgoHelper()

	By("Wait for DaemonSet to become healthy")
	Eventually(ctx, func(g Gomega) {
		g.Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(daemonSet), daemonSet)).To(Succeed())
		g.Expect(health.CheckDaemonSet(daemonSet)).To(Succeed())
	}).Should(Succeed())
}

func waitForCSINodeObject(ctx context.Context, shootClient client.Client) *storagev1.CSINode {
	GinkgoHelper()

	csiNodeList := &storagev1.CSINodeList{}

	Eventually(ctx, func(g Gomega) {
		g.Expect(shootClient.List(ctx, csiNodeList)).To(Succeed())
		g.Expect(csiNodeList.Items).To(HaveLen(1))
	}).Should(Succeed())

	return &csiNodeList.Items[0]
}

func patchCSINodeObjectWithRequiredDriver(ctx context.Context, shootClient client.Client, csiNode *storagev1.CSINode) {
	patch := client.MergeFrom(csiNode.DeepCopy())

	csiNode.Spec.Drivers = []storagev1.CSINodeDriver{
		{
			Name:   driverName,
			NodeID: string(uuid.NewUUID()),
		},
	}

	Eventually(ctx, func(g Gomega) {
		g.Expect(shootClient.Patch(ctx, csiNode, patch)).To(Succeed())
	}).Should(Succeed())
}

func cleanupNodeCriticalManagedResource(ctx context.Context, seedClient client.Client, namespace, name string) {
	GinkgoHelper()

	By("Cleanup ManagedResource for shoot with node-critical component")
	Eventually(ctx, func(g Gomega) {
		g.Expect(seedClient.DeleteAllOf(ctx, &resourcesv1alpha1.ManagedResource{}, client.InNamespace(namespace), client.MatchingLabels(getLabels(name)))).To(Succeed())
		g.Expect(seedClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(namespace), client.MatchingLabels(getLabels(name)))).To(Succeed())
	}).Should(Succeed())
}
