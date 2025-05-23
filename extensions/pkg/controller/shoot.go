// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
)

// ChartRendererFactory creates chartrenderer.Interface to be used by this actuator.
type ChartRendererFactory interface {
	// NewChartRendererForShoot creates a new chartrenderer.Interface for the shoot cluster.
	NewChartRendererForShoot(string) (chartrenderer.Interface, error)
}

// ChartRendererFactoryFunc is a function that satisfies ChartRendererFactory.
type ChartRendererFactoryFunc func(string) (chartrenderer.Interface, error)

// NewChartRendererForShoot creates a new chartrenderer.Interface for the shoot cluster.
func (f ChartRendererFactoryFunc) NewChartRendererForShoot(version string) (chartrenderer.Interface, error) {
	return f(version)
}

// GetPodNetwork returns the pod network CIDRs of the given Shoot.
func GetPodNetwork(cluster *Cluster) []string {
	var pods []string
	if cluster.Shoot.Spec.Networking != nil && cluster.Shoot.Spec.Networking.Pods != nil {
		pods = append(pods, *cluster.Shoot.Spec.Networking.Pods)
	}
	if cluster.Shoot.Status.Networking != nil {
		existing := sets.New(pods...)
		for _, p := range cluster.Shoot.Status.Networking.Pods {
			if !existing.Has(p) {
				pods = append(pods, p)
			}
		}
	}
	return pods
}

// GetServiceNetwork returns the service network CIDRs of the given Shoot.
func GetServiceNetwork(cluster *Cluster) []string {
	var services []string
	if cluster.Shoot.Spec.Networking != nil && cluster.Shoot.Spec.Networking.Services != nil {
		services = append(services, *cluster.Shoot.Spec.Networking.Services)
	}
	if cluster.Shoot.Status.Networking != nil {
		existing := sets.New(services...)
		for _, s := range cluster.Shoot.Status.Networking.Services {
			if !existing.Has(s) {
				services = append(services, s)
			}
		}
	}
	return services
}

// IsHibernationEnabled returns true if the shoot is marked for hibernation, or false otherwise.
func IsHibernationEnabled(cluster *Cluster) bool {
	return cluster.Shoot.Spec.Hibernation != nil && cluster.Shoot.Spec.Hibernation.Enabled != nil && *cluster.Shoot.Spec.Hibernation.Enabled
}

// IsHibernated returns true if shoot spec indicates that it is marked for hibernation and its status indicates that the hibernation is complete or false otherwise
func IsHibernated(cluster *Cluster) bool {
	return IsHibernationEnabled(cluster) && cluster.Shoot.Status.IsHibernated
}

// IsHibernatingOrWakingUp returns true if the cluster either wakes up from hibernation or is going into hibernation but not yet hibernated
func IsHibernatingOrWakingUp(cluster *Cluster) bool {
	return IsHibernationEnabled(cluster) != cluster.Shoot.Status.IsHibernated
}

// IsCreationInProcess returns true if the cluster is in the process of getting created, false otherwise
func IsCreationInProcess(cluster *Cluster) bool {
	return cluster.Shoot.Status.LastOperation == nil || cluster.Shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate
}

// IsFailed returns true if the embedded shoot is failed, or false otherwise.
func IsFailed(cluster *Cluster) bool {
	if cluster == nil {
		return false
	}
	return IsShootFailed(cluster.Shoot)
}

// IsShootFailed returns true if the shoot is failed, or false otherwise.
func IsShootFailed(shoot *gardencorev1beta1.Shoot) bool {
	if shoot == nil {
		return false
	}
	lastOperation := shoot.Status.LastOperation
	return lastOperation != nil && lastOperation.State == gardencorev1beta1.LastOperationStateFailed
}

// IsUnmanagedDNSProvider returns true if the shoot uses an unmanaged DNS provider.
func IsUnmanagedDNSProvider(cluster *Cluster) bool {
	dns := cluster.Shoot.Spec.DNS
	return dns == nil || (dns.Domain == nil && len(dns.Providers) > 0 && dns.Providers[0].Type != nil && *dns.Providers[0].Type == "unmanaged")
}

// GetReplicas returns the woken up replicas of the given Shoot.
func GetReplicas(cluster *Cluster, wokenUp int) int {
	if IsHibernationEnabled(cluster) {
		return 0
	}
	return wokenUp
}

// GetControlPlaneReplicas returns the woken up replicas for controlplane components of the given Shoot
// that should only be scaled down at the end of the flow.
func GetControlPlaneReplicas(cluster *Cluster, scaledDown bool, wokenUp int) int {
	if cluster.Shoot != nil && cluster.Shoot.DeletionTimestamp == nil && IsHibernationEnabled(cluster) && scaledDown {
		return 0
	}
	return wokenUp
}
