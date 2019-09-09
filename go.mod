module github.com/gardener/gardener

go 1.12

require (
	cloud.google.com/go v0.43.0
	github.com/Azure/azure-storage-blob-go v0.7.0
	github.com/Masterminds/semver v1.4.2
	github.com/Masterminds/sprig v2.20.0+incompatible // indirect
	github.com/aliyun/aliyun-oss-go-sdk v2.0.1+incompatible
	github.com/aws/aws-sdk-go v1.21.10
	github.com/elazarl/goproxy v0.0.0-20190711103511-473e67f1d7d2 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190711103511-473e67f1d7d2 // indirect
	github.com/gardener/controller-manager-library v0.0.0-20190715130150-315e86b01963 // indirect
	github.com/gardener/etcd-backup-restore v0.0.0-20190807103447-4c8bc2972b60
	github.com/gardener/external-dns-management v0.0.0-20190722114702-f6b12f6e4b43 // 0.6.1
	github.com/gardener/gardener-extensions v0.0.0-20190906102350-7507733d43d6 // 0.12.0
	github.com/gardener/gardener-resource-manager v0.0.0-20190828115855-7ceeb3021993
	github.com/gardener/machine-controller-manager v0.0.0-20190606071036-119056ee3fdd
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.19.2
	github.com/golang/mock v1.3.1
	github.com/googleapis/gnostic v0.3.0
	github.com/gophercloud/gophercloud v0.3.0
	github.com/gophercloud/utils v0.0.0-20190527093828-25f1b77b8c03
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/json-iterator/go v1.1.6
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pierrec/lz4 v2.0.5+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/common v0.6.0
	github.com/prometheus/procfs v0.0.3 // indirect
	github.com/robfig/cron v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	google.golang.org/api v0.7.0
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
	k8s.io/kube-openapi v0.0.0-20190722073852-5e22f3d471e6
	k8s.io/metrics v0.0.0-20190816224245-c61a0d549e17
	k8s.io/utils v0.0.0-20190712204705-3dccf664f023
	sigs.k8s.io/controller-runtime v0.2.0-beta.2
	sigs.k8s.io/structured-merge-diff v0.0.0-20190820212518-960c3cc04183 // indirect
)

replace (
	git.apache.org/thrift.git => github.com/apache/thrift v0.12.0
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	golang.org/x/net => golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc // needed due to CVE CVE-2019-9512 and CVE-2019-9514 https://github.com/golang/go/issues/33606
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
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.2.0-beta.2
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v0.0.0-20190302045857-e85c7b244fd2
)
