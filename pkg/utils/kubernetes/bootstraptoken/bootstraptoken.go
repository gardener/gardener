// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstraptoken

import (
	"context"
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

// validBootstrapTokenRegex is used to check if an existing token can be interpreted as a bootstrap token.
var validBootstrapTokenRegex = regexp.MustCompile(`[a-z0-9]{16}`)

// ComputeBootstrapToken computes and creates a new bootstrap token, and returns it.
func ComputeBootstrapToken(ctx context.Context, c client.Client, tokenID, description string, validity time.Duration) (secret *corev1.Secret, err error) {
	var (
		bootstrapTokenSecretKey string
	)

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstraptokenutil.BootstrapTokenSecretName(tokenID),
			Namespace: metav1.NamespaceSystem,
		},
	}

	if err = c.Get(ctx, client.ObjectKeyFromObject(secret), secret); client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	if existingSecretToken, ok := secret.Data[bootstraptokenapi.BootstrapTokenSecretKey]; ok && validBootstrapTokenRegex.Match(existingSecretToken) {
		bootstrapTokenSecretKey = string(existingSecretToken)
	} else {
		bootstrapTokenSecretKey, err = utils.GenerateRandomStringFromCharset(16, "0123456789abcdefghijklmnopqrstuvwxyz")
		if err != nil {
			return nil, err
		}
	}

	data := map[string][]byte{
		bootstraptokenapi.BootstrapTokenDescriptionKey:      []byte(description),
		bootstraptokenapi.BootstrapTokenIDKey:               []byte(tokenID),
		bootstraptokenapi.BootstrapTokenSecretKey:           []byte(bootstrapTokenSecretKey),
		bootstraptokenapi.BootstrapTokenExpirationKey:       []byte(metav1.Now().Add(validity).Format(time.RFC3339)),
		bootstraptokenapi.BootstrapTokenUsageAuthentication: []byte("true"),
		bootstraptokenapi.BootstrapTokenUsageSigningKey:     []byte("true"),
	}

	_, err2 := controllerutils.GetAndCreateOrMergePatch(ctx, c, secret, func() error {
		secret.Type = bootstraptokenapi.SecretTypeBootstrapToken
		secret.Data = data
		return nil
	})

	return secret, err2
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

	return utils.ComputeSHA256Hex([]byte(value))[:6]
}
