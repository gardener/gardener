// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package resourcereferencemanager_test

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/plugin/pkg/global/resourcereferencemanager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeAuthorizerType struct{}

func (fakeAuthorizerType) Authorize(a authorizer.Attributes) (authorizer.Decision, string, error) {
	username := a.GetUser().GetName()

	if username == "allowed-user" {
		return authorizer.DecisionAllow, "", nil
	}

	return authorizer.DecisionDeny, "", nil
}

var _ = Describe("resourcereferencemanager", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler      *ReferenceManager
			kubeInformerFactory   kubeinformers.SharedInformerFactory
			kubeClient            *fake.Clientset
			gardenInformerFactory gardeninformers.SharedInformerFactory
			fakeAuthorizer        fakeAuthorizerType

			shoot garden.Shoot

			namespace        = "default"
			cloudProfileName = "profile-1"
			seedName         = "seed-1"
			bindingName      = "binding-1"
			quotaName        = "quota-1"
			secretName       = "secret-1"
			shootName        = "shoot-1"
			projectName      = "project-1"
			allowedUser      = "allowed-user"
			finalizers       = []string{garden.GardenerName}

			defaultUserName = "test-user"
			defaultUserInfo = &user.DefaultInfo{Name: defaultUserName}

			secret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:       secretName,
					Namespace:  namespace,
					Finalizers: finalizers,
				},
			}

			cloudProfile = garden.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: cloudProfileName,
				},
			}
			seed = garden.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name:       seedName,
					Finalizers: finalizers,
				},
				Spec: garden.SeedSpec{
					Cloud: garden.SeedCloud{
						Profile: cloudProfileName,
					},
					SecretRef: corev1.SecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			}
			quota = garden.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      quotaName,
					Namespace: namespace,
				},
				Spec: garden.QuotaSpec{
					Scope: garden.QuotaScopeProject,
				},
			}
			secretBinding = garden.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       bindingName,
					Namespace:  namespace,
					Finalizers: finalizers,
				},
				SecretRef: corev1.SecretReference{
					Name:      secretName,
					Namespace: namespace,
				},
				Quotas: []corev1.ObjectReference{
					{
						Name:      quotaName,
						Namespace: namespace,
					},
				},
			}
			project = garden.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}
			shootBase = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:       shootName,
					Namespace:  namespace,
					Finalizers: finalizers,
				},
				Spec: garden.ShootSpec{
					Cloud: garden.Cloud{
						Profile: cloudProfileName,
						Seed:    &seedName,
						SecretBindingRef: corev1.LocalObjectReference{
							Name: bindingName,
						},
					},
				},
			}
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)

			kubeClient = &fake.Clientset{}
			admissionHandler.SetKubeClientset(kubeClient)

			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

			fakeAuthorizer = fakeAuthorizerType{}
			admissionHandler.SetAuthorizer(fakeAuthorizer)

			MissingSecretWait = 0

			shoot = shootBase
		})

		Context("tests for SecretBinding objects", func() {
			It("should accept because all referenced objects have been found (secret found in cache)", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, garden.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, garden.Resource("secretbindings").WithVersion("version"), "", admission.Create, false, user)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept because all referenced objects have been found (secret looked up live)", func() {
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: secret.Namespace,
							Name:      secret.Name,
						},
					}, nil
				})

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, garden.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, garden.Resource("secretbindings").WithVersion("version"), "", admission.Create, false, user)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced secret does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("nope, out of luck")
				})

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, garden.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, garden.Resource("secretbindings").WithVersion("version"), "", admission.Create, false, user)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the user is not allowed to read the referenced secret", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)

				user := &user.DefaultInfo{Name: "disallowed-user"}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, garden.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, garden.Resource("secretbindings").WithVersion("version"), "", admission.Create, false, user)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because one of the referenced quotas does not exist", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, garden.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, garden.Resource("secretbindings").WithVersion("version"), "", admission.Create, false, user)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the user is not allowed to read the referenced quota", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)

				user := &user.DefaultInfo{Name: "disallowed-user"}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, garden.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, garden.Resource("secretbindings").WithVersion("version"), "", admission.Create, false, user)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should pass because exact one quota per scope is referenced", func() {
				quotaName2 := "quota-2"
				quota2 := garden.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      quotaName2,
						Namespace: namespace,
					},
					Spec: garden.QuotaSpec{
						Scope: garden.QuotaScopeSecret,
					},
				}

				quota2Ref := corev1.ObjectReference{
					Name:      quotaName2,
					Namespace: namespace,
				}
				quotaRefList := secretBinding.Quotas
				quotaRefList = append(quotaRefList, quota2Ref)
				secretBinding.Quotas = quotaRefList

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota2)

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, garden.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, garden.Resource("secretbindings").WithVersion("version"), "", admission.Create, false, user)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because more than one quota of the same scope is referenced", func() {
				quotaName2 := "quota-2"
				quota2 := garden.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      quotaName2,
						Namespace: namespace,
					},
					Spec: garden.QuotaSpec{
						Scope: garden.QuotaScopeProject,
					},
				}

				quota2Ref := corev1.ObjectReference{
					Name:      quotaName2,
					Namespace: namespace,
				}
				quotaRefList := secretBinding.Quotas
				quotaRefList = append(quotaRefList, quota2Ref)
				secretBinding.Quotas = quotaRefList

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota2)

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, garden.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, garden.Resource("secretbindings").WithVersion("version"), "", admission.Create, false, user)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for Seed objects", func() {
			It("should accept because all referenced objects have been found (secret found in cache)", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

				attrs := admission.NewAttributesRecord(&seed, nil, garden.Kind("Seed").WithVersion("version"), "", seed.Name, garden.Resource("seeds").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept because all referenced objects have been found (secret looked up live)", func() {
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: secret.Namespace,
							Name:      secret.Name,
						},
					}, nil
				})

				attrs := admission.NewAttributesRecord(&seed, nil, garden.Kind("Seed").WithVersion("version"), "", seed.Name, garden.Resource("seeds").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced cloud profile does not exist", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)

				attrs := admission.NewAttributesRecord(&seed, nil, garden.Kind("Seed").WithVersion("version"), "", seed.Name, garden.Resource("seeds").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced secret does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("nope, out of luck")
				})

				attrs := admission.NewAttributesRecord(&seed, nil, garden.Kind("Seed").WithVersion("version"), "", seed.Name, garden.Resource("seeds").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for Shoot objects", func() {
			It("should add the created-by annotation", func() {
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, defaultUserInfo)

				Expect(shoot.Annotations).NotTo(HaveKeyWithValue(common.GardenCreatedBy, defaultUserName))

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Annotations).To(HaveKeyWithValue(common.GardenCreatedBy, defaultUserName))
			})

			It("should accept because all referenced objects have been found", func() {
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, defaultUserInfo)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced cloud profile does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, defaultUserInfo)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced seed does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, defaultUserInfo)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced secret binding does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, defaultUserInfo)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for Project objects", func() {
			It("should add the created-by annotation", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)

				attrs := admission.NewAttributesRecord(&project, nil, garden.Kind("Project").WithVersion("version"), project.Namespace, project.Name, garden.Resource("projects").WithVersion("version"), "", admission.Create, false, defaultUserInfo)

				Expect(project.Annotations).NotTo(HaveKeyWithValue(common.GardenCreatedBy, defaultUserName))

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(project.Annotations).To(HaveKeyWithValue(common.GardenCreatedBy, defaultUserName))
			})
		})
	})
})
