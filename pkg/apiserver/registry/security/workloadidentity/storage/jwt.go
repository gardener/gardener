// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"errors"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/utils/ptr"

	seedidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/seed"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityapi "github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
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
	contextObjects, err := r.resolveContextObject(user, tokenRequest.Spec.ContextObject, workloadIdentity)
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

func (r *TokenRequestREST) resolveContextObject(user user.Info, ctxObj *securityapi.ContextObject, workloadIdentity *securityapi.WorkloadIdentity) (*contextObjects, error) {
	if ctxObj == nil {
		return nil, nil
	}

	if !slices.Contains(user.GetGroups(), v1beta1constants.SeedsGroup) {
		return nil, nil
	}

	requesterSeedName, isSeed, _ := seedidentity.FromUserInfoInterface(user)
	if !isSeed {
		return nil, apierrors.NewForbidden(securityapi.Resource("workloadidentities"), "", errors.New("cannot determine the seed of the requester"))
	}

	// The context claims are only trustworthy if they are bound to the requester, hence context objects not related to
	// the requesting seed are rejected. This check must happen before comparing UIDs to avoid disclosing them.
	notRelatedToRequester := func() error {
		return apierrors.NewForbidden(securityapi.Resource("workloadidentities"), "", fmt.Errorf("context object %s is not related to seed %q", ctxObjectKey(ctxObj), requesterSeedName))
	}
	isAssignedToRequester := func(seedNames ...*string) bool {
		return slices.ContainsFunc(seedNames, func(s *string) bool { return ptr.Deref(s, "") == requesterSeedName })
	}
	// The context claims must not attribute a workload identity to an object which does not use it, e.g., to a shoot of
	// another project which is scheduled to the same seed.
	notReferencingWorkloadIdentity := func() error {
		return apierrors.NewForbidden(securityapi.Resource("workloadidentities"), workloadIdentity.Name, fmt.Errorf("workload identity %s/%s is not referenced by context object %s", workloadIdentity.Namespace, workloadIdentity.Name, ctxObjectKey(ctxObj)))
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

		// Accepting the status seed name allows the source seed to request tokens during control plane migration.
		if !isAssignedToRequester(shoot.Spec.SeedName, shoot.Status.SeedName) {
			return nil, notRelatedToRequester()
		}
		ctxObjects.shoot = shoot.GetObjectMeta()

		if shoot.UID != ctxObj.UID {
			return nil, fmt.Errorf("uid of contextObject (%s) and real world Shoot resource (%s) differ", ctxObj.UID, shoot.UID)
		}

		// Shoots may also use workload identities provided by their seed, e.g., for the internal or default domain.
		var referenced bool
		if referenced, err = r.shootReferencesWorkloadIdentity(shoot, workloadIdentity); err != nil {
			return nil, err
		}
		if !referenced {
			var requesterSeed *gardencorev1beta1.Seed
			if requesterSeed, err = r.seedLister.Get(requesterSeedName); err != nil {
				return nil, err
			}
			if !seedReferencesWorkloadIdentity(requesterSeed, workloadIdentity) {
				return nil, notReferencingWorkloadIdentity()
			}
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
		if ctxObj.Name != requesterSeedName {
			return nil, notRelatedToRequester()
		}

		if seed, err = r.seedLister.Get(ctxObj.Name); err != nil {
			return nil, err
		}

		if seed.UID != ctxObj.UID {
			return nil, fmt.Errorf("uid of contextObject (%s) and real world Seed resource (%s) differ", ctxObj.UID, seed.UID)
		}

		if !seedReferencesWorkloadIdentity(seed, workloadIdentity) {
			return nil, notReferencingWorkloadIdentity()
		}
		ctxObjects.seed = seed.GetObjectMeta()

	case gvk.Group == gardencorev1beta1.SchemeGroupVersion.Group && gvk.Kind == "BackupBucket":
		if backupBucket, err = r.backupBucketLister.Get(ctxObj.Name); err != nil {
			return nil, err
		}

		if !isAssignedToRequester(backupBucket.Spec.SeedName) {
			return nil, notRelatedToRequester()
		}

		if backupBucket.UID != ctxObj.UID {
			return nil, fmt.Errorf("uid of contextObject (%s) and real world BackupBucket resource (%s) differ", ctxObj.UID, backupBucket.UID)
		}

		if !isWorkloadIdentityObjectReference(backupBucket.Spec.CredentialsRef, workloadIdentity) {
			return nil, notReferencingWorkloadIdentity()
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

		// Accepting the status seed name allows the source seed to request tokens during control plane migration.
		if !isAssignedToRequester(backupEntry.Spec.SeedName, backupEntry.Status.SeedName) {
			return nil, notRelatedToRequester()
		}

		if backupEntry.UID != ctxObj.UID {
			return nil, fmt.Errorf("uid of contextObject (%s) and real world BackupEntry resource (%s) differ", ctxObj.UID, backupEntry.UID)
		}
		ctxObjects.backupEntry = backupEntry.GetObjectMeta()

		if backupBucket, err = r.backupBucketLister.Get(backupEntry.Spec.BucketName); err != nil {
			return nil, err
		}

		// BackupEntries use the credentials of the BackupBucket they are stored in.
		if !isWorkloadIdentityObjectReference(backupBucket.Spec.CredentialsRef, workloadIdentity) {
			return nil, notReferencingWorkloadIdentity()
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

func ctxObjectKey(ctxObj *securityapi.ContextObject) string {
	if ns := ptr.Deref(ctxObj.Namespace, ""); ns != "" {
		return fmt.Sprintf("%s %s/%s", ctxObj.Kind, ns, ctxObj.Name)
	}
	return fmt.Sprintf("%s %s", ctxObj.Kind, ctxObj.Name)
}

func isWorkloadIdentityReference(apiVersion, kind, namespace, name string, workloadIdentity *securityapi.WorkloadIdentity) bool {
	return apiVersion == securityv1alpha1.SchemeGroupVersion.String() &&
		kind == "WorkloadIdentity" &&
		namespace == workloadIdentity.Namespace &&
		name == workloadIdentity.Name
}

func isWorkloadIdentityObjectReference(ref *corev1.ObjectReference, workloadIdentity *securityapi.WorkloadIdentity) bool {
	return ref != nil && isWorkloadIdentityReference(ref.APIVersion, ref.Kind, ref.Namespace, ref.Name, workloadIdentity)
}

// seedReferencesWorkloadIdentity mirrors the Seed -> WorkloadIdentity edges of the SeedAuthorizer graph
// (pkg/utils/graph/eventhandler_seed.go). Keep both in sync when adding new credentials references to the Seed API.
func seedReferencesWorkloadIdentity(seed *gardencorev1beta1.Seed, workloadIdentity *securityapi.WorkloadIdentity) bool {
	if seed.Spec.Backup != nil && isWorkloadIdentityObjectReference(seed.Spec.Backup.CredentialsRef, workloadIdentity) {
		return true
	}

	if provider := seed.Spec.DNS.Provider; provider != nil && isWorkloadIdentityObjectReference(provider.CredentialsRef, workloadIdentity) {
		return true
	}

	if internal := seed.Spec.DNS.Internal; internal != nil && isWorkloadIdentityObjectReference(&internal.CredentialsRef, workloadIdentity) {
		return true
	}

	if slices.ContainsFunc(seed.Spec.DNS.Defaults, func(d gardencorev1beta1.SeedDNSProviderConfig) bool {
		return isWorkloadIdentityObjectReference(&d.CredentialsRef, workloadIdentity)
	}) {
		return true
	}

	return slices.ContainsFunc(seed.Spec.Resources, func(r gardencorev1beta1.NamedResourceReference) bool {
		return isWorkloadIdentityReference(r.ResourceRef.APIVersion, r.ResourceRef.Kind, v1beta1constants.GardenNamespace, r.ResourceRef.Name, workloadIdentity)
	})
}

// shootReferencesWorkloadIdentity mirrors the Shoot -> WorkloadIdentity edges of the SeedAuthorizer graph
// (pkg/utils/graph/eventhandler_shoot.go and eventhandler_credentialsbinding.go). Keep both in sync when adding new
// credentials references to the Shoot API.
func (r *TokenRequestREST) shootReferencesWorkloadIdentity(shoot *gardencorev1beta1.Shoot, workloadIdentity *securityapi.WorkloadIdentity) (bool, error) {
	if shoot.Spec.DNS != nil && slices.ContainsFunc(shoot.Spec.DNS.Providers, func(p gardencorev1beta1.DNSProvider) bool {
		return p.CredentialsRef != nil && isWorkloadIdentityReference(p.CredentialsRef.APIVersion, p.CredentialsRef.Kind, shoot.Namespace, p.CredentialsRef.Name, workloadIdentity)
	}) {
		return true, nil
	}

	if slices.ContainsFunc(shoot.Spec.Resources, func(r gardencorev1beta1.NamedResourceReference) bool {
		return isWorkloadIdentityReference(r.ResourceRef.APIVersion, r.ResourceRef.Kind, shoot.Namespace, r.ResourceRef.Name, workloadIdentity)
	}) {
		return true, nil
	}

	if shoot.Spec.CredentialsBindingName == nil {
		return false, nil
	}

	credentialsBinding, err := r.credentialsBindingLister.CredentialsBindings(shoot.Namespace).Get(*shoot.Spec.CredentialsBindingName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return isWorkloadIdentityObjectReference(&credentialsBinding.CredentialsRef, workloadIdentity), nil
}
