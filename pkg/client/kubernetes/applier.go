// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
)

// defaultApplier applies objects by retrieving their current state and then either creating / updating them
// (update happens with a predefined merge logic).
type defaultApplier struct {
	client     client.Client
	restMapper meta.RESTMapper
}

// NewApplier constructs a new Applier from the given client.
func NewApplier(c client.Client, restMapper meta.RESTMapper) Applier {
	return &defaultApplier{client: c, restMapper: restMapper}
}

// NewApplierForConfig creates a new Applier for the given rest.Config.
// Use NewApplier if you already have a client and RESTMapper at hand, as this will create a new direct client.
func NewApplierForConfig(config *rest.Config) (Applier, error) {
	opts := client.Options{}

	if err := setClientOptionsDefaults(config, &opts); err != nil {
		return nil, err
	}

	c, err := client.New(config, opts)
	if err != nil {
		return nil, err
	}

	return NewApplier(c, opts.Mapper), nil
}

func (a *defaultApplier) applyObject(ctx context.Context, desired *unstructured.Unstructured, options MergeFuncs) error {
	a.setNamespace(desired)

	key := client.ObjectKeyFromObject(desired)
	if len(key.Name) == 0 {
		return fmt.Errorf("missing 'metadata.name' in: %+v", desired)
	}

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(desired.GroupVersionKind())
	if err := a.client.Get(ctx, key, current); err != nil {
		if apierrors.IsNotFound(err) {
			return a.client.Create(ctx, desired)
		}
		return err
	}

	if err := a.mergeObjects(desired, current, options); err != nil {
		return err
	}

	return a.client.Update(ctx, desired)
}

func (a *defaultApplier) deleteObject(ctx context.Context, desired *unstructured.Unstructured, opts *DeleteManifestOptions) error {
	a.setNamespace(desired)

	if len(desired.GetName()) == 0 {
		return fmt.Errorf("missing 'metadata.name' in: %+v", desired)
	}

	err := a.client.Delete(ctx, desired)
	if err != nil {
		// this is kept for backwards compatibility.
		if apierrors.IsNotFound(err) {
			return nil
		}

		for _, tf := range opts.TolerateErrorFuncs {
			if tf != nil && tf(err) {
				return nil
			}
		}
	}

	return err
}

