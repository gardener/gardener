// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresources

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/utils/chart"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/managedresources/builder"
	"github.com/gardener/gardener/pkg/utils/retry"
)

const (
	// SecretPrefix is the prefix that can be used for secrets referenced by managed resources.
	SecretPrefix = "managedresource-"
	// LabelKeyOrigin is a key for a label on a managed resource with the value 'origin'.
	LabelKeyOrigin = "origin"
	// LabelValueGardener is a value for a label on a managed resource with the value 'gardener'.
	LabelValueGardener = "gardener"
	// LabelValueOperator is a value for an origin label on a managed resource with the value 'gardener-operator'.
	LabelValueOperator = "gardener-operator"
	// SigningSaltSecretNamespace is the namespace in which the signing secret is located.
	SigningSaltSecretNamespace = v1beta1constants.GardenNamespace
	// SigningSaltSecretName is the name of the secret containing the salt used for signing managed resources secrets.
	SigningSaltSecretName = "gardener-resource-manager-signing-secret-salt"
	// SigningSaltSecretKey is the key in the signing secret containing the salt used for signing managed resources secrets.
	SigningSaltSecretKey = "salt"
	// SignatureAnnotationKey is the key for the annotation on the secret containing the signature of managed resource secrets.
	SignatureAnnotationKey = "gardener.cloud/managed-resource-signature"
)

// New initiates a new ManagedResource object which can be reconciled.
func New(client client.Client, namespace, name, class string, keepObjects *bool, labels, injectedLabels map[string]string, forceOverwriteAnnotations *bool) *builder.ManagedResource {
	mr := builder.NewManagedResource(client).
		WithNamespacedName(namespace, name).
		WithClass(class).
		WithLabels(labels).
		WithInjectedLabels(injectedLabels)

	if keepObjects != nil {
		mr = mr.KeepObjects(*keepObjects)
	}
	if forceOverwriteAnnotations != nil {
		mr = mr.ForceOverwriteAnnotations(*forceOverwriteAnnotations)
	}

	return mr
}

// NewForShoot constructs a new ManagedResource object for the shoot's gardener-resource-manager.
// The origin is used to identify the creator of the managed resource. Gardener acts on resources
// with "origin=gardener" label. External callers (extension controllers or other components)
// of this function should provide their own unique origin value.
func NewForShoot(c client.Client, namespace, name, origin string, keepObjects bool) *builder.ManagedResource {
	var (
		injectedLabels = map[string]string{v1beta1constants.ShootNoCleanup: "true"}
		labels         = map[string]string{LabelKeyOrigin: origin}
	)

	return New(c, namespace, name, "", &keepObjects, labels, injectedLabels, nil)
}

// NewForSeed constructs a new ManagedResource object for the seed's gardener-resource-manager.
func NewForSeed(c client.Client, namespace, name string, keepObjects bool) *builder.ManagedResource {
	var labels map[string]string
	if !strings.HasPrefix(namespace, v1beta1constants.TechnicalIDPrefix) {
		labels = map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeedSystemComponent}
	}

	return New(c, namespace, name, v1beta1constants.SeedResourceManagerClass, &keepObjects, labels, nil, nil)
}

// NewSecret initiates a new immutable Secret object which can be reconciled.
func NewSecret(client client.Client, namespace, name string, data map[string][]byte, secretNameWithPrefix bool) (string, *builder.Secret) {
	secretName := secretName(name, secretNameWithPrefix)
	return builder.NewSecret(client).
		WithNamespacedName(namespace, secretName).
		WithKeyValues(data).
		Unique()
}

// secretName returns the name of a corev1.Secret for the given name of a resourcesv1alpha1.ManagedResource. If
// <withPrefix> is set then the name will be prefixed with 'managedresource-'.
func secretName(name string, withPrefix bool) string {
	if withPrefix {
		return SecretPrefix + name
	}
	return name
}

// CreateFromUnstructured creates a managed resource and its secret with the given name, class, and objects in the given namespace.
func CreateFromUnstructured(
	ctx context.Context,
	client client.Client,
	namespace, name string,
	secretNameWithPrefix bool,
	class string,
	objs []*unstructured.Unstructured,
	keepObjects bool,
	injectedLabels map[string]string,
) error {
	var data []byte
	for _, obj := range objs {
		bytes, err := obj.MarshalJSON()
		if err != nil {
			return fmt.Errorf("marshal failed for '%s/%s' for secret '%s/%s': %w", obj.GetNamespace(), obj.GetName(), namespace, name, err)
		}
		data = append(data, []byte("\n---\n")...)
		data = append(data, bytes...)
	}
	dataMap := map[string][]byte{}
	if len(data) > 0 {
		dataMap[name] = data
	}
	return Create(ctx, client, namespace, name, nil, secretNameWithPrefix, class, dataMap, &keepObjects, injectedLabels, ptr.To(false))
}

