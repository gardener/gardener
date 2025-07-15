// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"fmt"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityapi "github.com/gardener/gardener/pkg/apis/security"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

type gardenerClaims struct {
	Gardener gardener `json:"gardener.cloud"`
}

type gardener struct {
	WorkloadIdentity ref  `json:"workloadIdentity"`
	Shoot            *ref `json:"shoot,omitempty"`
	Project          *ref `json:"project,omitempty"`
	Seed             *ref `json:"seed,omitempty"`
	BackupBucket     *ref `json:"backupBucket,omitempty"`
	BackupEntry      *ref `json:"backupEntry,omitempty"`
}

type ref struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty,omitzero"`
	UID       string `json:"uid"`
}

// contextObjects contains metadata information about the objects
// which are in the context of given WorkloadIdentity
type contextObjects struct {
	shoot        metav1.Object
	seed         metav1.Object
	project      metav1.Object
	backupBucket metav1.Object
	backupEntry  metav1.Object
}

// issueToken generates the JSON Web Token based on the provided configurations.
func (r *TokenRequestREST) issueToken(user user.Info, tokenRequest *securityapi.TokenRequest, workloadIdentity *securityapi.WorkloadIdentity) (string, *time.Time, error) {
	contextObjects, err := r.resolveContextObject(user, tokenRequest.Spec.ContextObject)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve context object: %w", err)
	}

	token, exp, err := r.tokenIssuer.IssueToken(
		workloadIdentity.Status.Sub,
		workloadIdentity.Spec.Audiences,
		tokenRequest.Spec.ExpirationSeconds,
		r.getGardenerClaims(workloadIdentity, contextObjects),
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed to issue JSON Web Token: %w", err)
	}

	return token, exp, nil
}

func (r *TokenRequestREST) getGardenerClaims(workloadIdentity *securityapi.WorkloadIdentity, ctxObjects *contextObjects) *gardenerClaims {
	gardenerClaims := &gardenerClaims{
		Gardener: gardener{
			WorkloadIdentity: ref{
				Name:      workloadIdentity.Name,
				Namespace: workloadIdentity.Namespace,
				UID:       string(workloadIdentity.UID),
			},
		},
	}

	if ctxObjects == nil {
		return gardenerClaims
	}

	if ctxObjects.shoot != nil {
		gardenerClaims.Gardener.Shoot = &ref{
			Name:      ctxObjects.shoot.GetName(),
			Namespace: ctxObjects.shoot.GetNamespace(),
			UID:       string(ctxObjects.shoot.GetUID()),
		}
	}

	if ctxObjects.seed != nil {
		gardenerClaims.Gardener.Seed = &ref{
			Name: ctxObjects.seed.GetName(),
			UID:  string(ctxObjects.seed.GetUID()),
		}
	}

	if ctxObjects.project != nil {
		gardenerClaims.Gardener.Project = &ref{
			Name: ctxObjects.project.GetName(),
			UID:  string(ctxObjects.project.GetUID()),
		}
	}

	if ctxObjects.backupBucket != nil {
		gardenerClaims.Gardener.BackupBucket = &ref{
			Name: ctxObjects.backupBucket.GetName(),
			UID:  string(ctxObjects.backupBucket.GetUID()),
		}
	}

	if ctxObjects.backupEntry != nil {
		gardenerClaims.Gardener.BackupEntry = &ref{
			Name:      ctxObjects.backupEntry.GetName(),
			Namespace: ctxObjects.backupEntry.GetNamespace(),
			UID:       string(ctxObjects.backupEntry.GetUID()),
		}
	}

	return gardenerClaims
}

