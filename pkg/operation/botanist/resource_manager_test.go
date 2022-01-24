// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	mockresourcemanager "github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ResourceManager", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployGardenerResourceManager", func() {
		var (
			resourceManager *mockresourcemanager.MockInterface

			ctx     = context.TODO()
			fakeErr = fmt.Errorf("fake err")

			c             *mockclient.MockClient
			k8sSeedClient kubernetes.Interface

			seedNamespace = "fake-seed-ns"

			secretNameCA         = "ca"
			secretNameServer     = "gardener-resource-manager-server"
			secretChecksumServer = "5678"
			secretChecksumCA     = "9012"
			secretDataServerCA   = map[string][]byte{
				"ca.crt": []byte(`-----BEGIN CERTIFICATE-----
MIIDYDCCAkigAwIBAgIUEb00DjvE8F0HiGOlQY/B/AG1AjMwDQYJKoZIhvcNAQEL
BQAwSDELMAkGA1UEBhMCVVMxCzAJBgNVBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJh
bmNpc2NvMRQwEgYDVQQDEwtleGFtcGxlLm5ldDAeFw0yMDA1MTEwODU0MDBaFw0y
NTA1MTAwODU0MDBaMEgxCzAJBgNVBAYTAlVTMQswCQYDVQQIEwJDQTEWMBQGA1UE
BxMNU2FuIEZyYW5jaXNjbzEUMBIGA1UEAxMLZXhhbXBsZS5uZXQwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQC/YYT0dKAY9/1lR20Gs2CfAYdR3dMcc5bE
ncD6pWoes8eZ54Am9J572o7hvVto73S/FcxF5Do06owJPVRZvHcgI4tRzbMvj5Is
XyqmMyRayoayBKkaM6f/mOPjGonSIPl90ZboaDARp9Vk9MfoLoXvRWLLF/TNpQf9
Y2rAPkInJ0fekooZewob/amoTw2Ek/KLWaQHjr6LfddL0yXotniIG6iWddSmcrON
zZEAskqNqgau6kmfK06XUK2hyJL6voeTyPchtQh83bXG/Rih43CzO3qDY6IMnJH3
0wsGFrJDsLq/9bDO8X3xFwU7YoH628BzIyFHuixEz5uNdwE35L7LAgMBAAGjQjBA
MA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRIb9F1
tE70YCjoXdjm5LeJSK9eojANBgkqhkiG9w0BAQsFAAOCAQEAdVb8yUICkTt/jENU
gvYBF3EGr8nNVC728Vatx1gmzVgudCydqHWx7clV5nuLgikXziNBpEwvr2ige3WL
BwJ041CL8wBM2A/hhYkiujulpw0M1rzp/RGUvr5fxIHVvkaztB1lksWx83JCFzwC
u8zZlKIZNR44i5pcrMylzLuyiwoFF4dJrYOgCbV2STAnDUEsT4k15ifT98Cg3Yuz
xyIt/1Lw9rpEEUEB1hg3ouLDh4xjXFlJWSMw8X+n0DXToRWUrU1WPGut4C2uX58Y
RMn7d2EqHa/0L8QHsfJBSb4IiCZ3lkPnTICC0E9/dXagH0jq76TPcmFjIy1vIPuX
nvslPg==
-----END CERTIFICATE-----`),
				"ca.key": []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAv2GE9HSgGPf9ZUdtBrNgnwGHUd3THHOWxJ3A+qVqHrPHmeeA
JvSee9qO4b1baO90vxXMReQ6NOqMCT1UWbx3ICOLUc2zL4+SLF8qpjMkWsqGsgSp
GjOn/5jj4xqJ0iD5fdGW6GgwEafVZPTH6C6F70Viyxf0zaUH/WNqwD5CJydH3pKK
GXsKG/2pqE8NhJPyi1mkB46+i33XS9Ml6LZ4iBuolnXUpnKzjc2RALJKjaoGrupJ
nytOl1CtociS+r6Hk8j3IbUIfN21xv0YoeNwszt6g2OiDJyR99MLBhayQ7C6v/Ww
zvF98RcFO2KB+tvAcyMhR7osRM+bjXcBN+S+ywIDAQABAoIBAETvJWrAD2KvALDY
V2cQeX8Ml+dfFUmsQOQ1RmuB5YWFkCHZhwmBFwzZnpmlESXtCopBmcCbAnRI/4Pc
eWORRP9ojig7BY3eEvK0nLIcvb2OMZIxp49uh9bDBWKqDnaHthYhxk+UJ6xUXcLt
gIwbJdcXkQxCZsUj6orUooD4a++Zz7YGrfPZ7f1MTzhipzjwrv9sn4iTXJfijk+D
IpE1HA+jfSsnd0bENKRUoP58+iS0bNZnIspKL2PYUSVJwkDs5r/WLEAqkjT/j8oH
Sr/pHIQIh18Snp16urJPIuI0aHTG79qcD4Bcighr/mif2CehitInoPgSu5YlJ3Ki
y0jiORECgYEAwmVihnZVhk1rW5b9JAyj7mzlezdQJU5FtEqOn00H8ARgygFJNKK1
TpZxPC5nTRrnRSdeH6m4WPtSO/4lXfSY2oiO2f/S1gwWF13i1clBFxwiRnfpeDhF
LWWE/m6UvjSqoFF9D2g5NTTAtV55OgTT0NuPM0Dl9nRsyqLS2qceXXUCgYEA/AeH
5l9eUjYttDIM9zPKXSyqFzBME6P+Lje2znCRFkS0l6xPXecdjMg8zWF3CMuYKbdl
BfXqaArUdtPHN2yS3SolZkPtUdRsYncq0skmQqFjvAqsvxhHmd9vJ+14bK6qFwf9
Xk+0+6mvGXABiwfGtgKRGRVXptP065jIZP254z8CgYB68vivpqRM/yZJlWOhq0T7
hXBW0BMmpSy87PLrmiLNEVfOK6YLXmVhwRD5STgYsk1XlaCYUhXAYaQPQZyMoikS
/o+rHXxR2O8X9E+Fe3ZpkWe0Ph8x5BUMs0q8SWBWNKU+JIv+dKLKHgVMMOZnZao6
TMNzXTaU++na98R4en5gCQKBgDmwG5pOsA9PWWzKnA8laqejJpfCNVe1jOPVWuGs
AHnBZjjldxE+apQj7U7xhUadG4pI8TXJEUuZVwKP/SShlIhNMlxTJgo5/kkXj9TJ
uBk+Sc7r/piLHTCKZS4VfCAcZtB4wrUIt5t3Pp4q9h91uzVEJyQ/r11/XKtkwFHl
hdwPAoGARP6NCOGqsjC1lpasktu7nO9OB01BRBY234rInHijTDORY65nn5OuL/Xb
WifluILPHrTS8ZUNpjy6j2CUG626TPUORKmMw+p199Ve3VxlMp2g29INt+HwwJUO
nQwHTbS7lsjLl4cdJWWZ/k1euUyKSpeJtSIwiXyF2kogjOoNh84=
-----END RSA PRIVATE KEY-----`),
			}
			secrets resourcemanager.Secrets

			bootstrapKubeconfigSecret *corev1.Secret
			shootAccessSecret         *corev1.Secret
			managedResource           *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			resourceManager = mockresourcemanager.NewMockInterface(ctrl)

			c = mockclient.NewMockClient(ctrl)
			k8sSeedClient = fakekubernetes.NewClientSetBuilder().WithClient(c).Build()

			botanist.StoreCheckSum(secretNameCA, secretChecksumCA)
			botanist.StoreCheckSum(secretNameServer, secretChecksumServer)
			botanist.StoreSecret(secretNameCA, &corev1.Secret{Data: secretDataServerCA})

			botanist.K8sSeedClient = k8sSeedClient
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						ResourceManager: resourceManager,
					},
				},
				SeedNamespace: seedNamespace,
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeReconcile,
					},
				},
			})

			secrets = resourcemanager.Secrets{
				ServerCA: component.Secret{Name: secretNameCA, Checksum: secretChecksumCA, Data: secretDataServerCA},
				Server:   component.Secret{Name: secretNameServer, Checksum: secretChecksumServer},
				RootCA:   &component.Secret{Name: secretNameCA, Checksum: secretChecksumCA},
			}

			bootstrapKubeconfigSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-access-gardener-resource-manager-bootstrap",
					Namespace: seedNamespace,
				},
			}
			shootAccessSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-access-gardener-resource-manager",
					Namespace: seedNamespace,
					Annotations: map[string]string{
						"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339),
					},
				},
			}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-gardener-resource-manager",
					Namespace: seedNamespace,
				},
			}
		})

		Context("w/o bootstrapping", func() {
			Context("when GRM should not be scaled up", func() {
				AfterEach(func() {
					gomock.InOrder(
						// replicas are set to 0, i.e., GRM should not be scaled up
						resourceManager.EXPECT().GetReplicas().Return(pointer.Int32(0)),

						// always delete bootstrap kubeconfig
						c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, obj *corev1.Secret, opts ...client.DeleteOption) error {
							Expect(obj.Name).To(Equal(bootstrapKubeconfigSecret.Name))
							Expect(obj.Namespace).To(Equal(bootstrapKubeconfigSecret.Namespace))
							return nil
						}),

						// set secrets
						resourceManager.EXPECT().SetSecrets(secrets),
					)

					resourceManager.EXPECT().Deploy(ctx)
					Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
				})

				It("due to shoot reconciling and hibernated", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Hibernation: &gardencorev1beta1.Hibernation{
								Enabled: pointer.Bool(true),
							},
						},
						Status: gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								Type: gardencorev1beta1.LastOperationTypeReconcile,
							},
							IsHibernated: true,
						},
					})

					gomock.InOrder(
						resourceManager.EXPECT().GetReplicas(),
						c.EXPECT().Get(ctx, kutil.Key(seedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						resourceManager.EXPECT().SetReplicas(pointer.Int32(0)),
					)
				})

				It("due to shoot reconciling and not hibernated but deployment replicas are 0", func() {
					botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
						Status: gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								Type: gardencorev1beta1.LastOperationTypeReconcile,
							},
						},
					})

					gomock.InOrder(
						resourceManager.EXPECT().GetReplicas(),
						c.EXPECT().Get(ctx, kutil.Key(seedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						resourceManager.EXPECT().SetReplicas(pointer.Int32(0)),
					)
				})

				It("due to shoot creation and hibernated", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Hibernation: &gardencorev1beta1.Hibernation{
								Enabled: pointer.Bool(true),
							},
						},
						Status: gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								Type: gardencorev1beta1.LastOperationTypeCreate,
							},
							IsHibernated: true,
						},
					})

					gomock.InOrder(
						resourceManager.EXPECT().GetReplicas(),
						c.EXPECT().Get(ctx, kutil.Key(seedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						resourceManager.EXPECT().SetReplicas(pointer.Int32(0)),
					)
				})

				It("due to shoot restoration and hibernated", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Hibernation: &gardencorev1beta1.Hibernation{
								Enabled: pointer.Bool(true),
							},
						},
						Status: gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								Type: gardencorev1beta1.LastOperationTypeRestore,
							},
							IsHibernated: true,
						},
					})

					gomock.InOrder(
						resourceManager.EXPECT().GetReplicas(),
						c.EXPECT().Get(ctx, kutil.Key(seedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						resourceManager.EXPECT().SetReplicas(pointer.Int32(0)),
					)
				})
			})

			Context("shoot is not hibernated", func() {
				BeforeEach(func() {
					gomock.InOrder(
						resourceManager.EXPECT().GetReplicas(),
						c.EXPECT().Get(ctx, kutil.Key(seedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						resourceManager.EXPECT().SetReplicas(pointer.Int32(0)),
						resourceManager.EXPECT().GetReplicas().Return(pointer.Int32(3)),

						// ensure bootstrapping prerequisites are not met
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						}),
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),

						// always delete bootstrap kubeconfig
						c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, obj *corev1.Secret, opts ...client.DeleteOption) error {
							Expect(obj.Name).To(Equal(bootstrapKubeconfigSecret.Name))
							Expect(obj.Namespace).To(Equal(bootstrapKubeconfigSecret.Namespace))
							return nil
						}),

						// set secrets
						resourceManager.EXPECT().SetSecrets(secrets),
					)
				})

				It("should delete the bootstrap kubeconfig secret (if exists), set the secrets and deploy", func() {
					resourceManager.EXPECT().Deploy(ctx)
					Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
				})

				It("should fail when the deploy function fails", func() {
					resourceManager.EXPECT().Deploy(ctx).Return(fakeErr)
					Expect(botanist.DeployGardenerResourceManager(ctx)).To(MatchError(fakeErr))
				})
			})
		})

		Context("w/ bootstrapping", func() {
			Context("with success", func() {
				AfterEach(func() {
					defer test.WithVar(&TimeoutWaitForGardenerResourceManagerBootstrapping, time.Second)()

					gomock.InOrder(
						// create bootstrap kubeconfig
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(bootstrapKubeconfigSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, s *corev1.Secret, _ ...client.CreateOption) error {
							Expect(s.Data["kubeconfig"]).NotTo(BeNil())
							return nil
						}),

						// set secrets and deploy with bootstrap kubeconfig
						resourceManager.EXPECT().SetSecrets(&secretMatcher{
							bootstrapKubeconfigName: &bootstrapKubeconfigSecret.Name,
							serverCA:                secrets.ServerCA,
							server:                  secrets.Server,
							rootCA:                  secrets.RootCA,
						}),
						resourceManager.EXPECT().Deploy(ctx),

						// wait for shoot access secret to be reconciled and managed resource to be healthy
						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						}),
						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *resourcesv1alpha1.ManagedResource) error {
							obj.Status.ObservedGeneration = obj.Generation
							obj.Status.Conditions = []gardencorev1beta1.Condition{
								{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionTrue},
								{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
							}
							return nil
						}),

						// delete bootstrap kubeconfig
						c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, obj *corev1.Secret, opts ...client.DeleteOption) error {
							Expect(obj.Name).To(Equal(bootstrapKubeconfigSecret.Name))
							Expect(obj.Namespace).To(Equal(bootstrapKubeconfigSecret.Namespace))
							return nil
						}),

						// set secrets and deploy with shoot access token
						resourceManager.EXPECT().SetSecrets(secrets),
						resourceManager.EXPECT().Deploy(ctx),
					)

					Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
				})

				tests := func() {
					It("bootstraps because the shoot access secret was not found", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
					})

					It("bootstraps because the shoot access secret was never reconciled", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{}))
					})

					It("bootstraps because the shoot access secret was not renewed", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(-time.Hour).Format(time.RFC3339)}
							return nil
						})
					})

					It("bootstraps because the managed resource was not found", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						})
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
					})

					It("bootstraps because the managed resource indicates that the shoot access token lost access", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						})
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *resourcesv1alpha1.ManagedResource) error {
							obj.Status.ObservedGeneration = obj.Generation
							obj.Status.Conditions = []gardencorev1beta1.Condition{
								{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionFalse, Message: `forbidden: User "system:serviceaccount:kube-system:gardener-resource-manager" cannot do anything`},
								{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
							}
							return nil
						})
					})
				}

				Context("shoot is not hibernated", func() {
					BeforeEach(func() {
						botanist.Shoot.HibernationEnabled = false

						gomock.InOrder(
							resourceManager.EXPECT().GetReplicas(),
							c.EXPECT().Get(ctx, kutil.Key(seedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
							resourceManager.EXPECT().SetReplicas(pointer.Int32(0)),
							resourceManager.EXPECT().GetReplicas().Return(pointer.Int32(3)),
						)
					})

					tests()
				})

				Context("shoot is in the process of being woken-up", func() {
					BeforeEach(func() {
						botanist.Shoot.HibernationEnabled = false
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{IsHibernated: true}})

						gomock.InOrder(
							resourceManager.EXPECT().GetReplicas(),
							resourceManager.EXPECT().SetReplicas(pointer.Int32(3)),
							resourceManager.EXPECT().GetReplicas().Return(pointer.Int32(3)),
						)
					})

					tests()
				})

				Context("shoot is hibernated but GRM should be scaled up", func() {
					BeforeEach(func() {
						botanist.Shoot.HibernationEnabled = true
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{IsHibernated: true}})
						resourceManager.EXPECT().GetReplicas().Return(pointer.Int32(3)).Times(2)
					})

					tests()
				})
			})

			Context("with failure", func() {
				BeforeEach(func() {
					// ensure bootstrapping preconditions are met
					resourceManager.EXPECT().GetReplicas().Return(pointer.Int32(3)).Times(2)
					c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				})

				It("fails because the bootstrap kubeconfig secret cannot be created", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(bootstrapKubeconfigSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
					)

					Expect(botanist.DeployGardenerResourceManager(ctx)).To(MatchError(fakeErr))
				})

				Context("waiting for bootstrapping process", func() {
					BeforeEach(func() {
						gomock.InOrder(
							// create bootstrap kubeconfig
							c.EXPECT().Get(ctx, client.ObjectKeyFromObject(bootstrapKubeconfigSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
							c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),

							// set secrets and deploy with bootstrap kubeconfig
							resourceManager.EXPECT().SetSecrets(&secretMatcher{
								bootstrapKubeconfigName: &bootstrapKubeconfigSecret.Name,
								serverCA:                secrets.ServerCA,
								server:                  secrets.Server,
								rootCA:                  secrets.RootCA,
							}),
							resourceManager.EXPECT().Deploy(ctx),
						)
					})

					It("fails because the shoot access token was not generated", func() {
						defer test.WithVar(&TimeoutWaitForGardenerResourceManagerBootstrapping, time.Millisecond)()

						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
							obj.Annotations = nil
							return nil
						})

						Expect(botanist.DeployGardenerResourceManager(ctx)).To(MatchError(ContainSubstring("token not yet generated")))
					})

					It("fails because the shoot access token renew timestamp cannot be parsed", func() {
						defer test.WithVar(&TimeoutWaitForGardenerResourceManagerBootstrapping, time.Millisecond)()

						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"}
							return nil
						})

						Expect(botanist.DeployGardenerResourceManager(ctx).Error()).To(ContainSubstring("could not parse renew timestamp"))
					})

					It("fails because the shoot access token was not renewed", func() {
						defer test.WithVar(&TimeoutWaitForGardenerResourceManagerBootstrapping, time.Millisecond)()

						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(-time.Hour).Format(time.RFC3339)}
							return nil
						})

						Expect(botanist.DeployGardenerResourceManager(ctx).Error()).To(ContainSubstring("token not yet renewed"))
					})

					It("fails because the managed resource is not getting healthy", func() {
						defer test.WithVar(&TimeoutWaitForGardenerResourceManagerBootstrapping, time.Millisecond)()

						gomock.InOrder(
							c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
								obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
								return nil
							}),
							c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *resourcesv1alpha1.ManagedResource) error {
								obj.Status.ObservedGeneration = -1
								return nil
							}),
						)

						Expect(botanist.DeployGardenerResourceManager(ctx).Error()).To(ContainSubstring(fmt.Sprintf("managed resource %s/%s is not healthy", seedNamespace, managedResource.Name)))
					})
				})

				It("fails because the bootstrap kubeconfig cannot be deleted", func() {
					gomock.InOrder(
						// create bootstrap kubeconfig
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(bootstrapKubeconfigSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, s *corev1.Secret, _ ...client.CreateOption) error {
							Expect(s.Data["kubeconfig"]).NotTo(BeNil())
							return nil
						}),

						// set secrets and deploy with bootstrap kubeconfig
						resourceManager.EXPECT().SetSecrets(&secretMatcher{
							bootstrapKubeconfigName: &bootstrapKubeconfigSecret.Name,
							serverCA:                secrets.ServerCA,
							server:                  secrets.Server,
							rootCA:                  secrets.RootCA,
						}),
						resourceManager.EXPECT().Deploy(ctx),

						// wait for shoot access secret to be reconciled and managed resource to be healthy
						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						}),
						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *resourcesv1alpha1.ManagedResource) error {
							obj.Status.ObservedGeneration = obj.Generation
							obj.Status.Conditions = []gardencorev1beta1.Condition{
								{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionTrue},
								{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
							}
							return nil
						}),

						// delete bootstrap kubeconfig
						c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, obj *corev1.Secret, opts ...client.DeleteOption) error {
							Expect(obj.Name).To(Equal(bootstrapKubeconfigSecret.Name))
							Expect(obj.Namespace).To(Equal(bootstrapKubeconfigSecret.Namespace))
							return fakeErr
						}),
					)

					Expect(botanist.DeployGardenerResourceManager(ctx)).To(MatchError(fakeErr))
				})
			})
		})
	})
})

type secretMatcher struct {
	bootstrapKubeconfigName *string
	serverCA                component.Secret
	server                  component.Secret
	rootCA                  *component.Secret
}

func (m *secretMatcher) Matches(x interface{}) bool {
	req, ok := x.(resourcemanager.Secrets)
	if !ok {
		return false
	}

	if m.bootstrapKubeconfigName != nil && (req.BootstrapKubeconfig == nil || req.BootstrapKubeconfig.Name != *m.bootstrapKubeconfigName) {
		return false
	}

	if !apiequality.Semantic.DeepEqual(m.serverCA, req.ServerCA) {
		return false
	}

	if !apiequality.Semantic.DeepEqual(m.server, req.Server) {
		return false
	}

	if m.rootCA != nil && (req.RootCA == nil || !apiequality.Semantic.DeepEqual(*m.rootCA, *req.RootCA)) {
		return false
	}

	return true
}

func (m *secretMatcher) String() string {
	return fmt.Sprintf(`Secret Matcher:
bootstrapKubeconfigName: %v,
serverCA: %v
server: %v
rootCA: %v
`, m.bootstrapKubeconfigName, m.serverCA, m.server, m.rootCA)
}