// Update updates a managed resource and its secret with the given name, class, key, and data in the given namespace.
func Update(
	ctx context.Context,
	client client.Client,
	namespace, name string,
	labels map[string]string,
	secretNameWithPrefix bool,
	class string,
	data map[string][]byte,
	keepObjects *bool,
	injectedLabels map[string]string,
	forceOverwriteAnnotations *bool,
) error {
	var (
		signature, err     = calculateSignature(ctx, client, data)
		secretName, secret = NewSecret(client, namespace, name, data, secretNameWithPrefix)
		managedResource    = New(client, namespace, name, class, keepObjects, labels, injectedLabels, forceOverwriteAnnotations).WithSecretRef(secretName).CreateIfNotExists(false)
	)

	if err != nil {
		return err
	}

	secret.WithAnnotations(map[string]string{
		SignatureAnnotationKey: signature,
	})

	return deployManagedResource(ctx, secret, managedResource)
}

// Create creates a managed resource and its secret with the given name, class, key, and data in the given namespace.
func Create(
	ctx context.Context,
	c client.Client,
	namespace, name string,
	labels map[string]string,
	secretNameWithPrefix bool,
	class string,
	data map[string][]byte,
	keepObjects *bool,
	injectedLabels map[string]string,
	forceOverwriteAnnotations *bool,
) error {
	var (
		signature, err     = calculateSignature(ctx, c, data)
		secretName, secret = NewSecret(c, namespace, name, data, secretNameWithPrefix)
		managedResource    = New(c, namespace, name, class, keepObjects, labels, injectedLabels, forceOverwriteAnnotations).WithSecretRef(secretName)
	)
	if err != nil {
		return err
	}

	secret.WithAnnotations(map[string]string{
		SignatureAnnotationKey: signature,
	})

	// we should fetch the signing secret here and calculate the signature before deploying the managed resources

	return deployManagedResource(ctx, secret, managedResource)
}

// CreateForSeed deploys a ManagedResource CR for the seed's gardener-resource-manager.
func CreateForSeed(ctx context.Context, c client.Client, namespace, name string, keepObjects bool, data map[string][]byte) error {
	var (
		signature, err     = calculateSignature(ctx, c, data)
		secretName, secret = NewSecret(c, namespace, name, data, true)
		managedResource    = NewForSeed(c, namespace, name, keepObjects).WithSecretRef(secretName)
	)
	if err != nil {
		return err
	}

	secret.WithAnnotations(map[string]string{
		SignatureAnnotationKey: signature,
	})

	return deployManagedResource(ctx, secret, managedResource)
}

// CreateForSeedWithLabels deploys a ManagedResource CR for the seed's gardener-resource-manager and allows providing
// additional labels.
func CreateForSeedWithLabels(ctx context.Context, c client.Client, namespace, name string, keepObjects bool, labels map[string]string, data map[string][]byte) error {
	var (
		signature, err     = calculateSignature(ctx, c, data)
		secretName, secret = NewSecret(c, namespace, name, data, true)
		managedResource    = NewForSeed(c, namespace, name, keepObjects).WithSecretRef(secretName).WithLabels(labels)
	)
	if err != nil {
		return err
	}

	secret.WithAnnotations(map[string]string{
		SignatureAnnotationKey: signature,
	})

	return deployManagedResource(ctx, secret, managedResource)
}

// CreateForShoot deploys a ManagedResource CR for the shoot's gardener-resource-manager.
// The origin is used to identify the creator of the managed resource. Gardener acts on resources
// with "origin=gardener" label. External callers (extension controllers or other components)
// of this function should provide their own unique origin value.
func CreateForShoot(ctx context.Context, c client.Client, namespace, name, origin string, keepObjects bool, data map[string][]byte) error {
	var (
		signature, err     = calculateSignature(ctx, c, data)
		secretName, secret = NewSecret(c, namespace, name, data, true)
		managedResource    = NewForShoot(c, namespace, name, origin, keepObjects).WithSecretRef(secretName)
	)
	if err != nil {
		return err
	}

	secret.WithAnnotations(map[string]string{
		SignatureAnnotationKey: signature,
	})

	return deployManagedResource(ctx, secret, managedResource)
}

