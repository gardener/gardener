// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/keyutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityapi "github.com/gardener/gardener/pkg/apis/security"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var (
	rsaPrivateKey   *rsa.PrivateKey
	ecdsaPrivateKey *ecdsa.PrivateKey
)

var _ = BeforeSuite(func() {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 4096)
	Expect(err).ToNot(HaveOccurred())
	rsaPrivateKey = rsaKey

	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	Expect(err).ToNot(HaveOccurred())
	ecdsaPrivateKey = ecdsaKey
})

var _ = Describe("#JWT", func() {
	Context("#getClaims", func() {
		var (
			issuer          = "https://test.local.gardener.cloud"
			clusterIdentity = "test-local-gardener-cloud"
			minDuration     = time.Minute * 10
			maxDuration     = time.Hour * 48
			r               = TokenRequestREST{
				issuer:          issuer,
				clusterIdentity: clusterIdentity,
				minDuration:     int64(minDuration.Seconds()),
				maxDuration:     int64(maxDuration.Seconds()),
			}
			workloadName      = "identity"
			workloadNamespace = "garden-local"
			workloadUID       = "ab920696-dd12-4723-9bc1-204cfe9edd40"
			sub               = "gardener.cloud:workloadidentity:" + workloadNamespace + ":" + workloadName + ":" + workloadUID
			aud               = "gardener.cloud"

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
		)

		BeforeEach(func() {
			r.signingKey = rsaPrivateKey
		})

		DescribeTable("#getClaims",
			func(objType, name, namespace, uid string) {
				Expect(objType).To(BeElementOf("shoot", "seed", "project", "none"))

				var shoot, seed, project metav1.Object
				switch objType {
				case "shoot":
					Expect(name).ToNot(BeEmpty())
					Expect(namespace).ToNot(BeEmpty())
					Expect(uid).ToNot(BeEmpty())
					shoot = &metav1.ObjectMeta{Namespace: namespace, Name: name, UID: types.UID(uid)}

				case "seed":
					Expect(name).ToNot(BeEmpty())
					Expect(namespace).To(BeEmpty())
					Expect(uid).ToNot(BeEmpty())
					seed = &metav1.ObjectMeta{Name: name, UID: types.UID(uid)}

				case "project":
					Expect(name).ToNot(BeEmpty())
					Expect(namespace).To(BeEmpty())
					Expect(uid).ToNot(BeEmpty())
					project = &metav1.ObjectMeta{Name: name, UID: types.UID(uid)}

				case "none":
					Expect(name).To(BeEmpty())
					Expect(namespace).To(BeEmpty())
					Expect(uid).To(BeEmpty())
				}

				now := time.Now()
				exp := now.Add(time.Hour)

				expected := jwt.Expected{
					Issuer:      issuer,
					Subject:     sub,
					Time:        now,
					AnyAudience: jwt.Audience{aud},
				}

				s, g := r.getClaims(now, exp, workloadIdentity, shoot, seed, project)

				Expect(s).ToNot(BeNil())
				Expect(s.Issuer).To(Equal(issuer))
				Expect(s.Subject).To(Equal(sub))
				Expect(s.Audience).To(BeEquivalentTo([]string{aud}))
				Expect(s.IssuedAt.Time().Unix()).To(Equal(now.Unix()))
				Expect(s.NotBefore.Time().Unix()).To(Equal(now.Unix()))
				Expect(s.Expiry.Time().Unix()).To(Equal(exp.Unix()))
				Expect(s.Validate(expected)).ToNot(HaveOccurred())

				Expect(g).ToNot(BeNil())
				Expect(g.Gardener.WorkloadIdentity.Name).To(Equal(workloadName))
				Expect(g.Gardener.WorkloadIdentity.Namespace).To(Equal(workloadNamespace))
				Expect(g.Gardener.WorkloadIdentity.UID).To(Equal(workloadUID))
				Expect(g.Gardener.Garden).ToNot(BeNil())
				Expect(g.Gardener.Garden.ID).To(Equal(clusterIdentity))

				if shoot != nil {
					Expect(g.Gardener.Shoot).ToNot(BeNil())
					Expect(g.Gardener.Shoot.Name).To(Equal(shoot.GetName()))
					Expect(g.Gardener.Shoot.Namespace).To(Equal(shoot.GetNamespace()))
					Expect(g.Gardener.Shoot.UID).To(BeEquivalentTo(shoot.GetUID()))
				} else {
					Expect(g.Gardener.Shoot).To(BeNil())
				}

				if seed != nil {
					Expect(g.Gardener.Seed).ToNot(BeNil())
					Expect(g.Gardener.Seed.Name).To(Equal(seed.GetName()))
					Expect(g.Gardener.Seed.UID).To(BeEquivalentTo(seed.GetUID()))
					Expect(g.Gardener.Seed.Namespace).To(BeEmpty())
				} else {
					Expect(g.Gardener.Seed).To(BeNil())
				}

				if project != nil {
					Expect(g.Gardener.Project).ToNot(BeNil())
					Expect(g.Gardener.Project.Name).To(Equal(project.GetName()))
					Expect(g.Gardener.Project.UID).To(BeEquivalentTo(project.GetUID()))
					Expect(g.Gardener.Project.Namespace).To(BeEmpty())
				} else {
					Expect(g.Gardener.Project).To(BeNil())
				}
			},
			Entry("should successfully set common claims", "none", "", "", ""),
			Entry("should successfully set shoot claims", "shoot", "local", "garden-local", "d9bd264e-2192-437f-986a-94fcd8cf5d8a"),
			Entry("should successfully set seed claims", "seed", "local-seed", "", "857afd63-16eb-456a-9d35-c2f7d1578d32"),
			Entry("should successfully set project claims", "project", "local-seed", "", "34fecdd7-2694-47ba-a75a-0515d4fa0686"),
		)
	})

	Context("#getKeyID", func() {
		var (
			pubKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAySXbkmrd0VD+L24TilvW
wzwAf/M7SpXgn4FTc2pe5XbOAq2CU+rWAVPLEW8oRtGW9F4uenbugiB0usRA+aYW
b8JwsEKRoxpaeKzqPg4P+QL5t/aHsu4Vh9dxk7hEYSNaKZpEOhlJHARk4pPvqx5R
uCKk7csM19Tg2v9ustk6IK5PVieoSmEA55B5iKe6tBAH5IR2Qu2NjvdONcXPGP8Y
ujOpwVgXG82EJLTtbehyZOUjC801g2vxKyo0rdAx483kUBqDLW/GZfYK5Y+ZV5ty
Jc7N1Tp94TBXpmsw0KIMz1gjRtbQcDJpntWAcIRhQ9OyEWpfVW+NZoj8wstqU0pB
wQIDAQAB
-----END PUBLIC KEY-----`
			pubKeyID = "vyA3RTKakSnyzQG6KWl41qjwe-aXFPrjFTdesc3kZLk"
		)

		It("should correctly calculate key id", func() {
			keys, err := keyutil.ParsePublicKeysPEM([]byte(pubKey))
			Expect(err).ToNot(HaveOccurred())
			Expect(keys).To(HaveLen(1))

			keyID, err := getKeyID(keys[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(keyID).ToNot(BeEmpty())
			Expect(keyID).To(Equal(pubKeyID))
		})

		It("should provide unique key IDs", func() {
			k1, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())

			k2, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())

			Expect(k1.Equal(k2)).To(BeFalse())
			Expect(k2.Equal(k1)).To(BeFalse())

			id1, err := getKeyID(k1.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(id1).ToNot(BeEmpty())
			id2, err := getKeyID(k2.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(id2).ToNot(BeEmpty())

			Expect(id1).ToNot(Equal(pubKeyID))
			Expect(id2).ToNot(Equal(pubKeyID))
			Expect(id1).ToNot((Equal(id2)))
		})

		It("should be idempotent", func() {
			id1, err := getKeyID(rsaPrivateKey.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(id1).ToNot(BeEmpty())

			id2, err := getKeyID(rsaPrivateKey.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(id2).ToNot(BeEmpty())

			Expect(id1).To(Equal(id2))
		})

		It("should fail to get key ID for unsupported key", func() {
			type unsupportedKey struct{}

			var _ crypto.PublicKey = unsupportedKey{}
			u := unsupportedKey{}

			keyID, err := getKeyID(u)
			Expect(err).To(HaveOccurred())
			Expect(keyID).To(BeEmpty())
		})
	})

	Context("#getSigner", func() {
		It("should get RSA signer", func() {
			s, err := getSigner(rsaPrivateKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(s).ToNot(BeNil())
		})

		It("should get ECDSA signer", func() {
			s, err := getSigner(ecdsaPrivateKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(s).ToNot(BeNil())
		})

		It("should fail for unsupported key", func() {
			type unsupportedKey struct{}
			s, err := getSigner(unsupportedKey{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to construct signer from key type storage.unsupportedKey"))
			Expect(s).To(BeNil())
		})

		It("should fail for nil key", func() {
			signer, err := getRSASigner(nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("rsa: key must not be nil"))
			Expect(signer).To(BeNil())

			signer, err = getECDSASigner(nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("ecdsa: key must not be nil"))
			Expect(signer).To(BeNil())
		})
	})

	Context("#resolveContextObject", func() {
		var (
			r             = TokenRequestREST{}
			shootName     = "test-shoot"
			shootUID      = types.UID("9a134d22-dd61-4845-951e-9a20bde1648a")
			projectName   = "test-project"
			projectUID    = types.UID("01c9c6fa-2b8b-496f-8edf-e382f4d61905")
			namespaceName = "garden-" + projectName
			seedName      = "test-seed"
			seedUID       = types.UID("aa88fb5a-28ef-4987-8c2d-9f02afafcf09")

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
					Namespace: &namespaceName,
				},
			}
		)

		BeforeEach(func() {
			informerFactory := gardencoreinformers.NewSharedInformerFactory(nil, 0)

			err := informerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)
			Expect(err).ToNot(HaveOccurred())
			err = informerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)
			Expect(err).ToNot(HaveOccurred())
			err = informerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)
			Expect(err).ToNot(HaveOccurred())

			r.coreInformerFactory = informerFactory
		})

		It("should successfully resolve when the context object is nil", func() {
			shoot, seed, project, err := r.resolveContextObject(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(shoot).To(BeNil())
			Expect(seed).To(BeNil())
			Expect(project).To(BeNil())
		})

		It("should successfully resolve when the context object is shoot", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
				Name:       shootName,
				Namespace:  &namespaceName,
				UID:        shootUID,
			}

			By("Resolve context object before the shoot is scheduled to a seed")
			rShoot, rSeed, rProject, err := r.resolveContextObject(contextObject)
			Expect(err).ToNot(HaveOccurred())

			Expect(rShoot).ToNot(BeNil())
			Expect(rShoot.GetName()).To(Equal(shootName))
			Expect(rShoot.GetNamespace()).To(Equal(namespaceName))
			Expect(rShoot.GetUID()).To(Equal(shootUID))

			Expect(rSeed).To(BeNil())

			Expect(rProject).ToNot(BeNil())
			Expect(rProject.GetName()).To(Equal(projectName))
			Expect(rProject.GetUID()).To(Equal(projectUID))

			By("Schedule the shoot to a seed")
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec = gardencorev1beta1.ShootSpec{SeedName: &seedName}
			err = r.coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Update(shootCopy)
			Expect(err).ToNot(HaveOccurred())

			By("Resolve context object after the shoot is scheduled to a seed")
			rShoot, rSeed, rProject, err = r.resolveContextObject(contextObject)
			Expect(err).ToNot(HaveOccurred())

			Expect(rShoot).ToNot(BeNil())
			Expect(rShoot.GetName()).To(Equal(shootName))
			Expect(rShoot.GetNamespace()).To(Equal(namespaceName))
			Expect(rShoot.GetUID()).To(Equal(shootUID))

			Expect(rSeed).ToNot(BeNil())
			Expect(rSeed.GetName()).To(Equal(seedName))
			Expect(rSeed.GetUID()).To(Equal(seedUID))

			Expect(rProject).ToNot(BeNil())
			Expect(rProject.GetName()).To(Equal(projectName))
			Expect(rProject.GetUID()).To(Equal(projectUID))
		})

		It("should fail to resolve with shoot context object", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
				Name:       shootName,
				Namespace:  &namespaceName,
				UID:        shootUID,
			}

			By("shoot does not exist")
			err := r.coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Delete(shoot)
			Expect(err).ToNot(HaveOccurred())

			rShoot, rSeed, rProject, err := r.resolveContextObject(contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("shoot.core.gardener.cloud \"test-shoot\" not found"))

			Expect(rShoot).To(BeNil())
			Expect(rSeed).To(BeNil())
			Expect(rProject).To(BeNil())

			By("shoot and context uid does not match")
			err = r.coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)
			Expect(err).ToNot(HaveOccurred())

			uidMismatch := types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")
			uidMismatchContext := contextObject.DeepCopy()
			uidMismatchContext.UID = uidMismatch
			rShoot, rSeed, rProject, err = r.resolveContextObject(uidMismatchContext)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("uid of contextObject (" + uidMismatch + ") and real world resource(" + shootUID + ") differ"))

			Expect(rShoot).To(BeNil())
			Expect(rSeed).To(BeNil())
			Expect(rProject).To(BeNil())

			By("seed does not exist")
			shoot.Spec = gardencorev1beta1.ShootSpec{
				SeedName: &seedName,
			}
			err = r.coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Update(shoot)
			Expect(err).ToNot(HaveOccurred())
			err = r.coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Delete(seed)
			Expect(err).ToNot(HaveOccurred())

			rShoot, rSeed, rProject, err = r.resolveContextObject(contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("seed.core.gardener.cloud \"test-seed\" not found"))

			Expect(rShoot).To(BeNil())
			Expect(rSeed).To(BeNil())
			Expect(rProject).To(BeNil())

			By("project does not exist")
			err = r.coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)
			Expect(err).ToNot(HaveOccurred())
			err = r.coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Delete(project)
			Expect(err).ToNot(HaveOccurred())

			rShoot, rSeed, rProject, err = r.resolveContextObject(contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("Project.core.gardener.cloud \"<unknown>\" not found"))

			Expect(rShoot).To(BeNil())
			Expect(rSeed).To(BeNil())
			Expect(rProject).To(BeNil())

		})

		It("should successfully resolve when context object is seed", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Seed",
				Name:       seedName,
				UID:        seedUID,
			}

			rShoot, rSeed, rProject, err := r.resolveContextObject(contextObject)
			Expect(err).ToNot(HaveOccurred())

			Expect(rShoot).To(BeNil())
			Expect(rProject).To(BeNil())

			Expect(rSeed).ToNot(BeNil())
			Expect(rSeed.GetName()).To(Equal(seedName))
			Expect(rSeed.GetUID()).To(Equal(seedUID))
		})

		It("should fail to resolve with seed context object", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Seed",
				Name:       seedName,
				UID:        seedUID,
			}

			By("seed does not exist")
			err := r.coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Delete(seed)
			Expect(err).ToNot(HaveOccurred())

			rShoot, rSeed, rProject, err := r.resolveContextObject(contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(err.Error()).To(Equal("seed.core.gardener.cloud \"test-seed\" not found"))

			Expect(rShoot).To(BeNil())
			Expect(rSeed).To(BeNil())
			Expect(rProject).To(BeNil())

			By("seed and context uid does not match")
			err = r.coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)
			Expect(err).ToNot(HaveOccurred())

			uidMismatch := types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c")
			uidMismatchContext := contextObject.DeepCopy()
			uidMismatchContext.UID = uidMismatch
			rShoot, rSeed, rProject, err = r.resolveContextObject(uidMismatchContext)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("uid of contextObject (" + uidMismatch + ") and real world resource(" + seedUID + ") differ"))

			Expect(rShoot).To(BeNil())
			Expect(rSeed).To(BeNil())
			Expect(rProject).To(BeNil())
		})

		It("should fail to resolve with unsupported context object", func() {
			contextObject := &securityapi.ContextObject{
				APIVersion: "test/v1alpha1",
				Kind:       "Foo",
				Name:       "test-pod",
				UID:        types.UID("18dd0733-3c9e-4587-a8ae-d6fa5daf460c"),
			}

			rShoot, rSeed, rProject, err := r.resolveContextObject(contextObject)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unsupported GVK for context object: test/v1alpha1, Kind=Foo"))
			Expect(rShoot).To(BeNil())
			Expect(rSeed).To(BeNil())
			Expect(rProject).To(BeNil())
		})
	})

	Context("#issueToken", func() {
		var (
			minDuration = time.Minute * 10
			maxDuration = time.Hour * 48
			r           = TokenRequestREST{
				issuer:          "https://test.local.gardener.cloud",
				clusterIdentity: "test-local-gardener-cloud",
				minDuration:     int64(minDuration.Seconds()),
				maxDuration:     int64(maxDuration.Seconds()),
			}
			workloadName      = "identity"
			workloadNamespace = "garden-local"
			workloadUID       = "ab920696-dd12-4723-9bc1-204cfe9edd40"
			sub               = "gardener.cloud:workloadidentity:" + workloadNamespace + ":" + workloadName + ":" + workloadUID
			aud               = "gardener.cloud"

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

			shootName     = "test-shoot"
			shootUID      = types.UID("9a134d22-dd61-4845-951e-9a20bde1648a")
			projectName   = "test-project"
			projectUID    = types.UID("01c9c6fa-2b8b-496f-8edf-e382f4d61905")
			namespaceName = "garden-" + projectName
			seedName      = "test-seed"
			seedUID       = types.UID("aa88fb5a-28ef-4987-8c2d-9f02afafcf09")

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
					Namespace: &namespaceName,
				},
			}
		)

		BeforeEach(func() {
			informerFactory := gardencoreinformers.NewSharedInformerFactory(nil, 0)

			err := informerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)
			Expect(err).ToNot(HaveOccurred())
			err = informerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)
			Expect(err).ToNot(HaveOccurred())
			err = informerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)
			Expect(err).ToNot(HaveOccurred())

			r.coreInformerFactory = informerFactory
			r.signingKey = rsaPrivateKey
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
							Namespace:  &namespaceName,
							UID:        shootUID,
						},
						ExpirationSeconds: int64(3600),
					},
				}
			)
			token, exp, err := r.issueToken(tokenRequest, workloadIdentity)

			Expect(err).ToNot(HaveOccurred())
			Expect(exp).ToNot(BeNil())
			Expect(exp.After(now)).To(BeTrue())
			Expect(token).ToNot(BeEmpty())
		})
	})
})
