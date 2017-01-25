package openstack

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/lbaas/pools"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/lbaas/members"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/lbaas/monitors"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/lbaas/vips"
	"github.com/rackspace/gophercloud/pagination"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/api/v1/service"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"github.com/rackspace/gophercloud"
)

// TODO: This code currently ignores 'region' and always creates a
// loadbalancer in only the current OpenStack region.  We should take
// a list of regions (from config) and query/create loadbalancers in
// each region.

func (lb *LbaasV1) GetLoadBalancer(clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	loadBalancerName := cloudprovider.GetLoadBalancerName(service)
	vip, err := getVipByName(lb.network, loadBalancerName)
	if err == ErrNotFound {
		return nil, false, nil
	}
	if vip == nil {
		return nil, false, err
	}

	status := &v1.LoadBalancerStatus{}
	status.Ingress = []v1.LoadBalancerIngress{{IP: vip.Address}}

	return status, true, err
}

func getVipByName(client *gophercloud.ServiceClient, name string) (*vips.VirtualIP, error) {
	opts := vips.ListOpts{
		Name: name,
	}
	pager := vips.List(client, opts)

	vipList := make([]vips.VirtualIP, 0, 1)

	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		v, err := vips.ExtractVIPs(page)
		if err != nil {
			return false, err
		}
		vipList = append(vipList, v...)
		if len(vipList) > 1 {
			return false, ErrMultipleResults
		}
		return true, nil
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if len(vipList) == 0 {
		return nil, ErrNotFound
	} else if len(vipList) > 1 {
		return nil, ErrMultipleResults
	}

	return &vipList[0], nil
}

func (lb *LbaasV1) EnsureLoadBalancer(clusterName string, apiService *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	glog.V(4).Infof("EnsureLoadBalancer(%v, %v, %v, %v, %v, %v, %v)", clusterName, apiService.Namespace, apiService.Name, apiService.Spec.LoadBalancerIP, apiService.Spec.Ports, nodes, apiService.Annotations)

	ports := apiService.Spec.Ports
	if len(ports) > 1 {
		return nil, fmt.Errorf("multiple ports are not supported in openstack v1 load balancers")
	} else if len(ports) == 0 {
		return nil, fmt.Errorf("no ports provided to openstack load balancer")
	}

	// The service controller verified all the protocols match on the ports, just check and use the first one
	// TODO: Convert all error messages to use an event recorder
	if ports[0].Protocol != v1.ProtocolTCP {
		return nil, fmt.Errorf("Only TCP LoadBalancer is supported for openstack load balancers")
	}

	affinity := apiService.Spec.SessionAffinity
	var persistence *vips.SessionPersistence
	switch affinity {
	case v1.ServiceAffinityNone:
		persistence = nil
	case v1.ServiceAffinityClientIP:
		persistence = &vips.SessionPersistence{Type: "SOURCE_IP"}
	default:
		return nil, fmt.Errorf("unsupported load balancer affinity: %v", affinity)
	}

	sourceRanges, err := service.GetLoadBalancerSourceRanges(apiService)
	if err != nil {
		return nil, err
	}

	if !service.IsAllowAll(sourceRanges) {
		return nil, fmt.Errorf("Source range restrictions are not supported for openstack load balancers")
	}

	glog.V(2).Infof("Checking if openstack load balancer already exists: %s", cloudprovider.GetLoadBalancerName(apiService))
	_, exists, err := lb.GetLoadBalancer(clusterName, apiService)
	if err != nil {
		return nil, fmt.Errorf("error checking if openstack load balancer already exists: %v", err)
	}

	// TODO: Implement a more efficient update strategy for common changes than delete & create
	// In particular, if we implement hosts update, we can get rid of UpdateHosts
	if exists {
		err := lb.EnsureLoadBalancerDeleted(clusterName, apiService)
		if err != nil {
			return nil, fmt.Errorf("error deleting existing openstack load balancer: %v", err)
		}
	}

	lbmethod := lb.opts.LBMethod
	if lbmethod == "" {
		lbmethod = pools.LBMethodRoundRobin
	}
	name := cloudprovider.GetLoadBalancerName(apiService)
	pool, err := pools.Create(lb.network, pools.CreateOpts{
		Name:     name,
		Protocol: pools.ProtocolTCP,
		SubnetID: lb.opts.SubnetId,
		LBMethod: lbmethod,
	}).Extract()
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		addr, err := nodeAddressForLB(node)
		if err != nil {
			return nil, err
		}

		_, err = members.Create(lb.network, members.CreateOpts{
			PoolID:       pool.ID,
			ProtocolPort: int(ports[0].NodePort), //Note: only handles single port
			Address:      addr,
		}).Extract()
		if err != nil {
			pools.Delete(lb.network, pool.ID)
			return nil, err
		}
	}

	var mon *monitors.Monitor
	if lb.opts.CreateMonitor {
		mon, err = monitors.Create(lb.network, monitors.CreateOpts{
			Type:       monitors.TypeTCP,
			Delay:      int(lb.opts.MonitorDelay.Duration.Seconds()),
			Timeout:    int(lb.opts.MonitorTimeout.Duration.Seconds()),
			MaxRetries: int(lb.opts.MonitorMaxRetries),
		}).Extract()
		if err != nil {
			pools.Delete(lb.network, pool.ID)
			return nil, err
		}

		_, err = pools.AssociateMonitor(lb.network, pool.ID, mon.ID).Extract()
		if err != nil {
			monitors.Delete(lb.network, mon.ID)
			pools.Delete(lb.network, pool.ID)
			return nil, err
		}
	}

	createOpts := vips.CreateOpts{
		Name:         name,
		Description:  fmt.Sprintf("Kubernetes external service %s", name),
		Protocol:     "TCP",
		ProtocolPort: int(ports[0].Port), //TODO: need to handle multi-port
		PoolID:       pool.ID,
		SubnetID:     lb.opts.SubnetId,
		Persistence:  persistence,
	}

	loadBalancerIP := apiService.Spec.LoadBalancerIP
	if loadBalancerIP != "" {
		createOpts.Address = loadBalancerIP
	}

	vip, err := vips.Create(lb.network, createOpts).Extract()
	if err != nil {
		if mon != nil {
			monitors.Delete(lb.network, mon.ID)
		}
		pools.Delete(lb.network, pool.ID)
		return nil, err
	}

	status := &v1.LoadBalancerStatus{}

	status.Ingress = []v1.LoadBalancerIngress{{IP: vip.Address}}

	if lb.opts.FloatingNetworkId != "" {
		floatIPOpts := floatingips.CreateOpts{
			FloatingNetworkID: lb.opts.FloatingNetworkId,
			PortID:            vip.PortID,
		}
		floatIP, err := floatingips.Create(lb.network, floatIPOpts).Extract()
		if err != nil {
			return nil, err
		}

		status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{IP: floatIP.FloatingIP})
	}

	return status, nil

}

