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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/mock"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/Masterminds/semver"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("operatingsystemconfig", func() {
	var (
		ctrl                  *gomock.Controller
		operatingSystemConfig *mockoperatingsystemconfig.MockInterface
		botanist              *Botanist

		ctx        = context.TODO()
		fakeErr    = fmt.Errorf("fake")
		shootState = &gardencorev1alpha1.ShootState{}

		ca                    = []byte("ca")
		caKubelet             = []byte("ca-kubelet")
		caCloudProfile        = "ca-cloud-profile"
		sshPublicKey          = []byte("ssh-public-key")
		sshPublicKeyOld       = []byte("ssh-public-key-old")
		kubernetesVersion     = "1.2.3"
		promtailRBACAuthToken = "supersecrettoken"
		ingressDomain         = "seed-test.ingress.domain.com"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		operatingSystemConfig = mockoperatingsystemconfig.NewMockInterface(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{
			Shoot: &shootpkg.Shoot{
				CloudProfile: &gardencorev1beta1.CloudProfile{},
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						OperatingSystemConfig: operatingSystemConfig,
					},
				},
				Purpose: "development",
			},
			Seed: &seedpkg.Seed{},
		}}
		botanist.StoreSecret(v1beta1constants.SecretNameCACluster, &corev1.Secret{Data: map[string][]byte{"ca.crt": ca}})
		botanist.StoreSecret(v1beta1constants.SecretNameCAKubelet, &corev1.Secret{Data: map[string][]byte{"ca.crt": caKubelet}})
		botanist.StoreSecret(v1beta1constants.SecretNameSSHKeyPair, &corev1.Secret{Data: map[string][]byte{"id_rsa.pub": sshPublicKey}})
		botanist.StoreSecret(v1beta1constants.SecretNameOldSSHKeyPair, &corev1.Secret{Data: map[string][]byte{"id_rsa.pub": sshPublicKeyOld}})
		botanist.SetShootState(shootState)
		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Spec: gardencorev1beta1.SeedSpec{
				DNS: gardencorev1beta1.SeedDNS{
					IngressDomain: &ingressDomain,
				},
			},
		})
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: "shoot--garden-testing",
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployOperatingSystemConfig", func() {
		BeforeEach(func() {
			operatingSystemConfig.EXPECT().SetKubeletCACertificate(string(caKubelet))
			operatingSystemConfig.EXPECT().SetSSHPublicKeys([]string{string(sshPublicKey), string(sshPublicKeyOld)})
		})

		Context("deploy", func() {
			It("should deploy successfully (no CA)", func() {
				botanist.LoadSecret("ca").Data["ca.crt"] = nil
				operatingSystemConfig.EXPECT().SetCABundle(nil)

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should deploy successfully (only cluster CA)", func() {
				operatingSystemConfig.EXPECT().SetCABundle(pointer.String("\n" + string(ca)))

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should deploy successfully (only CloudProfile CA)", func() {
				botanist.Shoot.CloudProfile.Spec.CABundle = &caCloudProfile
				botanist.LoadSecret("ca").Data["ca.crt"] = nil
				operatingSystemConfig.EXPECT().SetCABundle(&caCloudProfile)

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should deploy successfully shoot logging components with non testing purpose", func() {
				botanist.Shoot.Purpose = "development"
				botanist.PromtailRBACAuthToken = promtailRBACAuthToken
				botanist.Config = &config.GardenletConfiguration{
					Logging: &config.Logging{
						ShootNodeLogging: &config.ShootNodeLogging{
							ShootPurposes: []gardencore.ShootPurpose{"evaluation", "development"},
						},
					},
				}
				Expect(gardenletfeatures.FeatureGate.SetFromMap(map[string]bool{string(features.Logging): true})).To(Succeed())
				operatingSystemConfig.EXPECT().SetCABundle(pointer.StringPtr("\n" + string(ca)))
				operatingSystemConfig.EXPECT().SetPromtailRBACAuthToken(promtailRBACAuthToken)
				operatingSystemConfig.EXPECT().SetLokiIngressHostName(botanist.ComputeLokiHost())

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should deploy successfully (both cluster and CloudProfile CA)", func() {
				botanist.Shoot.CloudProfile.Spec.CABundle = &caCloudProfile
				operatingSystemConfig.EXPECT().SetCABundle(pointer.String(caCloudProfile + "\n" + string(ca)))

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				operatingSystemConfig.EXPECT().SetCABundle(pointer.String("\n" + string(ca)))

				operatingSystemConfig.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			BeforeEach(func() {
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				})

				operatingSystemConfig.EXPECT().SetCABundle(pointer.String("\n" + string(ca)))
			})

			It("should restore successfully", func() {
				operatingSystemConfig.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				operatingSystemConfig.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(MatchError(fakeErr))
			})
		})
	})

	Describe("#DeployManagedResourceForCloudConfigExecutor", func() {
		var (
			kubernetesInterfaceSeed  *mockkubernetes.MockInterface
			kubernetesClientSeed     *mockclient.MockClient
			kubernetesInterfaceShoot *mockkubernetes.MockInterface
			kubernetesClientShoot    *mockclient.MockClient

			namespace = "shoot--foo--bar"
			imageVec  = imagevector.ImageVector{{Name: "hyperkube"}}

			bootstrapTokenID     = "123"
			bootstrapTokenSecret = "456"

			worker1Name            = "worker1"
			worker1OriginalContent = "w1content"
			worker1OriginalCommand = "/foo"
			worker1OriginalUnits   = []string{"w1u1", "w1u2"}
			worker1Key             = "cloud-config-" + worker1Name + "-77ac3"

			worker2Name                  = "worker2"
			worker2OriginalContent       = "w2content"
			worker2OriginalCommand       = "/bar"
			worker2OriginalUnits         = []string{"w2u2", "w2u2", "w2u3"}
			worker2Key                   = "cloud-config-" + worker2Name + "-77ac3"
			worker2KubeletDataVolumeName = "vol"

			workerNameToOperatingSystemConfigMaps = map[string]*operatingsystemconfig.OperatingSystemConfigs{
				worker1Name: {
					Original: operatingsystemconfig.Data{
						Content: worker1OriginalContent,
						Command: &worker1OriginalCommand,
						Units:   worker1OriginalUnits,
					},
				},
				worker2Name: {
					Original: operatingsystemconfig.Data{
						Content: worker2OriginalContent,
						Command: &worker2OriginalCommand,
						Units:   worker2OriginalUnits,
					},
				},
			}

			oldSecret1Name = "old-secret-1"
			oldSecret2Name = "old-secret-2"
		)

		BeforeEach(func() {
			kubernetesInterfaceSeed = mockkubernetes.NewMockInterface(ctrl)
			kubernetesClientSeed = mockclient.NewMockClient(ctrl)
			botanist.K8sSeedClient = kubernetesInterfaceSeed

			kubernetesInterfaceShoot = mockkubernetes.NewMockInterface(ctrl)
			kubernetesClientShoot = mockclient.NewMockClient(ctrl)
			botanist.K8sShootClient = kubernetesInterfaceShoot

			botanist.Shoot.SeedNamespace = namespace
			botanist.Shoot.KubernetesVersion = semver.MustParse("1.2.3")
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{
								Name: worker1Name,
							},
							{
								Name:                  worker2Name,
								KubeletDataVolumeName: &worker2KubeletDataVolumeName,
								DataVolumes: []gardencorev1beta1.DataVolume{
									{Name: worker2KubeletDataVolumeName},
								},
							},
						},
					},
				},
			})

			kubernetesInterfaceShoot.EXPECT().Client().Return(kubernetesClientShoot).AnyTimes()
			kubernetesInterfaceSeed.EXPECT().Client().Return(kubernetesClientSeed).AnyTimes()
		})

		type tableTestParams struct {
			downloaderGenerateRBACResourcesFnError error
			executorScriptFnError                  error

			imageVector                           imagevector.ImageVector
			workerNameToOperatingSystemConfigMaps map[string]*operatingsystemconfig.OperatingSystemConfigs

			bootstrapTokenSecretReadError            error
			managedResourceSecretReconciliationError error
			managedResourceReadError                 error
			staleSecretListingError                  error
			staleSecretDeletionError                 error
		}

		DescribeTable("table tests",
			func(params tableTestParams, matcherFn func(error)) {
				// fake function for generating downloader rbac resources
				oldDownloaderGenerateRBACResourcesDataFn := DownloaderGenerateRBACResourcesDataFn
				defer func() { DownloaderGenerateRBACResourcesDataFn = oldDownloaderGenerateRBACResourcesDataFn }()
				DownloaderGenerateRBACResourcesDataFn = func(secretNames []string) (map[string][]byte, error) {
					return map[string][]byte{"out": []byte(fmt.Sprintf("%s", secretNames))}, params.downloaderGenerateRBACResourcesFnError
				}

				// fake function for generation of executor script
				oldExecutorScriptFn := ExecutorScriptFn
				defer func() { ExecutorScriptFn = oldExecutorScriptFn }()
				ExecutorScriptFn = func(bootstrapToken string, cloudConfigUserData []byte, hyperkubeImage *imagevector.Image, kubernetesVersion string, kubeletDataVolume *gardencorev1beta1.DataVolume, reloadConfigCommand string, units []string) ([]byte, error) {
					return []byte(fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s", bootstrapToken, cloudConfigUserData, hyperkubeImage.String(), kubernetesVersion, kubeletDataVolume, reloadConfigCommand, units)), params.executorScriptFnError
				}

				// bootstrap token secret generation/retrieval
				kubernetesClientShoot.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), gomock.AssignableToTypeOf(&corev1.Secret{})).AnyTimes().Return(params.bootstrapTokenSecretReadError)

				if params.bootstrapTokenSecretReadError == nil {
					kubernetesClientShoot.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
						DoAndReturn(func(_ context.Context, obj *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
							(&corev1.Secret{Data: map[string][]byte{
								"token-id":     []byte(bootstrapTokenID),
								"token-secret": []byte(bootstrapTokenSecret),
							}}).DeepCopyInto(obj)
							return nil
						})

					// image vector for retrieval of required images
					botanist.ImageVector = params.imageVector

					if params.imageVector != nil {
						// operating system config maps retrieval for the worker pools
						operatingSystemConfig.EXPECT().WorkerNameToOperatingSystemConfigsMap().Return(params.workerNameToOperatingSystemConfigMaps)

						if params.downloaderGenerateRBACResourcesFnError == nil &&
							params.executorScriptFnError == nil &&
							params.workerNameToOperatingSystemConfigMaps != nil {

							// managed resource secret reconciliation for executor scripts for worker pools
							// worker pool 1
							worker1ExecutorScript, _ := ExecutorScriptFn(bootstrapTokenID+"."+bootstrapTokenSecret, []byte(worker1OriginalContent), &imagevector.Image{Tag: pointer.String("v")}, kubernetesVersion, nil, worker1OriginalCommand, worker1OriginalUnits)
							kubernetesClientSeed.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-shoot-cloud-config-execution-"+worker1Name), gomock.AssignableToTypeOf(&corev1.Secret{}))
							kubernetesClientSeed.EXPECT().Update(ctx, &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "managedresource-shoot-cloud-config-execution-" + worker1Name,
									Namespace: namespace,
									Labels:    map[string]string{"managed-resource": "shoot-cloud-config-execution"},
								},
								Type: corev1.SecretTypeOpaque,
								Data: map[string][]byte{"secret__kube-system__" + worker1Key + ".yaml": []byte(`apiVersion: v1
data:
  script: ` + utils.EncodeBase64(worker1ExecutorScript) + `
kind: Secret
metadata:
  annotations:
    checksum/data-script: ` + utils.ComputeSecretChecksum(map[string][]byte{"script": worker1ExecutorScript}) + `
  creationTimestamp: null
  labels:
    gardener.cloud/role: cloud-config
    worker.gardener.cloud/pool: ` + worker1Name + `
  name: ` + worker1Key + `
  namespace: kube-system
`)},
							})

							// worker pool 2
							worker2ExecutorScript, _ := ExecutorScriptFn(bootstrapTokenID+"."+bootstrapTokenSecret, []byte(worker2OriginalContent), &imagevector.Image{Tag: pointer.String("v")}, kubernetesVersion, &gardencorev1beta1.DataVolume{Name: worker2KubeletDataVolumeName}, worker2OriginalCommand, worker2OriginalUnits)
							kubernetesClientSeed.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-shoot-cloud-config-execution-"+worker2Name), gomock.AssignableToTypeOf(&corev1.Secret{}))
							kubernetesClientSeed.EXPECT().Update(ctx, &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "managedresource-shoot-cloud-config-execution-" + worker2Name,
									Namespace: namespace,
									Labels:    map[string]string{"managed-resource": "shoot-cloud-config-execution"},
								},
								Type: corev1.SecretTypeOpaque,
								Data: map[string][]byte{"secret__kube-system__" + worker2Key + ".yaml": []byte(`apiVersion: v1
data:
  script: ` + utils.EncodeBase64(worker2ExecutorScript) + `
kind: Secret
metadata:
  annotations:
    checksum/data-script: ` + utils.ComputeSecretChecksum(map[string][]byte{"script": worker2ExecutorScript}) + `
  creationTimestamp: null
  labels:
    gardener.cloud/role: cloud-config
    worker.gardener.cloud/pool: ` + worker2Name + `
  name: ` + worker2Key + `
  namespace: kube-system
`)},
							})

							// managed resource secret reconciliation for RBAC resources
							downloaderRBACResourcesData, _ := DownloaderGenerateRBACResourcesDataFn([]string{worker1Key, worker2Key})
							kubernetesClientSeed.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-shoot-cloud-config-rbac"), gomock.AssignableToTypeOf(&corev1.Secret{}))
							kubernetesClientSeed.EXPECT().Update(ctx, &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "managedresource-shoot-cloud-config-rbac",
									Namespace: namespace,
									Labels:    map[string]string{"managed-resource": "shoot-cloud-config-execution"},
								},
								Type: corev1.SecretTypeOpaque,
								Data: downloaderRBACResourcesData,
							}).Return(params.managedResourceSecretReconciliationError)

							if params.managedResourceSecretReconciliationError == nil {
								// managed resource reconciliation
								kubernetesClientSeed.EXPECT().Get(ctx, kutil.Key(namespace, "shoot-cloud-config-execution"), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(params.managedResourceReadError)

								if params.managedResourceReadError == nil {
									kubernetesClientSeed.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, obj *resourcesv1alpha1.ManagedResource, _ ...client.UpdateOption) error {
										Expect(obj.ObjectMeta).To(Equal(metav1.ObjectMeta{
											Name:      "shoot-cloud-config-execution",
											Namespace: namespace,
											Labels:    map[string]string{"origin": "gardener"},
										}))
										Expect(obj.Spec.SecretRefs).To(ConsistOf(
											corev1.LocalObjectReference{Name: "managedresource-shoot-cloud-config-execution-" + worker1Name},
											corev1.LocalObjectReference{Name: "managedresource-shoot-cloud-config-execution-" + worker2Name},
											corev1.LocalObjectReference{Name: "managedresource-shoot-cloud-config-rbac"},
										))
										Expect(obj.Spec.InjectLabels).To(Equal(map[string]string{"shoot.gardener.cloud/no-cleanup": "true"}))
										Expect(obj.Spec.KeepObjects).To(Equal(pointer.Bool(false)))
										return nil
									})

									// listing/finding of no longer required managed resource secrets
									kubernetesClientSeed.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespace), client.MatchingLabels(map[string]string{"managed-resource": "shoot-cloud-config-execution"})).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
										*list = corev1.SecretList{Items: []corev1.Secret{
											{ObjectMeta: metav1.ObjectMeta{Name: oldSecret1Name, Namespace: namespace}},
											{ObjectMeta: metav1.ObjectMeta{Name: oldSecret2Name, Namespace: namespace}},
										}}
										return nil
									}).Return(params.staleSecretListingError)

									if params.staleSecretListingError == nil {
										// cleanup of no longer required managed resource secrets
										kubernetesClientSeed.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: oldSecret1Name, Namespace: namespace}}).Return(params.staleSecretDeletionError)
										kubernetesClientSeed.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: oldSecret2Name, Namespace: namespace}})
									}
								}
							}
						}
					}
				}

				matcherFn(botanist.DeployManagedResourceForCloudConfigExecutor(ctx))
			},

			Entry("should fail because the bootstrap token cannot be computed",
				tableTestParams{
					bootstrapTokenSecretReadError: fakeErr,
				},
				func(err error) {
					Expect(err).To(MatchError(ContainSubstring("error computing bootstrap token for shoot cloud config")))
				},
			),

			Entry("should fail because the images cannot be found",
				tableTestParams{
					imageVector: nil,
				},
				func(err error) {
					Expect(err).To(MatchError(ContainSubstring("could not find image")))
				},
			),

			Entry("should fail because the operating system config maps for a worker pool are not available",
				tableTestParams{
					imageVector:                           imageVec,
					workerNameToOperatingSystemConfigMaps: nil,
				},
				func(err error) {
					Expect(err).To(MatchError(ContainSubstring("did not find osc data for worker pool")))
				},
			),

			Entry("should fail because the executor script generation fails",
				tableTestParams{
					imageVector:                           imageVec,
					workerNameToOperatingSystemConfigMaps: workerNameToOperatingSystemConfigMaps,
					executorScriptFnError:                 fakeErr,
				},
				func(err error) {
					Expect(err).To(MatchError(fakeErr))
				},
			),

			Entry("should fail because the downloader RBAC resources generation fails",
				tableTestParams{
					imageVector:                            imageVec,
					workerNameToOperatingSystemConfigMaps:  workerNameToOperatingSystemConfigMaps,
					downloaderGenerateRBACResourcesFnError: fakeErr,
				},
				func(err error) {
					Expect(err).To(MatchError(fakeErr))
				},
			),

			Entry("should fail because the managed resource secret reconciliation fails",
				tableTestParams{
					imageVector:                              imageVec,
					workerNameToOperatingSystemConfigMaps:    workerNameToOperatingSystemConfigMaps,
					managedResourceSecretReconciliationError: fakeErr,
				},
				func(err error) {
					Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
					Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
				},
			),

			Entry("should fail because the managed resource reconciliation fails",
				tableTestParams{
					imageVector:                           imageVec,
					workerNameToOperatingSystemConfigMaps: workerNameToOperatingSystemConfigMaps,
					managedResourceReadError:              fakeErr,
				},
				func(err error) {
					Expect(err).To(MatchError(fakeErr))
				},
			),

			Entry("should fail because the stale secret listing fails",
				tableTestParams{
					imageVector:                           imageVec,
					workerNameToOperatingSystemConfigMaps: workerNameToOperatingSystemConfigMaps,
					staleSecretListingError:               fakeErr,
				},
				func(err error) {
					Expect(err).To(MatchError(fakeErr))
				},
			),

			Entry("should fail because the stale secret deletion fails",
				tableTestParams{
					imageVector:                           imageVec,
					workerNameToOperatingSystemConfigMaps: workerNameToOperatingSystemConfigMaps,
					staleSecretDeletionError:              fakeErr,
				},
				func(err error) {
					Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
					Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
				},
			),

			Entry("should successfully compute the resources",
				tableTestParams{
					imageVector:                           imageVec,
					workerNameToOperatingSystemConfigMaps: workerNameToOperatingSystemConfigMaps,
				},
				func(err error) {
					Expect(err).To(Succeed())
				},
			),
		)
	})
})
