// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedrestriction

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/admissioncontroller/seedidentity"
	acadmission "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenoperationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	seedmanagementv1alpha1helper "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "seedrestriction"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/admission/seedrestriction"
)

var (
	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	backupBucketResource              = gardencorev1beta1.Resource("backupbuckets")
	backupEntryResource               = gardencorev1beta1.Resource("backupentries")
	bastionResource                   = gardenoperationsv1alpha1.Resource("bastions")
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	clusterRoleBindingResource        = rbacv1.Resource("clusterrolebindings")
	leaseResource                     = coordinationv1.Resource("leases")
	secretResource                    = corev1.Resource("secrets")
	seedResource                      = gardencorev1beta1.Resource("seeds")
	serviceAccountResource            = corev1.Resource("serviceaccounts")
	shootStateResource                = gardencorev1beta1.Resource("shootstates")
)

// New creates a new webhook handler restricting requests by gardenlets. It allows all requests.
func New(ctx context.Context, logger logr.Logger, cache cache.Cache) (*handler, error) {
	// Initialize caches here to ensure the readyz informer check will only succeed once informers required for this
	// handler have synced so that http requests can be served quicker with pre-syncronized caches.
	if _, err := cache.GetInformer(ctx, &gardencorev1beta1.BackupBucket{}); err != nil {
		return nil, err
	}
	if _, err := cache.GetInformer(ctx, &seedmanagementv1alpha1.ManagedSeed{}); err != nil {
		return nil, err
	}
	if _, err := cache.GetInformer(ctx, &gardencorev1beta1.Seed{}); err != nil {
		return nil, err
	}
	if _, err := cache.GetInformer(ctx, &gardencorev1beta1.Shoot{}); err != nil {
		return nil, err
	}

	return &handler{
		logger:      logger,
		cacheReader: cache,
	}, nil
}

type handler struct {
	logger      logr.Logger
	cacheReader client.Reader
	decoder     *admission.Decoder
}

var _ admission.Handler = &handler{}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	seedName, isSeed := seedidentity.FromAuthenticationV1UserInfo(request.UserInfo)
	if !isSeed {
		return acadmission.Allowed("")
	}

	requestResource := schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}
	switch requestResource {
	case backupBucketResource:
		return h.admitBackupBucket(ctx, seedName, request)
	case backupEntryResource:
		return h.admitBackupEntry(ctx, seedName, request)
	case bastionResource:
		return h.admitBastion(seedName, request)
	case certificateSigningRequestResource:
		return h.admitCertificateSigningRequest(seedName, request)
	case clusterRoleBindingResource:
		return h.admitClusterRoleBinding(ctx, seedName, request)
	case leaseResource:
		return h.admitLease(seedName, request)
	case secretResource:
		return h.admitSecret(ctx, seedName, request)
	case seedResource:
		return h.admitSeed(ctx, seedName, request)
	case serviceAccountResource:
		return h.admitServiceAccount(ctx, seedName, request)
	case shootStateResource:
		return h.admitShootState(ctx, seedName, request)
	}

	return acadmission.Allowed("")
}

func (h *handler) admitBackupBucket(ctx context.Context, seedName string, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Create:
		// If a gardenlet tries to create a BackupBucket then the request may only be allowed if the used `.spec.seedName`
		// is equal to the gardenlet's seed.
		backupBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.decoder.Decode(request, backupBucket); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return h.admit(seedName, backupBucket.Spec.SeedName)

	case admissionv1.Delete:
		// Allow request if seed name is not known (ambiguous case).
		if seedName == "" {
			return admission.Allowed("")
		}
		// If a gardenlet tries to delete a BackupBucket then it may only be allowed if the name is equal to the UID of
		// the gardenlet's seed.
		seed := &gardencorev1beta1.Seed{}
		if err := h.cacheReader.Get(ctx, kutil.Key(seedName), seed); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if string(seed.UID) != request.Name {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("cannot delete unrelated BackupBucket"))
		}
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
}

func (h *handler) admitBackupEntry(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	backupEntry := &gardencorev1beta1.BackupEntry{}
	if err := h.decoder.Decode(request, backupEntry); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if resp := h.admit(seedName, backupEntry.Spec.SeedName); !resp.Allowed {
		return resp
	}

	backupBucket := &gardencorev1beta1.BackupBucket{}
	if err := h.cacheReader.Get(ctx, kutil.Key(backupEntry.Spec.BucketName), backupBucket); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return h.admit(seedName, backupBucket.Spec.SeedName)
}

func (h *handler) admitBastion(seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	bastion := &gardenoperationsv1alpha1.Bastion{}
	if err := h.decoder.Decode(request, bastion); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	return h.admit(seedName, bastion.Spec.SeedName)
}

