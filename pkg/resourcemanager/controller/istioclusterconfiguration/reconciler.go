// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istioclusterconfiguration

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
	istioapiannotation "istio.io/api/annotation"
	istioapinetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/resourcemanager/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	istioutils "github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	serviceFQDNSuffix = ".svc." + gardencorev1beta1.DefaultDomain

	perConnectionBufferLimitBytes    = 32768
	http2InitialStreamWindowSize     = 65536
	http2InitialConnectionWindowSize = 1048576

	managedByLabelKey   = "resources.gardener.cloud/managed-by"
	managedByLabelValue = "istio-cluster-configuration"

	httpProtocolOptionsType = "type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions"
)

// Reconciler reconciles namespaces that contain DestinationRules and creates EnvoyFilters in istio-ingress namespaces.
type Reconciler struct {
	TargetClient client.Client
	Config       resourcemanagerconfigv1alpha1.IstioClusterConfigurationControllerConfig
}

// Reconcile performs the reconciliation for a source namespace containing DestinationRules.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	sourceNamespace := &corev1.Namespace{}
	if err := r.TargetClient.Get(ctx, types.NamespacedName{Name: request.Name}, sourceNamespace); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Source namespace is gone, nothing to do")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log.Info("Reconciling namespace")

	// List all DestinationRules in the source namespace.
	destinationRules := &istionetworkingv1beta1.DestinationRuleList{}
	if err := r.TargetClient.List(ctx, destinationRules, client.InNamespace(sourceNamespace.Name)); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list DestinationRules in namespace %s: %w", sourceNamespace.Name, err)
	}

	log.V(3).Info("Found DestinationRules", "count", len(destinationRules.Items))

	// Find all istio-ingress namespaces.
	istioIngressNamespaces, err := r.getIstioIngressNamespaces(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, r.reconcileEnvoyFilters(ctx, destinationRules.Items, sourceNamespace, istioIngressNamespaces)
}

func (r *Reconciler) getIstioIngressNamespaces(ctx context.Context) ([]string, error) {
	istioIngressNamespaces := &corev1.NamespaceList{}
	if err := r.TargetClient.List(ctx, istioIngressNamespaces, client.MatchingLabels{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress,
	}); err != nil {
		return nil, fmt.Errorf("failed to list istio-ingress namespaces: %w", err)
	}

	exposureClassNamespaces := &corev1.NamespaceList{}
	if err := r.TargetClient.List(ctx, exposureClassNamespaces, client.HasLabels{
		v1beta1constants.LabelExposureClassHandlerName,
	}); err != nil {
		return nil, fmt.Errorf("failed to list exposure class namespaces: %w", err)
	}

	namespaceSet := sets.NewString()
	for _, namespace := range istioIngressNamespaces.Items {
		namespaceSet.Insert(namespace.Name)
	}
	for _, namespace := range exposureClassNamespaces.Items {
		namespaceSet.Insert(namespace.Name)
	}

	namespaces := namespaceSet.UnsortedList()
	slices.Sort(namespaces)

	return namespaces, nil
}

