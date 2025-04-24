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

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
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

	fmt.Fprintln(opts.IOStreams.Out, bootstraptoken.FromSecretData(secret.Data))
	return nil
}
