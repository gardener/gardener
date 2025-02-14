// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package components

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// ConfigurableKubeletCLIFlags is the set of configurable kubelet command line parameters.
type ConfigurableKubeletCLIFlags struct{}

// KubeletCLIFlagsFromCoreV1beta1KubeletConfig computes the ConfigurableKubeletCLIFlags based on the provided
// gardencorev1beta1.KubeletConfig.
func KubeletCLIFlagsFromCoreV1beta1KubeletConfig(_ *gardencorev1beta1.KubeletConfig) ConfigurableKubeletCLIFlags {
	return ConfigurableKubeletCLIFlags{}
}

// ConfigurableKubeletConfigParameters is the set of configurable kubelet config parameters.
type ConfigurableKubeletConfigParameters struct {
	ContainerLogMaxSize              *string
	ContainerLogMaxFiles             *int32
	CpuCFSQuota                      *bool
	CpuManagerPolicy                 *string
	EvictionHard                     map[string]string
	EvictionMinimumReclaim           map[string]string
	EvictionSoft                     map[string]string
	EvictionSoftGracePeriod          map[string]string
	EvictionPressureTransitionPeriod *metav1.Duration
	EvictionMaxPodGracePeriod        *int32
	FailSwapOn                       *bool
	FeatureGates                     map[string]bool
	ImageGCHighThresholdPercent      *int32
	ImageGCLowThresholdPercent       *int32
	SeccompDefault                   *bool
	SerializeImagePulls              *bool
	StreamingConnectionIdleTimeout   *metav1.Duration
	RegistryPullQPS                  *int32
	RegistryBurst                    *int32
	KubeReserved                     map[string]string
	MaxPods                          *int32
	MemorySwap                       *kubeletconfigv1beta1.MemorySwapConfiguration
	PodPidsLimit                     *int64
	ProtectKernelDefaults            *bool
	SystemReserved                   map[string]string
}

const (
	// MemoryAvailable is a constant for the 'memory.available' eviction setting.
	MemoryAvailable = "memory.available"
	// ImageFSAvailable is a constant for the 'imagefs.available' eviction setting.
	ImageFSAvailable = "imagefs.available"
	// ImageFSInodesFree is a constant for the 'imagefs.inodesFree' eviction setting.
	ImageFSInodesFree = "imagefs.inodesFree"
	// NodeFSAvailable is a constant for the 'nodefs.available' eviction setting.
	NodeFSAvailable = "nodefs.available"
	// NodeFSInodesFree is a constant for the 'nodefs.inodesFree' eviction setting.
	NodeFSInodesFree = "nodefs.inodesFree"
)

