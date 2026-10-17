// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Restore", func() {
	const (
		priorNodeName      = "prior-node"
		oscSecretName      = "gardener-node-agent-control-plane-abc123"
		oscSecretNamespace = "kube-system"
	)

	var (
		b *GardenadmBotanist

		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.SeedScheme).
			WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
			Build()

		b = &GardenadmBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					Logger: logr.Discard(),
					Shoot:  &shootpkg.Shoot{ControlPlaneNamespace: oscSecretNamespace},
				},
			},
		}
	})

	Describe("#DeletePriorNode", func() {
		It("should error when priorNodeName is empty", func(ctx SpecContext) {
			Expect(b.DeletePriorNode(ctx, fakeClient, "")).To(MatchError(ContainSubstring("must not be empty")))
		})

		It("should not fail if the prior Node does not exist", func(ctx SpecContext) {
			Expect(b.DeletePriorNode(ctx, fakeClient, priorNodeName)).To(Succeed())
		})

		It("should delete the prior Node", func(ctx SpecContext) {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: priorNodeName}}
			other := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "other-node"}}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())
			Expect(fakeClient.Create(ctx, other)).To(Succeed())

			Expect(b.DeletePriorNode(ctx, fakeClient, priorNodeName)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(other), other)).To(Succeed())
		})
	})

	Describe("#ForceDeletePriorNodePods", func() {
		podOnNode := func(name, namespace, nodeName string) *corev1.Pod {
			return &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec:       corev1.PodSpec{NodeName: nodeName},
			}
		}

		It("should error when priorNodeName is empty", func(ctx SpecContext) {
			Expect(b.ForceDeletePriorNodePods(ctx, fakeClient, "")).To(MatchError(ContainSubstring("must not be empty")))
		})

		It("should not fail if there are no Pods running on the prior Node", func(ctx SpecContext) {
			Expect(b.ForceDeletePriorNodePods(ctx, fakeClient, priorNodeName)).To(Succeed())
		})

		It("should delete all Pods running on the prior Node", func(ctx SpecContext) {
			pod1 := podOnNode("pod-1", "kube-system", priorNodeName)
			pod2 := podOnNode("pod-2", "default", priorNodeName)
			pod3 := podOnNode("pod-3", "some-namespace", priorNodeName)
			pod4 := podOnNode("pod-4", "kube-system", "other-node")
			pod5 := podOnNode("pod-5", "kube-system", "")
			Expect(fakeClient.Create(ctx, pod1)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod2)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod3)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod4)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod5)).To(Succeed())

			Expect(b.ForceDeletePriorNodePods(ctx, fakeClient, priorNodeName)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod1), pod1)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod2), pod2)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod3), pod3)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod4), pod4)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod5), pod5)).To(Succeed())
		})
	})

	Describe("#DeleteStaleOperatingSystemConfigSecret", func() {
		BeforeEach(func() {
			b.operatingSystemConfigSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: oscSecretName, Namespace: oscSecretNamespace},
			}
		})

		It("should delete the stale OperatingSystemConfig Secret restored from the ETCD snapshot", func(ctx SpecContext) {
			restoredSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: oscSecretName, Namespace: oscSecretNamespace},
				Data:       map[string][]byte{"osc.yaml": []byte("managed-content")},
			}
			Expect(fakeClient.Create(ctx, restoredSecret)).To(Succeed())

			Expect(b.DeleteStaleOperatingSystemConfigSecret(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(restoredSecret), &corev1.Secret{})).To(BeNotFoundError())
		})

		It("should succeed when the Secret is absent (IgnoreNotFound)", func(ctx SpecContext) {
			Expect(b.DeleteStaleOperatingSystemConfigSecret(ctx, fakeClient)).To(Succeed())
		})

		It("should error when the OperatingSystemConfig Secret was not computed yet", func(ctx SpecContext) {
			b.operatingSystemConfigSecret = nil

			Expect(b.DeleteStaleOperatingSystemConfigSecret(ctx, fakeClient)).To(MatchError(ContainSubstring("operating system config secret is nil")))
		})
	})

	Describe("#FinalizeGardenerNodeAgentManagedResource", func() {
		managedResource := func(finalizers ...string) *resourcesv1alpha1.ManagedResource {
			return &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       botanistpkg.GardenerNodeAgentManagedResourceName,
					Namespace:  oscSecretNamespace,
					Finalizers: finalizers,
				},
			}
		}

		It("should succeed when the ManagedResource is absent", func(ctx SpecContext) {
			Expect(b.FinalizeGardenerNodeAgentManagedResource(ctx, fakeClient)).To(Succeed())
		})

		It("should delete the ManagedResource without finalizers", func(ctx SpecContext) {
			mr := managedResource()
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			Expect(b.FinalizeGardenerNodeAgentManagedResource(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
		})

		It("should remove the finalizer so the ManagedResource is deleted even without a running gardener-resource-manager", func(ctx SpecContext) {
			mr := managedResource("resources.gardener.cloud/gardener-resource-manager")
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			Expect(b.FinalizeGardenerNodeAgentManagedResource(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
		})
	})

	Describe("#DeleteStaleEtcdMemberLeases", func() {
		var (
			etcdClient client.Client

			etcdMain   *druidcorev1alpha1.Etcd
			etcdEvents *druidcorev1alpha1.Etcd

			mainMemberLease   *coordinationv1.Lease
			eventsMemberLease *coordinationv1.Lease
			deltaSnapLease    *coordinationv1.Lease
			fullSnapLease     *coordinationv1.Lease

			webhookConfig *admissionregistrationv1.ValidatingWebhookConfiguration
		)

		etcd := func(name, memberAddress string, withMembers bool) *druidcorev1alpha1.Etcd {
			e := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: oscSecretNamespace},
				Spec: druidcorev1alpha1.EtcdSpec{
					Replicas:                         1,
					ExternallyManagedMemberAddresses: []string{memberAddress},
				},
			}
			if withMembers {
				e.Status.Members = []druidcorev1alpha1.EtcdMemberStatus{{
					Name:   druidcorev1alpha1.GetMemberNameFromAddress(e, memberAddress),
					Role:   new(druidcorev1alpha1.EtcdRoleLeader),
					Status: druidcorev1alpha1.EtcdMemberStatusReady,
					Reason: "member is ready",
				}}
			}
			return e
		}

		lease := func(name string) *coordinationv1.Lease {
			return &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: oscSecretNamespace}}
		}

		BeforeEach(func() {
			// Status().Patch on the Etcd requires the status subresource to be registered on the fake client.
			etcdClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithStatusSubresource(&druidcorev1alpha1.Etcd{}).
				Build()

			etcdMain = etcd("etcd-main", "172.18.0.9", true)
			etcdEvents = etcd("etcd-events", "172.18.0.9", true)

			// Member leases: etcd-druid derives these from ExternallyManagedMemberAddresses (etcd-<name>-<address>).
			mainMemberLease = lease(druidcorev1alpha1.GetMemberLeaseNames(etcdMain)[0])
			eventsMemberLease = lease(druidcorev1alpha1.GetMemberLeaseNames(etcdEvents)[0])
			// Snapshot leases that etcd-backup-restore depends on - must be retained.
			deltaSnapLease = lease("etcd-main-delta-snap")
			fullSnapLease = lease("etcd-main-full-snap")

			webhookConfig = &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "etcd-druid"}}
		})

		It("should succeed and delete the webhook config when no Etcd resources exist", func(ctx SpecContext) {
			Expect(etcdClient.Create(ctx, webhookConfig)).To(Succeed())

			Expect(b.DeleteStaleEtcdMemberLeases(ctx, etcdClient)).To(Succeed())

			Expect(etcdClient.Get(ctx, client.ObjectKeyFromObject(webhookConfig), &admissionregistrationv1.ValidatingWebhookConfiguration{})).To(BeNotFoundError())
		})

		It("should succeed when nothing exists", func(ctx SpecContext) {
			Expect(b.DeleteStaleEtcdMemberLeases(ctx, etcdClient)).To(Succeed())
		})

		It("should delete the member Leases, retain the snapshot Leases, and clear the status members", func(ctx SpecContext) {
			Expect(etcdClient.Create(ctx, etcdMain)).To(Succeed())
			Expect(etcdClient.Create(ctx, etcdEvents)).To(Succeed())
			Expect(etcdClient.Status().Update(ctx, etcdMain)).To(Succeed())
			Expect(etcdClient.Status().Update(ctx, etcdEvents)).To(Succeed())
			Expect(etcdClient.Create(ctx, mainMemberLease)).To(Succeed())
			Expect(etcdClient.Create(ctx, eventsMemberLease)).To(Succeed())
			Expect(etcdClient.Create(ctx, deltaSnapLease)).To(Succeed())
			Expect(etcdClient.Create(ctx, fullSnapLease)).To(Succeed())
			Expect(etcdClient.Create(ctx, webhookConfig)).To(Succeed())

			Expect(b.DeleteStaleEtcdMemberLeases(ctx, etcdClient)).To(Succeed())

			By("The etcd-druid ValidatingWebhookConfiguration is deleted")
			Expect(etcdClient.Get(ctx, client.ObjectKeyFromObject(webhookConfig), &admissionregistrationv1.ValidatingWebhookConfiguration{})).To(BeNotFoundError())

			By("Member Leases are deleted")
			Expect(etcdClient.Get(ctx, client.ObjectKeyFromObject(mainMemberLease), &coordinationv1.Lease{})).To(BeNotFoundError())
			Expect(etcdClient.Get(ctx, client.ObjectKeyFromObject(eventsMemberLease), &coordinationv1.Lease{})).To(BeNotFoundError())

			By("Snapshot Leases are retained")
			Expect(etcdClient.Get(ctx, client.ObjectKeyFromObject(deltaSnapLease), &coordinationv1.Lease{})).To(Succeed())
			Expect(etcdClient.Get(ctx, client.ObjectKeyFromObject(fullSnapLease), &coordinationv1.Lease{})).To(Succeed())

			By("Status members are cleared")
			gotMain := &druidcorev1alpha1.Etcd{}
			Expect(etcdClient.Get(ctx, client.ObjectKeyFromObject(etcdMain), gotMain)).To(Succeed())
			Expect(gotMain.Status.Members).To(BeEmpty())
			gotEvents := &druidcorev1alpha1.Etcd{}
			Expect(etcdClient.Get(ctx, client.ObjectKeyFromObject(etcdEvents), gotEvents)).To(Succeed())
			Expect(gotEvents.Status.Members).To(BeEmpty())
		})

		It("should be a no-op for the member Leases that are already absent and still clear the status", func(ctx SpecContext) {
			Expect(etcdClient.Create(ctx, etcdMain)).To(Succeed())
			Expect(etcdClient.Status().Update(ctx, etcdMain)).To(Succeed())

			Expect(b.DeleteStaleEtcdMemberLeases(ctx, etcdClient)).To(Succeed())

			got := &druidcorev1alpha1.Etcd{}
			Expect(etcdClient.Get(ctx, client.ObjectKeyFromObject(etcdMain), got)).To(Succeed())
			Expect(got.Status.Members).To(BeEmpty())
		})

		It("should not delete member Leases of Etcds in a different namespace", func(ctx SpecContext) {
			otherEtcd := etcd("etcd-main", "172.18.0.9", true)
			otherEtcd.Namespace = "other-namespace"
			otherLease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: mainMemberLease.Name, Namespace: "other-namespace"}}
			Expect(etcdClient.Create(ctx, otherEtcd)).To(Succeed())
			Expect(etcdClient.Create(ctx, otherLease)).To(Succeed())

			Expect(b.DeleteStaleEtcdMemberLeases(ctx, etcdClient)).To(Succeed())

			Expect(etcdClient.Get(ctx, client.ObjectKeyFromObject(otherLease), &coordinationv1.Lease{})).To(Succeed())
		})
	})
})
