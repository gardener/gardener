// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/provider-local/local"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

// NewEnsurer creates cloudprovider ensurer.
func NewEnsurer(restConfig *rest.Config, logger logr.Logger) cloudprovider.Ensurer {
	return &ensurer{
		restConfig: restConfig,
		logger:     logger,
	}
}

type ensurer struct {
	restConfig *rest.Config
	logger     logr.Logger
}

// EnsureCloudProviderSecret ensures that cloudprovider secret contains
// the shared credentials file.
func (e *ensurer) EnsureCloudProviderSecret(_ context.Context, _ gcontext.GardenContext, newSecret, _ *corev1.Secret) error {
	if newSecret.Labels != nil && newSecret.Labels[securityv1alpha1constants.LabelWorkloadIdentityProvider] == local.Type {
		rawKubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig("mcm",
			clientcmdv1.Cluster{
				Server:               e.restConfig.Host,
				CertificateAuthority: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
			},
			clientcmdv1.AuthInfo{
				Token: string(newSecret.Data["token"]),
			},
		))
		if err != nil {
			return err
		}

		if newSecret.Data == nil {
			newSecret.Data = make(map[string][]byte)
		}
		newSecret.Data["kubeconfig"] = rawKubeconfig
		return nil
	}

	return nil
}
