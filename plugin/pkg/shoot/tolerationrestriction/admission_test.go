// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tolerationrestriction_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	. "github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction"
	"github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction"
)

var _ = Describe("toleration restriction", func() {
	Describe("#Admit", func() {
		var (
			namespace = "dummy"

			shoot   *core.Shoot
			project *gardencorev1beta1.Project

			attrs            admission.Attributes
			admissionHandler *TolerationRestriction

			gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory
		)

		BeforeEach(func() {
			gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)

			admissionHandler, _ = New(&shoottolerationrestriction.Configuration{})
			admissionHandler.AssignReadyFunc(func() bool { return true })
			admissionHandler.SetCoreInformerFactory(gardenCoreInformerFactory)

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy",
					Namespace: namespace,
				},
			}
			project = &gardencorev1beta1.Project{
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: &namespace,
				},
			}

		})

		It("should do nothing because the resource is not Shoot or Project", func() {
			attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		Context("#Admit", func() {
			Context("CREATE", func() {
				It("should do nothing because no defaults are defined", func() {
					shoot.Spec.Tolerations = []core.Toleration{{Key: "baz"}}

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.UpdateOptions{}, false, nil)
					Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())
					Expect(shoot.Spec.Tolerations).To(Equal([]core.Toleration{{Key: "baz"}}))
				})

				It("should merge the global and project-level default tolerations into the shoot tolerations", func() {
					config := &shoottolerationrestriction.Configuration{Defaults: []core.Toleration{{Key: "foo"}}}
					project.Spec.Tolerations = &gardencorev1beta1.ProjectTolerations{Defaults: []gardencorev1beta1.Toleration{{Key: "bar"}}}
					shoot.Spec.Tolerations = []core.Toleration{{Key: "baz"}}

					admissionHandler, _ = New(config)
					admissionHandler.AssignReadyFunc(func() bool { return true })
					admissionHandler.SetCoreInformerFactory(gardenCoreInformerFactory)

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.UpdateOptions{}, false, nil)
					Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())
					Expect(shoot.Spec.Tolerations).To(Equal([]core.Toleration{{Key: "baz"}, {Key: "foo"}, {Key: "bar"}}))
				})

				It("should not merge less-specific the global and project-level default tolerations into the shoot tolerations", func() {
					config := &shoottolerationrestriction.Configuration{Defaults: []core.Toleration{{Key: "foo"}}}
					project.Spec.Tolerations = &gardencorev1beta1.ProjectTolerations{Defaults: []gardencorev1beta1.Toleration{{Key: "bar"}, {Key: "baz", Value: ptr.To("foo")}}}
					shoot.Spec.Tolerations = []core.Toleration{{Key: "baz"}}

					admissionHandler, _ = New(config)
					admissionHandler.AssignReadyFunc(func() bool { return true })
					admissionHandler.SetCoreInformerFactory(gardenCoreInformerFactory)

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.UpdateOptions{}, false, nil)
					Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())
					Expect(shoot.Spec.Tolerations).To(Equal([]core.Toleration{{Key: "baz"}, {Key: "foo"}, {Key: "bar"}}))
				})
			})

			Context("UPDATE", func() {
				It("should not change the tolerations for already existing shoots", func() {
					config := &shoottolerationrestriction.Configuration{Whitelist: []core.Toleration{{Key: "foo"}}}
					project.Spec.Tolerations = &gardencorev1beta1.ProjectTolerations{Whitelist: []gardencorev1beta1.Toleration{{Key: "bar"}}}
					shoot.Spec.Tolerations = []core.Toleration{{Key: "baz"}}

					admissionHandler, _ = New(config)
					admissionHandler.AssignReadyFunc(func() bool { return true })
					admissionHandler.SetCoreInformerFactory(gardenCoreInformerFactory)

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)
					Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())
					Expect(shoot.Spec.Tolerations).To(Equal([]core.Toleration{{Key: "baz"}}))
				})
			})
		})

		Context("#Validate", func() {
			Context("CREATE", func() {
				It("should return an error because project for shoot was not found", func() {
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).NotTo(Succeed())
				})

				It("should allow creating the shoot because it doesn't have any tolerations", func() {
					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
				})

				It("should allow creating the shoot because its tolerations are in the project's whitelist", func() {
					project.Spec.Tolerations = &gardencorev1beta1.ProjectTolerations{Whitelist: []gardencorev1beta1.Toleration{
						{Key: "foo"},
						{Key: "bax"},
						{Key: "bar", Value: ptr.To("baz")},
					}}
					shoot.Spec.Tolerations = []core.Toleration{
						{Key: "foo"},
						{Key: "bax", Value: ptr.To("foo")},
						{Key: "bar", Value: ptr.To("baz")},
					}

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
				})

				It("should reject creating the shoot because its tolerations are not in the project's whitelist", func() {
					project.Spec.Tolerations = &gardencorev1beta1.ProjectTolerations{Whitelist: []gardencorev1beta1.Toleration{
						{Key: "foo"},
						{Key: "bax", Value: ptr.To("bar")},
						{Key: "bar", Value: ptr.To("baz")},
					}}
					shoot.Spec.Tolerations = []core.Toleration{
						{Key: "foo"},
						{Key: "bax", Value: ptr.To("foo")},
						{Key: "bar", Value: ptr.To("baz")},
					}

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).NotTo(Succeed())
				})

				It("should allow creating the shoot because its tolerations are in the global whitelist", func() {
					config := &shoottolerationrestriction.Configuration{Whitelist: []core.Toleration{
						{Key: "foo"},
						{Key: "bax"},
						{Key: "bar", Value: ptr.To("baz")},
					}}

					admissionHandler, _ = New(config)
					admissionHandler.AssignReadyFunc(func() bool { return true })
					admissionHandler.SetCoreInformerFactory(gardenCoreInformerFactory)

					shoot.Spec.Tolerations = []core.Toleration{
						{Key: "foo"},
						{Key: "bax", Value: ptr.To("foo")},
						{Key: "bar", Value: ptr.To("baz")},
					}

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
				})

				It("should reject creating the shoot because its tolerations are not in the global whitelist", func() {
					config := &shoottolerationrestriction.Configuration{Whitelist: []core.Toleration{
						{Key: "foo"},
						{Key: "bax", Value: ptr.To("bar")},
						{Key: "bar", Value: ptr.To("baz")},
					}}

					admissionHandler, _ = New(config)
					admissionHandler.AssignReadyFunc(func() bool { return true })
					admissionHandler.SetCoreInformerFactory(gardenCoreInformerFactory)

					shoot.Spec.Tolerations = []core.Toleration{
						{Key: "foo"},
						{Key: "bax", Value: ptr.To("foo")},
						{Key: "bar", Value: ptr.To("baz")},
					}

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).NotTo(Succeed())
				})
			})

			Context("UPDATE", func() {
				It("should return an error because project for shoot was not found", func() {
					attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).NotTo(Succeed())
				})

				It("should allow updating the shoot because no new (non-whitelisted) tolerations were added", func() {
					shoot.Spec.Tolerations = []core.Toleration{
						{Key: "foo"},
						{Key: "bax", Value: ptr.To("foo")},
						{Key: "bar", Value: ptr.To("baz")},
					}
					oldShoot := shoot.DeepCopy()

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
				})

				It("should allow updating the shoot because old (non-whitelisted) tolerations were removed", func() {
					shoot.Spec.Tolerations = []core.Toleration{
						{Key: "bar", Value: ptr.To("baz")},
					}
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.Tolerations = []core.Toleration{
						{Key: "foo"},
						{Key: "bax", Value: ptr.To("foo")},
						{Key: "bar", Value: ptr.To("baz")},
					}

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
				})

				It("should reject updating the shoot because old (non-whitelisted) tolerations were changed", func() {
					shoot.Spec.Tolerations = []core.Toleration{
						{Key: "foo"},
						{Key: "bax", Value: ptr.To("foo")},
						{Key: "bar", Value: ptr.To("baz")},
					}
					oldShoot := shoot.DeepCopy()
					shoot.Spec.Tolerations[0].Key = "new"

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).NotTo(Succeed())
				})

				It("should reject updating the shoot because new (non-whitelisted) tolerations were added", func() {
					shoot.Spec.Tolerations = []core.Toleration{
						{Key: "foo"},
						{Key: "bax", Value: ptr.To("foo")},
						{Key: "bar", Value: ptr.To("baz")},
					}
					oldShoot := shoot.DeepCopy()
					shoot.Spec.Tolerations = append(shoot.Spec.Tolerations, core.Toleration{Key: "new"})

					Expect(gardenCoreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					attrs = admission.NewAttributesRecord(shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).NotTo(Succeed())
				})
			})
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("ShootTolerationRestriction"))
		})
	})

	Describe("#New", func() {
		It("should only handle CREATE or UPDATE operations", func() {
			dr, err := New(&shoottolerationrestriction.Configuration{})

			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).To(BeTrue())
			Expect(dr.Handles(admission.Update)).To(BeTrue())
			Expect(dr.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if no ProjectLister is set", func() {
			dr, _ := New(&shoottolerationrestriction.Configuration{})

			err := dr.ValidateInitialization()
			Expect(err).To(HaveOccurred())
		})

		It("should not return error if ProjectLister is set", func() {
			dr, _ := New(&shoottolerationrestriction.Configuration{})
			dr.SetCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))

			err := dr.ValidateInitialization()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
