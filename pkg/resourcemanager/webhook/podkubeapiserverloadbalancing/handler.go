// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package podkubeapiserverloadbalancing

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	// HostsConfigMapKey defines the key in the configmap that contains the kube-apiserver hosts.
	HostsConfigMapKey = "hosts"
	// IstioNamespaceConfigMapKey defines the key in the configmap that contains the namespace of the istio-ingressgateway service.
	IstioNamespaceConfigMapKey = "istio-namespace"
	// IstioInternalLoadBalancingConfigMapName defines the name of the configmap that contains the kube-apiserver hosts and istio namespace.
	IstioInternalLoadBalancingConfigMapName = "istio-internal-load-balancing"
)

// Handler handles admission requests and sets host aliases and network policy label to istio-ingressgateway in Pod resources.
type Handler struct {
	Logger       logr.Logger
	TargetClient client.Reader
}

// Default defaults the scheduler name of the provided pod.
func (h *Handler) Default(ctx context.Context, obj runtime.Object) error {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("expected *corev1.Pod but got %T", obj)
	}

	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: pod.Namespace, Name: IstioInternalLoadBalancingConfigMapName}}
	if err := h.TargetClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get istio-internal-load-balancing configmap: %w", err)
	}

	hosts := strings.Split(configMap.Data[HostsConfigMapKey], ",")
	istioNamespace := configMap.Data[IstioNamespaceConfigMapKey]

	istioService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: istioNamespace, Name: v1beta1constants.InternalSNIIngressServiceName}}
	if err := h.TargetClient.Get(ctx, client.ObjectKeyFromObject(istioService), istioService); err != nil {
		return fmt.Errorf("failed to get internal istio-ingressgateway service: %w", err)
	}

	for _, clusterIP := range istioService.Spec.ClusterIPs {
		pod.Spec.HostAliases = append(pod.Spec.HostAliases, corev1.HostAlias{
			IP:        clusterIP,
			Hostnames: hosts,
		})
		h.Logger.Info("Added host alias to pod", "pod", client.ObjectKeyFromObject(pod), "ip", clusterIP, "hostnames", hosts)
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[gardenerutils.NetworkPolicyLabel("all-istio-ingresses-istio-ingressgateway-internal", 9443)] = v1beta1constants.LabelNetworkPolicyAllowed
	h.Logger.Info("Added network policy label for all istio ingresses to pod", "pod", client.ObjectKeyFromObject(pod))

	return nil
}
