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

package tokenrequestor

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1clientset "k8s.io/client-go/kubernetes/typed/core/v1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
)

const (
	defaultExpirationDuration = 12 * time.Hour
	maxExpirationDuration     = 24 * time.Hour
)

// Reconciler requests and refreshes tokens via the TokenRequest API.
type Reconciler struct {
	SourceClient       client.Client
	TargetClient       client.Client
	TargetCoreV1Client corev1clientset.CoreV1Interface
	Config             config.TokenRequestorControllerConfig
	Clock              clock.Clock
	JitterFunc         func(time.Duration, float64) time.Duration
}

// Reconcile requests and populates tokens.
func (r *Reconciler) Reconcile(reconcileCtx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(reconcileCtx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(reconcileCtx, time.Minute)
	defer cancel()

	secret := &corev1.Secret{}
	if err := r.SourceClient.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if !isRelevantSecret(secret) {
		return reconcile.Result{}, nil
	}

	mustRequeue, requeueAfter, err := r.requeue(ctx, secret)
	if err != nil {
		return reconcile.Result{}, err
	}
	if mustRequeue {
		log.Info("No need to generate new token, renewal is scheduled", "after", requeueAfter)
		return reconcile.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	log.Info("Requesting new token")

	serviceAccount, err := r.reconcileServiceAccount(ctx, secret)
	if err != nil {
		return reconcile.Result{}, err
	}

	expirationSeconds, err := tokenExpirationSeconds(secret)
	if err != nil {
		return reconcile.Result{}, err
	}

	tokenRequest, err := r.createServiceAccountToken(ctx, serviceAccount, expirationSeconds)
	if err != nil {
		return reconcile.Result{}, err
	}

	renewDuration := r.renewDuration(tokenRequest.Status.ExpirationTimestamp.Time)

	if err := r.reconcileSecret(ctx, log, secret, tokenRequest.Status.Token, renewDuration); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update Secret with token: %w", err)
	}

	log.Info("Successfully requested token and scheduled renewal", "after", renewDuration)
	return reconcile.Result{Requeue: true, RequeueAfter: renewDuration}, nil
}

func (r *Reconciler) reconcileServiceAccount(ctx context.Context, secret *corev1.Secret) (*corev1.ServiceAccount, error) {
	serviceAccount := getServiceAccountFromAnnotations(secret.Annotations)

	if _, err := controllerutil.CreateOrUpdate(ctx, r.TargetClient, serviceAccount, func() error {
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		return nil
	}); err != nil {
		return nil, err
	}

	return serviceAccount, nil
}

func (r *Reconciler) reconcileSecret(ctx context.Context, log logr.Logger, sourceSecret *corev1.Secret, token string, renewDuration time.Duration) error {
	// The "requesting component" (e.g. gardenlet) might concurrently update the kubeconfig field in order to update the
	// included CA bundle. Hence, we need to use optimistic locking to ensure we don't accidentally overwrite concurrent
	// updates.
	// ref https://github.com/gardener/gardener/issues/6092#issuecomment-1152434616
	patch := client.MergeFromWithOptions(sourceSecret.DeepCopy(), client.MergeFromWithOptimisticLock{})
	metav1.SetMetaDataAnnotation(&sourceSecret.ObjectMeta, resourcesv1alpha1.ServiceAccountTokenRenewTimestamp, r.Clock.Now().UTC().Add(renewDuration).Format(time.RFC3339))

	if targetSecret := getTargetSecretFromAnnotations(sourceSecret.Annotations); targetSecret != nil {
		log.Info("Populating the token to the target secret", "targetSecret", client.ObjectKeyFromObject(targetSecret))

		if _, err := controllerutil.CreateOrUpdate(ctx, r.TargetClient, targetSecret, r.populateToken(log, targetSecret, token)); err != nil {
			return err
		}

		log.Info("Depopulating the token from the source secret")

		if err := r.depopulateToken(sourceSecret)(); err != nil {
			return err
		}
	} else {
		log.Info("Populating the token to the source secret")

		if err := r.populateToken(log, sourceSecret, token)(); err != nil {
			return err
		}
	}

	return r.SourceClient.Patch(ctx, sourceSecret, patch)
}

func (r *Reconciler) populateToken(log logr.Logger, secret *corev1.Secret, token string) func() error {
	return func() error {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte, 1)
		}
		return updateTokenInSecretData(log, secret.Data, token)
	}
}

func (r *Reconciler) depopulateToken(secret *corev1.Secret) func() error {
	return func() error {
		delete(secret.Data, resourcesv1alpha1.DataKeyToken)
		delete(secret.Data, resourcesv1alpha1.DataKeyKubeconfig)
		return nil
	}
}

func (r *Reconciler) createServiceAccountToken(ctx context.Context, sa *corev1.ServiceAccount, expirationSeconds int64) (*authenticationv1.TokenRequest, error) {
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         []string{v1beta1constants.GardenerAudience},
			ExpirationSeconds: &expirationSeconds,
		},
	}

	// TODO (shafeeqes): Use SubResourceClient once fakeSubResourceClient supports Create
	return r.TargetCoreV1Client.ServiceAccounts(sa.Namespace).CreateToken(ctx, sa.Name, tokenRequest, metav1.CreateOptions{})
}

