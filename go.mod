module github.com/gardener/gardener

go 1.13

require (
	github.com/Masterminds/semver v1.4.2
	github.com/Masterminds/sprig v2.20.0+incompatible // indirect
	github.com/ahmetb/gen-crd-api-reference-docs v0.1.5
	github.com/coreos/etcd v3.3.13+incompatible // indirect
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/elazarl/goproxy v0.0.0-20190711103511-473e67f1d7d2 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190711103511-473e67f1d7d2 // indirect
	github.com/frankban/quicktest v1.5.0 // indirect
	github.com/gardener/controller-manager-library v0.0.0-20190830161011-2626c078acef // indirect
	github.com/gardener/external-dns-management v0.0.0-20190220100540-b4bbb5832a03
	github.com/gardener/gardener-extensions v0.0.0-20190906160200-5c329d46ae81
	github.com/gardener/gardener-resource-manager v0.0.0-20190828115855-7ceeb3021993
	github.com/gardener/hvpa-controller v0.0.0-20190924063424-ef5c3668949d
	github.com/gardener/machine-controller-manager v0.0.0-20190606071036-119056ee3fdd
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.19.2
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/mock v1.3.1
	github.com/googleapis/gnostic v0.3.0
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.9.5 // indirect
	github.com/hashicorp/go-multierror v0.0.0-20180717150148-3d5d8f294aa0
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/json-iterator/go v1.1.6
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pierrec/lz4 v2.3.0+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/common v0.6.0
	github.com/robfig/cron v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/net v0.0.0-20190813141303-74dc4d7220e7 // indirect
	golang.org/x/sys v0.0.0-20190804053845-51ab0e2deafa // indirect
	golang.org/x/tools v0.0.0-20190628153133-6cdbf07be9d0 // indirect
	google.golang.org/genproto v0.0.0-20190801165951-fa694d86fc64 // indirect
	google.golang.org/grpc v1.22.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apiextensions-apiserver v0.0.0-20190409022649-727a075fdec8
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/apiserver v0.0.0-20190313205120-8b27c41bdbb1
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cluster-bootstrap v0.0.0-20190314002537-50662da99b70
	k8s.io/code-generator v0.0.0-20190713022532-93d7507fc8ff
	k8s.io/component-base v0.0.0-20190314000054-4a91899592f4
	k8s.io/helm v2.14.2+incompatible
	k8s.io/klog v0.3.3
	k8s.io/kube-aggregator v0.0.0-20190314000639-da8327669ac5
	k8s.io/kube-openapi v0.0.0-20190320154901-5e45bb682580
	k8s.io/metrics v0.0.0-20190816224245-c61a0d549e17
	k8s.io/utils v0.0.0-20190607212802-c55fbcfc754a
	sigs.k8s.io/controller-runtime v0.2.0-beta.4
)

replace (
	github.com/gardener/external-dns-management => github.com/gardener/external-dns-management v0.0.0-20190220100540-b4bbb5832a03 // 0.1.0
	github.com/gardener/gardener-extensions => github.com/gardener/gardener-extensions v0.0.0-20190906160200-5c329d46ae81
	github.com/gardener/gardener-resource-manager => github.com/gardener/gardener-resource-manager v0.0.0-20190828115855-7ceeb3021993
	github.com/gardener/machine-controller-manager => github.com/gardener/machine-controller-manager v0.0.0-20190228095106-36a42c48af0a // 0.14.0
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	k8s.io/api => k8s.io/api v0.0.0-20190313235455-40a48860b5ab //kubernetes-1.14.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190315093550-53c4693659ed // kubernetes-1.14.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190313205120-d7deff9243b1 // kubernetes-1.14.0
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190313205120-8b27c41bdbb1 // kubernetes-1.14.0
	k8s.io/client-go => k8s.io/client-go v11.0.0+incompatible // kubernetes-1.14.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190314002537-50662da99b70 // kubernetes-1.14.0
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190311093542-50b561225d70 // kubernetes-1.14.4
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190314000054-4a91899592f4 // kubernetes-1.14.0
	k8s.io/helm => k8s.io/helm v2.13.1+incompatible
	k8s.io/klog => k8s.io/klog v0.1.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190314000639-da8327669ac5 // kubernetes-1.14.0
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190320154901-5e45bb682580
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.2.0-beta.4
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v0.0.0-20190302045857-e85c7b244fd2
)
