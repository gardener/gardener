// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	"github.com/coreos/go-systemd/v22/dbus"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
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
		b   *AutonomousBotanist

		fs         afero.Afero
		fakeDBus   *fakedbus.DBus
		fakeClient client.Client
		clientSet  kubernetes.Interface
	)

	BeforeEach(func() {
		fs = afero.Afero{Fs: afero.NewMemMapFs()}
		fakeDBus = fakedbus.New()
		fakeClient = fakeclient.NewClientBuilder().Build()
		clientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		b = &AutonomousBotanist{
			FS:   fs,
			DBus: fakeDBus,
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
				MatchFields(IgnoreExtras, Fields{"Path": Equal("/var/lib/gardener-node-agent/machine-name")}),
				MatchFields(IgnoreExtras, Fields{"Path": Equal("/var/lib/gardener-node-agent/config.yaml")}),
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
})
