package kubernetes

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetMetaDataLabel sets the key value pair in the labels section of the given ObjectMeta.
// If the given ObjectMeta did not yet have labels, they are initialized.
func SetMetaDataLabel(meta *metav1.ObjectMeta, key, value string) {
	if meta.Labels == nil {
		meta.Labels = make(map[string]string)
	}
	meta.Labels[key] = value
}

// HasMetaDataAnnotation checks if the passed meta object has the given key, value set in the annotations section.
func HasMetaDataAnnotation(meta *metav1.ObjectMeta, key, value string) bool {
	val, ok := meta.Annotations[key]
	return ok && val == value
}

// CreateOrUpdate creates or updates the object. Optionally, it executes a transformation function before the
// request is made.
func CreateOrUpdate(ctx context.Context, c client.Client, obj runtime.Object, transform func() error) error {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return err
	}

	if err := c.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			if transform != nil && transform() != nil {
				return err
			}
			return c.Create(ctx, obj)
		}
		return err
	}

	if transform != nil && transform() != nil {
		return err
	}
	return c.Update(ctx, obj)
}

func nameAndNamespace(namespaceOrName string, nameOpt ...string) (namespace, name string) {
	if len(nameOpt) > 1 {
		panic(fmt.Sprintf("more than name/namespace for key specified: %s/%v", namespaceOrName, nameOpt))
	}
	if len(nameOpt) == 0 {
		name = namespaceOrName
		return
	}
	namespace = namespaceOrName
	name = nameOpt[0]
	return
}

// Key creates a new client.ObjectKey from the given parameters.
// There are only two ways to call this function:
// - If only namespaceOrName is set, then a client.ObjectKey with name set to namespaceOrName is returned.
// - If namespaceOrName and one nameOpt is given, then a client.ObjectKey with namespace set to namespaceOrName
//   and name set to nameOpt[0] is returned.
// For all other cases, this method panics.
func Key(namespaceOrName string, nameOpt ...string) client.ObjectKey {
	namespace, name := nameAndNamespace(namespaceOrName, nameOpt...)
	return client.ObjectKey{Namespace: namespace, Name: name}
}

// ObjectMeta creates a new metav1.ObjectMeta from the given parameters.
// There are only two ways to call this function:
// - If only namespaceOrName is set, then a metav1.ObjectMeta with name set to namespaceOrName is returned.
// - If namespaceOrName and one nameOpt is given, then a metav1.ObjectMeta with namespace set to namespaceOrName
//   and name set to nameOpt[0] is returned.
// For all other cases, this method panics.
func ObjectMeta(namespaceOrName string, nameOpt ...string) metav1.ObjectMeta {
	namespace, name := nameAndNamespace(namespaceOrName, nameOpt...)
	return metav1.ObjectMeta{Namespace: namespace, Name: name}
}
