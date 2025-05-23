// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package deletionconfirmation_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	internalClientSet "github.com/gardener/gardener/pkg/client/core/clientset/versioned/fake"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/global/deletionconfirmation"
)

var _ = Describe("deleteconfirmation", func() {
	Describe("#Admit", func() {
		var (
			shoot      gardencorev1beta1.Shoot
			project    gardencorev1beta1.Project
			shootState gardencorev1beta1.ShootState

			shootStore      cache.Store
			projectStore    cache.Store
			shootStateStore cache.Store

			attrs            admission.Attributes
			admissionHandler *DeletionConfirmation

			coreInformerFactory gardencoreinformers.SharedInformerFactory
			gardenClient        *internalClientSet.Clientset

			userInfo *user.DefaultInfo
			userName = "some-user"
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)

			gardenClient = &internalClientSet.Clientset{}
			admissionHandler.SetCoreClientSet(gardenClient)

			shootStore = coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore()
			projectStore = coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore()
			shootStateStore = coreInformerFactory.Core().V1beta1().ShootStates().Informer().GetStore()

			shoot = gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy",
					Namespace: "dummy",
				},
			}
			project = gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dummy",
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: ptr.To("dummy"),
				},
			}
			shootState = gardencorev1beta1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy",
					Namespace: "dummy",
				},
			}

			userInfo = &user.DefaultInfo{Name: userName}
		})

		JustBeforeEach(func() {
			Expect(projectStore.Add(&project)).NotTo(HaveOccurred())
		})

		Describe("#Admit", func() {
			It("should do nothing because the resource is not Shoot", func() {
				attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())
			})

			It("should do nothing because the subresource is set", func() {
				attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "foo", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())
			})

			Context("Shoot resources", func() {
				Context("Create", func() {
					It("should set the 'confirmed-by' annotation if the deletion is confirmed", func() {
						attrs = admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

						shoot.Annotations = map[string]string{"confirmation.gardener.cloud/deletion": "true"}
						Expect(shootStore.Add(&shoot)).To(Succeed())

						Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

						Expect(shoot.Annotations).To(HaveKeyWithValue("deletion.gardener.cloud/confirmed-by", userName))
					})

					It("should override the 'confirmed-by' annotation if the deletion is confirmed", func() {
						attrs = admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

						shoot.Annotations = map[string]string{
							"confirmation.gardener.cloud/deletion": "true",
							"deletion.gardener.cloud/confirmed-by": "some-other-user",
						}
						Expect(shootStore.Add(&shoot)).To(Succeed())

						Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

						Expect(shoot.Annotations).To(HaveKeyWithValue("deletion.gardener.cloud/confirmed-by", userName))
					})

					It("should remove the 'confirmed-by' annotation if the deletion is not confirmed", func() {
						attrs = admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

						shoot.Annotations = map[string]string{"deletion.gardener.cloud/confirmed-by": userName}
						Expect(shootStore.Add(&shoot)).To(Succeed())

						Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

						Expect(shoot.Annotations).NotTo(HaveKey("deletion.gardener.cloud/confirmed-by"))
					})
				})

				Context("Update", func() {
					It("should set the 'confirmed-by' annotation if the deletion is confirmed", func() {
						oldShoot := shoot.DeepCopy()

						attrs = admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

						shoot.Annotations = map[string]string{"confirmation.gardener.cloud/deletion": "true"}
						Expect(shootStore.Add(&shoot)).To(Succeed())

						Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

						Expect(shoot.Annotations).To(HaveKeyWithValue("deletion.gardener.cloud/confirmed-by", userName))
					})

					It("should override the 'confirmed-by' annotation if the deletion is confirmed", func() {
						oldShoot := shoot.DeepCopy()
						oldShoot.Annotations = map[string]string{
							"confirmation.gardener.cloud/deletion": "true",
							"deletion.gardener.cloud/confirmed-by": "some-other-user",
						}

						attrs = admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

						shoot.Annotations = map[string]string{
							"confirmation.gardener.cloud/deletion": "true",
							"deletion.gardener.cloud/confirmed-by": "try-to-change-it",
						}
						Expect(shootStore.Add(&shoot)).To(Succeed())

						Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

						Expect(shoot.Annotations).To(HaveKeyWithValue("deletion.gardener.cloud/confirmed-by", "some-other-user"))
					})

					It("should remove the 'confirmed-by' annotation if the deletion is not confirmed", func() {
						oldShoot := shoot.DeepCopy()
						oldShoot.Annotations = map[string]string{
							"confirmation.gardener.cloud/deletion": "true",
							"deletion.gardener.cloud/confirmed-by": "some-other-user",
						}

						attrs = admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

						shoot.Annotations = map[string]string{
							"deletion.gardener.cloud/confirmed-by": userName,
						}
						Expect(shootStore.Add(&shoot)).To(Succeed())

						Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

						Expect(shoot.Annotations).NotTo(HaveKey("deletion.gardener.cloud/confirmed-by"))
					})
				})
			})
		})

		Describe("#Validate", func() {
			It("should do nothing because the resource is not Shoot, Project or ShootState", func() {
				attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("foos").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			Context("Shoot resources", func() {
				It("should do nothing because the resource is already removed", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
					gardenClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
						return true, nil, fmt.Errorf("shoot.core.gardener.cloud \"dummy\" not found")
					})

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(`shoot.core.gardener.cloud "dummy" not found`))
				})

				Context("delete", func() {
					It("should reject for nil annotation field", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should reject for false annotation value", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						shoot.Annotations = map[string]string{
							"confirmation.gardener.cloud/deletion": "false",
						}
						Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should succeed for true annotation value (cache lookup)", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						shoot.Annotations = map[string]string{
							"confirmation.gardener.cloud/deletion": "true",
						}
						Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should succeed for true annotation value (live lookup)", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())
						gardenClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
							return true, &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{
									Name:      shoot.Name,
									Namespace: shoot.Namespace,
									Annotations: map[string]string{
										"confirmation.gardener.cloud/deletion": "true",
									},
								},
							}, nil
						})

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					Context("dual approval", func() {
						labels := map[string]string{"foo": "bar"}

						BeforeEach(func() {
							project.Spec.DualApprovalForDeletion = append(project.Spec.DualApprovalForDeletion, gardencorev1beta1.DualApprovalForDeletion{
								Resource:               "shoots",
								Selector:               metav1.LabelSelector{MatchLabels: labels},
								IncludeServiceAccounts: ptr.To(false),
							})
						})

						When("label selector does not match", func() {
							It("should succeed for true annotation value", func() {
								attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)

								shoot.Annotations = map[string]string{"confirmation.gardener.cloud/deletion": "true"}

								Expect(shootStore.Add(&shoot)).To(Succeed())
								gardenClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
									return true, &shoot, nil
								})

								Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
							})
						})

						When("label selector matches", func() {
							BeforeEach(func() {
								shoot.Labels = labels
							})

							It("should fail if the same subject confirmed the deletion", func() {
								attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)

								shoot.Annotations = map[string]string{
									"confirmation.gardener.cloud/deletion": "true",
									"deletion.gardener.cloud/confirmed-by": userInfo.Name,
								}

								Expect(shootStore.Add(&shoot)).To(Succeed())
								gardenClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
									return true, &shoot, nil
								})

								Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(MatchError(ContainSubstring("you are not allowed to both confirm the deletion and send the actual DELETE request - another subject must perform the deletion")))
							})

							It("should succeed if the another subject confirmed the deletion", func() {
								attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)

								shoot.Annotations = map[string]string{
									"confirmation.gardener.cloud/deletion": "true",
									"deletion.gardener.cloud/confirmed-by": "other-user",
								}

								Expect(shootStore.Add(&shoot)).To(Succeed())
								gardenClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
									return true, &shoot, nil
								})

								Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
							})

							Context("for ServiceAccounts", func() {
								BeforeEach(func() {
									userInfo.Name = "system:serviceaccount:" + userName
								})

								When("ServiceAccounts are excluded", func() {
									BeforeEach(func() {
										project.Spec.DualApprovalForDeletion[0].IncludeServiceAccounts = ptr.To(false)
									})

									It("should succeed even if the same ServiceAccount confirmed the deletion", func() {
										attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)

										shoot.Annotations = map[string]string{
											"confirmation.gardener.cloud/deletion": "true",
											"deletion.gardener.cloud/confirmed-by": userInfo.Name,
										}

										Expect(shootStore.Add(&shoot)).To(Succeed())
										gardenClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
											return true, &shoot, nil
										})

										Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
									})
								})

								When("ServiceAccounts are included", func() {
									BeforeEach(func() {
										project.Spec.DualApprovalForDeletion[0].IncludeServiceAccounts = ptr.To(true)
									})

									It("should fail if the same ServiceAccount confirmed the deletion", func() {
										attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)

										shoot.Annotations = map[string]string{
											"confirmation.gardener.cloud/deletion": "true",
											"deletion.gardener.cloud/confirmed-by": userInfo.Name,
										}

										Expect(shootStore.Add(&shoot)).To(Succeed())
										gardenClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
											return true, &shoot, nil
										})

										Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(MatchError(ContainSubstring("you are not allowed to both confirm the deletion and send the actual DELETE request - another subject must perform the deletion")))
									})

									It("should succeed if the another subject confirmed the deletion", func() {
										attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)

										shoot.Annotations = map[string]string{
											"confirmation.gardener.cloud/deletion": "true",
											"deletion.gardener.cloud/confirmed-by": "other-user",
										}

										Expect(shootStore.Add(&shoot)).To(Succeed())
										gardenClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
											return true, &shoot, nil
										})

										Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
									})
								})
							})
						})
					})
				})

				Context("delete collection", func() {
					It("should allow because all shoots have the deletion confirmation annotation", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, "", core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						shoot.Annotations = map[string]string{"confirmation.gardener.cloud/deletion": "true"}
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
						shoot.Annotations = map[string]string{"confirmation.gardener.cloud/deletion": "true"}

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

					gardenClient.AddReactor("get", "projects", func(_ testing.Action) (bool, runtime.Object, error) {
						return true, nil, fmt.Errorf("project.core.gardenerutils.cloud \"dummy\" not found")
					})

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(msg))
				})

				Context("delete", func() {
					It("should reject for nil annotation field", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should reject for false annotation value", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						project.Annotations = map[string]string{
							"confirmation.gardener.cloud/deletion": "false",
						}

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should succeed for true annotation value (cache lookup)", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						project.Annotations = map[string]string{
							"confirmation.gardener.cloud/deletion": "true",
						}

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should succeed for true annotation value (live lookup)", func() {
						Expect(projectStore.Delete(&project)).To(Succeed())

						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						gardenClient.AddReactor("get", "projects", func(_ testing.Action) (bool, runtime.Object, error) {
							return true, &gardencorev1beta1.Project{
								ObjectMeta: metav1.ObjectMeta{
									Name: project.Name,
									Annotations: map[string]string{
										"confirmation.gardener.cloud/deletion": "true",
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

						project.Annotations = map[string]string{"confirmation.gardener.cloud/deletion": "true"}
						project2 := project.DeepCopy()
						project2.Name = "dummy2"

						Expect(projectStore.Add(project2)).NotTo(HaveOccurred())

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should deny because at least one project does not have the deletion confirmation annotation", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Project").WithVersion("version"), "", "", core.Resource("projects").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						project2 := project.DeepCopy()
						project2.Name = "dummy2"
						project.Annotations = map[string]string{"confirmation.gardener.cloud/deletion": "true"}

						Expect(projectStore.Add(project2)).NotTo(HaveOccurred())

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("ShootState resources", func() {
				It("should do nothing because the resource is already removed", func() {
					attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, shootState.Name, core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
					gardenClient.AddReactor("get", "shootstates", func(_ testing.Action) (bool, runtime.Object, error) {
						return true, nil, fmt.Errorf("shoot.core.gardener.cloud \"dummyName\" not found")
					})

					err := admissionHandler.Validate(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(`shoot.core.gardener.cloud "dummyName" not found`))
				})

				Context("delete", func() {
					It("should reject for nil annotation field", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, shootState.Name, core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should reject for false annotation value", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, shootState.Name, core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						shootState.Annotations = map[string]string{
							"confirmation.gardener.cloud/deletion": "false",
						}
						Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should succeed for true annotation value (cache lookup)", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, shootState.Name, core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						shootState.Annotations = map[string]string{
							"confirmation.gardener.cloud/deletion": "true",
						}
						Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should succeed for true annotation value (live lookup)", func() {
						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState.Namespace, shootState.Name, core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())
						gardenClient.AddReactor("get", "shootstates", func(_ testing.Action) (bool, runtime.Object, error) {
							return true, &gardencorev1beta1.ShootState{
								ObjectMeta: metav1.ObjectMeta{
									Name:      shootState.Name,
									Namespace: shootState.Namespace,
									Annotations: map[string]string{
										"confirmation.gardener.cloud/deletion": "true",
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

						shootState.Annotations = map[string]string{"confirmation.gardener.cloud/deletion": "true"}
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
						shootState.Annotations = map[string]string{"confirmation.gardener.cloud/deletion": "true"}

						Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())
						Expect(shootStateStore.Add(shootState2)).NotTo(HaveOccurred())

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).To(HaveOccurred())
					})

					It("should not deny deletecollection in this namespace because there is a shootstate in another namespace which does not have the deletion confirmation annotation", func() {
						shootState2 := shootState.DeepCopy()
						shootState2.Name = "dummyName2"
						shootState2.Namespace = "dummyNs2"
						shootState2.Annotations = map[string]string{"confirmation.gardener.cloud/deletion": "true"}

						project2 := project.DeepCopy()
						project2.Name = shootState2.Namespace
						project2.Spec.Namespace = &shootState2.Namespace
						Expect(projectStore.Add(project2)).To(Succeed())

						attrs = admission.NewAttributesRecord(nil, nil, core.Kind("ShootState").WithVersion("version"), shootState2.Namespace, "", core.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

						Expect(shootStateStore.Add(&shootState)).NotTo(HaveOccurred())
						Expect(shootStateStore.Add(shootState2)).NotTo(HaveOccurred())

						err := admissionHandler.Validate(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})
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
		It("should only handle CREATE,UPDATE,DELETE operations", func() {
			dr, err := New()

			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).To(BeTrue())
			Expect(dr.Handles(admission.Update)).To(BeTrue())
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
			gardenClient := &internalClientSet.Clientset{}
			dr.SetCoreClientSet(gardenClient)
			dr.SetCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))

			err := dr.ValidateInitialization()

			Expect(err).ToNot(HaveOccurred())
		})
	})
})
