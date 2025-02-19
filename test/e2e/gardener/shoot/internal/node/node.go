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
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
)

const nodeCriticalDaemonSetName = "e2e-test-node-critical"
const csiNodeDaemonSetName = "e2e-test-csi-node"
const waitForCSINodeAnnotation = v1beta1constants.AnnotationPrefixWaitForCSINode + "driver"
const driverName = "foo.driver.example.org"

// VerifyNodeCriticalComponentsBootstrapping tests the node readiness feature (see docs/usage/advanced/node-readiness.md).
func VerifyNodeCriticalComponentsBootstrapping(parentCtx context.Context, f *framework.ShootFramework) {
	ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
	defer cancel()

	shootClientSet, err := access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
	ExpectWithOffset(1, err).To(Succeed())

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
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(seedClient.DeleteAllOf(ctx, &machinev1alpha1.Machine{}, client.InNamespace(technicalID))).To(Succeed())
		g.Expect(shootClient.DeleteAllOf(ctx, &corev1.Node{})).To(Succeed())
	}).Should(Succeed())

	By("Wait for new Node to be created")
	var node *corev1.Node
	EventuallyWithOffset(1, func(g Gomega) {
		nodeList := &corev1.NodeList{}
		g.Expect(shootClient.List(ctx, nodeList)).To(Succeed())
		g.Expect(nodeList.Items).NotTo(BeEmpty(), "new Node should be created")
		node = &nodeList.Items[0]
	}).WithContext(ctx).WithTimeout(10 * time.Minute).Should(Succeed())

	By("Verify node-critical components not ready taint is present")
	ConsistentlyWithOffset(1, func(g Gomega) []corev1.Taint {
		g.Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		return node.Spec.Taints
	}).Should(ContainElement(corev1.Taint{
		Key:    v1beta1constants.TaintNodeCriticalComponentsNotReady,
		Effect: corev1.TaintEffectNoSchedule,
	}))

	By("Update ManagedResources for shoot with working node-critical component")
	// Use a container image that is already cached
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNamePauseContainer)
	ExpectWithOffset(1, err).To(Succeed())

	nodeCriticalDaemonSet := createOrUpdateNodeCriticalManagedResource(ctx, seedClient, shootClient, technicalID, nodeCriticalDaemonSetName, image.String(), false)
	csiNodeDaemonSet := createOrUpdateNodeCriticalManagedResource(ctx, seedClient, shootClient, technicalID, csiNodeDaemonSetName, image.String(), true)

	waitForDaemonSetToBecomeHealthy(ctx, shootClient, nodeCriticalDaemonSet)
	waitForDaemonSetToBecomeHealthy(ctx, shootClient, csiNodeDaemonSet)

	By("Patch CSINode object to contain required driver")
	patchCSINodeObjectWithRequiredDriver(ctx, shootClient)

	By("Verify node-critical components not ready taint is removed")
	EventuallyWithOffset(1, func(g Gomega) []corev1.Taint {
		g.Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
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

func createOrUpdateNodeCriticalManagedResource(ctx context.Context, seedClient, shootClient client.Client, namespace, name, image string, annotateAsCSINodePod bool) *appsv1.DaemonSet {
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
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	secretName, secret := managedresources.NewSecret(seedClient, namespace, name, data, true)
	managedResource := managedresources.NewForShoot(seedClient, namespace, name, managedresources.LabelValueGardener, false).WithSecretRef(secretName)

	ExpectWithOffset(1, secret.AddLabels(getLabels(name)).Reconcile(ctx)).To(Succeed())
	ExpectWithOffset(1, managedResource.WithLabels(getLabels(name)).Reconcile(ctx)).To(Succeed())

	By("Wait for DaemonSet to be applied in shoot")
	EventuallyWithOffset(1, func(g Gomega) string {
		g.Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(daemonSet), daemonSet)).To(Succeed())
		return daemonSet.Spec.Template.Spec.Containers[0].Image
	}).WithContext(ctx).WithTimeout(5 * time.Minute).Should(Equal(image))

	return daemonSet
}

func waitForDaemonSetToBecomeHealthy(ctx context.Context, shootClient client.Client, daemonSet *appsv1.DaemonSet) {
	By("Wait for DaemonSet to become healthy")
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(daemonSet), daemonSet)).To(Succeed())
		g.Expect(health.CheckDaemonSet(daemonSet)).To(Succeed())
	}).WithContext(ctx).WithTimeout(5 * time.Minute).Should(Succeed())
}

func patchCSINodeObjectWithRequiredDriver(ctx context.Context, shootClient client.Client) {
	csiNodeList := &storagev1.CSINodeList{}

	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(shootClient.List(ctx, csiNodeList)).To(Succeed())
		g.Expect(csiNodeList.Items).To(HaveLen(1))
	}).WithContext(ctx).WithTimeout(1 * time.Minute).Should(Succeed())

	csiNode := csiNodeList.Items[0].DeepCopy()
	csiNode.Spec.Drivers = []storagev1.CSINodeDriver{
		{
			Name:   driverName,
			NodeID: string(uuid.NewUUID()),
		},
	}

	ExpectWithOffset(1, shootClient.Update(ctx, csiNode)).To(Succeed())
}

func cleanupNodeCriticalManagedResource(ctx context.Context, seedClient client.Client, namespace, name string) {
	By("Cleanup ManagedResource for shoot with node-critical component")
	ExpectWithOffset(1, seedClient.DeleteAllOf(ctx, &resourcesv1alpha1.ManagedResource{}, client.InNamespace(namespace), client.MatchingLabels(getLabels(name)))).To(Succeed())
	ExpectWithOffset(1, seedClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(namespace), client.MatchingLabels(getLabels(name)))).To(Succeed())
}
