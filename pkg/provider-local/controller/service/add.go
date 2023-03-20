// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package service

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controller/service"
)

// ControllerName is the name of the controller.
const ControllerName = "service"

// DefaultAddOptions are the default AddOptions for AddToManager.
var DefaultAddOptions = AddOptions{}

// AddOptions are options to apply when adding the local infrastructure controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// HostIP is the IP address of the host.
	HostIP string
	// Zone0IP is the IP address to be used for the zone 0 istio ingress gateway.
	Zone0IP string
	// Zone1IP is the IP address to be used for the zone 1 istio ingress gateway.
	Zone1IP string
	// Zone2IP is the IP address to be used for the zone 2 istio ingress gateway.
	Zone2IP string
	// APIServerSNIEnabled states whether the APIServerSNI feature gate of the gardenlet is set to true.
	APIServerSNIEnabled bool
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(mgr manager.Manager, opts AddOptions) error {
	var istioIngressGatewayPredicates []predicate.Predicate
	for _, zone := range []*string{nil, pointer.String("0"), pointer.String("1"), pointer.String("2")} {
		predicate, err := predicate.LabelSelectorPredicate(
			metav1.LabelSelector{MatchExpressions: matchExpressionsIstioIngressGateway(opts.APIServerSNIEnabled, zone)},
		)
		if err != nil {
			return err
		}
		istioIngressGatewayPredicates = append(istioIngressGatewayPredicates, predicate)
	}

	nginxIngressPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: map[string]string{
		"app":       "nginx-ingress",
		"component": "controller",
	}})
	if err != nil {
		return err
	}

	return (&service.Reconciler{
		HostIP:  opts.HostIP,
		Zone0IP: opts.Zone0IP,
		Zone1IP: opts.Zone1IP,
		Zone2IP: opts.Zone2IP,
	}).AddToManager(mgr, predicate.Or(nginxIngressPredicate, predicate.Or(istioIngressGatewayPredicates...)))
}

// AddToManager adds a controller with the default Options.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}

func matchExpressionsIstioIngressGateway(apiServerSNIEnabled bool, zone *string) []metav1.LabelSelectorRequirement {
	if apiServerSNIEnabled {
		istioLabelValue := "ingressgateway"
		if zone != nil {
			istioLabelValue += "--zone--" + *zone
		}

		return []metav1.LabelSelectorRequirement{
			{
				Key:      "app",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"istio-ingressgateway"},
			},
			{
				Key:      "istio",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{istioLabelValue},
			},
		}
	}

	return []metav1.LabelSelectorRequirement{
		{
			Key:      v1beta1constants.LabelApp,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{v1beta1constants.LabelKubernetes},
		},
		{
			Key:      v1beta1constants.LabelRole,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{v1beta1constants.LabelAPIServer},
		},
		{
			Key:      v1beta1constants.LabelAPIServerExposure,
			Operator: metav1.LabelSelectorOpNotIn,
			Values:   []string{v1beta1constants.LabelAPIServerExposureGardenerManaged},
		},
	}
}
