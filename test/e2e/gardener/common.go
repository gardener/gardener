// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"net"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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

// DefaultShoot returns a Shoot object with default values for the e2e tests.
func DefaultShoot(name string) *gardencorev1beta1.Shoot {
	shoot := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				v1beta1constants.AnnotationShootCloudConfigExecutionMaxDelaySeconds: "0",
				v1beta1constants.AnnotationAuthenticationIssuer:                     v1beta1constants.AnnotationAuthenticationIssuerManaged,
			},
		},
		Spec: gardencorev1beta1.ShootSpec{
			ControlPlane:      getShootControlPlane(),
			Region:            "local",
			SecretBindingName: ptr.To("local"),
			CloudProfileName:  "local",
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version:                     "1.30.0",
				EnableStaticTokenKubeconfig: ptr.To(false),
				Kubelet: &gardencorev1beta1.KubeletConfig{
					SerializeImagePulls:   ptr.To(false),
					MaxParallelImagePulls: ptr.To[int32](10),
					RegistryPullQPS:       ptr.To[int32](10),
					RegistryBurst:         ptr.To[int32](20),
				},
				KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
			},
			Networking: &gardencorev1beta1.Networking{
				Type:  ptr.To("calico"),
				Nodes: ptr.To("10.10.0.0/16"),
			},
			Provider: gardencorev1beta1.Provider{
				Type: "local",
				Workers: []gardencorev1beta1.Worker{{
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
				}},
			},
			Extensions: []gardencorev1beta1.Extension{
				{
					Type: "local-ext-seed",
				},
				{
					Type: "local-ext-shoot",
				},
				{
					Type: "local-ext-shoot-after-worker",
				},
			},
		},
	}

	if os.Getenv("IPFAMILY") == "ipv6" {
		shoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}
		shoot.Spec.Networking.Nodes = ptr.To("fd00:10:a::/64")
	}

	return shoot
}

// DefaultWorkerlessShoot returns a workerless Shoot object with default values for the e2e tests.
func DefaultWorkerlessShoot(name string) *gardencorev1beta1.Shoot {
	shoot := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-wl",
		},
		Spec: gardencorev1beta1.ShootSpec{
			ControlPlane:     getShootControlPlane(),
			Region:           "local",
			CloudProfileName: "local",
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version:                     "1.30.0",
				EnableStaticTokenKubeconfig: ptr.To(false),
				KubeAPIServer:               &gardencorev1beta1.KubeAPIServerConfig{},
			},
			Provider: gardencorev1beta1.Provider{
				Type: "local",
			},
			Extensions: []gardencorev1beta1.Extension{
				{
					Type: "local-ext-seed",
				},
				{
					Type: "local-ext-shoot",
				},
			}},
	}

	if os.Getenv("IPFAMILY") == "ipv6" {
		shoot.Spec.Networking = &gardencorev1beta1.Networking{
			IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6},
		}
	}

	return shoot
}

// SetupDNSForMultiZoneTest sets the golang DefaultResolver to the CoreDNS server, which is port forwarded to the host 127.0.0.1:5353.
// Test uses the in-cluster CoreDNS for name resolution and can therefore resolve the API endpoint.
func SetupDNSForMultiZoneTest() {
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := net.Dialer{
				Timeout: time.Duration(5) * time.Second,
			}
			// We use tcp to distinguish easily in-cluster requests (done via udp) and requests from
			// the tests (using tcp). The result for cluster api names differ depending on the source.
			return dialer.DialContext(ctx, "tcp", "127.0.0.1:5353")
		},
	}
}
