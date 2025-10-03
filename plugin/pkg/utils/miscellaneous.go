// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// SkipVerification is a common function to skip object verification during admission
func SkipVerification(operation admission.Operation, metadata metav1.ObjectMeta) bool {
	return operation == admission.Update && metadata.DeletionTimestamp != nil
}

// IsSeedUsedByShoot checks whether there is a shoot cluster referencing the provided seed name
func IsSeedUsedByShoot(seedName string, shoots []*gardencorev1beta1.Shoot) bool {
	for _, shoot := range shoots {
		if isSeedUsedByShoot(seedName, shoot) {
			return true
		}
	}
	return false
}

// GetFilteredShootList returns shoots returned by the shootLister filtered via the predicateFn.
func GetFilteredShootList(shootLister gardencorev1beta1listers.ShootLister, predicateFn func(*gardencorev1beta1.Shoot) bool) ([]*gardencorev1beta1.Shoot, error) {
	var matchingShoots []*gardencorev1beta1.Shoot

	shoots, err := shootLister.List(labels.Everything())
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list shoots: %w", err))
	}

	for _, shoot := range shoots {
		if predicateFn(shoot) {
			matchingShoots = append(matchingShoots, shoot)
		}
	}
	return matchingShoots, nil
}

// NewAttributesWithName returns admission.Attributes with the given name and all other attributes kept same.
func NewAttributesWithName(a admission.Attributes, name string) admission.Attributes {
	return admission.NewAttributesRecord(a.GetObject(),
		a.GetOldObject(),
		a.GetKind(),
		a.GetNamespace(),
		name,
		a.GetResource(),
		a.GetSubresource(),
		a.GetOperation(),
		a.GetOperationOptions(),
		a.IsDryRun(),
		a.GetUserInfo())
}

// ValidateZoneRemovalFromSeeds returns an error when zones are removed from the old seed while it is still in use by
// shoots.
func ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec *core.SeedSpec, seedName string, shootLister gardencorev1beta1listers.ShootLister, kind string) error {
	if removedZones := sets.New(oldSeedSpec.Provider.Zones...).Difference(sets.New(newSeedSpec.Provider.Zones...)); removedZones.Len() > 0 {
		shoots, err := shootLister.List(labels.Everything())
		if err != nil {
			return err
		}

		if IsSeedUsedByShoot(seedName, shoots) {
			return apierrors.NewForbidden(core.Resource(kind), seedName, fmt.Errorf("cannot remove zones %v from %s %s as there are Shoots scheduled to this Seed", sets.List(removedZones), kind, seedName))
		}
	}

	return nil
}

// ValidateInternalDomainChangeForSeed returns an error when the internal domain is changed for a seed that is still in use by shoots.
func ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec *core.SeedSpec, seedName string, shootLister gardencorev1beta1listers.ShootLister, kind string) error {
	// TODO(dimityrmirchev): Remove this if check when dns.internal configuration becomes mandatory (after 1.129 release)
	if oldSeedSpec.DNS.Internal != nil && newSeedSpec.DNS.Internal != nil &&
		oldSeedSpec.DNS.Internal.Domain != newSeedSpec.DNS.Internal.Domain {
		shoots, err := shootLister.List(labels.Everything())
		if err != nil {
			return err
		}

		if IsSeedUsedByShoot(seedName, shoots) {
			return apierrors.NewForbidden(core.Resource(kind), seedName, fmt.Errorf("cannot change internal domain as the %s %q is still in use by shoots", kind, seedName))
		}
	}

	return nil
}

