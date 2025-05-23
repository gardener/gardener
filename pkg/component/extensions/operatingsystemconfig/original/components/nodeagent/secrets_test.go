// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Secrets", func() {
	Describe("#OperatingSystemConfigSecret", func() {
		var (
			ctx            = context.TODO()
			fakeClient     client.Client
			secretName     = "secret-name"
			workerPoolName = "worker-pool-name"

			namespace         = "namespace"
			fileSecret        *corev1.Secret
			fileSecretDataKey = "foo"
			fileSecretContent = []byte("bar")
			osc               *extensionsv1alpha1.OperatingSystemConfig
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			fileSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "file-secret",
					Namespace: namespace,
				},
				Data: map[string][]byte{fileSecretDataKey: fileSecretContent},
			}

			Expect(fakeClient.Create(ctx, fileSecret)).To(Succeed())
			DeferCleanup(func() {
				Expect(fakeClient.Delete(ctx, fileSecret)).To(Succeed())
			})

			osc = &extensionsv1alpha1.OperatingSystemConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "osc-name",
					Namespace:       namespace,
					ResourceVersion: "1",
					UID:             "foo",
					OwnerReferences: []metav1.OwnerReference{{}},
					Labels:          map[string]string{"foo": "bar"},
					Annotations:     map[string]string{"bar": "foo"},
				},
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					CRIConfig: &extensionsv1alpha1.CRIConfig{
						Name: "containerd",
						Containerd: &extensionsv1alpha1.ContainerdConfig{
							SandboxImage: "registry.k8s.io/pause:latest",
							Registries: []extensionsv1alpha1.RegistryConfig{
								{Upstream: "registry.k8s.io"},
							},
						},
					},
					Units: []extensionsv1alpha1.Unit{{
						Name: "some-unit.service",
					}},
					Files: []extensionsv1alpha1.File{{
						Path: "/some/path",
						Content: extensionsv1alpha1.FileContent{
							SecretRef: &extensionsv1alpha1.FileContentSecretRef{
								Name:    fileSecret.Name,
								DataKey: fileSecretDataKey,
							},
						},
					}},
				},
				Status: extensionsv1alpha1.OperatingSystemConfigStatus{
					ExtensionUnits: []extensionsv1alpha1.Unit{{
						Name: "some-other-unit.service",
					}},
					ExtensionFiles: []extensionsv1alpha1.File{{
						Path: "/some/other/path",
					}},
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							LastUpdateTime: metav1.Now(),
						},
					},
				},
			}
		})

		It("should generate the expected secret", func() {
			secret, err := OperatingSystemConfigSecret(ctx, fakeClient, osc, secretName, workerPoolName)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: "kube-system",
					Annotations: map[string]string{
						"checksum/data-script": "a3efca387130b4a0291595c1608d80cad758ca95f71901f1fc91b0d74847c0ff",
					},
					Labels: map[string]string{
						"gardener.cloud/role":        "operating-system-config",
						"worker.gardener.cloud/pool": workerPoolName,
					},
				},
				Data: map[string][]byte{"osc.yaml": []byte(`apiVersion: extensions.gardener.cloud/v1alpha1
kind: OperatingSystemConfig
metadata:
  creationTimestamp: null
spec:
  criConfig:
    containerd:
      registries:
      - upstream: registry.k8s.io
      sandboxImage: registry.k8s.io/pause:latest
    name: containerd
  files:
  - content:
      inline:
        data: ` + utils.EncodeBase64(fileSecretContent) + `
        encoding: b64
    path: /some/path
  purpose: ""
  type: ""
  units:
  - name: some-unit.service
status:
  extensionFiles:
  - content: {}
    path: /some/other/path
  extensionUnits:
  - name: some-other-unit.service
`)},
			}))
		})

		It("should return an error because a referenced secret cannot be found", func() {
			osc.Spec.Files = append(osc.Spec.Files, extensionsv1alpha1.File{
				Path: "/non/existing/path",
				Content: extensionsv1alpha1.FileContent{
					SecretRef: &extensionsv1alpha1.FileContentSecretRef{
						Name:    "non-existing",
						DataKey: "foo",
					},
				},
			})

			secret, err := OperatingSystemConfigSecret(ctx, fakeClient, osc, secretName, workerPoolName)
			Expect(err).To(MatchError(ContainSubstring(`cannot resolve secret ref from osc: secrets "non-existing" not found`)))
			Expect(secret).To(BeNil())
		})
	})
})
