// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityapi "github.com/gardener/gardener/pkg/apis/security"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	keyUsageSig string = "sig"
)

type gardenerClaims struct {
	Gardener gardener `json:"gardener.cloud,omitempty"`
}

type gardener struct {
	WorkloadIdentity ref     `json:"workloadIdentity,omitempty"`
	Shoot            *ref    `json:"shoot,omitempty"`
	Project          *ref    `json:"project,omitempty"`
	Seed             *ref    `json:"seed,omitempty"`
	Garden           *garden `json:"garden,omitempty"`
}

type ref struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	UID       string `json:"uid,omitempty"`
}

type garden struct {
	ID string `json:"id,omitempty"`
}

// issueToken generates the JSON Web Token based on the provided configurations.
func (r *TokenRequestREST) issueToken(tokenRequest *securityapi.TokenRequest, workloadIdentity *securityapi.WorkloadIdentity) (string, *time.Time, error) {
	signer, err := getSigner(r.signingKey)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get signer: %w", err)
	}

	duration := tokenRequest.Spec.ExpirationSeconds
	if duration < r.minDuration {
		duration = r.minDuration
	} else if duration > r.maxDuration {
		duration = r.maxDuration
	}

	var (
		iat = time.Now()
		exp = iat.Add(time.Second * time.Duration(duration))
	)

	shoot, seed, project, err := r.resolveContextObject(tokenRequest.Spec.ContextObject)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve context object: %w", err)
	}

	standardClaims, gardenerClaims := r.getClaims(iat, exp, workloadIdentity, shoot, seed, project)
	token, err := jwt.Signed(signer).Claims(gardenerClaims).Claims(standardClaims).Serialize()
	if err != nil {
		return "", nil, fmt.Errorf("failed to issue JSON Web token: %w", err)
	}

	return token, &exp, nil
}

func (r *TokenRequestREST) getClaims(iat, exp time.Time, workloadIdentity *securityapi.WorkloadIdentity, shoot, seed, project metav1.Object) (*jwt.Claims, *gardenerClaims) {
	standardClaims := &jwt.Claims{
		Issuer:    r.issuer,
		Subject:   workloadIdentity.Status.Sub,
		Audience:  workloadIdentity.Spec.Audiences,
		IssuedAt:  jwt.NewNumericDate(iat),
		Expiry:    jwt.NewNumericDate(exp),
		NotBefore: jwt.NewNumericDate(iat),
	}

	gardenerClaims := &gardenerClaims{
		Gardener: gardener{
			WorkloadIdentity: ref{
				Name:      workloadIdentity.Name,
				Namespace: workloadIdentity.Namespace,
				UID:       string(workloadIdentity.UID),
			},
			Garden: &garden{
				ID: r.clusterIdentity,
			},
		},
	}

	if shoot != nil {
		gardenerClaims.Gardener.Shoot = &ref{
			Name:      shoot.GetName(),
			Namespace: shoot.GetNamespace(),
			UID:       string(shoot.GetUID()),
		}
	}

	if seed != nil {
		gardenerClaims.Gardener.Seed = &ref{
			Name: seed.GetName(),
			UID:  string(seed.GetUID()),
		}
	}

	if project != nil {
		gardenerClaims.Gardener.Project = &ref{
			Name: project.GetName(),
			UID:  string(project.GetUID()),
		}
	}

	return standardClaims, gardenerClaims
}

