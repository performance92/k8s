/*
Copyright 2017 The Kubernetes Authors.

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

// Note: the example only works with the code within the same release/branch.
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	tprv1 "k8s.io/client-go/examples/third-party-resources/apis/tpr/v1"
	exampleclient "k8s.io/client-go/examples/third-party-resources/client"
	examplecontroller "k8s.io/client-go/examples/third-party-resources/controller"
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "Path to a kube config. Only required if out-of-cluster.")
	flag.Parse()

	// Create the client config. Use kubeconfig if given, otherwise assume in-cluster.
	config, err := buildConfig(*kubeconfig)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// initialize third party resource if it does not exist
	err = exampleclient.CreateTPR(clientset)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		panic(err)
	}

	// make a new config for our extension's API group, using the first config as a baseline
	exampleClient, _, err := exampleclient.NewClient(config)
	if err != nil {
		panic(err)
	}

	// wait until TPR gets processed
	exampleclient.WaitForExampleResource(exampleClient)

	// start a controller on instances of our TPR
	controller := examplecontroller.ExampleController{
		ExampleClient: exampleClient,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	go controller.Run(ctx)

	// The sleep below is just to make sure that the watcher.Run() goroutine has successfully executed
	// and the watcher is handling the events about Example TPR instances.
	// In the normal application there is no need for it, because:
	// 1. It's unlikely to create a watcher and a TPR instance at the same time in the same application.
	// 2. The application with watcher would most probably keep running instead of exiting right after the watcher startup.
	time.Sleep(5 * time.Second)

	// Create an instance of our TPR
	example := &tprv1.Example{
		Metadata: metav1.ObjectMeta{
			Name: "example1",
		},
		Spec: tprv1.ExampleSpec{
			Foo: "hello",
			Bar: true,
		},
	}
	var result tprv1.Example
	err = exampleClient.Post().
		Resource(tprv1.ExampleResourcePlural).
		Namespace(apiv1.NamespaceDefault).
		Body(example).
		Do().Into(&result)
	if (err == nil) {
		fmt.Printf("CREATED: %#v\n", result)
	} else if (apierrors.IsAlreadyExists(err)) {
		fmt.Printf("ALREADY EXISTS: %#v\n", result)
	} else {
		panic(err)
	}

	// Fetch a list of our TPRs
	exampleList := tprv1.ExampleList{}
	err = exampleClient.Get().Resource(tprv1.ExampleResourcePlural).Do().Into(&exampleList)
	if err != nil {
		panic(err)
	}
	fmt.Printf("LIST: %#v\n", exampleList)
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
