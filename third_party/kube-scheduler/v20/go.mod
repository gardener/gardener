module k8s.io/kube-scheduler/v20

go 1.15

require (
	k8s.io/api v0.18.10
	k8s.io/apimachinery v0.18.10
	k8s.io/component-base v0.18.10
	k8s.io/kube-scheduler v0.18.10
	sigs.k8s.io/yaml v1.2.0
)

replace k8s.io/kube-scheduler => ../