// DefaultMergeFuncs contains options for common k8s objects, e.g. Service, ServiceAccount.
var (
	DefaultMergeFuncs = MergeFuncs{
		corev1.SchemeGroupVersion.WithKind("Service").GroupKind(): func(newObj, oldObj *unstructured.Unstructured) {
			newSvcType, found, _ := unstructured.NestedString(newObj.Object, "spec", "type")
			if !found {
				newSvcType = string(corev1.ServiceTypeClusterIP)
				_ = unstructured.SetNestedField(newObj.Object, newSvcType, "spec", "type")
			}

			oldSvcType, found, _ := unstructured.NestedString(oldObj.Object, "spec", "type")
			if !found {
				oldSvcType = string(corev1.ServiceTypeClusterIP)
			}

			annotations, found, _ := unstructured.NestedMap(oldObj.Object, "metadata", "annotations")
			if found {
				mergedAnnotations := make(map[string]any)
				for key, value := range annotations {
					annotation := key
					annotationValue := value.(string)
					for _, keepAnnotation := range keepServiceAnnotations() {
						if strings.HasPrefix(annotation, keepAnnotation) {
							mergedAnnotations[annotation] = annotationValue
						}
					}
				}

				newAnnotations, found, _ := unstructured.NestedMap(newObj.Object, "metadata", "annotations")
				if found {
					for key, value := range newAnnotations {
						mergedAnnotations[key] = value.(string)
					}
				}

				_ = unstructured.SetNestedMap(newObj.Object, mergedAnnotations, "metadata", "annotations")
			}

			switch newSvcType {
			case string(corev1.ServiceTypeLoadBalancer), string(corev1.ServiceTypeNodePort):
				oldPorts, found, _ := unstructured.NestedSlice(oldObj.Object, "spec", "ports")
				if !found {
					// no old ports probably means that the service was of type External name before.
					break
				}

				newPorts, found, _ := unstructured.NestedSlice(newObj.Object, "spec", "ports")
				if !found {
					// no new ports is safe to ignore
					break
				}

				ports := make([]any, 0, len(newPorts))
				for _, newPort := range newPorts {
					np := newPort.(map[string]any)
					npName, _, _ := unstructured.NestedString(np, "name")
					npPort, _ := nestedFloat64OrInt64(np, "port")
					nodePort, ok := nestedFloat64OrInt64(np, "nodePort")

					for _, oldPortObj := range oldPorts {
						op := oldPortObj.(map[string]any)
						opName, _, _ := unstructured.NestedString(op, "name")
						opPort, _ := nestedFloat64OrInt64(op, "port")

						if (opName == npName || opPort == npPort) && (!ok || nodePort == 0) {
							np["nodePort"] = op["nodePort"]
						}
					}

					ports = append(ports, np)
				}

				_ = unstructured.SetNestedSlice(newObj.Object, ports, "spec", "ports")

			case string(corev1.ServiceTypeExternalName):
				// there is no ClusterIP in this case
				return
			}

			// ClusterIP is immutable unless that old service is of type ExternalName
			if oldSvcType != string(corev1.ServiceTypeExternalName) {
				newClusterIP, _, _ := unstructured.NestedString(newObj.Object, "spec", "clusterIP")
				if newClusterIP != corev1.ClusterIPNone || newSvcType != string(corev1.ServiceTypeClusterIP) {
					oldClusterIP, _, _ := unstructured.NestedString(oldObj.Object, "spec", "clusterIP")
					_ = unstructured.SetNestedField(newObj.Object, oldClusterIP, "spec", "clusterIP")
				}
			}

			newETP, _, _ := unstructured.NestedString(newObj.Object, "spec", "externalTrafficPolicy")
			oldETP, _, _ := unstructured.NestedString(oldObj.Object, "spec", "externalTrafficPolicy")

			if oldSvcType == string(corev1.ServiceTypeLoadBalancer) &&
				newSvcType == string(corev1.ServiceTypeLoadBalancer) &&
				newETP == string(corev1.ServiceExternalTrafficPolicyLocal) &&
				oldETP == string(corev1.ServiceExternalTrafficPolicyLocal) {
				newHealthCheckPort, _ := nestedFloat64OrInt64(newObj.Object, "spec", "healthCheckNodePort")
				if newHealthCheckPort == 0 {
					oldHealthCheckPort, _ := nestedFloat64OrInt64(oldObj.Object, "spec", "healthCheckNodePort")
					_ = unstructured.SetNestedField(newObj.Object, oldHealthCheckPort, "spec", "healthCheckNodePort")
				}
			}

		},
		corev1.SchemeGroupVersion.WithKind("ServiceAccount").GroupKind(): func(newObj, oldObj *unstructured.Unstructured) {
			// We do not want to overwrite a ServiceAccount's `.secrets[]` list or `.imagePullSecrets[]`.
			newObj.Object["secrets"] = oldObj.Object["secrets"]
			newObj.Object["imagePullSecrets"] = oldObj.Object["imagePullSecrets"]
		},
		{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}: func(newObj, oldObj *unstructured.Unstructured) {
			// Never override the status of VPA resources
			if oldStatus := oldObj.Object["status"]; oldStatus != nil {
				newObj.Object["status"] = oldStatus
			}
		},
	}

	DeploymentKeepReplicasMergeFunc = MergeFunc(func(newObj, oldObj *unstructured.Unstructured) {
		oldReplicas, ok := nestedFloat64OrInt64(oldObj.Object, "spec", "replicas")
		if !ok {
			return
		}
		_ = unstructured.SetNestedField(newObj.Object, oldReplicas, "spec", "replicas")
	})
)

func nestedFloat64OrInt64(obj map[string]any, fields ...string) (int64, bool) {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil {
		return 0, found
	}

	f, ok := val.(float64)
	if ok {
		return int64(f), true
	}

	i, ok := val.(int64)
	if ok {
		return i, true
	}

	return 0, false
}

// CopyApplierOptions returns a copies of the provided applier options.
func CopyApplierOptions(in MergeFuncs) MergeFuncs {
	out := make(MergeFuncs, len(in))

	for k, v := range in {
		out[k] = v
	}

	return out
}

func keepServiceAnnotations() []string {
	return []string{"loadbalancer.openstack.org"}
}

func (a *defaultApplier) mergeObjects(newObj, oldObj *unstructured.Unstructured, mergeFuncs MergeFuncs) error {
	newObj.SetResourceVersion(oldObj.GetResourceVersion())

	// We do not want to overwrite the Finalizers.
	newObj.Object["metadata"].(map[string]any)["finalizers"] = oldObj.Object["metadata"].(map[string]any)["finalizers"]

	if merge, ok := mergeFuncs[newObj.GroupVersionKind().GroupKind()]; ok {
		merge(newObj, oldObj)
	}

	return nil
}

// setNamespace looks up scope of objects' kind to check if we should default the namespace field
func (a *defaultApplier) setNamespace(desired *unstructured.Unstructured) {
	mapping, err := a.restMapper.RESTMapping(desired.GroupVersionKind().GroupKind(), desired.GroupVersionKind().Version)
	if err != nil || mapping == nil {
		// Don't reset RESTMapper in case of cache misses. Most probably indicates, that the corresponding CRD is not yet applied.
		// CRD might be applied later as part of the same chart

		// default namespace on a best effort basis
		if desired.GetKind() != "Namespace" && desired.GetNamespace() == "" {
			desired.SetNamespace(metav1.NamespaceDefault)
		}
	} else {
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// default namespace field to `default` in case of namespaced kinds
			if desired.GetNamespace() == "" {
				desired.SetNamespace(metav1.NamespaceDefault)
			}
		} else {
			// unset namespace field in case of non-namespaced kinds
			desired.SetNamespace("")
		}
	}
}

