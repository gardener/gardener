// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"strings"
	"sync"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

// keychain implements Keychain with the semantics of the standard Docker config file.
type keychain struct {
	mu         sync.Mutex
	pullSecret string
}

var _ authn.Keychain = &keychain{}
var _ authn.ContextKeychain = &keychain{}

// ResolveContext implements ContextKeychain.
func (k *keychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	return k.ResolveContext(context.Background(), target)
}

// Resolve implements Keychain.
func (k *keychain) ResolveContext(_ context.Context, target authn.Resource) (authn.Authenticator, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	cf, err := config.LoadFromReader(strings.NewReader(k.pullSecret))
	if err != nil {
		return nil, err
	}

	// same logic as in defaultKeychain.ResolveContext (github.com/google/go-containerregistry/pkg/authn)
	// See:
	// https://github.com/google/ko/issues/90
	// https://github.com/moby/moby/blob/fc01c2b481097a6057bec3cd1ab2d7b4488c50c4/registry/config.go#L397-L404
	var cfg, empty types.AuthConfig
	for _, key := range []string{
		target.String(),
		target.RegistryStr(),
	} {
		if key == name.DefaultRegistry {
			key = authn.DefaultAuthKey
		}

		cfg, err = cf.GetAuthConfig(key)
		if err != nil {
			return nil, err
		}
		// cf.GetAuthConfig automatically sets the ServerAddress attribute. Since
		// we don't make use of it, clear the value for a proper "is-empty" test.
		// See: https://github.com/google/go-containerregistry/issues/1510
		cfg.ServerAddress = ""
		if cfg != empty {
			break
		}
	}
	if cfg == empty {
		return authn.Anonymous, nil
	}

	return authn.FromConfig(authn.AuthConfig{
		Username:      cfg.Username,
		Password:      cfg.Password,
		Auth:          cfg.Auth,
		IdentityToken: cfg.IdentityToken,
		RegistryToken: cfg.RegistryToken,
	}), nil
}
