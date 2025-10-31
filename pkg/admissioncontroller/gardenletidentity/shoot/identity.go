// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"crypto/x509"
	"slices"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// FromUserInfoInterface returns the shoot namespace and name, a boolean indicating whether the provided user is an
// self-hosted shoot client, and the client's UserType.
func FromUserInfoInterface(u user.Info) (namespace string, name string, isSelfHostedShoot bool, userType gardenletidentity.UserType) {
	if u == nil {
		return "", "", false, ""
	}

	if slices.Contains(u.GetGroups(), v1beta1constants.ShootsGroup) {
		return getIdentityForShootsGroup(u)
	}

	if slices.Contains(u.GetGroups(), serviceaccount.AllServiceAccountsGroup) {
		return getIdentityForServiceAccountsGroup(u)
	}

	return "", "", false, ""
}

// FromAuthenticationV1UserInfo converts an authenticationv1.UserInfo structure to the user.Info interface and calls
// FromUserInfoInterface to return the shoot namespace and name.
func FromAuthenticationV1UserInfo(userInfo authenticationv1.UserInfo) (namespace string, name string, isSelfHostedShoot bool, userType gardenletidentity.UserType) {
	return FromUserInfoInterface(&user.DefaultInfo{
		Name:   userInfo.Username,
		UID:    userInfo.UID,
		Groups: userInfo.Groups,
		Extra:  convertAuthenticationV1ExtraValueToUserInfoExtra(userInfo.Extra),
	})
}

// FromCertificateSigningRequest converts a *x509.CertificateRequest structure to the user.Info interface and calls
// FromUserInfoInterface to return the shoot namespace and name.
func FromCertificateSigningRequest(csr *x509.CertificateRequest) (namespace string, name string, isSelfHostedShoot bool, userType gardenletidentity.UserType) {
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

func getIdentityForShootsGroup(u user.Info) (namespace string, name string, isSelfHostedShoot bool, userType gardenletidentity.UserType) {
	userName := u.GetName()

	var prefix string
	switch {
	case strings.HasPrefix(userName, v1beta1constants.ShootUserNamePrefix):
		prefix = v1beta1constants.ShootUserNamePrefix
	case strings.HasPrefix(userName, v1beta1constants.GardenadmUserNamePrefix):
		prefix = v1beta1constants.GardenadmUserNamePrefix
	default:
		return "", "", false, ""
	}

	var (
		namespaceName = strings.TrimPrefix(userName, prefix)
		split         = strings.Split(namespaceName, ":")
	)

	if len(split) != 2 {
		return "", "", false, ""
	}

	return split[0], split[1], true, userTypeFromPrefix(prefix)
}

func userTypeFromPrefix(prefix string) gardenletidentity.UserType {
	if prefix == v1beta1constants.ShootUserNamePrefix {
		return gardenletidentity.UserTypeGardenlet
	}
	if prefix == v1beta1constants.GardenadmUserNamePrefix {
		return gardenletidentity.UserTypeGardenadm
	}
	return ""
}

func getIdentityForServiceAccountsGroup(_ user.Info) (namespace string, name string, isSelfHostedShoot bool, userType gardenletidentity.UserType) {
	// TODO(rfranzke): Implement this function once the concept of how extensions running in self-hosted shoots
	//  authenticate with the garden cluster gets clear.
	return "", "", false, gardenletidentity.UserTypeExtension
}
