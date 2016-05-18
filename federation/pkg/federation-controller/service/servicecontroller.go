/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package service

import (
	"fmt"
	"sync"
	"time"

	"reflect"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/api/errors"
	cache "k8s.io/kubernetes/pkg/client/cache"
	federationcache "k8s.io/kubernetes/federation/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	federationclientset "k8s.io/kubernetes/federation/client/clientset_generated/federation_release_1_3"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	pkg_runtime "k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/federation/apis/federation/v1alpha1"
	"k8s.io/kubernetes/federation/pkg/dnsprovider"
	//"k8s.io/kubernetes/federation/pkg/dnsprovider/rrstype"

	"k8s.io/kubernetes/pkg/conversion"
)

const (
	// TODO update to 10 mins before merge
	serviceSyncPeriod = 30 * time.Second
	clusterSyncPeriod = 100 * time.Second

	// How long to wait before retrying the processing of a service change.
	// If this changes, the sleep in hack/jenkins/e2e.sh before downing a cluster
	// should be changed appropriately.
	minRetryDelay = 5 * time.Second
	maxRetryDelay = 300 * time.Second

	// client retry count and interval is when accessing a remote kube-apiserver or federation apiserver
	// how many times should be attempted and how long it should sleep when failure occurs
	// the retry should be in short time so no exponential backoff
	clientRetryCount    = 5

	retryable    = true

	doNotRetry = time.Duration(0)

	UserAgentName = "Federation-Service-Controller"
	KubeAPIQPS    = 20.0
	KubeAPIBurst  = 30
)

type cachedService struct {

	//release_1_3 client returns v1 service
	lastState      *v1.Service
	// The state as successfully applied to the DNS server
	appliedState   *v1.Service
	// cluster endpoint map hold subset info from kubernetes clusters
	// key clusterName
	// value is a flag that if there is ready addresses
	endpointMap    map[string]int
	// cluster service map hold serivice status info from kubernetes clusters
	// key clusterName

	serviceStatusMap    map[string]v1.LoadBalancerStatus

	// Ensures only one goroutine can operate on this service at any given time.
	rwlock         sync.Mutex

	// Controls error back-off for procceeding federation service to k8s clusters
	lastRetryDelay time.Duration
	// Controls error back-off for updating federation service back to federation apiserver
	lastFedUpdateDelay time.Duration
	// Controls error back-off for dns record update
	lastDNSUpdateDelay time.Duration

}

type serviceCache struct {
	rwlock        sync.Mutex // protects serviceMap
	// federation service map contains all service received from federation apiserver
	// key serviceName
	fedServiceMap map[string]*cachedService
}

type ServiceController struct {
	dns               dnsprovider.Interface
	federationClient  federationclientset.Interface
	federationName    string
	// each federation should be configured with a single zone (e.g. "mycompany.com")
	zones             []dnsprovider.Zone
	serviceCache      *serviceCache
	clusterCache      *clusterClientCache
	// A store of services, populated by the serviceController
	serviceStore      cache.StoreToServiceLister
	// Watches changes to all services
	serviceController *framework.Controller
	// A store of services, populated by the serviceController
	clusterStore      federationcache.StoreToClusterLister
	// Watches changes to all services
	clusterController *framework.Controller
	eventBroadcaster  record.EventBroadcaster
	eventRecorder     record.EventRecorder
	//clusterLister     federationcache.StoreToClusterLister
	// services that need to be synced
	queue             *workqueue.Type
	knownClusterSet   sets.String
}

// New returns a new service controller to keep DNS provider service resources
// (like Kubernetes Services and DNS server records for service discovery) in sync with the registry.

