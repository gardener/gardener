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
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Tunnel", func() {
	Describe("CheckTunnelConnection", func() {
		var (
			ctrl *gomock.Controller

			ctx        context.Context
			cl         *mockclient.MockClient
			clientset  *fake.ClientSet
			log        logr.Logger
			tunnelName string
			tunnelPod  corev1.Pod
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			ctx = context.Background()
			cl = mockclient.NewMockClient(ctrl)
			log = logr.Discard()
			tunnelName = "vpn-shoot"
			tunnelPod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceSystem,
					Name:      tunnelName,
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		JustBeforeEach(func() {
			clientset = fake.NewClientSetBuilder().
				WithClient(cl).
				Build()
		})

		Context("unavailable tunnel pod", func() {
			It("should fail because pod does not exist", func() {
				cl.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{"app": tunnelName}).Return(nil)
				done, err := botanist.CheckTunnelConnection(context.Background(), log, clientset, tunnelName)
				Expect(done).To(BeFalse())
				Expect(err).To(HaveOccurred())
			})
			It("should fail because pod list returns error", func() {
				cl.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{"app": tunnelName}).Return(errors.New("foo"))
				done, err := botanist.CheckTunnelConnection(context.Background(), log, clientset, tunnelName)
				Expect(done).To(BeTrue())
				Expect(err).To(HaveOccurred())
			})
			It("should fail because pod is not running", func() {
				cl.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{"app": tunnelName}).DoAndReturn(
					func(_ context.Context, podList *corev1.PodList, _ ...client.ListOption) error {
						podList.Items = append(podList.Items, tunnelPod)
						return nil
					})
				done, err := botanist.CheckTunnelConnection(context.Background(), log, clientset, tunnelName)
				Expect(done).To(BeFalse())
				Expect(err).To(HaveOccurred())
			})
		})
		Context("available tunnel pod", func() {
			BeforeEach(func() {
				tunnelPod.Status = corev1.PodStatus{
					Phase: corev1.PodRunning,
				}
				cl.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{"app": tunnelName}).DoAndReturn(
					func(_ context.Context, podList *corev1.PodList, _ ...client.ListOption) error {
						podList.Items = append(podList.Items, tunnelPod)
						return nil
					})
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

					done, err := botanist.CheckTunnelConnection(context.Background(), log, clientset, tunnelName)
					Expect(done).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())
				})
			})
			Context("broken connection", func() {
				It("should fail because pod is running but connection is not established", func() {
					defer test.WithVar(&botanist.SetupPortForwarder, func(context.Context, *rest.Config, string, string, int, int) (kubernetes.PortForwarder, error) {
						return nil, errors.New("foo")
					})()

					done, err := botanist.CheckTunnelConnection(context.Background(), log, clientset, tunnelName)
					Expect(done).To(BeFalse())
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
