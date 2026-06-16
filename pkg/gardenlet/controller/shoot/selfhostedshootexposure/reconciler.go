// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/api/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	extensionsselfhostedshootexposure "github.com/gardener/gardener/pkg/component/extensions/selfhostedshootexposure"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

const nodeRoleControlPlaneLabel = "node-role.kubernetes.io/control-plane"

// Reconciler keeps the SelfHostedShootExposure endpoints (or the external DNSRecord values in DNS-only setups) in
// sync with the self-hosted shoot's control-plane Node addresses.
type Reconciler struct {
	GardenClient  client.Client
	RuntimeClient client.Client
	ShootKey      types.NamespacedName
	Clock         clock.PassiveClock
}

// Reconcile recomputes the control-plane endpoints from the current Node state and patches the SelfHostedShootExposure
// resource or the external DNSRecord accordingly.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx).WithName(ControllerName)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.GardenClient.Get(ctx, r.ShootKey, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed getting Shoot: %w", err)
	}

	if v1beta1helper.HasExtensionExposure(shoot) {
		// Extension-based exposure: some extensions opt out of continuously updated endpoints via the ControllerRegistration.
		enabled, err := r.endpointUpdatesEnabled(ctx, shoot)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !enabled {
			log.V(1).Info("Endpoint updates disabled by ControllerRegistration")
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, r.reconcileExtensionExposure(ctx, log, shoot)
	}

	if v1beta1helper.HasDNSExposure(shoot) {
		// DNS-based exposure: the external DNSRecord points directly at the control-plane nodes' (preferably external) addresses.
		return reconcile.Result{}, r.reconcileDNSExposure(ctx, log, shoot)
	}

	// Exposure is omitted. If it was previously extension-based: point the external
	// DNSRecord at the control-plane nodes, then delete the SelfHostedShootExposure. Otherwise: do nothing.
	return reconcile.Result{}, r.reconcileExposureDisabled(ctx, log, shoot)
}

func (r *Reconciler) reconcileDNSExposure(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) error {
	nodes, err := r.listControlPlaneNodes(ctx)
	if err != nil {
		return err
	}
	// Steady-state DNS exposure: the record tracks the healthy nodes' preferably external addresses (erroring keeps the last good values).
	if err := r.updateExternalDNSRecordFromNodes(ctx, log, shoot, health.FilterHealthyNodes(nodes), corev1.NodeExternalIP, corev1.NodeInternalIP); err != nil {
		return err
	}
	// Remove a SelfHostedShootExposure left over from a previous extension-based exposure (switch).
	return r.deleteExposureIfExists(ctx, log, shoot)
}

// reconcileExposureDisabled performs the switch-off handoff for extension-based exposure to unmanaged.
func (r *Reconciler) reconcileExposureDisabled(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) error {
	exposure, err := r.getExposure(ctx, shoot)
	if err != nil || exposure == nil {
		return err
	}
	if exposure.DeletionTimestamp != nil {
		// Deletion already in flight; the final DNSRecord update was done on the reconcile that issued it.
		return nil
	}

	// Bootstrap address semantics: keep the record resolvable within the cluster network after the handoff.
	nodes, err := r.listControlPlaneNodes(ctx)
	if err != nil {
		return err
	}
	if err := r.updateExternalDNSRecordFromNodes(ctx, log, shoot, nodes, corev1.NodeInternalIP, corev1.NodeExternalIP); err != nil {
		return err
	}

	log.Info("Control plane exposure disabled, deleting SelfHostedShootExposure after final external DNSRecord update")
	return extensions.DeleteExtensionObject(ctx, r.RuntimeClient, exposure)
}

// updateExternalDNSRecordFromNodes computes the DNSRecord values from the given nodes (in the given address-type
// preference order) and patches the external DNSRecord.
func (r *Reconciler) updateExternalDNSRecordFromNodes(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot, nodes []corev1.Node, addressTypePreference ...corev1.NodeAddressType) error {
	values, recordType, err := extensionsv1alpha1helper.DNSValuesFromNodes(nodes, shoot.Spec.Networking.IPFamilies, addressTypePreference...)
	if err != nil {
		return err
	}
	return r.patchExternalDNSRecord(ctx, log, shoot, values, recordType)
}

// getExposure returns the shoot's SelfHostedShootExposure, or nil if it does not exist.
func (r *Reconciler) getExposure(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*extensionsv1alpha1.SelfHostedShootExposure, error) {
	exposure := &extensionsv1alpha1.SelfHostedShootExposure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shoot.Name,
			Namespace: v1beta1helper.ControlPlaneNamespaceForShoot(shoot),
		},
	}
	if err := r.RuntimeClient.Get(ctx, client.ObjectKeyFromObject(exposure), exposure); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return exposure, nil
}

