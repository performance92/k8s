//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright The KCP Authors.

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

// Code generated by kcp code-generator. DO NOT EDIT.

package v1alpha1

import (
	kcpcache "github.com/kcp-dev/apimachinery/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v2"

	rbacv1alpha1 "k8s.io/api/rbac/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	rbacv1alpha1listers "k8s.io/client-go/listers/rbac/v1alpha1"
	"k8s.io/client-go/tools/cache"
)

// RoleBindingClusterLister can list RoleBindings across all workspaces, or scope down to a RoleBindingLister for one workspace.
type RoleBindingClusterLister interface {
	List(selector labels.Selector) (ret []*rbacv1alpha1.RoleBinding, err error)
	Cluster(cluster logicalcluster.Name) rbacv1alpha1listers.RoleBindingLister
}

type roleBindingClusterLister struct {
	indexer cache.Indexer
}

// NewRoleBindingClusterLister returns a new RoleBindingClusterLister.
func NewRoleBindingClusterLister(indexer cache.Indexer) *roleBindingClusterLister {
	return &roleBindingClusterLister{indexer: indexer}
}

// List lists all RoleBindings in the indexer across all workspaces.
func (s *roleBindingClusterLister) List(selector labels.Selector) (ret []*rbacv1alpha1.RoleBinding, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*rbacv1alpha1.RoleBinding))
	})
	return ret, err
}

// Cluster scopes the lister to one workspace, allowing users to list and get RoleBindings.
func (s *roleBindingClusterLister) Cluster(cluster logicalcluster.Name) rbacv1alpha1listers.RoleBindingLister {
	return &roleBindingLister{indexer: s.indexer, cluster: cluster}
}

// roleBindingLister implements the rbacv1alpha1listers.RoleBindingLister interface.
type roleBindingLister struct {
	indexer cache.Indexer
	cluster logicalcluster.Name
}

// List lists all RoleBindings in the indexer for a workspace.
func (s *roleBindingLister) List(selector labels.Selector) (ret []*rbacv1alpha1.RoleBinding, err error) {
	selectAll := selector == nil || selector.Empty()

	list, err := s.indexer.ByIndex(kcpcache.ClusterIndexName, kcpcache.ClusterIndexKey(s.cluster))
	if err != nil {
		return nil, err
	}

	for i := range list {
		obj := list[i].(*rbacv1alpha1.RoleBinding)
		if selectAll {
			ret = append(ret, obj)
		} else {
			if selector.Matches(labels.Set(obj.GetLabels())) {
				ret = append(ret, obj)
			}
		}
	}

	return ret, err
}

// RoleBindings returns an object that can list and get RoleBindings in one namespace.
func (s *roleBindingLister) RoleBindings(namespace string) rbacv1alpha1listers.RoleBindingNamespaceLister {
	return &roleBindingNamespaceLister{indexer: s.indexer, cluster: s.cluster, namespace: namespace}
}

// roleBindingNamespaceLister implements the rbacv1alpha1listers.RoleBindingNamespaceLister interface.
type roleBindingNamespaceLister struct {
	indexer   cache.Indexer
	cluster   logicalcluster.Name
	namespace string
}

// List lists all RoleBindings in the indexer for a given workspace and namespace.
func (s *roleBindingNamespaceLister) List(selector labels.Selector) (ret []*rbacv1alpha1.RoleBinding, err error) {
	selectAll := selector == nil || selector.Empty()

	var list []interface{}
	if s.namespace == metav1.NamespaceAll {
		list, err = s.indexer.ByIndex(kcpcache.ClusterIndexName, kcpcache.ClusterIndexKey(s.cluster))
	} else {
		list, err = s.indexer.ByIndex(kcpcache.ClusterAndNamespaceIndexName, kcpcache.ClusterAndNamespaceIndexKey(s.cluster, s.namespace))
	}
	if err != nil {
		return nil, err
	}

	for i := range list {
		obj := list[i].(*rbacv1alpha1.RoleBinding)
		if selectAll {
			ret = append(ret, obj)
		} else {
			if selector.Matches(labels.Set(obj.GetLabels())) {
				ret = append(ret, obj)
			}
		}
	}
	return ret, err
}

// Get retrieves the RoleBinding from the indexer for a given workspace, namespace and name.
func (s *roleBindingNamespaceLister) Get(name string) (*rbacv1alpha1.RoleBinding, error) {
	key := kcpcache.ToClusterAwareKey(s.cluster.String(), s.namespace, name)
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(rbacv1alpha1.Resource("RoleBinding"), name)
	}
	return obj.(*rbacv1alpha1.RoleBinding), nil
}