func (r *TokenRequestREST) resolveContextObject(user user.Info, ctxObj *securityapi.ContextObject) (*contextObjects, error) {
	if ctxObj == nil {
		return nil, nil
	}

	if !slices.Contains(user.GetGroups(), v1beta1constants.SeedsGroup) {
		return nil, nil
	}

	var (
		ctxObjects   = &contextObjects{}
		shoot        *gardencorev1beta1.Shoot
		seed         *gardencorev1beta1.Seed
		project      *gardencorev1beta1.Project
		backupBucket *gardencorev1beta1.BackupBucket
		backupEntry  *gardencorev1beta1.BackupEntry

		err error
	)

	switch gvk := schema.FromAPIVersionAndKind(ctxObj.APIVersion, ctxObj.Kind); {
	case gvk.Group == gardencorev1beta1.SchemeGroupVersion.Group && gvk.Kind == "Shoot":
		if shoot, err = r.shootListers.Shoots(*ctxObj.Namespace).Get(ctxObj.Name); err != nil {
			return nil, err
		}
		ctxObjects.shoot = shoot.GetObjectMeta()

		if shoot.UID != ctxObj.UID {
			return nil, fmt.Errorf("uid of contextObject (%s) and real world Shoot resource (%s) differ", ctxObj.UID, shoot.UID)
		}

		if shoot.Spec.SeedName != nil {
			seedName := *shoot.Spec.SeedName
			if seed, err = r.seedLister.Get(seedName); err != nil {
				return nil, err
			}
			ctxObjects.seed = seed.GetObjectMeta()
		}

		if project, err = admissionutils.ProjectForNamespaceFromLister(r.projectLister, shoot.Namespace); err != nil {
			return nil, err
		}
		ctxObjects.project = project.GetObjectMeta()

	case gvk.Group == gardencorev1beta1.SchemeGroupVersion.Group && gvk.Kind == "Seed":
		if seed, err = r.seedLister.Get(ctxObj.Name); err != nil {
			return nil, err
		}

		if seed.UID != ctxObj.UID {
			return nil, fmt.Errorf("uid of contextObject (%s) and real world Seed resource (%s) differ", ctxObj.UID, seed.UID)
		}
		ctxObjects.seed = seed.GetObjectMeta()

	case gvk.Group == gardencorev1beta1.SchemeGroupVersion.Group && gvk.Kind == "BackupBucket":
		if backupBucket, err = r.backupBucketLister.Get(ctxObj.Name); err != nil {
			return nil, err
		}

		if backupBucket.UID != ctxObj.UID {
			return nil, fmt.Errorf("uid of contextObject (%s) and real world BackupBucket resource (%s) differ", ctxObj.UID, backupBucket.UID)
		}
		ctxObjects.backupBucket = backupBucket.GetObjectMeta()

		if backupBucket.Spec.SeedName != nil {
			seedName := *backupBucket.Spec.SeedName
			if seed, err = r.seedLister.Get(seedName); err != nil {
				return nil, err
			}
			ctxObjects.seed = seed.GetObjectMeta()
		}

	case gvk.Group == gardencorev1beta1.SchemeGroupVersion.Group && gvk.Kind == "BackupEntry":
		if backupEntry, err = r.backupEntryLister.BackupEntries(*ctxObj.Namespace).Get(ctxObj.Name); err != nil {
			return nil, err
		}

		if backupEntry.UID != ctxObj.UID {
			return nil, fmt.Errorf("uid of contextObject (%s) and real world BackupEntry resource (%s) differ", ctxObj.UID, backupEntry.UID)
		}
		ctxObjects.backupEntry = backupEntry.GetObjectMeta()

		if backupBucket, err = r.backupBucketLister.Get(backupEntry.Spec.BucketName); err != nil {
			return nil, err
		}
		ctxObjects.backupBucket = backupBucket.GetObjectMeta()

		if backupEntry.Spec.SeedName != nil {
			seedName := *backupEntry.Spec.SeedName
			if seed, err = r.seedLister.Get(seedName); err != nil {
				return nil, err
			}
			ctxObjects.seed = seed.GetObjectMeta()
		}

		if shootName := gardenerutils.GetShootNameFromOwnerReferences(backupEntry); shootName != "" {
			shoot, err := r.shootListers.Shoots(backupEntry.GetNamespace()).Get(shootName)
			if err == nil {
				shootTechnicalID, shootUID := gardenerutils.ExtractShootDetailsFromBackupEntryName(ctxObj.Name)
				if shootTechnicalID == shoot.Status.TechnicalID && shootUID == shoot.GetUID() {
					ctxObjects.shoot = shoot.GetObjectMeta()

					if project, err = admissionutils.ProjectForNamespaceFromLister(r.projectLister, shoot.Namespace); err != nil {
						return nil, err
					}
					ctxObjects.project = project.GetObjectMeta()
				}
			} else if !apierrors.IsNotFound(err) {
				return nil, err
			}
		}

	default:
		return nil, fmt.Errorf("unsupported GVK for context object: %s", gvk.String())
	}

	return ctxObjects, nil
}
