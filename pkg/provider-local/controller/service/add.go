// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, opts AddOptions) error {
	var istioIngressGatewayPredicates []predicate.Predicate
	for _, zone := range []*string{
		nil,
		ptr.To("0"),
		ptr.To("1"),
		ptr.To("2"),
	} {
		predicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchExpressions: matchExpressionsIstioIngressGateway(zone)})
		if err != nil {
			return err
		}
		istioIngressGatewayPredicates = append(istioIngressGatewayPredicates, predicate)
	}

	isMultiZone, err := HasNodesInMultipleZones(ctx, mgr.GetAPIReader())
	if err != nil {
		return err
	}

	return (&service.Reconciler{
		HostIP:      opts.HostIP,
		Zone0IP:     opts.Zone0IP,
		Zone1IP:     opts.Zone1IP,
		Zone2IP:     opts.Zone2IP,
		IsMultiZone: isMultiZone,
	}).AddToManager(mgr, predicate.Or(istioIngressGatewayPredicates...))
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}

func matchExpressionsIstioIngressGateway(zone *string) []metav1.LabelSelectorRequirement {
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

// HasNodesInMultipleZones indicates whether there are nodes in multiple availability zones or not.
func HasNodesInMultipleZones(ctx context.Context, c client.Reader) (bool, error) {
	nodes := &metav1.PartialObjectMetadataList{}
	nodes.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))
	if err := c.List(ctx, nodes); err != nil {
		return false, err
	}

	var firstZone *string
	for _, node := range nodes.Items {
		if zone := node.Labels[corev1.LabelTopologyZone]; firstZone == nil {
			firstZone = &zone
		} else if *firstZone != zone {
			return true, nil
		}
	}
	return false, nil
}
