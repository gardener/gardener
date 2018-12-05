package kubernetes

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