func New(federationClient federationclientset.Interface, dns dnsprovider.Interface, federationName string) *ServiceController {
	broadcaster := record.NewBroadcaster()
	// federationClient event is not supported yet?
	//broadcaster.StartRecordingToSink(&unversioned_core.EventSinkImpl{Interface: kubeClient.Core().Events("")})
	recorder := broadcaster.NewRecorder(api.EventSource{Component: "federated-service-controller"})

	s := &ServiceController{
		dns:              dns,
		federationClient: federationClient,
		federationName:   federationName,
		serviceCache:     &serviceCache{fedServiceMap: make(map[string]*cachedService)},
		clusterCache: 	  &clusterClientCache{
			rwlock: sync.Mutex{},
			clientMap: make(map[string]*clusterCache),
		},
		eventBroadcaster: broadcaster,
		eventRecorder:    recorder,
		queue:            workqueue.New(),
		knownClusterSet:    make(sets.String),
	}
	s.serviceStore.Store, s.serviceController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (pkg_runtime.Object, error) {
				return s.federationClient.Core().Services(api.NamespaceAll).List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return s.federationClient.Core().Services(api.NamespaceAll).Watch(options)
			},
		},
		&v1.Service{},
		serviceSyncPeriod,
		framework.ResourceEventHandlerFuncs{
			AddFunc: s.enqueueService,
			UpdateFunc: func(old, cur interface{}) {
				// there is case that old and new are equals but we still catch the event now.
				if !reflect.DeepEqual(old, cur) {
					s.enqueueService(cur)
				}
			},
			DeleteFunc: s.enqueueService,
		},
	)
	s.clusterStore.Store, s.clusterController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (pkg_runtime.Object, error) {
				return s.federationClient.Federation().Clusters().List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return s.federationClient.Federation().Clusters().Watch(options)
			},
		},
		&v1alpha1.Cluster{},
		clusterSyncPeriod,
		framework.ResourceEventHandlerFuncs{
			DeleteFunc: s.clusterCache.delFromClusterSet,
			AddFunc:    s.clusterCache.addToClientMap,
			UpdateFunc:    func(old, cur interface{}) {
				s.clusterCache.addToClientMap(cur)
			},
		},
	)
	return s
}

// obj could be an *api.Service, or a DeletionFinalStateUnknown marker item.
func (s *ServiceController) enqueueService (obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	s.queue.Add(key)
}

// Run starts a background goroutine that watches for changes to Federated services
// and ensures that they have Kubernetes services created, updated or deleted appropriately.
// federationSyncPeriod controls how often we check the federation's services to
// ensure that the correct Kubernetes services (and associated DNS entries) exist.
// This is only necessary to fudge over failed watches.
// clusterSyncPeriod controls how often we check the federation's underlying clusters and
// their Kubernetes services to ensure that matching services created independently of the Federation
// (e.g. directly via the underlying cluster's API) are correctly accounted for.

// It's an error to call Run() more than once for a given ServiceController
// object.
func (s *ServiceController) Run(workers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	go s.serviceController.Run(stopCh)
	go s.clusterController.Run(stopCh)
	for i := 0; i < workers; i++ {
		go wait.Until(s.fedServiceWorker, time.Second, stopCh)
	}
	go wait.Until(s.clusterEndpointWorker, time.Second, stopCh)
	go wait.Until(s.clusterServiceWorker, time.Second, stopCh)
	go wait.Until(s.clusterSyncLoop, clusterSyncPeriod, stopCh)
	<-stopCh
	glog.Infof("Shutting down Federation Service Controller")
	s.queue.ShutDown()
	return nil
}

// fedServiceWorker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncService is never invoked concurrently with the same key.
func (s *ServiceController) fedServiceWorker() {
	for {
		func() {
			key, quit := s.queue.Get()
			if quit {
				return
			}
			defer s.queue.Done(key)
			err := s.syncService(key.(string))
			if err != nil {
				glog.Errorf("Error syncing service: %v", err)
			}
		}()
	}
}

func wantsDNSRecords(service *v1.Service) bool{
	return service.Spec.Type == v1.ServiceTypeLoadBalancer
}

// processServiceForCluster creates or updates service to all registered running clusters,
// update DNS records and update the service info with DNS entries to federation apiserver.
// the function returns any error caught
func (s *ServiceController) processServiceForCluster(cachedService *cachedService, clusterName string, service *v1.Service, client *clientset.Clientset) error {
	glog.V(4).Infof("process service %s/%s for cluster %s", service.Namespace, service.Name, clusterName)
	// Create or Update k8s Service
	err := s.ensureClusterService(cachedService, clusterName, service, client)
	if err != nil {
		glog.V(4).Infof("Failed to process service %s/%s for cluster %s", service.Namespace, service.Name, clusterName)
		return err
	}
	glog.V(4).Infof("Successfully process service %s/%s for cluster %s", service.Namespace, service.Name, clusterName)
	return nil
}

