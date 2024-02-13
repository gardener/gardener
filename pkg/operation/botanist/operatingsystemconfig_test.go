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

	"github.com/Masterminds/semver/v3"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/mock"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
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
						Enabled: ptr.To(true),
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

	Context("Operating System Config secrets", func() {
		var (
			namespace = "shoot--foo--bar"

			worker1Name            = "worker1"
			worker1OriginalContent = "w1content"
			worker1OriginalCommand = "/foo"
			worker1OriginalUnits   = []string{"w1u1", "w1u2"}
			worker1OriginalFiles   = []string{"w1f1", "w1f2"}
			worker1Key             string

			worker2Name                  = "worker2"
			worker2OriginalContent       = "w2content"
			worker2OriginalCommand       = "/bar"
			worker2OriginalUnits         = []string{"w2u2", "w2u2", "w2u3"}
			worker2OriginalFiles         = []string{"w2f2", "w2f2", "w2f3"}
			worker2KubernetesVersion     = "4.5.6"
			worker2Key                   string
			worker2KubeletDataVolumeName = "vol"

			workerNameToOperatingSystemConfigMaps = map[string]*operatingsystemconfig.OperatingSystemConfigs{
				worker1Name: {
					Original: operatingsystemconfig.Data{
						Content: worker1OriginalContent,
						Command: &worker1OriginalCommand,
						Units:   worker1OriginalUnits,
						Files:   worker1OriginalFiles,
						Object: &extensionsv1alpha1.OperatingSystemConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: worker1Name + "-original",
							},
							Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
								Units: []extensionsv1alpha1.Unit{{Name: "w1u1"}, {Name: "w1u2"}},
								Files: []extensionsv1alpha1.File{{Path: "w1f1"}, {Path: "w1f2"}},
							},
						},
					},
				},
				worker2Name: {
					Original: operatingsystemconfig.Data{
						Content: worker2OriginalContent,
						Command: &worker2OriginalCommand,
						Units:   worker2OriginalUnits,
						Files:   worker2OriginalFiles,
						Object: &extensionsv1alpha1.OperatingSystemConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: worker2Name + "-original",
							},
							Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
								Units: []extensionsv1alpha1.Unit{{Name: "w2u1"}, {Name: "w2u2"}},
								Files: []extensionsv1alpha1.File{{Path: "w2f1"}, {Path: "w2f2"}},
							},
						},
					},
				},
			}

			oldSecret1Name = "old-secret-1"
			oldSecret2Name = "old-secret-2"
		)

		JustBeforeEach(func() {
			worker1Key = operatingsystemconfig.Key(worker1Name, semver.MustParse(kubernetesVersion), nil)
			worker2Key = operatingsystemconfig.Key(worker2Name, semver.MustParse(worker2KubernetesVersion), nil)

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
		})

		Describe("#DeployManagedResourceForCloudConfigExecutor", func() {
			var (
				kubernetesInterfaceSeed  *kubernetesmock.MockInterface
				kubernetesClientSeed     *mockclient.MockClient
				kubernetesInterfaceShoot *kubernetesmock.MockInterface
				kubernetesClientShoot    *mockclient.MockClient

				hyperkubeImage = &imagevector.ImageSource{Name: "hyperkube", Repository: "europe-docker.pkg.dev/gardener-project/releases/hyperkube"}
			)

			BeforeEach(func() {
				kubernetesInterfaceSeed = kubernetesmock.NewMockInterface(ctrl)
				kubernetesClientSeed = mockclient.NewMockClient(ctrl)
				botanist.SeedClientSet = kubernetesInterfaceSeed

				kubernetesInterfaceShoot = kubernetesmock.NewMockInterface(ctrl)
				kubernetesClientShoot = mockclient.NewMockClient(ctrl)
				botanist.ShootClientSet = kubernetesInterfaceShoot

				kubernetesInterfaceShoot.EXPECT().Client().Return(kubernetesClientShoot).AnyTimes()
				kubernetesInterfaceSeed.EXPECT().Client().Return(kubernetesClientSeed).AnyTimes()

				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, false))
			})

			type tableTestParams struct {
				downloaderGenerateRBACResourcesFnError error
				executorScriptFnError                  error

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

					if params.downloaderGenerateRBACResourcesFnError == nil &&
						params.executorScriptFnError == nil &&
						params.workerNameToOperatingSystemConfigMaps != nil {

						// managed resource secret reconciliation for executor scripts for worker pools
						// worker pool 1
						worker1ExecutorScript, _ := ExecutorScriptFn([]byte(worker1OriginalContent), cloudConfigExecutionMaxDelaySeconds, hyperkubeImage.ToImage(&kubernetesVersion), kubernetesVersion, nil, worker1OriginalCommand, worker1OriginalUnits, worker1OriginalFiles)
						mrSecretPool1 := &corev1.Secret{
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
`)}}

						utilruntime.Must(kubernetesutils.MakeUnique(mrSecretPool1))
						kubernetesClientSeed.EXPECT().Get(ctx, client.ObjectKeyFromObject(mrSecretPool1), gomock.AssignableToTypeOf(&corev1.Secret{})).MaxTimes(2)
						kubernetesClientSeed.EXPECT().Update(ctx, mrSecretPool1)
						kubernetesClientSeed.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).MaxTimes(3)

						// worker pool 2
						worker2ExecutorScript, _ := ExecutorScriptFn([]byte(worker2OriginalContent), cloudConfigExecutionMaxDelaySeconds, hyperkubeImage.ToImage(&worker2KubernetesVersion), worker2KubernetesVersion, &gardencorev1beta1.DataVolume{Name: worker2KubeletDataVolumeName}, worker2OriginalCommand, worker2OriginalUnits, worker2OriginalFiles)
						mrSecretPool2 := &corev1.Secret{
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
`)}}

						utilruntime.Must(kubernetesutils.MakeUnique(mrSecretPool2))
						kubernetesClientSeed.EXPECT().Get(ctx, client.ObjectKeyFromObject(mrSecretPool2), gomock.AssignableToTypeOf(&corev1.Secret{})).MaxTimes(2)
						kubernetesClientSeed.EXPECT().Update(ctx, mrSecretPool2)

						// managed resource secret reconciliation for RBAC resources
						downloaderRBACResourcesData, _ := DownloaderGenerateRBACResourcesDataFn([]string{worker1Key, worker2Key})
						mrRBACSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "managedresource-shoot-cloud-config-rbac",
								Namespace: namespace,
								Labels:    map[string]string{"managed-resource": "shoot-cloud-config-execution"},
							},
							Type: corev1.SecretTypeOpaque,
							Data: downloaderRBACResourcesData,
						}

						utilruntime.Must(kubernetesutils.MakeUnique(mrRBACSecret))
						kubernetesClientSeed.EXPECT().Get(ctx, client.ObjectKeyFromObject(mrRBACSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).MaxTimes(2)
						kubernetesClientSeed.EXPECT().Update(ctx, mrRBACSecret).Return(params.managedResourceSecretReconciliationError)

						if params.managedResourceSecretReconciliationError == nil {
							// managed resource reconciliation
							kubernetesClientSeed.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "shoot-cloud-config-execution"), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(params.managedResourceReadError)

							if params.managedResourceReadError == nil {
								kubernetesClientSeed.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, obj *resourcesv1alpha1.ManagedResource, _ ...client.UpdateOption) error {
									Expect(obj.ObjectMeta).To(Equal(metav1.ObjectMeta{
										Name:      "shoot-cloud-config-execution",
										Namespace: namespace,
										Labels:    map[string]string{"origin": "gardener"},
										Annotations: map[string]string{
											"reference.resources.gardener.cloud/secret-99da0a78": "managedresource-shoot-cloud-config-execution-worker1-4ba77085",
											"reference.resources.gardener.cloud/secret-34039e5e": "managedresource-shoot-cloud-config-execution-worker2-4448afb0",
											"reference.resources.gardener.cloud/secret-db6befd8": "managedresource-shoot-cloud-config-rbac-94106240",
										},
									}))
									Expect(obj.Spec.SecretRefs).To(ConsistOf(
										corev1.LocalObjectReference{Name: "managedresource-shoot-cloud-config-execution-" + worker1Name + "-4ba77085"},
										corev1.LocalObjectReference{Name: "managedresource-shoot-cloud-config-execution-" + worker2Name + "-4448afb0"},
										corev1.LocalObjectReference{Name: "managedresource-shoot-cloud-config-rbac-94106240"},
									))
									Expect(obj.Spec.InjectLabels).To(Equal(map[string]string{"shoot.gardener.cloud/no-cleanup": "true"}))
									Expect(obj.Spec.KeepObjects).To(Equal(ptr.To(false)))
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

					matcherFn(botanist.DeployManagedResourceForCloudConfigExecutor(ctx))
				},

				Entry("should fail because the operating system config maps for a worker pool are not available",
					tableTestParams{
						workerNameToOperatingSystemConfigMaps: nil,
					},
					func(err error) {
						Expect(err).To(MatchError(ContainSubstring("did not find osc data for worker pool")))
					},
				),

				Entry("should fail because the executor script generation fails",
					tableTestParams{
						workerNameToOperatingSystemConfigMaps: workerNameToOperatingSystemConfigMaps,
						executorScriptFnError:                 fakeErr,
					},
					func(err error) {
						Expect(err).To(MatchError(fakeErr))
					},
				),

				Entry("should fail because the downloader RBAC resources generation fails",
					tableTestParams{
						workerNameToOperatingSystemConfigMaps:  workerNameToOperatingSystemConfigMaps,
						downloaderGenerateRBACResourcesFnError: fakeErr,
					},
					func(err error) {
						Expect(err).To(MatchError(fakeErr))
					},
				),

				Entry("should fail because the managed resource secret reconciliation fails",
					tableTestParams{
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
						workerNameToOperatingSystemConfigMaps: workerNameToOperatingSystemConfigMaps,
						managedResourceReadError:              fakeErr,
					},
					func(err error) {
						Expect(err).To(MatchError(fakeErr))
					},
				),

				Entry("should fail because the stale secret listing fails",
					tableTestParams{
						workerNameToOperatingSystemConfigMaps: workerNameToOperatingSystemConfigMaps,
						staleSecretListingError:               fakeErr,
					},
					func(err error) {
						Expect(err).To(MatchError(fakeErr))
					},
				),

				Entry("should fail because the stale secret deletion fails",
					tableTestParams{
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
						workerNameToOperatingSystemConfigMaps: workerNameToOperatingSystemConfigMaps,
					},
					func(err error) {
						Expect(err).To(Succeed())
					},
				),
			)
		})

		Describe("#DeployManagedResourceForGardenerNodeAgent", func() {
			BeforeEach(func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, true))
				botanist.SeedClientSet = kubernetesfake.NewClientSetBuilder().WithClient(fakeClient).Build()
			})

			It("should fail because the operating system config maps for a worker pool are not available", func() {
				operatingSystemConfig.EXPECT().WorkerNameToOperatingSystemConfigsMap().Return(nil)

				Expect(botanist.DeployManagedResourceForGardenerNodeAgent(ctx)).To(MatchError(ContainSubstring("did not find osc data for worker pool")))
			})

			When("operating system config maps are available", func() {
				BeforeEach(func() {
					operatingSystemConfig.EXPECT().WorkerNameToOperatingSystemConfigsMap().Return(workerNameToOperatingSystemConfigMaps)
				})

				It("should fail because the secret data generation function fails", func() {
					DeferCleanup(test.WithVar(&NodeAgentOSCSecretFn, func(context.Context, client.Client, *extensionsv1alpha1.OperatingSystemConfig, string, string) (*corev1.Secret, error) {
						return nil, fakeErr
					}))

					Expect(botanist.DeployManagedResourceForGardenerNodeAgent(ctx)).To(MatchError(fakeErr))
				})

				It("should fail because the RBAC resources data generation function fails", func() {
					DeferCleanup(test.WithVar(&NodeAgentRBACResourcesDataFn, func([]string) (map[string][]byte, error) {
						return nil, fakeErr
					}))

					Expect(botanist.DeployManagedResourceForGardenerNodeAgent(ctx)).To(MatchError(fakeErr))
				})

				It("should successfully generate the resources and cleanup stale resources", func() {
					var (
						versions = schema.GroupVersions([]schema.GroupVersion{corev1.SchemeGroupVersion})
						codec    = kubernetes.ShootCodec.CodecForVersions(kubernetes.ShootSerializer, kubernetes.ShootSerializer, versions, versions)

						oldSecret1 = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: oldSecret1Name, Namespace: namespace, Labels: map[string]string{"managed-resource": "shoot-gardener-node-agent"}}}
						oldSecret2 = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: oldSecret2Name, Namespace: namespace, Labels: map[string]string{"managed-resource": "shoot-gardener-node-agent"}}}
					)

					By("Create old ManagedResource secrets")
					Expect(fakeClient.Create(ctx, oldSecret1)).To(Succeed())
					Expect(fakeClient.Create(ctx, oldSecret2)).To(Succeed())

					By("Execute DeployManagedResourceForGardenerNodeAgent function")
					Expect(botanist.DeployManagedResourceForGardenerNodeAgent(ctx)).To(Succeed())

					expectedOSCSecretWorker1, err := NodeAgentOSCSecretFn(ctx, fakeClient, workerNameToOperatingSystemConfigMaps[worker1Name].Original.Object, worker1Key, worker1Name)
					Expect(err).NotTo(HaveOccurred())
					expectedOSCSecretWorker1Raw, err := runtime.Encode(codec, expectedOSCSecretWorker1)
					Expect(err).NotTo(HaveOccurred())
					expectedMRSecretWorker1 := &corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            "managedresource-shoot-gardener-node-agent-" + worker1Name,
							Namespace:       namespace,
							Labels:          map[string]string{"managed-resource": "shoot-gardener-node-agent"},
							ResourceVersion: "1",
						},
						Type: corev1.SecretTypeOpaque,
						Data: map[string][]byte{"secret__kube-system__" + worker1Key + ".yaml": expectedOSCSecretWorker1Raw},
					}
					utilruntime.Must(kubernetesutils.MakeUnique(expectedMRSecretWorker1))

					expectedOSCSecretWorker2, err := NodeAgentOSCSecretFn(ctx, fakeClient, workerNameToOperatingSystemConfigMaps[worker2Name].Original.Object, worker2Key, worker2Name)
					Expect(err).NotTo(HaveOccurred())
					expectedOSCSecretWorker2Raw, err := runtime.Encode(codec, expectedOSCSecretWorker2)
					Expect(err).NotTo(HaveOccurred())
					expectedMRSecretWorker2 := &corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            "managedresource-shoot-gardener-node-agent-" + worker2Name,
							Namespace:       namespace,
							Labels:          map[string]string{"managed-resource": "shoot-gardener-node-agent"},
							ResourceVersion: "1",
						},
						Type: corev1.SecretTypeOpaque,
						Data: map[string][]byte{"secret__kube-system__" + worker2Key + ".yaml": expectedOSCSecretWorker2Raw},
					}
					utilruntime.Must(kubernetesutils.MakeUnique(expectedMRSecretWorker2))

					nodeAgentRBACResourcesData, err := NodeAgentRBACResourcesDataFn([]string{expectedOSCSecretWorker1.Name, expectedOSCSecretWorker2.Name})
					Expect(err).NotTo(HaveOccurred())
					expectedMRSecretRBAC := &corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            "managedresource-shoot-gardener-node-agent-rbac",
							Namespace:       namespace,
							Labels:          map[string]string{"managed-resource": "shoot-gardener-node-agent"},
							ResourceVersion: "1",
						},
						Type: corev1.SecretTypeOpaque,
						Data: nodeAgentRBACResourcesData,
					}
					utilruntime.Must(kubernetesutils.MakeUnique(expectedMRSecretRBAC))

					expectedManagedResource := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "shoot-gardener-node-agent",
							Namespace:       namespace,
							Labels:          map[string]string{"origin": "gardener"},
							ResourceVersion: "1",
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							SecretRefs: []corev1.LocalObjectReference{
								{Name: expectedMRSecretWorker1.Name},
								{Name: expectedMRSecretWorker2.Name},
								{Name: expectedMRSecretRBAC.Name},
							},
							InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
							KeepObjects:  ptr.To(false),
						},
					}
					utilruntime.Must(references.InjectAnnotations(expectedManagedResource))

					By("Assert expected creation of ManagedResource and related secrets")
					mrSecretWorker1 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: expectedMRSecretWorker1.Name, Namespace: expectedMRSecretWorker1.Namespace}}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mrSecretWorker1), mrSecretWorker1)).To(Succeed())
					Expect(mrSecretWorker1).To(Equal(expectedMRSecretWorker1))

					mrSecretWorker2 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: expectedMRSecretWorker2.Name, Namespace: expectedMRSecretWorker2.Namespace}}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mrSecretWorker2), mrSecretWorker2)).To(Succeed())
					Expect(mrSecretWorker2).To(Equal(expectedMRSecretWorker2))

					mrSecretRBAC := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: expectedMRSecretRBAC.Name, Namespace: expectedMRSecretRBAC.Namespace}}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mrSecretRBAC), mrSecretRBAC)).To(Succeed())
					Expect(mrSecretRBAC).To(Equal(expectedMRSecretRBAC))

					managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: expectedManagedResource.Name, Namespace: expectedManagedResource.Namespace}}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					Expect(managedResource.ObjectMeta).To(Equal(expectedManagedResource.ObjectMeta))
					Expect(managedResource.Spec.SecretRefs).To(ConsistOf(expectedManagedResource.Spec.SecretRefs))
					Expect(managedResource.Spec.InjectLabels).To(Equal(expectedManagedResource.Spec.InjectLabels))
					Expect(managedResource.Spec.KeepObjects).To(Equal(expectedManagedResource.Spec.KeepObjects))

					By("Assert expected deletion of no longer required ManagedResource secrets")
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(oldSecret1), oldSecret1)).To(BeNotFoundError())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(oldSecret2), oldSecret2)).To(BeNotFoundError())
				})
			})
		})
	})
})
