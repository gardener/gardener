// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
)

var diskSizeRegex = regexp.MustCompile(`^(\d+)`)

// MachineDeployment holds information about the name, class, replicas of a MachineDeployment
// managed by the machine-controller-manager.
type MachineDeployment struct {
	Name                         string
	ClassName                    string
	SecretName                   string
	Minimum                      int32
	Maximum                      int32
	Priority                     *int32
	MaxSurge                     intstr.IntOrString
	MaxUnavailable               intstr.IntOrString
	Labels                       map[string]string
	Annotations                  map[string]string
	Taints                       []corev1.Taint
	State                        *shootstate.MachineDeploymentState
	MachineConfiguration         *machinev1alpha1.MachineConfiguration
	ClusterAutoscalerAnnotations map[string]string
}

// MachineDeployments is a list of machine deployments.
type MachineDeployments []MachineDeployment

// HasDeployment checks whether the <name> is part of the <machineDeployments>
// list, i.e. whether there is an entry whose 'Name' attribute matches <name>. It returns true or false.
func (m MachineDeployments) HasDeployment(name string) bool {
	for _, deployment := range m {
		if name == deployment.Name {
			return true
		}
	}
	return false
}

// FindByName finds the deployment with the <name> from the <machineDeployments>
// returns the machine deployment or nil
func (m MachineDeployments) FindByName(name string) *MachineDeployment {
	for _, deployment := range m {
		if name == deployment.Name {
			return &deployment
		}
	}
	return nil
}

// HasClass checks whether the <className> is part of the <machineDeployments>
// list, i.e. whether there is an entry whose 'ClassName' attribute matches <name>. It returns true or false.
func (m MachineDeployments) HasClass(className string) bool {
	for _, deployment := range m {
		if className == deployment.ClassName {
			return true
		}
	}
	return false
}

// HasSecret checks whether the <secretName> is part of the <machineDeployments>
// list, i.e. whether there is an entry whose 'SecretName' attribute matches <name>. It returns true or false.
func (m MachineDeployments) HasSecret(secretName string) bool {
	for _, deployment := range m {
		if secretName == deployment.SecretName {
			return true
		}
	}
	return false
}

// WorkerPoolHash returns a hash value for a given worker pool and a given cluster resource.
func WorkerPoolHash(pool extensionsv1alpha1.WorkerPool, cluster *extensionscontroller.Cluster, additionalDataV1 []string, additionalDataV2 []string) (string, error) {
	if pool.NodeAgentSecretName != nil {
		return WorkerPoolHashV2(*pool.NodeAgentSecretName, additionalDataV2...)
	}
	return WorkerPoolHashV1(pool, cluster, additionalDataV1...)
}

// WorkerPoolHashV1 returns a hash value for a given worker pool and a given cluster resource.
func WorkerPoolHashV1(pool extensionsv1alpha1.WorkerPool, cluster *extensionscontroller.Cluster, additionalData ...string) (string, error) {
	kubernetesVersion := cluster.Shoot.Spec.Kubernetes.Version
	if pool.KubernetesVersion != nil {
		kubernetesVersion = *pool.KubernetesVersion
	}
	shootVersionMajorMinor, err := util.VersionMajorMinor(kubernetesVersion)
	if err != nil {
		return "", err
	}

	data := []string{
		shootVersionMajorMinor,
		pool.MachineType,
		pool.MachineImage.Name + pool.MachineImage.Version,
	}

	if pool.Volume != nil {
		data = append(data, pool.Volume.Size)

		if pool.Volume.Type != nil {
			data = append(data, *pool.Volume.Type)
		}
	}

	if pool.ProviderConfig != nil && pool.ProviderConfig.Raw != nil {
		data = append(data, string(pool.ProviderConfig.Raw))
	}

	data = append(data, additionalData...)

	for _, w := range cluster.Shoot.Spec.Provider.Workers {
		if pool.Name == w.Name {
			if w.CRI != nil {
				data = append(data, string(w.CRI.Name))
			}
		}
	}

	if status := cluster.Shoot.Status; status.Credentials != nil && status.Credentials.Rotation != nil {
		if status.Credentials.Rotation.CertificateAuthorities != nil {
			if lastInitiationTime := v1beta1helper.LastInitiationTimeForWorkerPool(pool.Name, status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts, status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime); lastInitiationTime != nil {
				data = append(data, lastInitiationTime.Time.String())
			}
		}
		if status.Credentials.Rotation.ServiceAccountKey != nil {
			if lastInitiationTime := v1beta1helper.LastInitiationTimeForWorkerPool(pool.Name, status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts, status.Credentials.Rotation.ServiceAccountKey.LastInitiationTime); lastInitiationTime != nil {
				data = append(data, lastInitiationTime.Time.String())
			}
		}
	}

	if v1beta1helper.IsNodeLocalDNSEnabled(cluster.Shoot.Spec.SystemComponents) {
		data = append(data, "node-local-dns")
	}

	var result string
	for _, v := range data {
		result += utils.ComputeSHA256Hex([]byte(v))
	}

	return utils.ComputeSHA256Hex([]byte(result))[:5], nil
}

