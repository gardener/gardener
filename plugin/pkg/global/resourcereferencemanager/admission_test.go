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
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/plugin/pkg/global/resourcereferencemanager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
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
			admissionHandler          *ReferenceManager
			kubeInformerFactory       kubeinformers.SharedInformerFactory
			kubeClient                *fake.Clientset
			gardenCoreInformerFactory coreinformers.SharedInformerFactory
			fakeAuthorizer            fakeAuthorizerType

			shoot core.Shoot

			namespace        = "default"
			cloudProfileName = "profile-1"
			seedName         = "seed-1"
			bindingName      = "binding-1"
			quotaName        = "quota-1"
			secretName       = "secret-1"
			configMapName    = "config-map-1"
			shootName        = "shoot-1"
			projectName      = "project-1"
			allowedUser      = "allowed-user"
			resourceVersion  = "123456"
			finalizers       = []string{core.GardenerName}

			defaultUserName = "test-user"
			defaultUserInfo = &user.DefaultInfo{Name: defaultUserName}

			secret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:       secretName,
					Namespace:  namespace,
					Finalizers: finalizers,
				},
			}

			configMap = corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            configMapName,
					Namespace:       namespace,
					Finalizers:      finalizers,
					ResourceVersion: resourceVersion,
				},
			}

			cloudProfile = core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: cloudProfileName,
				},
			}
			seed = core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name:       seedName,
					Finalizers: finalizers,
				},
				Spec: core.SeedSpec{
					SecretRef: &corev1.SecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			}
			quota = core.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      quotaName,
					Namespace: namespace,
				},
				Spec: core.QuotaSpec{
					Scope: corev1.ObjectReference{
						APIVersion: "core.gardener.cloud/v1beta1",
						Kind:       "Project",
					},
				},
			}
			secretBinding = core.SecretBinding{
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
			project = core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}
			shootBase = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:       shootName,
					Namespace:  namespace,
					Finalizers: finalizers,
				},
				Spec: core.ShootSpec{
					CloudProfileName:  cloudProfileName,
					SeedName:          &seedName,
					SecretBindingName: bindingName,
					Kubernetes: core.Kubernetes{
						KubeAPIServer: &core.KubeAPIServerConfig{
							AuditConfig: &core.AuditConfig{
								AuditPolicy: &core.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: configMapName,
									},
								},
							},
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

			gardenCoreInformerFactory = coreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(gardenCoreInformerFactory)

			fakeAuthorizer = fakeAuthorizerType{}
			admissionHandler.SetAuthorizer(fakeAuthorizer)

			MissingSecretWait = 0

			shoot = shootBase
		})

		Context("tests for SecretBinding objects", func() {
			It("should accept because all referenced objects have been found (secret found in cache)", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept because all referenced objects have been found (secret looked up live)", func() {
				gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: secret.Namespace,
							Name:      secret.Name,
						},
					}, nil
				})

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced secret does not exist", func() {
				gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("nope, out of luck")
				})

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the user is not allowed to read the referenced secret", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)

				user := &user.DefaultInfo{Name: "disallowed-user"}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because one of the referenced quotas does not exist", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the user is not allowed to read the referenced quota", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)

				user := &user.DefaultInfo{Name: "disallowed-user"}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should pass because exact one quota per scope is referenced", func() {
				quotaName2 := "quota-2"
				quota2 := core.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      quotaName2,
						Namespace: namespace,
					},
					Spec: core.QuotaSpec{
						Scope: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
						},
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
				gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
				gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota2)

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because more than one quota of the same scope is referenced", func() {
				quotaName2 := "quota-2"
				quota2 := core.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      quotaName2,
						Namespace: namespace,
					},
					Spec: core.QuotaSpec{
						Scope: corev1.ObjectReference{
							APIVersion: "core.gardener.cloud/v1beta1",
							Kind:       "Project",
						},
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
				gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
				gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota2)

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for Seed objects", func() {
			It("should accept because all referenced objects have been found (secret found in cache)", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

				attrs := admission.NewAttributesRecord(&seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept because all referenced objects have been found (secret looked up live)", func() {
				gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: secret.Namespace,
							Name:      secret.Name,
						},
					}, nil
				})

				attrs := admission.NewAttributesRecord(&seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced secret does not exist", func() {
				gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("nope, out of luck")
				})

				attrs := admission.NewAttributesRecord(&seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject adding the disable-dns taint because shoots reference the seed", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shoot)

				newSeed := seed.DeepCopy()
				newSeed.Spec.Taints = append(newSeed.Spec.Taints, core.SeedTaint{
					Key: core.SeedTaintDisableDNS,
				})

				attrs := admission.NewAttributesRecord(newSeed, &seed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("may not add/remove \"" + core.SeedTaintDisableDNS + "\" taint when shoots are still referencing to a seed"))
			})

			It("should accept adding the disable-dns taint because no shoots reference the seed", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

				newSeed := seed.DeepCopy()
				newSeed.Spec.Taints = append(newSeed.Spec.Taints, core.SeedTaint{
					Key: core.SeedTaintDisableDNS,
				})

				attrs := admission.NewAttributesRecord(newSeed, &seed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept because the secret does not have a secret ref", func() {
				seedObj := core.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Name:       seedName,
						Finalizers: finalizers,
					},
				}

				attrs := admission.NewAttributesRecord(&seedObj, nil, core.Kind("Seed").WithVersion("version"), "", seedObj.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("tests for Shoot objects", func() {
			It("should add the created-by annotation", func() {
				gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
				kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				Expect(shoot.Annotations).NotTo(HaveKeyWithValue(common.GardenCreatedBy, defaultUserName))
				Expect(shoot.Annotations).NotTo(HaveKeyWithValue(common.GardenCreatedByDeprecated, defaultUserName))

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Annotations).To(HaveKeyWithValue(common.GardenCreatedBy, defaultUserName))
				Expect(shoot.Annotations).To(HaveKeyWithValue(common.GardenCreatedByDeprecated, defaultUserName))
			})

			It("should accept because all referenced objects have been found", func() {
				gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
				kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.ResourceVersion).To(Equal(resourceVersion))
			})

			It("should reject because the referenced cloud profile does not exist", func() {
				gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
				kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced seed does not exist", func() {
				gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
				kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced secret binding does not exist", func() {
				gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced config map does not exist", func() {
				gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for Project objects", func() {
			It("should set the created-by field", func() {
				gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)

				attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(project.Spec.CreatedBy).To(Equal(&rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     rbacv1.UserKind,
					Name:     defaultUserName,
				}))
			})

			It("should set the owner field", func() {
				gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)

				attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(project.Spec.Owner).To(Equal(&rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     rbacv1.UserKind,
					Name:     defaultUserName,
				}))
			})

			It("should add the owner to members", func() {
				gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)

				attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(project.Spec.Members).To(ContainElement(Equal(core.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     rbacv1.UserKind,
						Name:     defaultUserName,
					},
					Role: core.ProjectMemberAdmin,
				})))
			})
		})
	})
})
