// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx       = context.TODO()
		nopLogger = logr.Discard()

		seedName       = "seed"
		sourceSeedName = "sourceSeed"
		seedLabels     = map[string]string{"foo": "bar"}
		seedObjectMeta = metav1.ObjectMeta{
			Name:   seedName,
			Labels: seedLabels,
		}

		alwaysPolicy         = gardencorev1beta1.ControllerDeploymentPolicyAlways
		alwaysIfShootsPolicy = gardencorev1beta1.ControllerDeploymentPolicyAlwaysExceptNoShoots
		onDemandPolicy       = gardencorev1beta1.ControllerDeploymentPolicyOnDemand
		now                  = metav1.Now()

		type1  = "type1"
		type2  = "type2"
		type3  = "type3"
		type4  = "type4"
		type5  = "type5"
		type6  = "type6"
		type8  = "type8"
		type9  = "type9"
		type10 = "type10"
		type11 = "type11"
		type12 = "type12"

		sourceType = "sourceType"

		backupBucket1 = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bb1",
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				SeedName: &seedName,
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type: type2,
				},
			},
		}
		backupBucket2 = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bb2",
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				SeedName: &seedName,
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type: type3,
				},
			},
		}
		sourceBackupBucket = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sourceBackupBucket",
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				SeedName: &sourceSeedName,
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type: sourceType,
				},
			},
		}

		backupBucketsMap = map[string]*gardencorev1beta1.BackupBucket{
			backupBucket1.Name:      backupBucket1,
			backupBucket2.Name:      backupBucket2,
			sourceBackupBucket.Name: sourceBackupBucket,
		}

		backupEntry2 = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "be2",
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				SeedName:   &seedName,
				BucketName: backupBucket1.Name,
			},
		}
		backupEntry3 = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "be3",
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				SeedName:   &seedName,
				BucketName: backupBucket1.Name,
			},
		}
		sourceBackupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sourceBackupEntry",
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				SeedName:   &seedName,
				BucketName: sourceBackupBucket.Name,
			},
		}
		backupEntryList = &gardencorev1beta1.BackupEntryList{
			Items: []gardencorev1beta1.BackupEntry{
				*backupEntry2,
				*backupEntry3,
				*sourceBackupEntry,
			},
		}

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Type: type11,
				},
				Backup: &gardencorev1beta1.Backup{
					Provider: type8,
				},
			},
		}

		shoot1 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Type: type1,
				},
			},
		}
		shoot2 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s2",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: &seedName,
				Provider: gardencorev1beta1.Provider{
					Type: type2,
					Workers: []gardencorev1beta1.Worker{
						{
							Machine: gardencorev1beta1.Machine{
								Image: &gardencorev1beta1.ShootMachineImage{
									Name: type5,
								},
							},
						},
					},
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To(type3),
				},
				Extensions: []gardencorev1beta1.Extension{
					{Type: type4},
				},
			},
		}
		shoot3 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s3",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: &seedName,
				Provider: gardencorev1beta1.Provider{
					Type: type6,
					Workers: []gardencorev1beta1.Worker{
						{
							CRI: &gardencorev1beta1.CRI{
								ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
									{Type: type12},
								},
							},
						},
					},
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To(type3),
				},
			},
		}
		shootList = []gardencorev1beta1.Shoot{
			*shoot1,
			*shoot2,
			*shoot3,
		}

		internalDomain = &gardenerutils.Domain{
			Provider: type9,
		}

		controllerDeployment = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fooDeployment",
			},
			Helm: &gardencorev1.HelmControllerDeployment{
				RawChart: []byte("foo"),
			},
		}

		controllerRegistration1 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr1",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: extensionsv1alpha1.BackupBucketResource,
						Type: type1,
					},
					{
						Kind:            extensionsv1alpha1.ExtensionResource,
						GloballyEnabled: ptr.To(true),
						Type:            type10,
					},
					{
						Kind:    extensionsv1alpha1.NetworkResource,
						Type:    type2,
						Primary: ptr.To(false),
					},
				},
			},
		}
		controllerRegistration2 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr2",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: extensionsv1alpha1.NetworkResource,
						Type: type2,
					},
					{
						Kind: extensionsv1alpha1.ContainerRuntimeResource,
						Type: type12,
					},
				},
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					Policy: &onDemandPolicy,
					DeploymentRefs: []gardencorev1beta1.DeploymentRef{
						{Name: controllerDeployment.Name},
					},
				},
			},
		}
		controllerRegistration3 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr3",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: extensionsv1alpha1.ControlPlaneResource,
						Type: type3,
					},
					{
						Kind: extensionsv1alpha1.InfrastructureResource,
						Type: type3,
					},
					{
						Kind: extensionsv1alpha1.WorkerResource,
						Type: type3,
					},
				},
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					Policy: &onDemandPolicy,
				},
			},
		}
		controllerRegistration4 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr4",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					Policy: &alwaysPolicy,
				},
			},
		}
		controllerRegistration5 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr5",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					Policy: &alwaysPolicy,
					SeedSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"bar": "foo",
						},
					},
				},
			},
		}
		controllerRegistration6 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr6",
			},
		}
		controllerRegistration7 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr7",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					Policy: &onDemandPolicy,
				},
			},
		}
		controllerRegistration8 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr8",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					Policy: &alwaysIfShootsPolicy,
				},
			},
		}
		controllerRegistrationList = &gardencorev1beta1.ControllerRegistrationList{
			Items: []gardencorev1beta1.ControllerRegistration{
				*controllerRegistration1,
				*controllerRegistration2,
				*controllerRegistration3,
				*controllerRegistration4,
				*controllerRegistration5,
				*controllerRegistration6,
				*controllerRegistration7,
				*controllerRegistration8,
			},
		}
		controllerRegistrations = map[string]controllerRegistration{
			controllerRegistration1.Name: {obj: controllerRegistration1},
			controllerRegistration2.Name: {obj: controllerRegistration2},
			controllerRegistration3.Name: {obj: controllerRegistration3},
			controllerRegistration4.Name: {obj: controllerRegistration4, deployAlways: true},
			controllerRegistration5.Name: {obj: controllerRegistration5, deployAlways: true},
			controllerRegistration6.Name: {obj: controllerRegistration6},
			controllerRegistration7.Name: {obj: controllerRegistration7},
			controllerRegistration8.Name: {obj: controllerRegistration8, deployAlwaysExceptNoShoots: true},
		}

		controllerInstallation1 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ci1",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: "another-seed",
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration1.Name,
				},
			},
		}
		controllerInstallation2 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ci2",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seedName,
				},
				DeploymentRef: &corev1.ObjectReference{
					Name: controllerDeployment.Name,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration2.Name,
				},
			},
		}
		controllerInstallation3 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ci3",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seedName,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration3.Name,
				},
			},
		}
		controllerInstallation4 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ci4",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seedName,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration4.Name,
				},
			},
		}
		controllerInstallation7 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ci7",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seedName,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration7.Name,
				},
			},
			Status: gardencorev1beta1.ControllerInstallationStatus{
				Conditions: []gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationRequired,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
			},
		}
		controllerInstallationList = &gardencorev1beta1.ControllerInstallationList{
			Items: []gardencorev1beta1.ControllerInstallation{
				*controllerInstallation1,
				*controllerInstallation2,
				*controllerInstallation3,
				*controllerInstallation4,
				*controllerInstallation7,
			},
		}
	)

	Describe("#computeKindTypesForBackupBuckets", func() {
		It("should return empty results for empty input", func() {
			kindTypes := computeKindTypesForBackupBuckets(map[string]*gardencorev1beta1.BackupBucket{}, seedName)

			Expect(kindTypes.Len()).To(BeZero())
		})

		It("should correctly compute the result", func() {
			kindTypes := computeKindTypesForBackupBuckets(backupBucketsMap, seedName)

			Expect(kindTypes).To(Equal(sets.New(
				extensionsv1alpha1.BackupBucketResource+"/"+backupBucket1.Spec.Provider.Type,
				extensionsv1alpha1.BackupBucketResource+"/"+backupBucket2.Spec.Provider.Type,
			)))
		})
	})

	Describe("#computeKindTypesForBackupEntries", func() {
		It("should return empty results for empty input", func() {
			kindTypes := computeKindTypesForBackupEntries(nopLogger, &gardencorev1beta1.BackupEntryList{}, nil)

			Expect(kindTypes.Len()).To(BeZero())
		})

		It("should correctly compute the result", func() {
			kindTypes := computeKindTypesForBackupEntries(nopLogger, backupEntryList, backupBucketsMap)

			Expect(kindTypes).To(Equal(sets.New(
				extensionsv1alpha1.BackupEntryResource+"/"+backupBucket1.Spec.Provider.Type,
				extensionsv1alpha1.BackupEntryResource+"/"+sourceBackupBucket.Spec.Provider.Type,
			)))
		})
	})

	Describe("#computeKindTypesForShoots", func() {
		var (
			ignoreCurrent goleak.Option
		)
		BeforeEach(func() {
			ignoreCurrent = goleak.IgnoreCurrent()
		})
		AfterEach(func() {
			goleak.VerifyNone(GinkgoT(), ignoreCurrent)
		})

		It("should correctly compute the result for a seed", func() {

			kindTypes := computeKindTypesForShoots(ctx, nopLogger, nil, shootList, seed, controllerRegistrationList, internalDomain, nil)

			Expect(kindTypes).To(Equal(sets.New(
				// seed types
				extensionsv1alpha1.BackupBucketResource+"/"+type8,
				extensionsv1alpha1.BackupEntryResource+"/"+type8,
				extensionsv1alpha1.ControlPlaneResource+"/"+type11,

				// shoot2 types
				extensionsv1alpha1.ControlPlaneResource+"/"+type2,
				extensionsv1alpha1.InfrastructureResource+"/"+type2,
				extensionsv1alpha1.WorkerResource+"/"+type2,
				extensionsv1alpha1.OperatingSystemConfigResource+"/"+type5,
				extensionsv1alpha1.NetworkResource+"/"+type3,
				extensionsv1alpha1.ExtensionResource+"/"+type4,

				// shoot3 types
				extensionsv1alpha1.ControlPlaneResource+"/"+type6,
				extensionsv1alpha1.InfrastructureResource+"/"+type6,
				extensionsv1alpha1.WorkerResource+"/"+type6,
				extensionsv1alpha1.ContainerRuntimeResource+"/"+type12,

				// internal domain + globally enabled extensions
				extensionsv1alpha1.ExtensionResource+"/"+type10,
				extensionsv1alpha1.DNSRecordResource+"/"+type9,
			)))
		})

		It("should correctly compute types for shoot that has the Seed`s name as status not spec", func() {
			shootList = []gardencorev1beta1.Shoot{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "s4",
					},
					Spec: gardencorev1beta1.ShootSpec{
						SeedName: ptr.To("anotherSeed"),
						Provider: gardencorev1beta1.Provider{
							Type: type2,
							Workers: []gardencorev1beta1.Worker{
								{
									Machine: gardencorev1beta1.Machine{
										Image: &gardencorev1beta1.ShootMachineImage{
											Name: type5,
										},
									},
								},
							},
						},
						Networking: &gardencorev1beta1.Networking{
							Type: ptr.To(type3),
						},
						Extensions: []gardencorev1beta1.Extension{
							{Type: type4},
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						SeedName: &seedName,
					},
				},
			}

			kindTypes := computeKindTypesForShoots(ctx, nopLogger, nil, shootList, seed, controllerRegistrationList, internalDomain, nil)

			Expect(kindTypes).To(Equal(sets.New(
				// seed types
				extensionsv1alpha1.BackupBucketResource+"/"+type8,
				extensionsv1alpha1.BackupEntryResource+"/"+type8,
				extensionsv1alpha1.ControlPlaneResource+"/"+type11,

				// shoot4 types
				extensionsv1alpha1.ControlPlaneResource+"/"+type2,
				extensionsv1alpha1.InfrastructureResource+"/"+type2,
				extensionsv1alpha1.WorkerResource+"/"+type2,
				extensionsv1alpha1.OperatingSystemConfigResource+"/"+type5,
				extensionsv1alpha1.NetworkResource+"/"+type3,
				extensionsv1alpha1.ExtensionResource+"/"+type4,

				// internal domain + globally enabled extensions
				extensionsv1alpha1.ExtensionResource+"/"+type10,
				extensionsv1alpha1.DNSRecordResource+"/"+type9,
			)))
		})
	})

	Describe("#computeKindTypesForSeed", func() {
		var providerType = "fake-provider-type"

		It("should add the DNSRecord extension", func() {
			seed := &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Type: type1,
					},
					DNS: gardencorev1beta1.SeedDNS{
						Provider: &gardencorev1beta1.SeedDNSProvider{
							Type: providerType,
						},
					},
				},
			}

			expected := sets.New(
				extensionsv1alpha1.DNSRecordResource+"/fake-provider-type",
				extensionsv1alpha1.ControlPlaneResource+"/type1",
				extensionsv1alpha1.InfrastructureResource+"/type1",
				extensionsv1alpha1.WorkerResource+"/type1",
			)
			actual := computeKindTypesForSeed(seed)
			Expect(actual).To(Equal(expected))
		})

		It("should not add an extension if Seed has a deletion timestamp", func() {
			deletionTimestamp := metav1.Now()
			seed := &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &deletionTimestamp,
				},
				Spec: gardencorev1beta1.SeedSpec{
					DNS: gardencorev1beta1.SeedDNS{
						Provider: &gardencorev1beta1.SeedDNSProvider{
							Type: providerType,
						},
					},
				},
			}

			expected := sets.New[string]()
			actual := computeKindTypesForSeed(seed)
			Expect(actual).To(Equal(expected))
		})

		It("should only add seed provider type extensions", func() {
			seed := &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Type: type1,
					},
				},
			}

			expected := sets.New(
				extensionsv1alpha1.ControlPlaneResource+"/type1",
				extensionsv1alpha1.InfrastructureResource+"/type1",
				extensionsv1alpha1.WorkerResource+"/type1",
			)
			actual := computeKindTypesForSeed(seed)
			Expect(actual).To(Equal(expected))
		})
	})

	Describe("#computeControllerRegistrationMaps", func() {
		It("should correctly compute the result", func() {
			registrations := computeControllerRegistrationMaps(controllerRegistrationList)

			Expect(registrations).To(Equal(controllerRegistrations))
		})
	})

	Describe("#computeWantedControllerRegistrationNames", func() {
		It("should correctly compute the result w/o error", func() {
			wantedKindTypeCombinations := sets.New(
				extensionsv1alpha1.NetworkResource+"/"+type2,
				extensionsv1alpha1.ControlPlaneResource+"/"+type3,
			)

			names, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, len(shootList), seedObjectMeta)

			Expect(names).To(Equal(sets.New(controllerRegistration1.Name, controllerRegistration2.Name, controllerRegistration3.Name, controllerRegistration4.Name, controllerRegistration7.Name, controllerRegistration8.Name)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not consider 'always-deploy-if-shoots' registrations when seed has no shoots", func() {
			wantedKindTypeCombinations := sets.New[string]()

			names, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, 0, seedObjectMeta)

			Expect(names).To(Equal(sets.New(controllerRegistration4.Name, controllerRegistration7.Name)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should consider 'always-deploy' registrations when seed has no shoots but no deletion timestamp", func() {
			wantedKindTypeCombinations := sets.New[string]()

			names, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, 0, seedObjectMeta)

			Expect(names).To(Equal(sets.New(controllerRegistration4.Name, controllerRegistration7.Name)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not consider 'always-deploy' registrations when seed has no shoots and deletion timestamp", func() {
			seedObjectMetaCopy := seedObjectMeta.DeepCopy()
			time := metav1.Time{}
			seedObjectMetaCopy.DeletionTimestamp = &time
			wantedKindTypeCombinations := sets.New[string]()

			names, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, 0, *seedObjectMetaCopy)

			Expect(names).To(Equal(sets.New(controllerRegistration7.Name)))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#computeRegistrationNameToInstallationNameMap", func() {
		It("should correctly compute the result w/o error", func() {
			regNameToInstallationName, err := computeRegistrationNameToInstallationMap(controllerInstallationList, controllerRegistrations, seedName)

			Expect(err).NotTo(HaveOccurred())
			Expect(regNameToInstallationName).To(Equal(map[string]*gardencorev1beta1.ControllerInstallation{
				controllerRegistration2.Name: controllerInstallation2,
				controllerRegistration3.Name: controllerInstallation3,
				controllerRegistration4.Name: controllerInstallation4,
				controllerRegistration7.Name: controllerInstallation7,
			}))
		})

		It("should fail to compute the result and return error", func() {
			regNameToInstallationName, err := computeRegistrationNameToInstallationMap(controllerInstallationList, map[string]controllerRegistration{}, seedName)

			Expect(err).To(HaveOccurred())
			Expect(regNameToInstallationName).To(BeNil())
		})
	})

	Context("deployment and deletion", func() {
		var (
			ctrl      *gomock.Controller
			k8sClient *mockclient.MockClient
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			k8sClient = mockclient.NewMockClient(ctrl)

			k8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: controllerDeployment.Name}, gomock.AssignableToTypeOf(&gardencorev1.ControllerDeployment{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *gardencorev1.ControllerDeployment, _ ...client.GetOption) error {
					*obj = *controllerDeployment
					return nil
				},
			).AnyTimes()

		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("#deployNeededInstallations", func() {
			It("should return an error when cannot get controller installation", func() {
				var (
					wantedControllerRegistrations  = sets.New(controllerRegistration2.Name)
					registrationNameToInstallation = map[string]*gardencorev1beta1.ControllerInstallation{
						controllerRegistration1.Name: controllerInstallation1,
						controllerRegistration2.Name: controllerInstallation2,
						controllerRegistration3.Name: controllerInstallation3,
					}
					fakeErr = errors.New("err")
				)

				k8sClient.EXPECT().Get(ctx, client.ObjectKey{Name: controllerInstallation2.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{})).Return(fakeErr)

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seed, wantedControllerRegistrations, controllerRegistrations, registrationNameToInstallation)

				Expect(err).To(Equal(fakeErr))
			})

			It("should return an error when needed controller installation is being deleted", func() {
				installation2 := controllerInstallation2.DeepCopy()
				installation2.DeletionTimestamp = &now
				var (
					wantedControllerRegistrations  = sets.New(controllerRegistration2.Name)
					registrationNameToInstallation = map[string]*gardencorev1beta1.ControllerInstallation{
						controllerRegistration1.Name: controllerInstallation1,
						controllerRegistration2.Name: installation2,
					}
				)

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seed, wantedControllerRegistrations, controllerRegistrations, registrationNameToInstallation)

				Expect(err).To(HaveOccurred())
			})

			It("should correctly deploy needed controller installations", func() {
				var (
					wantedControllerRegistrations  = sets.New(controllerRegistration2.Name, controllerRegistration3.Name, controllerRegistration4.Name)
					registrationNameToInstallation = map[string]*gardencorev1beta1.ControllerInstallation{
						controllerRegistration1.Name: controllerInstallation1,
						controllerRegistration2.Name: controllerInstallation2,
						controllerRegistration3.Name: controllerInstallation3,
						controllerRegistration4.Name: nil,
					}
				)

				installation2 := controllerInstallation2.DeepCopy()
				installation2.Labels = map[string]string{
					ControllerDeploymentHash: "deb30f197b882cd1",
					RegistrationSpecHash:     "61ca93a1782c5fa3",
					SeedSpecHash:             "8e09957b7d0d3c19",
				}

				installation3 := controllerInstallation3.DeepCopy()
				installation3.Labels = map[string]string{
					RegistrationSpecHash: "61ca93a1782c5fa3",
					SeedSpecHash:         "8e09957b7d0d3c19",
				}

				k8sClient.EXPECT().Get(ctx, client.ObjectKey{Name: controllerInstallation2.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))
				k8sClient.EXPECT().Patch(ctx, installation2, gomock.Any())

				k8sClient.EXPECT().Get(ctx, client.ObjectKey{Name: controllerInstallation3.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))
				k8sClient.EXPECT().Patch(ctx, installation3, gomock.Any())

				k8sClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seed, wantedControllerRegistrations, controllerRegistrations, registrationNameToInstallation)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should not skip the controller registration that is after one in deletion", func() {
				registration1 := controllerRegistration1.DeepCopy()
				registration1.DeletionTimestamp = &now
				var (
					wantedControllerRegistrations  = sets.New(registration1.Name, controllerRegistration2.Name)
					registrationNameToInstallation = map[string]*gardencorev1beta1.ControllerInstallation{
						registration1.Name:           controllerInstallation1,
						controllerRegistration2.Name: controllerInstallation2,
					}
					registrations = map[string]controllerRegistration{
						registration1.Name:           {obj: registration1, deployAlways: false},
						controllerRegistration2.Name: {obj: controllerRegistration2, deployAlways: false},
					}
				)

				installation2 := controllerInstallation2.DeepCopy()
				installation2.Labels = map[string]string{
					ControllerDeploymentHash: "deb30f197b882cd1",
					RegistrationSpecHash:     "61ca93a1782c5fa3",
					SeedSpecHash:             "8e09957b7d0d3c19",
				}

				k8sClient.EXPECT().Get(ctx, client.ObjectKey{Name: controllerInstallation2.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))
				k8sClient.EXPECT().Patch(ctx, installation2, gomock.Any())

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seed, wantedControllerRegistrations, registrations, registrationNameToInstallation)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should not create or update controller installation for controller registration in deletion", func() {
				registration1 := controllerRegistration1.DeepCopy()
				registration1.DeletionTimestamp = &now
				registration2 := controllerRegistration2.DeepCopy()
				registration2.DeletionTimestamp = &now
				var (
					wantedControllerRegistrations  = sets.New(registration1.Name, registration2.Name)
					registrationNameToInstallation = map[string]*gardencorev1beta1.ControllerInstallation{
						registration1.Name: controllerInstallation1,
						registration2.Name: nil,
					}
					registrations = map[string]controllerRegistration{
						registration1.Name: {obj: registration1, deployAlways: false},
						registration2.Name: {obj: registration2, deployAlways: false},
					}
				)

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seed, wantedControllerRegistrations, registrations, registrationNameToInstallation)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should deploy controller installation with correct security.gardener.cloud/pod-security-enforce annotation", func() {
				registration2 := controllerRegistration2.DeepCopy()
				registration2.Annotations = map[string]string{
					v1beta1constants.AnnotationPodSecurityEnforce: "baseline",
				}
				var (
					wantedControllerRegistrations  = sets.New(registration2.Name)
					registrationNameToInstallation = map[string]*gardencorev1beta1.ControllerInstallation{
						controllerRegistration1.Name: controllerInstallation1,
						registration2.Name:           controllerInstallation2,
					}
					registrations = map[string]controllerRegistration{
						controllerRegistration1.Name: {obj: controllerRegistration1, deployAlways: false},
						registration2.Name:           {obj: registration2, deployAlways: false},
					}
				)

				installation2 := controllerInstallation2.DeepCopy()
				installation2.Labels = map[string]string{
					ControllerDeploymentHash: "deb30f197b882cd1",
					RegistrationSpecHash:     "61ca93a1782c5fa3",
					SeedSpecHash:             "8e09957b7d0d3c19",
				}
				installation2.Annotations = map[string]string{
					v1beta1constants.AnnotationPodSecurityEnforce: "baseline",
				}

				k8sClient.EXPECT().Get(ctx, client.ObjectKey{Name: controllerInstallation2.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))
				k8sClient.EXPECT().Patch(ctx, installation2, gomock.Any())

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seed, wantedControllerRegistrations, registrations, registrationNameToInstallation)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should deploy controller installation without security.gardener.cloud/pod-security-enforce annotation when removed from controller registration", func() {
				registration2 := controllerRegistration2.DeepCopy()
				registration2.Annotations = map[string]string{
					v1beta1constants.AnnotationPodSecurityEnforce: "baseline",
				}
				var (
					wantedControllerRegistrations  = sets.New(registration2.Name)
					registrationNameToInstallation = map[string]*gardencorev1beta1.ControllerInstallation{
						controllerRegistration1.Name: controllerInstallation1,
						registration2.Name:           controllerInstallation2,
					}
					registrations = map[string]controllerRegistration{
						controllerRegistration1.Name: {obj: controllerRegistration1, deployAlways: false},
						registration2.Name:           {obj: registration2, deployAlways: false},
					}
				)

				installation2 := controllerInstallation2.DeepCopy()
				installation2.Labels = map[string]string{
					ControllerDeploymentHash: "deb30f197b882cd1",
					RegistrationSpecHash:     "61ca93a1782c5fa3",
					SeedSpecHash:             "8e09957b7d0d3c19",
				}
				installation2.Annotations = map[string]string{
					v1beta1constants.AnnotationPodSecurityEnforce: "baseline",
				}

				k8sClient.EXPECT().Get(ctx, client.ObjectKey{Name: controllerInstallation2.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))
				k8sClient.EXPECT().Patch(ctx, installation2, gomock.Any())

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seed, wantedControllerRegistrations, registrations, registrationNameToInstallation)

				Expect(err).NotTo(HaveOccurred())

				delete(registration2.Annotations, v1beta1constants.AnnotationPodSecurityEnforce)
				registrations[registration2.Name] = controllerRegistration{obj: registration2, deployAlways: false}
				installation2 = controllerInstallation2.DeepCopy()
				installation2.Labels = map[string]string{
					ControllerDeploymentHash: "deb30f197b882cd1",
					RegistrationSpecHash:     "61ca93a1782c5fa3",
					SeedSpecHash:             "8e09957b7d0d3c19",
				}

				k8sClient.EXPECT().Get(ctx, client.ObjectKey{Name: controllerInstallation2.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))
				k8sClient.EXPECT().Patch(ctx, installation2, gomock.Any())

				err = deployNeededInstallations(ctx, nopLogger, k8sClient, seed, wantedControllerRegistrations, registrations, registrationNameToInstallation)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("#deleteUnneededInstallations", func() {
			It("should return an error", func() {
				var (
					wantedControllerRegistrationNames = sets.New[string]()
					registrationNameToInstallation    = map[string]*gardencorev1beta1.ControllerInstallation{
						controllerRegistration1.Name: controllerInstallation1,
					}
					fakeErr = errors.New("err")
				)

				k8sClient.EXPECT().Delete(ctx, controllerInstallation1).Return(fakeErr)

				err := deleteUnneededInstallations(ctx, nopLogger, k8sClient, wantedControllerRegistrationNames, registrationNameToInstallation)

				Expect(err).To(Equal(fakeErr))
			})

			It("should correctly delete unneeded controller installations", func() {
				var (
					wantedControllerRegistrationNames = sets.New(controllerRegistration2.Name)
					registrationNameToInstallation    = map[string]*gardencorev1beta1.ControllerInstallation{
						controllerRegistration1.Name: controllerInstallation1,
						controllerRegistration2.Name: controllerInstallation2,
						controllerRegistration3.Name: controllerInstallation3,
					}
				)

				k8sClient.EXPECT().Delete(ctx, controllerInstallation1)
				k8sClient.EXPECT().Delete(ctx, controllerInstallation3)

				err := deleteUnneededInstallations(ctx, nopLogger, k8sClient, wantedControllerRegistrationNames, registrationNameToInstallation)

				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