// ValidateDefaultDomainsChangeForSeed returns an error when default domains that are being used by a shoot scheduled on the seed are removed,
// or when default domains are being added but do not cover all the globally available default domains used by shoots on the seed.
func ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec *core.SeedSpec, seedName string, shootLister gardencorev1beta1listers.ShootLister, secretLister kubecorev1listers.SecretLister, kind string) error {
	oldDomains := sets.New[string]()
	for _, defaultDomain := range oldSeedSpec.DNS.Defaults {
		oldDomains.Insert(defaultDomain.Domain)
	}

	newDomains := sets.New[string]()
	for _, defaultDomain := range newSeedSpec.DNS.Defaults {
		newDomains.Insert(defaultDomain.Domain)
	}

	shoots, err := shootLister.List(labels.Everything())
	if err != nil {
		return err
	}

	// Get shoots on this seed
	seedShoots := make([]*gardencorev1beta1.Shoot, 0)
	for _, shoot := range shoots {
		if isSeedUsedByShoot(seedName, shoot) {
			seedShoots = append(seedShoots, shoot)
		}
	}

	removedDomains := oldDomains.Difference(newDomains)
	if removedDomains.Len() > 0 {
		usedRemovedDomains := sets.New[string]()
		for _, shoot := range seedShoots {
			if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
				continue
			}

			shootDomain := *shoot.Spec.DNS.Domain

			for removedDomain := range removedDomains {
				if strings.HasSuffix(shootDomain, "."+removedDomain) {
					usedRemovedDomains.Insert(removedDomain)
					break
				}
			}
		}

		if usedRemovedDomains.Len() > 0 {
			formatted := make([]string, 0, usedRemovedDomains.Len())
			for domain := range usedRemovedDomains {
				formatted = append(formatted, domain)
			}
			return apierrors.NewForbidden(core.Resource(kind), seedName, fmt.Errorf("cannot remove default domains %v from %s %q as they are still being used by shoots", formatted, kind, seedName))
		}
	}

	// Validate addition of explicit domain configuration when transitioning from global defaults
	// TODO(dimityrmirchev): This logic would become obsolete once explicit DNS configuration becomes mandatory
	// remove it after release v1.131
	if len(oldDomains) == 0 && len(newDomains) > 0 {
		// Old seed had no explicit DNS defaults (used global defaults), new seed has explicit defaults
		// Need to ensure all globally configured default domains used by shoots are covered
		globalDefaultDomains, err := getGlobalDefaultDomains(secretLister)
		if err != nil {
			return err
		}

		usedGlobalDomains := sets.New[string]()
		for _, shoot := range seedShoots {
			if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
				continue
			}

			shootDomain := *shoot.Spec.DNS.Domain

			for _, globalDomain := range globalDefaultDomains {
				if strings.HasSuffix(shootDomain, "."+globalDomain) {
					usedGlobalDomains.Insert(globalDomain)
					break
				}
			}
		}

		missingDomains := usedGlobalDomains.Difference(newDomains)
		if missingDomains.Len() > 0 {
			formatted := make([]string, 0, missingDomains.Len())
			for domain := range missingDomains {
				formatted = append(formatted, domain)
			}
			return apierrors.NewForbidden(core.Resource(kind), seedName, fmt.Errorf("cannot configure explicit default domains for %s %q without including domains %v that are currently being used by shoots", kind, seedName, formatted))
		}
	}

	return nil
}

// getGlobalDefaultDomains retrieves the globally configured default domain secrets
func getGlobalDefaultDomains(secretLister kubecorev1listers.SecretLister) ([]string, error) {
	selector, err := labels.Parse(fmt.Sprintf("%s=%s", v1beta1constants.GardenRole, v1beta1constants.GardenRoleDefaultDomain))
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	domainSecrets, err := secretLister.Secrets(v1beta1constants.GardenNamespace).List(selector)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	var globalDefaultDomains []string
	for _, domainSecret := range domainSecrets {
		_, domain, _, err := gardenerutils.GetDomainInfoFromAnnotations(domainSecret.GetAnnotations())
		if err != nil {
			return nil, err
		}
		globalDefaultDomains = append(globalDefaultDomains, domain)
	}
	return globalDefaultDomains, nil
}

// isSeedUsedByShoot checks whether the provided shoot is using the specified seed name
func isSeedUsedByShoot(seedName string, shoot *gardencorev1beta1.Shoot) bool {
	if shoot.Spec.SeedName != nil && *shoot.Spec.SeedName == seedName {
		return true
	}
	if shoot.Status.SeedName != nil && *shoot.Status.SeedName == seedName {
		return true
	}
	return false
}
