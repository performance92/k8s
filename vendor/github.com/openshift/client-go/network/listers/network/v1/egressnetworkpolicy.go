// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/openshift/api/network/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// EgressNetworkPolicyLister helps list EgressNetworkPolicies.
type EgressNetworkPolicyLister interface {
	// List lists all EgressNetworkPolicies in the indexer.
	List(selector labels.Selector) (ret []*v1.EgressNetworkPolicy, err error)
	// EgressNetworkPolicies returns an object that can list and get EgressNetworkPolicies.
	EgressNetworkPolicies(namespace string) EgressNetworkPolicyNamespaceLister
	EgressNetworkPolicyListerExpansion
}

// egressNetworkPolicyLister implements the EgressNetworkPolicyLister interface.
type egressNetworkPolicyLister struct {
	indexer cache.Indexer
}

// NewEgressNetworkPolicyLister returns a new EgressNetworkPolicyLister.
func NewEgressNetworkPolicyLister(indexer cache.Indexer) EgressNetworkPolicyLister {
	return &egressNetworkPolicyLister{indexer: indexer}
}

// List lists all EgressNetworkPolicies in the indexer.
func (s *egressNetworkPolicyLister) List(selector labels.Selector) (ret []*v1.EgressNetworkPolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.EgressNetworkPolicy))
	})
	return ret, err
}

// EgressNetworkPolicies returns an object that can list and get EgressNetworkPolicies.
func (s *egressNetworkPolicyLister) EgressNetworkPolicies(namespace string) EgressNetworkPolicyNamespaceLister {
	return egressNetworkPolicyNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// EgressNetworkPolicyNamespaceLister helps list and get EgressNetworkPolicies.
type EgressNetworkPolicyNamespaceLister interface {
	// List lists all EgressNetworkPolicies in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1.EgressNetworkPolicy, err error)
	// Get retrieves the EgressNetworkPolicy from the indexer for a given namespace and name.
	Get(name string) (*v1.EgressNetworkPolicy, error)
	EgressNetworkPolicyNamespaceListerExpansion
}

// egressNetworkPolicyNamespaceLister implements the EgressNetworkPolicyNamespaceLister
// interface.
type egressNetworkPolicyNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all EgressNetworkPolicies in the indexer for a given namespace.
func (s egressNetworkPolicyNamespaceLister) List(selector labels.Selector) (ret []*v1.EgressNetworkPolicy, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.EgressNetworkPolicy))
	})
	return ret, err
}

// Get retrieves the EgressNetworkPolicy from the indexer for a given namespace and name.
func (s egressNetworkPolicyNamespaceLister) Get(name string) (*v1.EgressNetworkPolicy, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("egressnetworkpolicy"), name)
	}
	return obj.(*v1.EgressNetworkPolicy), nil
}
