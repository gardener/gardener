// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/gardenadm/staticpod"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("OperatingSystemConfig - ETCD", func() {
	var (
		ctx        = context.Background()
		namespace  = "shoot--foo--bar"
		fakeClient client.Client
		b          *GardenadmBotanist
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		b = &GardenadmBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					SeedClientSet: fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build(),
					Shoot: &shootpkg.Shoot{
						ControlPlaneNamespace: namespace,
					},
				},
			},
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				ControlPlane: &gardencorev1beta1.ControlPlane{
					HighAvailability: &gardencorev1beta1.HighAvailability{
						FailureTolerance: gardencorev1beta1.FailureTolerance{
							Type: gardencorev1beta1.FailureToleranceTypeNode,
						},
					},
				},
			},
		})
	})

	Describe("#getControlPlaneHostNames", func() {
		It("should return hostnames from all control plane nodes", func() {
			for _, node := range []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node1",
						Labels: map[string]string{"node-role.kubernetes.io/control-plane": "", corev1.LabelHostname: "host1"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node2",
						Labels: map[string]string{"node-role.kubernetes.io/control-plane": "", corev1.LabelHostname: "host2"},
					},
				},
			} {
				Expect(fakeClient.Create(ctx, node.DeepCopy())).To(Succeed())
			}

			hostNames, err := b.getControlPlaneHostNames(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(hostNames).To(ConsistOf("host1", "host2"))
		})

		It("should not return worker nodes", func() {
			Expect(fakeClient.Create(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "cp-node",
					Labels: map[string]string{"node-role.kubernetes.io/control-plane": "", corev1.LabelHostname: "cp-host"},
				},
			})).To(Succeed())

			Expect(fakeClient.Create(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "worker-node",
					Labels: map[string]string{corev1.LabelHostname: "worker-host"},
				},
			})).To(Succeed())

			hostNames, err := b.getControlPlaneHostNames(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(hostNames).To(ConsistOf("cp-host"))
		})

		It("should return an error if a control plane node has no hostname label", func() {
			Expect(fakeClient.Create(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "no-hostname-node",
					Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
				},
			})).To(Succeed())

			_, err := b.getControlPlaneHostNames(ctx)
			Expect(err).To(MatchError(ContainSubstring("does not have any")))
		})
	})

	Describe("#appendEtcdNodeSpecificAssetsToFiles", func() {
		It("should remove existing etcd files and re-add them with hostname", func() {
			// Create secrets and configmaps that FetchSecrets/FetchConfigMaps will find.
			for _, role := range []string{"main", "events"} {
				for _, secretType := range []string{"server", "peer"} {
					Expect(fakeClient.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      secretType + "-" + role + "-node1",
							Namespace: namespace,
							Labels: map[string]string{
								"managed-by":       "secrets-manager",
								"etcd-secret-type": secretType,
								"etcd-role":        role,
								"hostName":         "node1",
							},
						},
						Data: map[string][]byte{"tls.crt": []byte(secretType + "-cert-" + role)},
					})).To(Succeed())
				}

				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "etcd-" + role + "-config",
						Namespace: namespace,
					},
					Data: map[string]string{"etcd.conf.yaml": "config-" + role},
				})).To(Succeed())
			}

			// Existing files: some etcd files that should be removed, and one non-etcd file that should be kept.
			existingFiles := []extensionsv1alpha1.File{
				{Path: "/some/other/file"},
				{Path: filepath.Join(staticpod.HostPath(etcd.Name("main"), "etcd-server-tls"), "tls.crt")},
				{Path: filepath.Join(staticpod.HostPath(etcd.Name("main"), "etcd-peer-server-tls"), "tls.crt")},
				{Path: filepath.Join(staticpod.HostPath(etcd.Name("main"), "etcd-config-file"), "etcd.conf.yaml")},
				{Path: filepath.Join(staticpod.HostPath(etcd.Name("events"), "etcd-server-tls"), "tls.crt")},
			}

			result, err := b.appendEtcdNodeSpecificAssetsToFiles(ctx, existingFiles, []string{"node1"})
			Expect(err).NotTo(HaveOccurred())

			// The non-etcd file must still be present.
			Expect(result).To(ContainElement(HaveField("Path", "/some/other/file")))

			// The old etcd files (without hostname) must be gone.
			for _, file := range result {
				if file.HostName == nil {
					Expect(file.Path).NotTo(HavePrefix(staticpod.HostPath(etcd.Name("main"), "etcd-server-tls")))
					Expect(file.Path).NotTo(HavePrefix(staticpod.HostPath(etcd.Name("main"), "etcd-peer-server-tls")))
					Expect(file.Path).NotTo(HavePrefix(staticpod.HostPath(etcd.Name("main"), "etcd-config-file")))
					Expect(file.Path).NotTo(HavePrefix(staticpod.HostPath(etcd.Name("events"), "etcd-server-tls")))
				}
			}

			// New etcd files must have the hostname set.
			var filesWithHostName int
			for _, file := range result {
				if file.HostName != nil && *file.HostName == "node1" {
					filesWithHostName++
				}
			}
			Expect(filesWithHostName).To(BeNumerically(">", 0))
		})

		It("should not modify the original files slice", func() {
			for _, role := range []string{"main", "events"} {
				for _, secretType := range []string{"server", "peer"} {
					Expect(fakeClient.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      secretType + "-" + role + "-node1",
							Namespace: namespace,
							Labels: map[string]string{
								"managed-by":       "secrets-manager",
								"etcd-secret-type": secretType,
								"etcd-role":        role,
								"hostName":         "node1",
							},
						},
						Data: map[string][]byte{"tls.crt": []byte("cert")},
					})).To(Succeed())
				}

				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "etcd-" + role + "-config",
						Namespace: namespace,
					},
					Data: map[string]string{"etcd.conf.yaml": "cfg"},
				})).To(Succeed())
			}

			original := []extensionsv1alpha1.File{{Path: "/keep-me"}}
			result, err := b.appendEtcdNodeSpecificAssetsToFiles(ctx, original, []string{"node1"})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(result)).To(BeNumerically(">", len(original)))
			Expect(original).To(HaveLen(1))
		})
	})
})
