/*
Copyright 2023 The Kubernetes Authors.

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

package kubeletplugin

import (
	"context"
	"fmt"

	registerapi "k8s.io/kubelet/pkg/apis/pluginregistration/v1"
)

// registrationServer implements the kubelet plugin registration gRPC interface.
type registrationServer struct {
	driverName        string
	endpoint          string
	supportedVersions []string
	status            *registerapi.RegistrationStatus
}

var _ registerapi.RegistrationServer = &registrationServer{}

// GetInfo is the RPC invoked by plugin watcher.
func (e *registrationServer) GetInfo(ctx context.Context, req *registerapi.InfoRequest) (*registerapi.PluginInfo, error) {
	return &registerapi.PluginInfo{
		Type:              registerapi.DRAPlugin,
		Name:              e.driverName,
		Endpoint:          e.endpoint,
		SupportedVersions: e.supportedVersions,
	}, nil
}

// NotifyRegistrationStatus is the RPC invoked by plugin watcher.
func (e *registrationServer) NotifyRegistrationStatus(ctx context.Context, status *registerapi.RegistrationStatus) (*registerapi.RegistrationStatusResponse, error) {
	e.status = status
	if !status.PluginRegistered {
		return nil, fmt.Errorf("failed registration process: %+v", status.Error)
	}

	return &registerapi.RegistrationStatusResponse{}, nil
}
