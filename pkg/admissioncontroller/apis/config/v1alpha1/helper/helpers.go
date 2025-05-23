// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
)

// APIGroupMatches returns `true` if the given group has a match in the given limit.
func APIGroupMatches(limit admissioncontrollerconfigv1alpha1.ResourceLimit, group string) bool {
	for _, grp := range limit.APIGroups {
		if grp == admissioncontrollerconfigv1alpha1.WildcardAll || grp == group {
			return true
		}
	}

	return false
}

// ResourceMatches returns `true` if the given resource has a match in the given limit.
func ResourceMatches(limit admissioncontrollerconfigv1alpha1.ResourceLimit, resource string) bool {
	for _, res := range limit.Resources {
		if res == admissioncontrollerconfigv1alpha1.WildcardAll || res == resource {
			return true
		}
	}
	return false
}

// VersionMatches returns `true` if the given version has a match in the given limit.
func VersionMatches(limit admissioncontrollerconfigv1alpha1.ResourceLimit, version string) bool {
	for _, ver := range limit.APIVersions {
		if ver == admissioncontrollerconfigv1alpha1.WildcardAll || ver == version {
			return true
		}
	}
	return false
}

// UserMatches returns `true` if the given user in the subject has a match in the given userConfig.
func UserMatches(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	if subject.Kind != rbacv1.UserKind {
		return false
	}

	return subject.Name == admissioncontrollerconfigv1alpha1.WildcardAll || subject.Name == userInfo.Username
}

// UserGroupMatches returns `true` if the given group in the subject has a match in the given userConfig.
// Always returns true if `admissioncontrollerconfigv1alpha1.WildcardAll` is used in subject.
func UserGroupMatches(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	if subject.Kind != rbacv1.GroupKind {
		return false
	}

	if subject.Name == admissioncontrollerconfigv1alpha1.WildcardAll {
		return true
	}

	for _, group := range userInfo.Groups {
		if group == subject.Name {
			return true
		}
	}
	return false
}

// ServiceAccountMatches returns `true` if the given service account in the subject has a match in the given userConfig.
// Supports `admissioncontrollerconfigv1alpha1.WildcardAll` in subject name.
func ServiceAccountMatches(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	if subject.Kind != rbacv1.ServiceAccountKind {
		return false
	}

	if subject.Name == admissioncontrollerconfigv1alpha1.WildcardAll {
		saPrefix := fmt.Sprintf("%s%s:", serviceaccount.ServiceAccountUsernamePrefix, subject.Namespace)
		return strings.HasPrefix(userInfo.Username, saPrefix)
	}

	return serviceaccount.MatchesUsername(subject.Namespace, subject.Name, userInfo.Username)
}
