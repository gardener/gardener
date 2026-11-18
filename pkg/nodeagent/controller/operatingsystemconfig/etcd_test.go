// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"fmt"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	"github.com/gardener/gardener/pkg/nodeagent"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("etcd", func() {
	var (
		ctx        context.Context
		reconciler *Reconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &Reconciler{
			APIReader: fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build(),
		}
	})

	Describe("#fileContentForPath", func() {
		var osc *extensionsv1alpha1.OperatingSystemConfig

		BeforeEach(func() {
			osc = &extensionsv1alpha1.OperatingSystemConfig{}
		})

		It("should return not-found when path is absent", func() {
			data, found, err := reconciler.fileContentForPath(ctx, osc, "/some/path/ca.crt")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
			Expect(data).To(BeNil())
		})

		It("should return data for an inline plain-encoded file", func() {
			osc.Spec.Files = []extensionsv1alpha1.File{
				{
					Path: "/etc/ca.crt",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: string(extensionsv1alpha1.PlainFileCodecID),
							Data:     "cert-data",
						},
					},
				},
			}

			data, found, err := reconciler.fileContentForPath(ctx, osc, "/etc/ca.crt")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(data).To(Equal([]byte("cert-data")))
		})

		It("should return an error for a file without inline or secretRef content", func() {
			osc.Spec.Files = []extensionsv1alpha1.File{
				{
					Path:    "/etc/ca.crt",
					Content: extensionsv1alpha1.FileContent{},
				},
			}

			data, found, err := reconciler.fileContentForPath(ctx, osc, "/etc/ca.crt")
			Expect(err).To(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(data).To(BeNil())
		})
	})

	Describe("#mutateETCDStaticPodManifest", func() {
		var (
			originalLookupIP func(string) ([]net.IP, error)
			machineIPv4      = net.ParseIP("1.2.3.4")
		)

		BeforeEach(func() {
			originalLookupIP = utils.LookupIP
			DeferCleanup(func() { utils.LookupIP = originalLookupIP })
			utils.LookupIP = func(string) ([]net.IP, error) {
				return []net.IP{machineIPv4}, nil
			}
			reconciler.HostName = "node1"
		})

		podManifest := func(containers ...corev1.Container) []byte {
			pod := &corev1.Pod{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
				ObjectMeta: metav1.ObjectMeta{Name: "etcd-main"},
				Spec:       corev1.PodSpec{Containers: containers},
			}
			data, err := runtime.Encode(nodeagent.Codec, pod)
			Expect(err).NotTo(HaveOccurred())
			return data
		}

		It("should set POD_IP on the backup-restore container only", func() {
			data := podManifest(
				corev1.Container{Name: etcdconstants.ContainerNameEtcd},
				corev1.Container{Name: etcdconstants.ContainerNameBackupRestore},
			)

			result, err := reconciler.mutateETCDStaticPodManifest(data)
			Expect(err).NotTo(HaveOccurred())

			pod := &corev1.Pod{}
			Expect(runtime.DecodeInto(nodeagent.Codec, result, pod)).To(Succeed())

			var etcdEnvVars, backupRestoreEnvVars []corev1.EnvVar
			for _, c := range pod.Spec.Containers {
				switch c.Name {
				case etcdconstants.ContainerNameEtcd:
					etcdEnvVars = c.Env
				case etcdconstants.ContainerNameBackupRestore:
					backupRestoreEnvVars = c.Env
				}
			}

			Expect(etcdEnvVars).To(BeEmpty())
			Expect(backupRestoreEnvVars).To(ContainElement(corev1.EnvVar{Name: "POD_IP", Value: machineIPv4.String()}))
		})

		It("should overwrite an existing POD_IP on the backup-restore container", func() {
			data := podManifest(
				corev1.Container{
					Name: etcdconstants.ContainerNameBackupRestore,
					Env:  []corev1.EnvVar{{Name: "POD_IP", Value: "0.0.0.0"}},
				},
			)

			result, err := reconciler.mutateETCDStaticPodManifest(data)
			Expect(err).NotTo(HaveOccurred())

			pod := &corev1.Pod{}
			Expect(runtime.DecodeInto(nodeagent.Codec, result, pod)).To(Succeed())

			Expect(pod.Spec.Containers).To(HaveLen(1))
			Expect(pod.Spec.Containers[0].Env).To(ConsistOf(corev1.EnvVar{Name: "POD_IP", Value: machineIPv4.String()}))
		})

		It("should return an error when the machine IP cannot be determined", func() {
			utils.LookupIP = func(string) ([]net.IP, error) {
				return nil, fmt.Errorf("lookup failed")
			}

			_, err := reconciler.mutateETCDStaticPodManifest(podManifest(
				corev1.Container{Name: etcdconstants.ContainerNameBackupRestore},
			))
			Expect(err).To(MatchError(ContainSubstring("failed determining the machine IP")))
		})

		It("should return an error for invalid manifest data", func() {
			_, err := reconciler.mutateETCDStaticPodManifest([]byte("not-valid-yaml"))
			Expect(err).To(MatchError(ContainSubstring("failed decoding the data into a corev1.Pod object")))
		})
	})

	Describe("#rawCADataFromOperatingSystemConfig", func() {
		inlineFile := func(path, data string) extensionsv1alpha1.File {
			return extensionsv1alpha1.File{
				Path: path,
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: string(extensionsv1alpha1.PlainFileCodecID),
						Data:     data,
					},
				},
			}
		}

		currentCertPath := func() string {
			return "/var/lib/etcd/ca/current/ca.crt"
		}
		currentKeyPath := func() string {
			return "/var/lib/etcd/ca/current/ca.key"
		}
		oldCertPath := func() string {
			return "/var/lib/etcd/ca/old/ca.crt"
		}
		oldKeyPath := func() string {
			return "/var/lib/etcd/ca/old/ca.key"
		}

		It("should return an error when the current cert is missing", func() {
			osc := &extensionsv1alpha1.OperatingSystemConfig{}

			_, _, _, _, err := reconciler.rawCADataFromOperatingSystemConfig(ctx, osc, "/var/lib/etcd/ca")
			Expect(err).To(MatchError(ContainSubstring("current ETCD-related CA not found")))
		})

		It("should return an error when the current key is missing", func() {
			osc := &extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					Files: []extensionsv1alpha1.File{
						inlineFile(currentCertPath(), "current-cert"),
					},
				},
			}

			_, _, _, _, err := reconciler.rawCADataFromOperatingSystemConfig(ctx, osc, "/var/lib/etcd/ca")
			Expect(err).To(MatchError(ContainSubstring("current ETCD-related CA not found")))
		})

		It("should return current cert and key when old CA is absent", func() {
			osc := &extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					Files: []extensionsv1alpha1.File{
						inlineFile(currentCertPath(), "current-cert"),
						inlineFile(currentKeyPath(), "current-key"),
					},
				},
			}

			currentCert, currentKey, oldCert, oldKey, err := reconciler.rawCADataFromOperatingSystemConfig(ctx, osc, "/var/lib/etcd/ca")
			Expect(err).NotTo(HaveOccurred())
			Expect(currentCert).To(Equal([]byte("current-cert")))
			Expect(currentKey).To(Equal([]byte("current-key")))
			Expect(oldCert).To(BeNil())
			Expect(oldKey).To(BeNil())
		})

		It("should return current and old cert+key when old CA is present", func() {
			osc := &extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					Files: []extensionsv1alpha1.File{
						inlineFile(currentCertPath(), "current-cert"),
						inlineFile(currentKeyPath(), "current-key"),
						inlineFile(oldCertPath(), "old-cert"),
						inlineFile(oldKeyPath(), "old-key"),
					},
				},
			}

			currentCert, currentKey, oldCert, oldKey, err := reconciler.rawCADataFromOperatingSystemConfig(ctx, osc, "/var/lib/etcd/ca")
			Expect(err).NotTo(HaveOccurred())
			Expect(currentCert).To(Equal([]byte("current-cert")))
			Expect(currentKey).To(Equal([]byte("current-key")))
			Expect(oldCert).To(Equal([]byte("old-cert")))
			Expect(oldKey).To(Equal([]byte("old-key")))
		})
	})
})
