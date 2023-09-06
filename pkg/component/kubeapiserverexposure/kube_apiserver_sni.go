// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserverexposure

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	//go:embed templates/envoyfilter.yaml
	envoyFilterSpecTemplateContent string
	envoyFilterSpecTemplate        *template.Template
)

func init() {
	envoyFilterSpecTemplate = template.Must(template.
		New("envoy-filter-spec").
		Funcs(sprig.TxtFuncMap()).
		Parse(envoyFilterSpecTemplateContent),
	)
}

// SNIValues configure the kube-apiserver service SNI.
type SNIValues struct {
	Hosts               []string
	APIServerProxy      *APIServerProxy
	IstioIngressGateway IstioIngressGateway
}

// APIServerProxy contains values for the APIServer proxy protocol configuration.
type APIServerProxy struct {
	NamespaceUID       types.UID
	APIServerClusterIP string
}

// IstioIngressGateway contains the values for istio ingress gateway configuration.
type IstioIngressGateway struct {
	Namespace string
	Labels    map[string]string
}

// NewSNI creates a new instance of DeployWaiter which deploys Istio resources for
// kube-apiserver SNI access.
func NewSNI(
	client client.Client,
	applier kubernetes.Applier,
	name string,
	namespace string,
	valuesFunc func() *SNIValues,
) component.DeployWaiter {
	if valuesFunc == nil {
		valuesFunc = func() *SNIValues { return &SNIValues{} }
	}

	return &sni{
		client:     client,
		applier:    applier,
		name:       name,
		namespace:  namespace,
		valuesFunc: valuesFunc,
	}
}

type sni struct {
	client     client.Client
	applier    kubernetes.Applier
	name       string
	namespace  string
	valuesFunc func() *SNIValues
}

type envoyFilterTemplateValues struct {
	*APIServerProxy
	IngressGatewayLabels map[string]string
	Name                 string
	Namespace            string
	Host                 string
	Port                 int
}

func (s *sni) Deploy(ctx context.Context) error {
	var (
		values = s.valuesFunc()

		destinationRule = s.emptyDestinationRule()
		gateway         = s.emptyGateway()
		virtualService  = s.emptyVirtualService()

		hostName        = fmt.Sprintf("%s.%s.svc.%s", s.name, s.namespace, gardencorev1beta1.DefaultDomain)
		envoyFilterSpec bytes.Buffer
	)

	if values.APIServerProxy != nil {
		envoyFilter := s.emptyEnvoyFilter()

		if err := envoyFilterSpecTemplate.Execute(&envoyFilterSpec, envoyFilterTemplateValues{
			APIServerProxy:       values.APIServerProxy,
			IngressGatewayLabels: values.IstioIngressGateway.Labels,
			Name:                 envoyFilter.Name,
			Namespace:            envoyFilter.Namespace,
			Host:                 hostName,
			Port:                 kubeapiserverconstants.Port,
		}); err != nil {
			return err
		}
		if err := s.applier.ApplyManifest(ctx, kubernetes.NewManifestReader(envoyFilterSpec.Bytes()), kubernetes.DefaultMergeFuncs); err != nil {
			return err
		}
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, destinationRule, istio.DestinationRuleWithLocalityPreference(destinationRule, getLabels(), hostName)); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, gateway, istio.GatewayWithTLSPassthrough(gateway, getLabels(), s.valuesFunc().IstioIngressGateway.Labels, s.valuesFunc().Hosts, kubeapiserverconstants.Port)); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, virtualService, istio.VirtualServiceWithSNIMatch(virtualService, getLabels(), s.valuesFunc().Hosts, gateway.Name, kubeapiserverconstants.Port, hostName)); err != nil {
		return err
	}

	return nil
}

func (s *sni) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(
		ctx,
		s.client,
		s.emptyDestinationRule(),
		s.emptyEnvoyFilter(),
		s.emptyGateway(),
		s.emptyVirtualService(),
	)
}

func (s *sni) Wait(_ context.Context) error        { return nil }
func (s *sni) WaitCleanup(_ context.Context) error { return nil }

func (s *sni) emptyDestinationRule() *istionetworkingv1beta1.DestinationRule {
	return &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: s.name, Namespace: s.namespace}}
}

func (s *sni) emptyEnvoyFilter() *istionetworkingv1alpha3.EnvoyFilter {
	return &istionetworkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Name: s.namespace, Namespace: s.valuesFunc().IstioIngressGateway.Namespace}}
}

func (s *sni) emptyGateway() *istionetworkingv1beta1.Gateway {
	return &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: s.name, Namespace: s.namespace}}
}

func (s *sni) emptyVirtualService() *istionetworkingv1beta1.VirtualService {
	return &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: s.name, Namespace: s.namespace}}
}
