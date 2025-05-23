// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverexposure

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
)

type internalNameService struct {
	client    client.Client
	namespace string
}

// NewInternalNameService creates a new instance of Deployer for the service kubernetes.default.svc.cluster.local.
func NewInternalNameService(c client.Client, namespace string) component.Deployer {
	return &internalNameService{
		client:    c,
		namespace: namespace,
	}
}

func (in *internalNameService) Deploy(ctx context.Context) error {
	svc := in.emptyKubernetesDefaultService()

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, in.client, svc, func() error {
		metav1.SetMetaDataAnnotation(&svc.ObjectMeta, "networking.istio.io/exportTo", "*")
		return nil
	}); err != nil {
		return err
	}
	return client.IgnoreNotFound(in.client.Delete(ctx, in.emptyService()))
}

func (in *internalNameService) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(in.client.Delete(ctx, in.emptyService()))
}

func (in *internalNameService) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: in.namespace}}
}

func (in *internalNameService) emptyKubernetesDefaultService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: metav1.NamespaceDefault}}
}
