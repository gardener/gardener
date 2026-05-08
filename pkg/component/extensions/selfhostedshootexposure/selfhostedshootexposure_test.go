// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/selfhostedshootexposure"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("SelfHostedShootExposure", func() {
	const (
		name      = "test-shoot"
		namespace = "kube-system"
		extType   = "local"
	)

	var (
		ctx context.Context
		c   client.Client

		fakeClock *testclock.FakeClock
		now       time.Time

		values   *Values
		deployer Interface

		expected *extensionsv1alpha1.SelfHostedShootExposure
	)

	BeforeEach(func() {
		ctx = context.TODO()
		now = time.Unix(60, 0)
		fakeClock = testclock.NewFakeClock(now)

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).To(Succeed())
		Expect(gardencorev1beta1.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		values = &Values{
			Name:      name,
			Namespace: namespace,
			Type:      extType,
			Port:      443,
		}

		expected = &extensionsv1alpha1.SelfHostedShootExposure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
				},
			},
			Spec: extensionsv1alpha1.SelfHostedShootExposureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: extType,
				},
				Port: 443,
			},
		}

		deployer = New(logr.Discard(), c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	Describe("#Deploy", func() {
		It("should create the resource with correct spec and annotations", func() {
			defer test.WithVars(&TimeNow, fakeClock.Now)()

			Expect(deployer.Deploy(ctx)).To(Succeed())

			actual := &extensionsv1alpha1.SelfHostedShootExposure{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, actual)).To(Succeed())
			Expect(actual).To(DeepDerivativeEqual(expected))
		})

		It("should include CredentialsRef when set", func() {
			defer test.WithVars(&TimeNow, fakeClock.Now)()

			credRef := &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "cloudprovider",
				Namespace:  namespace,
			}
			values.CredentialsRef = credRef
			deployer = New(logr.Discard(), c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)

			Expect(deployer.Deploy(ctx)).To(Succeed())

			actual := &extensionsv1alpha1.SelfHostedShootExposure{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, actual)).To(Succeed())
			Expect(actual.Spec.CredentialsRef).To(Equal(credRef))
		})

		It("should update endpoints set via SetEndpoints", func() {
			defer test.WithVars(&TimeNow, fakeClock.Now)()

			endpoints := []extensionsv1alpha1.ControlPlaneEndpoint{
				{
					NodeName: "node-1",
					Addresses: []corev1.NodeAddress{
						{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
					},
				},
			}
			deployer.SetEndpoints(endpoints)
			Expect(deployer.Deploy(ctx)).To(Succeed())

			actual := &extensionsv1alpha1.SelfHostedShootExposure{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, actual)).To(Succeed())
			Expect(actual.Spec.Endpoints).To(Equal(endpoints))
		})
	})

	Describe("#Wait", func() {
		It("should return error when the resource does not exist", func() {
			Expect(deployer.Wait(ctx)).To(Not(Succeed()))
		})

		It("should return nil when the resource is ready", func() {
			obj := expected.DeepCopy()
			delete(obj.Annotations, v1beta1constants.GardenerOperation)
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Time{Time: now.Add(time.Second)},
			}
			Expect(c.Create(ctx, obj)).To(Succeed())

			Expect(deployer.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when the resource does not exist", func() {
			Expect(deployer.Destroy(ctx)).To(Succeed())
		})

		It("should delete the resource when it exists", func() {
			Expect(c.Create(ctx, expected)).To(Succeed())
			Expect(deployer.Destroy(ctx)).To(Succeed())

			actual := &extensionsv1alpha1.SelfHostedShootExposure{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, actual)).To(BeNotFoundError())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when the resource is already gone", func() {
			Expect(deployer.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