// updateFederationService Returns whatever error occurred along with a boolean indicator of whether it
// should be retried.
func (s *ServiceController) updateFederationService(key string, cachedService *cachedService) (error, bool) {
	// Clone federation service, and create them in underlying k8s cluster
	clone, err := conversion.NewCloner().DeepCopy(cachedService.lastState)
	if err != nil {
		return err, !retryable
	}
	service, ok := clone.(*v1.Service)
	if !ok {
		return fmt.Errorf("Unexpected service cast error : %v\n", service), !retryable
	}

	// handle available clusters one by one
	var hasErr bool
	for clusterName, cluster := range s.clusterCache.clientMap {
		go func(){
			err = s.processServiceForCluster(cachedService, clusterName, service, cluster.clientset)
			if err != nil {
				hasErr = true
			}
		}()
	}
	if hasErr {
		// detail error has been dumpped inside the loop
		return fmt.Errorf("service %s/%s was not successfully updated to all clusters", service.Namespace, service.Name), retryable
	}
	return nil, !retryable
}

func (s *ServiceController) deleteFederationService(cachedService *cachedService) (error, bool) {
	// handle available clusters one by one
	var hasErr bool
	for clusterName, cluster := range s.clusterCache.clientMap {
		err, _ := s.deleteClusterService(clusterName, cachedService, cluster.clientset)
		if err != nil {
			hasErr = true
		}
	}
	if hasErr {
		// detail error has been dumpped inside the loop
		return fmt.Errorf("service %s/%s was not successfully updated to all clusters", cachedService.lastState.Namespace, cachedService.lastState.Name), retryable
	}
	return nil, !retryable
}

func (s *ServiceController) deleteClusterService(clusterName string, cachedService *cachedService, clientset *clientset.Clientset) (error, bool) {
	service := cachedService.lastState
	glog.V(4).Infof("Deleting service %s/%s from cluster %s", service.Namespace, service.Name, clusterName)
	var err error
	for i := 0; i < clientRetryCount; i++ {
		err = clientset.Core().Services(service.Namespace).Delete(service.Name, &api.DeleteOptions{})
		if err == nil || errors.IsNotFound(err) {
			glog.V(4).Infof("service %s/%s deleted from cluster %s", service.Namespace, service.Name, clusterName)
			return nil, !retryable
		}
		time.Sleep(cachedService.nextRetryDelay())
	}
	glog.V(4).Infof("failed to delete service %s/%s from cluster %s, %+v", service.Namespace, service.Name, clusterName, err)
	return err, retryable
}

func (s *ServiceController) ensureClusterService(cachedService *cachedService, clusterName string, service *v1.Service, client *clientset.Clientset) error {
	var err error
	var needUpdate bool
	for i := 0; i < clientRetryCount; i++ {
		svc, err := client.Core().Services(service.Namespace).Get(service.Name)
		if err == nil && svc != nil {
			// service exists
			glog.V(5).Infof("found service %s/%s from cluster %s", service.Namespace, service.Name, clusterName)
			//reserve immutable fields
			service.Spec.ClusterIP = svc.Spec.ClusterIP

			//reserve auto assigned field
			for i, oldPort := range svc.Spec.Ports{
				for _, port := range service.Spec.Ports {
					if port.NodePort == 0 {
						if !portEqualExcludeNodePort(&oldPort, &port) {
							svc.Spec.Ports[i] = port
							needUpdate = true
						}
					} else {
						if !portEqualForLB(&oldPort, &port) {
							svc.Spec.Ports[i] = port
							needUpdate = true
						}
					}
				}
			}

			if needUpdate {
				// we only apply spec update for now
				svc.Spec = service.Spec
				_, err = client.Core().Services(svc.Namespace).Update(svc)
				if err == nil {
					glog.V(5).Infof("service %s/%s successfully updated to cluster %s", svc.Namespace, svc.Name, clusterName)
					return nil
				} else {
					glog.V(4).Infof("failed to update %+v", err)
				}
			} else {
				glog.V(5).Infof("service %s/%s is not updated to cluster %s as the spec are identical", svc.Namespace, svc.Name, clusterName)
				return nil
			}
		}
		if svc == nil || errors.IsNotFound(err) {
			// Create service if it is not found
			glog.Infof("Service '%s/%s' does not exist in cluster %s, trying to create new",
				service.Namespace, service.Name, clusterName)
			service.ResourceVersion = ""
			_, err = client.Core().Services(service.Namespace).Create(service)
			if err == nil {
				glog.V(5).Infof("service %s/%s successfully created to cluster %s", service.Namespace, service.Name, clusterName)
				return nil
			}
			glog.V(4).Infof("failed to create %+v", err)
		}
		if errors.IsConflict(err) {
			glog.V(4).Infof("Not persisting update to service '%s/%s' that has been changed since we received it: %v",
				service.Namespace, service.Name, err)
		}
		// should we reuse same retry delay for all clusters?
		time.Sleep(cachedService.nextRetryDelay())
	}
	return err
}

