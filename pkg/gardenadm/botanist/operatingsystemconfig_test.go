// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/Masterminds/semver/v3"
	"github.com/coreos/go-systemd/v22/dbus"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/component-base/version"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
)

var _ = Describe("OperatingSystemConfig", func() {
	var (
		ctx = context.Background()
		b   *GardenadmBotanist

		fs         afero.Afero
		fakeDBus   *fakedbus.DBus
		fakeClient client.Client
		clientSet  kubernetes.Interface

		node *corev1.Node
		zone string
	)

	BeforeEach(func() {
		fs = afero.Afero{Fs: afero.NewMemMapFs()}
		fakeDBus = fakedbus.New()
		fakeClient = fakeclient.NewClientBuilder().Build()
		clientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		b = &GardenadmBotanist{
			FS:       fs,
			DBus:     fakeDBus,
			HostName: "foo-host",
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					SeedClientSet: clientSet,
					Shoot: &shootpkg.Shoot{
						KubernetesVersion: semver.MustParse("1.33.0"),
					},
				},
			},
		}
	})

	Describe("#PrepareGardenerNodeInitConfiguration", func() {
		var (
			secretName          = "secret-name"
			controlPlaneAddress = "control-plane-address"
			caBundle            = []byte("ca-bundle")
			bootstrapToken      = "bootstrap-token"
		)

		It("should create the secret containing and OperatingSystemConfig for gardener-node-init", func() {
			Expect(b.PrepareGardenerNodeInitConfiguration(ctx, secretName, controlPlaneAddress, caBundle, bootstrapToken)).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secretList, client.InNamespace("kube-system"))).To(Succeed())
			Expect(secretList.Items).To(HaveLen(1))

			Expect(secretList.Items[0].Data).To(HaveKey("osc.yaml"))
			osc := &extensionsv1alpha1.OperatingSystemConfig{}
			Expect(runtime.DecodeInto(serializer.NewCodecFactory(kubernetes.SeedScheme).UniversalDecoder(), secretList.Items[0].Data["osc.yaml"], osc)).To(Succeed())

			Expect(osc.Spec.Units).To(HaveExactElements(
				MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-node-init.service")}),
			))
			Expect(osc.Spec.Files).To(HaveExactElements(
				MatchFields(IgnoreExtras, Fields{
					"Path": Equal("/var/lib/gardener-node-agent/credentials/bootstrap-token"),
					"Content": MatchFields(IgnoreExtras, Fields{
						"Inline": PointTo(MatchFields(IgnoreExtras, Fields{
							"Data": Equal(bootstrapToken),
						})),
					}),
				}),
				MatchFields(IgnoreExtras, Fields{"Path": Equal("/var/lib/gardener-node-agent/init.sh")}),
				MatchFields(IgnoreExtras, Fields{
					"Path": Equal("/var/lib/gardener-node-agent/machine-name"),
					"Content": MatchFields(IgnoreExtras, Fields{
						"Inline": PointTo(MatchFields(IgnoreExtras, Fields{
							"Data": Equal("foo-host"),
						})),
					}),
				}),
				MatchFields(IgnoreExtras, Fields{"Path": Equal(fmt.Sprintf("/var/lib/gardener-node-agent/config-%s.yaml", version.Get().GitVersion))}),
			))
		})
	})

	Describe("#IsGardenerNodeAgentInitialized", func() {
		It("should return false because the gardener-node-agent unit was not found", func() {
			isInitialized, err := b.IsGardenerNodeAgentInitialized(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(isInitialized).To(BeFalse())
		})

		It("should return false because the bootstrap token file still exists", func() {
			fakeDBus.AddUnitsToList(dbus.UnitStatus{Name: "gardener-node-agent.service"})

			_, err := fs.Create("/var/lib/gardener-node-agent/credentials/bootstrap-token")
			Expect(err).NotTo(HaveOccurred())

			isInitialized, err := b.IsGardenerNodeAgentInitialized(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(isInitialized).To(BeFalse())
		})

		It("should return true because the bootstrap token file does no longer exist", func() {
			fakeDBus.AddUnitsToList(dbus.UnitStatus{Name: "gardener-node-agent.service"})

			isInitialized, err := b.IsGardenerNodeAgentInitialized(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(isInitialized).To(BeTrue())
		})
	})

	Describe("#ApplyOperatingSystemConfig - Zone file handling", func() {
		BeforeEach(func() {
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						corev1.LabelHostname: b.HostName,
					},
				},
			}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			oscSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "osc-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"osc.yaml": []byte("test-data"),
				},
			}

			rs := reflect.ValueOf(b).Elem()
			rf := rs.FieldByName("operatingSystemConfigSecret")
			reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem().Set(reflect.ValueOf(oscSecret))
		})

		Context("when Zone is nil", func() {
			BeforeEach(func() {
				b.Zone = nil
			})

			It("should not write zone file", func() {
				Expect(b.ApplyOperatingSystemConfig(ctx)).To(Succeed())

				_, statErr := fs.Stat("/var/lib/gardener-node-agent/zone")
				Expect(statErr).To(MatchError(ContainSubstring("file does not exist")))
			})
		})

		Context("when Zone is set", func() {
			BeforeEach(func() {
				zone = "zone-a"
				b.Zone = ptr.To(zone)
			})

			It("should write zone file", func() {
				Expect(b.ApplyOperatingSystemConfig(ctx)).To(Succeed())

				zoneContent, readErr := fs.ReadFile("/var/lib/gardener-node-agent/zone")
				Expect(readErr).NotTo(HaveOccurred())
				Expect(string(zoneContent)).To(Equal(zone))
			})

			It("should overwrite existing zone file", func() {
				Expect(fs.MkdirAll("/var/lib/gardener-node-agent", 0755)).To(Succeed())
				Expect(fs.WriteFile("/var/lib/gardener-node-agent/zone", []byte("existing-zone"), 0600)).To(Succeed())

				Expect(b.ApplyOperatingSystemConfig(ctx)).To(Succeed())

				zoneContent, readErr := fs.ReadFile("/var/lib/gardener-node-agent/zone")
				Expect(readErr).NotTo(HaveOccurred())
				Expect(string(zoneContent)).To(Equal(zone))
			})
		})
	})
})
