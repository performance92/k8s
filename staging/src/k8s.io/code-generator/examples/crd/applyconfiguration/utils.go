/*
Copyright The Kubernetes Authors.

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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package applyconfiguration

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
	v1 "k8s.io/code-generator/examples/crd/apis/example/v1"
	example2v1 "k8s.io/code-generator/examples/crd/apis/example2/v1"
	examplev1 "k8s.io/code-generator/examples/crd/applyconfiguration/example/v1"
	applyconfigurationexample2v1 "k8s.io/code-generator/examples/crd/applyconfiguration/example2/v1"
	internal "k8s.io/code-generator/examples/crd/applyconfiguration/internal"
)

// ForKind returns an apply configuration type for the given GroupVersionKind, or nil if no
// apply configuration type exists for the given GroupVersionKind.
func ForKind(kind schema.GroupVersionKind) interface{} {
	switch kind {
	// Group=example.crd.code-generator.k8s.io, Version=v1
	case v1.SchemeGroupVersion.WithKind("ClusterTestType"):
		return &examplev1.ClusterTestTypeApplyConfiguration{}
	case v1.SchemeGroupVersion.WithKind("ClusterTestTypeStatus"):
		return &examplev1.ClusterTestTypeStatusApplyConfiguration{}
	case v1.SchemeGroupVersion.WithKind("TestType"):
		return &examplev1.TestTypeApplyConfiguration{}
	case v1.SchemeGroupVersion.WithKind("TestTypeStatus"):
		return &examplev1.TestTypeStatusApplyConfiguration{}

		// Group=example.test.crd.code-generator.k8s.io, Version=v1
	case example2v1.SchemeGroupVersion.WithKind("TestType"):
		return &applyconfigurationexample2v1.TestTypeApplyConfiguration{}
	case example2v1.SchemeGroupVersion.WithKind("TestTypeStatus"):
		return &applyconfigurationexample2v1.TestTypeStatusApplyConfiguration{}

	}
	return nil
}

func NewTypeConverter(scheme *runtime.Scheme) *testing.TypeConverter {
	return &testing.TypeConverter{Scheme: scheme, TypeResolver: internal.Parser()}
}