// KubeletConfigParametersFromCoreV1beta1KubeletConfig computes the ConfigurableKubeletConfigParameters based on the provided
// gardencorev1beta1.KubeletConfig.
func KubeletConfigParametersFromCoreV1beta1KubeletConfig(kubeletConfig *gardencorev1beta1.KubeletConfig) ConfigurableKubeletConfigParameters {
	var out ConfigurableKubeletConfigParameters

	if kubeletConfig != nil {
		out.ContainerLogMaxFiles = kubeletConfig.ContainerLogMaxFiles
		if val := kubeletConfig.ContainerLogMaxSize; val != nil {
			out.ContainerLogMaxSize = ptr.To(val.String())
		}
		out.CpuCFSQuota = kubeletConfig.CPUCFSQuota
		out.CpuManagerPolicy = kubeletConfig.CPUManagerPolicy
		out.EvictionMaxPodGracePeriod = kubeletConfig.EvictionMaxPodGracePeriod
		out.EvictionPressureTransitionPeriod = kubeletConfig.EvictionPressureTransitionPeriod
		out.FailSwapOn = kubeletConfig.FailSwapOn
		out.ImageGCHighThresholdPercent = kubeletConfig.ImageGCHighThresholdPercent
		out.ImageGCLowThresholdPercent = kubeletConfig.ImageGCLowThresholdPercent
		out.SeccompDefault = kubeletConfig.SeccompDefault
		out.SerializeImagePulls = kubeletConfig.SerializeImagePulls
		out.RegistryPullQPS = kubeletConfig.RegistryPullQPS
		out.RegistryBurst = kubeletConfig.RegistryBurst
		out.FeatureGates = kubeletConfig.FeatureGates
		out.KubeReserved = reservedFromKubeletConfig(kubeletConfig.KubeReserved)
		out.MaxPods = kubeletConfig.MaxPods
		out.PodPidsLimit = kubeletConfig.PodPIDsLimit
		out.ProtectKernelDefaults = kubeletConfig.ProtectKernelDefaults
		out.StreamingConnectionIdleTimeout = kubeletConfig.StreamingConnectionIdleTimeout
		out.SystemReserved = reservedFromKubeletConfig(kubeletConfig.SystemReserved)

		if eviction := kubeletConfig.EvictionHard; eviction != nil {
			if out.EvictionHard == nil {
				out.EvictionHard = make(map[string]string)
			}

			if val := eviction.MemoryAvailable; val != nil {
				out.EvictionHard[MemoryAvailable] = *val
			}
			if val := eviction.ImageFSAvailable; val != nil {
				out.EvictionHard[ImageFSAvailable] = *val
			}
			if val := eviction.ImageFSInodesFree; val != nil {
				out.EvictionHard[ImageFSInodesFree] = *val
			}
			if val := eviction.NodeFSAvailable; val != nil {
				out.EvictionHard[NodeFSAvailable] = *val
			}
			if val := eviction.NodeFSInodesFree; val != nil {
				out.EvictionHard[NodeFSInodesFree] = *val
			}
		}

		if eviction := kubeletConfig.EvictionSoft; eviction != nil {
			if out.EvictionSoft == nil {
				out.EvictionSoft = make(map[string]string)
			}

			if val := eviction.MemoryAvailable; val != nil {
				out.EvictionSoft[MemoryAvailable] = *val
			}
			if val := eviction.ImageFSAvailable; val != nil {
				out.EvictionSoft[ImageFSAvailable] = *val
			}
			if val := eviction.ImageFSInodesFree; val != nil {
				out.EvictionSoft[ImageFSInodesFree] = *val
			}
			if val := eviction.NodeFSAvailable; val != nil {
				out.EvictionSoft[NodeFSAvailable] = *val
			}
			if val := eviction.NodeFSInodesFree; val != nil {
				out.EvictionSoft[NodeFSInodesFree] = *val
			}
		}

		if eviction := kubeletConfig.EvictionMinimumReclaim; eviction != nil {
			if out.EvictionMinimumReclaim == nil {
				out.EvictionMinimumReclaim = make(map[string]string)
			}

			if val := eviction.MemoryAvailable; val != nil {
				out.EvictionMinimumReclaim[MemoryAvailable] = val.String()
			}
			if val := eviction.ImageFSAvailable; val != nil {
				out.EvictionMinimumReclaim[ImageFSAvailable] = val.String()
			}
			if val := eviction.ImageFSInodesFree; val != nil {
				out.EvictionMinimumReclaim[ImageFSInodesFree] = val.String()
			}
			if val := eviction.NodeFSAvailable; val != nil {
				out.EvictionMinimumReclaim[NodeFSAvailable] = val.String()
			}
			if val := eviction.NodeFSInodesFree; val != nil {
				out.EvictionMinimumReclaim[NodeFSInodesFree] = val.String()
			}
		}

		if eviction := kubeletConfig.EvictionSoftGracePeriod; eviction != nil {
			if out.EvictionSoftGracePeriod == nil {
				out.EvictionSoftGracePeriod = make(map[string]string)
			}

			if val := eviction.MemoryAvailable; val != nil {
				out.EvictionSoftGracePeriod[MemoryAvailable] = val.Duration.String()
			}
			if val := eviction.ImageFSAvailable; val != nil {
				out.EvictionSoftGracePeriod[ImageFSAvailable] = val.Duration.String()
			}
			if val := eviction.ImageFSInodesFree; val != nil {
				out.EvictionSoftGracePeriod[ImageFSInodesFree] = val.Duration.String()
			}
			if val := eviction.NodeFSAvailable; val != nil {
				out.EvictionSoftGracePeriod[NodeFSAvailable] = val.Duration.String()
			}
			if val := eviction.NodeFSInodesFree; val != nil {
				out.EvictionSoftGracePeriod[NodeFSInodesFree] = val.Duration.String()
			}
		}

		if kubeletConfig.MemorySwap != nil && kubeletConfig.MemorySwap.SwapBehavior != nil {
			out.MemorySwap = &kubeletconfigv1beta1.MemorySwapConfiguration{SwapBehavior: string(*kubeletConfig.MemorySwap.SwapBehavior)}
		}
	}

	return out
}

func reservedFromKubeletConfig(reserved *gardencorev1beta1.KubeletConfigReserved) map[string]string {
	if reserved == nil {
		return nil
	}

	out := make(map[string]string)

	if cpu := reserved.CPU; cpu != nil {
		out["cpu"] = cpu.String()
	}
	if memory := reserved.Memory; memory != nil {
		out["memory"] = memory.String()
	}
	if ephemeralStorage := reserved.EphemeralStorage; ephemeralStorage != nil {
		out["ephemeral-storage"] = ephemeralStorage.String()
	}
	if pid := reserved.PID; pid != nil {
		out["pid"] = pid.String()
	}

	return out
}

// CalculateDataStringForKubeletConfiguration returns a data string for the relevant fields of the kubelet configuration.
func CalculateDataStringForKubeletConfiguration(kubeletConfiguration *gardencorev1beta1.KubeletConfig) []string {
	data := []string{}

	if kubeletConfiguration == nil {
		return data
	}

	if resources := v1beta1helper.SumResourceReservations(kubeletConfiguration.KubeReserved, kubeletConfiguration.SystemReserved); resources != nil {
		data = append(data, fmt.Sprintf("%s-%s-%s-%s", resources.CPU, resources.Memory, resources.PID, resources.EphemeralStorage))
	}
	if eviction := kubeletConfiguration.EvictionHard; eviction != nil {
		data = append(data, fmt.Sprintf("%s-%s-%s-%s-%s",
			ptr.Deref(eviction.ImageFSAvailable, ""),
			ptr.Deref(eviction.ImageFSInodesFree, ""),
			ptr.Deref(eviction.MemoryAvailable, ""),
			ptr.Deref(eviction.NodeFSAvailable, ""),
			ptr.Deref(eviction.NodeFSInodesFree, ""),
		))
	}

	if policy := kubeletConfiguration.CPUManagerPolicy; policy != nil {
		data = append(data, *policy)
	}

	return data
}
