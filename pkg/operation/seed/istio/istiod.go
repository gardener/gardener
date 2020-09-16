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

type istiod struct {
	namespace string
	values    *IstiodValues
	kubernetes.ChartApplier
	chartPath string
	client    crclient.Client
}

type IstiodValues struct {
	TrustDomain string `json:"trustDomain,omitempty"`
	Image       string `json:"image,omitempty"`
}

// NewIstiod can be used to deploy istio's istiod in a namespace.
// Destroy does nothing.
func NewIstiod(
	values *IstiodValues,
	namespace string,
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	client crclient.Client,
) component.DeployWaiter {
	return &istiod{
		values:       values,
		namespace:    namespace,
		ChartApplier: applier,
		chartPath:    filepath.Join(chartsRootPath, istioReleaseName, "istio-istiod"),
		client:       client,
	}
}

func (i *istiod) Deploy(ctx context.Context) error {
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

func (i *istiod) Destroy(ctx context.Context) error {
	// istio cannot be safely removed
	return nil
}

func (i *istiod) Wait(ctx context.Context) error {
	return nil
}

func (i *istiod) WaitCleanup(ctx context.Context) error {
	return nil
}
