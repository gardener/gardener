// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstraptoken

import (
	"context"
	"fmt"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// CharSet defines which characters are allowed to be used for bootstrap token ids and secrets.
	CharSet = "0123456789abcdefghijklmnopqrstuvwxyz"
	// IDLength is the length of the token id.
	IDLength = 6
	// SecretLength is the length of the token secret.
	SecretLength = 16
)

var (
	// ValidBootstrapTokenRegex is used to check if an existing token can be interpreted as a bootstrap token.
	ValidBootstrapTokenRegex = regexp.MustCompile(fmt.Sprintf(`^[a-z0-9]{%d}.[a-z0-9]{%d}$`, IDLength, SecretLength))
	// validBootstrapTokenSecretRegex is used to check if an existing token can be interpreted as a bootstrap token.
	validBootstrapTokenSecretRegex = regexp.MustCompile(fmt.Sprintf(`[a-z0-9]{%d}`, SecretLength))

	// Now computes the current time.
	// Exposed for unit testing.
	Now = metav1.Now
)

// ComputeBootstrapTokenWithSecret computes and creates a new bootstrap token with a specified token secret, and returns it.
func ComputeBootstrapTokenWithSecret(ctx context.Context, c client.Client, tokenID, tokenSecret, description string, validity time.Duration) (secret *corev1.Secret, err error) {
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstraptokenutil.BootstrapTokenSecretName(tokenID),
			Namespace: metav1.NamespaceSystem,
		},
	}

	if err = c.Get(ctx, client.ObjectKeyFromObject(secret), secret); client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	if existingSecretToken, ok := secret.Data[bootstraptokenapi.BootstrapTokenSecretKey]; ok && validBootstrapTokenSecretRegex.Match(existingSecretToken) {
		tokenSecret = string(existingSecretToken)
	} else if tokenSecret == "" {
		tokenSecret, err = utils.GenerateRandomStringFromCharset(SecretLength, CharSet)
		if err != nil {
			return nil, err
		}
	}

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, c, secret, func() error {
		secret.Type = bootstraptokenapi.SecretTypeBootstrapToken
		secret.Data = map[string][]byte{
			bootstraptokenapi.BootstrapTokenDescriptionKey:      []byte(description),
			bootstraptokenapi.BootstrapTokenIDKey:               []byte(tokenID),
			bootstraptokenapi.BootstrapTokenSecretKey:           []byte(tokenSecret),
			bootstraptokenapi.BootstrapTokenExpirationKey:       []byte(Now().Add(validity).Format(time.RFC3339)),
			bootstraptokenapi.BootstrapTokenUsageAuthentication: []byte("true"),
			bootstraptokenapi.BootstrapTokenUsageSigningKey:     []byte("true"),
		}
		return nil
	})

	return secret, err
}

// ComputeBootstrapToken computes and creates a new bootstrap token with a randomly-generated secret, and returns it.
func ComputeBootstrapToken(ctx context.Context, c client.Client, tokenID, description string, validity time.Duration) (secret *corev1.Secret, err error) {
	return ComputeBootstrapTokenWithSecret(ctx, c, tokenID, "", description, validity)
}

// FromSecretData returns the bootstrap token based on the secret data.
func FromSecretData(data map[string][]byte) string {
	return bootstraptokenutil.TokenFromIDAndSecret(string(data[bootstraptokenapi.BootstrapTokenIDKey]), string(data[bootstraptokenapi.BootstrapTokenSecretKey]))
}

// TokenID returns the token id based on the given metadata.
func TokenID(meta metav1.ObjectMeta) string {
	value := meta.Name
	if meta.Namespace != "" {
		value = meta.Namespace + "--" + meta.Name
	}

	return utils.ComputeSHA256Hex([]byte(value))[:IDLength]
}
