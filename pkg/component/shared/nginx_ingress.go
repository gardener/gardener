// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/nginxingress"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// NewNginxIngress returns a deployer for nginx-ingress-controller.
func NewNginxIngress(
	c client.Client,
	namespaceName string,
	targetNamespace string,
	kubernetesVersion *semver.Version,
	config map[string]string,
	loadBalancerAnnotations map[string]string,
	loadBalancerSourceRanges []string,
	priorityClassName string,
	pspDisabled bool,
	vpaEnabled bool,
	clusterType component.ClusterType,
	externalTrafficPolicy corev1.ServiceExternalTrafficPolicyType,
	ingressClass string,
) (
	component.DeployWaiter,
	error,
) {
	imageController, err := imagevector.ImageVector().FindImage(imagevector.ImageNameNginxIngressController, imagevectorutils.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, err
	}
	imageDefaultBackend, err := imagevector.ImageVector().FindImage(imagevector.ImageNameIngressDefaultBackend, imagevectorutils.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, err
	}

	values := nginxingress.Values{
		ImageController:          imageController.String(),
		ImageDefaultBackend:      imageDefaultBackend.String(),
		IngressClass:             ingressClass,
		ConfigData:               config,
		LoadBalancerAnnotations:  loadBalancerAnnotations,
		LoadBalancerSourceRanges: loadBalancerSourceRanges,
		PriorityClassName:        priorityClassName,
		PSPDisabled:              pspDisabled,
		VPAEnabled:               vpaEnabled,
		TargetNamespace:          targetNamespace,
		ClusterType:              clusterType,
		ExternalTrafficPolicy:    externalTrafficPolicy,
	}

	return nginxingress.New(c, namespaceName, values), nil
}
