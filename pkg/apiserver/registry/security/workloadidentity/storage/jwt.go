// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"fmt"
	"time"

	"gopkg.in/square/go-jose.v2/jwt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	tokenissuer "k8s.io/kubernetes/pkg/serviceaccount"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityapi "github.com/gardener/gardener/pkg/apis/security"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

type privateClaims struct {
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

// generateToken generates the JSON Web Token based on the provided configurations.
func (r *TokenRequestREST) generateToken(workloadIdentity *securityapi.WorkloadIdentity, iat, exp time.Time, shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed, project *gardencorev1beta1.Project) (string, error) {
	generator, err := tokenissuer.JWTTokenGenerator(r.issuer, r.signingKey)
	if err != nil {
		return "", err
	}

	standardClaims := jwt.Claims{
		Subject:   workloadIdentity.Status.Sub,
		Audience:  workloadIdentity.Spec.Audiences,
		Expiry:    jwt.NewNumericDate(exp),
		NotBefore: jwt.NewNumericDate(iat),
		IssuedAt:  jwt.NewNumericDate(iat),
	}

	pc := &privateClaims{
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
		pc.Gardener.Shoot = &ref{
			Name:      shoot.Name,
			Namespace: shoot.Namespace,
			UID:       string(shoot.UID),
		}
	}

	if seed != nil {
		pc.Gardener.Seed = &ref{
			Name: seed.Name,
			UID:  string(seed.UID),
		}
	}

	if project != nil {
		pc.Gardener.Project = &ref{
			Name:      project.Name,
			Namespace: project.Namespace,
			UID:       string(project.UID),
		}
	}

	return generator.GenerateToken(&standardClaims, pc)
}

func (r *TokenRequestREST) resolveContextObject(ctxObj *securityapi.ContextObject) (*gardencorev1beta1.Shoot, *gardencorev1beta1.Seed, *gardencorev1beta1.Project, error) {
	if ctxObj == nil {
		return nil, nil, nil, nil
	}

	var (
		shoot         *gardencorev1beta1.Shoot
		seed          *gardencorev1beta1.Seed
		project       *gardencorev1beta1.Project
		err           error
		coreInterface = r.coreInformerFactory.Core().V1beta1()
	)

	gvk := schema.FromAPIVersionAndKind(ctxObj.APIVersion, ctxObj.Kind)
	switch {
	case gvk.Group == gardencorev1beta1.SchemeGroupVersion.Group && gvk.Kind == "Shoot":
		if shoot, err = coreInterface.Shoots().Lister().Shoots(*ctxObj.Namespace).Get(ctxObj.Name); err != nil {
			return nil, nil, nil, err
		}

		if shoot.UID != ctxObj.UID {
			return nil, nil, nil, fmt.Errorf("uid of contextObject (%s) and real world resource(%s) differ", ctxObj.UID, shoot.UID)
		}

		if shoot.Spec.SeedName != nil {
			seedName := *shoot.Spec.SeedName
			if seed, err = coreInterface.Seeds().Lister().Get(seedName); err != nil {
				return nil, nil, nil, err
			}
		}

		if project, err = admissionutils.ProjectForNamespaceFromLister(coreInterface.Projects().Lister(), shoot.Namespace); err != nil {
			return nil, nil, nil, err
		}

	case gvk.Group == gardencorev1beta1.SchemeGroupVersion.Group && gvk.Kind == "Seed":
		if seed, err = coreInterface.Seeds().Lister().Get(ctxObj.Name); err != nil {
			return nil, nil, nil, err
		}

		if seed.UID != ctxObj.UID {
			return nil, nil, nil, fmt.Errorf("uid of contextObject (%s) and real world resource(%s) differ", ctxObj.UID, seed.UID)
		}

	default:
		return nil, nil, nil, fmt.Errorf("cannot issue workload identity token in the context type: %q", gvk.String())
	}

	return shoot, seed, project, nil
}
