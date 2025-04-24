// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

// CreateClientSet creates a new client set using the AutonomousBotanist to create the client set.
// Exposed for unit testing.
var CreateClientSet = func(ctx context.Context, log logr.Logger) (kubernetes.Interface, error) {
	return (&botanist.AutonomousBotanist{Botanist: &botanistpkg.Botanist{Operation: &operation.Operation{Logger: log}}}).CreateClientSet(ctx)
}

// CreateBootstrapToken creates a bootstrap token with the given ID and secret. If the secret is empty, a random secret
// will be generated. If the token already exists, an error will be returned. The default is printing the token to the
// output stream. Alternatively, you can set the printJoinCommand flag to true to print the `gardenadm join` command
// instead.
func CreateBootstrapToken(ctx context.Context, clientSet kubernetes.Interface, opts *Options, tokenID, tokenSecret string) error {
	if err := clientSet.Client().Get(ctx, client.ObjectKey{Name: bootstraptokenutil.BootstrapTokenSecretName(tokenID), Namespace: metav1.NamespaceSystem}, &corev1.Secret{}); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed checking if bootstrap token with ID %q already exists: %w", tokenID, err)
	} else if err == nil {
		return fmt.Errorf("bootstrap token with ID %q already exists", tokenID)
	}

	secret, err := bootstraptoken.ComputeBootstrapTokenWithSecret(ctx, clientSet.Client(), tokenID, tokenSecret, opts.Description, opts.Validity)
	if err != nil {
		return fmt.Errorf("failed creating a bootstrap token secret: %w", err)
	}

	if !opts.PrintJoinCommand {
		fmt.Fprintln(opts.Out, bootstraptoken.FromSecretData(secret.Data))
	} else {
		output, err := printJoinCommand(ctx, opts, clientSet, secret)
		if err != nil {
			return fmt.Errorf("failed computing join command: %w", err)
		}
		fmt.Fprint(opts.Out, output)
	}

	return nil
}

func printJoinCommand(ctx context.Context, opts *Options, clientSet kubernetes.Interface, bootstrapTokenSecret *corev1.Secret) (string, error) {
	secretList := &corev1.SecretList{}
	if err := clientSet.Client().List(ctx, secretList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{
		v1beta1constants.GardenRole:      v1beta1constants.GardenRoleOperatingSystemConfig,
		v1beta1constants.LabelWorkerPool: opts.WorkerPoolName,
	}); err != nil {
		return "", fmt.Errorf("failed listing gardener-node-agent secrets: %w", err)
	}

	if len(secretList.Items) == 0 {
		return "", fmt.Errorf("no gardener-node-agent secrets found for worker pool %q", opts.WorkerPoolName)
	}

	gardenerNodeAgentSecret := secretList.Items[0]
	if len(secretList.Items) > 1 {
		opts.Log.V(1).Info("Multiple gardener-node-agent secrets found, using the first one", "secretName", gardenerNodeAgentSecret.Name)
	}

	return fmt.Sprintf(`gardenadm join --bootstrap-token %s --ca-certificate "%s" --gardener-node-agent-secret-name %s %s`,
		bootstraptoken.FromSecretData(bootstrapTokenSecret.Data),
		utils.EncodeBase64(clientSet.RESTConfig().CAData),
		gardenerNodeAgentSecret.Name,
		clientSet.RESTConfig().Host,
	), nil
}
