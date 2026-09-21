// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers_test

import (
	"context"
	"net"
	"strings"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/nodeagent/bootstrappers"
)

var _ = Describe("ControlPlaneNodesEndpoints", func() {
	var (
		ctx context.Context

		fakeFS afero.Afero

		bootstrapper *ControlPlaneNodesEndpoints

		filePath = "/var/lib/etcd/control-plane-nodes-endpoints"

		node1 = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-1",
				Labels: map[string]string{v1beta1constants.LabelNodeRoleControlPlane: ""},
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
				},
			},
		}
		node2 = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-2",
				Labels: map[string]string{v1beta1constants.LabelNodeRoleControlPlane: ""},
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "10.0.0.2"},
				},
			},
		}
		workerNode = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-3",
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "10.0.0.3"},
				},
			},
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
	})

	JustBeforeEach(func() {
		bootstrapper = &ControlPlaneNodesEndpoints{
			Log:    logr.Discard(),
			FS:     fakeFS,
			Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(node1, node2, workerNode).WithStatusSubresource(node1, node2, workerNode).Build(),
		}
	})

	Describe("Start", func() {
		When("the file already exists", func() {
			BeforeEach(func() {
				Expect(fakeFS.WriteFile(filePath, []byte("10.0.0.99"), 0600)).To(Succeed())
			})

			It("should not overwrite the file", func() {
				Expect(bootstrapper.Start(ctx)).To(Succeed())

				content, err := fakeFS.ReadFile(filePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("10.0.0.99"))
			})
		})

		When("the file does not exist yet", func() {
			It("should write the IP addresses of all control plane nodes, newline-separated and excluding worker nodes", func() {
				Expect(bootstrapper.Start(ctx)).To(Succeed())

				content, err := fakeFS.ReadFile(filePath)
				Expect(err).NotTo(HaveOccurred())

				lines := strings.Split(string(content), "\n")
				Expect(lines).To(HaveLen(2))
				for _, line := range lines {
					Expect(net.ParseIP(line)).NotTo(BeNil(), "expected %q to be a valid IP address", line)
				}
				Expect(lines).To(ConsistOf("10.0.0.1", "10.0.0.2"))
			})
		})
	})
})
