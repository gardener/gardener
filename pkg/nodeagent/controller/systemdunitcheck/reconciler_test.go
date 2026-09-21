// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package systemdunitcheck_test

import (
	"context"
	"time"

	systemddbus "github.com/coreos/go-systemd/v22/dbus"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/nodeagent"
	. "github.com/gardener/gardener/pkg/nodeagent/controller/systemdunitcheck"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
)

var _ = Describe("Reconciler", func() {
	var (
		fakeClient client.Client
		fakeDBus   *fakedbus.DBus
		fakeFS     afero.Afero
		fakeClock  *testclock.FakeClock

		node       *corev1.Node
		reconciler *Reconciler

		// concurrentUpdate is executed by the client interceptor right after the reconciler fetched the node, i.e., it
		// simulates a concurrent update of the node by another component.
		concurrentUpdate func(ctx context.Context, c client.Client)
	)

	BeforeEach(func() {
		fakeDBus = fakedbus.New()
		fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
		fakeClock = testclock.NewFakeClock(time.Now())
		concurrentUpdate = nil

		node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node"}}
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.SeedScheme).
			WithObjects(node).
			WithStatusSubresource(&corev1.Node{}).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					err := c.Get(ctx, key, obj, opts...)
					if concurrentUpdate != nil {
						update := concurrentUpdate
						concurrentUpdate = nil
						update(ctx, c)
					}
					return err
				},
			}).
			Build()

		reconciler = &Reconciler{
			Client: fakeClient,
			DBus:   fakeDBus,
			Clock:  fakeClock,
			FS:     fakeFS,
			Config: nodeagentconfigv1alpha1.SystemdUnitCheckControllerConfig{
				SyncPeriod:     &metav1.Duration{Duration: time.Minute},
				StuckThreshold: &metav1.Duration{Duration: 5 * time.Minute},
			},
		}

		osc := &extensionsv1alpha1.OperatingSystemConfig{
			TypeMeta: metav1.TypeMeta{APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(), Kind: "OperatingSystemConfig"},
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{{Name: "kubelet.service", Enable: new(true)}},
			},
		}
		data, err := runtime.Encode(nodeagent.Codec, osc)
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeFS.WriteFile(nodeagentconfigv1alpha1.LastAppliedOperatingSystemConfigFilePath, data, 0600)).To(Succeed())

		fakeDBus.SetUnits(systemddbus.UnitStatus{Name: "kubelet.service", ActiveState: "active", SubState: "running"})
	})

	reconcileNode := func(ctx context.Context) (reconcile.Result, error) {
		return reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(node)})
	}

	It("should add the SystemdUnitsReady condition", func(ctx SpecContext) {
		result, err := reconcileNode(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{RequeueAfter: time.Minute}))

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		Expect(node.Status.Conditions).To(ConsistOf(And(
			HaveField("Type", nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			HaveField("Status", corev1.ConditionTrue),
			HaveField("Reason", "AllUnitsHealthy"),
		)))
	})

	It("should not drop conditions concurrently added by other components", func(ctx SpecContext) {
		networkUnavailable := corev1.NodeCondition{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse, Reason: "CalicoIsUp"}

		concurrentUpdate = func(ctx context.Context, c client.Client) {
			concurrentNode := node.DeepCopy()
			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), concurrentNode)).To(Succeed())
			concurrentNode.Status.Conditions = append(concurrentNode.Status.Conditions, networkUnavailable)
			Expect(c.Status().Update(ctx, concurrentNode)).To(Succeed())
		}

		By("First reconciliation fails because the node was updated concurrently")
		_, err := reconcileNode(ctx)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsConflict(err)).To(BeTrue(), "expected conflict error, got %v", err)

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		Expect(node.Status.Conditions).To(ConsistOf(HaveField("Type", corev1.NodeNetworkUnavailable)))

		By("Second reconciliation succeeds and keeps the concurrently added condition")
		_, err = reconcileNode(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		Expect(node.Status.Conditions).To(ConsistOf(
			HaveField("Type", corev1.NodeNetworkUnavailable),
			HaveField("Type", nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
		))
	})
})
