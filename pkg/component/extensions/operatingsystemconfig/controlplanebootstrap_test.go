// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	"k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("controlPlaneBootstrap", func() {
	const (
		namespace = "test-namespace"
	)

	var (
		ctx      context.Context
		c        client.Client
		values   *ControlPlaneBootstrapValues
		worker   *gardencorev1beta1.Worker
		deployer Interface
		clk      *testing.FakePassiveClock
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		clk = testing.NewFakePassiveClock(time.Now())

		DeferCleanup(test.WithVars(
			&TimeNow, clk.Now,
		))

		worker = &gardencorev1beta1.Worker{
			Name: "control-plane",
			Machine: gardencorev1beta1.Machine{
				Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
				Image: &gardencorev1beta1.ShootMachineImage{
					Name:    "type1",
					Version: ptr.To("12.34"),
					ProviderConfig: &runtime.RawExtension{
						Raw: []byte(`{"foo":"bar"}`),
					},
				},
			},
			ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
		}

		values = &ControlPlaneBootstrapValues{
			Namespace:      namespace,
			Worker:         worker,
			GardenadmImage: "gardenadm-image",
		}
		deployer = NewControlPlaneBootstrap(logr.Discard(), c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	Describe("#Deploy", func() {
		It("Should correctly deploy the OSC", func() {
			Expect(deployer.Deploy(ctx)).To(Succeed())
			actual := &extensionsv1alpha1.OperatingSystemConfig{}
			Expect(c.Get(ctx, client.ObjectKey{Name: oscName(worker), Namespace: namespace}, actual)).To(Succeed())

			Expect(actual.Annotations).To(And(
				HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile),
				HaveKeyWithValue(v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano)),
			))

			Expect(actual.Spec.Purpose).To(Equal(extensionsv1alpha1.OperatingSystemConfigPurposeProvision))
			// The content of units/files is not tested here, because it is tested in the nodeinit package
			Expect(actual.Spec.Units).To(ConsistOf(
				HaveField("Name", "gardenadm-download.service"),
			))
			Expect(actual.Spec.Files).To(ConsistOf(And(
				HaveField("Path", nodeinit.GardenadmPathDownloadScript),
				HaveField("Permissions", ptr.To(uint32(0755))),
				HaveField("Content.Inline.Encoding", "b64"),
				HaveField("Content.Inline.Data", Not(BeEmpty())),
			)))

			Expect(actual.Spec.Type).To(Equal("type1"))
			Expect(actual.Spec.ProviderConfig).To(Equal(worker.Machine.Image.ProviderConfig))
		})
	})

	Describe("#Wait", func() {
		var osc *extensionsv1alpha1.OperatingSystemConfig

		BeforeEach(func() {
			Expect(deployer.Deploy(ctx)).To(Succeed())
			osc = &extensionsv1alpha1.OperatingSystemConfig{}
			Expect(c.Get(ctx, client.ObjectKey{Name: oscName(worker), Namespace: namespace}, osc)).To(Succeed())
		})

		It("Should wait for the OSC to be ready", func() {
			Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not yet picked up by controller")))

			By("updating the OSC to fake a reconciliation")
			ccSecret := fakeOSCReconcile(ctx, clk, c, osc)

			Expect(deployer.Wait(ctx)).To(BeNotFoundError(), "expected deployer to check if the cloud-config secret exists")

			By("creating the cloud-config secret")
			Expect(c.Create(ctx, ccSecret)).To(Succeed())

			Expect(deployer.Wait(ctx)).To(Succeed())
		})
	})

	Describe("Stale Resources", func() {
		var staleOSC *extensionsv1alpha1.OperatingSystemConfig

		BeforeEach(func() {
			staleOSC = &extensionsv1alpha1.OperatingSystemConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "foo",
					Namespace:  namespace,
					Finalizers: []string{"gardener.cloud/finalizer"},
				},
			}
			Expect(c.Create(ctx, staleOSC)).To(Succeed())
		})

		It("should delete stale OSCs", func() {
			Expect(deployer.DeleteStaleResources(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(staleOSC), staleOSC)).To(Succeed())
			Expect(staleOSC.DeletionTimestamp.IsZero()).To(BeFalse())
		})

		It("should wait for the cleanup", func() {
			Expect(deployer.DeleteStaleResources(ctx)).To(Succeed())
			Expect(deployer.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("is still present")))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(staleOSC), staleOSC)).To(Succeed())
			staleOSC.Finalizers = nil
			Expect(c.Update(ctx, staleOSC)).To(Succeed())

			Expect(deployer.WaitCleanupStaleResources(ctx)).To(Succeed())
		})
	})

	Describe("#WorkerPoolNameToOperatingSystemConfigsMap", func() {
		var (
			osc      *extensionsv1alpha1.OperatingSystemConfig
			ccSecret *corev1.Secret
		)

		BeforeEach(func() {
			Expect(deployer.Deploy(ctx)).To(Succeed())
			osc = &extensionsv1alpha1.OperatingSystemConfig{}
			Expect(c.Get(ctx, client.ObjectKey{Name: oscName(worker), Namespace: namespace}, osc)).To(Succeed())
			ccSecret = fakeOSCReconcile(ctx, clk, c, osc)
			Expect(c.Create(ctx, ccSecret)).To(Succeed())
			Expect(deployer.Wait(ctx)).To(Succeed())
		})

		It("should return the correct result from the Deploy and Wait operations", func() {
			Expect(deployer.WorkerPoolNameToOperatingSystemConfigsMap()).To(Equal(map[string]*OperatingSystemConfigs{
				worker.Name: {
					Init: Data{
						SecretName: ptr.To(ccSecret.Name),
					},
				},
			}))
		})
	})
})

func oscName(worker *gardencorev1beta1.Worker) string {
	return fmt.Sprintf("gardenadm-%s", worker.Name)
}

func fakeOSCReconcile(ctx context.Context, clk clock.PassiveClock, c client.Client, osc *extensionsv1alpha1.OperatingSystemConfig) *corev1.Secret {
	GinkgoHelper()
	ccSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cc-" + osc.Name,
			Namespace: osc.Namespace,
		},
	}
	patch := client.MergeFrom(osc.DeepCopy())
	delete(osc.Annotations, v1beta1constants.GardenerOperation)
	osc.Status.LastOperation = &gardencorev1beta1.LastOperation{
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		LastUpdateTime: metav1.Time{Time: clk.Now().UTC().Add(time.Second)},
	}
	osc.Status.CloudConfig = &extensionsv1alpha1.CloudConfig{
		SecretRef: corev1.SecretReference{
			Name:      ccSecret.Name,
			Namespace: ccSecret.Namespace,
		},
	}
	Expect(c.Patch(ctx, osc, patch)).To(Succeed())
	return ccSecret
}
