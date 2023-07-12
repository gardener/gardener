// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("operatingsystemconfig", func() {
	var (
		ctrl                  *gomock.Controller
		operatingSystemConfig *mockoperatingsystemconfig.MockInterface

		fakeClient client.Client
		sm         secretsmanager.Interface

		botanist *Botanist

		ctx        = context.TODO()
		namespace  = "namespace"
		fakeErr    = fmt.Errorf("fake")
		shootState = &gardencorev1beta1.ShootState{}

		cloudConfigExecutionMaxDelaySeconds = 500
		apiServerAddress                    = "1.2.3.4"
		caCloudProfile                      = "ca-cloud-profile"
		shootDomain                         = "shoot.domain.com"
		kubernetesVersion                   = "1.2.3"
		ingressDomain                       = "seed-test.ingress.domain.com"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		operatingSystemConfig = mockoperatingsystemconfig.NewMockInterface(ctrl)

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)

		By("Create secrets managed outside of this function for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ssh-keypair", Namespace: namespace}})).To(Succeed())

		botanist = &Botanist{
			Operation: &operation.Operation{
				APIServerAddress: apiServerAddress,
				SecretsManager:   sm,
				Shoot: &shootpkg.Shoot{
					CloudProfile: &gardencorev1beta1.CloudProfile{},
					Components: &shootpkg.Components{
						Extensions: &shootpkg.Extensions{
							OperatingSystemConfig: operatingSystemConfig,
						},
					},
					InternalClusterDomain:               shootDomain,
					Purpose:                             "development",
					CloudConfigExecutionMaxDelaySeconds: cloudConfigExecutionMaxDelaySeconds,
				},
				Seed: &seedpkg.Seed{},
			},
		}
		botanist.Shoot.SetShootState(shootState)
		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Spec: gardencorev1beta1.SeedSpec{
				Ingress: &gardencorev1beta1.Ingress{
					Domain: ingressDomain,
				},
			},
		})
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{Name: "foo"},
					},
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: "shoot--garden-testing",
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployOperatingSystemConfig", func() {
		Context("deploy", func() {
			BeforeEach(func() {
				operatingSystemConfig.EXPECT().SetAPIServerURL(fmt.Sprintf("https://api.%s", shootDomain))
				operatingSystemConfig.EXPECT().SetSSHPublicKeys(gomock.AssignableToTypeOf([]string{}))
			})

			It("should deploy successfully (only CloudProfile CA)", func() {
				botanist.Shoot.CloudProfile.Spec.CABundle = &caCloudProfile
				operatingSystemConfig.EXPECT().SetCABundle(&caCloudProfile)

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should deploy successfully shoot logging components with non testing purpose", func() {
				botanist.Shoot.Purpose = "development"
				botanist.Config = &config.GardenletConfiguration{
					Logging: &config.Logging{
						Enabled: pointer.Bool(true),
						ShootNodeLogging: &config.ShootNodeLogging{
							ShootPurposes: []gardencore.ShootPurpose{"evaluation", "development"},
						},
					},
				}
				operatingSystemConfig.EXPECT().SetCABundle(nil)

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				operatingSystemConfig.EXPECT().SetCABundle(nil)

				operatingSystemConfig.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			BeforeEach(func() {
				operatingSystemConfig.EXPECT().SetAPIServerURL(fmt.Sprintf("https://api.%s", shootDomain))
				operatingSystemConfig.EXPECT().SetSSHPublicKeys(gomock.AssignableToTypeOf([]string{}))

				shoot := botanist.Shoot.GetInfo()
				shoot.Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				}
				botanist.Shoot.SetInfo(shoot)

				operatingSystemConfig.EXPECT().SetCABundle(nil)
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
			kubernetesInterfaceSeed  *kubernetesmock.MockInterface
			kubernetesClientSeed     *mockclient.MockClient
			kubernetesInterfaceShoot *kubernetesmock.MockInterface
			kubernetesClientShoot    *mockclient.MockClient

			namespace      = "shoot--foo--bar"
			hyperkubeImage = &imagevector.ImageSource{Name: "hyperkube", Tag: pointer.String("v")}
			imageVec       = imagevector.ImageVector{hyperkubeImage}

			worker1Name            = "worker1"
			worker1OriginalContent = "w1content"
			worker1OriginalCommand = "/foo"
			worker1OriginalUnits   = []string{"w1u1", "w1u2"}
			worker1OriginalFiles   = []string{"w1f1", "w1f2"}
			worker1Key             = operatingsystemconfig.Key(worker1Name, semver.MustParse(kubernetesVersion), nil)

			worker2Name                  = "worker2"
			worker2OriginalContent       = "w2content"
			worker2OriginalCommand       = "/bar"
			worker2OriginalUnits         = []string{"w2u2", "w2u2", "w2u3"}
			worker2OriginalFiles         = []string{"w2f2", "w2f2", "w2f3"}
			worker2KubernetesVersion     = "4.5.6"
			worker2Key                   = operatingsystemconfig.Key(worker2Name, semver.MustParse(worker2KubernetesVersion), nil)
			worker2KubeletDataVolumeName = "vol"

			workerNameToOperatingSystemConfigMaps = map[string]*operatingsystemconfig.OperatingSystemConfigs{
				worker1Name: {
					Original: operatingsystemconfig.Data{
						Content: worker1OriginalContent,
						Command: &worker1OriginalCommand,
						Units:   worker1OriginalUnits,
						Files:   worker1OriginalFiles,
					},
				},
				worker2Name: {
					Original: operatingsystemconfig.Data{
						Content: worker2OriginalContent,
						Command: &worker2OriginalCommand,
						Units:   worker2OriginalUnits,
						Files:   worker2OriginalFiles,
					},
				},
			}

			oldSecret1Name = "old-secret-1"
			oldSecret2Name = "old-secret-2"
		)

		BeforeEach(func() {
			kubernetesInterfaceSeed = kubernetesmock.NewMockInterface(ctrl)
			kubernetesClientSeed = mockclient.NewMockClient(ctrl)
			botanist.SeedClientSet = kubernetesInterfaceSeed

			kubernetesInterfaceShoot = kubernetesmock.NewMockInterface(ctrl)
			kubernetesClientShoot = mockclient.NewMockClient(ctrl)
			botanist.ShootClientSet = kubernetesInterfaceShoot

			botanist.Shoot.SeedNamespace = namespace
			botanist.Shoot.KubernetesVersion = semver.MustParse(kubernetesVersion)
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
								Kubernetes: &gardencorev1beta1.WorkerKubernetes{
									Version: &worker2KubernetesVersion,
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
				ExecutorScriptFn = func(cloudConfigUserData []byte, cloudConfigExecutionMaxDelaySeconds int, hyperkubeImage *imagevector.Image, kubernetesVersion string, kubeletDataVolume *gardencorev1beta1.DataVolume, reloadConfigCommand string, units, files []string) ([]byte, error) {
					return []byte(fmt.Sprintf("%s_%d_%s_%s_%s_%s_%s_%s", cloudConfigUserData, cloudConfigExecutionMaxDelaySeconds, hyperkubeImage.String(), kubernetesVersion, kubeletDataVolume, reloadConfigCommand, units, files)), params.executorScriptFnError
				}

				// operating system config maps retrieval for the worker pools
				operatingSystemConfig.EXPECT().WorkerNameToOperatingSystemConfigsMap().Return(params.workerNameToOperatingSystemConfigMaps)

				// image vector for retrieval of required images
				botanist.ImageVector = params.imageVector

				if params.imageVector != nil {
					if params.downloaderGenerateRBACResourcesFnError == nil &&
						params.executorScriptFnError == nil &&
						params.workerNameToOperatingSystemConfigMaps != nil {

						// managed resource secret reconciliation for executor scripts for worker pools
						// worker pool 1
						worker1ExecutorScript, _ := ExecutorScriptFn([]byte(worker1OriginalContent), cloudConfigExecutionMaxDelaySeconds, hyperkubeImage.ToImage(&kubernetesVersion), kubernetesVersion, nil, worker1OriginalCommand, worker1OriginalUnits, worker1OriginalFiles)
						kubernetesClientSeed.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "managedresource-shoot-cloud-config-execution-"+worker1Name), gomock.AssignableToTypeOf(&corev1.Secret{}))
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
						worker2ExecutorScript, _ := ExecutorScriptFn([]byte(worker2OriginalContent), cloudConfigExecutionMaxDelaySeconds, hyperkubeImage.ToImage(&worker2KubernetesVersion), worker2KubernetesVersion, &gardencorev1beta1.DataVolume{Name: worker2KubeletDataVolumeName}, worker2OriginalCommand, worker2OriginalUnits, worker2OriginalFiles)
						kubernetesClientSeed.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "managedresource-shoot-cloud-config-execution-"+worker2Name), gomock.AssignableToTypeOf(&corev1.Secret{}))
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
						kubernetesClientSeed.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "managedresource-shoot-cloud-config-rbac"), gomock.AssignableToTypeOf(&corev1.Secret{}))
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
							kubernetesClientSeed.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "shoot-cloud-config-execution"), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(params.managedResourceReadError)

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

				matcherFn(botanist.DeployManagedResourceForCloudConfigExecutor(ctx))
			},

			Entry("should fail because the images cannot be found",
				tableTestParams{
					imageVector:                           nil,
					workerNameToOperatingSystemConfigMaps: workerNameToOperatingSystemConfigMaps,
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
