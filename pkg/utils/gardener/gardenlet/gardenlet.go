// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

// SeedIsGarden returns 'true' if the cluster is registered as a Garden cluster.
func SeedIsGarden(ctx context.Context, seedClient client.Reader) (bool, error) {
	seedIsGarden, err := kubernetesutils.ResourcesExist(ctx, seedClient, &operatorv1alpha1.GardenList{}, operatorclient.RuntimeScheme)
	if err != nil {
		if !meta.IsNoMatchError(err) {
			return false, err
		}
		seedIsGarden = false
	}
	return seedIsGarden, nil
}

// SetDefaultGardenClusterAddress sets the default garden cluster address in the given gardenlet configuration if it is not already set.
func SetDefaultGardenClusterAddress(log logr.Logger, gardenletConfigRaw runtime.RawExtension, gardenClusterAddress string) (runtime.RawExtension, error) {
	gardenletConfig, err := encoding.DecodeGardenletConfiguration(&gardenletConfigRaw, false)
	if err != nil {
		return runtime.RawExtension{}, fmt.Errorf("failed to decode gardenlet configuration: %w", err)
	}

	if gardenletConfig == nil {
		return gardenletConfigRaw, nil
	}

	if gardenletConfig.GardenClientConnection == nil {
		gardenletConfig.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{}
	}

	if gardenletConfig.GardenClientConnection.GardenClusterAddress == nil {
		log.Info("Setting default garden cluster address", "gardenClusterAddress", gardenClusterAddress)
		gardenletConfig.GardenClientConnection.GardenClusterAddress = &gardenClusterAddress
	}

	newGardenletConfigRaw, err := encoding.EncodeGardenletConfiguration(gardenletConfig)
	if err != nil {
		return runtime.RawExtension{}, fmt.Errorf("failed to encode gardenlet configuration: %w", err)
	}

	return *newGardenletConfigRaw, nil
}

// ResourcePrefixSelfHostedShoot is the prefix for resources related to Gardenlet created for self-hosted shoots.
const ResourcePrefixSelfHostedShoot = "self-hosted-shoot-"

// IsResponsibleForSelfHostedShoot checks if the current process is responsible for managing self-hosted shoots. This is
// determined by checking if the environment variable "NAMESPACE" is set to the kube-system namespace.
func IsResponsibleForSelfHostedShoot() bool {
	return os.Getenv("NAMESPACE") == metav1.NamespaceSystem
}

// ShootMetaFromBootstrapToken extracts the shoot namespace and name from the description of the given bootstrap token
// secret. This only works if the secret has been created with 'gardenadm token create' which writes a proper
// description.
func ShootMetaFromBootstrapToken(ctx context.Context, reader client.Reader, bootstrapTokenSecretName string) (types.NamespacedName, error) {
	bootstrapTokenSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: bootstrapTokenSecretName, Namespace: metav1.NamespaceSystem}}
	if err := reader.Get(ctx, client.ObjectKeyFromObject(bootstrapTokenSecret), bootstrapTokenSecret); err != nil {
		return types.NamespacedName{}, fmt.Errorf("failed to read bootstrap token secret %s: %w", client.ObjectKeyFromObject(bootstrapTokenSecret), err)
	}

	return extractShootMetaFromBootstrapToken(bootstrapTokenSecret)
}

func extractShootMetaFromBootstrapToken(bootstrapTokenSecret *corev1.Secret) (types.NamespacedName, error) {
	description := string(bootstrapTokenSecret.Data[bootstraptokenapi.BootstrapTokenDescriptionKey])
	if !strings.HasPrefix(description, bootstraptoken.SelfHostedShootBootstrapTokenSecretDescriptionPrefix) {
		return types.NamespacedName{}, fmt.Errorf("bootstrap token description does not start with %q: %s", bootstraptoken.SelfHostedShootBootstrapTokenSecretDescriptionPrefix, description)
	}

	parts := strings.Fields(strings.TrimPrefix(description, bootstraptoken.SelfHostedShootBootstrapTokenSecretDescriptionPrefix))
	if len(parts) == 0 {
		return types.NamespacedName{}, fmt.Errorf("could not extract shoot meta from bootstrap token description: %s", description)
	}

	split := strings.Split(parts[0], "/")
	if len(split) != 2 {
		return types.NamespacedName{}, fmt.Errorf("could not extract shoot namespace and name from bootstrap token description: %s", description)
	}

	return types.NamespacedName{Namespace: split[0], Name: split[1]}, nil
}