// CreateForShootWithLabels deploys a ManagedResource CR for the shoot's gardener-resource-manager. The origin is used
// to identify the creator of the managed resource. Gardener acts on resources with "origin=gardener" label. External
// callers (extension controllers or other components) of this function should provide their own unique origin value.
// This function allows providing additional labels.
func CreateForShootWithLabels(ctx context.Context, c client.Client, namespace, name, origin string, keepObjects bool, labels map[string]string, data map[string][]byte) error {
	var (
		signature, err     = calculateSignature(ctx, c, data)
		secretName, secret = NewSecret(c, namespace, name, data, true)
		managedResource    = NewForShoot(c, namespace, name, origin, keepObjects).WithSecretRef(secretName).WithLabels(labels)
	)
	if err != nil {
		return err
	}

	secret.WithAnnotations(map[string]string{
		SignatureAnnotationKey: signature,
	})

	return deployManagedResource(ctx, secret, managedResource)
}

func VerifySignature(ctx context.Context, c client.Client, secret *corev1.Secret) error {
	want, err := calculateSignature(ctx, c, secret.Data)
	if err != nil {
		return err
	}
	got, ok := secret.Annotations[SignatureAnnotationKey]
	if !ok {
		return fmt.Errorf("missing signature annotation %q in secret %q", SignatureAnnotationKey, client.ObjectKeyFromObject(secret).String())
	}

	if want != got {
		return fmt.Errorf("invalid signature annotation %q for secret %q", SignatureAnnotationKey, client.ObjectKeyFromObject(secret).String())
	}

	return nil
}

func calculateSignature(ctx context.Context, c client.Client, data map[string][]byte) (string, error) {
	saltSecret := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{
		Name:      SigningSaltSecretName,
		Namespace: SigningSaltSecretNamespace,
	}, saltSecret)
	if err != nil {
		return "", err
	}

	rawSalt, ok := saltSecret.Data[SigningSaltSecretKey]
	if !ok {
		return "", fmt.Errorf("could not find %q key in secret %q", SigningSaltSecretKey, client.ObjectKeyFromObject(saltSecret).String())
	}

	hash := sha512.New()
	hash.Write(rawSalt)

	secretKeys := make([]string, 0, len(data))
	for secretKey := range data {
		secretKeys = append(secretKeys, secretKey)
	}
	slices.Sort(secretKeys)

	for _, secretKey := range secretKeys {
		hash.Write([]byte(secretKey))
		hash.Write(data[secretKey])
	}

	signature := hex.EncodeToString(hash.Sum(nil))
	return signature, nil
}

func deployManagedResource(ctx context.Context, secret *builder.Secret, managedResource *builder.ManagedResource) error {
	if err := secret.Reconcile(ctx); err != nil {
		return fmt.Errorf("could not create or update secret of managed resources: %w", err)
	}

	if err := managedResource.Reconcile(ctx); err != nil {
		return fmt.Errorf("could not create or update managed resource: %w", err)
	}

	return nil
}

// Delete deletes the managed resource and its secrets with the given name in the given namespace.
func Delete(ctx context.Context, c client.Client, namespace string, name string, secretNameWithPrefix bool) error {
	// Always try to delete the secret with generated name.
	// This is done in order to guarantee backwards compatibility with previous versions of this library
	// when the underlying mananaged resource secrets were not immutable and not garbage collectable.
	// For more details, please see https://github.com/gardener/gardener/pull/8116
	secretName := secretName(name, secretNameWithPrefix)

	mr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	mrKey := client.ObjectKeyFromObject(mr)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}}

	err := c.Get(ctx, mrKey, mr)
	if err != nil && apierrors.IsNotFound(err) {
		// just try to delete the secret with generated name
		if err := client.IgnoreNotFound(c.Delete(ctx, secret)); err != nil {
			return fmt.Errorf("could not delete secret '%s' of managed resource: %w", client.ObjectKeyFromObject(secret).String(), err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("could not get managed resource '%s': %w", mrKey.String(), err)
	}

	secretsToDelete := []*corev1.Secret{secret}
	for _, secretRef := range mr.Spec.SecretRefs {
		secretsToDelete = append(secretsToDelete, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      secretRef.Name,
			Namespace: namespace,
		}})
	}

	// Delete the secrets first so we do not lose reference to them
	// in case the mr gets deleted and something fails immediately after that.
	// Finalizers should prevent the deletion of the secrets before the managed resource is deleted.
	for _, s := range secretsToDelete {
		if err := client.IgnoreNotFound(c.Delete(ctx, s)); err != nil {
			return fmt.Errorf("could not delete secret '%s' of managed resource: %w", client.ObjectKeyFromObject(s).String(), err)
		}
	}

	if err := client.IgnoreNotFound(c.Delete(ctx, mr)); err != nil {
		return fmt.Errorf("could not delete managed resource '%s': %w", mrKey.String(), err)
	}

	return nil
}

