package validation

import (
	"strings"

	"github.com/gardener/etcd-druid/api/v1alpha1"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateEtcd validates Etcd object.
func ValidateEtcd(etcd *v1alpha1.Etcd) field.ErrorList {
	return validateEtcdSpec(etcd, field.NewPath("spec"))
}

// ValidateEtcdUpdate validates a Etcd object before an update.
func ValidateEtcdUpdate(new, old *v1alpha1.Etcd) field.ErrorList {
	return validateEtcdSpecUpdate(&new.Spec, &old.Spec, field.NewPath("spec"))
}

func validateEtcdSpec(etcd *v1alpha1.Etcd, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	prefix := etcd.Spec.Backup.Store.Prefix
	if !validatePrefixName(prefix, etcd.Name, etcd.Namespace) {
		allErrs = append(allErrs, field.Invalid(path.Child("backup.store.prefix"), prefix, "field is required"))
	}

	return allErrs
}

func validatePrefixName(prefix, name, ns string) bool {
	return strings.Contains(prefix, ns) && strings.Contains(prefix, name)
}

func validateEtcdSpecUpdate(new, old *v1alpha1.EtcdSpec, path *field.Path) field.ErrorList {
	return apivalidation.ValidateImmutableField(new.Backup.Store, old.Backup.Store, path)
}
