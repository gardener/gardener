// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (b *Botanist) newKubeAPIServiceServiceComponent(sniPhase component.Phase) component.DeployWaiter {
	var sniServiceKey = client.ObjectKey{Name: *b.Config.SNI.Ingress.ServiceName, Namespace: *b.Config.SNI.Ingress.Namespace}
	if b.ExposureClassHandler != nil {
		sniServiceKey.Name = *b.ExposureClassHandler.SNI.Ingress.ServiceName
		sniServiceKey.Namespace = *b.ExposureClassHandler.SNI.Ingress.Namespace
	}

	return kubeapiserverexposure.NewService(
		b.Logger,
		b.K8sSeedClient.Client(),
		&kubeapiserverexposure.ServiceValues{
			Annotations: b.getKubeAPIServerServiceAnnotations(sniPhase),
			SNIPhase:    sniPhase,
		},
		client.ObjectKey{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace},
		sniServiceKey,
		nil,
		b.setAPIServerServiceClusterIP,
		func(address string) {
			b.APIServerAddress = address
			b.newDNSComponentsTargetingAPIServerAddress()
		},
	)
}

// DefaultKubeAPIServerService returns a deployer for the kube-apiserver service.
func (b *Botanist) DefaultKubeAPIServerService(sniPhase component.Phase) component.DeployWaiter {
	return b.newKubeAPIServiceServiceComponent(sniPhase)
}

// DeployKubeAPIService deploys the kube-apiserver service.
func (b *Botanist) DeployKubeAPIService(ctx context.Context, sniPhase component.Phase) error {
	return b.newKubeAPIServiceServiceComponent(sniPhase).Deploy(ctx)
}

func (b *Botanist) getKubeAPIServerServiceAnnotations(sniPhase component.Phase) map[string]string {
	if b.ExposureClassHandler != nil && sniPhase != component.PhaseEnabled {
		return utils.MergeStringMaps(b.Seed.LoadBalancerServiceAnnotations, b.ExposureClassHandler.LoadBalancerService.Annotations)
	}
	return b.Seed.LoadBalancerServiceAnnotations
}

// APIServerSNIEnabled returns true if APIServerSNI feature gate is enabled and the shoot uses internal and external
// DNS.
func (b *Botanist) APIServerSNIEnabled() bool {
	return gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) && b.NeedsInternalDNS() && b.NeedsExternalDNS()
}

// DefaultKubeAPIServerSNI returns a deployer for the kube-apiserver SNI.
func (b *Botanist) DefaultKubeAPIServerSNI() component.DeployWaiter {
	return component.OpDestroy(kubeapiserverexposure.NewSNI(
		b.K8sSeedClient.Client(),
		b.K8sSeedClient.Applier(),
		b.Shoot.SeedNamespace,
		&kubeapiserverexposure.SNIValues{
			IstioIngressGateway:      b.getIngressGatewayConfig(),
			APIServerInternalDNSName: b.outOfClusterAPIServerFQDN(),
		},
	))
}

// DeployKubeAPIServerSNI deploys the kube-apiserver-sni chart.
func (b *Botanist) DeployKubeAPIServerSNI(ctx context.Context) error {
	return b.Shoot.Components.ControlPlane.KubeAPIServerSNI.Deploy(ctx)
}

func (b *Botanist) getIngressGatewayConfig() kubeapiserverexposure.IstioIngressGateway {
	ingressGatewayConfig := kubeapiserverexposure.IstioIngressGateway{
		Namespace: *b.Config.SNI.Ingress.Namespace,
		Labels:    b.Config.SNI.Ingress.Labels,
	}

	if b.ExposureClassHandler != nil {
		ingressGatewayConfig.Namespace = *b.ExposureClassHandler.SNI.Ingress.Namespace
		ingressGatewayConfig.Labels = gutil.GetMandatoryExposureClassHandlerSNILabels(b.ExposureClassHandler.SNI.Ingress.Labels, b.ExposureClassHandler.Name)
	}

	return ingressGatewayConfig
}

// SNIPhase returns the current phase of the SNI enablement of kube-apiserver's service.
func (b *Botanist) SNIPhase(ctx context.Context) (component.Phase, error) {
	var (
		svc        = &corev1.Service{}
		sniEnabled = b.APIServerSNIEnabled()
	)

	if err := b.K8sSeedClient.APIReader().Get(
		ctx,
		client.ObjectKey{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace},
		svc,
	); client.IgnoreNotFound(err) != nil {
		return component.PhaseUnknown, err
	}

	switch {
	case svc.Spec.Type == corev1.ServiceTypeLoadBalancer && sniEnabled:
		return component.PhaseEnabling, nil
	case svc.Spec.Type == corev1.ServiceTypeClusterIP && sniEnabled:
		return component.PhaseEnabled, nil
	case svc.Spec.Type == corev1.ServiceTypeClusterIP && !sniEnabled:
		return component.PhaseDisabling, nil
	default:
		if sniEnabled {
			// initial cluster creation with SNI enabled (enabling only relevant for migration).
			return component.PhaseEnabled, nil
		}
		// initial cluster creation with SNI disabled.
		return component.PhaseDisabled, nil
	}
}

func (b *Botanist) setAPIServerServiceClusterIP(clusterIP string) {
	if b.Shoot.Components.ControlPlane.KubeAPIServerSNIPhase == component.PhaseDisabled {
		return
	}

	b.APIServerClusterIP = clusterIP
	b.Shoot.Components.ControlPlane.KubeAPIServerSNI = kubeapiserverexposure.NewSNI(
		b.K8sSeedClient.Client(),
		b.K8sSeedClient.Applier(),
		b.Shoot.SeedNamespace,
		&kubeapiserverexposure.SNIValues{
			APIServerClusterIP: clusterIP,
			NamespaceUID:       b.SeedNamespaceObject.UID,
			Hosts: []string{
				gutil.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
				gutil.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
			},
			IstioIngressGateway:      b.getIngressGatewayConfig(),
			APIServerInternalDNSName: b.outOfClusterAPIServerFQDN(),
		},
	)
}
