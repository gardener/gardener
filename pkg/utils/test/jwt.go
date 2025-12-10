// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"bytes"

	"github.com/go-jose/go-jose/v4"
)

// SampleServiceAccountToken generates a sample Kubernetes ServiceAccount JWT token string with the given UID.
func SampleServiceAccountToken(uid string) string {
	key := bytes.Repeat([]byte("A"), 32)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: key}, nil)
	if err != nil {
		panic(err)
	}

	payload := []byte(`{
    "kubernetes.io": {
        "namespace": "n",
        "serviceaccount": {
            "name": "test",
            "uid": "` + uid + `"
        }
    }
}`)

	jws, err := signer.Sign(payload)
	if err != nil {
		panic(err)
	}
	token, err := jws.CompactSerialize()
	if err != nil {
		panic(err)
	}
	return token
}