func (s *serviceCache) allServices() []*cachedService {
	s.rwlock.Lock()
	defer s.rwlock.Unlock()
	services := make([]*cachedService, 0, len(s.fedServiceMap))
	for _, v := range s.fedServiceMap {
		services = append(services, v)
	}
	return services
}

func (s *serviceCache) get(serviceName string) (*cachedService, bool) {
	s.rwlock.Lock()
	defer s.rwlock.Unlock()
	service, ok := s.fedServiceMap[serviceName]
	return service, ok
}

func (s *serviceCache) getOrCreate(serviceName string) *cachedService {
	s.rwlock.Lock()
	defer s.rwlock.Unlock()
	service, ok := s.fedServiceMap[serviceName]
	if !ok {
		service = &cachedService {
			endpointMap: make(map[string]int),
			serviceStatusMap: make(map[string]v1.LoadBalancerStatus),
		}
		s.fedServiceMap[serviceName] = service
	}
	return service
}

func (s *serviceCache) set(serviceName string, service *cachedService) {
	s.rwlock.Lock()
	defer s.rwlock.Unlock()
	s.fedServiceMap[serviceName] = service
}

func (s *serviceCache) delete(serviceName string) {
	s.rwlock.Lock()
	defer s.rwlock.Unlock()
	delete(s.fedServiceMap, serviceName)
}

// needsUpdateDNS check if the dns records of the given service should be updated
func (s *ServiceController) needsUpdateDNS(oldService *v1.Service, newService *v1.Service) bool {
	if !wantsDNSRecords(oldService) && !wantsDNSRecords(newService) {
		return false
	}
	if wantsDNSRecords(oldService) != wantsDNSRecords(newService) {
		s.eventRecorder.Eventf(newService, api.EventTypeNormal, "Type", "%v -> %v",
			oldService.Spec.Type, newService.Spec.Type)
		return true
	}
	if !portsEqualForLB(oldService, newService) || oldService.Spec.SessionAffinity != newService.Spec.SessionAffinity {
		return true
	}
	if !LoadBalancerIPsAreEqual(oldService, newService) {
		s.eventRecorder.Eventf(newService, api.EventTypeNormal, "LoadbalancerIP", "%v -> %v",
			oldService.Spec.LoadBalancerIP, newService.Spec.LoadBalancerIP)
		return true
	}
	if len(oldService.Spec.ExternalIPs) != len(newService.Spec.ExternalIPs) {
		s.eventRecorder.Eventf(newService, api.EventTypeNormal, "ExternalIP", "Count: %v -> %v",
			len(oldService.Spec.ExternalIPs), len(newService.Spec.ExternalIPs))
		return true
	}
	for i := range oldService.Spec.ExternalIPs {
		if oldService.Spec.ExternalIPs[i] != newService.Spec.ExternalIPs[i] {
			s.eventRecorder.Eventf(newService, api.EventTypeNormal, "ExternalIP", "Added: %v",
				newService.Spec.ExternalIPs[i])
			return true
		}
	}
	if !reflect.DeepEqual(oldService.Annotations, newService.Annotations) {
		return true
	}
	if oldService.UID != newService.UID {
		s.eventRecorder.Eventf(newService, api.EventTypeNormal, "UID", "%v -> %v",
			oldService.UID, newService.UID)
		return true
	}

	return false
}

func getPortsForLB(service *v1.Service) ([]*v1.ServicePort, error) {
	// TODO: quinton: Probably applies for DNS SVC records.  Come back to this.
	//var protocol api.Protocol

	ports := []*v1.ServicePort{}
	for i := range service.Spec.Ports {
		sp := &service.Spec.Ports[i]
		// The check on protocol was removed here.  The DNS provider itself is now responsible for all protocol validation
		ports = append(ports, sp)
		//TODO:jesse
		//if protocol == "" {
		//	protocol = sp.Protocol
		//} else if protocol != sp.Protocol && wantsLoadBalancer(service) {
		//	// TODO:  Convert error messages to use event recorder
		//	return nil, fmt.Errorf("mixed protocol external load balancers are not supported.")
		//}
	}
	return ports, nil
}

