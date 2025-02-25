// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/timewindow"
	"github.com/gardener/gardener/test/framework"
)

// DefaultGardenConfig returns a GardenerConfig framework object with default values for the e2e tests.
func DefaultGardenConfig(projectNamespace string) *framework.GardenerConfig {
	return &framework.GardenerConfig{
		CommonConfig: &framework.CommonConfig{
			DisableStateDump: true,
		},
		ProjectNamespace:   projectNamespace,
		GardenerKubeconfig: os.Getenv("KUBECONFIG"),
	}
}

func baseShoot(name string) *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "garden-local",
		},
		Spec: gardencorev1beta1.ShootSpec{
			ControlPlane: getShootControlPlane(),
			Region:       "local",
			CloudProfile: &gardencorev1beta1.CloudProfileReference{
				Name: "local",
			},
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version:                     "1.32.2",
				EnableStaticTokenKubeconfig: ptr.To(false),
				KubeAPIServer:               &gardencorev1beta1.KubeAPIServerConfig{},
			},
			Provider: gardencorev1beta1.Provider{
				Type: "local",
			},
			Extensions: []gardencorev1beta1.Extension{
				{Type: "local-ext-seed"},
				{Type: "local-ext-shoot"},
			},
			Maintenance: getDelayedShootMaintenance(),
		},
	}
}

// DefaultShoot returns a Shoot object with default values for the e2e tests.
func DefaultShoot(name string) *gardencorev1beta1.Shoot {
	shoot := baseShoot(name)

	metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootCloudConfigExecutionMaxDelaySeconds, "0")
	metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationAuthenticationIssuer, v1beta1constants.AnnotationAuthenticationIssuerManaged)

	shoot.Spec.SecretBindingName = ptr.To("local")
	shoot.Spec.Kubernetes.Kubelet = &gardencorev1beta1.KubeletConfig{
		SerializeImagePulls: ptr.To(false),
		RegistryPullQPS:     ptr.To[int32](10),
		RegistryBurst:       ptr.To[int32](20),
	}
	shoot.Spec.Networking = &gardencorev1beta1.Networking{
		Type:  ptr.To("calico"),
		Nodes: ptr.To("10.10.0.0/16"),
	}
	shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{
		Name: "local",
		Machine: gardencorev1beta1.Machine{
			Type: "local",
		},
		CRI: &gardencorev1beta1.CRI{
			Name: gardencorev1beta1.CRINameContainerD,
		},
		Labels: map[string]string{
			"foo": "bar",
		},
		Minimum: 1,
		Maximum: 1,
	})
	shoot.Spec.Extensions = append(shoot.Spec.Extensions, gardencorev1beta1.Extension{Type: "local-ext-shoot-after-worker"})

	if os.Getenv("IPFAMILY") == "ipv6" {
		shoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}
		shoot.Spec.Networking.Nodes = ptr.To("fd00:10:a::/64")
		shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{"ipv6":{"sourceNATEnabled":true}}`)}
	}

	return shoot
}

// DefaultWorkerlessShoot returns a workerless Shoot object with default values for the e2e tests.
func DefaultWorkerlessShoot(name string) *gardencorev1beta1.Shoot {
	shoot := baseShoot(name + "-wl")

	if os.Getenv("IPFAMILY") == "ipv6" {
		shoot.Spec.Networking = &gardencorev1beta1.Networking{
			IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6},
		}
	}

	return shoot
}

// DefaultNamespacedCloudProfile returns a NamespacedCloudProfile object with default values for the e2e tests.
func DefaultNamespacedCloudProfile() *gardencorev1beta1.NamespacedCloudProfile {
	return &gardencorev1beta1.NamespacedCloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-profile",
			Namespace: "garden-local",
		},
		Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
			Parent: gardencorev1beta1.CloudProfileReference{
				Kind: "CloudProfile",
				Name: "local",
			},
		},
	}
}

// getShootControlPlane returns a ControlPlane object based on env variable SHOOT_FAILURE_TOLERANCE_TYPE value
func getShootControlPlane() *gardencorev1beta1.ControlPlane {
	var failureToleranceType gardencorev1beta1.FailureToleranceType

	switch os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE") {
	case "zone":
		failureToleranceType = gardencorev1beta1.FailureToleranceTypeZone
	case "node":
		failureToleranceType = gardencorev1beta1.FailureToleranceTypeNode
	default:
		return nil
	}

	return &gardencorev1beta1.ControlPlane{
		HighAvailability: &gardencorev1beta1.HighAvailability{
			FailureTolerance: gardencorev1beta1.FailureTolerance{
				Type: failureToleranceType,
			},
		},
	}
}

// This computes a time window for Shoot maintenance which is ensured to be at least 2 hours in the future.
// This is to prevent that we create Shoots in our e2e tests which are immediately in maintenance, since this can cause
// that they might be reconciled multiple times (e.g., when gardenlet restarts). This might be undesired in some
// test cases (e.g., upgrade tests).
func getDelayedShootMaintenance() *gardencorev1beta1.Maintenance {
	hour := (time.Now().UTC().Hour() + 3) % 24

	return &gardencorev1beta1.Maintenance{TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
		Begin: timewindow.NewMaintenanceTime(hour, 0, 0).Formatted(),
		End:   timewindow.NewMaintenanceTime((hour+1)%24, 0, 0).Formatted(),
	}}
}
