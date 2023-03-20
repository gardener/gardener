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
	"k8s.io/apiserver/pkg/authentication/user"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

// FromUserInfoInterface returns the seed name and a boolean indicating whether the provided user has the
// gardener.cloud:system:seeds group.
func FromUserInfoInterface(u user.Info) (string, bool) {
	if u == nil {
		return "", false
	}

	userName := u.GetName()
	if !strings.HasPrefix(userName, v1beta1constants.SeedUserNamePrefix) {
		return "", false
	}

	if !utils.ValueExists(v1beta1constants.SeedsGroup, u.GetGroups()) {
		return "", false
	}

	seedName := strings.TrimPrefix(userName, v1beta1constants.SeedUserNamePrefix)
	if seedName == "" {
		return "", false
	}

	return seedName, true
}

// FromAuthenticationV1UserInfo converts an authenticationv1.UserInfo structure to the user.Info interface and calls
// FromUserInfoInterface to return the seed name.
func FromAuthenticationV1UserInfo(userInfo authenticationv1.UserInfo) (string, bool) {
	return FromUserInfoInterface(&user.DefaultInfo{
		Name:   userInfo.Username,
		UID:    userInfo.UID,
		Groups: userInfo.Groups,
		Extra:  convertAuthenticationV1ExtraValueToUserInfoExtra(userInfo.Extra),
	})
}

// FromCertificateSigningRequest converts a *x509.CertificateRequest structure to the user.Info interface and calls
// FromUserInfoInterface to return the seed name.
func FromCertificateSigningRequest(csr *x509.CertificateRequest) (string, bool) {
	return FromUserInfoInterface(&user.DefaultInfo{
		Name:   csr.Subject.CommonName,
		Groups: csr.Subject.Organization,
	})
}

func convertAuthenticationV1ExtraValueToUserInfoExtra(extra map[string]authenticationv1.ExtraValue) map[string][]string {
	if extra == nil {
		return nil
	}
	ret := map[string][]string{}
	for k, v := range extra {
		ret[k] = v
	}

	return ret
}