var (
	// DeleteForSeed is a function alias for deleteWithSecretNamePrefix.
	DeleteForSeed = deleteWithSecretNamePrefix
	// DeleteForShoot is a function alias for deleteWithSecretNamePrefix.
	DeleteForShoot = deleteWithSecretNamePrefix
)

func deleteWithSecretNamePrefix(ctx context.Context, client client.Client, namespace string, name string) error {
	return Delete(ctx, client, namespace, name, true)
}

// IntervalWait is the interval when waiting for managed resources.
var IntervalWait = 2 * time.Second

// WaitUntilHealthy waits until the given managed resource is healthy.
func WaitUntilHealthy(ctx context.Context, reader client.Reader, namespace, name string) error {
	return waitUntilHealthy(ctx, reader, namespace, name, false)
}

// WaitUntilHealthyAndNotProgressing waits until the given managed resource is healthy and not progressing.
func WaitUntilHealthyAndNotProgressing(ctx context.Context, reader client.Reader, namespace, name string) error {
	return waitUntilHealthy(ctx, reader, namespace, name, true)
}

func waitUntilHealthy(ctx context.Context, reader client.Reader, namespace, name string, andNotProgressing bool) error {
	obj := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return retry.Until(ctx, IntervalWait, func(ctx context.Context) (done bool, err error) {
		if err := reader.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckManagedResource(obj); err != nil {
			return retry.MinorError(fmt.Errorf("managed resource %s/%s is not healthy", namespace, name))
		}

		if andNotProgressing {
			if err := health.CheckManagedResourceProgressing(obj); err != nil {
				return retry.MinorError(fmt.Errorf("managed resource %s/%s is still progressing", namespace, name))
			}
		}

		return retry.Ok()
	})
}

// WaitUntilListDeleted waits until the given managed resources are deleted.
func WaitUntilListDeleted(ctx context.Context, client client.Client, mrList *resourcesv1alpha1.ManagedResourceList, listOps ...client.ListOption) error {
	allErrs := v1beta1helper.NewMultiErrorWithCodes(
		errorsutils.NewErrorFormatFuncWithPrefix("error while waiting for all resources to be deleted: "),
	)

	if err := kubernetesutils.WaitUntilResourcesDeleted(ctx, client, mrList, IntervalWait, listOps...); err != nil {
		for _, mr := range mrList.Items {
			resourcesAppliedCondition := v1beta1helper.GetCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
			if resourcesAppliedCondition != nil && resourcesAppliedCondition.Status != gardencorev1beta1.ConditionTrue &&
				(resourcesAppliedCondition.Reason == resourcesv1alpha1.ConditionDeletionFailed || resourcesAppliedCondition.Reason == resourcesv1alpha1.ConditionDeletionPending) {
				deleteError := fmt.Errorf("%w:\n%s", err, resourcesAppliedCondition.Message)

				allErrs.Append(v1beta1helper.NewErrorWithCodes(deleteError, checkConfigurationError(err)...))
			}
		}
	}

	return allErrs.ErrorOrNil()
}

// WaitUntilDeleted waits until the given managed resource is deleted.
func WaitUntilDeleted(ctx context.Context, client client.Client, namespace, name string) error {
	mr := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := kubernetesutils.WaitUntilResourceDeleted(ctx, client, mr, IntervalWait); err != nil {
		resourcesAppliedCondition := v1beta1helper.GetCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
		if resourcesAppliedCondition != nil && resourcesAppliedCondition.Status != gardencorev1beta1.ConditionTrue &&
			(resourcesAppliedCondition.Reason == resourcesv1alpha1.ConditionDeletionFailed || resourcesAppliedCondition.Reason == resourcesv1alpha1.ConditionDeletionPending) {
			deleteError := fmt.Errorf("error while waiting for all resources to be deleted: %w:\n%s", err, resourcesAppliedCondition.Message)
			return v1beta1helper.NewErrorWithCodes(deleteError, checkConfigurationError(err)...)
		}
		return err
	}
	return nil
}

// SetKeepObjects updates the keepObjects field of the managed resource with the given name in the given namespace.
func SetKeepObjects(ctx context.Context, c client.Writer, namespace, name string, keepObjects bool) error {
	resource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	patch := client.MergeFrom(resource.DeepCopy())
	resource.Spec.KeepObjects = &keepObjects
	if err := c.Patch(ctx, resource, patch); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("could not update managed resource '%s/%s': %w", namespace, name, err)
	}

	return nil
}

