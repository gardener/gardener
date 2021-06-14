module github.com/gardener/gardener

go 1.16

require (
	github.com/Masterminds/semver v1.5.0
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/ahmetb/gen-crd-api-reference-docs v0.2.0
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/envoyproxy/go-control-plane v0.9.7-0.20200730005029-803dd64f0468
	github.com/gardener/component-spec/bindings-go v0.0.33
	github.com/gardener/etcd-druid v0.5.0
	github.com/gardener/external-dns-management v0.7.18
	github.com/gardener/gardener-resource-manager v0.18.0
	github.com/gardener/hvpa-controller v0.3.1
	github.com/gardener/landscaper/apis v0.7.0
	github.com/gardener/machine-controller-manager v0.33.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.3.0
	github.com/go-openapi/spec v0.19.3
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/googleapis/gnostic v0.5.1
	github.com/hashicorp/go-multierror v1.1.0
	github.com/huandu/xstrings v1.3.2
	github.com/json-iterator/go v1.1.10
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.5
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/robfig/cron v1.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	github.com/texttheater/golang-levenshtein v0.0.0-20191208221605-eb6844b05fc6
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	gomodules.xyz/jsonpatch/v2 v2.1.0
	gonum.org/v1/gonum v0.8.2
	gopkg.in/yaml.v2 v2.3.0
	istio.io/api v0.0.0-20201123152548-197f11e4ea09
	istio.io/client-go v1.8.1
	k8s.io/api v0.20.7
	k8s.io/apiextensions-apiserver v0.20.7
	k8s.io/apimachinery v0.20.7
	k8s.io/apiserver v0.20.7
	k8s.io/autoscaler v0.0.0-20190805135949-100e91ba756e
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cluster-bootstrap v0.20.7
	k8s.io/code-generator v0.20.7
	k8s.io/component-base v0.20.7
	k8s.io/gengo v0.0.0-20201113003025-83324d819ded
	k8s.io/helm v2.16.1+incompatible
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.4.0
	k8s.io/kube-aggregator v0.20.7
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd // keep this value in sync with k8s.io/apiserver
	k8s.io/kubelet v0.20.7
	k8s.io/metrics v0.20.7
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/emicklei/go-restful => github.com/emicklei/go-restful v2.9.5+incompatible // keep this value in sync with k8s.io/apiserver
	github.com/envoyproxy/go-control-plane => github.com/envoyproxy/go-control-plane v0.9.4
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.7.1 // keep this value in sync with sigs.k8s.io/controller-runtime
	google.golang.org/grpc => google.golang.org/grpc v1.27.0 // keep this value in sync with k8s.io/apiserver
	k8s.io/api => k8s.io/api v0.20.7
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.7
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.7
	k8s.io/apiserver => k8s.io/apiserver v0.20.7
	k8s.io/client-go => k8s.io/client-go v0.20.7
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.20.7
	k8s.io/code-generator => k8s.io/code-generator v0.20.7
	k8s.io/component-base => k8s.io/component-base v0.20.7
	k8s.io/helm => k8s.io/helm v2.13.1+incompatible
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.20.7
	sigs.k8s.io/controller-runtime => github.com/gardener/controller-runtime v0.8.3-gardener.1
)
