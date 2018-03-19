// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"sort"
	"strconv"
)

// EncodeBase64 takes a byte slice and returns the Base64-encoded string.
func EncodeBase64(in []byte) string {
	encodedLength := base64.StdEncoding.EncodedLen(len(in))
	buffer := make([]byte, encodedLength)
	out := buffer[0:encodedLength]
	base64.StdEncoding.Encode(out, in)
	return string(out)
}

// DecodeBase64 takes a Base64-encoded string and returns the decoded byte slice.
func DecodeBase64(in string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(in)
}

// EncodePrivateKey takes a RSA private key object, encodes it to the PEM format, and returns it as
// a byte slice.
func EncodePrivateKey(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// DecodePrivateKey takes a byte slice, decodes it from the PEM format, converts it to an rsa.PrivateKey
// object, and returns it. In case an error occurs, it returns the error.
func DecodePrivateKey(bytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(bytes)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, errors.New("could not decode the PEM-encoded RSA private key")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// EncodeCertificate takes a certificate as a byte slice, encodes it to the PEM format, and returns
// it as byte slice.
func EncodeCertificate(certificate []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificate,
	})
}

// DecodeCertificate takes a byte slice, decodes it from the PEM format, converts it to an x509.Certificate
// object, and returns it. In case an error occurs, it returns the error.
func DecodeCertificate(bytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(bytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("could not decode the PEM-encoded certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

// SHA1 takes a byte slice and returns the sha1-hashed byte slice.
func SHA1(in []byte) []byte {
	s := sha1.New()
	s.Write(in)
	return s.Sum(nil)
}

// SHA256 takes a byte slice and returns the sha256-hashed byte slice.
func SHA256(in []byte) []byte {
	h := sha256.Sum256(in)
	return h[:]
}

// EncodeSHA1 takes a byte slice and returns the sha1-hashed string (base64-encoded).
func EncodeSHA1(in []byte) string {
	return EncodeBase64(SHA1(in))
}

// CreateSHA1Secret takes a username and a password and returns a sha1-schemed credentials pair as string.
func CreateSHA1Secret(username, password []byte) string {
	credentials := append([]byte(username), ":{SHA}"...)
	credentials = append(credentials, EncodeSHA1(password)...)
	return EncodeBase64(credentials)
}

// ComputeSHA1Hex computes the hexadecimal representation of the SHA1 hash of the given input byte
// slice <in>, converts it to a string and returns it (length of returned string is 40 characters).
func ComputeSHA1Hex(in []byte) string {
	return hex.EncodeToString(SHA1(in))
}

// ComputeSHA256Hex computes the hexadecimal representation of the SHA256 hash of the given input byte
// slice <in>, converts it to a string and returns it.
func ComputeSHA256Hex(in []byte) string {
	return hex.EncodeToString(SHA256(in))
}

// HashForMap creates a hash value for a map of type map[string]interface{} and returns it.
func HashForMap(m map[string]interface{}) string {
	var (
		hash string
		keys []string
	)

	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			hash += ComputeSHA256Hex([]byte(v))
		case int:
			hash += ComputeSHA256Hex([]byte(strconv.Itoa(v)))
		case bool:
			hash += ComputeSHA256Hex([]byte(strconv.FormatBool(v)))
		case []string:
			for _, val := range v {
				hash += ComputeSHA256Hex([]byte(val))
			}
		case map[string]interface{}:
			hash += HashForMap(v)
		case []map[string]interface{}:
			for _, val := range v {
				hash += HashForMap(val)
			}
		}
	}

	return ComputeSHA256Hex([]byte(hash))
}
