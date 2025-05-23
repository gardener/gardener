// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"fmt"
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityapi "github.com/gardener/gardener/pkg/apis/security"
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
}

type ref struct {
	Name      string  `json:"name"`
	Namespace *string `json:"namespace,omitempty"`
	UID       string  `json:"uid"`
}

// issueToken generates the JSON Web Token based on the provided configurations.
func (r *TokenRequestREST) issueToken(user user.Info, tokenRequest *securityapi.TokenRequest, workloadIdentity *securityapi.WorkloadIdentity) (string, *time.Time, error) {
	shoot, seed, project, err := r.resolveContextObject(user, tokenRequest.Spec.ContextObject)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve context object: %w", err)
	}

	token, exp, err := r.tokenIssuer.IssueToken(
		workloadIdentity.Status.Sub,
		workloadIdentity.Spec.Audiences,
		tokenRequest.Spec.ExpirationSeconds,
		r.getGardenerClaims(workloadIdentity, shoot, seed, project),
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed to issue JSON Web Token: %w", err)
	}

	return token, exp, nil
}

func (r *TokenRequestREST) getGardenerClaims(workloadIdentity *securityapi.WorkloadIdentity, shoot, seed, project metav1.Object) *gardenerClaims {
	gardenerClaims := &gardenerClaims{
		Gardener: gardener{
			WorkloadIdentity: ref{
				Name:      workloadIdentity.Name,
				Namespace: ptr.To(workloadIdentity.Namespace),
				UID:       string(workloadIdentity.UID),
			},
		},
	}

	if shoot != nil {
		gardenerClaims.Gardener.Shoot = &ref{
			Name:      shoot.GetName(),
			Namespace: ptr.To(shoot.GetNamespace()),
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

	return gardenerClaims
}

func (r *TokenRequestREST) resolveContextObject(user user.Info, ctxObj *securityapi.ContextObject) (metav1.Object, metav1.Object, metav1.Object, error) {
	if ctxObj == nil {
		return nil, nil, nil, nil
	}

	if !slices.Contains(user.GetGroups(), v1beta1constants.SeedsGroup) {
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

	switch gvk := schema.FromAPIVersionAndKind(ctxObj.APIVersion, ctxObj.Kind); {
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

		if project, err = admissionutils.ProjectForNamespaceFromLister(coreInformers.Projects().Lister(), shoot.Namespace); err != nil {
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
