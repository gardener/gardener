// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/user"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityapi "github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardenv1betainformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	securityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	securityv1alpha1informers "github.com/gardener/gardener/pkg/client/security/informers/externalversions/security/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

var (
	rsaPrivateKey *rsa.PrivateKey
	seedUser      user.DefaultInfo
	nonSeedUser   user.DefaultInfo
)

var _ = BeforeSuite(func() {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 4096)
	Expect(err).ToNot(HaveOccurred())
	rsaPrivateKey = rsaKey
})

var _ = Describe("#TokenRequest", func() {
	BeforeEach(func() {
		seedUser = user.DefaultInfo{
			Name: v1beta1constants.SeedUserNamePrefix + "test-seed",
			Groups: []string{
				v1beta1constants.SeedsGroup,
				"foo",
			},
		}
		nonSeedUser = user.DefaultInfo{
			Name:   "foo",
			Groups: []string{"foo", "bar"},
		}
	})

	Context("#getGardenerClaims", func() {
		const (
			workloadName      = "identity"
			workloadNamespace = "garden-local"
			workloadUID       = "ab920696-dd12-4723-9bc1-204cfe9edd40"
			sub               = "gardener.cloud:workloadidentity:" + workloadNamespace + ":" + workloadName + ":" + workloadUID
			aud               = "gardener.cloud"
		)

		var (
			r                TokenRequestREST
			workloadIdentity *securityapi.WorkloadIdentity
		)

		BeforeEach(func() {
			r = TokenRequestREST{}
			workloadIdentity = &securityapi.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workloadName,
					Namespace: workloadNamespace,
					UID:       types.UID(workloadUID),
				},
				Spec: securityapi.WorkloadIdentitySpec{
					Audiences: []string{aud},
				},
				Status: securityapi.WorkloadIdentityStatus{
					Sub: sub,
				},
			}
		})

		DescribeTable("#getGardenerClaims",
			func(objType, name, namespace, uid string) {
				Expect(objType).To(BeElementOf("shoot", "seed", "project", "backupBucket", "backupEntry", "none"))

				var ctxObjects *contextObjects
				switch objType {
				case "shoot":
					Expect(name).ToNot(BeEmpty())
					Expect(namespace).ToNot(BeEmpty())
					Expect(uid).ToNot(BeEmpty())
					ctxObjects = &contextObjects{shoot: &metav1.ObjectMeta{Namespace: namespace, Name: name, UID: types.UID(uid)}}

				case "seed":
					Expect(name).ToNot(BeEmpty())
					Expect(namespace).To(BeEmpty())
					Expect(uid).ToNot(BeEmpty())
					ctxObjects = &contextObjects{seed: &metav1.ObjectMeta{Name: name, UID: types.UID(uid)}}

				case "project":
					Expect(name).ToNot(BeEmpty())
					Expect(namespace).To(BeEmpty())
					Expect(uid).ToNot(BeEmpty())
					ctxObjects = &contextObjects{project: &metav1.ObjectMeta{Name: name, UID: types.UID(uid)}}

				case "backupBucket":
					Expect(name).ToNot(BeEmpty())
					Expect(namespace).To(BeEmpty())
					Expect(uid).ToNot(BeEmpty())
					ctxObjects = &contextObjects{backupBucket: &metav1.ObjectMeta{Name: name, UID: types.UID(uid)}}
				case "backupEntry":
					Expect(name).ToNot(BeEmpty())
					Expect(namespace).ToNot(BeEmpty())
					Expect(uid).ToNot(BeEmpty())
					ctxObjects = &contextObjects{backupEntry: &metav1.ObjectMeta{Namespace: namespace, Name: name, UID: types.UID(uid)}}

				case "none":
					Expect(name).To(BeEmpty())
					Expect(namespace).To(BeEmpty())
					Expect(uid).To(BeEmpty())
					ctxObjects = nil
				}

				g := r.getGardenerClaims(workloadIdentity, ctxObjects)

				Expect(g).ToNot(BeNil())
				Expect(g.Gardener.WorkloadIdentity.Name).To(Equal(workloadName))
				Expect(g.Gardener.WorkloadIdentity.Namespace).To(Equal(workloadNamespace))
				Expect(g.Gardener.WorkloadIdentity.UID).To(Equal(workloadUID))

				if objType == "none" {
					Expect(ctxObjects).To(BeNil())
					return
				}

				Expect(ctxObjects).ToNot(BeNil())

				if objType == "shoot" {
					Expect(ctxObjects.shoot).ToNot(BeNil())
					Expect(g.Gardener.Shoot).ToNot(BeNil())
					Expect(g.Gardener.Shoot.Name).To(Equal(ctxObjects.shoot.GetName()))
					Expect(g.Gardener.Shoot.Namespace).To(Equal(ctxObjects.shoot.GetNamespace()))
					Expect(g.Gardener.Shoot.UID).To(BeEquivalentTo(ctxObjects.shoot.GetUID()))
				} else {
					Expect(ctxObjects.shoot).To(BeNil())
					Expect(g.Gardener.Shoot).To(BeNil())
				}

				if objType == "seed" {
					Expect(ctxObjects.seed).ToNot(BeNil())
					Expect(g.Gardener.Seed).ToNot(BeNil())
					Expect(g.Gardener.Seed.Name).To(Equal(ctxObjects.seed.GetName()))
					Expect(g.Gardener.Seed.UID).To(BeEquivalentTo(ctxObjects.seed.GetUID()))
					Expect(g.Gardener.Seed.Namespace).To(BeZero())
				} else {
					Expect(ctxObjects.seed).To(BeNil())
					Expect(g.Gardener.Seed).To(BeNil())
				}

				if objType == "project" {
					Expect(ctxObjects.project).ToNot(BeNil())
					Expect(g.Gardener.Project).ToNot(BeNil())
					Expect(g.Gardener.Project.Name).To(Equal(ctxObjects.project.GetName()))
					Expect(g.Gardener.Project.UID).To(BeEquivalentTo(ctxObjects.project.GetUID()))
					Expect(g.Gardener.Project.Namespace).To(BeZero())
				} else {
					Expect(ctxObjects.project).To(BeNil())
					Expect(g.Gardener.Project).To(BeNil())
				}

				if objType == "backupBucket" {
					Expect(ctxObjects.backupBucket).ToNot(BeNil())
					Expect(g.Gardener.BackupBucket).ToNot(BeNil())
					Expect(g.Gardener.BackupBucket.Name).To(Equal(ctxObjects.backupBucket.GetName()))
					Expect(g.Gardener.BackupBucket.UID).To(BeEquivalentTo(ctxObjects.backupBucket.GetUID()))
					Expect(g.Gardener.BackupBucket.Namespace).To(BeZero())
				} else {
					Expect(ctxObjects.backupBucket).To(BeNil())
					Expect(g.Gardener.BackupBucket).To(BeNil())
				}

				if objType == "backupEntry" {
					Expect(ctxObjects.backupEntry).ToNot(BeNil())
					Expect(g.Gardener.BackupEntry).ToNot(BeNil())
					Expect(g.Gardener.BackupEntry.Name).To(Equal(ctxObjects.backupEntry.GetName()))
					Expect(g.Gardener.BackupEntry.UID).To(BeEquivalentTo(ctxObjects.backupEntry.GetUID()))
					Expect(g.Gardener.BackupEntry.Namespace).To(Equal(ctxObjects.backupEntry.GetNamespace()))
				} else {
					Expect(ctxObjects.backupEntry).To(BeNil())
					Expect(g.Gardener.BackupEntry).To(BeNil())
				}
			},
			Entry("should successfully set common claims", "none", "", "", ""),
			Entry("should successfully set shoot claims", "shoot", "local", "garden-local", "d9bd264e-2192-437f-986a-94fcd8cf5d8a"),
			Entry("should successfully set seed claims", "seed", "local-seed", "", "857afd63-16eb-456a-9d35-c2f7d1578d32"),
			Entry("should successfully set project claims", "project", "local-project", "", "34fecdd7-2694-47ba-a75a-0515d4fa0686"),
			Entry("should successfully set backupBucket claims", "backupBucket", "local-backup-bucket", "", "490e39da-d607-4f24-ba04-fecab68320f6"),
			Entry("should successfully set backupEntry claims", "backupEntry", "local-backup-entry", "garden-local", "b5ff6765-7328-4af2-b284-54d6817ca0e1"),
		)
	})

	Context("#resolveContextObject", func() {
		const (
			shootName        = "test-shoot"
			shootUID         = types.UID("9a134d22-dd61-4845-951e-9a20bde1648a")
			projectName      = "test-project"
			shootTechnicalID = "shoot--" + projectName + "--" + shootName
			projectUID       = types.UID("01c9c6fa-2b8b-496f-8edf-e382f4d61905")
			namespaceName    = "garden-" + projectName
			seedName         = "test-seed"
			seedUID          = types.UID("aa88fb5a-28ef-4987-8c2d-9f02afafcf09")
			backupBucketName = "test-backup-bucket"
			backupBucketUID  = types.UID("493d8e6c-ca45-45de-acf7-68c16547f299")
			backupEntryName  = shootTechnicalID + "--" + string(shootUID)
			backupEntryUID   = types.UID("0a9c3b4a-e932-481e-bb3f-55de31163068")
		)

		var (
			r            TokenRequestREST
			shoot        *gardencorev1beta1.Shoot
			seed         *gardencorev1beta1.Seed
			project      *gardencorev1beta1.Project
			backupBucket *gardencorev1beta1.BackupBucket
			backupEntry  *gardencorev1beta1.BackupEntry

			workloadIdentity    *securityapi.WorkloadIdentity
			workloadIdentityRef *corev1.ObjectReference

			shootInformer              gardenv1betainformers.ShootInformer
			seedInformer               gardenv1betainformers.SeedInformer
			projectInformer            gardenv1betainformers.ProjectInformer
			backupBucketInformer       gardenv1betainformers.BackupBucketInformer
			backupEntryInformer        gardenv1betainformers.BackupEntryInformer
			credentialsBindingInformer securityv1alpha1informers.CredentialsBindingInformer
		)

		BeforeEach(func() {
			// The workload identity is referenced by the seed and the backup bucket. Shoots on the seed may use it, too.
			workloadIdentity = &securityapi.WorkloadIdentity{ObjectMeta: metav1.ObjectMeta{Namespace: v1beta1constants.GardenNamespace, Name: "garden-identity"}}
			workloadIdentityRef = &corev1.ObjectReference{
				APIVersion: securityv1alpha1.SchemeGroupVersion.String(),
				Kind:       "WorkloadIdentity",
				Namespace:  workloadIdentity.Namespace,
				Name:       workloadIdentity.Name,
			}

			shoot = &gardencorev1beta1.Shoot{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Shoot"},
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: shootName, UID: shootUID},
				Spec:       gardencorev1beta1.ShootSpec{SeedName: new(seedName)},
				Status:     gardencorev1beta1.ShootStatus{TechnicalID: shootTechnicalID},
			}
			seed = &gardencorev1beta1.Seed{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Seed"},
				ObjectMeta: metav1.ObjectMeta{Name: seedName, UID: seedUID},
				Spec:       gardencorev1beta1.SeedSpec{Backup: &gardencorev1beta1.Backup{CredentialsRef: workloadIdentityRef}},
			}
			project = &gardencorev1beta1.Project{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Project"},
				ObjectMeta: metav1.ObjectMeta{Name: projectName, UID: projectUID},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: new(namespaceName),
				},
			}
			backupBucket = &gardencorev1beta1.BackupBucket{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "BackupBucket"},
				ObjectMeta: metav1.ObjectMeta{Name: backupBucketName, UID: backupBucketUID},
				Spec:       gardencorev1beta1.BackupBucketSpec{SeedName: new(seedName), CredentialsRef: workloadIdentityRef},
			}
			backupEntry = &gardencorev1beta1.BackupEntry{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "BackupEntry"},
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: backupEntryName, UID: backupEntryUID},
				Spec:       gardencorev1beta1.BackupEntrySpec{BucketName: backupBucketName, SeedName: new(seedName)},
			}
			informerFactory := gardencoreinformers.NewSharedInformerFactory(nil, 0)

			shootInformer = informerFactory.Core().V1beta1().Shoots()
			seedInformer = informerFactory.Core().V1beta1().Seeds()
			projectInformer = informerFactory.Core().V1beta1().Projects()
			backupBucketInformer = informerFactory.Core().V1beta1().BackupBuckets()
			backupEntryInformer = informerFactory.Core().V1beta1().BackupEntries()

			Expect(shootInformer.Informer().GetStore().Add(shoot)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Add(seed)).To(Succeed())
			Expect(projectInformer.Informer().GetStore().Add(project)).To(Succeed())
			Expect(backupBucketInformer.Informer().GetStore().Add(backupBucket)).To(Succeed())
			Expect(backupEntryInformer.Informer().GetStore().Add(backupEntry)).To(Succeed())

			credentialsBindingInformer = securityinformers.NewSharedInformerFactory(nil, 0).Security().V1alpha1().CredentialsBindings()

			r = TokenRequestREST{
				shootListers:             shootInformer.Lister(),
				seedLister:               seedInformer.Lister(),
				projectLister:            projectInformer.Lister(),
				backupBucketLister:       backupBucketInformer.Lister(),
				backupEntryLister:        backupEntryInformer.Lister(),
				credentialsBindingLister: credentialsBindingInformer.Lister(),
			}
		})

		It("should successfully resolve when the context object is nil and user is seed", func() {
			ctxObjects, err := r.resolveContextObject(&seedUser, nil, workloadIdentity)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when the context object is nil and user is not seed", func() {
			ctxObjects, err := r.resolveContextObject(&nonSeedUser, nil, workloadIdentity)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when the context object is not nil and user is not seed", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
				Name:       shootName,
				Namespace:  new(namespaceName),
				UID:        shootUID,
			}

			ctxObjects, err := r.resolveContextObject(&nonSeedUser, contextObject, workloadIdentity)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when the context object is shoot", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
				Name:       shootName,
				Namespace:  new(namespaceName),
				UID:        shootUID,
			}

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).ToNot(BeNil())

			Expect(ctxObjects.shoot).ToNot(BeNil())
			Expect(ctxObjects.shoot.GetName()).To(Equal(shootName))
			Expect(ctxObjects.shoot.GetNamespace()).To(Equal(namespaceName))
			Expect(ctxObjects.shoot.GetUID()).To(Equal(shootUID))

			Expect(ctxObjects.seed).ToNot(BeNil())
			Expect(ctxObjects.seed.GetName()).To(Equal(seedName))
			Expect(ctxObjects.seed.GetUID()).To(Equal(seedUID))

			Expect(ctxObjects.project).ToNot(BeNil())
			Expect(ctxObjects.project.GetName()).To(Equal(projectName))
			Expect(ctxObjects.project.GetUID()).To(Equal(projectUID))

			Expect(ctxObjects.backupBucket).To(BeNil())
			Expect(ctxObjects.backupEntry).To(BeNil())
		})

		It("should fail to resolve with shoot context object", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
				Name:       shootName,
				Namespace:  new(namespaceName),
				UID:        shootUID,
			}

			By("shoot does not exist")
			Expect(shootInformer.Informer().GetStore().Delete(shoot)).To(Succeed())

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("shoot.core.gardener.cloud \"test-shoot\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("shoot and context uid does not match")
			Expect(shootInformer.Informer().GetStore().Add(shoot)).To(Succeed())

			uidMismatch := types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")
			uidMismatchContext := contextObject.DeepCopy()
			uidMismatchContext.UID = uidMismatch

			ctxObjects, err = r.resolveContextObject(&seedUser, uidMismatchContext, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("uid of contextObject (" + uidMismatch + ") and real world Shoot resource (" + shootUID + ") differ"))
			Expect(ctxObjects).To(BeNil())

			By("seed does not exist")
			shoot.Spec = gardencorev1beta1.ShootSpec{
				SeedName: new(seedName),
			}
			Expect(shootInformer.Informer().GetStore().Update(shoot)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Delete(seed)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("seed.core.gardener.cloud \"test-seed\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("project does not exist")
			Expect(seedInformer.Informer().GetStore().Add(seed)).To(Succeed())
			Expect(projectInformer.Informer().GetStore().Delete(project)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("Project.core.gardener.cloud \"<unknown>\" not found"))
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when context object is seed", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Seed",
				Name:       seedName,
				UID:        seedUID,
			}

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).ToNot(BeNil())

			Expect(ctxObjects.shoot).To(BeNil())
			Expect(ctxObjects.project).To(BeNil())
			Expect(ctxObjects.backupBucket).To(BeNil())
			Expect(ctxObjects.backupEntry).To(BeNil())

			Expect(ctxObjects.seed).ToNot(BeNil())
			Expect(ctxObjects.seed.GetName()).To(Equal(seedName))
			Expect(ctxObjects.seed.GetUID()).To(Equal(seedUID))
		})

		It("should fail to resolve with seed context object", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Seed",
				Name:       seedName,
				UID:        seedUID,
			}

			By("seed does not exist")
			Expect(seedInformer.Informer().GetStore().Delete(seed)).To(Succeed())

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("seed.core.gardener.cloud \"test-seed\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("seed and context uid does not match")
			Expect(seedInformer.Informer().GetStore().Add(seed)).To(Succeed())

			uidMismatch := types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")
			uidMismatchContext := contextObject.DeepCopy()
			uidMismatchContext.UID = uidMismatch
			ctxObjects, err = r.resolveContextObject(&seedUser, uidMismatchContext, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("uid of contextObject (" + uidMismatch + ") and real world Seed resource (" + seedUID + ") differ"))
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when context object is backupBucket", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "BackupBucket",
				Name:       backupBucketName,
				UID:        backupBucketUID,
			}

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).ToNot(BeNil())

			Expect(ctxObjects.shoot).To(BeNil())
			Expect(ctxObjects.project).To(BeNil())
			Expect(ctxObjects.backupEntry).To(BeNil())

			Expect(ctxObjects.backupBucket).ToNot(BeNil())
			Expect(ctxObjects.backupBucket.GetName()).To(Equal(backupBucketName))
			Expect(ctxObjects.backupBucket.GetUID()).To(Equal(backupBucketUID))

			Expect(ctxObjects.seed).ToNot(BeNil())
			Expect(ctxObjects.seed.GetName()).To(Equal(seedName))
			Expect(ctxObjects.seed.GetUID()).To(Equal(seedUID))
		})

		It("should fail to resolve with backupBucket context object", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "BackupBucket",
				Name:       backupBucketName,
				UID:        backupBucketUID,
			}

			By("backupBucket does not exist")
			Expect(backupBucketInformer.Informer().GetStore().Delete(backupBucket)).To(Succeed())

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("backupbucket.core.gardener.cloud \"test-backup-bucket\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("backupBucket and context uid does not match")
			Expect(backupBucketInformer.Informer().GetStore().Add(backupBucket)).To(Succeed())

			uidMismatch := types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")
			uidMismatchContext := contextObject.DeepCopy()
			uidMismatchContext.UID = uidMismatch

			ctxObjects, err = r.resolveContextObject(&seedUser, uidMismatchContext, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("uid of contextObject (" + uidMismatch + ") and real world BackupBucket resource (" + backupBucketUID + ") differ"))
			Expect(ctxObjects).To(BeNil())

			By("bind the backupBucket to the seed")
			backupBucketCopy := backupBucket.DeepCopy()
			backupBucketCopy.Spec = gardencorev1beta1.BackupBucketSpec{
				SeedName:       new(seedName),
				CredentialsRef: workloadIdentityRef,
			}
			Expect(backupBucketInformer.Informer().GetStore().Update(backupBucketCopy)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Delete(seed)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("seed.core.gardener.cloud \"test-seed\" not found"))
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when context object is backupEntry", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "BackupEntry",
				Namespace:  new(namespaceName),
				Name:       backupEntryName,
				UID:        backupEntryUID,
			}

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).ToNot(BeNil())

			Expect(ctxObjects.shoot).To(BeNil())
			Expect(ctxObjects.project).To(BeNil())

			Expect(ctxObjects.seed).ToNot(BeNil())
			Expect(ctxObjects.seed.GetName()).To(Equal(seedName))
			Expect(ctxObjects.seed.GetUID()).To(Equal(seedUID))

			Expect(ctxObjects.backupEntry).ToNot(BeNil())
			Expect(ctxObjects.backupEntry.GetName()).To(Equal(backupEntryName))
			Expect(ctxObjects.backupEntry.GetNamespace()).To(Equal(namespaceName))
			Expect(ctxObjects.backupEntry.GetUID()).To(Equal(backupEntryUID))

			Expect(ctxObjects.backupBucket).ToNot(BeNil())
			Expect(ctxObjects.backupBucket.GetName()).To(Equal(backupBucketName))
			Expect(ctxObjects.backupBucket.GetUID()).To(Equal(backupBucketUID))

			By("set shoot as owner of the BackupEntry")
			backupEntry.OwnerReferences = []metav1.OwnerReference{{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
				Name:       shoot.Name,
				UID:        shootUID,
			}}
			Expect(backupEntryInformer.Informer().GetStore().Update(backupEntry)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).ToNot(BeNil())

			Expect(ctxObjects.backupEntry).ToNot(BeNil())
			Expect(ctxObjects.backupEntry.GetName()).To(Equal(backupEntryName))
			Expect(ctxObjects.backupEntry.GetNamespace()).To(Equal(namespaceName))
			Expect(ctxObjects.backupEntry.GetUID()).To(Equal(backupEntryUID))

			Expect(ctxObjects.backupBucket).ToNot(BeNil())
			Expect(ctxObjects.backupBucket.GetName()).To(Equal(backupBucketName))
			Expect(ctxObjects.backupBucket.GetUID()).To(Equal(backupBucketUID))

			Expect(ctxObjects.shoot).ToNot(BeNil())
			Expect(ctxObjects.shoot.GetName()).To(Equal(shootName))
			Expect(ctxObjects.shoot.GetNamespace()).To(Equal(namespaceName))
			Expect(ctxObjects.shoot.GetUID()).To(Equal(shootUID))

			Expect(ctxObjects.project).ToNot(BeNil())
			Expect(ctxObjects.project.GetName()).To(Equal(projectName))
			Expect(ctxObjects.project.GetUID()).To(Equal(projectUID))

			Expect(ctxObjects.seed).ToNot(BeNil())
			Expect(ctxObjects.seed.GetName()).To(Equal(seedName))
			Expect(ctxObjects.seed.GetUID()).To(Equal(seedUID))

			By("delete the shoot")
			Expect(shootInformer.Informer().GetStore().Delete(shoot)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).ToNot(BeNil())

			Expect(ctxObjects.backupEntry).ToNot(BeNil())
			Expect(ctxObjects.backupEntry.GetName()).To(Equal(backupEntryName))
			Expect(ctxObjects.backupEntry.GetNamespace()).To(Equal(namespaceName))
			Expect(ctxObjects.backupEntry.GetUID()).To(Equal(backupEntryUID))

			Expect(ctxObjects.backupBucket).ToNot(BeNil())
			Expect(ctxObjects.backupBucket.GetName()).To(Equal(backupBucketName))
			Expect(ctxObjects.backupBucket.GetUID()).To(Equal(backupBucketUID))

			Expect(ctxObjects.seed).ToNot(BeNil())
			Expect(ctxObjects.seed.GetName()).To(Equal(seedName))
			Expect(ctxObjects.seed.GetUID()).To(Equal(seedUID))

			Expect(ctxObjects.shoot).To(BeNil())
			Expect(ctxObjects.project).To(BeNil())
		})

		It("should fail to resolve with backupEntry context object", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "BackupEntry",
				Namespace:  new(namespaceName),
				Name:       backupEntryName,
				UID:        backupEntryUID,
			}

			By("backupEntry does not exist")
			Expect(backupEntryInformer.Informer().GetStore().Delete(backupEntry)).To(Succeed())

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("backupentry.core.gardener.cloud \"shoot--test-project--test-shoot--9a134d22-dd61-4845-951e-9a20bde1648a\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("backupEntry and context uid does not match")
			Expect(backupEntryInformer.Informer().GetStore().Add(backupEntry)).To(Succeed())

			uidMismatch := types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")
			uidMismatchContext := contextObject.DeepCopy()
			uidMismatchContext.UID = uidMismatch

			ctxObjects, err = r.resolveContextObject(&seedUser, uidMismatchContext, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("uid of contextObject (" + uidMismatch + ") and real world BackupEntry resource (" + backupEntryUID + ") differ"))
			Expect(ctxObjects).To(BeNil())

			By("backupBucket does not exist")
			Expect(backupBucketInformer.Informer().GetStore().Delete(backupBucket)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("backupbucket.core.gardener.cloud \"test-backup-bucket\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("bind the backupEntry to the seed")
			backupEntryCopy := backupEntry.DeepCopy()
			backupEntryCopy.Spec.SeedName = new(seedName)
			Expect(backupEntryInformer.Informer().GetStore().Update(backupEntryCopy)).To(Succeed())
			Expect(backupBucketInformer.Informer().GetStore().Add(backupBucket)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Delete(seed)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("seed.core.gardener.cloud \"test-seed\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("project does not exist")
			backupEntry.OwnerReferences = []metav1.OwnerReference{{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
				Name:       shoot.Name,
				UID:        shootUID,
			}}
			Expect(backupEntryInformer.Informer().GetStore().Update(backupEntry)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Add(seed)).To(Succeed())
			Expect(shootInformer.Informer().GetStore().Add(shoot)).To(Succeed())
			Expect(projectInformer.Informer().GetStore().Delete(project)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("Project.core.gardener.cloud \"<unknown>\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("getting the shoot returns error other than notFound")
			Expect(projectInformer.Informer().GetStore().Add(project)).To(Succeed())
			expectedError := errors.New("expect getting shoot to fail")
			r.shootListers = &fakeShootLister{err: expectedError}

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedError))
			Expect(ctxObjects).To(BeNil())
		})

		It("should fail to resolve with unsupported context object", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: "test/v1alpha1",
				Kind:       "Foo",
				Name:       "test-pod",
				UID:        types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c"),
			}

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unsupported GVK for context object: test/v1alpha1, Kind=Foo"))
			Expect(ctxObjects).To(BeNil())
		})

		It("should forbid requesters in the seeds group without a seed identity", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Seed",
				Name:       seedName,
				UID:        seedUID,
			}
			invalidSeedUser := &user.DefaultInfo{Name: "foo", Groups: []string{v1beta1constants.SeedsGroup}}

			ctxObjects, err := r.resolveContextObject(invalidSeedUser, contextObject, workloadIdentity)
			Expect(err).To(BeForbiddenError())
			Expect(ctxObjects).To(BeNil())
		})

		Context("context object not related to the requesting seed", func() {
			const otherSeedName = "other-seed"

			var uidMismatch = types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")

			It("should forbid shoots of other seeds without disclosing their UID", func() {
				shoot.Spec.SeedName = new(otherSeedName)
				Expect(shootInformer.Informer().GetStore().Update(shoot)).To(Succeed())

				ctxObjects, err := r.resolveContextObject(&seedUser, &securityapi.ContextObject{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Shoot",
					Namespace:  new(namespaceName),
					Name:       shootName,
					UID:        uidMismatch,
				}, workloadIdentity)
				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).NotTo(ContainSubstring(string(shootUID)))
				Expect(ctxObjects).To(BeNil())
			})

			It("should forbid shoots which are not scheduled", func() {
				shoot.Spec.SeedName = nil
				Expect(shootInformer.Informer().GetStore().Update(shoot)).To(Succeed())

				ctxObjects, err := r.resolveContextObject(&seedUser, &securityapi.ContextObject{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Shoot",
					Namespace:  new(namespaceName),
					Name:       shootName,
					UID:        shootUID,
				}, workloadIdentity)
				Expect(err).To(BeForbiddenError())
				Expect(ctxObjects).To(BeNil())
			})

			It("should allow the source seed to resolve shoots during control plane migration", func() {
				Expect(seedInformer.Informer().GetStore().Add(&gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: otherSeedName}})).To(Succeed())
				shoot.Spec.SeedName = new(otherSeedName)
				shoot.Status.SeedName = new(seedName)
				Expect(shootInformer.Informer().GetStore().Update(shoot)).To(Succeed())

				ctxObjects, err := r.resolveContextObject(&seedUser, &securityapi.ContextObject{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Shoot",
					Namespace:  new(namespaceName),
					Name:       shootName,
					UID:        shootUID,
				}, workloadIdentity)
				Expect(err).ToNot(HaveOccurred())
				Expect(ctxObjects.shoot.GetUID()).To(Equal(shootUID))
			})

			It("should forbid other seeds without looking them up", func() {
				ctxObjects, err := r.resolveContextObject(&seedUser, &securityapi.ContextObject{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Seed",
					Name:       "non-existing-seed",
					UID:        uidMismatch,
				}, workloadIdentity)
				Expect(err).To(BeForbiddenError())
				Expect(ctxObjects).To(BeNil())
			})

			It("should forbid backupBuckets of other seeds without disclosing their UID", func() {
				backupBucket.Spec.SeedName = new(otherSeedName)
				Expect(backupBucketInformer.Informer().GetStore().Update(backupBucket)).To(Succeed())

				ctxObjects, err := r.resolveContextObject(&seedUser, &securityapi.ContextObject{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "BackupBucket",
					Name:       backupBucketName,
					UID:        uidMismatch,
				}, workloadIdentity)
				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).NotTo(ContainSubstring(string(backupBucketUID)))
				Expect(ctxObjects).To(BeNil())
			})

			It("should forbid backupBuckets which are not assigned to a seed", func() {
				backupBucket.Spec.SeedName = nil
				Expect(backupBucketInformer.Informer().GetStore().Update(backupBucket)).To(Succeed())

				ctxObjects, err := r.resolveContextObject(&seedUser, &securityapi.ContextObject{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "BackupBucket",
					Name:       backupBucketName,
					UID:        backupBucketUID,
				}, workloadIdentity)
				Expect(err).To(BeForbiddenError())
				Expect(ctxObjects).To(BeNil())
			})

			It("should forbid backupEntries of other seeds without disclosing their UID", func() {
				backupEntry.Spec.SeedName = new(otherSeedName)
				Expect(backupEntryInformer.Informer().GetStore().Update(backupEntry)).To(Succeed())

				ctxObjects, err := r.resolveContextObject(&seedUser, &securityapi.ContextObject{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "BackupEntry",
					Namespace:  new(namespaceName),
					Name:       backupEntryName,
					UID:        uidMismatch,
				}, workloadIdentity)
				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).NotTo(ContainSubstring(string(backupEntryUID)))
				Expect(ctxObjects).To(BeNil())
			})

			It("should allow the source seed to resolve backupEntries during control plane migration", func() {
				Expect(seedInformer.Informer().GetStore().Add(&gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: otherSeedName}})).To(Succeed())
				backupEntry.Spec.SeedName = new(otherSeedName)
				backupEntry.Status.SeedName = new(seedName)
				Expect(backupEntryInformer.Informer().GetStore().Update(backupEntry)).To(Succeed())

				ctxObjects, err := r.resolveContextObject(&seedUser, &securityapi.ContextObject{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "BackupEntry",
					Namespace:  new(namespaceName),
					Name:       backupEntryName,
					UID:        backupEntryUID,
				}, workloadIdentity)
				Expect(err).ToNot(HaveOccurred())
				Expect(ctxObjects.backupEntry.GetUID()).To(Equal(backupEntryUID))
			})
		})

		Context("workload identity not referenced by the context object", func() {
			const projectWorkloadIdentityName = "project-identity"

			var (
				projectWorkloadIdentity   *securityapi.WorkloadIdentity
				shootContextObject        *securityapi.ContextObject
				seedContextObject         *securityapi.ContextObject
				backupBucketContextObject *securityapi.ContextObject
				backupEntryContextObject  *securityapi.ContextObject

				workloadIdentityCrossVersionRef = func(name string) autoscalingv1.CrossVersionObjectReference {
					return autoscalingv1.CrossVersionObjectReference{APIVersion: securityv1alpha1.SchemeGroupVersion.String(), Kind: "WorkloadIdentity", Name: name}
				}
			)

			BeforeEach(func() {
				projectWorkloadIdentity = &securityapi.WorkloadIdentity{ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: projectWorkloadIdentityName}}

				shootContextObject = &securityapi.ContextObject{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Shoot", Namespace: new(namespaceName), Name: shootName, UID: shootUID}
				seedContextObject = &securityapi.ContextObject{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Seed", Name: seedName, UID: seedUID}
				backupBucketContextObject = &securityapi.ContextObject{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "BackupBucket", Name: backupBucketName, UID: backupBucketUID}
				backupEntryContextObject = &securityapi.ContextObject{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "BackupEntry", Namespace: new(namespaceName), Name: backupEntryName, UID: backupEntryUID}

				seed.Spec.Backup = nil
				Expect(seedInformer.Informer().GetStore().Update(seed)).To(Succeed())
			})

			It("should forbid shoots and seeds which do not reference the workload identity", func() {
				_, err := r.resolveContextObject(&seedUser, shootContextObject, workloadIdentity)
				Expect(err).To(BeForbiddenError())

				_, err = r.resolveContextObject(&seedUser, seedContextObject, workloadIdentity)
				Expect(err).To(BeForbiddenError())
			})

			It("should forbid backupBuckets and backupEntries whose bucket uses another workload identity", func() {
				_, err := r.resolveContextObject(&seedUser, backupBucketContextObject, projectWorkloadIdentity)
				Expect(err).To(BeForbiddenError())

				_, err = r.resolveContextObject(&seedUser, backupEntryContextObject, projectWorkloadIdentity)
				Expect(err).To(BeForbiddenError())
			})

			It("should forbid a workload identity of another project with the same name as the referenced one", func() {
				shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{Name: "identity", ResourceRef: workloadIdentityCrossVersionRef(projectWorkloadIdentityName)}}
				Expect(shootInformer.Informer().GetStore().Update(shoot)).To(Succeed())

				otherProjectWorkloadIdentity := &securityapi.WorkloadIdentity{ObjectMeta: metav1.ObjectMeta{Namespace: "garden-other", Name: projectWorkloadIdentityName}}

				_, err := r.resolveContextObject(&seedUser, shootContextObject, otherProjectWorkloadIdentity)
				Expect(err).To(BeForbiddenError())
			})

			DescribeTable("should allow workload identities referenced by the shoot",
				func(mutate func(*gardencorev1beta1.Shoot)) {
					mutate(shoot)
					Expect(shootInformer.Informer().GetStore().Update(shoot)).To(Succeed())

					ctxObjects, err := r.resolveContextObject(&seedUser, shootContextObject, projectWorkloadIdentity)
					Expect(err).ToNot(HaveOccurred())
					Expect(ctxObjects.shoot.GetUID()).To(Equal(shootUID))
				},

				Entry("via resources", func(s *gardencorev1beta1.Shoot) {
					s.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{Name: "identity", ResourceRef: workloadIdentityCrossVersionRef(projectWorkloadIdentityName)}}
				}),
				Entry("via DNS provider", func(s *gardencorev1beta1.Shoot) {
					ref := workloadIdentityCrossVersionRef(projectWorkloadIdentityName)
					s.Spec.DNS = &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{CredentialsRef: &ref}}}
				}),
			)

			It("should allow and forbid workload identities based on the shoot's credentials binding", func() {
				credentialsBinding := &securityv1alpha1.CredentialsBinding{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: "infra"},
					CredentialsRef: corev1.ObjectReference{
						APIVersion: securityv1alpha1.SchemeGroupVersion.String(),
						Kind:       "WorkloadIdentity",
						Namespace:  namespaceName,
						Name:       projectWorkloadIdentityName,
					},
				}
				Expect(credentialsBindingInformer.Informer().GetStore().Add(credentialsBinding)).To(Succeed())
				shoot.Spec.CredentialsBindingName = new(credentialsBinding.Name)
				Expect(shootInformer.Informer().GetStore().Update(shoot)).To(Succeed())

				_, err := r.resolveContextObject(&seedUser, shootContextObject, projectWorkloadIdentity)
				Expect(err).ToNot(HaveOccurred())

				By("credentials binding referencing another workload identity")
				credentialsBinding.CredentialsRef.Name = "other-identity"
				Expect(credentialsBindingInformer.Informer().GetStore().Update(credentialsBinding)).To(Succeed())

				_, err = r.resolveContextObject(&seedUser, shootContextObject, projectWorkloadIdentity)
				Expect(err).To(BeForbiddenError())

				By("credentials binding does not exist")
				Expect(credentialsBindingInformer.Informer().GetStore().Delete(credentialsBinding)).To(Succeed())

				_, err = r.resolveContextObject(&seedUser, shootContextObject, projectWorkloadIdentity)
				Expect(err).To(BeForbiddenError())
			})

			DescribeTable("should allow workload identities referenced by the requesting seed for seeds and their shoots",
				func(mutate func(*gardencorev1beta1.Seed, *corev1.ObjectReference)) {
					mutate(seed, workloadIdentityRef)
					Expect(seedInformer.Informer().GetStore().Update(seed)).To(Succeed())

					_, err := r.resolveContextObject(&seedUser, seedContextObject, workloadIdentity)
					Expect(err).ToNot(HaveOccurred())

					_, err = r.resolveContextObject(&seedUser, shootContextObject, workloadIdentity)
					Expect(err).ToNot(HaveOccurred())
				},

				Entry("via backup", func(s *gardencorev1beta1.Seed, ref *corev1.ObjectReference) {
					s.Spec.Backup = &gardencorev1beta1.Backup{CredentialsRef: ref}
				}),
				Entry("via DNS provider", func(s *gardencorev1beta1.Seed, ref *corev1.ObjectReference) {
					s.Spec.DNS.Provider = &gardencorev1beta1.SeedDNSProvider{CredentialsRef: ref}
				}),
				Entry("via internal DNS", func(s *gardencorev1beta1.Seed, ref *corev1.ObjectReference) {
					s.Spec.DNS.Internal = &gardencorev1beta1.SeedDNSProviderConfig{CredentialsRef: *ref}
				}),
				Entry("via default DNS", func(s *gardencorev1beta1.Seed, ref *corev1.ObjectReference) {
					s.Spec.DNS.Defaults = []gardencorev1beta1.SeedDNSProviderConfig{{CredentialsRef: *ref}}
				}),
				Entry("via resources", func(s *gardencorev1beta1.Seed, ref *corev1.ObjectReference) {
					s.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{Name: "identity", ResourceRef: workloadIdentityCrossVersionRef(ref.Name)}}
				}),
			)
		})
	})

	Context("#issueToken", func() {
		const (
			issuer            = "https://test.local.gardener.cloud"
			workloadName      = "identity"
			workloadNamespace = "garden-local"
			workloadUID       = "ab920696-dd12-4723-9bc1-204cfe9edd40"
			sub               = "gardener.cloud:workloadidentity:" + workloadNamespace + ":" + workloadName + ":" + workloadUID
			aud               = "gardener.cloud"
			shootName         = "test-shoot"
			shootUID          = types.UID("9a134d22-dd61-4845-951e-9a20bde1648a")
			projectName       = "test-project"
			projectUID        = types.UID("01c9c6fa-2b8b-496f-8edf-e382f4d61905")
			namespaceName     = "garden-" + projectName
			seedName          = "test-seed"
			seedUID           = types.UID("aa88fb5a-28ef-4987-8c2d-9f02afafcf09")
		)

		var (
			minDuration      int64
			maxDuration      int64
			r                TokenRequestREST
			workloadIdentity *securityapi.WorkloadIdentity
			shoot            *gardencorev1beta1.Shoot
			seed             *gardencorev1beta1.Seed
			project          *gardencorev1beta1.Project
		)

		BeforeEach(func() {
			minDuration = int64(time.Minute.Seconds() * 10)
			maxDuration = int64(time.Hour.Seconds() * 48)

			workloadIdentity = &securityapi.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workloadName,
					Namespace: workloadNamespace,
					UID:       types.UID(workloadUID),
				},
				Spec: securityapi.WorkloadIdentitySpec{
					Audiences: []string{aud},
				},
				Status: securityapi.WorkloadIdentityStatus{
					Sub: sub,
				},
			}
			shoot = &gardencorev1beta1.Shoot{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Shoot"},
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: shootName, UID: shootUID},
			}
			seed = &gardencorev1beta1.Seed{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Seed"},
				ObjectMeta: metav1.ObjectMeta{Name: seedName, UID: seedUID},
				Spec: gardencorev1beta1.SeedSpec{Backup: &gardencorev1beta1.Backup{CredentialsRef: &corev1.ObjectReference{
					APIVersion: securityv1alpha1.SchemeGroupVersion.String(),
					Kind:       "WorkloadIdentity",
					Namespace:  workloadNamespace,
					Name:       workloadName,
				}}},
			}
			project = &gardencorev1beta1.Project{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Project"},
				ObjectMeta: metav1.ObjectMeta{Name: projectName, UID: projectUID},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: new(namespaceName),
				},
			}

			informerFactory := gardencoreinformers.NewSharedInformerFactory(nil, 0)

			shootInformer := informerFactory.Core().V1beta1().Shoots()
			seedInformer := informerFactory.Core().V1beta1().Seeds()
			projectInformer := informerFactory.Core().V1beta1().Projects()

			Expect(shootInformer.Informer().GetStore().Add(shoot)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Add(seed)).To(Succeed())
			Expect(projectInformer.Informer().GetStore().Add(project)).To(Succeed())

			tokenIssuer, err := workloadidentity.NewTokenIssuer(rsaPrivateKey, issuer, minDuration, maxDuration)
			Expect(err).ToNot(HaveOccurred())

			r = TokenRequestREST{
				shootListers:  shootInformer.Lister(),
				seedLister:    seedInformer.Lister(),
				projectLister: projectInformer.Lister(),

				tokenIssuer: tokenIssuer,
			}
		})

		It("should successfully issue token", func() {
			var (
				now          = time.Now()
				tokenRequest = &securityapi.TokenRequest{
					Spec: securityapi.TokenRequestSpec{
						ContextObject: &securityapi.ContextObject{
							APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
							Kind:       "Shoot",
							Name:       shootName,
							Namespace:  new(namespaceName),
							UID:        shootUID,
						},
						ExpirationSeconds: int64(3600),
					},
				}
				user = user.DefaultInfo{
					Name:   "foo",
					Groups: []string{"foo", "bar"},
				}
			)
			token, exp, err := r.issueToken(&user, tokenRequest, workloadIdentity)

			Expect(err).ToNot(HaveOccurred())
			Expect(exp).ToNot(BeNil())
			Expect(exp.After(now)).To(BeTrue())
			Expect(token).ToNot(BeEmpty())
		})

		It("should omit setting empty namespace claim", func() {
			var (
				now          = time.Now()
				tokenRequest = &securityapi.TokenRequest{
					Spec: securityapi.TokenRequestSpec{
						ContextObject: &securityapi.ContextObject{
							APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
							Kind:       "Seed",
							Name:       seedName,
							UID:        seedUID,
						},
						ExpirationSeconds: int64(3600),
					},
				}
			)
			token, exp, err := r.issueToken(&seedUser, tokenRequest, workloadIdentity)

			Expect(err).ToNot(HaveOccurred())
			Expect(exp).ToNot(BeNil())
			Expect(exp.After(now)).To(BeTrue())
			Expect(token).ToNot(BeEmpty())
			Expect(strings.Split(token, ".")).To(HaveLen(3))

			encodedPayload := strings.Split(token, ".")[1]
			Expect(encodedPayload).ToNot(BeEmpty())

			payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
			Expect(err).ToNot(HaveOccurred())

			gardenerClaims := map[string]any{}
			Expect(json.Unmarshal(payload, &gardenerClaims)).To(Succeed())
			Expect(gardenerClaims).To(HaveKey("gardener.cloud"))

			gardenerCloud, ok := (gardenerClaims["gardener.cloud"]).(map[string]any)
			Expect(ok).To(BeTrue())

			Expect(gardenerCloud).To(HaveKey("seed"))
			seedClaims, ok := (gardenerCloud["seed"]).(map[string]any)
			Expect(ok).To(BeTrue())

			Expect(seedClaims).To(HaveLen(2))
			Expect(seedClaims).To(Not(HaveKey("namespace")))
			Expect(seedClaims).To(And(HaveKey("name"), HaveKey("uid")))

			name, ok := (seedClaims["name"]).(string)
			Expect(ok).To(BeTrue())
			Expect(name).To(Equal(seedName))

			uid, ok := (seedClaims["uid"]).(string)
			Expect(ok).To(BeTrue())
			Expect(uid).To(Equal(string(seedUID)))
		})
	})
})

var (
	_ gardencorev1beta1listers.ShootLister          = (*fakeShootLister)(nil)
	_ gardencorev1beta1listers.ShootNamespaceLister = (*fakeShootNamespaceLister)(nil)
)

type fakeShootLister struct {
	err error
}

func (*fakeShootLister) List(_ labels.Selector) (ret []*gardencorev1beta1.Shoot, err error) {
	return []*gardencorev1beta1.Shoot{}, nil
}

func (f *fakeShootLister) Shoots(_ string) gardencorev1beta1listers.ShootNamespaceLister {
	return &fakeShootNamespaceLister{err: f.err}
}

type fakeShootNamespaceLister struct {
	err error
}

func (*fakeShootNamespaceLister) List(_ labels.Selector) (ret []*gardencorev1beta1.Shoot, err error) {
	return []*gardencorev1beta1.Shoot{}, nil
}

func (f *fakeShootNamespaceLister) Get(_ string) (*gardencorev1beta1.Shoot, error) {
	return nil, f.err
}
