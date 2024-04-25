// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"net"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ClusterAutoscalerRequired returns whether the given worker pool configuration indicates that a cluster-autoscaler
// is needed.
func ClusterAutoscalerRequired(pools []extensionsv1alpha1.WorkerPool) bool {
	for _, pool := range pools {
		if pool.Maximum > pool.Minimum {
			return true
		}
	}
	return false
}

// GetDNSRecordType returns the appropriate DNS record type (A/AAAA or CNAME) for the given address.
func GetDNSRecordType(address string) extensionsv1alpha1.DNSRecordType {
	if ip := net.ParseIP(address); ip != nil {
		if ip.To4() != nil {
			return extensionsv1alpha1.DNSRecordTypeA
		}
		return extensionsv1alpha1.DNSRecordTypeAAAA
	}
	return extensionsv1alpha1.DNSRecordTypeCNAME
}

// GetDNSRecordTTL returns the value of the given ttl, or 120 if nil.
func GetDNSRecordTTL(ttl *int64) int64 {
	if ttl != nil {
		return *ttl
	}
	return 120
}

// DeterminePrimaryIPFamily determines the primary IP family out of a specified list of IP families.
func DeterminePrimaryIPFamily(ipFamilies []extensionsv1alpha1.IPFamily) extensionsv1alpha1.IPFamily {
	if len(ipFamilies) == 0 {
		return extensionsv1alpha1.IPFamilyIPv4
	}
	return ipFamilies[0]
}

// FilePathsFrom returns the paths for all the given files.
func FilePathsFrom(files []extensionsv1alpha1.File) []string {
	var out []string

	for _, file := range files {
		out = append(out, file.Path)
	}

	return out
}

// GetMachineDeploymentClusterAutoscalerAnnotations returns a map of annotations with values intended to be used as cluster-autoscaler options for the worker group
func GetMachineDeploymentClusterAutoscalerAnnotations(caOptions *extensionsv1alpha1.ClusterAutoscalerOptions) map[string]string {
	var annotations map[string]string
	if caOptions != nil {
		annotations = map[string]string{}
		if caOptions.ScaleDownUtilizationThreshold != nil {
			annotations[extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation] = *caOptions.ScaleDownUtilizationThreshold
		}
		if caOptions.ScaleDownGpuUtilizationThreshold != nil {
			annotations[extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation] = *caOptions.ScaleDownGpuUtilizationThreshold
		}
		if caOptions.ScaleDownUnneededTime != nil {
			annotations[extensionsv1alpha1.ScaleDownUnneededTimeAnnotation] = caOptions.ScaleDownUnneededTime.Duration.String()
		}
		if caOptions.ScaleDownUnreadyTime != nil {
			annotations[extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation] = caOptions.ScaleDownUnreadyTime.Duration.String()
		}
		if caOptions.MaxNodeProvisionTime != nil {
			annotations[extensionsv1alpha1.MaxNodeProvisionTimeAnnotation] = caOptions.MaxNodeProvisionTime.Duration.String()
		}
	}

	return annotations
}
