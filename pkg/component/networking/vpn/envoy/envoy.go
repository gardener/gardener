// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	volumeNameCerts            = "certs"
	volumeNameEnvoyConfig      = "envoy-config"
	envoyPort                  = 9443
	metricsPort                = 15000
	envoyProxyContainerName    = "envoy-proxy"
	fileNameEnvoyConfig        = "envoy.yaml"
	fileNameCABundle           = "ca.crt"
	volumeMountPathEnvoyConfig = "/etc/envoy"
	volumeMountPathCerts       = "/srv/secrets/envoy-proxy"
)

var (
	tplNameEnvoy = "envoy.yaml.tpl"
	//go:embed templates/envoy.yaml.tpl
	tplContentEnvoy string
	tplEnvoy        *template.Template
)

func init() {
	var err error
	tplEnvoy, err = template.
		New(tplNameEnvoy).
		Parse(tplContentEnvoy)
	utilruntime.Must(err)
}

// GetEnvoyConfig returns the Envoy configuration for the VPN proxy. The same configuration is used both in HA or non-HA mode.
func GetEnvoyConfig() (string, error) {
	values := map[string]any{
		"listenAddress":   "0.0.0.0",
		"listenAddressV6": "::",
		"dnsLookupFamily": "ALL",
		"envoyPort":       fmt.Sprintf("%d", envoyPort),
		"certChain":       volumeMountPathCerts + `/` + secrets.DataKeyCertificate,
		"privateKey":      volumeMountPathCerts + `/` + secrets.DataKeyPrivateKey,
		"caCert":          volumeMountPathCerts + `/` + fileNameCABundle,
		"metricsPort":     fmt.Sprintf("%d", metricsPort),
	}

	var envoyConfig strings.Builder
	err := tplEnvoy.Execute(&envoyConfig, values)
	if err != nil {
		return "", err
	}

	return envoyConfig.String(), nil
}

// GetEnvoyProxyContainer returns the Envoy proxy container configuration. It is used with Kube-ApiServer (HA) and VPN-SeedServer (non-HA).
func GetEnvoyProxyContainer(image string) *corev1.Container {
	return &corev1.Container{
		Name:            envoyProxyContainerName,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"envoy",
			"--concurrency",
			"2",
			"-c",
			fmt.Sprintf("%s/%s", volumeMountPathEnvoyConfig, fileNameEnvoyConfig),
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(envoyPort),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(envoyPort),
				},
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("850M"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"all",
				},
			},
			RunAsGroup:   ptr.To(int64(v1beta1constants.EnvoyVPNGroupId)),
			RunAsNonRoot: ptr.To(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeNameCerts,
				MountPath: volumeMountPathCerts,
			},
			{
				Name:      volumeNameEnvoyConfig,
				MountPath: volumeMountPathEnvoyConfig,
			},
		},
	}
}
