# Image Vector

The Gardenlet is deploying several different container images into the seed and the shoot clusters.
The image repositories and tags are defined in a [central image vector file](../../charts/images.yaml).
Obviously, the image versions defined there must fit together with the deployment manifests (e.g., some command-line flags do only exist in certain versions).

## Example

```yaml
images:
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: gcr.io/google_containers/pause-amd64
  tag: "3.0"
  version: 1.15.x
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: gcr.io/google_containers/pause-amd64
  tag: "3.1"
  version: ">= 1.16"
...
```

That means that the Gardenlet will use the `pause-container` in with tag `3.0` for all seed/shoot clusters with Kubernetes version `1.15.x`, and tag `3.1` for all clusters with Kubernetes `>= 1.16`.

## Overwrite image vector

In some environment it is not possible to use these "pre-defined" images that come with a Gardener release.
A prominent example for that is Alicloud in China which does not allow access to Google's GCR.
In these cases you might want to overwrite certain images, e.g., point the `pause-container` to a different registry.

:warning: If you specify an image that does not fit to the resource manifest then the seed/shoot reconciliation might fail.

In order to overwrite the images you must provide a similar file to Gardenlet:

```yaml
images:
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: my-custom-image-registry/pause-amd64
  tag: "3.0"
  version: 1.15.x
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: my-custom-image-registry/pause-amd64
  tag: "3.1"
  version: ">= 1.16"
...
```

During deployment of the gardenlet create a `ConfigMap` containing the above content and mount it as a volume into the gardenlet pod.
Next, specify the environment variable `IMAGEVECTOR_OVERWRITE` whose value must be the path to the file you just mounted:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gardenlet-images-overwrite
  namespace: garden
data:
  images_overwrite.yaml: |
    images:
    - ...
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gardenlet
  namespace: garden
spec:
  template:
    ...
    spec:
      containers:
      - name: gardenlet
        env:
        - name: IMAGEVECTOR_OVERWRITE
          value: /charts-overwrite/images_overwrite.yaml
        volumeMounts:
        - name: gardenlet-images-overwrite
          mountPath: /charts-overwrite
        ...
      volumes:
      - name: gardenlet-images-overwrite
        configMap:
          name: gardenlet-images-overwrite
  ...
```

## Image vectors for dependent components

The gardenlet is deploying a lot of different components that might deploy other images themselves.
These components might use an image vector as well.
Operators might want to customize the image locations for these transitive images as well, hence, they might need to specify an image vector overwrite for the components directly deployed by Gardener.

It is possible to specify the `IMAGEVECTOR_OVERWRITE_COMPONENTS` environment variable to the gardenlet that points to a file with the following content:

```yaml
components:
- name: etcd-druid
  imageVectorOverwrite: |
    images:
    - name: etcd
      tag: v1.2.3
      repository: etcd/etcd
...
``` 

The gardenlet will, if supported by the directly deployed component (`etcd-druid` in this example), inject the given `imageVectorOverwrite` into the `Deployment` manifest.
The respective component is responsible for using the overwritten images instead of its defaults.
