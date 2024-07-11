---
title: Admission Configuration for the `PodSecurity` Admission Plugin
description: Adding custom configuration for the `PodSecurity` plugin in `.spec.kubernetes.kubeAPIServer.admissionPlugins`
---

## Admission Configuration for the `PodSecurity` Admission Plugin

If you wish to add your custom configuration for the `PodSecurity` plugin, you can do so in the Shoot spec under `.spec.kubernetes.kubeAPIServer.admissionPlugins` by adding:

```yaml
admissionPlugins:
- name: PodSecurity
  config:
    apiVersion: pod-security.admission.config.k8s.io/v1
    kind: PodSecurityConfiguration
    # Defaults applied when a mode label is not set.
    #
    # Level label values must be one of:
    # - "privileged" (default)
    # - "baseline"
    # - "restricted"
    #
    # Version label values must be one of:
    # - "latest" (default) 
    # - specific version like "v1.25"
    defaults:
      enforce: "privileged"
      enforce-version: "latest"
      audit: "privileged"
      audit-version: "latest"
      warn: "privileged"
      warn-version: "latest"
    exemptions:
      # Array of authenticated usernames to exempt.
      usernames: []
      # Array of runtime class names to exempt.
      runtimeClasses: []
      # Array of namespaces to exempt.
      namespaces: []
```

For proper functioning of Gardener, `kube-system` namespace will also be automatically added to the `exemptions.namespaces` list.
