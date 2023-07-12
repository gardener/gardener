// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deletionconfirmation_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	internalClientSet "github.com/gardener/gardener/pkg/client/core/clientset/internalversion/fake"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/global/deletionconfirmation"
)

var _ = Describe("deleteconfirmation", func() {
	Describe("#Admit", func() {
		var (
			shoot      core.Shoot
			project    core.Project
			shootState core.ShootState

			shootStore      cache.Store
			projectStore    cache.Store
			shootStateStore cache.Store

			attrs            admission.Attributes
			admissionHandler *DeletionConfirmation

			intCoreInformerFactory gardencoreinformers.SharedInformerFactory
			intGardenClient        *internalClientSet.Clientset
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			intCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(intCoreInformerFactory)

			intGardenClient = &internalClientSet.Clientset{}
			admissionHandler.SetInternalCoreClientset(intGardenClient)

			shootStore = intCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore()
			projectStore = intCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore()
			shootStateStore = intCoreInformerFactory.Core().InternalVersion().ShootStates().Informer().GetStore()

			shoot = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy",
					Namespace: "dummy",
				},
			}
			project = core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dummy",
				},
			}
			shootState = core.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummyName",
					Namespace: "dummyNs",
				},
			}
		})

		It("should do nothing because the resource is not Shoot, Project or ShootState", func() {
			attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("foos").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		Context("Shoot resources", func() {
			It("should do nothing because the resource is already removed", func() {
				attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
				msg := `shoot.core.gardener.cloud "dummy" not found`

				intGardenClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf(msg)
				})

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`shoot.core.gardener.cloud "dummy" not found`))
			})

			Context("no annotation", func() {
				It("should reject for nil annotation field", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject for false annotation value", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					shoot.Annotations = map[string]string{
						gardenerutils.ConfirmationDeletion: "false",
					}
					Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should succeed for true annotation value (cache lookup)", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					shoot.Annotations = map[string]string{
						gardenerutils.ConfirmationDeletion: "true",
					}
					Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should succeed for true annotation value (live lookup)", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())
					intGardenClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
						return true, &core.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Name:      shoot.Name,
								Namespace: shoot.Namespace,
								Annotations: map[string]string{
									gardenerutils.ConfirmationDeletion: "true",
								},
							},
						}, nil
					})

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("no ignore annotation", func() {
				It("should reject if the ignore-shoot annotation is set", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					shoot.Annotations = map[string]string{
						gardenerutils.ConfirmationDeletion: "true",
						v1beta1constants.ShootIgnore:       "true",
					}
					Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})
			})

			Context("delete collection", func() {
				It("should allow because all shoots have the deletion confirmation annotation", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, "", core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					shoot.Annotations = map[string]string{gardenerutils.ConfirmationDeletion: "true"}
					shoot2 := shoot.DeepCopy()
					shoot2.Name = "dummy2"

					Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())
					Expect(shootStore.Add(shoot2)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should deny because at least one shoot does not have the deletion confirmation annotation", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, "", core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					shoot2 := shoot.DeepCopy()
					shoot2.Name = "dummy2"
					shoot.Annotations = map[string]string{gardenerutils.ConfirmationDeletion: "true"}

					Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())
					Expect(shootStore.Add(shoot2)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("Project resources", func() {
			It("should do nothing because the resource is already removed", func() {
				attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
				msg := `project.core.gardenerutils.cloud "dummy" not found`

				intGardenClient.AddReactor("get", "projects", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf(msg)
				})

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(msg))
			})

			Context("no annotation", func() {
				It("should reject for nil annotation field", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					Expect(projectStore.Add(&project)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject for false annotation value", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					project.Annotations = map[string]string{
						gardenerutils.ConfirmationDeletion: "false",
					}
					Expect(projectStore.Add(&project)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should succeed for true annotation value (cache lookup)", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					project.Annotations = map[string]string{
						gardenerutils.ConfirmationDeletion: "true",
					}
					Expect(projectStore.Add(&project)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should succeed for true annotation value (live lookup)", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					intGardenClient.AddReactor("get", "projects", func(action testing.Action) (bool, runtime.Object, error) {
						return true, &core.Project{
							ObjectMeta: metav1.ObjectMeta{
								Name: project.Name,
								Annotations: map[string]string{
									gardenerutils.ConfirmationDeletion: "true",
								},
							},
						}, nil
					})

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("delete collection", func() {
				It("should allow because all projects have the deletion confirmation annotation", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", "", core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					project.Annotations = map[string]string{gardenerutils.ConfirmationDeletion: "true"}
					project2 := project.DeepCopy()
					project2.Name = "dummy2"

					Expect(projectStore.Add(&project)).NotTo(HaveOccurred())
					Expect(projectStore.Add(project2)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should deny because at least one project does not have the deletion confirmation annotation", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", "", core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					project2 := project.DeepCopy()
					project2.Name = "dummy2"
					project.Annotations = map[string]string{gardenerutils.ConfirmationDeletion: "true"}

					Expect(projectStore.Add(&project)).NotTo(HaveOccurred())
					Expect(projectStore.Add(project2)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("ShootState resources", func() {
			It("should do nothing because the resource is already removed", func() {
				attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, shootState.Name, core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
				msg := `shoot.core.gardener.cloud "dummyName" not found`

				intGardenClient.AddReactor("get", "shootstates", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf(msg)
				})

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`shoot.core.gardener.cloud "dummyName" not found`))
			})

			Context("no annotation", func() {
				It("should reject for nil annotation field", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, shootState.Name, core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject for false annotation value", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, shootState.Name, core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					shootState.Annotations = map[string]string{
						gardenerutils.ConfirmationDeletion: "false",
					}
					Expect(shootStateStore.Add(&shoot)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should succeed for true annotation value (cache lookup)", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, shootState.Name, core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					shootState.Annotations = map[string]string{
						gardenerutils.ConfirmationDeletion: "true",
					}
					Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should succeed for true annotation value (live lookup)", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, shootState.Name, core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())
					intGardenClient.AddReactor("get", "shootstates", func(action testing.Action) (bool, runtime.Object, error) {
						return true, &core.ShootState{
							ObjectMeta: metav1.ObjectMeta{
								Name:      shootState.Name,
								Namespace: shootState.Namespace,
								Annotations: map[string]string{
									gardenerutils.ConfirmationDeletion: "true",
								},
							},
						}, nil
					})

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("delete collection", func() {
				It("should allow because all shootStates have the deletion confirmation annotation", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, "", core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					shootState.Annotations = map[string]string{gardenerutils.ConfirmationDeletion: "true"}
					shootState2 := shootState.DeepCopy()
					shootState2.Name = "dummyName2"

					Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())
					Expect(shootStateStore.Add(shootState2)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should deny because at least one shoot does not have the deletion confirmation annotation", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, "", core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					shootState2 := shootState.DeepCopy()
					shootState2.Name = "dummyName2"
					shootState.Annotations = map[string]string{gardenerutils.ConfirmationDeletion: "true"}

					Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())
					Expect(shootStateStore.Add(shootState2)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
				})

				It("should not deny deletecollection in this namespace because there is a shootstate in another namespace which does not have the deletion confirmation annotation", func() {
					shootState2 := shootState.DeepCopy()
					shootState2.Name = "dummyName2"
					shootState2.Namespace = "dummyNs2"
					shootState2.Annotations = map[string]string{gardenerutils.ConfirmationDeletion: "true"}

					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState2.Namespace, "", core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())
					Expect(shootStateStore.Add(shootState2)).NotTo(HaveOccurred())

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
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
			Expect(registered).To(ContainElement("DeletionConfirmation"))
		})
	})

	Describe("#NewFactory", func() {
		It("should create a new PluginFactory", func() {
			f, err := NewFactory(nil)

			Expect(f).NotTo(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#New", func() {
		It("should only handle DELETE operations", func() {
			dr, err := New()

			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Update)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Delete)).To(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if no ShootLister or ProjectLister is set", func() {
			dr, _ := New()

			err := dr.ValidateInitialization()

			Expect(err).To(HaveOccurred())
		})

		It("should not return error if lister and core clients are set", func() {
			dr, _ := New()
			intGardenClient := &internalClientSet.Clientset{}
			dr.SetInternalCoreClientset(intGardenClient)
			dr.SetInternalCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))

			err := dr.ValidateInitialization()

			Expect(err).ToNot(HaveOccurred())
		})
	})
})