func portsEqualForLB(x, y *v1.Service) bool {
	xPorts, err := getPortsForLB(x)
	if err != nil {
		return false
	}
	yPorts, err := getPortsForLB(y)
	if err != nil {
		return false
	}
	return portSlicesEqualForLB(xPorts, yPorts)
}

func portSlicesEqualForLB(x, y []*v1.ServicePort) bool {
	if len(x) != len(y) {
		return false
	}

	for i := range x {
		if !portEqualForLB(x[i], y[i]) {
			return false
		}
	}
	return true
}

func portEqualForLB(x, y *v1.ServicePort) bool {
	// TODO: Should we check name?  (In theory, an LB could expose it)
	if x.Name != y.Name {
		return false
	}

	if x.Protocol != y.Protocol {
		return false
	}

	if x.Port != y.Port {
		return false
	}
	if x.NodePort != y.NodePort {
		return false
	}
	return true
}

func portEqualExcludeNodePort(x, y *v1.ServicePort) bool {
	// TODO: Should we check name?  (In theory, an LB could expose it)
	if x.Name != y.Name {
		return false
	}

	if x.Protocol != y.Protocol {
		return false
	}

	if x.Port != y.Port {
		return false
	}
	return true
}

func clustersFromList(list *v1alpha1.ClusterList) []string {
	result := []string{}
	for ix := range list.Items {
		result = append(result, list.Items[ix].Name)
	}
	return result
}

// getClusterConditionPredicate filter all clusters meet condition of
// condition.type=Ready and condition.status=true
func getClusterConditionPredicate() federationcache.ClusterConditionPredicate {
	return func(cluster v1alpha1.Cluster) bool {
		// If we have no info, don't accept
		if len(cluster.Status.Conditions) == 0 {
			return false
		}
		for _, cond := range cluster.Status.Conditions {
			//We consider the cluster for load balancing only when its ClusterReady condition status
			//is ConditionTrue
			if cond.Type == v1alpha1.ClusterReady && cond.Status != v1.ConditionTrue {
				glog.V(4).Infof("Ignoring cluser %v with %v condition status %v", cluster.Name, cond.Type, cond.Status)
				return false
			}
		}
		return true
	}
}

// clusterSyncLoop observes running clusters changes, and apply all services to new added cluster
// and add dns records for the changes
func (s *ServiceController) clusterSyncLoop() {
	var servicesToUpdate []*cachedService
	// should we remove cache for cluster from ready to not ready? should remove the condition predicate if no
	clusters, err := s.clusterStore.ClusterCondition(getClusterConditionPredicate()).List()
	if err != nil {
		glog.Infof("fail to get cluster list")
		return
	}
	newClusters := clustersFromList(&clusters)
	var newSet, increase sets.String
	newSet = sets.NewString(newClusters...)
	if newSet.Equal(s.knownClusterSet) {
		// The set of cluster names in the services in the federation hasn't changed, but we can retry
		// updating any services that we failed to update last time around.
		servicesToUpdate = s.updateDNSRecords(servicesToUpdate, newClusters)
		return
	}
	glog.Infof("Detected change in list of cluster names. New  set: %v, Old set: %v", newSet, s.knownClusterSet)
	increase = newSet.Difference(newSet)
	// do nothing when cluster is removed.
	if increase != nil {
		for newCluster, _ := range increase {
			glog.Infof("New cluster observed %s", newCluster)
			s.updateAllServicesToCluster(servicesToUpdate, newCluster)
		}
		// Try updating all services, and save the ones that fail to try again next
		// round.
		servicesToUpdate = s.serviceCache.allServices()
		numServices := len(servicesToUpdate)
		servicesToUpdate = s.updateDNSRecords(servicesToUpdate, newClusters)
		glog.Infof("Successfully updated %d out of %d DNS records to direct traffic to the updated cluster",
			numServices-len(servicesToUpdate), numServices)
	}
	s.knownClusterSet = newSet
}

func (s *ServiceController)updateAllServicesToCluster(services []*cachedService, clusterName string){
	cluster, ok := s.clusterCache.clientMap[clusterName]
	if ok {
		for _, cachedService := range services{
			appliedState := cachedService.appliedState
			s.processServiceForCluster(cachedService, clusterName, appliedState, cluster.clientset)
		}
	}
}