func (r *TokenRequestREST) resolveContextObject(ctxObj *securityapi.ContextObject) (metav1.Object, metav1.Object, metav1.Object, error) {
	if ctxObj == nil {
		return nil, nil, nil, nil
	}

	var (
		shoot   *gardencorev1beta1.Shoot
		seed    *gardencorev1beta1.Seed
		project *gardencorev1beta1.Project

		shootMeta, seedMeta, projectMeta metav1.Object
		err                              error
		coreInformers                    = r.coreInformerFactory.Core().V1beta1()
	)

	gvk := schema.FromAPIVersionAndKind(ctxObj.APIVersion, ctxObj.Kind)
	switch {
	case gvk.Group == gardencorev1beta1.SchemeGroupVersion.Group && gvk.Kind == "Shoot":
		if shoot, err = coreInformers.Shoots().Lister().Shoots(*ctxObj.Namespace).Get(ctxObj.Name); err != nil {
			return nil, nil, nil, err
		}
		shootMeta = shoot.GetObjectMeta()

		if shoot.UID != ctxObj.UID {
			return nil, nil, nil, fmt.Errorf("uid of contextObject (%s) and real world resource(%s) differ", ctxObj.UID, shoot.UID)
		}

		if shoot.Spec.SeedName != nil {
			seedName := *shoot.Spec.SeedName
			if seed, err = coreInformers.Seeds().Lister().Get(seedName); err != nil {
				return nil, nil, nil, err
			}
			seedMeta = seed.GetObjectMeta()
		}

		if project, err = gardenerutils.ProjectForNamespaceFromLister(coreInformers.Projects().Lister(), shoot.Namespace); err != nil {
			return nil, nil, nil, err
		}
		projectMeta = project.GetObjectMeta()

	case gvk.Group == gardencorev1beta1.SchemeGroupVersion.Group && gvk.Kind == "Seed":
		if seed, err = coreInformers.Seeds().Lister().Get(ctxObj.Name); err != nil {
			return nil, nil, nil, err
		}

		if seed.UID != ctxObj.UID {
			return nil, nil, nil, fmt.Errorf("uid of contextObject (%s) and real world resource(%s) differ", ctxObj.UID, seed.UID)
		}
		seedMeta = seed.GetObjectMeta()

	default:
		return nil, nil, nil, fmt.Errorf("unsupported GVK for context object: %s", gvk.String())
	}

	return shootMeta, seedMeta, projectMeta, nil
}

func getSigner(key any) (jose.Signer, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return getRSASigner(k)
	case *ecdsa.PrivateKey:
		return getECDSASigner(k)
	case jose.OpaqueSigner:
		return getOpaqueSigner(k)
	}

	return nil, fmt.Errorf("failed to construct signer from key type %T", key)
}

func getRSASigner(key *rsa.PrivateKey) (jose.Signer, error) {
	if key == nil {
		return nil, fmt.Errorf("rsa: key must not be nil")
	}

	keyID, err := getKeyID(key.Public())
	if err != nil {
		return nil, fmt.Errorf("rsa: failed to get key id: %w", err)
	}

	alg := jose.RS256
	sk := jose.SigningKey{
		Algorithm: alg,
		Key: jose.JSONWebKey{
			Algorithm: string(alg),
			Use:       keyUsageSig,
			Key:       key,
			KeyID:     keyID,
		},
	}

	return jose.NewSigner(sk, nil)
}

func getECDSASigner(key *ecdsa.PrivateKey) (jose.Signer, error) {
	if key == nil {
		return nil, fmt.Errorf("ecdsa: key must not be nil")
	}

	var alg jose.SignatureAlgorithm

	switch key.Curve {
	case elliptic.P256():
		alg = jose.ES256
	case elliptic.P384():
		alg = jose.ES384
	case elliptic.P521():
		alg = jose.ES512
	default:
		return nil, fmt.Errorf("ecdsa: failed to determine signature algorithm")
	}

	keyID, err := getKeyID(key.Public())
	if err != nil {
		return nil, fmt.Errorf("ecdsa: failed to get key id: %w", err)
	}

	sk := jose.SigningKey{
		Algorithm: alg,
		Key: jose.JSONWebKey{
			Algorithm: string(alg),
			Use:       keyUsageSig,
			Key:       key,
			KeyID:     keyID,
		},
	}

	return jose.NewSigner(sk, nil)
}

func getOpaqueSigner(key jose.OpaqueSigner) (jose.Signer, error) {
	alg := jose.SignatureAlgorithm(key.Public().Algorithm)

	sk := jose.SigningKey{
		Algorithm: alg,
		Key: jose.JSONWebKey{
			Algorithm: string(alg),
			Use:       keyUsageSig,
			Key:       key,
			KeyID:     key.Public().KeyID,
		},
	}

	return jose.NewSigner(sk, nil)
}

func getKeyID(publicKey crypto.PublicKey) (string, error) {
	marshaled, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	shaSum := sha256.Sum256(marshaled)
	id := base64.RawURLEncoding.EncodeToString(shaSum[:])

	return id, nil
}