func (h *handler) admitCertificateSigningRequest(seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	var (
		req    []byte
		usages []certificatesv1.KeyUsage
	)

	switch request.Resource.Version {
	case "v1":
		csr := &certificatesv1.CertificateSigningRequest{}
		if err := h.decoder.Decode(request, csr); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		req = csr.Spec.Request
		usages = csr.Spec.Usages

	case "v1beta1":
		csr := &certificatesv1beta1.CertificateSigningRequest{}
		if err := h.decoder.Decode(request, csr); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		req = csr.Spec.Request
		usages = kutil.CertificatesV1beta1UsagesToCertificatesV1Usages(csr.Spec.Usages)
	}

	x509cr, err := utils.DecodeCertificateRequest(req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !gutil.IsSeedClientCert(x509cr, usages) {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("can only create CSRs for seed clusters"))
	}

	seedNameInCSR, _ := seedidentity.FromCertificateSigningRequest(x509cr)
	return h.admit(seedName, &seedNameInCSR)
}

func (h *handler) admitClusterRoleBinding(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Allow gardenlet to create cluster role bindings referencing service accounts which can be used to bootstrap other
	// gardenlets deployed as part of the ManagedSeed reconciliation.
	if strings.HasPrefix(request.Name, bootstraputil.ClusterRoleBindingNamePrefix) {
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
		if err := h.decoder.Decode(request, clusterRoleBinding); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if clusterRoleBinding.RoleRef.APIGroup != rbacv1.GroupName ||
			clusterRoleBinding.RoleRef.Kind != "ClusterRole" ||
			clusterRoleBinding.RoleRef.Name != bootstraputil.GardenerSeedBootstrapper {

			return admission.Errored(http.StatusForbidden, fmt.Errorf("can only bindings referring to the bootstrapper role"))
		}

		managedSeedNamespace, managedSeedName := bootstraputil.MetadataFromClusterRoleBindingName(request.Name)
		return h.allowIfManagedSeedIsNotYetBootstrapped(ctx, seedName, managedSeedNamespace, managedSeedName)
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *handler) admitLease(seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// This allows the gardenlet to create a Lease for leader election (if the garden cluster is a seed as well).
	if request.Name == "gardenlet-leader-election" {
		return admission.Allowed("")
	}

	// Each gardenlet creates a Lease with the name of its own seed in the `gardener-system-seed-lease` namespace.
	if request.Namespace == gardencorev1beta1.GardenerSeedLeaseNamespace {
		return h.admit(seedName, &request.Name)
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *handler) admitSecret(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Check if the secret is related to a BackupBucket assigned to the seed the gardenlet is responsible for.
	if strings.HasPrefix(request.Name, v1beta1constants.SecretPrefixGeneratedBackupBucket) {
		backupBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.cacheReader.Get(ctx, kutil.Key(strings.TrimPrefix(request.Name, v1beta1constants.SecretPrefixGeneratedBackupBucket)), backupBucket); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, backupBucket.Spec.SeedName)
	}

	// Check if the secret is related to a Shoot assigned to the seed the gardenlet is responsible for.
	if shootName, ok := gutil.IsShootProjectSecret(request.Name); ok {
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.cacheReader.Get(ctx, kutil.Key(request.Namespace, shootName), shoot); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, shoot.Spec.SeedName)
	}

	// Check if the secret is a bootstrap token for a ManagedSeed.
	if strings.HasPrefix(request.Name, bootstraptokenapi.BootstrapTokenSecretPrefix) && request.Namespace == metav1.NamespaceSystem {
		secret := &corev1.Secret{}
		if err := h.decoder.Decode(request, secret); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if secret.Type != corev1.SecretTypeBootstrapToken {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected secret type: %q", secret.Type))
		}
		if string(secret.Data[bootstraptokenapi.BootstrapTokenUsageAuthentication]) != "true" {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("%q must be set to 'true'", bootstraptokenapi.BootstrapTokenUsageAuthentication))
		}
		if string(secret.Data[bootstraptokenapi.BootstrapTokenUsageSigningKey]) != "true" {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("%q must be set to 'true'", bootstraptokenapi.BootstrapTokenUsageSigningKey))
		}
		if _, ok := secret.Data[bootstraptokenapi.BootstrapTokenExtraGroupsKey]; ok {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("%q must not be set", bootstraptokenapi.BootstrapTokenExtraGroupsKey))
		}

		managedSeedNamespace, managedSeedName := bootstraputil.MetadataFromDescription(
			string(secret.Data[bootstraptokenapi.BootstrapTokenDescriptionKey]),
			bootstraputil.KindManagedSeed,
		)

		return h.allowIfManagedSeedIsNotYetBootstrapped(ctx, seedName, managedSeedNamespace, managedSeedName)
	}

	// Check if the secret is related to a ManagedSeed assigned to the seed the gardenlet is responsible for.
	managedSeedList := &seedmanagementv1alpha1.ManagedSeedList{}
	if err := h.cacheReader.List(ctx, managedSeedList); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	for _, managedSeed := range managedSeedList.Items {
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.cacheReader.Get(ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if !h.admit(seedName, shoot.Spec.SeedName).Allowed {
			continue
		}

		seedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(&managedSeed)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if seedTemplate.Spec.SecretRef != nil &&
			seedTemplate.Spec.SecretRef.Namespace == request.Namespace &&
			seedTemplate.Spec.SecretRef.Name == request.Name {
			return admission.Allowed("")
		}

		if seedTemplate.Spec.Backup != nil &&
			seedTemplate.Spec.Backup.SecretRef.Namespace == request.Namespace &&
			seedTemplate.Spec.Backup.SecretRef.Name == request.Name {
			return admission.Allowed("")
		}
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *handler) admitSeed(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation == admissionv1.Connect {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	response := h.admit(seedName, &request.Name)
	if response.Allowed {
		return response
	}

	// If the request is not allowed then we check whether the Seed object in question is the result of a ManagedSeed
	// reconciliation. In this case, the another gardenlet (the "parent gardenlet") which is usually responsible for a
	// different seed is doing the request.
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := h.cacheReader.Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, request.Name), managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			return response
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	switch request.Operation {
	case admissionv1.Create, admissionv1.Update:
		// If a gardenlet tries to create/update a Seed belonging to a ManagedSeed then the request may only be
		// considered further if the `.spec.seedTemplate` is set.
		if managedSeed.Spec.SeedTemplate == nil {
			return response
		}
	case admissionv1.Delete:
		// If a gardenlet tries to delete a Seed belonging to a ManagedSeed then the request may only be considered
		// further if the `.spec.deletionTimestamp` is set (gardenlets themselves are not allowed to delete ManagedSeeds,
		// so it's safe to only continue if somebody else has set this deletion timestamp).
		if managedSeed.DeletionTimestamp == nil {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("object can only be deleted if corresponding ManagedSeed has a deletion timestamp"))
		}
	}

	// If for whatever reason the `.spec.shoot` is nil then we exit early.
	if managedSeed.Spec.Shoot == nil {
		return response
	}

	// Check if the `.spec.seedName` of the Shoot referenced in the `.spec.shoot.name` field of the ManagedSeed matches
	// the seed name of the requesting gardenlet.
	shoot := &gardencorev1beta1.Shoot{}
	if err := h.cacheReader.Get(ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return h.admit(seedName, shoot.Spec.SeedName)
}

func (h *handler) admitServiceAccount(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Allow gardenlet to create service accounts which can be used to bootstrap other gardenlets deployed as part of
	// the ManagedSeed reconciliation.
	if strings.HasPrefix(request.Name, bootstraputil.ServiceAccountNamePrefix) {
		return h.allowIfManagedSeedIsNotYetBootstrapped(ctx, seedName, request.Namespace, strings.TrimPrefix(request.Name, bootstraputil.ServiceAccountNamePrefix))
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *handler) admitShootState(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := h.cacheReader.Get(ctx, kutil.Key(request.Namespace, request.Name), shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return h.admit(seedName, shoot.Spec.SeedName, shoot.Status.SeedName)
}

func (h *handler) admit(seedName string, seedNamesForObject ...*string) admission.Response {
	// Allow request if seed name is not known (ambiguous case).
	if seedName == "" {
		return admission.Allowed("")
	}

	// Allow request if one of the seed names for the object matches the seed name of the requesting user.
	for _, seedNameForObject := range seedNamesForObject {
		if seedNameForObject != nil && *seedNameForObject == seedName {
			return admission.Allowed("")
		}
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *handler) allowIfManagedSeedIsNotYetBootstrapped(ctx context.Context, seedName, managedSeedNamespace, managedSeedName string) admission.Response {
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := h.cacheReader.Get(ctx, kutil.Key(managedSeedNamespace, managedSeedName), managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusForbidden, err)
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := h.cacheReader.Get(ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if response := h.admit(seedName, shoot.Spec.SeedName); !response.Allowed {
		return response
	}

	seed := &gardencorev1beta1.Seed{}
	if err := h.cacheReader.Get(ctx, kutil.Key(managedSeedName), seed); err != nil {
		if !apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.Allowed("")
	} else if seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(time.Now().UTC()) {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("managed seed %s/%s is already bootstrapped", managedSeed.Namespace, managedSeed.Name))
}