// RenderChartAndCreate renders a chart and creates a ManagedResource for the gardener-resource-manager
// out of the results.
func RenderChartAndCreate(ctx context.Context, namespace string, name string, secretNameWithPrefix bool, client client.Client, chartRenderer chartrenderer.Interface, chart chart.Interface, values map[string]any, imageVector imagevector.ImageVector, chartNamespace string, version string, withNoCleanupLabel bool, forceOverwriteAnnotations bool) error {
	chartName, data, err := chart.Render(chartRenderer, chartNamespace, imageVector, version, version, values)
	if err != nil {
		return fmt.Errorf("could not render chart: %w", err)
	}

	// Create or update managed resource referencing the previously created secret
	var injectedLabels map[string]string
	if withNoCleanupLabel {
		injectedLabels = map[string]string{v1beta1constants.ShootNoCleanup: "true"}
	}

	return Create(ctx, client, namespace, name, nil, secretNameWithPrefix, "", map[string][]byte{chartName: data}, ptr.To(false), injectedLabels, &forceOverwriteAnnotations)
}

// configurationProblemRegex is used to check if an error is caused by a bad managed resource configuration.
var configurationProblemRegex = regexp.MustCompile(`(?i)(error during apply of object .* is invalid:)`)

func checkConfigurationError(err error) []gardencorev1beta1.ErrorCode {
	var errorCodes []gardencorev1beta1.ErrorCode
	if configurationProblemRegex.MatchString(err.Error()) {
		errorCodes = append(errorCodes, gardencorev1beta1.ErrorConfigurationProblem)
	}

	return errorCodes
}

// CheckIfManagedResourcesExist checks if some ManagedResources of the given class still exist. If yes it returns true.
func CheckIfManagedResourcesExist(ctx context.Context, c client.Client, class *string, excludeNames ...string) (bool, error) {
	managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
	if err := c.List(ctx, managedResourceList); err != nil {
		return false, err
	}

	for _, managedResource := range managedResourceList.Items {
		if ptr.Equal(managedResource.Spec.Class, class) && !sets.New(excludeNames...).Has(managedResource.Name) {
			return true, nil
		}
	}

	return false, nil
}

// GetObjects returns the objects which belong to this managed resource.
func GetObjects(ctx context.Context, c client.Client, namespace, name string) ([]client.Object, error) {
	var objects []client.Object

	managedResource := &resourcesv1alpha1.ManagedResource{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, managedResource); err != nil {
		return nil, fmt.Errorf("could not get ManagedResource %q: %w", client.ObjectKey{Namespace: namespace, Name: name}, err)
	}

	decoder := serializer.NewCodecFactory(c.Scheme()).UniversalDeserializer()
	for _, secretRef := range managedResource.Spec.SecretRefs {
		secret := &corev1.Secret{}
		if err := c.Get(ctx, client.ObjectKey{Name: secretRef.Name, Namespace: managedResource.Namespace}, secret); err != nil {
			return nil, fmt.Errorf("could not get secret %q: %w", client.ObjectKey{Name: secretRef.Name, Namespace: managedResource.Namespace}, err)
		}

		objectsFromSecret, err := extractObjectsFromSecret(decoder, secret)
		if err != nil {
			return nil, fmt.Errorf("could not extract objects from secret %q: %w", client.ObjectKeyFromObject(secret), err)
		}

		objects = append(objects, objectsFromSecret...)
	}

	return objects, nil
}

func extractObjectsFromSecret(decoder runtime.Decoder, secret *corev1.Secret) ([]client.Object, error) {
	var objects []client.Object

	for key, value := range secret.Data {
		var data []byte

		if strings.HasSuffix(key, resourcesv1alpha1.BrotliCompressionSuffix) {
			reader := brotli.NewReader(bytes.NewReader(value))
			var err error
			data, err = io.ReadAll(reader)
			if err != nil {
				return nil, fmt.Errorf("could not read brotli compressed data from key %q: %w", key, err)
			}
		} else {
			data = value
		}

		for _, objRaw := range strings.Split(string(data), "---\n") {
			if objRaw == "" {
				continue
			}

			obj, _, err := decoder.Decode([]byte(objRaw), nil, nil)
			if err != nil {
				return nil, fmt.Errorf("could not decode object: %w", err)
			}

			objects = append(objects, obj.(client.Object))
		}
	}

	return objects, nil
}
