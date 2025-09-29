// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"crypto/x509"
	"slices"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// FromUserInfoInterface returns the seed name, a boolean indicating whether the provided user is a seed client,
// and the client's UserType.
func FromUserInfoInterface(u user.Info) (string, bool, gardenletidentity.UserType) {
	if u == nil {
		return "", false, ""
	}

	if slices.Contains(u.GetGroups(), v1beta1constants.SeedsGroup) {
		return getIdentityForSeedsGroup(u)
	}

	if slices.Contains(u.GetGroups(), serviceaccount.AllServiceAccountsGroup) {
		return getIdentityForServiceAccountsGroup(u)
	}

	return "", false, ""
}

// FromAuthenticationV1UserInfo converts an authenticationv1.UserInfo structure to the user.Info interface and calls
// FromUserInfoInterface to return the seed name.
func FromAuthenticationV1UserInfo(userInfo authenticationv1.UserInfo) (string, bool, gardenletidentity.UserType) {
	return FromUserInfoInterface(&user.DefaultInfo{
		Name:   userInfo.Username,
		UID:    userInfo.UID,
		Groups: userInfo.Groups,
		Extra:  convertAuthenticationV1ExtraValueToUserInfoExtra(userInfo.Extra),
	})
}

// FromCertificateSigningRequest converts a *x509.CertificateRequest structure to the user.Info interface and calls
// FromUserInfoInterface to return the seed name.
func FromCertificateSigningRequest(csr *x509.CertificateRequest) (string, bool, gardenletidentity.UserType) {
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

func getIdentityForSeedsGroup(u user.Info) (string, bool, gardenletidentity.UserType) {
	userName := u.GetName()

	if !strings.HasPrefix(userName, v1beta1constants.SeedUserNamePrefix) {
		return "", false, ""
	}

	seedName := strings.TrimPrefix(userName, v1beta1constants.SeedUserNamePrefix)
	if seedName == "" {
		return "", false, ""
	}

	return seedName, true, gardenletidentity.UserTypeGardenlet
}

func getIdentityForServiceAccountsGroup(u user.Info) (string, bool, gardenletidentity.UserType) {
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
		return seedName, true, gardenletidentity.UserTypeExtension
	}

	return "", false, ""
}
