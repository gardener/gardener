// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"crypto/rand"
	"crypto/rsa"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityapi "github.com/gardener/gardener/pkg/apis/security"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardenv1betainformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

var (
	rsaPrivateKey *rsa.PrivateKey
)

var _ = BeforeSuite(func() {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 4096)
	Expect(err).ToNot(HaveOccurred())
	rsaPrivateKey = rsaKey
})

var _ = Describe("#TokenRequest", func() {
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
				Expect(g.Gardener.WorkloadIdentity.Namespace).ToNot(BeNil())
				Expect(*g.Gardener.WorkloadIdentity.Namespace).To(Equal(workloadNamespace))
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
					Expect(g.Gardener.Shoot.Namespace).ToNot(BeNil())
					Expect(*g.Gardener.Shoot.Namespace).To(Equal(ctxObjects.shoot.GetNamespace()))
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
					Expect(g.Gardener.Seed.Namespace).To(BeNil())
				} else {
					Expect(ctxObjects.seed).To(BeNil())
					Expect(g.Gardener.Seed).To(BeNil())
				}

				if objType == "project" {
					Expect(ctxObjects.project).ToNot(BeNil())
					Expect(g.Gardener.Project).ToNot(BeNil())
					Expect(g.Gardener.Project.Name).To(Equal(ctxObjects.project.GetName()))
					Expect(g.Gardener.Project.UID).To(BeEquivalentTo(ctxObjects.project.GetUID()))
					Expect(g.Gardener.Project.Namespace).To(BeNil())
				} else {
					Expect(ctxObjects.project).To(BeNil())
					Expect(g.Gardener.Project).To(BeNil())
				}

				if objType == "backupBucket" {
					Expect(ctxObjects.backupBucket).ToNot(BeNil())
					Expect(g.Gardener.BackupBucket).ToNot(BeNil())
					Expect(g.Gardener.BackupBucket.Name).To(Equal(ctxObjects.backupBucket.GetName()))
					Expect(g.Gardener.BackupBucket.UID).To(BeEquivalentTo(ctxObjects.backupBucket.GetUID()))
					Expect(g.Gardener.BackupBucket.Namespace).To(BeNil())
				} else {
					Expect(ctxObjects.backupBucket).To(BeNil())
					Expect(g.Gardener.BackupBucket).To(BeNil())
				}

				if objType == "backupEntry" {
					Expect(ctxObjects.backupEntry).ToNot(BeNil())
					Expect(g.Gardener.BackupEntry).ToNot(BeNil())
					Expect(g.Gardener.BackupEntry.Name).To(Equal(ctxObjects.backupEntry.GetName()))
					Expect(g.Gardener.BackupEntry.UID).To(BeEquivalentTo(ctxObjects.backupEntry.GetUID()))
					Expect(g.Gardener.BackupEntry.Namespace).ToNot(BeNil())
					Expect(*g.Gardener.BackupEntry.Namespace).To(Equal(ctxObjects.backupEntry.GetNamespace()))
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
			seedUser     user.DefaultInfo
			nonSeedUser  user.DefaultInfo

			shootInformer        gardenv1betainformers.ShootInformer
			seedInformer         gardenv1betainformers.SeedInformer
			projectInformer      gardenv1betainformers.ProjectInformer
			backupBucketInformer gardenv1betainformers.BackupBucketInformer
			backupEntryInformer  gardenv1betainformers.BackupEntryInformer
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Shoot"},
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: shootName, UID: shootUID},
				Status:     gardencorev1beta1.ShootStatus{TechnicalID: shootTechnicalID},
			}
			seed = &gardencorev1beta1.Seed{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Seed"},
				ObjectMeta: metav1.ObjectMeta{Name: seedName, UID: seedUID},
			}
			project = &gardencorev1beta1.Project{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Project"},
				ObjectMeta: metav1.ObjectMeta{Name: projectName, UID: projectUID},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: ptr.To(namespaceName),
				},
			}
			backupBucket = &gardencorev1beta1.BackupBucket{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "BackupBucket"},
				ObjectMeta: metav1.ObjectMeta{Name: backupBucketName, UID: backupBucketUID},
			}
			backupEntry = &gardencorev1beta1.BackupEntry{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "BackupEntry"},
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: backupEntryName, UID: backupEntryUID},
				Spec:       gardencorev1beta1.BackupEntrySpec{BucketName: backupBucketName},
			}
			seedUser = user.DefaultInfo{
				Name: v1beta1constants.SeedUserNamePrefix + "foo",
				Groups: []string{
					v1beta1constants.SeedsGroup,
					"foo",
				},
			}
			nonSeedUser = user.DefaultInfo{
				Name:   "foo",
				Groups: []string{"foo", "bar"},
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

			r = TokenRequestREST{
				shootListers:       shootInformer.Lister(),
				seedLister:         seedInformer.Lister(),
				projectLister:      projectInformer.Lister(),
				backupBucketLister: backupBucketInformer.Lister(),
				backupEntryLister:  backupEntryInformer.Lister(),
			}
		})

		It("should successfully resolve when the context object is nil and user is seed", func() {
			ctxObjects, err := r.resolveContextObject(&seedUser, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when the context object is nil and user is not seed", func() {
			ctxObjects, err := r.resolveContextObject(&nonSeedUser, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when the context object is not nil and user is not seed", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
				Name:       shootName,
				Namespace:  ptr.To(namespaceName),
				UID:        shootUID,
			}

			ctxObjects, err := r.resolveContextObject(&nonSeedUser, contextObject)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when the context object is shoot", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
				Name:       shootName,
				Namespace:  ptr.To(namespaceName),
				UID:        shootUID,
			}

			By("Resolve context object before the shoot is scheduled to a seed")
			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).ToNot(BeNil())

			Expect(ctxObjects.shoot).ToNot(BeNil())
			Expect(ctxObjects.shoot.GetName()).To(Equal(shootName))
			Expect(ctxObjects.shoot.GetNamespace()).To(Equal(namespaceName))
			Expect(ctxObjects.shoot.GetUID()).To(Equal(shootUID))

			Expect(ctxObjects.seed).To(BeNil())
			Expect(ctxObjects.backupBucket).To(BeNil())
			Expect(ctxObjects.backupEntry).To(BeNil())

			Expect(ctxObjects.project).ToNot(BeNil())
			Expect(ctxObjects.project.GetName()).To(Equal(projectName))
			Expect(ctxObjects.project.GetUID()).To(Equal(projectUID))

			By("Schedule the shoot to a seed")
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec = gardencorev1beta1.ShootSpec{SeedName: ptr.To(seedName)}
			Expect(shootInformer.Informer().GetStore().Update(shootCopy)).To(Succeed())

			By("Resolve context object after the shoot is scheduled to a seed")
			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
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
				Namespace:  ptr.To(namespaceName),
				UID:        shootUID,
			}

			By("shoot does not exist")
			Expect(shootInformer.Informer().GetStore().Delete(shoot)).To(Succeed())

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("shoot.core.gardener.cloud \"test-shoot\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("shoot and context uid does not match")
			Expect(shootInformer.Informer().GetStore().Add(shoot)).To(Succeed())

			uidMismatch := types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")
			uidMismatchContext := contextObject.DeepCopy()
			uidMismatchContext.UID = uidMismatch

			ctxObjects, err = r.resolveContextObject(&seedUser, uidMismatchContext)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("uid of contextObject (" + uidMismatch + ") and real world resource(" + shootUID + ") differ"))
			Expect(ctxObjects).To(BeNil())

			By("seed does not exist")
			shoot.Spec = gardencorev1beta1.ShootSpec{
				SeedName: ptr.To(seedName),
			}
			Expect(shootInformer.Informer().GetStore().Update(shoot)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Delete(seed)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("seed.core.gardener.cloud \"test-seed\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("project does not exist")
			Expect(seedInformer.Informer().GetStore().Add(seed)).To(Succeed())
			Expect(projectInformer.Informer().GetStore().Delete(project)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
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

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject)
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

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("seed.core.gardener.cloud \"test-seed\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("seed and context uid does not match")
			Expect(seedInformer.Informer().GetStore().Add(seed)).To(Succeed())

			uidMismatch := types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")
			uidMismatchContext := contextObject.DeepCopy()
			uidMismatchContext.UID = uidMismatch
			ctxObjects, err = r.resolveContextObject(&seedUser, uidMismatchContext)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("uid of contextObject (" + uidMismatch + ") and real world resource(" + seedUID + ") differ"))
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when context object is backupBucket", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "BackupBucket",
				Name:       backupBucketName,
				UID:        backupBucketUID,
			}

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).ToNot(BeNil())

			Expect(ctxObjects.shoot).To(BeNil())
			Expect(ctxObjects.seed).To(BeNil())
			Expect(ctxObjects.project).To(BeNil())
			Expect(ctxObjects.backupEntry).To(BeNil())

			Expect(ctxObjects.backupBucket).ToNot(BeNil())
			Expect(ctxObjects.backupBucket.GetName()).To(Equal(backupBucketName))
			Expect(ctxObjects.backupBucket.GetUID()).To(Equal(backupBucketUID))

			By("bind the backupBucket to the seed")
			backupBucketCopy := backupBucket.DeepCopy()
			backupBucketCopy.Spec = gardencorev1beta1.BackupBucketSpec{
				SeedName: ptr.To(seedName),
			}
			Expect(backupBucketInformer.Informer().GetStore().Update(backupBucketCopy)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
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

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("backupbucket.core.gardener.cloud \"test-backup-bucket\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("backupBucket and context uid does not match")
			Expect(backupBucketInformer.Informer().GetStore().Add(backupBucket)).To(Succeed())

			uidMismatch := types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")
			uidMismatchContext := contextObject.DeepCopy()
			uidMismatchContext.UID = uidMismatch

			ctxObjects, err = r.resolveContextObject(&seedUser, uidMismatchContext)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("uid of contextObject (" + uidMismatch + ") and real world resource(" + backupBucketUID + ") differ"))
			Expect(ctxObjects).To(BeNil())

			By("bind the backupBucket to the seed")
			backupBucketCopy := backupBucket.DeepCopy()
			backupBucketCopy.Spec = gardencorev1beta1.BackupBucketSpec{
				SeedName: ptr.To(seedName),
			}
			Expect(backupBucketInformer.Informer().GetStore().Update(backupBucketCopy)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Delete(seed)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("seed.core.gardener.cloud \"test-seed\" not found"))
			Expect(ctxObjects).To(BeNil())
		})

		It("should successfully resolve when context object is backupEntry", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "BackupEntry",
				Namespace:  ptr.To(namespaceName),
				Name:       backupEntryName,
				UID:        backupEntryUID,
			}

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).ToNot(BeNil())

			Expect(ctxObjects.shoot).To(BeNil())
			Expect(ctxObjects.project).To(BeNil())
			Expect(ctxObjects.seed).To(BeNil())

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

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
			Expect(err).ToNot(HaveOccurred())
			Expect(ctxObjects).ToNot(BeNil())

			Expect(ctxObjects.seed).To(BeNil())

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

			By("bind the backupEntry to the seed")
			backupEntryCopy := backupEntry.DeepCopy()
			backupEntryCopy.Spec.SeedName = ptr.To(seedName)
			Expect(backupEntryInformer.Informer().GetStore().Update(backupEntryCopy)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
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
		})

		It("should fail to resolve with backupEntry context object", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "BackupEntry",
				Namespace:  ptr.To(namespaceName),
				Name:       backupEntryName,
				UID:        backupEntryUID,
			}

			By("backupEntry does not exist")
			Expect(backupEntryInformer.Informer().GetStore().Delete(backupEntry)).To(Succeed())

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("backupentry.core.gardener.cloud \"shoot--test-project--test-shoot--9a134d22-dd61-4845-951e-9a20bde1648a\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("backupEntry and context uid does not match")
			Expect(backupEntryInformer.Informer().GetStore().Add(backupEntry)).To(Succeed())

			uidMismatch := types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")
			uidMismatchContext := contextObject.DeepCopy()
			uidMismatchContext.UID = uidMismatch

			ctxObjects, err = r.resolveContextObject(&seedUser, uidMismatchContext)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("uid of contextObject (" + uidMismatch + ") and real world resource(" + backupEntryUID + ") differ"))
			Expect(ctxObjects).To(BeNil())

			By("backupBucket does not exist")
			Expect(backupBucketInformer.Informer().GetStore().Delete(backupBucket)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("backupbucket.core.gardener.cloud \"test-backup-bucket\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("bind the backupEntry to the seed")
			backupEntryCopy := backupEntry.DeepCopy()
			backupEntryCopy.Spec.SeedName = ptr.To(seedName)
			Expect(backupEntryInformer.Informer().GetStore().Update(backupEntryCopy)).To(Succeed())
			Expect(backupBucketInformer.Informer().GetStore().Add(backupBucket)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Delete(seed)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("seed.core.gardener.cloud \"test-seed\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("shoot does not exist")
			backupEntry.OwnerReferences = []metav1.OwnerReference{{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
				Name:       shoot.Name,
				UID:        shootUID,
			}}
			Expect(backupEntryInformer.Informer().GetStore().Update(backupEntry)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Add(seed)).To(Succeed())
			Expect(shootInformer.Informer().GetStore().Delete(shoot)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("shoot.core.gardener.cloud \"test-shoot\" not found"))
			Expect(ctxObjects).To(BeNil())

			By("project does not exist")
			Expect(shootInformer.Informer().GetStore().Add(shoot)).To(Succeed())
			Expect(projectInformer.Informer().GetStore().Delete(project)).To(Succeed())

			ctxObjects, err = r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("Project.core.gardener.cloud \"<unknown>\" not found"))
			Expect(ctxObjects).To(BeNil())
		})

		It("should fail to resolve with unsupported context object", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: "test/v1alpha1",
				Kind:       "Foo",
				Name:       "test-pod",
				UID:        types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c"),
			}

			ctxObjects, err := r.resolveContextObject(&seedUser, contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unsupported GVK for context object: test/v1alpha1, Kind=Foo"))
			Expect(ctxObjects).To(BeNil())
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
			}
			project = &gardencorev1beta1.Project{
				TypeMeta:   metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Project"},
				ObjectMeta: metav1.ObjectMeta{Name: projectName, UID: projectUID},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: ptr.To(namespaceName),
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
							Namespace:  ptr.To(namespaceName),
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
	})
})
