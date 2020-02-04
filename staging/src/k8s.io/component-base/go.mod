// This is a generated file. Do not edit directly.

module k8s.io/component-base

go 1.13

require (
	github.com/blang/semver v3.5.0+incompatible
	github.com/google/go-cmp v0.3.1
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.4.1
	github.com/prometheus/procfs v0.0.2
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20191217005138-9e5e9d854fcc
)

replace (
	github.com/jstemmer/go-junit-report => github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024
	go.opencensus.io => go.opencensus.io v0.21.0
	golang.org/x/exp => golang.org/x/exp v0.0.0-20190312203227-4b39c73a6495
	golang.org/x/lint => golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/net => golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys => golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a // pinned to release-branch.go1.13
	golang.org/x/tools => golang.org/x/tools v0.0.0-20190821162956-65e3620a7ae7 // pinned to release-branch.go1.13
	google.golang.org/appengine => google.golang.org/appengine v1.5.0
	honnef.co/go/tools => honnef.co/go/tools v0.0.1-2019.2.2
	k8s.io/api => ../api
	k8s.io/apimachinery => ../apimachinery
	k8s.io/client-go => ../client-go
	k8s.io/component-base => ../component-base
)
