# Projects

The Gardener API server supports a cluster-scoped `Project` resource which is used to group usage of Gardener.
For example, each development team has its own project to manage its own shoot clusters.

Each `Project` is backed by a Kubernetes `Namespace` that contains the actual related Kubernetes resources like `Secret`s or `Shoot`s.

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

The `.spec.namespace` field is optional and will be initialized if it's unset.
The name of the resulting namespace will be generated and look like `garden-dev-5anj3`, i.e., it has a random suffix.
It's also possible to adopt existing namespaces by labeling them `gardener.cloud/role=project` and `project.gardener.cloud/name=dev` beforehand (otherwise, they cannot be adopted). 

When deleting a Project resource, the corresponding namespace is also deleted. 
To keep a namespace after project deletion, an administrator/operator (not Project members!) can label the project-namespace with `namespace.gardener.cloud/keep-after-project-deletion`.

The `spec.description` and `.spec.purpose` fields can be used to describe to fellow team members and Gardener operators what this project is used for.

Each project has one dedicated owner, configured in `.spec.owner` using the `rbac.authorization.k8s.io/v1.Subject` type.
The owner is the main contact person for Gardener operators.
Please note that the `.spec.owner` field is deprecated and will be removed in future API versions in favor of the `owner` role, see below.

The list of members (again a list in `.spec.members[]` using the `rbac.authorization.k8s.io/v1.Subject` type) contains all the people that are associated with the project in any way.
Each project member must have at least one role (currently described in `.spec.members[].role`, additional roles can be added to `.spec.members[].roles[]`). The following roles exist:

* `admin`: This allows to fully manage resources inside the project (e.g., secrets, shoots, configmaps, and similar).
* `uam`: This allows to add/modify/remove human users or groups to/from the project member list. Technical users (service accounts) can be managed by all admins.
* `viewer`: This allows to read all resources inside the project except secrets.
* `owner`: This combines the `admin` and `uam` roles.
* Extension roles (prefixed with `extension:`): Please refer to [this document](../extensions/project-roles.md).

The [project controller](../concepts/controller-manager.md#project-controller) inside the Gardener Controller Manager is managing RBAC resources that grant the described privileges to the respective members.

There are two central `ClusterRole`s `gardener.cloud:system:project-member` and `gardener.cloud:system:project-viewer` that grant the permissions for namespaced resources (e.g., `Secret`s, `Shoot`s, etc.).
Via referring `RoleBinding`s created in the respective namespace the project members get bound to these `ClusterRole`s and, thus, the needed permissions.
There are also project-specific `ClusterRole`s granting the permissions for cluster-scoped resources, e.g. the `Namespace` or `Project` itself.  
For each role, the following `ClusterRole`s, `ClusterRoleBinding`s, and `RoleBinding`s are created:

| Role | `ClusterRole` | `ClusterRoleBinding` | `RoleBinding` |
| ---- | ----------- | ------------------ | ----------- |
| `admin` | `gardener.cloud:system:project-member:<projectName>` | `gardener.cloud:system:project-member:<projectName>` | `gardener.cloud:system:project-member` |
| `uam`   | `gardener.cloud:system:project-uam:<projectName>` | `gardener.cloud:system:project-uam:<projectName>` | |
| `viewer` | `gardener.cloud:system:project-viewer:<projectName>` | `gardener.cloud:system:project-viewer:<projectName>` | `gardener.cloud:system:project-viewer` |
| `owner` | `gardener.cloud:system:project:<projectName>` | `gardener.cloud:system:project:<projectName>` |  |
| `extension:*` | `gardener.cloud:extension:project:<projectName>:<extensionRoleName>` | | `gardener.cloud:extension:project:<projectName>:<extensionRoleName>` |

## User Access Management

For `Project`s created before Gardener v1.8 all admins were allowed to manage other members.
Beginning with v1.8 the new `uam` role is being introduced.
It is backed by the `manage-members` custom RBAC verb which allows to add/modify/remove human users or groups to/from the project member list.
Human users are subjects with `kind=User` and `name!=system:serviceaccount:*`, and groups are subjects with `kind=Group`.
The management of service account subjects (`kind=ServiecAccount` or `name=system:serviceaccount:*`) is not controlled via the `uam` custom verb but with the standard `update`/`patch` verbs for projects.

All newly created projects will only bind the owner to the `uam` role.
The owner can still grant the `uam` role to other members if desired.
For projects created before Gardener v1.8 the Gardener Controller Manager will migrate all projects to also assign the `uam` role to all `admin` members (to not break existing use-cases). The corresponding migration logic is present in Gardener Controller Manager from v1.8 to v1.13.
The project owner can gradually remove these roles if desired. 
