# Istio

[Istio](https://istio.io) offers a service mesh implementation with focus on several important features - traffic, observability, security and policy.

## Gardener `ManagedIstio` feature gate

When enabled in gardenlet the `ManagedIstio` feature gate can be used to deploy a Gardener-tailored Istio installation in Seed clusters. It's main usage is to enable features such as [Shoot API server SNI](../proposals/08-shoot-apiserver-via-sni.md). This feature should not be enabled on a Seed cluster where Istio is already deployed.

## Prerequisites

- Third-party JWT is used, therefore each Seed cluster where this feature is enabled must have [Service Account Token Volume Projection](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection) enabled.
- Kubernetes 1.16+

## Differences with Istio's default profile

The [default profile](https://istio.io/docs/setup/additional-setup/config-profiles/) which is recommended for production deployment, is not suitable for the Gardener use case as it offers more functionality than desired. The current installation goes through heavy refactorings due to the `IstioOperator` and the mixture of Helm values + Kubernetes API specification makes configuring and fine-tuning it very hard. A more simplistic deployment is used by Gardener. The differences are the following:

- Telemetry is not deployed.
- `istiod` is deployed.
- `istio-ingress-gateway` is deployed in a separate `istio-ingress` namespace.
- `istio-egress-gateway` is not deployed.
- None of the Istio addons are deployed.
- Mixer (deprecated) is not deployed
- Mixer CDRs are not deployed.
- Kubernetes `Service`, Istio's `VirtualService` and `ServiceEntry` are **NOT** advertised in the service mesh. This means that if a `Service` needs to be accessed directly from the Istio Ingress Gateway, it should have `networking.istio.io/exportTo: "*"` annotation. `VirtualService` and `ServiceEntry` must have `.spec.exportTo: ["*"]` set on them respectively.
- Istio injector is not enabled.
- mTLS is enabled by default.
