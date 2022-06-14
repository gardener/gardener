// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://wwr.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package worker

import (
	"fmt"
	"math"
	"regexp"
	"strconv"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var diskSizeRegexp *regexp.Regexp

func init() {
	regexp, err := regexp.Compile(`^(\d+)`)
	utilruntime.Must(err)
	diskSizeRegexp = regexp
}

// MachineDeployment holds information about the name, class, replicas of a MachineDeployment
// managed by the machine-controller-manager.
type MachineDeployment struct {
	Name                 string
	ClassName            string
	SecretName           string
	Minimum              int32
	Maximum              int32
	MaxSurge             intstr.IntOrString
	MaxUnavailable       intstr.IntOrString
	Labels               map[string]string
	Annotations          map[string]string
	Taints               []corev1.Taint
	State                *MachineDeploymentState
	MachineConfiguration *machinev1alpha1.MachineConfiguration
}

// MachineDeployments is a list of machine deployments.
type MachineDeployments []MachineDeployment

// MachineDeploymentState stores the last versions of the machine sets and machine which
// the machine deployment corresponds
type MachineDeploymentState struct {
	Replicas    int32                        `json:"replicas,omitempty"`
	MachineSets []machinev1alpha1.MachineSet `json:"machineSets,omitempty"`
	Machines    []machinev1alpha1.Machine    `json:"machines,omitempty"`
}

// State represent the last known state of a Worker
type State struct {
	MachineDeployments map[string]*MachineDeploymentState `json:"machineDeployments,omitempty"`
}

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
func WorkerPoolHash(pool extensionsv1alpha1.WorkerPool, cluster *extensionscontroller.Cluster, additionalData ...string) (string, error) {
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
			if w.CRI != nil && w.CRI.Name != gardencorev1beta1.CRINameDocker {
				data = append(data, string(w.CRI.Name))
			}
		}
	}

	if status := cluster.Shoot.Status; status.Credentials != nil && status.Credentials.Rotation != nil {
		if status.Credentials.Rotation.CertificateAuthorities != nil && status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime != nil {
			data = append(data, status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time.String())
		}
		if status.Credentials.Rotation.ServiceAccountKey != nil && status.Credentials.Rotation.ServiceAccountKey.LastInitiationTime != nil {
			data = append(data, status.Credentials.Rotation.ServiceAccountKey.LastInitiationTime.Time.String())
		}
	}

	// Do not consider the shoot annotations here to prevent unintended node roll outs.
	if helper.IsNodeLocalDNSEnabled(cluster.Shoot.Spec.SystemComponents, map[string]string{}) {
		data = append(data, "node-local-dns")
	}

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
	return intstr.FromInt(int(DistributeOverZones(zoneIndex, intOrPercent.IntVal, zoneSize)))
}

// DiskSize extracts the numerical component of DiskSize strings, i.e. strings like "10Gi" and
// returns it as string, i.e. "10" will be returned.
func DiskSize(size string) (int, error) {
	i, err := strconv.Atoi(diskSizeRegexp.FindString(size))
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
