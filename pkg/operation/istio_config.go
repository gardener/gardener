// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	alternativeZoneKey = v1beta1constants.GardenRole
	zoneInfix          = "--zone--"
)

// IstioServiceName is the currently used name of the istio ingress service, which is responsible for the shoot cluster.
func (o *Operation) IstioServiceName() string {
	return *o.sniConfig().Ingress.ServiceName
}

// IstioNamespace is the currently used namespace of the istio ingress gateway, which is responsible for the shoot cluster.
func (o *Operation) IstioNamespace() string {
	return o.addZonePinningIfRequired(*o.sniConfig().Ingress.Namespace)
}

// IstioLoadBalancerAnnotations contain the annotation to be used for the istio ingress service load balancer.
func (o *Operation) IstioLoadBalancerAnnotations() map[string]string {
	zone := o.singleZoneIfPinned()
	if exposureClassHandler := o.exposureClassHandler(); exposureClassHandler != nil {
		if zone != nil {
			return utils.MergeStringMaps(exposureClassHandler.LoadBalancerService.Annotations, o.Seed.GetZonalLoadBalancerServiceAnnotations(*zone))
		}
		return utils.MergeStringMaps(o.Seed.GetLoadBalancerServiceAnnotations(), exposureClassHandler.LoadBalancerService.Annotations)
	}
	if zone != nil {
		return o.Seed.GetZonalLoadBalancerServiceAnnotations(*zone)
	}
	return o.Seed.GetLoadBalancerServiceAnnotations()
}

// IstioLabels contain the labels to be used for the istio ingress gateway entities.
func (o *Operation) IstioLabels() map[string]string {
	zone := o.singleZoneIfPinned()
	if exposureClassHandler := o.exposureClassHandler(); exposureClassHandler != nil {
		return GetIstioZoneLabels(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(exposureClassHandler.SNI.Ingress.Labels, exposureClassHandler.Name), zone)
	}
	return GetIstioZoneLabels(o.sniConfig().Ingress.Labels, zone)
}

func (o *Operation) exposureClassHandler() *gardenletconfig.ExposureClassHandler {
	if exposureClassName := o.Shoot.GetInfo().Spec.ExposureClassName; exposureClassName != nil {
		for _, handler := range o.Config.ExposureClassHandlers {
			if *exposureClassName == handler.Name {
				return &handler
			}
		}
	}
	return nil
}

func (o *Operation) sniConfig() *gardenletconfig.SNI {
	if exposureClassHandler := o.exposureClassHandler(); exposureClassHandler != nil {
		return exposureClassHandler.SNI
	}
	return o.Config.SNI
}

func (o *Operation) addZonePinningIfRequired(namespace string) string {
	// Only clusters pinned to exactly one zone are exposed via zonal istio ingress gateway.
	// All other clusters, i.e. true HA and legacy/accidental multi-zonal clusters, are exposed
	// via the default multi-zonal istio ingress gateway.
	zone := o.singleZoneIfPinned()
	if zone != nil {
		return GetIstioNamespaceForZone(namespace, *zone)
	}
	return namespace
}

func (o *Operation) singleZoneIfPinned() *string {
	// Zone-specific istio ingress gateways are only deployed with more than one zone
	if len(o.Seed.GetInfo().Spec.Provider.Zones) <= 1 {
		return nil
	}
	if v, ok := o.SeedNamespaceObject.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones]; ok {
		zones := sets.List(sets.New[string](strings.Split(v, ",")...).Delete(""))
		if len(zones) == 1 {
			return &zones[0]
		}
	}
	return nil
}

// GetIstioNamespaceForZone returns the namespace to use for a given zone.
// In case the zone name is too long the first five characters of the hash of the zone are used as zone identifiers.
func GetIstioNamespaceForZone(defaultNamespace string, zone string) string {
	const format = "%s--%s"
	if ns := fmt.Sprintf(format, defaultNamespace, zone); len(ns) <= validation.DNS1035LabelMaxLength {
		return ns
	}
	// Use the first five characters of the hash of the zone
	hashedZone := utils.ComputeSHA256Hex([]byte(zone))
	return fmt.Sprintf(format, defaultNamespace, hashedZone[:5])
}

// GetIstioZoneLabels returns the labels to be used for istio with the mandatory zone label set.
func GetIstioZoneLabels(labels map[string]string, zone *string) map[string]string {
	// Use "istio" for the default gateways and v1beta1constants.LabelExposureClassHandlerName for exposure classes
	zonekey := istio.DefaultZoneKey
	zoneValue := "ingressgateway"
	if value, ok := labels[zonekey]; ok {
		zoneValue = value
	} else if value, ok := labels[alternativeZoneKey]; ok {
		zonekey = alternativeZoneKey
		zoneValue = value
	}
	if zone != nil {
		zoneValue = fmt.Sprintf("%s%s%s", zoneValue, zoneInfix, *zone)
	}
	return utils.MergeStringMaps(labels, map[string]string{zonekey: zoneValue})
}

// IsZonalIstioExtension indicates whether the namespace related to the given labels is a zonal istio extension.
// It also returns the zone.
func IsZonalIstioExtension(labels map[string]string) (bool, string) {
	if v, ok := labels[istio.DefaultZoneKey]; ok {
		i := strings.Index(v, zoneInfix)
		if i < 0 {
			return false, ""
		}
		// There should be at least one character before and after the zone infix.
		return i > 0 && i < len(v)-len(zoneInfix), v[i+len(zoneInfix):]
	}
	if _, ok := labels[v1beta1constants.LabelExposureClassHandlerName]; ok {
		if v, ok := labels[alternativeZoneKey]; ok && strings.HasPrefix(v, v1beta1constants.GardenRoleExposureClassHandler) {
			i := strings.Index(v, zoneInfix)
			if i < 0 {
				return false, ""
			}
			// There should be at least v1beta1constants.GardenRoleExposureClassHandler characters before
			// and one after the zone infix.
			return i >= len(v1beta1constants.GardenRoleExposureClassHandler) && i < len(v)-len(zoneInfix), v[i+len(zoneInfix):]
		}
	}
	return false, ""
}
