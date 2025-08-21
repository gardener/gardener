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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// UserType is used for distinguishing between clients running on am autonomous shoot cluster when authenticating
// against the garden cluster.
type UserType string

const (
	// UserTypeGardenlet is the UserType of a gardenlet client.
	UserTypeGardenlet UserType = "gardenlet"
	// UserTypeExtension is the UserType of a extension client.
	UserTypeExtension UserType = "extension"
)

// FromUserInfoInterface returns the seed name, a boolean indicating whether the provided user is an autonomous shoot
// client, and the client's UserType.
func FromUserInfoInterface(u user.Info) (namespace string, name string, isAutonomousShoot bool, userType UserType) {
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
// FromUserInfoInterface to return the seed name.
func FromAuthenticationV1UserInfo(userInfo authenticationv1.UserInfo) (namespace string, name string, isAutonomousShoot bool, userType UserType) {
	return FromUserInfoInterface(&user.DefaultInfo{
		Name:   userInfo.Username,
		UID:    userInfo.UID,
		Groups: userInfo.Groups,
		Extra:  convertAuthenticationV1ExtraValueToUserInfoExtra(userInfo.Extra),
	})
}

// FromCertificateSigningRequest converts a *x509.CertificateRequest structure to the user.Info interface and calls
// FromUserInfoInterface to return the seed name.
func FromCertificateSigningRequest(csr *x509.CertificateRequest) (namespace string, name string, isAutonomousShoot bool, userType UserType) {
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

func getIdentityForShootsGroup(u user.Info) (namespace string, name string, isAutonomousShoot bool, userType UserType) {
	userName := u.GetName()

	if !strings.HasPrefix(userName, v1beta1constants.ShootUserNamePrefix) {
		return "", "", false, ""
	}

	var (
		namespaceName = strings.TrimPrefix(userName, v1beta1constants.ShootUserNamePrefix)
		split         = strings.Split(namespaceName, ":")
	)

	if len(split) != 2 {
		return "", "", false, ""
	}

	return split[0], split[1], true, UserTypeGardenlet
}

func getIdentityForServiceAccountsGroup(_ user.Info) (namespace string, name string, isAutonomousShoot bool, userType UserType) {
	// TODO(rfranzke): Implement this function once the concept of how extensions running in autonomous shoots
	//  authenticate with the garden cluster gets clear.
	return "", "", false, UserTypeExtension
}
