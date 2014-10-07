/*
Copyright 2014 Google Inc. All rights reserved.

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

package pod

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/service"
)

type ManifestFactory interface {
	// Make a container object for a given pod, given the machine that the pod is running on.
	MakeManifest(machine string, pod api.Pod) (api.ContainerManifest, error)
}

type BasicManifestFactory struct {
	// TODO: this should really point at the API rather than a registry
	ServiceRegistry service.Registry
}

func (b *BasicManifestFactory) MakeManifest(machine string, pod api.Pod) (api.ContainerManifest, error) {
	envVars, err := service.GetServiceEnvironmentVariables(api.NewContext(), b.ServiceRegistry, machine)
	if err != nil {
		return api.ContainerManifest{}, err
	}
	for ix, container := range pod.Spec.Containers {
		pod.Spec.ID = pod.ID
		pod.Spec.Containers[ix].Env = append(container.Env, envVars...)
	}
	return pod.Spec, nil
}
