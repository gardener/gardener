// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedidentity

import (
	"crypto/x509"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// UserType is used for distinguishing between clients running on a seed cluster when authenticating against the garden
// cluster.
type UserType string

const (
	// UserTypeGardenlet is the UserType of a gardenlet client.
	UserTypeGardenlet UserType = "gardenlet"
	// UserTypeExtension is the UserType of a extension client.
	UserTypeExtension UserType = "extension"
)

// FromUserInfoInterface returns the seed name, a boolean indicating whether the provided user is a seed client,
// and the client's UserType.
func FromUserInfoInterface(u user.Info) (string, bool, UserType) {
	if u == nil {
		return "", false, ""
	}

	if utils.ValueExists(v1beta1constants.SeedsGroup, u.GetGroups()) {
		return getIdentityForSeedsGroup(u)
	}

	if utils.ValueExists(serviceaccount.AllServiceAccountsGroup, u.GetGroups()) {
		return getIdentityForServiceAccountsGroup(u)
	}

	return "", false, ""
}

// FromAuthenticationV1UserInfo converts an authenticationv1.UserInfo structure to the user.Info interface and calls
// FromUserInfoInterface to return the seed name.
func FromAuthenticationV1UserInfo(userInfo authenticationv1.UserInfo) (string, bool, UserType) {
	return FromUserInfoInterface(&user.DefaultInfo{
		Name:   userInfo.Username,
		UID:    userInfo.UID,
		Groups: userInfo.Groups,
		Extra:  convertAuthenticationV1ExtraValueToUserInfoExtra(userInfo.Extra),
	})
}

// FromCertificateSigningRequest converts a *x509.CertificateRequest structure to the user.Info interface and calls
// FromUserInfoInterface to return the seed name.
func FromCertificateSigningRequest(csr *x509.CertificateRequest) (string, bool, UserType) {
	return FromUserInfoInterface(&user.DefaultInfo{
		Name:   csr.Subject.CommonName,
		Groups: csr.Subject.Organization,
	})
}

func convertAuthenticationV1ExtraValueToUserInfoExtra(extra map[string]authenticationv1.ExtraValue) map[string][]string {
	if extra == nil {
		return nil
	}
	ret := make(map[string][]string, len(extra))
	for k, v := range extra {
		ret[k] = v
	}

	return ret
}

func getIdentityForSeedsGroup(u user.Info) (string, bool, UserType) {
	userName := u.GetName()

	if !strings.HasPrefix(userName, v1beta1constants.SeedUserNamePrefix) {
		return "", false, ""
	}

	seedName := strings.TrimPrefix(userName, v1beta1constants.SeedUserNamePrefix)
	if seedName == "" {
		return "", false, ""
	}

	return seedName, true, UserTypeGardenlet
}

func getIdentityForServiceAccountsGroup(u user.Info) (string, bool, UserType) {
	var serviceAccountNamespaceGroup string
	for _, g := range u.GetGroups() {
		if strings.HasPrefix(g, serviceaccount.ServiceAccountGroupPrefix) {
			serviceAccountNamespaceGroup = g
			break
		}
	}

	seedNamespace := strings.TrimPrefix(serviceAccountNamespaceGroup, serviceaccount.ServiceAccountGroupPrefix)
	if !strings.HasPrefix(seedNamespace, gardenerutils.SeedNamespaceNamePrefix) {
		return "", false, ""
	}

	seedName := strings.TrimPrefix(seedNamespace, gardenerutils.SeedNamespaceNamePrefix)
	name := strings.TrimPrefix(u.GetName(), serviceaccount.ServiceAccountUsernamePrefix+seedNamespace+serviceaccount.ServiceAccountUsernameSeparator)

	if seedName != "" && strings.HasPrefix(name, v1beta1constants.ExtensionGardenServiceAccountPrefix) {
		return seedName, true, UserTypeExtension
	}

	return "", false, ""
}
