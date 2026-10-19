// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package etcdmember_test

import (
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("EtcdMember controller", func() {
	var (
		namespace                         *corev1.Namespace
		machine                           *machinev1alpha1.Machine
		etcdMain, etcdEvents              *druidcorev1alpha1.Etcd
		ensureEtcdReconciliationSucceeded func()
	)

	BeforeEach(OncePerOrdered, func() {
		logBuffer.Reset()

		namespace = newNamespace()
		Expect(testClient.Create(ctx, namespace)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, namespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		// Create the Etcd objects the controller will mutate. The CRD validation requires
		// spec.externallyManagedMemberAddresses to be present at creation time (it cannot be added
		// later). Pre-populate it with a placeholder address so the controller can append/remove entries.
		newEtcd := func(role string) *druidcorev1alpha1.Etcd {
			return &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: etcd.Name(role), Namespace: namespace.Name},
				Spec: druidcorev1alpha1.EtcdSpec{
					Replicas:                         1,
					Labels:                           map[string]string{testID: testRunID},
					Selector:                         &metav1.LabelSelector{MatchLabels: map[string]string{testID: testRunID}},
					ExternallyManagedMemberAddresses: []string{"192.168.0.1"},
				},
			}
		}
		etcdMain = newEtcd("main")
		etcdEvents = newEtcd("events")
		Expect(testClient.Create(ctx, etcdMain)).To(Succeed())
		Expect(testClient.Create(ctx, etcdEvents)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, etcdMain))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, etcdEvents))).To(Succeed())

			Eventually(func() error { return testClient.Get(ctx, client.ObjectKeyFromObject(etcdMain), etcdMain) }).Should(BeNotFoundError())
			Eventually(func() error { return testClient.Get(ctx, client.ObjectKeyFromObject(etcdEvents), etcdEvents) }).Should(BeNotFoundError())
		})

		machine = &machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "machine-",
				Namespace:    namespace.Name,
				Annotations:  map[string]string{"node.machine.sapcloud.io.suspend-instance-deletion/remove-etcd-member": "gardenlet"},
			},
		}
		Expect(testClient.Create(ctx, machine)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, machine))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, machine))).To(Succeed())
			Eventually(func() error { return testClient.Get(ctx, client.ObjectKeyFromObject(machine), machine) }).Should(BeNotFoundError())
		})

		DeferCleanup(test.WithVars(
			&etcd.DefaultInterval, 1*time.Second,
			&etcd.DefaultTimeout, 2*time.Second,
			&etcd.DefaultSevereThreshold, 3*time.Second,
		))

		ensureEtcdReconciliationSucceeded = func() {
			// These should be done by the etcd-druid controller, which is not running here and must be faked for
			// the tests to pass.
			Eventually(func(g Gomega) {
				makeHealthy := func(etcd *druidcorev1alpha1.Etcd) {
					patch := client.MergeFrom(etcd.DeepCopy())
					etcd.Status.ObservedGeneration = new(etcd.Generation)
					g.Expect(testClient.Status().Patch(ctx, etcd, patch)).To(Succeed())

					patch = client.MergeFrom(etcd.DeepCopy())
					delete(etcd.Annotations, "gardener.cloud/operation")
					g.Expect(testClient.Patch(ctx, etcd, patch)).To(Succeed())
				}

				makeHealthy(etcdMain)
				makeHealthy(etcdEvents)
			}).Should(Succeed())
		}
	})

	Context("machine without addresses", Ordered, func() {
		It("should not add a finalizer or mutate the ETCD objects", func() {
			// Machine has no .status.addresses, so the controller should exit early.
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
				g.Expect(machine.Finalizers).NotTo(ContainElement("gardenlet.gardener.cloud/etcd-member"))
			}).Should(Succeed())

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(etcdMain), etcdMain)).To(Succeed())
			Expect(etcdMain.Spec.ExternallyManagedMemberAddresses).To(ConsistOf("192.168.0.1"))
		})
	})

	Context("machine with an address", Ordered, func() {
		const machineIP = "10.0.0.1"

		It("should report machine addresses", func() {
			patch := client.MergeFrom(machine.DeepCopy())
			machine.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: machineIP}}
			Expect(testClient.Status().Patch(ctx, machine, patch)).To(Succeed())
		})

		It("should add the finalizer", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
				g.Expect(machine.Finalizers).To(ContainElement("gardenlet.gardener.cloud/etcd-member"))
			}).Should(Succeed())
		})

		It("should add the address to etcd-main and increase replicas", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(etcdMain), etcdMain)).To(Succeed())
				g.Expect(etcdMain.Spec.ExternallyManagedMemberAddresses).To(ContainElement(machineIP))
				g.Expect(etcdMain.Spec.Replicas).To(BeEquivalentTo(2))
			}).Should(Succeed())
		})

		It("should add the address to etcd-events and increase replicas", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(etcdEvents), etcdEvents)).To(Succeed())
				g.Expect(etcdEvents.Spec.ExternallyManagedMemberAddresses).To(ContainElement(machineIP))
				g.Expect(etcdEvents.Spec.Replicas).To(BeEquivalentTo(2))
			}).Should(Succeed())
		})

		It("should ensure Etcd reconciliation is succeeded", func() {
			ensureEtcdReconciliationSucceeded()
		})

		It("should ensure controller waits for Etcd reconciliation", func() {
			Eventually(func() string { return logBuffer.String() }).Should(ContainSubstring(`"reason":"AddedToETCD"`))
		})

		It("should trigger deletion of machine", func() {
			Expect(testClient.Delete(ctx, machine)).To(Succeed())
		})

		It("should set the InstanceDeletionSuspended condition on machine", func() {
			patch := client.MergeFrom(machine.DeepCopy())
			machine.Status.Conditions = []corev1.NodeCondition{{Type: "InstanceDeletionSuspended", Status: corev1.ConditionTrue}}
			Expect(testClient.Status().Patch(ctx, machine, patch)).To(Succeed())
		})

		It("should remove the address from etcd-main and decrease replicas", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(etcdMain), etcdMain)).To(Succeed())
				g.Expect(etcdMain.Spec.ExternallyManagedMemberAddresses).NotTo(ContainElement(machineIP))
				g.Expect(etcdMain.Spec.Replicas).To(BeEquivalentTo(1))
			}).Should(Succeed())
		})

		It("should remove the address from etcd-events and decrease replicas", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(etcdEvents), etcdEvents)).To(Succeed())
				g.Expect(etcdEvents.Spec.ExternallyManagedMemberAddresses).NotTo(ContainElement(machineIP))
				g.Expect(etcdEvents.Spec.Replicas).To(BeEquivalentTo(1))
			}).Should(Succeed())
		})

		It("should ensure Etcd reconciliation is succeeded", func() {
			ensureEtcdReconciliationSucceeded()
		})

		It("should ensure controller waits for Etcd reconciliation", func() {
			Eventually(func() string { return logBuffer.String() }).Should(ContainSubstring(`"reason":"RemovedFromETCD"`))
		})

		It("should remove the suspend annotation and the finalizer, thus proceed with object deletion", func() {
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)
			}).Should(BeNotFoundError())
		})
	})
})