// ApplyManifest is a function which does the same like `kubectl apply -f <file>`. It takes a bunch of manifests <m>,
// all concatenated in a byte slice, and sends them one after the other to the API server. If a resource
// already exists at the API server, it will update it. It returns an error as soon as the first error occurs.
func (a *defaultApplier) ApplyManifest(ctx context.Context, r UnstructuredReader, options MergeFuncs) error {
	allErrs := &multierror.Error{
		ErrorFormat: errorsutils.NewErrorFormatFuncWithPrefix("failed to apply manifests"),
	}

	for {
		obj, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("could not read object: %+v", err))
			continue
		}
		if obj == nil {
			continue
		}

		if err := a.applyObject(ctx, obj, options); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("could not apply object of kind %q \"%s/%s\": %+v", obj.GetKind(), obj.GetNamespace(), obj.GetName(), err))
			continue
		}
	}

	return allErrs.ErrorOrNil()
}

// DeleteManifest is a function which does the same like `kubectl delete -f <file>`. It takes a bunch of manifests <m>,
// all concatenated in a byte slice, and sends them one after the other to the API server for deletion.
// It returns an error as soon as the first error occurs.
func (a *defaultApplier) DeleteManifest(ctx context.Context, r UnstructuredReader, opts ...DeleteManifestOption) error {
	allErrs := &multierror.Error{
		ErrorFormat: errorsutils.NewErrorFormatFuncWithPrefix("failed to delete manifests"),
	}

	deleteOps := &DeleteManifestOptions{}

	for _, o := range opts {
		if o != nil {
			o.MutateDeleteManifestOptions(deleteOps)
		}
	}

	for {
		obj, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("could not read object: %+v", err))
			continue
		}
		if obj == nil {
			continue
		}

		if err := a.deleteObject(ctx, obj, deleteOps); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("could not delete object of kind %q \"%s/%s\": %+v", obj.GetKind(), obj.GetNamespace(), obj.GetName(), err))
			continue
		}
	}

	return allErrs.ErrorOrNil()
}

// UnstructuredReader an interface that all manifest readers should implement
type UnstructuredReader interface {
	Read() (*unstructured.Unstructured, error)
}

// NewManifestReader initializes a reader for yaml manifests
func NewManifestReader(manifest []byte) UnstructuredReader {
	return &manifestReader{
		decoder:  yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 1024),
		manifest: manifest,
	}
}

// manifestReader is an unstructured reader that contains a JSONDecoder
type manifestReader struct {
	decoder  *yaml.YAMLOrJSONDecoder
	manifest []byte
}

// Read decodes yaml data into an unstructured object
func (m *manifestReader) Read() (*unstructured.Unstructured, error) {
	// loop for skipping empty yaml objects
	for {
		var data map[string]any

		err := m.decoder.Decode(&data)
		if err == io.EOF {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("error '%+v' decoding manifest: %s", err, string(m.manifest))
		}
		if data == nil {
			continue
		}

		return &unstructured.Unstructured{Object: data}, nil
	}
}

// NewNamespaceSettingReader initializes a reader for yaml manifests with support for setting the namespace
func NewNamespaceSettingReader(mReader UnstructuredReader, namespace string) UnstructuredReader {
	return &namespaceSettingReader{
		reader:    mReader,
		namespace: namespace,
	}
}

// namespaceSettingReader is an unstructured reader that contains a JSONDecoder and a manifest reader (or other reader types)
type namespaceSettingReader struct {
	reader    UnstructuredReader
	namespace string
}

// Read decodes yaml data into an unstructured object
func (n *namespaceSettingReader) Read() (*unstructured.Unstructured, error) {
	readObj, err := n.reader.Read()
	if err != nil {
		return nil, err
	}

	readObj.SetNamespace(n.namespace)

	return readObj, nil
}

// NewObjectReferenceReader initializes a reader from ObjectReference
func NewObjectReferenceReader(objectReference *corev1.ObjectReference) UnstructuredReader {
	return &objectReferenceReader{
		objectReference: objectReference,
	}
}

// objectReferenceReader is an unstructured reader that contains a ObjectReference
type objectReferenceReader struct {
	objectReference *corev1.ObjectReference
}

// Read translates ObjectReference into Unstructured object
func (r *objectReferenceReader) Read() (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(r.objectReference.APIVersion)
	obj.SetKind(r.objectReference.Kind)
	obj.SetNamespace(r.objectReference.Namespace)
	obj.SetName(r.objectReference.Name)

	return obj, nil
}
