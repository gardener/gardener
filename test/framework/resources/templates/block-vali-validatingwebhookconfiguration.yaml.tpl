---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: block-vali-updates
webhooks:
- admissionReviewVersions:
  - v1beta1
  clientConfig:
    caBundle: {{ .CABundle }}
    service:
      name: unreal-service
      namespace: unreal-namespace
  failurePolicy: Fail
  matchPolicy: Exact
  name: block.vali.seed.admission.core.gardener.cloud
  namespaceSelector:
    matchLabels:
      {{ .NamespaceLabelKey }}: {{ .NamespaceLabelValue }}
  objectSelector:
    matchExpressions:
    - key: app
      operator: In
      values:
      - vali
    - key: role
      operator: In
      values:
      - logging
  rules:
  - apiGroups:
    - "apps"
    apiVersions:
    - v1
    operations:
    - UPDATE
    resources:
    - statefulsets
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
