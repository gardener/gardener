// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package access

import (
	"context"
	"fmt"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/retry"
)

const namespaceE2ETestServiceAccountTokenAccess = metav1.NamespaceDefault

// labelsE2ETestDynamicServiceAccountTokenAccess is the set of labels added to all ServiceAccounts and
// ClusterRoleBindings for easy cleanup.
var labelsE2ETestDynamicServiceAccountTokenAccess = map[string]string{"e2e-test": "serviceaccount-dynamic-access"}

// CreateShootClientFromDynamicServiceAccountToken creates a ServiceAccount, uses the kube-apiserver's TokenRequest API
// to request a token for it, and then creates a new shoot client from it.
// You should call CleanupObjectsFromDynamicServiceAccountTokenAccess to clean up the objects created by this function.
func CreateShootClientFromDynamicServiceAccountToken(ctx context.Context, shootClient kubernetes.Interface, name string) (kubernetes.Interface, error) {
	return createShootClientFromServiceAccount(ctx, shootClient, name, labelsE2ETestDynamicServiceAccountTokenAccess, func(serviceAccount *corev1.ServiceAccount) (string, error) {
		tokenRequest := &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				Audiences:         []string{v1beta1constants.GardenerAudience},
				ExpirationSeconds: pointer.Int64(3600),
			},
		}

		if err := shootClient.Client().SubResource("token").Create(ctx, serviceAccount, tokenRequest); err != nil {
			return "", err
		}

		return tokenRequest.Status.Token, nil
	})
}

// CleanupObjectsFromDynamicServiceAccountTokenAccess cleans up all objects in the shoot created by all calls to
// CreateShootClientFromDynamicServiceAccountToken.
func CleanupObjectsFromDynamicServiceAccountTokenAccess(ctx context.Context, shootClient kubernetes.Interface) error {
	return flow.Parallel(func(ctx context.Context) error {
		return shootClient.Client().DeleteAllOf(ctx, &corev1.ServiceAccount{}, client.InNamespace(namespaceE2ETestServiceAccountTokenAccess), client.MatchingLabels(labelsE2ETestDynamicServiceAccountTokenAccess))
	}, func(ctx context.Context) error {
		return shootClient.Client().DeleteAllOf(ctx, &rbacv1.ClusterRoleBinding{}, client.MatchingLabels(labelsE2ETestDynamicServiceAccountTokenAccess))
	})(ctx)
}

// labelsE2ETestStaticServiceAccountToken is the set of labels added to all ServiceAccounts, Secrets, and
// ClusterRoleBindings for easy cleanup.
var labelsE2ETestStaticServiceAccountToken = map[string]string{"e2e-test": "serviceaccount-static-access"}

// CreateShootClientFromStaticServiceAccountToken creates a ServiceAccount, a corresponding static token secret (issued
// by kube-controller-manager), and then creates a new shoot client from it.
// You should call CleanupObjectsFromStaticServiceAccountTokenAccess to clean up the objects created by this function.
func CreateShootClientFromStaticServiceAccountToken(ctx context.Context, shootClient kubernetes.Interface, name string) (kubernetes.Interface, error) {
	return createShootClientFromServiceAccount(ctx, shootClient, name, labelsE2ETestStaticServiceAccountToken, func(serviceAccount *corev1.ServiceAccount) (string, error) {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: serviceAccount.Namespace}}
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, shootClient.Client(), secret, func() error {
			secret.Labels = utils.MergeStringMaps(secret.Labels, labelsE2ETestStaticServiceAccountToken)
			secret.Annotations = utils.MergeStringMaps(secret.Annotations, map[string]string{corev1.ServiceAccountNameKey: serviceAccount.Name})
			secret.Type = corev1.SecretTypeServiceAccountToken
			return nil
		}); err != nil {
			return "", err
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := retry.Until(timeoutCtx, 500*time.Millisecond, func(ctx context.Context) (bool, error) {
			if err := shootClient.Client().Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
				return retry.SevereError(err)
			}

			if len(secret.Data[corev1.ServiceAccountTokenKey]) == 0 {
				return retry.MinorError(fmt.Errorf("token for secret %s not yet populated by kube-controller-manager", client.ObjectKeyFromObject(secret)))
			}

			return retry.Ok()
		}); err != nil {
			return "", nil
		}

		return string(secret.Data[corev1.ServiceAccountTokenKey]), nil
	})
}

// CleanupObjectsFromStaticServiceAccountTokenAccess cleans up all objects in the shoot created by all calls to
// CreateShootClientFromStaticServiceAccountToken.
func CleanupObjectsFromStaticServiceAccountTokenAccess(ctx context.Context, shootClient kubernetes.Interface) error {
	return flow.Parallel(func(ctx context.Context) error {
		return shootClient.Client().DeleteAllOf(ctx, &corev1.ServiceAccount{}, client.InNamespace(namespaceE2ETestServiceAccountTokenAccess), client.MatchingLabels(labelsE2ETestStaticServiceAccountToken))
	}, func(ctx context.Context) error {
		return shootClient.Client().DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(namespaceE2ETestServiceAccountTokenAccess), client.MatchingLabels(labelsE2ETestStaticServiceAccountToken))
	}, func(ctx context.Context) error {
		return shootClient.Client().DeleteAllOf(ctx, &rbacv1.ClusterRoleBinding{}, client.MatchingLabels(labelsE2ETestStaticServiceAccountToken))
	})(ctx)
}

func createShootClientFromServiceAccount(
	ctx context.Context,
	shootClient kubernetes.Interface,
	name string,
	labels map[string]string,
	getTokenForServiceAccount func(*corev1.ServiceAccount) (string, error),
) (
	kubernetes.Interface,
	error,
) {
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespaceE2ETestServiceAccountTokenAccess}}
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, shootClient.Client(), serviceAccount, func() error {
		serviceAccount.Labels = utils.MergeStringMaps(serviceAccount.Labels, labels)
		return nil
	}); err != nil {
		return nil, err
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, shootClient.Client(), clusterRoleBinding, func() error {
		clusterRoleBinding.Labels = utils.MergeStringMaps(serviceAccount.Labels, labels)
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccount.Name,
			Namespace: serviceAccount.Namespace,
		}}
		return nil
	}); err != nil {
		return nil, err
	}

	token, err := getTokenForServiceAccount(serviceAccount)
	if err != nil {
		return nil, err
	}

	r := shootClient.RESTConfig()
	restConfig := &rest.Config{
		Host:            r.Host,
		TLSClientConfig: rest.TLSClientConfig{CAData: r.CAData},
		BearerToken:     token,
	}

	return kubernetes.NewWithConfig(kubernetes.WithRESTConfig(restConfig), kubernetes.WithDisabledCachedClient())
}