func (r *Reconciler) reconcileEnvoyFilters(ctx context.Context, destinationRules []*istionetworkingv1beta1.DestinationRule, sourceNamespace *corev1.Namespace, istioIngressNamespaces []string) error {
	envoyConfigPatches := map[string][]*istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{}

	for _, destinationRule := range destinationRules {
		log := logf.FromContext(ctx).WithValues("destinationRule", destinationRule.Name, "namespace", destinationRule.Namespace)

		targetNamespaces := resolveExportTo(destinationRule.Spec.ExportTo, destinationRule.Namespace, istioIngressNamespaces)

		// We don't use subsets and workload selectors, so we skip those cases to simplify the logic.
		if len(destinationRule.Spec.Subsets) > 0 {
			log.V(1).Info("DestinationRule has subsets, skipping")
			continue
		}
		if destinationRule.Spec.WorkloadSelector != nil {
			log.V(1).Info("DestinationRule has workloadSelector, skipping")
			continue
		}

		service, err := r.getServiceForDestinationRule(ctx, destinationRule)
		if err != nil {
			return fmt.Errorf("error getting service for DestinationRule: %w", err)
		}

		if service == nil {
			continue
		}

		serviceExportTo := strings.Split(service.Annotations[istioapiannotation.NetworkingExportTo.Name], ",")
		serviceTargetNamespaces := resolveExportTo(serviceExportTo, service.Namespace, targetNamespaces)

		for _, port := range service.Spec.Ports {
			protocolMode := istioutils.DetermineProtocolMode(destinationRule, port)
			clusterName := fmt.Sprintf("outbound|%d||%s", port.Port, kubernetesutils.FQDNForService(service.Name, service.Namespace))
			patch := getEnvoyConfigPatch(clusterName, protocolMode)

			for _, targetNamespace := range targetNamespaces {
				if !slices.Contains(serviceTargetNamespaces, targetNamespace) {
					log.V(1).Info("Service is not exported to target namespace, skipping", "service", service.Name, "namespace", service.Namespace, "targetNamespace", targetNamespace)
					continue
				}

				envoyConfigPatches[targetNamespace] = append(envoyConfigPatches[targetNamespace], patch)
			}
		}
	}

	ownerNamespaceGVK, err := apiutil.GVKForObject(sourceNamespace, kubernetes.SeedScheme)
	if err != nil {
		return fmt.Errorf("failed to get GVK for namespace %q: %w", sourceNamespace.Name, err)
	}
	ownerReference := metav1.NewControllerRef(sourceNamespace, ownerNamespaceGVK)

	for _, istioIngressNamespace := range istioIngressNamespaces {
		log := logf.FromContext(ctx).WithValues("targetNamespace", istioIngressNamespace)

		envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getEnvoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace,
			},
		}

		switch len(envoyConfigPatches[istioIngressNamespace]) {
		case 0:
			if err := r.TargetClient.Delete(ctx, envoyFilter); err == nil {
				log.V(1).Info("No active DestinationRule in sourceNamespace for targetNamespace, EnvoyFilter deleted", "sourceNamespace", sourceNamespace.Name, "envoyFilter", envoyFilter.Name)
			} else if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete EnvoyFilter %s/%s: %w", envoyFilter.Namespace, envoyFilter.Name, err)
			}
		default:
			if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.TargetClient, envoyFilter, func() error {
				envoyFilter.Labels = map[string]string{
					managedByLabelKey: managedByLabelValue,
				}
				envoyFilter.OwnerReferences = []metav1.OwnerReference{*ownerReference}
				envoyFilter.Spec = istioapinetworkingv1alpha3.EnvoyFilter{
					ConfigPatches: envoyConfigPatches[istioIngressNamespace],
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to create/update EnvoyFilter %s/%s: %w", istioIngressNamespace, envoyFilter.Name, err)
			}

			log.V(1).Info("Reconciled EnvoyFilter", "envoyFilter", envoyFilter.Name)
		}
	}

	return nil
}

func (r *Reconciler) getServiceForDestinationRule(ctx context.Context, destinationRule *istionetworkingv1beta1.DestinationRule) (*corev1.Service, error) {
	log := logf.FromContext(ctx).WithValues("destinationRule", destinationRule.Name, "namespace", destinationRule.Namespace)

	var serviceNamespacedName types.NamespacedName

	switch {
	case !strings.Contains(destinationRule.Spec.Host, "."):
		serviceNamespacedName.Name = destinationRule.Spec.Host
		serviceNamespacedName.Namespace = destinationRule.Namespace
	case strings.HasSuffix(destinationRule.Spec.Host, serviceFQDNSuffix):
		hostNoSuffix := strings.TrimSuffix(destinationRule.Spec.Host, serviceFQDNSuffix)
		nameNamespace := strings.Split(hostNoSuffix, ".")

		if len(nameNamespace) != 2 {
			log.V(1).Info("DestinationRule has no valid service host", "host", destinationRule.Spec.Host)
			return nil, nil
		}

		serviceNamespacedName.Name = nameNamespace[0]
		serviceNamespacedName.Namespace = nameNamespace[1]
	default:
		log.V(1).Info("DestinationRule has non-cluster-internal service host", "host", destinationRule.Spec.Host)
		return nil, nil
	}

	service := &corev1.Service{}
	if err := r.TargetClient.Get(ctx, serviceNamespacedName, service); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("error retrieving object from store: %w", err)
		}

		log.V(1).Info("Service configured in DestinationRule host not found", "service", serviceNamespacedName)
		return nil, nil
	}

	return service, nil
}

