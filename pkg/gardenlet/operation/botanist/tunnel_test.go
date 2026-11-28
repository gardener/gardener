// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	"k8s.io/apimachinery/pkg/util/sets"
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

var _ = Describe("VPN client verification", func() {
	Describe("ParseConnectedVPNClients", func() {
		It("should extract common names from per-client series", func() {
			metrics := `# HELP openvpn_server_client_received_bytes_total total
# TYPE openvpn_server_client_received_bytes_total counter
openvpn_server_client_received_bytes_total{common_name="vpn-shoot-client-0",real_address="1.2.3.4:5"} 123
openvpn_server_client_sent_bytes_total{common_name="vpn-shoot-client-0",real_address="1.2.3.4:5"} 456
openvpn_server_client_received_bytes_total{real_address="5.6.7.8:9",common_name="vpn-shoot-client-temp-0"} 7
openvpn_up 1
`
			Expect(botanist.ParseConnectedVPNClients(metrics)).To(Equal(sets.New("vpn-shoot-client-0", "vpn-shoot-client-temp-0")))
		})

		It("should return an empty set when there are no per-client series", func() {
			Expect(botanist.ParseConnectedVPNClients("openvpn_up 1\n# comment line\n")).To(BeEmpty())
		})

		It("should ignore per-client series without a common_name label", func() {
			Expect(botanist.ParseConnectedVPNClients(`openvpn_server_client_received_bytes_total{real_address="1.2.3.4:5"} 1`)).To(BeEmpty())
		})
	})

	Describe("ExpectedVPNClientCNs", func() {
		It("should build permanent client common names", func() {
			Expect(botanist.ExpectedVPNClientCNs("", 2)).To(Equal(sets.New("vpn-shoot-client-0", "vpn-shoot-client-1")))
		})

		It("should build temporary client common names", func() {
			Expect(botanist.ExpectedVPNClientCNs("temp", 2)).To(Equal(sets.New("vpn-shoot-client-temp-0", "vpn-shoot-client-temp-1")))
		})
	})
})
