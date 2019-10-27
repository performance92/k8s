/*
Copyright 2019 The Kubernetes Authors.

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

package topologymanager

import (
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager/bitmask"
	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
	"sort"
)

type singleNumaNodePolicy struct{}

var _ Policy = &singleNumaNodePolicy{}

// PolicySingleNumaNode policy name.
const PolicySingleNumaNode string = "single-numa-node"

// NewSingleNumaNodePolicy returns single-numa-node policy.
func NewSingleNumaNodePolicy() Policy {
	return &singleNumaNodePolicy{}
}

func (p *singleNumaNodePolicy) Name() string {
	return PolicySingleNumaNode
}

func (p *singleNumaNodePolicy) CanAdmitPodResult(hint *TopologyHint) lifecycle.PodAdmitResult {
	if !hint.Preferred || hint.NUMANodeAffinity.Count() > 1 {
		return lifecycle.PodAdmitResult{
			Admit:   false,
			Reason:  "Topology Affinity Error",
			Message: "Resources cannot be allocated with Topology Locality",
		}
	}
	return lifecycle.PodAdmitResult{
		Admit: true,
	}
}

// Get the narrowest hint aligned to one socket
// assumption: allResourcesHints are not empty, all hints have SocketAffinity != nil,
// all hints have exactly one socket set in SocketAffinity
func (p *singleNumaNodePolicy) getHintMatch(allResourcesHints [][]TopologyHint) (bool, TopologyHint) {
	// Sort all resources hints
	for _, resource := range allResourcesHints {
		sort.Slice(resource, func(i, j int) bool {
			if resource[i].NUMANodeAffinity.GetBits()[0] < resource[j].NUMANodeAffinity.GetBits()[0] {
				return true
			}
			return false
		})

	}
	// find a match by searching a hint of one resource in the rest
	var match TopologyHint
	var foundMatch bool

	if len(allResourcesHints) == 1 {
		match = allResourcesHints[0][0]
		match.Preferred = true
		return true, match
	}
	for _, candidate := range allResourcesHints[0] {
		foundMatch = true
		for _, hints := range allResourcesHints[1:] {
			res := sort.Search(len(hints), func(i int) bool {
				return hints[i].NUMANodeAffinity.GetBits()[0] >= candidate.NUMANodeAffinity.GetBits()[0]
			})
			if res >= len(hints) ||
				!hints[res].NUMANodeAffinity.IsEqual(candidate.NUMANodeAffinity) {
				// hint not found, move to next hint from allResourcesHints[0]
				foundMatch = false
				break
			}
		}
		if foundMatch {
			match = candidate
			match.Preferred = true
			break
		}
	}
	return foundMatch, match
}

// Return hints that have valid bitmasks with exactly one bit set
func (p *singleNumaNodePolicy) filterHints(allResourcesHints [][]TopologyHint) [][]TopologyHint {
	var filteredResourcesHints [][]TopologyHint
	if len(allResourcesHints) > 0 {
		for _, oneResourceHints := range allResourcesHints {
			var filtered []TopologyHint
			if len(oneResourceHints) > 0 {
				for _, hint := range oneResourceHints {
					if hint.NUMANodeAffinity != nil && hint.NUMANodeAffinity.Count() == 1 {
						filtered = append(filtered, hint)
					}
				}
			}
			filteredResourcesHints = append(filteredResourcesHints, filtered)
		}
	}
	return filteredResourcesHints
}

func (p *singleNumaNodePolicy) Merge(providersHints []map[string][]TopologyHint, numaNodes []int) TopologyHint {
	// Set the default hint to return from this function as an any-socket
	// affinity with an unpreferred allocation. This will only be returned if
	// no better hint can be found when merging hints from each hint provider.
	defaultAffinity, _ := bitmask.NewBitMask(numaNodes...)
	defaultHint := TopologyHint{defaultAffinity, false}

	// Loop through all provider hints and save an accumulated list of the
	// hints returned by each hint provider. If no hints are provided, assume
	// that provider has no preference for topology-aware allocation.
	var allResourcesHints [][]TopologyHint
	for _, hints := range providersHints {
		if len(hints) == 0 {
			klog.Infof("[topologymanager] Hint Provider has no preference for socket affinity with any resource, " +
				"skipping.")
			continue
		}

		// Otherwise, accumulate the hints for each resource type into allProviderHints.
		for resource := range hints {
			if hints[resource] == nil {
				klog.Infof("[topologymanager] Hint Provider has no preference for socket affinity with resource "+
					"'%s', skipping.", resource)
				continue
			}

			if len(hints[resource]) == 0 {
				klog.Infof("[topologymanager] Hint Provider has no possible socket affinities for resource '%s'",
					resource)
				// return defaultHint which will fail pod admission
				return defaultHint
			}
			klog.Infof("[topologymanager] TopologyHints for resource '%v': %v", resource, hints[resource])
			allResourcesHints = append(allResourcesHints, hints[resource])
		}
	}
	// In case allProviderHints length is zero it means that we have no
	// preference for socket affinity. In that case update default hint preferred
	// to true to allow scheduling.
	if len(allResourcesHints) == 0 {
		klog.Infof("[topologymanager] No preference for socket affinity from all providers")
		defaultHint.Preferred = true
		return defaultHint
	}

	allResourcesHints = p.filterHints(allResourcesHints)
	// if no hints or there is a resource with empty hints after filtering, then policy cannot be satisfied
	if len(allResourcesHints) == 0 {
		klog.Infof("[topologymanager] No hints that align to a single socket.")
		return defaultHint
	}
	for _, hints := range allResourcesHints {
		if len(hints) == 0 {
			klog.Infof("[topologymanager] No hints that align to a single socket for resource.")
			return defaultHint
		}
	}

	found, match := p.getHintMatch(allResourcesHints)
	if found {
		klog.Infof("[topologymanager] Strict Policy match: %v", match)
		return match
	}
	klog.Infof("[topologymanager] Strict Policy no match: %v", defaultHint)
	return defaultHint
}