func getEnvoyConfigPatch(clusterName string, httpProtocolPolicy istioutils.HTTPProtocolPolicy) *istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch {
	envoyConfigPatch := &istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: istioapinetworkingv1alpha3.EnvoyFilter_CLUSTER,
		Match: &istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: istioapinetworkingv1alpha3.EnvoyFilter_GATEWAY,
			ObjectTypes: &istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Cluster{
				Cluster: &istioapinetworkingv1alpha3.EnvoyFilter_ClusterMatch{
					Name: clusterName,
				},
			},
		},
		Patch: &istioapinetworkingv1alpha3.EnvoyFilter_Patch{
			Operation: istioapinetworkingv1alpha3.EnvoyFilter_Patch_MERGE,
			Value: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"per_connection_buffer_limit_bytes": structpb.NewNumberValue(perConnectionBufferLimitBytes),
				},
			},
		},
	}

	http2Options := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"initial_stream_window_size":     structpb.NewNumberValue(http2InitialStreamWindowSize),
			"initial_connection_window_size": structpb.NewNumberValue(http2InitialConnectionWindowSize),
		},
	}

	var protocolOptions *structpb.Struct
	switch httpProtocolPolicy {
	case istioutils.HTTPProtocolPolicyExplicitHTTP2:
		protocolOptions = &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"@type":                        structpb.NewStringValue(httpProtocolOptionsType),
				"common_http_protocol_options": structpb.NewStructValue(&structpb.Struct{}),
				"explicit_http_config": structpb.NewStructValue(&structpb.Struct{
					Fields: map[string]*structpb.Value{
						"http2_protocol_options": structpb.NewStructValue(http2Options),
					},
				}),
			},
		}
	case istioutils.HTTPProtocolPolicyUseClientProtocol:
		protocolOptions = &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"@type":                        structpb.NewStringValue(httpProtocolOptionsType),
				"common_http_protocol_options": structpb.NewStructValue(&structpb.Struct{}),
				"use_downstream_protocol_config": structpb.NewStructValue(&structpb.Struct{
					Fields: map[string]*structpb.Value{
						"http2_protocol_options": structpb.NewStructValue(http2Options),
					},
				}),
			},
		}
	default:
		return envoyConfigPatch
	}

	envoyConfigPatch.Patch.Value.Fields["typed_extension_protocol_options"] = structpb.NewStructValue(&structpb.Struct{
		Fields: map[string]*structpb.Value{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": structpb.NewStructValue(protocolOptions),
		},
	})

	return envoyConfigPatch
}

func getEnvoyFilterName(sourceNamespace string) string {
	return sourceNamespace + "-cluster-configuration"
}

func resolveExportTo(exportTo []string, sourceNamespace string, istioIngressNamespaces []string) []string {
	if len(exportTo) == 0 {
		// "~" is the default export for services and destination rules in Gardener.
		// See https://github.com/gardener/gardener/blob/master/pkg/component/networking/istio/charts/istio/istio-istiod/templates/configmap.yaml
		return []string{}
	}

	var targetNamespaces []string
	for _, export := range exportTo {
		switch export {
		case "~":
			return []string{}
		case "*":
			return istioIngressNamespaces
		case ".":
			if slices.Contains(istioIngressNamespaces, sourceNamespace) {
				targetNamespaces = append(targetNamespaces, sourceNamespace)
			}
		default:
			if slices.Contains(istioIngressNamespaces, export) {
				targetNamespaces = append(targetNamespaces, export)
			}
		}
	}
	return targetNamespaces
}
