# Extending Project Roles

The `Project` resource allows to specify a list of roles for every member (`.spec.members[*].roles`).
There are a few standard roles defined by Gardener itself.
Please consult [Projects](../usage/project/projects.md) for further information.

However, extension controllers running in the garden cluster may also create `CustomResourceDefinition`s that project members might be able to CRUD.
For this purpose, Gardener also allows to specify extension roles.

An extension role is prefixed with `extension:`, e.g.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Project
metadata:
  name: dev
spec:
  members:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: alice.doe@example.com
    role: admin
    roles:
    - owner
    - extension:foo
```

The project controller will, for every extension role, create a `ClusterRole` with name `gardener.cloud:extension:project:<projectName>:<roleName>`, i.e., for the above example: `gardener.cloud:extension:project:dev:foo`.
This `ClusterRole` aggregates other `ClusterRole`s that are labeled with `rbac.gardener.cloud/aggregate-to-extension-role=foo` which might be created by extension controllers.

An extension that might want to contribute to the core `admin` or `viewer` roles can use the labels `rbac.gardener.cloud/aggregate-to-project-member=true` or `rbac.gardener.cloud/aggregate-to-project-viewer=true`, respectively.

Please note that the names of the extension roles are restricted to 20 characters!

Moreover, the project controller will also create a corresponding `RoleBinding` with the same name in the project namespace.
It will automatically assign all members that are assigned to this extension role.
