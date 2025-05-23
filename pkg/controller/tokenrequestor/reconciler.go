// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tokenrequestor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

const (
	defaultExpirationDuration = 12 * time.Hour
	maxExpirationDuration     = 24 * time.Hour
)

// Reconciler requests and refreshes tokens via the TokenRequest API.
type Reconciler struct {
	SourceClient    client.Client
	TargetClient    client.Client
	ConcurrentSyncs int
	Clock           clock.Clock
	JitterFunc      func(time.Duration, float64) time.Duration
	Class           *string
	APIAudiences    []string
	CAData          []byte
	// TargetNamespace is the namespace that requested ServiceAccounts should be created in.
	// If TargetNamespace is empty, the controller uses the namespace specified in the
	// serviceaccount.resources.gardener.cloud/namespace annotation.
	TargetNamespace string
}

// Reconcile requests and populates tokens.
func (r *Reconciler) Reconcile(reconcileCtx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(reconcileCtx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(reconcileCtx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	secret := &corev1.Secret{}
	if err := r.SourceClient.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if !r.isRelevantSecret(secret) {
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
	serviceAccount := r.getServiceAccountFromAnnotations(secret.Annotations)

	var labels map[string]string
	if labelsJSON := secret.Annotations[resourcesv1alpha1.ServiceAccountLabels]; labelsJSON != "" {
		labels = make(map[string]string)
		if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
			return nil, fmt.Errorf("failed unmarshalling service account labels from secret annotation %q (%s): %w", resourcesv1alpha1.ServiceAccountLabels, labelsJSON, err)
		}
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.TargetClient, serviceAccount, func() error {
		serviceAccount.Labels = labels
		serviceAccount.AutomountServiceAccountToken = ptr.To(false)
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
	shouldInjectCA, _ := strconv.ParseBool(sourceSecret.Annotations[resourcesv1alpha1.ServiceAccountInjectCABundle])
	if shouldInjectCA {
		log.Info("Injecting CA bundle into secret")
	}

	if targetSecret := getTargetSecretFromAnnotations(sourceSecret.Annotations); targetSecret != nil {
		log.Info("Populating the token to the target secret", "targetSecret", client.ObjectKeyFromObject(targetSecret))

		if _, err := controllerutil.CreateOrUpdate(ctx, r.TargetClient, targetSecret, r.populateSecretData(log, targetSecret, token, shouldInjectCA)); err != nil {
			return err
		}

		log.Info("Depopulating the token from the source secret")

		if err := r.depopulateToken(sourceSecret)(); err != nil {
			return err
		}
	} else {
		log.Info("Populating the token to the source secret")

		if err := r.populateSecretData(log, sourceSecret, token, shouldInjectCA)(); err != nil {
			return err
		}
	}

	return r.SourceClient.Patch(ctx, sourceSecret, patch)
}

func (r *Reconciler) populateSecretData(log logr.Logger, secret *corev1.Secret, token string, shouldInjectCA bool) func() error {
	return func() error {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte, 1)
		}
		var ca []byte
		if shouldInjectCA {
			ca = r.CAData
		}
		return updateSecretData(log, secret.Data, token, ca)
	}
}

func (r *Reconciler) depopulateToken(secret *corev1.Secret) func() error {
	return func() error {
		delete(secret.Data, resourcesv1alpha1.DataKeyToken)
		delete(secret.Data, resourcesv1alpha1.DataKeyCABundle)
		delete(secret.Data, resourcesv1alpha1.DataKeyKubeconfig)
		return nil
	}
}

func (r *Reconciler) createServiceAccountToken(ctx context.Context, sa *corev1.ServiceAccount, expirationSeconds int64) (*authenticationv1.TokenRequest, error) {
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         r.APIAudiences,
			ExpirationSeconds: &expirationSeconds,
		},
	}

	if err := r.TargetClient.SubResource("token").Create(ctx, sa, tokenRequest); err != nil {
		return nil, err
	}

	return tokenRequest, nil
}

