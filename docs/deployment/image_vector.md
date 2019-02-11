# Image Vector

Gardener is deploying several different container images into the seed and the shoot clusters.
The image repositories and tags are defined in a [central image vector file](`../../charts/images.yaml`).
Obviously, the image versions defined there must fit together with the deployment manifests (e.g., some command-line flags do only exist in certain versions).

## Example

```yaml
images:
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: gcr.io/google_containers/pause-amd64
  tag: "3.0"
  version: 1.11.x
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: gcr.io/google_containers/pause-amd64
  tag: "3.1"
  version: ">= 1.12"
...
```

That means that Gardener will use the `pause-container` in with tag `3.0` for all seed/shoot clusters with Kubernetes version `1.11.x`, and tag `3.1` for all clusters with Kubernetes `>= 1.12`.

## Overwrite image vector

In some environment it is not possible to use these "pre-defined" images that come with a Gardener release.
A prominent example for that is Alicloud in China which does not allow access to Google's GCR.
In these cases you might want to overwrite certain images, e.g., point the `pause-container` to a different registry.

:warning: If you specify an image that does not fit to the resource manifest then the seed/shoot reconciliation might fail.

In order to overwrite the images you must provide a similar file to Gardener:

```yaml
images:
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: my-custom-image-registry/pause-amd64
  tag: "3.0"
  version: 1.11.x
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: my-custom-image-registry/pause-amd64
  tag: "3.1"
  version: ">= 1.12"
...
```

During deployment of the gardener-controller-manager create a `ConfigMap` containing the above content and mount it as a volume into the gardener-controller-manager pod.
Next, specify the environment variable `IMAGEVECTOR_OVERWRITE` whose value must be the path to the file you just mounted:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gardener-controller-manager-images-overwrite
  namespace: garden
data:
  images_overwrite.yaml: |
    images:
    - ...
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gardener-controller-manager
  namespace: garden
spec:
  template:
    ...
    spec:
      containers:
      - name: gardener-controller-manager
        env:
        - name: IMAGEVECTOR_OVERWRITE
          value: /charts/images_overwrite.yaml
        volumeMounts:
        - name: gardener-controller-manager-images-overwrite
          mountPath: /charts/images_overwrite.yaml
        ...
      volumes:
      - name: gardener-controller-manager-images-overwrite
        configMap:
          name: gardener-controller-manager-images-overwrite
  ...
```
