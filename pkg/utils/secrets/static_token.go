// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"fmt"
	"strings"
)

const (
	// DataKeyStaticTokenCSV is the key in a secret data holding the CSV format of a secret.
	DataKeyStaticTokenCSV = "static_tokens.csv" // #nosec G101 -- No credential.
	// DataKeyToken is the key in a secret data holding the token.
	DataKeyToken = "token"
)

// StaticTokenSecretConfig contains the specification a to-be-generated static token secret.
type StaticTokenSecretConfig struct {
	Name string

	Tokens map[string]TokenConfig
}

// TokenConfig contains configuration for a token.
type TokenConfig struct {
	Username string
	UserID   string
	Groups   []string
}

// StaticToken contains the username, the password, optionally hash of the password and the format for serializing the static token
type StaticToken struct {
	Name string

	Tokens []Token
}

// Token contains fields of a generated token.
type Token struct {
	Username string
	UserID   string
	Groups   []string
	Token    string
}

// GetName returns the name of the secret.
func (s *StaticTokenSecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *StaticTokenSecretConfig) Generate() (DataInterface, error) {
	tokens := make([]Token, 0, len(s.Tokens))

	for _, tokenConfig := range s.Tokens {
		token, err := GenerateRandomString(128)
		if err != nil {
			return nil, err
		}

		tokens = append(tokens, Token{
			Username: tokenConfig.Username,
			UserID:   tokenConfig.UserID,
			Groups:   tokenConfig.Groups,
			Token:    token,
		})
	}

	return &StaticToken{
		Name:   s.Name,
		Tokens: tokens,
	}, nil
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (b *StaticToken) SecretData() map[string][]byte {
	var (
		data   = make(map[string][]byte, 1)
		tokens = make([]string, 0, len(b.Tokens))
	)

	for _, token := range b.Tokens {
		groups := strings.Join(token.Groups, ",")
		if len(token.Groups) > 1 {
			groups = fmt.Sprintf("%q", groups)
		}
		tokens = append(tokens, fmt.Sprintf("%s,%s,%s,%s", token.Token, token.Username, token.UserID, groups))
	}

	data[DataKeyStaticTokenCSV] = []byte(strings.Join(tokens, "\n"))
	return data
}

// GetTokenForUsername returns the token for the given username.
func (b *StaticToken) GetTokenForUsername(username string) (*Token, error) {
	for _, token := range b.Tokens {
		if token.Username == username {
			return &token, nil
		}
	}
	return nil, fmt.Errorf("could not find token for username %q", username)
}

// LoadStaticTokenFromCSV loads the static token data from the given CSV-formatted <data>.
func LoadStaticTokenFromCSV(name string, data []byte) (*StaticToken, error) {
	var (
		lines  = strings.Split(string(data), "\n")
		tokens = make([]Token, 0, len(lines))
	)

	for _, token := range lines {
		csv := strings.Split(token, ",")
		if len(csv) < 4 {
			return nil, fmt.Errorf("invalid CSV for loading static token data: %s", string(data))
		}

		tokens = append(tokens, Token{
			Username: csv[1],
			UserID:   csv[2],
			Groups:   strings.Split(csv[3], ","),
			Token:    csv[0],
		})
	}

	return &StaticToken{
		Name:   name,
		Tokens: tokens,
	}, nil
}