func (lb *LbaasV1) UpdateLoadBalancer(clusterName string, service *v1.Service, nodes []*v1.Node) error {
	loadBalancerName := cloudprovider.GetLoadBalancerName(service)
	glog.V(4).Infof("UpdateLoadBalancer(%v, %v, %v)", clusterName, loadBalancerName, nodes)

	vip, err := getVipByName(lb.network, loadBalancerName)
	if err != nil {
		return err
	}

	// Set of member (addresses) that _should_ exist
	addrs := map[string]bool{}
	for _, node := range nodes {
		addr, err := nodeAddressForLB(node)
		if err != nil {
			return err
		}

		addrs[addr] = true
	}

	// Iterate over members that _do_ exist
	pager := members.List(lb.network, members.ListOpts{PoolID: vip.PoolID})
	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		memList, err := members.ExtractMembers(page)
		if err != nil {
			return false, err
		}

		for _, member := range memList {
			if _, found := addrs[member.Address]; found {
				// Member already exists
				delete(addrs, member.Address)
			} else {
				// Member needs to be deleted
				err = members.Delete(lb.network, member.ID).ExtractErr()
				if err != nil {
					return false, err
				}
			}
		}

		return true, nil
	})
	if err != nil {
		return err
	}

	// Anything left in addrs is a new member that needs to be added
	for addr := range addrs {
		_, err := members.Create(lb.network, members.CreateOpts{
			PoolID:       vip.PoolID,
			Address:      addr,
			ProtocolPort: vip.ProtocolPort,
		}).Extract()
		if err != nil {
			return err
		}
	}

	return nil
}

func (lb *LbaasV1) EnsureLoadBalancerDeleted(clusterName string, service *v1.Service) error {
	loadBalancerName := cloudprovider.GetLoadBalancerName(service)
	glog.V(4).Infof("EnsureLoadBalancerDeleted(%v, %v)", clusterName, loadBalancerName)

	vip, err := getVipByName(lb.network, loadBalancerName)
	if err != nil && err != ErrNotFound {
		return err
	}

	if lb.opts.FloatingNetworkId != "" && vip != nil {
		floatingIP, err := getFloatingIPByPortID(lb.network, vip.PortID)
		if err != nil && !isNotFound(err) {
			return err
		}
		if floatingIP != nil {
			err = floatingips.Delete(lb.network, floatingIP.ID).ExtractErr()
			if err != nil && !isNotFound(err) {
				return err
			}
		}
	}

	// We have to delete the VIP before the pool can be deleted,
	// so no point continuing if this fails.
	if vip != nil {
		err := vips.Delete(lb.network, vip.ID).ExtractErr()
		if err != nil && !isNotFound(err) {
			return err
		}
	}

	var pool *pools.Pool
	if vip != nil {
		pool, err = pools.Get(lb.network, vip.PoolID).Extract()
		if err != nil && !isNotFound(err) {
			return err
		}
	} else {
		// The VIP is gone, but it is conceivable that a Pool
		// still exists that we failed to delete on some
		// previous occasion.  Make a best effort attempt to
		// cleanup any pools with the same name as the VIP.
		pool, err = getPoolByName(lb.network, service.Name)
		if err != nil && err != ErrNotFound {
			return err
		}
	}

	if pool != nil {
		for _, monId := range pool.MonitorIDs {
			_, err = pools.DisassociateMonitor(lb.network, pool.ID, monId).Extract()
			if err != nil {
				return err
			}

			err = monitors.Delete(lb.network, monId).ExtractErr()
			if err != nil && !isNotFound(err) {
				return err
			}
		}
		for _, memberId := range pool.MemberIDs {
			err = members.Delete(lb.network, memberId).ExtractErr()
			if err != nil && !isNotFound(err) {
				return err
			}
		}
		err = pools.Delete(lb.network, pool.ID).ExtractErr()
		if err != nil && !isNotFound(err) {
			return err
		}
	}

	return nil
}

func getPoolByName(client *gophercloud.ServiceClient, name string) (*pools.Pool, error) {
	opts := pools.ListOpts{
		Name: name,
	}
	pager := pools.List(client, opts)

	poolList := make([]pools.Pool, 0, 1)

	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		p, err := pools.ExtractPools(page)
		if err != nil {
			return false, err
		}
		poolList = append(poolList, p...)
		if len(poolList) > 1 {
			return false, ErrMultipleResults
		}
		return true, nil
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if len(poolList) == 0 {
		return nil, ErrNotFound
	} else if len(poolList) > 1 {
		return nil, ErrMultipleResults
	}

	return &poolList[0], nil
}