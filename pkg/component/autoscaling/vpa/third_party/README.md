# Vertical Pod Autoscaler CustomResourceDefinitions (Vendored from Upstream)

The Vertical Pod Autoscaler CustomResourceDefinitions (`verticalpodautoscalers.autoscaling.k8s.io` and `verticalpodautoscalercheckpoints.autoscaling.k8s.io`) are manually copied here to avoid running VPA 1.7.0 against VPA CRDs 1.6.0.
The CustomResourceDefinitions in the Gardener repositories are generated with `controller-gen` based on the API types which are fetched from the module dependency. In the VPA case, the module dependency is `k8s.io/autoscaler/vertical-pod-autoscaler`.
It is not possible to update the `k8s.io/autoscaler/vertical-pod-autoscaler` module dependency to v1.7.0 until https://github.com/gardener/gardener/issues/14734 is resolved.
For this reason, the VPA CRDs are manually copied in this directory.

TODO(ialidzhikov): Remove this copy, once https://github.com/gardener/gardener/issues/14734 is resolved and `k8s.io/autoscaler/vertical-pod-autoscaler` is updated to v1.7.0.
