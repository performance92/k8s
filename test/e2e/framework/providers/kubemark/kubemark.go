/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubemark

import (
	"flag"
	"fmt"

	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubemark"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/gomega"
)

var (
	kubemarkExternalKubeConfig = flag.String(fmt.Sprintf("%s-%s", "kubemark-external", clientcmd.RecommendedConfigPathFlag), "", "Path to kubeconfig containing embedded authinfo for external cluster.")
)

func init() {
	framework.RegisterProvider("kubemark", NewProvider)
}

func NewProvider() (framework.ProviderInterface, error) {
	// Actual initialization happens when the e2e framework gets constructed.
	return &Provider{}, nil
}

type Provider struct {
	framework.NullProvider
	controller   *kubemark.KubemarkController
	closeChannel chan struct{}
}

func (p *Provider) ResizeGroup(group string, size int32) error {
	return p.controller.SetNodeGroupSize(group, int(size))
}

func (p *Provider) GetGroupNodes(group string) ([]string, error) {
	return p.controller.GetNodeNamesForNodeGroup(group)
}

func (p *Provider) FrameworkBeforeEach(f *framework.Framework) {
	if *kubemarkExternalKubeConfig != "" && p.controller == nil {
		externalConfig, err := clientcmd.BuildConfigFromFlags("", *kubemarkExternalKubeConfig)
		externalConfig.QPS = f.Options.ClientQPS
		externalConfig.Burst = f.Options.ClientBurst
		Expect(err).NotTo(framework.HaveOccurredAt())
		externalClient, err := clientset.NewForConfig(externalConfig)
		Expect(err).NotTo(framework.HaveOccurredAt())
		f.KubemarkExternalClusterClientSet = externalClient
		p.closeChannel = make(chan struct{})
		externalInformerFactory := informers.NewSharedInformerFactory(externalClient, 0)
		kubemarkInformerFactory := informers.NewSharedInformerFactory(f.ClientSet, 0)
		kubemarkNodeInformer := kubemarkInformerFactory.Core().V1().Nodes()
		go kubemarkNodeInformer.Informer().Run(p.closeChannel)
		p.controller, err = kubemark.NewKubemarkController(externalClient, externalInformerFactory, f.ClientSet, kubemarkNodeInformer)
		Expect(err).NotTo(framework.HaveOccurredAt())
		externalInformerFactory.Start(p.closeChannel)
		Expect(p.controller.WaitForCacheSync(p.closeChannel)).To(BeTrue())
		go p.controller.Run(p.closeChannel)
	}
}

func (p *Provider) FrameworkAfterEach(f *framework.Framework) {
	if p.closeChannel != nil {
		close(p.closeChannel)
		p.controller = nil
		p.closeChannel = nil
	}
}

func (p *Provider) GroupSize(group string) (int, error) {
	return p.controller.GetNodeGroupSize(group)
}
