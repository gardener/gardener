module github.com/gardener/gardener

go 1.15

require (
	github.com/Masterminds/semver v1.5.0
	github.com/ahmetb/gen-crd-api-reference-docs v0.2.0
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/emicklei/go-restful v2.9.6+incompatible
	github.com/gardener/etcd-druid v0.3.0
	github.com/gardener/external-dns-management v0.7.18
	github.com/gardener/gardener-resource-manager v0.18.0
	github.com/gardener/hvpa-controller v0.3.1
	github.com/gardener/machine-controller-manager v0.33.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.3
	github.com/gobuffalo/packr v1.30.1
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.4.4-0.20200731163441-8734ec565a4d
	github.com/googleapis/gnostic v0.3.1
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/huandu/xstrings v1.3.1
	github.com/json-iterator/go v1.1.10
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.8.0
	github.com/robfig/cron v1.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v0.0.6
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.1
	github.com/texttheater/golang-levenshtein v0.0.0-20191208221605-eb6844b05fc6
	go.uber.org/zap v1.13.0
	golang.org/x/crypto v0.0.0-20200220183623-bac4c82f6975
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	gomodules.xyz/jsonpatch/v2 v2.0.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.10
	k8s.io/apiextensions-apiserver v0.18.10
	k8s.io/apimachinery v0.18.10
	k8s.io/apiserver v0.18.10
	k8s.io/autoscaler v0.0.0-20190805135949-100e91ba756e
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cluster-bootstrap v0.18.10
	k8s.io/code-generator v0.18.10
	k8s.io/component-base v0.18.10
	k8s.io/gengo v0.0.0-20200413195148-3a45101e95ac
	k8s.io/helm v2.16.1+incompatible
	k8s.io/klog v1.0.0
	k8s.io/kube-aggregator v0.18.10
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29 // release-1.18
	k8s.io/kubelet v0.18.10
	k8s.io/metrics v0.18.10
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/controller-tools v0.3.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/emicklei/go-restful => github.com/emicklei/go-restful v2.9.5+incompatible // keep this value in sync with k8s.io/apiserver
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	k8s.io/api => k8s.io/api v0.18.10
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.10
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.10
	k8s.io/apiserver => k8s.io/apiserver v0.18.10
	k8s.io/client-go => k8s.io/client-go v0.18.10
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.10
	k8s.io/code-generator => k8s.io/code-generator v0.18.10
	k8s.io/component-base => k8s.io/component-base v0.18.10
	k8s.io/helm => k8s.io/helm v2.13.1+incompatible
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.10
	k8s.io/kube-openapi => github.com/gardener/kube-openapi v0.0.0-20200831104310-b5db060350c7
	sigs.k8s.io/controller-runtime => github.com/gardener/controller-runtime v0.6.3-gardener.1
	sigs.k8s.io/structured-merge-diff/v3 => sigs.k8s.io/structured-merge-diff/v3 v3.0.1-0.20201124164700-f5fd4ea1e4c9
)