func (s *ServiceController)removeAllServicesFromCluster(services []*cachedService, clusterName string){
	client, ok := s.clusterCache.clientMap[clusterName]
	if ok {
		for _, cachedService := range services{
			s.deleteClusterService(clusterName, cachedService, client.clientset)
		}
		glog.Infof("synced all services to cluster %s", clusterName)
	}
}

// updateDNSRecords updates all existing federated service DNS Records so that
// they will match the list of cluster names provided.
// Returns the list of services that couldn't be updated.
func (s *ServiceController) updateDNSRecords(services []*cachedService, clusters []string) (servicesToRetry []*cachedService) {
	for _, service := range services {
		func() {
			service.rwlock.Lock()
			defer service.rwlock.Unlock()
			// If the applied state is nil, that means it hasn't yet been successfully dealt
			// with by the DNS Record reconciler. We can trust the DNS Record
			// reconciler to ensure the federated service's DNS records are created to target
			// the correct backend service IP's
			if service.appliedState == nil {
				return
			}
			if err := s.lockedUpdateDNSRecords(service, clusters); err != nil {
				glog.Errorf("External error while updating DNS Records: %v.", err)
				servicesToRetry = append(servicesToRetry, service)
			}
		}()
	}
	return servicesToRetry
}

// lockedUpdateDNSRecords Updates the DNS records of a service, assuming we hold the mutex
// associated with the service.
// TODO: quinton: Still screwed up in the same way as above.  Fix.
func (s *ServiceController) lockedUpdateDNSRecords(service *cachedService, clusterNames []string) error {
	if !wantsDNSRecords(service.appliedState) {
		return nil
	}
	for key, _ := range s.clusterCache.clientMap {
		for _, clusterName := range clusterNames {
			if key == clusterName {
				ensureDNSRecords(clusterName, service)
			}
		}
	}
	return nil
}

func LoadBalancerIPsAreEqual(oldService, newService *v1.Service) bool {
	return oldService.Spec.LoadBalancerIP == newService.Spec.LoadBalancerIP
}

// Computes the next retry, using exponential backoff
// mutex must be held.
func (s *cachedService) nextRetryDelay() time.Duration {
	s.lastRetryDelay = s.lastRetryDelay * 2
	if s.lastRetryDelay < minRetryDelay {
		s.lastRetryDelay = minRetryDelay
	}
	if s.lastRetryDelay > maxRetryDelay {
		s.lastRetryDelay = maxRetryDelay
	}
	return s.lastRetryDelay
}

// resetRetryDelay Resets the retry exponential backoff.  mutex must be held.
func (s *cachedService) resetRetryDelay() {
	s.lastRetryDelay = time.Duration(0)
}

// Computes the next retry, using exponential backoff
// mutex must be held.
func (s *cachedService) nextFedUpdateDelay() time.Duration {
	s.lastFedUpdateDelay = s.lastFedUpdateDelay * 2
	if s.lastFedUpdateDelay < minRetryDelay {
		s.lastFedUpdateDelay = minRetryDelay
	}
	if s.lastFedUpdateDelay > maxRetryDelay {
		s.lastFedUpdateDelay = maxRetryDelay
	}
	return s.lastFedUpdateDelay
}

// resetRetryDelay Resets the retry exponential backoff.  mutex must be held.
func (s *cachedService) resetFedUpdateDelay() {
	s.lastFedUpdateDelay = time.Duration(0)
}

// Computes the next retry, using exponential backoff
// mutex must be held.
func (s *cachedService) nextDNSUpdateDelay() time.Duration {
	s.lastDNSUpdateDelay = s.lastDNSUpdateDelay * 2
	if s.lastDNSUpdateDelay < minRetryDelay {
		s.lastDNSUpdateDelay = minRetryDelay
	}
	if s.lastDNSUpdateDelay > maxRetryDelay {
		s.lastDNSUpdateDelay = maxRetryDelay
	}
	return s.lastDNSUpdateDelay
}

// resetRetryDelay Resets the retry exponential backoff.  mutex must be held.
func (s *cachedService) resetDNSUpdateDelay() {
	s.lastDNSUpdateDelay = time.Duration(0)
}


