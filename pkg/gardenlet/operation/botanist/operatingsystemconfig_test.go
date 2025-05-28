// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/Masterminds/semver/v3"
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
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

		ctx        = context.Background()
		namespace  = "namespace"
		fakeErr    = errors.New("fake")
		shootState = &gardencorev1beta1.ShootState{}

		apiServerAddress  = "1.2.3.4"
		caCloudProfile    = "ca-cloud-profile"
		caBundle          = "ca-bundle"
		shootDomain       = "shoot.domain.com"
		kubernetesVersion = "1.2.3"
		ingressDomain     = "seed-test.ingress.domain.com"
		coreDNS           = []string{"10.0.0.10", "2001:db8::10"}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		operatingSystemConfig = mockoperatingsystemconfig.NewMockInterface(ctrl)

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)

		By("Create secrets managed outside of this function for which secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}, Data: map[string][]byte{"bundle.crt": []byte(caBundle)}})).To(Succeed())
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
					InternalClusterDomain: shootDomain,
					Purpose:               "development",
					Networks: &shootpkg.Networks{
						CoreDNS: []net.IP{net.ParseIP(coreDNS[0]), net.ParseIP(coreDNS[1])},
					},
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
				Networking: &gardencorev1beta1.Networking{
					IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
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
				operatingSystemConfig.EXPECT().SetClusterDNSAddresses(coreDNS)
			})

			It("should deploy successfully (only CloudProfile CA)", func() {
				botanist.Shoot.CloudProfile.Spec.CABundle = &caCloudProfile
				operatingSystemConfig.EXPECT().SetCABundle(fmt.Sprintf("%s\n%s", caCloudProfile, caBundle))

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should deploy successfully shoot logging components with non testing purpose", func() {
				botanist.Shoot.Purpose = "development"
				botanist.Config = &gardenletconfigv1alpha1.GardenletConfiguration{
					Logging: &gardenletconfigv1alpha1.Logging{
						Enabled: ptr.To(true),
						ShootNodeLogging: &gardenletconfigv1alpha1.ShootNodeLogging{
							ShootPurposes: []gardencorev1beta1.ShootPurpose{"evaluation", "development"},
						},
					},
				}
				operatingSystemConfig.EXPECT().SetCABundle(caBundle)

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should deploy successfully with ipFamily IPv6", func() {

				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{
								{Name: "foo"},
							},
						},
						Networking: &gardencorev1beta1.Networking{
							IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6},
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: "shoot--garden-testing",
					},
				})
				botanist.Shoot.Purpose = "development"
				operatingSystemConfig.EXPECT().SetCABundle(caBundle)

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				operatingSystemConfig.EXPECT().SetCABundle(caBundle)

				operatingSystemConfig.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			BeforeEach(func() {
				operatingSystemConfig.EXPECT().SetAPIServerURL(fmt.Sprintf("https://api.%s", shootDomain))
				operatingSystemConfig.EXPECT().SetSSHPublicKeys(gomock.AssignableToTypeOf([]string{}))
				operatingSystemConfig.EXPECT().SetClusterDNSAddresses(coreDNS)

				shoot := botanist.Shoot.GetInfo()
				shoot.Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				}
				botanist.Shoot.SetInfo(shoot)

				operatingSystemConfig.EXPECT().SetCABundle(caBundle)
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

			worker1Name = "worker1"
			worker1Key  = operatingsystemconfig.KeyV1(worker1Name, semver.MustParse(kubernetesVersion), nil)

			worker2Name                  = "worker2"
			worker2KubernetesVersion     = "4.5.6"
			worker2Key                   = operatingsystemconfig.KeyV1(worker2Name, semver.MustParse(worker2KubernetesVersion), nil)
			worker2KubeletDataVolumeName = "vol"

			workerNameToOperatingSystemConfigMaps = map[string]*operatingsystemconfig.OperatingSystemConfigs{
				worker1Name: {
					Original: operatingsystemconfig.Data{
						GardenerNodeAgentSecretName: worker1Key,
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
						GardenerNodeAgentSecretName: worker2Key,
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
			botanist.Shoot.ControlPlaneNamespace = namespace
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

		Describe("#DeployManagedResourceForGardenerNodeAgent", func() {
			BeforeEach(func() {
				botanist.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
			})

			It("should fail because the operating system config maps for a worker pool are not available", func() {
				operatingSystemConfig.EXPECT().WorkerPoolNameToOperatingSystemConfigsMap().Return(nil)

				Expect(botanist.DeployManagedResourceForGardenerNodeAgent(ctx)).To(MatchError(ContainSubstring("did not find osc data for worker pool")))
			})

			When("operating system config maps are available", func() {
				BeforeEach(func() {
					operatingSystemConfig.EXPECT().WorkerPoolNameToOperatingSystemConfigsMap().Return(workerNameToOperatingSystemConfigMaps)
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
					compressedOSCSecretWorker1Raw, err := test.BrotliCompression(expectedOSCSecretWorker1Raw)
					Expect(err).NotTo(HaveOccurred())

					expectedMRSecretWorker1 := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "managedresource-shoot-gardener-node-agent-" + worker1Name,
							Namespace:       namespace,
							Labels:          map[string]string{"managed-resource": "shoot-gardener-node-agent"},
							ResourceVersion: "1",
						},
						Type: corev1.SecretTypeOpaque,
						Data: map[string][]byte{"data.yaml.br": compressedOSCSecretWorker1Raw},
					}
					utilruntime.Must(kubernetesutils.MakeUnique(expectedMRSecretWorker1))

					expectedOSCSecretWorker2, err := NodeAgentOSCSecretFn(ctx, fakeClient, workerNameToOperatingSystemConfigMaps[worker2Name].Original.Object, worker2Key, worker2Name)
					Expect(err).NotTo(HaveOccurred())
					expectedOSCSecretWorker2Raw, err := runtime.Encode(codec, expectedOSCSecretWorker2)
					Expect(err).NotTo(HaveOccurred())
					compressedOSCSecretWorker2Raw, err := test.BrotliCompression(expectedOSCSecretWorker2Raw)
					Expect(err).NotTo(HaveOccurred())

					expectedMRSecretWorker2 := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "managedresource-shoot-gardener-node-agent-" + worker2Name,
							Namespace:       namespace,
							Labels:          map[string]string{"managed-resource": "shoot-gardener-node-agent"},
							ResourceVersion: "1",
						},
						Type: corev1.SecretTypeOpaque,
						Data: map[string][]byte{"data.yaml.br": compressedOSCSecretWorker2Raw},
					}
					utilruntime.Must(kubernetesutils.MakeUnique(expectedMRSecretWorker2))

					nodeAgentRBACResourcesData, err := NodeAgentRBACResourcesDataFn([]string{expectedOSCSecretWorker1.Name, expectedOSCSecretWorker2.Name})
					Expect(err).NotTo(HaveOccurred())
					expectedMRSecretRBAC := &corev1.Secret{
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