// WorkerPoolHashV2 returns a hash value for a given nodeAgentSecretName and additional data.
func WorkerPoolHashV2(nodeAgentSecretName string, additionalData ...string) (string, error) {
	data := []string{nodeAgentSecretName}

	data = append(data, additionalData...)

	var result string
	for _, v := range data {
		result += utils.ComputeSHA256Hex([]byte(v))
	}

	return utils.ComputeSHA256Hex([]byte(result))[:5], nil
}

// DistributeOverZones is a function which is used to determine how many nodes should be used
// for each availability zone. It takes the number of availability zones (<zoneSize>), the
// index of the current zone (<zoneIndex>) and the number of nodes which must be distributed
// over the zones (<size>) and returns the number of nodes which should be placed in the zone
// of index <zoneIndex>.
// The distribution happens equally. In case of an uneven number <size>, the last zone will have
// one more node than the others.
func DistributeOverZones(zoneIndex, size, zoneSize int32) int32 {
	first := size / zoneSize
	second := int32(0)
	if zoneIndex < (size % zoneSize) {
		second = 1
	}
	return first + second
}

// DistributePercentOverZones distributes a given percentage value over zones in relation to
// the given total value. In case the total value is evenly divisible over the zones, this
// always just returns the initial percentage. Otherwise, the total value is used to determine
// the weight of a specific zone in relation to the other zones and adapt the given percentage
// accordingly.
func DistributePercentOverZones(zoneIndex int32, percent string, zoneSize, total int32) string {
	percents, err := strconv.Atoi(percent[:len(percent)-1])
	if err != nil {
		panic(fmt.Sprintf("given value %q is not a percent value", percent))
	}

	var weightedPercents int
	if total%zoneSize == 0 {
		// Zones are evenly sized, we don't need to adapt the percentage per zone
		weightedPercents = percents
	} else {
		// Zones are not evenly sized, we need to calculate the ratio of each zone
		// and modify the percentage depending on that ratio.
		zoneTotal := DistributeOverZones(zoneIndex, total, zoneSize)
		absoluteTotalRatio := float64(total) / float64(zoneSize)
		ratio := 100.0 / absoluteTotalRatio * float64(zoneTotal)
		// Optimistic rounding up, this will cause an actual max surge / max unavailable percentage to be a bit higher.
		weightedPercents = int(math.Ceil(ratio * float64(percents) / 100.0))
	}

	return fmt.Sprintf("%d%%", weightedPercents)
}

// DistributePositiveIntOrPercent distributes a given int or percentage value over zones in relation to
// the given total value. In case the total value is evenly divisible over the zones, this
// always just returns the initial percentage. Otherwise, the total value is used to determine
// the weight of a specific zone in relation to the other zones and adapt the given percentage
// accordingly.
func DistributePositiveIntOrPercent(zoneIndex int32, intOrPercent intstr.IntOrString, zoneSize, total int32) intstr.IntOrString {
	if intOrPercent.Type == intstr.String {
		return intstr.FromString(DistributePercentOverZones(zoneIndex, intOrPercent.StrVal, zoneSize, total))
	}
	return intstr.FromInt32(DistributeOverZones(zoneIndex, intOrPercent.IntVal, zoneSize))
}

// DiskSize extracts the numerical component of DiskSize strings, i.e. strings like "10Gi" and
// returns it as string, i.e. "10" will be returned.
func DiskSize(size string) (int, error) {
	i, err := strconv.Atoi(diskSizeRegex.FindString(size))
	if err != nil {
		return -1, err
	}
	return i, nil
}

// ErrorMachineImageNotFound returns an appropriate error message for an unknown name/version image pair.
func ErrorMachineImageNotFound(name, version string, opt ...string) error {
	ext := ""
	for _, o := range opt {
		ext += "/" + o
	}
	return fmt.Errorf("could not find machine image for %s/%s%s neither in cloud profile nor in worker status", name, version, ext)
}

// FetchUserData fetches the user data for a worker pool.
func FetchUserData(ctx context.Context, c client.Client, namespace string, pool extensionsv1alpha1.WorkerPool) ([]byte, error) {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: pool.UserDataSecretRef.Name, Namespace: namespace}}
	if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		return nil, fmt.Errorf("failed fetching user data secret %s referenced in worker pool %s: %w", pool.UserDataSecretRef.Name, pool.Name, err)
	}

	userData, ok := secret.Data[pool.UserDataSecretRef.Key]
	if !ok || len(userData) == 0 {
		return nil, fmt.Errorf("user data secret %s for worker pool %s has no %s field or it's empty", pool.UserDataSecretRef.Name, pool.Name, pool.UserDataSecretRef.Key)
	}

	return userData, nil
}
