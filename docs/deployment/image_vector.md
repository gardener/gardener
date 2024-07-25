# Image Vector

The Gardener components are deploying several different container images into the garden, seed, and the shoot clusters.
The image repositories and tags are defined in a [central image vector file](../../imagevector/images.yaml).
Obviously, the image versions defined there must fit together with the deployment manifests (e.g., some command-line flags do only exist in certain versions).

## Example

```yaml
images:
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: registry.k8s.io/pause
  tag: "3.4"
  targetVersion: "1.20.x"
  architectures:
  - amd64
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  ref: registry.k8s.io/pause:3.5
  targetVersion: ">= 1.21"
  architectures:
  - arm64
...
```

That means that Gardener will use the `pause-container` with tag `3.4` for all clusters with Kubernetes version `1.20.x`, and the image with ref `registry.k8s.io/pause:3.5` for all clusters with Kubernetes `>= 1.21`.

> [!NOTE]
> As you can see, it is possible to provide the full image reference via the `ref` field.
> Another option is to use the `repository` and `tag` fields. `tag` may also be a digest only (starting with `sha256:...`), or it can contain both tag and digest (`v1.2.3@sha256:...`).

## Architectures

```yaml
images:
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: registry.k8s.io/pause
  tag: "3.5"
  architectures:
  - amd64
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  ref: registry.k8s.io/pause:3.5
  architectures:
  - arm64
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  ref: registry.k8s.io/pause:3.5
  architectures:
  - amd64
  - arm64
...
```

`architectures` is an optional field of image. It is a list of strings specifying CPU architecture of machines on which this image can be used. The valid options for the architectures field are as follows:
- `amd64` : This specifies that the image can run only on machines having CPU architecture `amd64`.
- `arm64` : This specifies that the image can run only on machines having CPU architecture `arm64`.

If an image doesn't specify any architectures, then by default it is considered to support both `amd64` and `arm64` architectures.

## Overwriting Image Vector

In some environments it is not possible to use these "pre-defined" images that come with a Gardener release.
A prominent example for that is Alicloud in China, which does not allow access to Google's GCR.
In these cases, you might want to overwrite certain images, e.g., point the `pause-container` to a different registry.

:warning: If you specify an image that does not fit to the resource manifest, then the reconciliations might fail.

In order to overwrite the images, you must provide a similar file to the Gardener component:

```yaml
images:
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  repository: my-custom-image-registry/pause
  tag: "3.4"
  version: "1.20.x"
- name: pause-container
  sourceRepository: github.com/kubernetes/kubernetes/blob/master/build/pause/Dockerfile
  ref: my-custom-image-registry/pause:3.5
  version: ">= 1.21"
...
```

> [!IMPORTANT]
> When the overwriting file contains `ref` for an image but the source file doesn't, then this invalidates both `repository` and `tag` of the source.
> When it contains `repository` for an image but the source file uses `ref`, then this invalidates `ref` of the source.

For `gardenlet`, you can create a `ConfigMap` containing the above content and mount it as a volume into the `gardenlet` pod.
Next, specify the environment variable `IMAGEVECTOR_OVERWRITE`, whose value must be the path to the file you just mounted.
The approach works similarly for `gardener-operator`.

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
    # ...
    spec:
      containers:
      - name: gardenlet
        env:
        - name: IMAGEVECTOR_OVERWRITE
          value: /charts-overwrite/images_overwrite.yaml
        volumeMounts:
        - name: gardenlet-images-overwrite
          mountPath: /charts-overwrite
        # ...
      volumes:
      - name: gardenlet-images-overwrite
        configMap:
          name: gardenlet-images-overwrite
  # ...
```

## Image Vectors for Dependent Components

Gardener is deploying a lot of different components that might deploy other images themselves.
These components might use an image vector as well.
Operators might want to customize the image locations for these transitive images as well, hence, they might need to specify an image vector overwrite for the components directly deployed by Gardener.

It is possible to specify the `IMAGEVECTOR_OVERWRITE_COMPONENTS` environment variable to Gardener that points to a file with the following content:

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

Gardener will, if supported by the directly deployed component (`etcd-druid` in this example), inject the given `imageVectorOverwrite` into the `Deployment` manifest.
The respective component is responsible for using the overwritten images instead of its defaults.
