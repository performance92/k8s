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

package v1beta1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	rest "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/scheme"
)

type RbacV1beta1Interface interface {
	RESTClient() rest.Interface
	ClusterRolesGetter
	ClusterRoleBindingsGetter
	RolesGetter
	RoleBindingsGetter
}

// RbacV1beta1Client is used to interact with features provided by the rbac.authorization.k8s.io group.
type RbacV1beta1Client struct {
	restClient     rest.Interface
	parameterCodec runtime.ParameterCodec
}

func (c *RbacV1beta1Client) ClusterRoles() ClusterRoleInterface {
	return newClusterRoles(c, c.parameterCodec)
}

func (c *RbacV1beta1Client) ClusterRoleBindings() ClusterRoleBindingInterface {
	return newClusterRoleBindings(c, c.parameterCodec)
}

func (c *RbacV1beta1Client) Roles(namespace string) RoleInterface {
	return newRoles(c, namespace, c.parameterCodec)
}

func (c *RbacV1beta1Client) RoleBindings(namespace string) RoleBindingInterface {
	return newRoleBindings(c, namespace, c.parameterCodec)
}

// NewForConfig creates a new RbacV1beta1Client for the given config.
func NewForConfig(c *rest.Config) (*RbacV1beta1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &RbacV1beta1Client{client, c.ParameterCodec}, nil
}

// NewForConfigOrDie creates a new RbacV1beta1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *RbacV1beta1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new RbacV1beta1Client for the given RESTClient.
func New(c rest.Interface) *RbacV1beta1Client {
	return &RbacV1beta1Client{c, scheme.ParameterCodec}
}

func setConfigDefaults(config *rest.Config) error {
	gv := schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1beta1"}
	if config.NegotiatedSerializer == nil {
		return fmt.Errorf("expected non-nil NegotiatedSerializer for %v client", gv)
	}
	if config.ParameterCodec == nil {
		return fmt.Errorf("expected non-nil ParameterCodec for %v client", gv)
	}
	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	config.GroupVersion = &gv

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *RbacV1beta1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