// deleteExposureIfExists deletes the shoot's SelfHostedShootExposure if present (idempotent), used when exposure is no
// longer extension-based.
func (r *Reconciler) deleteExposureIfExists(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) error {
	exposure, err := r.getExposure(ctx, shoot)
	if err != nil || exposure == nil || exposure.DeletionTimestamp != nil {
		return err
	}
	log.Info("Deleting SelfHostedShootExposure left over from a previous extension-based exposure")
	return extensions.DeleteExtensionObject(ctx, r.RuntimeClient, exposure)
}

// patchExternalDNSRecord keeps the shoot's external DNSRecord in sync with the desired values and record type.
func (r *Reconciler) patchExternalDNSRecord(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot, values []string, recordType extensionsv1alpha1.DNSRecordType) error {
	slices.Sort(values)

	dnsRecord := &extensionsv1alpha1.DNSRecord{}
	key := types.NamespacedName{
		Name:      shoot.Name + "-" + v1beta1constants.DNSRecordExternalName,
		Namespace: v1beta1helper.ControlPlaneNamespaceForShoot(shoot),
	}
	if err := r.RuntimeClient.Get(ctx, key, dnsRecord); err != nil {
		return fmt.Errorf("failed getting external DNSRecord %q: %w", key, err)
	}

	if dnsRecord.Spec.RecordType == recordType && slices.Equal(dnsRecord.Spec.Values, values) {
		log.V(1).Info("External DNSRecord already up-to-date", "recordType", recordType, "values", values)
		return nil
	}

	patch := client.MergeFrom(dnsRecord.DeepCopy())
	dnsRecord.Spec.Values = values
	dnsRecord.Spec.RecordType = recordType
	metav1.SetMetaDataAnnotation(&dnsRecord.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
	if err := r.RuntimeClient.Patch(ctx, dnsRecord, patch); err != nil {
		return fmt.Errorf("failed patching external DNSRecord %q: %w", key, err)
	}

	log.Info("Updated external DNSRecord", "recordType", recordType, "values", values)
	return nil
}

func (r *Reconciler) endpointUpdatesEnabled(ctx context.Context, shoot *gardencorev1beta1.Shoot) (bool, error) {
	controllerInstallations := &gardencorev1beta1.ControllerInstallationList{}
	if err := r.GardenClient.List(ctx, controllerInstallations, client.MatchingFields{
		core.ShootRefName:      shoot.Name,
		core.ShootRefNamespace: shoot.Namespace,
	}); err != nil {
		return false, fmt.Errorf("failed listing ControllerInstallations: %w", err)
	}

	registrations, err := gardenerutils.GetControllerRegistrationsForInstallations(ctx, r.GardenClient, controllerInstallations)
	if err != nil {
		return false, fmt.Errorf("failed getting ControllerRegistrations: %w", err)
	}

	return v1beta1helper.ContinuousEndpointUpdateEnabled(registrations.Items, v1beta1helper.SelfHostedShootExposureExtensionType(shoot)), nil
}

func (r *Reconciler) listControlPlaneNodes(ctx context.Context) ([]corev1.Node, error) {
	nodes := &corev1.NodeList{}
	if err := r.RuntimeClient.List(ctx, nodes, client.MatchingLabels{nodeRoleControlPlaneLabel: ""}); err != nil {
		return nil, fmt.Errorf("failed listing control-plane nodes: %w", err)
	}
	return nodes.Items, nil
}

func (r *Reconciler) reconcileExtensionExposure(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) error {
	nodes, err := r.listControlPlaneNodes(ctx)
	if err != nil {
		return err
	}

	endpoints, err := gardenerutils.ControlPlaneEndpointsFromNodes(nodes)
	if err != nil {
		return err
	}

	values := &extensionsselfhostedshootexposure.Values{
		Name:      shoot.Name,
		Namespace: v1beta1helper.ControlPlaneNamespaceForShoot(shoot),
		Type:      v1beta1helper.SelfHostedShootExposureExtensionType(shoot),
		Endpoints: endpoints,
	}
	if v1beta1helper.HasManagedInfrastructure(shoot) {
		values.CredentialsRef = &corev1.ObjectReference{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
			Name:       v1beta1constants.SecretNameCloudProvider,
			Namespace:  values.Namespace,
		}
	}

	c := extensionsselfhostedshootexposure.New(log, r.RuntimeClient, values)
	if r.Clock != nil {
		c.Clock = r.Clock
	}

	existing, err := r.getExposure(ctx, shoot)
	if err != nil {
		return err
	}

	if existing != nil && health.CheckExtensionObject(existing) == nil &&
		existing.Spec.Type == values.Type &&
		apiequality.Semantic.DeepEqual(existing.Spec.Endpoints, values.Endpoints) {
		c.Ingress = existing.Status.Ingress
	} else if err := component.OpWait(c).Deploy(ctx); err != nil {
		return err
	}

	dnsValues, recordType, err := extensionsv1alpha1helper.DNSValuesFromIngress(c.Ingress, shoot.Spec.Networking.IPFamilies)
	if err != nil {
		return err
	}
	return r.patchExternalDNSRecord(ctx, log, shoot, dnsValues, recordType)
}
