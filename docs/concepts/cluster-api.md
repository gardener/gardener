---
title: Cluster API
description: Understand the evolution of the Gardener API and its relation to the Cluster API
categories:
  - Users
---

## Relation Between Gardener API and Cluster API (SIG Cluster Lifecycle)

The Cluster API (CAPI) and Gardener approach Kubernetes cluster management with different, albeit related, philosophies. In essence, **Cluster API primarily harmonizes *how to get to* clusters, while Gardener goes a significant step further by also harmonizing *the clusters themselves*.**

Gardener already provides a declarative, Kubernetes-native API to manage the full lifecycle of conformant Kubernetes clusters. This is a key distinction, as many other managed Kubernetes services are often exposed via proprietary REST APIs or imperative CLIs, whereas Gardener's API *is* Kubernetes. Gardener is inherently multi-cloud and, by design, unifies far more aspects of a cluster's make-up and operational behavior than Cluster API currently aims to.

### Gardener's Homogeneity vs. CAPI's Provider Model

The Cluster API delegates the specifics of cluster creation and management to providers for infrastructures (e.g., AWS, Azure, GCP) and control planes, each with their own Custom Resource Definitions (CRDs). This means that different Cluster API providers can result in vastly different Kubernetes clusters in terms of their configuration (see, e.g., [AKS](https://capz.sigs.k8s.io/managed/managedcluster#specification) vs [GKE](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-gcp/refs/heads/main/templates/cluster-template-gke.yaml)), available Kubernetes versions, operating systems, control plane setup, included add-ons, and operational behavior.

In stark contrast, Gardener provides homogeneous clusters. Regardless of the underlying infrastructure (AWS, Azure, GCP, OpenStack, etc.), you get clusters with the exact same Kubernetes version, operating system choices, control plane configuration (API server, kubelet, etc.), core add-ons (overlay network, DNS, metrics, logging, etc.), and consistent behavior for updates, auto-scaling, self-healing, credential rotation, you name it. This deep harmonization is a core design goal of Gardener, aimed at simplifying operations for teams developing and shipping software on Kubernetes across diverse environments. Gardener's extensive coverage in the [official Kubernetes conformance test grid](https://testgrid.k8s.io/conformance-gardener) underscores this commitment.

### Historical Context and Evolution

Gardener has actively followed and contributed to Cluster API. Notably, Gardener heavily influenced the Machine API concepts within Cluster API through its [Machine Controller Manager](https://github.com/gardener/machine-controller-manager) and was the [first to adopt it](https://github.com/kubernetes-sigs/cluster-api/commit/00b1ead264aea6f88585559056c180771cce3815). A [joint KubeCon talk between SIG Cluster Lifecycle and Gardener](https://www.youtube.com/watch?v=Mtg8jygK3Hs) further details this collaboration.

Cluster API has evolved significantly, especially from `v1alpha1` to `v1alpha2`, which put a strong emphasis on a machine-based paradigm ([The Cluster API Book - Updating a v1alpha1 provider to a v1alpha2 infrastructure provider](https://release-0-2.cluster-api.sigs.k8s.io/providers/v1alpha1-to-v1alpha2)). This "demoted" v1alpha1 providers to mere infrastructure providers, creating an "impedance mismatch" for fully managed services like GKE (which runs on Borg), Gardener (which uses a "kubeception" model, running control planes as pods in a seed cluster), and others, making direct adoption difficult.

### Challenges of Integrating Fully Managed Services with Cluster API

Despite the recent improvements, integrating fully managed Kubernetes services with Cluster API presents inherent challenges:

1.  **Opinionated Structure:** Cluster API's `Cluster` / `ControlPlane` / `MachineDeployment` / `MachinePool` structure is opinionated and doesn't always align naturally with how fully managed services architect their offerings. In particular, the separation between [`ControlPlane`](https://cluster-api.sigs.k8s.io/developer/providers/contracts/control-plane) and [`InfraCluster`](https://cluster-api.sigs.k8s.io/developer/providers/contracts/infra-cluster) objects can be difficult to map cleanly. Fully managed services often abstract away the details of how control planes are provisioned and operated, making the distinction between these two components less meaningful or even redundant in those contexts.
2.  **Provider-Specific CRDs:** While CAPI provides a common `Cluster` resource, the crucial `ControlPlane` and infrastructure-specific resources (e.g., `AWSManagedControlPlane`, `GCPManagedCluster`, `AzureManagedControlPlane`) are entirely different for each provider. This means you cannot simply swap `provider: foo` with `provider: bar` in a CAPI manifest and expect it to work. Users still need to understand the unique CRDs and capabilities of each CAPI provider.
3.  **Limited Unification:** CAPI unifies the *act of requesting a cluster*, but not the resulting cluster's features, available Kubernetes versions, release cycles, supported operating systems, specific add-ons, or nuanced operational behaviors like credential rotation procedures and their side effects.
4.  **Experimental or Limited Support:** For some managed services, CAPI provider support is still experimental or limited. For example:
    *   The CAPI provider for AKS notes that the `AzureManagedClusterTemplate` is "basically a no-op," with most configuration in `AzureManagedControlPlaneTemplate` ([CAPZ Docs](https://capz.sigs.k8s.io/topics/clusterclass.md)).
    *   The CAPI provider for GKE states: "Provisioning managed clusters (GKE) is an experimental feature..." and is disabled by default ([CAPG Docs](https://cluster-api-gcp.sigs.k8s.io/managed/)).

### Gardener's Perspective on a Cluster API Provider

Given that Gardener already offers a robust, Kubernetes-native API for homogenous multi-cloud cluster management, a Cluster API provider for Gardener could still act as a bridge for users transitioning from CAPI to Gardener, enabling them to gradually adopt Gardener's capabilities while maintaining compatibility with existing CAPI workflows.

*   **For users exclusively using Gardener:** Wrapping Gardener's comprehensive API within CAPI's structure offers limited additional value other than maintaining compatibility with existing CAPI workflows, as Gardener's native API is already declarative and Kubernetes-centric, meaning any tool or language binding that can handle Kubernetes CRDs will work with Gardener.
*   **For users managing diverse clusters (e.g., ACK, AKS, EKS, GKE, Gardener, `kubeadm`) via CAPI:** Cluster API offers a unified interface for initiating cluster provisioning across diverse managed services, but it does not harmonize the differences in provider-specific CRDs, capabilities, or operational behaviors. This can limit its ability to leverage the unique strengths of each service. However, we understand that a CAPI provider for Gardener could open doors for users familiar with CAPI's workflows. It would allow them to explore Gardener's enterprise-grade, homogeneous cluster management while maintaining compatibility with existing CAPI workflows, fostering a smoother transition to Gardener.

The mapping from the Gardener API to a potential Cluster API provider for Gardener would be mostly syntactic. However, the fundamental value proposition of Gardener — providing homogeneous Kubernetes clusters across all supported infrastructures — extends beyond what Cluster API currently aims to achieve.

We follow Cluster API's development with great interest and remain active members of the community.