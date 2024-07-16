---
description: Project operations and roles. Four-Eyes-Principle for resource deletion
---

# Projects

The Gardener API server supports a cluster-scoped `Project` resource that is backed by a Kubernetes `Namespace` containing the related Kubernetes resources, like `Secret`s or `Shoot`s. It is used for data isolation between individual Gardener consumers. For example, each development team has its own project to manage its own shoot clusters. 

**Example resource:**

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Project
metadata:
  name: dev
spec:
  namespace: garden-dev
  description: "This is my first project"
  purpose: "Experimenting with Gardener"
  owner:
    apiGroup: rbac.authorization.k8s.io
    kind: User
    name: john.doe@example.com
  members:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: alice.doe@example.com
    role: admin
  # roles:
  # - viewer 
  # - uam
  # - serviceaccountmanager
  # - extension:foo
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: bob.doe@example.com
    role: viewer
# tolerations:
#   defaults:
#   - key: <some-key>
#   whitelist:
#   - key: <some-key>
```

The `.spec.namespace` field is optional and is initialized if unset.
The name of the resulting namespace will be determined based on the `Project` name and UID, e.g., `garden-dev-5aef3`.
It's also possible to adopt existing namespaces by labeling them `gardener.cloud/role=project` and `project.gardener.cloud/name=dev` beforehand (otherwise, they cannot be adopted).

When deleting a Project resource, the corresponding namespace is also deleted.
To keep a namespace after project deletion, an administrator/operator (not Project members!) can annotate the project-namespace with `namespace.gardener.cloud/keep-after-project-deletion`.

The `spec.description` and `.spec.purpose` fields can be used to describe to fellow team members and Gardener operators what this project is used for.

Each project has one dedicated owner, configured in `.spec.owner` using the `rbac.authorization.k8s.io/v1.Subject` type.
The owner is the main contact person for Gardener operators.
Please note that the `.spec.owner` field is deprecated and will be removed in future API versions in favor of the `owner` role, see below.

The list of members (`.spec.members[]` using the `rbac.authorization.k8s.io/v1.Subject` type) contains all the people that are associated with the project in any way.
Each project member must have at least one role (currently described in `.spec.members[].role`, additional roles can be added to `.spec.members[].roles[]`). The following roles exist:

* `admin`: This allows to fully manage resources inside the project (e.g., secrets, shoots, configmaps, and similar). Mind that the `admin` role has read only access to service accounts.
* `serviceaccountmanager`: This allows to fully manage service accounts inside the project namespace and request tokens for them. The permissions of the created service accounts are instead managed by the `admin` role. Please refer to [Service Account Manager](./service-account-manager.md).
* `uam`: This allows to add/modify/remove human users or groups to/from the project member list.
* `viewer`: This allows to read all resources inside the project except secrets.
* `owner`: This combines the `admin`, `uam`, and `serviceaccountmanager` roles.
* Extension roles (prefixed with `extension:`): Please refer to [Extending Project Roles](../extensions/project-roles.md).

The [project controller](../concepts/controller-manager.md#project-controller) inside the Gardener Controller Manager is managing RBAC resources that grant the described privileges to the respective members.

There are three central `ClusterRole`s `gardener.cloud:system:project-member`, `gardener.cloud:system:project-viewer`, and `gardener.cloud:system:project-serviceaccountmanager` that grant the permissions for namespaced resources (e.g., `Secret`s, `Shoot`s, `ServiceAccount`s).
Via referring `RoleBinding`s created in the respective namespace the project members get bound to these `ClusterRole`s and, thus, the needed permissions.
There are also project-specific `ClusterRole`s granting the permissions for cluster-scoped resources, e.g., the `Namespace` or `Project` itself.  
For each role, the following `ClusterRole`s, `ClusterRoleBinding`s, and `RoleBinding`s are created:

| Role | `ClusterRole` | `ClusterRoleBinding` | `RoleBinding` |
| ---- | ----------- | ------------------ | ----------- |
| `admin` | `gardener.cloud:system:project-member:<projectName>` | `gardener.cloud:system:project-member:<projectName>` | `gardener.cloud:system:project-member` |
| `serviceaccountmanager` | | | `gardener.cloud:system:project-serviceaccountmanager` |
| `uam`   | `gardener.cloud:system:project-uam:<projectName>` | `gardener.cloud:system:project-uam:<projectName>` | |
| `viewer` | `gardener.cloud:system:project-viewer:<projectName>` | `gardener.cloud:system:project-viewer:<projectName>` | `gardener.cloud:system:project-viewer` |
| `owner` | `gardener.cloud:system:project:<projectName>` | `gardener.cloud:system:project:<projectName>` |  |
| `extension:*` | `gardener.cloud:extension:project:<projectName>:<extensionRoleName>` | | `gardener.cloud:extension:project:<projectName>:<extensionRoleName>` |

## User Access Management

For `Project`s created before Gardener v1.8, all admins were allowed to manage other members.
Beginning with v1.8, the new `uam` role is being introduced.
It is backed by the `manage-members` custom RBAC verb which allows to add/modify/remove human users or groups to/from the project member list.
Human users are subjects with `kind=User` and `name!=system:serviceaccount:*`, and groups are subjects with `kind=Group`.
The management of service account subjects (`kind=ServiceAccount` or `name=system:serviceaccount:*`) is not controlled via the `uam` custom verb but with the standard `update`/`patch` verbs for projects.

All newly created projects will only bind the owner to the `uam` role.
The owner can still grant the `uam` role to other members if desired.
For projects created before Gardener v1.8, the Gardener Controller Manager will migrate all projects to also assign the `uam` role to all `admin` members (to not break existing use-cases). The corresponding migration logic is present in Gardener Controller Manager from v1.8 to v1.13.
The project owner can gradually remove these roles if desired.

## Stale Projects

When a project is not actively used for some period of time, it is marked as "stale". This is done by a controller called ["Stale Projects Reconciler"](../concepts/controller-manager.md#stale-projects-reconciler). Once the project is marked as stale, there is a time frame in which if not used it will be deleted by that controller.

## Four-Eyes-Principle For Resource Deletion

In order to delete a `Shoot`, the deletion must be confirmed upfront with the `confirmation.gardener.cloud/deletion=true` annotation.
Without this annotation being set, `gardener-apiserver` denies any DELETE request.
Still, users sometimes accidentally shot themselves in the foot, meaning that they accidentally deleted a `Shoot` despite the confirmation requirement.

To prevent that (or make it harder, at least), the `Project` can be configured to apply the dual approval concept for `Shoot` deletion.
This means that the subject confirming the deletion must not be the same as the subject sending the DELETE request.

Example:

```yaml
spec:
  dualApprovalForDeletion:
  - resource: shoots
    selector:
      matchLabels: {}
    includeServiceAccounts: true
```

> [!NOTE]
> As of today, `core.gardener.cloud/v1beta1.Shoot` is the only resource for which this concept is implemented.

As usual, `.spec.dualApprovalForDeletion[].selector.matchLabels={}` matches all resources, `.spec.dualApprovalForDeletion[].selector.matchLabels=null` matches none at all.
It can also be decided to specify an individual label selector if this concept shall only apply to a subset of the `Shoot`s in the project (e.g., CI/development clusters shall be excluded).

The `includeServiceAccounts` (default: `true`) controls whether the concept also applies when the `Shoot` deletion confirmation and actual deletion is triggered via `ServiceAccount`s.
This is to prevent that CI jobs have to follow this concept as well, adding additional complexity/overhead.
Alternatively, you could also use two `ServiceAccount`s, one for confirming the deletion, and another one for actually sending the DELETE request, if desired.

> [!IMPORTANT]
> Project members can still change the labels of `Shoot`s (or the selector itself) to circumvent the dual approval concept.
> This concern is intentionally excluded/ignored for now since the principle is not a "security feature" but shall just help preventing *accidental* deletion.