func (r *Reconciler) requeue(ctx context.Context, secret *corev1.Secret) (bool, time.Duration, error) {
	var (
		secretContainingToken = secret // token is expected in source secret by default
		renewTimestamp        = secret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp]
		checkBundle, _        = strconv.ParseBool(secret.Annotations[resourcesv1alpha1.ServiceAccountInjectCABundle])
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
	if checkBundle {
		isBundleOk, err := r.isCABundleUpdated(secretContainingToken.Data)
		if err != nil {
			return false, 0, fmt.Errorf("could not check whether the caBundle is up to date: %w", err)
		}
		if !isBundleOk {
			return false, 0, nil
		}
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

func (r *Reconciler) getServiceAccountFromAnnotations(annotations map[string]string) *corev1.ServiceAccount {
	namespace := r.TargetNamespace
	if namespace == "" {
		namespace = annotations[resourcesv1alpha1.ServiceAccountNamespace]
	}

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      annotations[resourcesv1alpha1.ServiceAccountName],
			Namespace: namespace,
		},
	}
}

func (r *Reconciler) isCABundleUpdated(data map[string][]byte) (bool, error) {
	if _, ok := data[resourcesv1alpha1.DataKeyKubeconfig]; !ok {
		return bytes.Equal(data[resourcesv1alpha1.DataKeyCABundle], r.CAData), nil
	}

	kubeconfig, err := decodeKubeconfig(data[resourcesv1alpha1.DataKeyKubeconfig])
	if err != nil {
		return false, err
	}

	cluster, err := getCluster(kubeconfig)
	if err != nil {
		return false, err
	}

	return bytes.Equal(cluster.CertificateAuthorityData, r.CAData), nil
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

func updateSecretData(log logr.Logger, data map[string][]byte, token string, caData []byte) error {
	if _, ok := data[resourcesv1alpha1.DataKeyKubeconfig]; !ok {
		log.Info("Writing token to data")
		data[resourcesv1alpha1.DataKeyToken] = []byte(token)
		if len(caData) > 0 {
			data[resourcesv1alpha1.DataKeyCABundle] = caData
		} else {
			delete(data, resourcesv1alpha1.DataKeyCABundle)
		}
		return nil
	}

	log.Info("Writing token as part of kubeconfig to data")

	kubeconfig, err := decodeKubeconfig(data[resourcesv1alpha1.DataKeyKubeconfig])
	if err != nil {
		return err
	}

	authInfo, err := getAuthInfo(kubeconfig)
	if err != nil {
		return err
	}

	if authInfo != nil {
		authInfo.Token = token
	}

	if len(caData) > 0 {
		cluster, err := getCluster(kubeconfig)
		if err != nil {
			return err
		}
		cluster.CertificateAuthorityData = caData
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

	kubeconfig, err := decodeKubeconfig(data[resourcesv1alpha1.DataKeyKubeconfig])
	if err != nil {
		return false, err
	}

	authInfo, err := getAuthInfo(kubeconfig)
	if err != nil {
		return false, err
	}

	return authInfo != nil && authInfo.Token != "", nil
}

func decodeKubeconfig(data []byte) (*clientcmdv1.Config, error) {
	kubeconfig := &clientcmdv1.Config{}
	if _, _, err := clientcmdlatest.Codec.Decode(data, nil, kubeconfig); err != nil {
		return nil, err
	}
	return kubeconfig, nil
}

func getAuthInfo(kubeconfig *clientcmdv1.Config) (*clientcmdv1.AuthInfo, error) {
	ctx, err := getCurrentContext(kubeconfig)
	if err != nil {
		return nil, err
	}

	for i, authInfo := range kubeconfig.AuthInfos {
		if authInfo.Name == ctx.AuthInfo {
			return &kubeconfig.AuthInfos[i].AuthInfo, nil
		}
	}

	return nil, fmt.Errorf("did not find authInfo of current context named %s", ctx.AuthInfo)
}

func getCluster(kubeconfig *clientcmdv1.Config) (*clientcmdv1.Cluster, error) {
	ctx, err := getCurrentContext(kubeconfig)
	if err != nil {
		return nil, err
	}

	for i, cluster := range kubeconfig.Clusters {
		if cluster.Name == ctx.Cluster {
			return &kubeconfig.Clusters[i].Cluster, nil
		}
	}

	return nil, fmt.Errorf("did not find cluster of current context named %s", ctx.Cluster)
}

func getCurrentContext(kubeconfig *clientcmdv1.Config) (clientcmdv1.Context, error) {
	for _, namedContext := range kubeconfig.Contexts {
		if namedContext.Name == kubeconfig.CurrentContext {
			return namedContext.Context, nil
		}
	}
	return clientcmdv1.Context{}, fmt.Errorf("did not find context defined in current context")
}