func (r *Reconciler) requeue(ctx context.Context, secret *corev1.Secret) (bool, time.Duration, error) {
	var (
		secretContainingToken = secret // token is expected in source secret by default
		renewTimestamp        = secret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp]
	)

	if len(renewTimestamp) == 0 {
		return false, 0, nil
	}

	if targetSecret := getTargetSecretFromAnnotations(secret.Annotations); targetSecret != nil {
		if err := r.TargetClient.Get(ctx, client.ObjectKeyFromObject(targetSecret), targetSecret); err != nil {
			if !apierrors.IsNotFound(err) {
				return false, 0, fmt.Errorf("could not read target secret: %w", err)
			}
			// target secret is not found, so do not requeue to make sure it gets created
			return false, 0, nil
		}

		secretContainingToken = targetSecret // token is expected in target secret
	}

	tokenExists, err := tokenExistsInSecretData(secretContainingToken.Data)
	if err != nil {
		return false, 0, fmt.Errorf("could not check whether token exists in secret data: %w", err)
	}
	if !tokenExists {
		return false, 0, nil
	}

	renewTime, err := time.Parse(time.RFC3339, renewTimestamp)
	if err != nil {
		return false, 0, fmt.Errorf("could not parse renew timestamp: %w", err)
	}

	if r.Clock.Now().UTC().Before(renewTime.UTC()) {
		return true, renewTime.UTC().Sub(r.Clock.Now().UTC()), nil
	}

	return false, 0, nil
}

func (r *Reconciler) renewDuration(expirationTimestamp time.Time) time.Duration {
	expirationDuration := expirationTimestamp.UTC().Sub(r.Clock.Now().UTC())
	if expirationDuration >= maxExpirationDuration {
		expirationDuration = maxExpirationDuration
	}

	return r.JitterFunc(expirationDuration*80/100, 0.05)
}

func tokenExpirationSeconds(secret *corev1.Secret) (int64, error) {
	var (
		expirationDuration = defaultExpirationDuration
		err                error
	)

	if v, ok := secret.Annotations[resourcesv1alpha1.ServiceAccountTokenExpirationDuration]; ok {
		expirationDuration, err = time.ParseDuration(v)
		if err != nil {
			return 0, err
		}
	}

	return int64(expirationDuration / time.Second), nil
}

func getServiceAccountFromAnnotations(annotations map[string]string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      annotations[resourcesv1alpha1.ServiceAccountName],
			Namespace: annotations[resourcesv1alpha1.ServiceAccountNamespace],
		},
	}
}

func getTargetSecretFromAnnotations(annotations map[string]string) *corev1.Secret {
	var (
		name      = annotations[resourcesv1alpha1.TokenRequestorTargetSecretName]
		namespace = annotations[resourcesv1alpha1.TokenRequestorTargetSecretNamespace]
	)

	if name == "" || namespace == "" {
		return nil
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func updateTokenInSecretData(log logr.Logger, data map[string][]byte, token string) error {
	if _, ok := data[resourcesv1alpha1.DataKeyKubeconfig]; !ok {
		log.Info("Writing token to data")
		data[resourcesv1alpha1.DataKeyToken] = []byte(token)
		return nil
	}

	log.Info("Writing token as part of kubeconfig to data")

	kubeconfig, authInfo, err := decodeKubeconfigAndGetUser(data[resourcesv1alpha1.DataKeyKubeconfig])
	if err != nil {
		return err
	}

	if authInfo != nil {
		authInfo.Token = token
	}

	kubeconfigEncoded, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig)
	if err != nil {
		return err
	}

	data[resourcesv1alpha1.DataKeyKubeconfig] = kubeconfigEncoded
	return nil
}

func tokenExistsInSecretData(data map[string][]byte) (bool, error) {
	if _, ok := data[resourcesv1alpha1.DataKeyKubeconfig]; !ok {
		return data[resourcesv1alpha1.DataKeyToken] != nil, nil
	}

	_, authInfo, err := decodeKubeconfigAndGetUser(data[resourcesv1alpha1.DataKeyKubeconfig])
	if err != nil {
		return false, err
	}

	return authInfo != nil && authInfo.Token != "", nil
}

func decodeKubeconfigAndGetUser(data []byte) (*clientcmdv1.Config, *clientcmdv1.AuthInfo, error) {
	kubeconfig := &clientcmdv1.Config{}
	if _, _, err := clientcmdlatest.Codec.Decode(data, nil, kubeconfig); err != nil {
		return nil, nil, err
	}

	var userName string
	for _, namedContext := range kubeconfig.Contexts {
		if namedContext.Name == kubeconfig.CurrentContext {
			userName = namedContext.Context.AuthInfo
			break
		}
	}

	for i, users := range kubeconfig.AuthInfos {
		if users.Name == userName {
			return kubeconfig, &kubeconfig.AuthInfos[i].AuthInfo, nil
		}
	}

	return nil, nil, nil
}
