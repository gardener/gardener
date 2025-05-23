// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusteropenidconnectpreset_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	settingsinformers "github.com/gardener/gardener/pkg/client/settings/informers/externalversions"
	. "github.com/gardener/gardener/plugin/pkg/shoot/oidc/clusteropenidconnectpreset"
)

var _ = Describe("Cluster OpenIDConfig Preset", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler        *ClusterOpenIDConnectPreset
			settingsInformerFactory settingsinformers.SharedInformerFactory
			shoot                   *core.Shoot
			project                 *gardencorev1beta1.Project
			preset                  *settingsv1alpha1.ClusterOpenIDConnectPreset
			coreInformerFactory     gardencoreinformers.SharedInformerFactory
		)

		BeforeEach(func() {
			namespace := "my-namespace"
			shootName := "shoot"
			presetName := "preset-1"
			projectName := "project-1"
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					Kubernetes: core.Kubernetes{
						Version: "1.31.0",
					},
				},
			}

			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: ptr.To(namespace),
				},
				Status: gardencorev1beta1.ProjectStatus{
					Phase: gardencorev1beta1.ProjectReady,
				},
			}

			preset = &settingsv1alpha1.ClusterOpenIDConnectPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      presetName,
					Namespace: namespace,
				},
				Spec: settingsv1alpha1.ClusterOpenIDConnectPresetSpec{
					ProjectSelector: &metav1.LabelSelector{},
					OpenIDConnectPresetSpec: settingsv1alpha1.OpenIDConnectPresetSpec{
						// select everything
						ShootSelector: &metav1.LabelSelector{},
						Weight:        0,
						Server: settingsv1alpha1.KubeAPIServerOpenIDConnect{
							CABundle:     ptr.To("cert"),
							ClientID:     "client-id",
							IssuerURL:    "https://foo.bar",
							GroupsClaim:  ptr.To("groupz"),
							GroupsPrefix: ptr.To("group-prefix"),
							RequiredClaims: map[string]string{
								"claim-1": "value-1",
								"claim-2": "value-2",
							},
							SigningAlgs:    []string{"alg-1", "alg-2"},
							UsernameClaim:  ptr.To("user"),
							UsernamePrefix: ptr.To("user-prefix"),
						},
						Client: &settingsv1alpha1.OpenIDConnectClientAuthentication{
							Secret:      ptr.To("secret"),
							ExtraConfig: map[string]string{"foo": "bar", "baz": "dap"},
						},
					},
				},
			}
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			settingsInformerFactory = settingsinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetSettingsInformerFactory(settingsInformerFactory)
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)

		})

		Context("should do nothing when", func() {

			var (
				expected *core.Shoot
				attrs    admission.Attributes
			)

			BeforeEach(func() {
				expected = shoot.DeepCopy()
				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("v1beta1"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			})

			AfterEach(func() {
				Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).NotTo(HaveOccurred())
				Expect(shoot).To(Equal(expected))
			})

			DescribeTable("disabled operations",
				func(op admission.Operation) {
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("v1beta1"), "", op, nil, false, nil)
				},
				Entry("update verb", admission.Update),
				Entry("delete verb", admission.Delete),
				Entry("connect verb", admission.Connect),
			)

			It("subresource is status", func() {
				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("v1beta1"), "status", admission.Create, &metav1.CreateOptions{}, false, nil)
			})

			It("preset shoot label selector doesn't match", func() {
				preset.Spec.ShootSelector.MatchLabels = map[string]string{"not": "existing"}
				Expect(settingsInformerFactory.Settings().V1alpha1().ClusterOpenIDConnectPresets().Informer().GetStore().Add(preset)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
			})

			It("preset preset label selector doesn't match", func() {
				preset.Spec.ProjectSelector.MatchLabels = map[string]string{"not": "existing"}
				Expect(settingsInformerFactory.Settings().V1alpha1().ClusterOpenIDConnectPresets().Informer().GetStore().Add(preset)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
			})

			It("oidc settings already exist", func() {
				shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{
					OIDCConfig: &core.OIDCConfig{},
				}
				expected = shoot.DeepCopy()
			})
		})

		Context("should return error", func() {
			var (
				expected     *core.Shoot
				attrs        admission.Attributes
				errorFunc    func(error) bool
				errorMessage string
			)

			BeforeEach(func() {
				expected = shoot.DeepCopy()
				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("v1beta1"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				errorFunc = nil
				errorMessage = ""
			})

			AfterEach(func() {
				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(errorFunc(err)).To(BeTrue(), "error type should be the same")
				Expect(shoot).To(Equal(expected))
				Expect(err.Error()).To(Equal(errorMessage))
			})

			It("when received not a Shoot object", func() {
				attrs = admission.NewAttributesRecord(&core.Seed{}, nil, core.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("v1beta1"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				errorFunc = apierrors.IsBadRequest
				errorMessage = "could not convert resource into Shoot object"
			})

			It("when it's not ready", func() {
				Skip("this takes 10seconds to pass")
				admissionHandler.AssignReadyFunc(func() bool { return false })
				errorFunc = apierrors.IsForbidden
				errorMessage = `presets.core.gardener.cloud "shoot" is forbidden: not yet ready to handle request`
			})

		})

		Context("should mutate the result", func() {
			var (
				expected *core.Shoot
			)

			BeforeEach(func() {
				expected = shoot.DeepCopy()
				expected.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{
					OIDCConfig: &core.OIDCConfig{
						CABundle:     ptr.To("cert"),
						ClientID:     ptr.To("client-id"),
						IssuerURL:    ptr.To("https://foo.bar"),
						GroupsClaim:  ptr.To("groupz"),
						GroupsPrefix: ptr.To("group-prefix"),
						RequiredClaims: map[string]string{
							"claim-1": "value-1",
							"claim-2": "value-2",
						},
						SigningAlgs:    []string{"alg-1", "alg-2"},
						UsernameClaim:  ptr.To("user"),
						UsernamePrefix: ptr.To("user-prefix"),

						ClientAuthentication: &core.OpenIDConnectClientAuthentication{
							Secret:      ptr.To("secret"),
							ExtraConfig: map[string]string{"foo": "bar", "baz": "dap"},
						},
					},
				}
			})

			AfterEach(func() {
				Expect(settingsInformerFactory.Settings().V1alpha1().ClusterOpenIDConnectPresets().Informer().GetStore().Add(preset)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("v1alpha1"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Kubernetes.KubeAPIServer).NotTo(BeNil())
				Expect(shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig).NotTo(BeNil())
				Expect(shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientAuthentication).NotTo(BeNil())
				Expect(shoot).To(Equal(expected))
			})

			It("shoot's kube-apiserver-oidc settings is not set", func() {
				shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{}
			})

			It("successfully", func() {
			})

			It("presets which match even with lower weight", func() {
				preset2 := preset.DeepCopy()

				preset.Spec.Weight = 100
				preset.Spec.ShootSelector.MatchLabels = map[string]string{"not": "existing"}

				preset2.Name = "preset-2"
				preset2.Spec.Server.ClientID = "client-id-2"

				expected.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = ptr.To("client-id-2")

				Expect(settingsInformerFactory.Settings().V1alpha1().ClusterOpenIDConnectPresets().Informer().GetStore().Add(preset2)).To(Succeed())
			})

			It("preset with higher weight", func() {
				preset2 := preset.DeepCopy()
				preset2.Name = "preset-2"
				preset2.Spec.Weight = 100
				preset2.Spec.Server.ClientID = "client-id-2"

				expected.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = ptr.To("client-id-2")

				Expect(settingsInformerFactory.Settings().V1alpha1().ClusterOpenIDConnectPresets().Informer().GetStore().Add(preset2)).To(Succeed())
			})

			It("presets ordered lexicographically", func() {
				preset.Name = "01preset"
				preset2 := preset.DeepCopy()
				preset2.Name = "02preset"
				preset2.Spec.Server.ClientID = "client-id-2"

				expected.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = ptr.To("client-id-2")

				Expect(settingsInformerFactory.Settings().V1alpha1().ClusterOpenIDConnectPresets().Informer().GetStore().Add(preset2)).To(Succeed())
			})

			It("presets which don't match the project selector", func() {
				preset2 := preset.DeepCopy()
				preset2.Spec.ProjectSelector.MatchLabels = map[string]string{"not": "existing"}
				preset2.Spec.Weight = 100
				preset2.Spec.Server.ClientID = "client-id-2"

				Expect(settingsInformerFactory.Settings().V1alpha1().ClusterOpenIDConnectPresets().Informer().GetStore().Add(preset2)).To(Succeed())
			})
		})
	})

	Describe("#ValidateInitialization", func() {

		var (
			plugin *ClusterOpenIDConnectPreset
		)

		BeforeEach(func() {
			plugin = &ClusterOpenIDConnectPreset{}
		})

		Context("error occurs", func() {
			It("when clusterOIDCLister is not set", func() {
				err := plugin.ValidateInitialization()
				Expect(err).To(HaveOccurred())
			})

			It("when projectLister is not set", func() {
				plugin.SetSettingsInformerFactory(settingsinformers.NewSharedInformerFactory(nil, 0))
				err := plugin.ValidateInitialization()
				Expect(err).To(HaveOccurred())
			})
		})

		It("should return nil error when everything is set", func() {
			plugin.SetSettingsInformerFactory(settingsinformers.NewSharedInformerFactory(nil, 0))
			plugin.SetCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))
			Expect(plugin.ValidateInitialization()).ToNot(HaveOccurred())
		})
	})
})
