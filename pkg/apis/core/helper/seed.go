// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// TaintsHave returns true if the given key is part of the taints list.
func TaintsHave(taints []core.SeedTaint, key string) bool {
	for _, taint := range taints {
		if taint.Key == key {
			return true
		}
	}
	return false
}

// TaintsAreTolerated returns true when all the given taints are tolerated by the given tolerations.
func TaintsAreTolerated(taints []core.SeedTaint, tolerations []core.Toleration) bool {
	if len(taints) == 0 {
		return true
	}
	if len(taints) > len(tolerations) {
		return false
	}

	tolerationKeyValues := make(map[string]string, len(tolerations))
	for _, toleration := range tolerations {
		v := ""
		if toleration.Value != nil {
			v = *toleration.Value
		}
		tolerationKeyValues[toleration.Key] = v
	}

	for _, taint := range taints {
		tolerationValue, ok := tolerationKeyValues[taint.Key]
		if !ok {
			return false
		}
		if taint.Value != nil && *taint.Value != tolerationValue {
			return false
		}
	}

	return true
}

// SeedSettingSchedulingVisible returns true if the 'scheduling' setting is set to 'visible'.
func SeedSettingSchedulingVisible(settings *core.SeedSettings) bool {
	return settings == nil || settings.Scheduling == nil || settings.Scheduling.Visible
}

// SeedSettingTopologyAwareRoutingEnabled returns true if the topology-aware routing is enabled.
func SeedSettingTopologyAwareRoutingEnabled(settings *core.SeedSettings) bool {
	return settings != nil && settings.TopologyAwareRouting != nil && settings.TopologyAwareRouting.Enabled
}

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(core.AddToScheme(scheme))
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme))
}

// ConvertSeed converts the given external Seed version to an internal version.
func ConvertSeed(obj runtime.Object) (*core.Seed, error) {
	obj, err := scheme.ConvertToVersion(obj, core.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*core.Seed)
	if !ok {
		return nil, errors.New("could not convert Seed to internal version")
	}
	return result, nil
}

// ConvertSeedExternal converts the given internal Seed version to an external version.
func ConvertSeedExternal(obj runtime.Object) (*gardencorev1beta1.Seed, error) {
	obj, err := scheme.ConvertToVersion(obj, gardencorev1beta1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return nil, fmt.Errorf("could not convert Seed to version %s", gardencorev1beta1.SchemeGroupVersion.String())
	}
	return result, nil
}

// CalculateSeedUsage returns a map representing the number of shoots per seed from the given list of shoots.
// It takes both spec.seedName and status.seedName into account.
func CalculateSeedUsage(shootList []*core.Shoot) map[string]int {
	m := map[string]int{}

	for _, shoot := range shootList {
		var (
			specSeed   = ptr.Deref(shoot.Spec.SeedName, "")
			statusSeed = ptr.Deref(shoot.Status.SeedName, "")
		)

		if specSeed != "" {
			m[specSeed]++
		}
		if statusSeed != "" && specSeed != statusSeed {
			m[statusSeed]++
		}
	}

	return m
}

// ConvertSeedTemplate converts the given external SeedTemplate version to an internal version.
func ConvertSeedTemplate(obj *gardencorev1beta1.SeedTemplate) (*core.SeedTemplate, error) {
	seed, err := ConvertSeed(&gardencorev1beta1.Seed{
		Spec: obj.Spec,
	})
	if err != nil {
		return nil, errors.New("could not convert SeedTemplate to internal version")
	}

	return &core.SeedTemplate{
		Spec: seed.Spec,
	}, nil
}

// ConvertSeedTemplateExternal converts the given internal SeedTemplate version to an external version.
func ConvertSeedTemplateExternal(obj *core.SeedTemplate) (*gardencorev1beta1.SeedTemplate, error) {
	seed, err := ConvertSeedExternal(&core.Seed{
		Spec: obj.Spec,
	})
	if err != nil {
		return nil, errors.New("could not convert SeedTemplate to external version")
	}

	return &gardencorev1beta1.SeedTemplate{
		Spec: seed.Spec,
	}, nil
}
