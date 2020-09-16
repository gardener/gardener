// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"context"
	"path/filepath"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ingress struct {
	values    *IngressValues
	namespace string
	kubernetes.ChartApplier
	chartPath string
	client    crclient.Client
}

// IngressValues holds values for the istio-ingress chart.
// The only opened port is 15021.
type IngressValues struct {
	TrustDomain     string            `json:"trustDomain,omitempty"`
	Image           string            `json:"image,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	IstiodNamespace string            `json:"istiodNamespace,omitempty"`
	// Ports is a list of all Ports the istio-ingress gateways is listening on.
	// Port 15021 and 15000 cannot be used.
	Ports []corev1.ServicePort `json:"ports,omitempty"`
}

// NewIngressGateway creates a new DeployWaiter for istio ingress gateway in
// "istio-ingress" namespace.
// It only supports Deploy. Destroy does nothing.
func NewIngressGateway(
	values *IngressValues,
	namespace string,
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	client crclient.Client,
) component.DeployWaiter {
	return &ingress{
		values:       values,
		namespace:    namespace,
		ChartApplier: applier,
		chartPath:    filepath.Join(chartsRootPath, istioReleaseName, "istio-ingress"),
		client:       client,
	}
}

func (i *ingress) Deploy(ctx context.Context) error {
	if err := i.client.Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: i.namespace,
				Labels: map[string]string{
					"istio-operator-managed": "Reconcile",
					"istio-injection":        "disabled",
				},
			},
		},
	); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	applierOptions := kubernetes.CopyApplierOptions(kubernetes.DefaultMergeFuncs)
	applierOptions[appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind()] = kubernetes.DeploymentKeepReplicasMergeFunc

	return i.Apply(ctx, i.chartPath, i.namespace, istioReleaseName, kubernetes.Values(i.values), applierOptions)
}

func (i *ingress) Destroy(ctx context.Context) error {
	// istio cannot be safely removed
	return nil
}

func (i *ingress) Wait(ctx context.Context) error {
	return nil
}

func (i *ingress) WaitCleanup(ctx context.Context) error {
	return nil
}
