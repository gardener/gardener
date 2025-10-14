// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/shoot/mutator"
)

var _ = Describe("mutator", func() {
	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("ShootMutator"))
		})
	})

	Describe("#New", func() {
		It("should handle CREATE and UPDATE operations", func() {
			admissionHandler, err := New()
			Expect(err).NotTo(HaveOccurred())
			Expect(admissionHandler.Handles(admission.Create)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Update)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Connect)).To(BeFalse())
			Expect(admissionHandler.Handles(admission.Delete)).To(BeFalse())
		})
	})

	Describe("#Admit", func() {
		var (
			ctx context.Context

			userInfo = &user.DefaultInfo{Name: "foo"}

			shoot core.Shoot

			admissionHandler *MutateShoot
		)

		BeforeEach(func() {
			ctx = context.Background()

			shoot = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "garden-my-project",
				},
			}

			var err error
			admissionHandler, err = New()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should ignore a kind other than shoot", func() {
			project := core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
			}
			attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

			Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())
		})

		It("should fail when object is not shoot", func() {
			project := core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
			}
			attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

			err := admissionHandler.Admit(ctx, attrs, nil)
			Expect(err).To(BeBadRequestError())
			Expect(err).To(MatchError("could not convert object to Shoot"))
		})

		Context("created-by annotation", func() {
			It("should add the created-by annotation on shoot creation", func() {
				Expect(shoot.Annotations).NotTo(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, userInfo.Name))

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())
				Expect(shoot.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, userInfo.Name))
			})

			It("should not add the created-by annotation on shoot update", func() {
				oldShoot := shoot.DeepCopy()
				Expect(shoot.Annotations).NotTo(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, userInfo.Name))

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())
				Expect(shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenCreatedBy))
			})
		})
	})
})