// syncService will sync the Service with the given key if it has had its expectations fulfilled,
// meaning it did not expect to see any more of its pods created or deleted. This function is not meant to be
// invoked concurrently with the same key.
func (s *ServiceController) syncService(key string) error {
	startTime := time.Now()
	var cachedService *cachedService
	var retryDelay time.Duration
	defer func() {
		glog.V(4).Infof("Finished syncing service %q (%v)", key, time.Now().Sub(startTime))
	}()
	// obj holds the latest service info from apiserver
	obj, exists, err := s.serviceStore.Store.GetByKey(key)
	if err != nil {
		glog.Infof("Unable to retrieve service %v from store: %v", key, err)
		s.queue.Add(key)
		return err
	}

	if !exists {
		// service absence in store means watcher caught the deletion, ensure LB info is cleaned
		glog.Infof("Service has been deleted %v", key)
		err, retryDelay = s.processServiceDeletion(key)
	}

	if exists {
		service, ok := obj.(*v1.Service)
		if ok {
			cachedService = s.serviceCache.getOrCreate(key)
			err, retryDelay = s.processServiceUpdate(cachedService, service, key)
		} else {
			tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
			if !ok {
				return fmt.Errorf("object contained wasn't a service or a deleted key: %+v", obj)
			}
			glog.Infof("Found tombstone for %v", key)
			err, retryDelay = s.processServiceDeletion(tombstone.Key)
		}
	}

	if retryDelay != 0 {
		s.enqueueService(obj)
	} else if err != nil {
		runtime.HandleError(fmt.Errorf("Failed to process service. Not retrying: %v", err))
	}
	return nil
}

// processServiceUpdate returns an error if processing the service update failed, along with a time.Duration
// indicating whether processing should be retried; zero means no-retry; otherwise
// we should retry in that Duration.
func (s *ServiceController) processServiceUpdate(cachedService *cachedService, service *v1.Service, key string) (error, time.Duration) {
	// Ensure that no other goroutine will interfere with our processing of the
	// service.
	cachedService.rwlock.Lock()
	defer cachedService.rwlock.Unlock()

	// Update the cached service (used above for populating synthetic deletes)
	// alway trust service, which is retrieve from serviceStore, which keeps the latest service info getting from apiserver
	// if the same service is changed before this go routine finished, there will be another queue entry to handle that.
	cachedService.lastState = service
	err, retry := s.updateFederationService(key, cachedService)
	if err != nil {
		message := "Error occurs when updating service to all clusters"
		if retry {
			message += " (will retry): "
		} else {
			message += " (will not retry): "
		}
		message += err.Error()
		s.eventRecorder.Event(service, api.EventTypeWarning, "UpdateServiceFail", message)
		return err, cachedService.nextRetryDelay()
	}
	// Always update the cache upon success.
	// NOTE: Since we update the cached service if and only if we successfully
	// processed it, a cached service being nil implies that it hasn't yet
	// been successfully processed.
	//cachedService.appliedState = service
	s.serviceCache.set(key, cachedService)
	glog.V(4).Infof("successfully procceeded services %s", key)
	cachedService.resetRetryDelay()
	return nil, doNotRetry
}

// processServiceDeletion returns an error if processing the service deletion failed, along with a time.Duration
// indicating whether processing should be retried; zero means no-retry; otherwise
// we should retry in that Duration.
func (s *ServiceController) processServiceDeletion(key string) (error, time.Duration){
	glog.V(2).Infof("process service deletion for %v", key)
	cachedService, ok := s.serviceCache.get(key)
	if !ok {
		return fmt.Errorf("Service %s not in cache even though the watcher thought it was. Ignoring the deletion.", key), doNotRetry
	}
	service := cachedService.lastState
	cachedService.rwlock.Lock()
	defer cachedService.rwlock.Unlock()
	s.eventRecorder.Event(service, api.EventTypeNormal, "DeletingDNSRecord", "Deleting DNS Records")
	// TODO should we delete dns info here or wait for endpoint changes? prefer here
	// or we do nothing for service deletion
	//err := s.dns.balancer.EnsureLoadBalancerDeleted(service)
	err, retry := s.deleteFederationService(cachedService)
	if err != nil {
		message := "Error occurs when deleting federation service"
		if retry {
			message += " (will retry): "
		} else {
			message += " (will not retry): "
		}
		s.eventRecorder.Event(service, api.EventTypeWarning, "DeletingDNSRecordFailed", message)
		return err, cachedService.nextRetryDelay()
	}
	s.eventRecorder.Event(service, api.EventTypeNormal, "DeletedDNSRecord", "Deleted DNS Records")
	s.serviceCache.delete(key)

	cachedService.resetRetryDelay()
	return nil, doNotRetry
}
