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
	"net/http"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	corefake "github.com/gardener/gardener/pkg/client/core/clientset/internalversion/fake"
	externalcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/global/resourcereferencemanager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
)

type fakeAuthorizerType struct{}

func (fakeAuthorizerType) Authorize(_ context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
	username := a.GetUser().GetName()

	if username == "allowed-user" {
		return authorizer.DecisionAllow, "", nil
	}

	return authorizer.DecisionDeny, "", nil
}

var _ = Describe("resourcereferencemanager", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler                  *ReferenceManager
			kubeInformerFactory               kubeinformers.SharedInformerFactory
			kubeClient                        *fake.Clientset
			gardenCoreClient                  *corefake.Clientset
			gardenCoreInformerFactory         coreinformers.SharedInformerFactory
			gardenCoreExternalInformerFactory externalcoreinformers.SharedInformerFactory
			fakeAuthorizer                    fakeAuthorizerType
			scheme                            *runtime.Scheme
			dynamicClient                     *dynamicfake.FakeDynamicClient

			shoot core.Shoot

			namespace                  = "default"
			cloudProfileName           = "profile-1"
			seedName                   = "seed-1"
			bindingName                = "binding-1"
			quotaName                  = "quota-1"
			secretName                 = "secret-1"
			configMapName              = "config-map-1"
			controllerDeploymentName   = "controller-deployment-1"
			controllerRegistrationName = "controller-reg-1"
			shootName                  = "shoot-1"
			projectName                = "project-1"
			allowedUser                = "allowed-user"
			resourceVersion            = "123456"
			finalizers                 = []string{core.GardenerName}

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

			controllerDeployment = gardencore.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerDeploymentName,
				},
			}

			cloudProfile = core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: cloudProfileName,
				},
			}
			controllerRegistration = core.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerRegistrationName,
				},
				Spec: core.ControllerRegistrationSpec{
					Deployment: &core.ControllerRegistrationDeployment{
						DeploymentRefs: []core.DeploymentRef{
							{Name: controllerDeploymentName},
						},
					},
				},
			}
			project = core.Project{}
			seed    = core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name:       seedName,
					Finalizers: finalizers,
				},
				Spec: core.SeedSpec{
					SecretRef: &corev1.SecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
					Settings: &core.SeedSettings{
						ShootDNS: &core.SeedSettingShootDNS{
							Enabled: true,
						},
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
			projectBase = core.Project{
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
					Resources: []core.NamedResourceReference{
						{
							Name: secretName,
							ResourceRef: autoscalingv1.CrossVersionObjectReference{
								Kind:       "Secret",
								Name:       secretName,
								APIVersion: "v1",
							},
						},
					},
				},
			}

			discoveryClientResources = []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "secrets",
							Namespaced: true,
							Group:      "core",
							Version:    "v1",
							Kind:       "Secret",
						},
					},
				},
			}

			discoveryGardenClientResources = []*metav1.APIResourceList{
				{
					GroupVersion: "",
					APIResources: []metav1.APIResource{
						{
							Name:       "controllerdeployments",
							Namespaced: false,
							Group:      "core.gardener.cloud",
							Version:    "",
							Kind:       "ControllerDeployment",
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

			kubeClient = fake.NewSimpleClientset()
			kubeClient.Fake = testing.Fake{Resources: discoveryClientResources}
			admissionHandler.SetKubeClientset(kubeClient)

			gardenCoreClient = corefake.NewSimpleClientset()
			gardenCoreClient.Fake = testing.Fake{Resources: discoveryGardenClientResources}
			admissionHandler.SetInternalCoreClientset(gardenCoreClient)

			gardenCoreInformerFactory = coreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(gardenCoreInformerFactory)

			gardenCoreExternalInformerFactory = externalcoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetExternalCoreInformerFactory(gardenCoreExternalInformerFactory)

			fakeAuthorizer = fakeAuthorizerType{}
			admissionHandler.SetAuthorizer(fakeAuthorizer)

			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())

			dynamicClient = dynamicfake.NewSimpleDynamicClient(scheme, &secret)
			admissionHandler.SetDynamicClient(dynamicClient)

			MissingSecretWait = 0

			project = projectBase
			shoot = shootBase
		})

		Context("tests for ControllerRegistration objects", func() {
			It("should accept because all referenced objects have been found (controller deployment found in cache)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().ControllerDeployments().Informer().GetStore().Add(&controllerDeployment)).To(Succeed())

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&controllerRegistration, nil, core.Kind("ControllerRegistration").WithVersion("version"), "", controllerRegistration.Name, core.Resource("controllerregistrations").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept because all referenced objects have been found (controller deployment looked up live)", func() {
				gardenCoreClient.AddReactor("get", "controllerdeployments", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &controllerDeployment, nil
				})

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&controllerRegistration, nil, core.Kind("ControllerRegistration").WithVersion("version"), "", controllerRegistration.Name, core.Resource("controllerregistrations").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced secret does not exist", func() {
				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&controllerRegistration, nil, core.Kind("ControllerRegistration").WithVersion("version"), "", controllerRegistration.Name, core.Resource("controllerregistrations").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)
				gardenCoreClient.AddReactor("get", "controllerdeployments", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("nope, out of luck")
				})

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("nope, out of luck"))
			})
		})

		Context("tests for SecretBinding objects", func() {
			It("should accept because all referenced objects have been found (secret found in cache)", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)).To(Succeed())

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept because all referenced objects have been found (secret looked up live)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)).To(Succeed())
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
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)).To(Succeed())
				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("nope, out of luck")
				})

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the user is not allowed to read the referenced secret", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)).To(Succeed())

				user := &user.DefaultInfo{Name: "disallowed-user"}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because one of the referenced quotas does not exist", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the user is not allowed to read the referenced quota", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)).To(Succeed())

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

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota2)).To(Succeed())

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

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quota2)).To(Succeed())

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), secretBinding.Namespace, secretBinding.Name, core.Resource("secretbindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for Seed objects", func() {
			It("should accept because all referenced objects have been found (secret found in cache)", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())

				attrs := admission.NewAttributesRecord(&seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept because all referenced objects have been found (secret looked up live)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
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
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("nope, out of luck")
				})

				attrs := admission.NewAttributesRecord(&seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject changing the shoot dns setting because shoots reference the seed", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shoot)).To(Succeed())

				newSeed := seed.DeepCopy()
				newSeed.Spec.Settings = &core.SeedSettings{
					ShootDNS: &core.SeedSettingShootDNS{
						Enabled: false,
					},
				}

				attrs := admission.NewAttributesRecord(newSeed, &seed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("may not change shoot DNS enablement setting when shoots are still referencing to a seed"))
			})

			It("should accept changing the shoot dns setting because no shoots reference the seed", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())

				newSeed := seed.DeepCopy()
				newSeed.Spec.Settings = &core.SeedSettings{
					ShootDNS: &core.SeedSettingShootDNS{
						Enabled: false,
					},
				}

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
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				Expect(shoot.Annotations).NotTo(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, user.Name))

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, user.Name))
			})

			It("should accept because all referenced objects have been found", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept because spec was not changed", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Annotations = map[string]string{
					"delete": "me",
				}
				shoot.Labels = map[string]string{
					"nice": "label",
				}
				shoot.Status.TechnicalID = "should-never-change"
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced cloud profile does not exist (create)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced cloud profile does not exist (update)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.CloudProfileName = ""

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced seed does not exist (create)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced seed does not exist (update)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SeedName = nil

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced secret binding does not exist (create)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced secret binding does not exist (update)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SecretBindingName = ""

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			Context("exposure class reference", func() {
				var exposureClassName = "test-exposureclass"

				BeforeEach(func() {
					Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

					shoot.Spec.ExposureClassName = &exposureClassName
				})

				It("should reject because the referenced exposure class does not exists", func() {
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)
					Expect(err).To(HaveOccurred())
				})

				It("should accept because the referenced exposure class exists", func() {
					var exposureClass = gardencorev1alpha1.ExposureClass{
						ObjectMeta: metav1.ObjectMeta{
							Name: exposureClassName,
						},
					}

					Expect(gardenCoreExternalInformerFactory.Core().V1alpha1().ExposureClasses().Informer().GetStore().Add(&exposureClass)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)
					Expect(err).To(HaveOccurred())
				})
			})

			It("should reject because the referenced config map does not exist", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the user is not allowed to read the referenced resource (create)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				user := &user.DefaultInfo{Name: "disallowed-user"}
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(MatchError("shoots.core.gardener.cloud \"shoot-1\" is forbidden: shoot cannot reference a resource you are not allowed to read"))
			})

			It("should reject because the user is not allowed to read the referenced resource (update)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				shoot.Spec.Resources = append(shoot.Spec.Resources, core.NamedResourceReference{
					Name: "foo",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind:       "Secret",
						Name:       "foo",
						APIVersion: "v1",
					},
				})

				oldShoot := shoot.DeepCopy()
				shoot.Spec.Resources[0] = oldShoot.Spec.Resources[1]
				shoot.Spec.Resources[1] = oldShoot.Spec.Resources[0]

				user := &user.DefaultInfo{Name: "disallowed-user"}
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(MatchError("shoots.core.gardener.cloud \"shoot-1\" is forbidden: shoot cannot reference a resource you are not allowed to read"))
			})

			It("should allow because the user is not allowed to read the referenced resource but resource spec hasn't changed", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				oldShoot := shoot.DeepCopy()

				user := &user.DefaultInfo{Name: "disallowed-user"}
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})

			It("should reject because the referenced resource does not exist", func() {
				dynamicClient = dynamicfake.NewSimpleDynamicClient(scheme)
				admissionHandler.SetDynamicClient(dynamicClient)
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(MatchError(ContainSubstring("failed to resolve shoot resource reference")))
			})

			It("should reject because the referenced DNS provider secret does not exist (create)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				shoot.Spec.DNS = &core.DNS{
					Providers: []core.DNSProvider{
						{SecretName: pointer.String("foo")},
					},
				}

				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("nope, out of luck")
				})

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(MatchError(ContainSubstring("failed to reference DNS provider secret")))
			})

			It("should reject because the referenced DNS provider secret does not exist (update)", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				oldShoot := shoot.DeepCopy()

				shoot.Spec.DNS = &core.DNS{
					Providers: []core.DNSProvider{
						{SecretName: pointer.String("foo")},
					},
				}

				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("nope, out of luck")
				})

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(MatchError(ContainSubstring("failed to reference DNS provider secret")))
			})

			It("should pass because the referenced DNS provider secret does not exist but shoot has deletion timestamp", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				oldShoot := shoot.DeepCopy()

				shoot.Spec.DNS = &core.DNS{
					Providers: []core.DNSProvider{
						{SecretName: pointer.String("foo")},
					},
				}

				now := metav1.Now()
				shoot.DeletionTimestamp = &now

				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("nope, out of luck")
				})

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})

			It("should pass because the referenced DNS provider secret exists", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().ConfigMaps().Informer().GetStore().Add(&configMap)).To(Succeed())

				shoot.Spec.DNS = &core.DNS{
					Providers: []core.DNSProvider{
						{SecretName: pointer.String("foo")},
					},
				}

				kubeClient.AddReactor("get", "secrets", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, nil
				})

				user := &user.DefaultInfo{Name: allowedUser}
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, user)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})
		})

		Context("tests for Project objects", func() {
			It("should set the created-by field", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())

				attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(project.Spec.CreatedBy).To(Equal(&rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     rbacv1.UserKind,
					Name:     defaultUserName,
				}))
			})

			It("should set the owner field (member with owner role found)", func() {
				projectCopy := project.DeepCopy()
				ownerMember := &rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     rbacv1.UserKind,
					Name:     "owner",
				}
				projectCopy.Name = "foo"
				projectCopy.Spec.Members = []core.ProjectMember{
					{
						Subject: *ownerMember,
						Roles:   []string{core.ProjectMemberOwner},
					},
				}

				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(projectCopy)).To(Succeed())

				attrs := admission.NewAttributesRecord(projectCopy, nil, core.Kind("Project").WithVersion("version"), projectCopy.Namespace, projectCopy.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(projectCopy.Spec.Owner).To(Equal(ownerMember))
				Expect(projectCopy.Spec.CreatedBy).To(Equal(&rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     rbacv1.UserKind,
					Name:     defaultUserName,
				}))
			})

			It("should set the owner field", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())

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
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())

				attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(project.Spec.Members).To(ContainElement(Equal(core.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     rbacv1.UserKind,
						Name:     defaultUserName,
					},
					Roles: []string{
						core.ProjectMemberAdmin,
						core.ProjectMemberOwner,
					},
				})))
			})

			It("should allow specifying a namespace which is not in use (create)", func() {
				project.Spec.Namespace = pointer.String("garden-foo")
				projectCopy := project.DeepCopy()
				projectCopy.Name = "project-2"
				projectCopy.Spec.Namespace = pointer.String("garden-bar")
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(projectCopy)).To(Succeed())

				attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})

			It("should allow specifying a namespace which is not in use (update)", func() {
				projectOld := project.DeepCopy()
				projectCopy := project.DeepCopy()
				project.Spec.Namespace = pointer.String("garden-foo")
				projectCopy.Name = "project-2"
				projectCopy.Spec.Namespace = pointer.String("garden-bar")
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(projectOld)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(projectCopy)).To(Succeed())

				attrs := admission.NewAttributesRecord(&project, projectOld, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})

			It("should allow specifying multiple projects w/o a namespace", func() {
				projectCopy := project.DeepCopy()
				projectCopy.Name = "project-2"
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(projectCopy)).To(Succeed())

				attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})

			It("should forbid specifying a namespace which is already used by another project (create)", func() {
				project.Spec.Namespace = pointer.String("garden-foo")
				projectCopy := project.DeepCopy()
				projectCopy.Name = "project-2"
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(projectCopy)).To(Succeed())

				attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"ErrStatus": MatchFields(IgnoreExtras, Fields{
						"Code":    Equal(int32(http.StatusForbidden)),
						"Message": ContainSubstring("namespace \"garden-foo\" is already used by another project"),
					}),
				})))
			})

			It("should forbid specifying a namespace which is already used by another project (update)", func() {
				projectOld := project.DeepCopy()
				project.Spec.Namespace = pointer.String("garden-foo")
				projectCopy := project.DeepCopy()
				projectCopy.Name = "project-2"
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(projectOld)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(projectCopy)).To(Succeed())

				attrs := admission.NewAttributesRecord(&project, projectOld, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"ErrStatus": MatchFields(IgnoreExtras, Fields{
						"Code":    Equal(int32(http.StatusForbidden)),
						"Message": ContainSubstring("namespace \"garden-foo\" is already used by another project"),
					}),
				})))
			})
		})

		Context("CloudProfile - Update Kubernetes versions", func() {
			versions := []core.ExpirableVersion{
				{Version: "1.17.2"},
				{Version: "1.17.1"},
				{Version: "1.17.0"},
			}
			shootOne := shootBase
			shootOne.Name = "shoot-One"
			shootOne.Spec.Provider.Type = "aws"
			shootOne.Spec.CloudProfileName = "aws-profile"
			shootOne.Spec.Kubernetes.Version = "1.17.2"

			shootTwo := shootOne
			shootTwo.Name = "shoot-Two"
			shootTwo.Spec.Kubernetes.Version = "1.17.1"
			var (
				cloudProfile = core.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{Name: "aws-profile"},
					Spec: core.CloudProfileSpec{
						Kubernetes: core.KubernetesSettings{
							Versions: versions,
						},
					},
				}
			)
			It("should accept if no kubernetes version has been removed", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootOne)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootTwo)).To(Succeed())

				attrs := admission.NewAttributesRecord(&cloudProfile, &cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept removal of kubernetes version that is not in use by any shoot", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootOne)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootTwo)).To(Succeed())

				cloudProfileNew := cloudProfile
				cloudProfileNew.Spec = core.CloudProfileSpec{
					Kubernetes: core.KubernetesSettings{
						Versions: []core.ExpirableVersion{
							{Version: "1.17.2"},
							{Version: "1.17.1"},
						},
					},
				}

				attrs := admission.NewAttributesRecord(&cloudProfileNew, &cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject removal of kubernetes versions that are still in use by shoots", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootOne)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootTwo)).To(Succeed())

				cloudProfileNew := cloudProfile
				cloudProfileNew.Spec = core.CloudProfileSpec{
					Kubernetes: core.KubernetesSettings{
						Versions: []core.ExpirableVersion{
							{Version: "1.17.2"},
						},
					},
				}

				attrs := admission.NewAttributesRecord(&cloudProfileNew, &cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("1.17.1"))
			})

			It("should accept removal of kubernetes version that is still in use by a shoot that is being deleted", func() {
				t := metav1.Now()
				shootTwoDeleted := shootTwo
				shootTwoDeleted.DeletionTimestamp = &t

				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootOne)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootTwoDeleted)).To(Succeed())

				cloudProfileNew := cloudProfile
				cloudProfileNew.Spec = core.CloudProfileSpec{
					Kubernetes: core.KubernetesSettings{
						Versions: []core.ExpirableVersion{
							{Version: "1.17.2"},
						},
					},
				}

				attrs := admission.NewAttributesRecord(&cloudProfileNew, &cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("CloudProfile - Update Machine image versions", func() {
			versions := []core.MachineImageVersion{
				{
					ExpirableVersion: core.ExpirableVersion{
						Version: "1.17.3",
					},
				},
				{
					ExpirableVersion: core.ExpirableVersion{
						Version: "1.17.2",
					},
				},
				{
					ExpirableVersion: core.ExpirableVersion{
						Version: "1.17.1",
					},
				},
				{
					ExpirableVersion: core.ExpirableVersion{
						Version: "1.17.0",
					},
				},
				{
					ExpirableVersion: core.ExpirableVersion{
						Version: "1.16.0",
					},
				},
			}
			shootOne := shootBase
			shootOne.Name = "shoot-One"
			shootOne.Spec.Provider.Type = "aws"
			shootOne.Spec.CloudProfileName = "aws-profile"
			shootOne.Spec.Provider.Workers = []core.Worker{
				{
					Name: "coreos-worker",
					Machine: core.Machine{
						Image: &core.ShootMachineImage{
							Name:    "coreos",
							Version: "1.17.3",
						},
					},
				},
			}

			shootTwo := shootOne
			shootTwo.Name = "shoot-Two"
			shootTwo.Spec.Provider.Workers = []core.Worker{
				{
					Name: "ubuntu-worker-1",
					Machine: core.Machine{
						Image: &core.ShootMachineImage{
							Name:    "ubuntu",
							Version: "1.17.2",
						},
					},
				},
				{
					Name: "ubuntu-worker-2",
					Machine: core.Machine{
						Image: &core.ShootMachineImage{
							Name:    "ubuntu",
							Version: "1.17.1",
						},
					},
				},
			}

			var (
				cloudProfile = core.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{Name: "aws-profile"},
					Spec: core.CloudProfileSpec{
						MachineImages: []core.MachineImage{
							{
								Name:     "coreos",
								Versions: versions,
							},
							{
								Name:     "ubuntu",
								Versions: versions,
							},
						},
					},
				}
			)
			It("should accept if no machine image version has been removed", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootOne)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootTwo)).To(Succeed())

				attrs := admission.NewAttributesRecord(&cloudProfile, &cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})
			It("should accept removal of a machine version that is not in use by any shoot", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootOne)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootTwo)).To(Succeed())

				newVersions := []core.MachineImageVersion{
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.3",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.2",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.1",
						},
					},
				}

				// new cloud profile has version 1.17.0 and 1.16.0 removed. These are not in use by any worker of any shoot
				cloudProfileNew := cloudProfile
				cloudProfileNew.Spec = core.CloudProfileSpec{
					MachineImages: []core.MachineImage{
						{
							Name:     "coreos",
							Versions: newVersions,
						},
						{
							Name:     "ubuntu",
							Versions: newVersions,
						},
					},
				}

				attrs := admission.NewAttributesRecord(&cloudProfileNew, &cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject removal of a machine image version that is in use by a shoot", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootOne)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootTwo)).To(Succeed())

				newVersions := []core.MachineImageVersion{
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.3",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.0",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.16.0",
						},
					},
				}

				// new cloud profile has version 1.17.1 removed.
				cloudProfileNew := cloudProfile
				cloudProfileNew.Spec = core.CloudProfileSpec{
					MachineImages: []core.MachineImage{
						{
							Name:     "coreos",
							Versions: newVersions,
						},
						{
							Name:     "ubuntu",
							Versions: newVersions,
						},
					},
				}

				attrs := admission.NewAttributesRecord(&cloudProfileNew, &cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("1.17.2"))
				Expect(err.Error()).To(ContainSubstring("1.17.1"))
				Expect(err.Error()).To(ContainSubstring(shootTwo.Spec.Provider.Workers[0].Machine.Image.Name))
				Expect(err.Error()).To(ContainSubstring(shootTwo.Spec.Provider.Workers[1].Machine.Image.Name))
			})

			It("should accept removal of a machine image version that is in use by a shoot that is being deleted", func() {
				t := metav1.Now()
				shootTwoDeleted := shootTwo
				shootTwoDeleted.DeletionTimestamp = &t

				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootOne)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootTwoDeleted)).To(Succeed())

				newVersions := []core.MachineImageVersion{
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.3",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.0",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.16.0",
						},
					},
				}

				// new cloud profile has version 1.17.1 removed.
				cloudProfileNew := cloudProfile
				cloudProfileNew.Spec = core.CloudProfileSpec{
					MachineImages: []core.MachineImage{
						{
							Name:     "coreos",
							Versions: newVersions,
						},
						{
							Name:     "ubuntu",
							Versions: newVersions,
						},
					},
				}

				attrs := admission.NewAttributesRecord(&cloudProfileNew, &cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			// for existing Gardener installations
			// Shoot uses Machine Image that does not exist in the CloudProfile and uses machine image version that should be removed
			It("should reject deletion of image version", func() {
				s := shootTwo.DeepCopy()
				s.Spec.Provider.Workers = []core.Worker{
					{
						Name: "dummy-worker-1",
						Machine: core.Machine{
							Image: &core.ShootMachineImage{
								Name: "dummy",
								// version does not matter for this test, as image does not exist
								Version: "1.1.1",
							},
						},
					},
					{
						Name: "ubuntu-worker",
						Machine: core.Machine{
							Image: &core.ShootMachineImage{
								Name:    "ubuntu",
								Version: "1.17.2",
							},
						},
					},
				}
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootOne)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(s)).To(Succeed())

				newVersions := []core.MachineImageVersion{
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.3",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.0",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.16.0",
						},
					},
				}

				// new cloud profile has version 1.17.1 removed.
				cloudProfileNew := cloudProfile
				cloudProfileNew.Spec = core.CloudProfileSpec{
					MachineImages: []core.MachineImage{
						{
							Name:     "coreos",
							Versions: newVersions,
						},
						{
							Name:     "ubuntu",
							Versions: newVersions,
						},
					},
				}

				attrs := admission.NewAttributesRecord(&cloudProfileNew, &cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("1.17.2"))
				Expect(err.Error()).To(ContainSubstring(s.Spec.Provider.Workers[1].Name))
			})

			It("should reject removal of a whole machine image which versions are in use by a shoot", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootOne)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shootTwo)).To(Succeed())

				newVersions := []core.MachineImageVersion{
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.3",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.2",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.1",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.17.0",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "1.16.0",
						},
					},
				}

				// new cloud profile has ubuntu image removed.
				cloudProfileNew := cloudProfile
				cloudProfileNew.Spec = core.CloudProfileSpec{
					MachineImages: []core.MachineImage{
						{
							Name:     "coreos",
							Versions: newVersions,
						},
					},
				}

				attrs := admission.NewAttributesRecord(&cloudProfileNew, &cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, defaultUserInfo)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("1.17.2"))
				Expect(err.Error()).To(ContainSubstring("1.17.1"))
				Expect(err.Error()).To(ContainSubstring(shootTwo.Spec.Provider.Workers[0].Name))
				Expect(err.Error()).To(ContainSubstring(shootTwo.Spec.Provider.Workers[1].Name))
			})
		})
	})
})
