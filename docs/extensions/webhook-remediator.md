# Webhook remediator

## Explanation
As a Gardener deployment grows to host hundreds (if not thousands) of clusters, it becomes hard for the operators to effitially answer all support requests from cluster's users. One issue that has been recuring is related to validating/mutating webhooks : Since kubernetes provides webhook resource to validate / modify resources created by cluster's users, it is possible that a wrongly configured webhook interferes with Gardener's ability modify the cluster as required.

To correct this problem, the webhook remediator has been created. This extension simply checks whether a customer's webhook matches a set of rules, described [here](https://github.com/gardener/gardener/blob/master/pkg/operation/botanist/matchers/matcher.go#L66-L180). If at least one of the rule matches, it will change 2 status constraints (`.status.constraints[].type: HibernationPossible` and `.status.constraints[].type: MaintenancePreconditionsSatisfied`) to `False` in the shoot yaml.

# How to correct this error
In most cases, you simply need to exclude the kube-system namespace from your webhook.

Example
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
webhooks:
  - name: my-webhook.example.com
    namespaceSelector:
      matchExpressions:
      - key: gardener.cloud/purpose
        operator: NotIn
        values:
          - kube-system
    rules:
      - operations: ["*"]
        apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods"]
        scope: "Namespaced"
```

However, some other resources might still trigger the remediator, namely:
- endpoints
- nodes
- podsecuritypolicies
- clusterroles
- clusterrolebindings
- customresourcedefinitions
- apiservices
- certificatesigningrequests
- priorityclasses

In these cases, please make sure that your `rules` don't overlap with one of those resources (and their subresources).

You can also find help from the [kubernetes documentation](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#best-practices-and-warnings)
