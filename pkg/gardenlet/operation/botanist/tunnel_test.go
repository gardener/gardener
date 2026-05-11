// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Tunnel", func() {
	Describe("CheckTunnelConnection", func() {
		var (
			ctx        context.Context
			fakeClient client.Client
			clientset  *fake.ClientSet
			log        logr.Logger
			tunnelName string
			tunnelPod  corev1.Pod
		)

		BeforeEach(func() {
			ctx = context.Background()
			log = logr.Discard()
			tunnelName = "vpn-shoot"
			tunnelPod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceSystem,
					Name:      tunnelName,
					Labels:    map[string]string{"app": tunnelName},
				},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).WithStatusSubresource(&corev1.Pod{}).Build()
			clientset = fake.NewClientSetBuilder().WithClient(fakeClient).Build()
		})

		Context("unavailable tunnel pod", func() {
			It("should fail because pod does not exist", func() {
				done, err := botanist.CheckTunnelConnection(ctx, log, clientset, tunnelName)
				Expect(done).To(BeFalse())
				Expect(err).To(HaveOccurred())
			})

			It("should fail because pod list returns error", func() {
				fakeClientWithErr := fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return errors.New("foo")
					},
				}).Build()
				clientset = fake.NewClientSetBuilder().WithClient(fakeClientWithErr).Build()

				done, err := botanist.CheckTunnelConnection(ctx, log, clientset, tunnelName)
				Expect(done).To(BeTrue())
				Expect(err).To(HaveOccurred())
			})

			It("should fail because pod is not running", func() {
				Expect(fakeClient.Create(ctx, &tunnelPod)).To(Succeed())

				done, err := botanist.CheckTunnelConnection(ctx, log, clientset, tunnelName)
				Expect(done).To(BeFalse())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("available tunnel pod", func() {
			BeforeEach(func() {
				tunnelPod.Status = corev1.PodStatus{
					Phase: corev1.PodRunning,
				}

				Expect(fakeClient.Create(ctx, &tunnelPod)).To(Succeed())
			})

			Context("established connection", func() {
				It("should succeed because pod is running and connection successful", func() {
					fw := fake.PortForwarder{
						ReadyChan: make(chan struct{}, 1),
					}

					defer test.WithVar(&botanist.SetupPortForwarder, func(context.Context, *rest.Config, string, string, int, int) (kubernetes.PortForwarder, error) {
						return fw, nil
					})()
					close(fw.ReadyChan)

					done, err := botanist.CheckTunnelConnection(ctx, log, clientset, tunnelName)
					Expect(done).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("broken connection", func() {
				It("should fail because pod is running but connection is not established", func() {
					defer test.WithVar(&botanist.SetupPortForwarder, func(context.Context, *rest.Config, string, string, int, int) (kubernetes.PortForwarder, error) {
						return nil, errors.New("foo")
					})()

					done, err := botanist.CheckTunnelConnection(ctx, log, clientset, tunnelName)
					Expect(done).To(BeFalse())
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
