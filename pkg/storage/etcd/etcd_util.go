/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package etcd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"

	"k8s.io/kubernetes/pkg/tools"

	etcd "github.com/coreos/etcd/client"
)

// IsEtcdNotFound returns true if and only if err is an etcd not found error.
func IsEtcdNotFound(err error) bool {
	return isEtcdErrorNum(err, tools.EtcdErrorCodeNotFound)
}

// IsEtcdNodeExist returns true if and only if err is an etcd node already exist error.
func IsEtcdNodeExist(err error) bool {
	return isEtcdErrorNum(err, tools.EtcdErrorCodeNodeExist)
}

// IsEtcdTestFailed returns true if and only if err is an etcd write conflict.
func IsEtcdTestFailed(err error) bool {
	return isEtcdErrorNum(err, tools.EtcdErrorCodeTestFailed)
}

// IsEtcdWatchStoppedByUser returns true if and only if err is a client triggered stop.
func IsEtcdWatchStoppedByUser(err error) bool {
	return tools.EtcdErrWatchStoppedByUser == err
}

// isEtcdErrorNum returns true if and only if err is an etcd error, whose errorCode matches errorCode
func isEtcdErrorNum(err error, errorCode int) bool {

	// NOTE: This only exists b/c of the plethora of weird passing 
	// that exists in the code and clients and how it has shifted in etcd 2.2
	//if 
	etcdError, ok := err.(etcd.Error) //; ok {
	return ok && etcdError.Code == errorCode
	/*} else {
		etcdError, ok := err.(*etcd.Error)
		return ok && etcdError != nil && etcdError.Code == errorCode
	}*/
}

// etcdErrorIndex returns the index associated with the error message and whether the
// index was available.
func etcdErrorIndex(err error) (uint64, bool) {
	if etcdError, ok := err.(etcd.Error); ok {
		return etcdError.Index, true
	} /*else {
		if etcdError, ok := err.(*etcd.Error); ok {
			return etcdError.Index, true
		} 
	}*/
	return 0, false
}

// GetEtcdVersion performs a version check against the provided Etcd server,
// returning the string response, and error (if any).
func GetEtcdVersion(host string) (string, error) {
	response, err := http.Get(host + "/version")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unsuccessful response from etcd server %q: %v", host, err)
	}
	versionBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	return string(versionBytes), nil
}

func startEtcd() (*exec.Cmd, error) {
	cmd := exec.Command("etcd")
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

type etcdHealth struct {
	// Note this has to be public so the json library can modify it.
	Health string `json:"health"`
}

func EtcdHealthCheck(data []byte) error {
	obj := etcdHealth{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if obj.Health != "true" {
		return fmt.Errorf("Unhealthy status: %s", obj.Health)
	}
	return nil
}
