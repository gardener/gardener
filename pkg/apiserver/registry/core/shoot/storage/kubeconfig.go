// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/gardener/gardener/pkg/api"
	authenticationapi "github.com/gardener/gardener/pkg/apis/authentication"
	authenticationvalidation "github.com/gardener/gardener/pkg/apis/authentication/validation"
	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

// KubeconfigREST implements a RESTStorage for a kubeconfig request.
type KubeconfigREST struct {
	// TODO(petersutter): Remove secretLister field from struct after v1.135 has been released, as the cluster CA should then only be read from the ConfigMap.
	secretLister         kubecorev1listers.SecretLister
	internalSecretLister gardencorev1beta1listers.InternalSecretLister
	configMapLister      kubecorev1listers.ConfigMapLister
	shootStorage         getter
	maxExpirationSeconds int64

	gvk                           schema.GroupVersionKind
	newObjectFunc                 func() runtime.Object
	clientCertificateOrganization string
}

var (
	_ = rest.NamedCreater(&KubeconfigREST{})
	_ = rest.GroupVersionKindProvider(&KubeconfigREST{})
)

// New returns an instance of the object.
func (r *KubeconfigREST) New() runtime.Object {
	return r.newObjectFunc()
}

// Destroy cleans up its resources on shutdown.
func (r *KubeconfigREST) Destroy() {
	// Given that underlying store is shared with REST, we don't destroy it here explicitly.
}

// Create returns a kubeconfig request with kubeconfig based on
// - shoot's advertised addresses
// - shoot's certificate authority
// - user making the request
// - configured organization for the client certificate
func (r *KubeconfigREST) Create(ctx context.Context, name string, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	if createValidation != nil {
		if err := createValidation(ctx, obj.DeepCopyObject()); err != nil {
			return nil, err
		}
	}

	kubeconfigRequest := &authenticationapi.KubeconfigRequest{}
	if err := api.Scheme.Convert(obj, kubeconfigRequest, nil); err != nil {
		return nil, fmt.Errorf("failed converting %T to %T: %w", obj, kubeconfigRequest, err)
	}

	if errs := authenticationvalidation.ValidateKubeconfigRequest(kubeconfigRequest); len(errs) != 0 {
		return nil, apierrors.NewInvalid(r.gvk.GroupKind(), "", errs)
	}

	userInfo, ok := genericapirequest.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("no user in context")
	}

	// prepare: get shoot object
	shootObj, err := r.shootStorage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	shoot, ok := shootObj.(*core.Shoot)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("cannot convert to *core.Shoot object - got type %T", shootObj))
	}

	// filter only addresses that actually advertise the kube-apiserver
	// it is possible that the list of addresses also include URLs like the shoot's issuer URL
	var kubeAPIServerAddresses []core.ShootAdvertisedAddress
	for _, addr := range shoot.Status.AdvertisedAddresses {
		if addr.Name == v1beta1constants.AdvertisedAddressExternal ||
			addr.Name == v1beta1constants.AdvertisedAddressInternal ||
			addr.Name == v1beta1constants.AdvertisedAddressWildcardTLSSeedBound ||
			addr.Name == v1beta1constants.AdvertisedAddressUnmanaged {
			kubeAPIServerAddresses = append(kubeAPIServerAddresses, addr)
		}
	}
	if len(kubeAPIServerAddresses) == 0 {
		fieldErr := field.Invalid(field.NewPath("status", "status"), shoot.Status.AdvertisedAddresses, "no suitable advertised address for kube-apiserver found in .status.advertisedAddresses")
		return nil, apierrors.NewInvalid(r.gvk.GroupKind(), shoot.Name, field.ErrorList{fieldErr})
	}

	// prepare: get cluster and client CA
	caClientSecret, err := r.internalSecretLister.InternalSecrets(shoot.Namespace).Get(gardenerutils.ComputeShootProjectResourceName(shoot.Name, gardenerutils.ShootProjectSecretSuffixCAClient))
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not get client CA secret: %w", err))
	}

	clientCACertificate, err := secrets.LoadCertificate("", caClientSecret.Data[secrets.DataKeyPrivateKeyCA], caClientSecret.Data[secrets.DataKeyCertificateCA])
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not load client CA certificate from secret: %w", err))
	}

	var clusterCABundle []byte
	caClusterConfigMap, err := r.configMapLister.ConfigMaps(shoot.Namespace).Get(gardenerutils.ComputeShootProjectResourceName(shoot.Name, gardenerutils.ShootProjectConfigMapSuffixCACluster))
	// TODO(petersutter): Remove this fallback of reading the <shoot-name>.ca-cluster Secret after v1.135 has been released
	if apierrors.IsNotFound(err) {
		caClusterSecret, err := r.secretLister.Secrets(shoot.Namespace).Get(gardenerutils.ComputeShootProjectResourceName(shoot.Name, gardenerutils.ShootProjectSecretSuffixCACluster))
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("could not get cluster CA secret: %w", err))
		}
		clusterCABundle = caClusterSecret.Data[secrets.DataKeyCertificateCA]
	} else if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not get cluster CA config map: %w", err))
	} else {
		clusterCABundle = []byte(caClusterConfigMap.Data[secrets.DataKeyCertificateCA])
	}

	if len(clusterCABundle) == 0 {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not load cluster CA bundle"))
	}

	// generate kubeconfig with client certificate
	if r.maxExpirationSeconds > 0 && kubeconfigRequest.Spec.ExpirationSeconds > r.maxExpirationSeconds {
		kubeconfigRequest.Spec.ExpirationSeconds = r.maxExpirationSeconds
	}

	var (
		validity = time.Duration(kubeconfigRequest.Spec.ExpirationSeconds) * time.Second
		authName = fmt.Sprintf("%s--%s", shoot.Namespace, shoot.Name)
		cpsc     = secrets.ControlPlaneSecretConfig{
			Name: authName,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:   userInfo.GetName(),
				Organization: []string{r.clientCertificateOrganization},
				CertType:     secrets.ClientCert,
				Validity:     &validity,
				SigningCA:    clientCACertificate,
			},
		}
	)

	for _, address := range kubeAPIServerAddresses {
		u, err := url.Parse(address.URL)
		if err != nil {
			return nil, err
		}

		request := secrets.KubeConfigRequest{
			ClusterName:   fmt.Sprintf("%s-%s", authName, address.Name),
			APIServerHost: u.Host,
		}

		if address.Name != v1beta1constants.AdvertisedAddressWildcardTLSSeedBound {
			request.CAData = clusterCABundle
		}

		cpsc.KubeConfigRequests = append(cpsc.KubeConfigRequests, request)
	}

	cp, err := cpsc.Generate()
	if err != nil {
		return nil, err
	}
	controlPlaneSecret := cp.(*secrets.ControlPlane)

	// return generated kubeconfig in status
	kubeconfigRequest.Status.Kubeconfig = controlPlaneSecret.Kubeconfig
	kubeconfigRequest.Status.ExpirationTimestamp = metav1.Time{Time: controlPlaneSecret.Certificate.Certificate.NotAfter}

	if err := api.Scheme.Convert(kubeconfigRequest, obj, nil); err != nil {
		return nil, fmt.Errorf("failed converting %T to %T: %w", kubeconfigRequest, obj, err)
	}

	return obj, nil
}

// GroupVersionKind returns the GVK for the kubeconfig request type.
func (r *KubeconfigREST) GroupVersionKind(schema.GroupVersion) schema.GroupVersionKind {
	return r.gvk
}

type getter interface {
	Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
}
