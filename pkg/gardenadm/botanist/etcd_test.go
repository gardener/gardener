// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
)

var _ = Describe("ETCD Assets", func() {
	var (
		ctx        = context.Background()
		namespace  = "shoot--foo--bar"
		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	Describe("ETCDRoleToAssets", func() {
		var assets ETCDRoleToAssets

		BeforeEach(func() {
			assets = ETCDRoleToAssets{"main": {}, "events": {}}
		})

		Describe("#FetchSecrets", func() {
			It("should fetch server and peer secrets for each role", func() {
				for _, role := range []string{"main", "events"} {
					Expect(fakeClient.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "server-" + role,
							Namespace: namespace,
							Labels: map[string]string{
								"managed-by":       "secrets-manager",
								"etcd-secret-type": "server",
								"etcd-role":        role,
								"hostName":         "node1",
							},
						},
						Data: map[string][]byte{"tls.crt": []byte("server-cert-" + role)},
					})).To(Succeed())

					Expect(fakeClient.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "peer-" + role,
							Namespace: namespace,
							Labels: map[string]string{
								"managed-by":       "secrets-manager",
								"etcd-secret-type": "peer",
								"etcd-role":        role,
								"hostName":         "node1",
							},
						},
						Data: map[string][]byte{"tls.crt": []byte("peer-cert-" + role)},
					})).To(Succeed())
				}

				Expect(assets.FetchSecrets(ctx, fakeClient, namespace, "node1")).To(Succeed())

				Expect(assets["main"].ServerSecret.Data).To(HaveKeyWithValue("tls.crt", []byte("server-cert-main")))
				Expect(assets["main"].PeerSecret.Data).To(HaveKeyWithValue("tls.crt", []byte("peer-cert-main")))
				Expect(assets["events"].ServerSecret.Data).To(HaveKeyWithValue("tls.crt", []byte("server-cert-events")))
				Expect(assets["events"].PeerSecret.Data).To(HaveKeyWithValue("tls.crt", []byte("peer-cert-events")))
			})

			It("should only fetch secrets matching the given hostname", func() {
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "server-main-node1",
						Namespace: namespace,
						Labels: map[string]string{
							"managed-by":       "secrets-manager",
							"etcd-secret-type": "server",
							"etcd-role":        "main",
							"hostName":         "node1",
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "peer-main-node1",
						Namespace: namespace,
						Labels: map[string]string{
							"managed-by":       "secrets-manager",
							"etcd-secret-type": "peer",
							"etcd-role":        "main",
							"hostName":         "node1",
						},
					},
				})).To(Succeed())

				// Secret for a different hostname should not be fetched.
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "server-main-node2",
						Namespace: namespace,
						Labels: map[string]string{
							"managed-by":       "secrets-manager",
							"etcd-secret-type": "server",
							"etcd-role":        "main",
							"hostName":         "node2",
						},
					},
					Data: map[string][]byte{"tls.crt": []byte("wrong-node")},
				})).To(Succeed())

				singleRoleAssets := ETCDRoleToAssets{"main": {}}
				Expect(singleRoleAssets.FetchSecrets(ctx, fakeClient, namespace, "node1")).To(Succeed())
				Expect(singleRoleAssets["main"].ServerSecret.Name).To(Equal("server-main-node1"))
			})

			It("should panic if no server secret is found", func() {
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "peer-main",
						Namespace: namespace,
						Labels: map[string]string{
							"managed-by":       "secrets-manager",
							"etcd-secret-type": "peer",
							"etcd-role":        "main",
							"hostName":         "node1",
						},
					},
				})).To(Succeed())

				singleRoleAssets := ETCDRoleToAssets{"main": {}}
				Expect(func() {
					_ = singleRoleAssets.FetchSecrets(ctx, fakeClient, namespace, "node1")
				}).To(Panic())
			})
		})

		Describe("#FetchConfigMaps", func() {
			It("should fetch ConfigMaps for each role", func() {
				for _, role := range []string{"main", "events"} {
					Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "etcd-" + role + "-config",
							Namespace: namespace,
						},
						Data: map[string]string{"etcd.conf.yaml": "config-" + role},
					})).To(Succeed())
				}

				Expect(assets.FetchConfigMaps(ctx, fakeClient, namespace)).To(Succeed())

				Expect(assets["main"].Config.Data).To(HaveKeyWithValue("etcd.conf.yaml", "config-main"))
				Expect(assets["events"].Config.Data).To(HaveKeyWithValue("etcd.conf.yaml", "config-events"))
			})

			It("should return an error if a ConfigMap is missing", func() {
				Expect(assets.FetchConfigMaps(ctx, fakeClient, namespace)).To(MatchError(ContainSubstring("failed to fetch the configuration ConfigMap")))
			})
		})

		Describe("#WriteToDisk", func() {
			It("should write all asset files to disk", func() {
				assets = ETCDRoleToAssets{
					"main": {
						ServerSecret: &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("server-cert")}},
						PeerSecret:   &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("peer-cert")}},
						Config:       &corev1.ConfigMap{Data: map[string]string{"etcd.conf.yaml": "config-data"}},
					},
				}

				fs := afero.Afero{Fs: afero.NewMemMapFs()}
				Expect(assets.WriteToDisk(fs)).To(Succeed())

				content, err := fs.ReadFile(filepath.Join("/", "var", "lib", "static-pods", "etcd-main", "etcd-server-tls", "tls.crt"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("server-cert"))

				content, err = fs.ReadFile(filepath.Join("/", "var", "lib", "static-pods", "etcd-main", "etcd-peer-server-tls", "tls.crt"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("peer-cert"))

				content, err = fs.ReadFile(filepath.Join("/", "var", "lib", "static-pods", "etcd-main", "etcd-config-file", "etcd.conf.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("config-data"))
			})
		})
	})

	Describe("HostNameToETCDAssets", func() {
		Describe("#AppendToFiles", func() {
			It("should append node-specific files with the correct hostName", func() {
				hostNameToAssets := HostNameToETCDAssets{
					"node1": ETCDRoleToAssets{
						"main": {
							ServerSecret: &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("cert")}},
							PeerSecret:   &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("peer")}},
							Config:       &corev1.ConfigMap{Data: map[string]string{"etcd.conf.yaml": "cfg"}},
						},
					},
				}

				existingFiles := []extensionsv1alpha1.File{
					{Path: "/some/existing/file"},
				}

				result := hostNameToAssets.AppendToFiles(existingFiles)
				Expect(result).To(ContainElement(existingFiles[0]))
				Expect(len(result)).To(BeNumerically(">", len(existingFiles)))

				for _, file := range result[1:] {
					Expect(file.HostName).To(PointTo(Equal("node1")))
					Expect(file.Permissions).To(PointTo(BeEquivalentTo(0600)))
					Expect(file.Content.Inline).NotTo(BeNil())
					Expect(file.Content.Inline.Encoding).To(Equal("b64"))
				}
			})

			It("should not modify the original files slice", func() {
				hostNameToAssets := HostNameToETCDAssets{
					"node1": ETCDRoleToAssets{
						"main": {
							ServerSecret: &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("cert")}},
							PeerSecret:   &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("peer")}},
							Config:       &corev1.ConfigMap{Data: map[string]string{"etcd.conf.yaml": "cfg"}},
						},
					},
				}

				original := []extensionsv1alpha1.File{{Path: "/existing"}}
				result := hostNameToAssets.AppendToFiles(original)
				Expect(len(result)).To(BeNumerically(">", len(original)))
				Expect(original).To(HaveLen(1))
			})

			It("should handle multiple hostnames", func() {
				hostNameToAssets := HostNameToETCDAssets{
					"node1": ETCDRoleToAssets{
						"main": {
							ServerSecret: &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("cert1")}},
							PeerSecret:   &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("peer1")}},
							Config:       &corev1.ConfigMap{Data: map[string]string{"etcd.conf.yaml": "cfg1"}},
						},
					},
					"node2": ETCDRoleToAssets{
						"main": {
							ServerSecret: &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("cert2")}},
							PeerSecret:   &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("peer2")}},
							Config:       &corev1.ConfigMap{Data: map[string]string{"etcd.conf.yaml": "cfg2"}},
						},
					},
				}

				result := hostNameToAssets.AppendToFiles(nil)

				var hostNames []string
				for _, file := range result {
					if file.HostName != nil {
						hostNames = append(hostNames, *file.HostName)
					}
				}
				Expect(hostNames).To(ContainElements("node1", "node2"))
			})
		})
	})
})
