// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpa_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/plugin/pkg/shoot/vpa"
)

var _ = Describe("ShootVPAEnabledByDefault", func() {
	var (
		ctx      context.Context
		plugin   admission.MutationInterface
		attrs    admission.Attributes
		userInfo *user.DefaultInfo

		shoot, expectedShoot *core.Shoot
	)

	BeforeEach(func() {
		ctx = context.Background()
		plugin = vpa.New()

		userInfo = &user.DefaultInfo{Name: "foo"}

		shoot = &core.Shoot{}
		expectedShoot = shoot.DeepCopy()
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			vpa.Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("ShootVPAEnabledByDefault"))
		})
	})

	Describe("#Handles", func() {
		It("should only handle CREATE operation", func() {
			Expect(plugin.Handles(admission.Create)).To(BeTrue())
			Expect(plugin.Handles(admission.Update)).NotTo(BeTrue())
			Expect(plugin.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(plugin.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})

	Describe("#Admit", func() {
		Context("ignored requests", func() {
			It("should ignore resources other than Shoot", func() {
				project := &core.Project{}
				attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
			})
			It("should ignore operations other than Create", func() {
				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
				Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(shoot).To(Equal(expectedShoot))
			})
			It("should ignore subresources", func() {
				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "status", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(shoot).To(Equal(expectedShoot))
			})
		})

		It("should fail, if object is not a shoot", func() {
			attrs = admission.NewAttributesRecord(&core.Project{}, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
			err := plugin.Admit(ctx, attrs, nil)
			Expect(err).To(BeInternalServerError())
			Expect(err).To(MatchError(ContainSubstring("could not convert")))
		})

		It("should not enable VPA if explicitly disabled", func() {
			shoot.Spec.Kubernetes.VerticalPodAutoscaler = &core.VerticalPodAutoscaler{Enabled: false}
			expectedShoot = shoot.DeepCopy()

			attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
			Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
			Expect(shoot).To(Equal(expectedShoot))
		})

		It("should enable VPA if not explicitly disabled for a Shoot with workers", func() {
			shoot = &core.Shoot{
				Spec: core.ShootSpec{
					Provider: core.Provider{
						Workers: []core.Worker{
							{
								Name: "worker",
							},
						},
					},
				},
			}
			expectedShoot = shoot.DeepCopy()
			expectedShoot.Spec.Kubernetes.VerticalPodAutoscaler = &core.VerticalPodAutoscaler{Enabled: true}

			attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
			Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
			Expect(shoot).To(Equal(expectedShoot))
		})

		It("should not enable VPA for a workerless Shoot", func() {
			expectedShoot.Spec.Kubernetes.VerticalPodAutoscaler = nil

			attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
			Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
			Expect(shoot).To(Equal(expectedShoot))
		})
	})
})
