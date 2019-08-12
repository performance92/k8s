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

package cacher

import (
	"bytes"
	"io"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/klog"
)

var (
	_ runtime.CustomEncoder = &VersionedObjectWithSerializations{}
	_ runtime.CustomEncoder = &ObjectWithSerializations{}
)

// serializationResult captures a result of serialization.
type serializationResult struct {
	// once should be used to ensure serialization is computed once.
	once sync.Once

	// raw is serialized object.
	raw []byte
	// err is error from serialization.
	err error
}

// VersionedObjectWithSerializations is caching serialization results
// of a versioned runtime.Object.
type VersionedObjectWithSerializations struct {
	// Object is object for which serializations are cached.
	// Object is assumed to be in external version.
	Object runtime.Object

	// Versioner used to convert object to its actual GroupVersion.
	Versioner runtime.GroupVersioner

	lock sync.Mutex
	// FIXME: sync.Map looks like a good usecase for us:
	// - StoreAndLoad a function that computes stuff
	// - the function that computes has sync.Once underneath.
	serializations map[runtime.Encoder]*serializationResult
}

// InterceptEncode implements runtime.CustomEncoder interface.
func (o *VersionedObjectWithSerializations) InterceptEncode(e runtime.WithVersionEncoder, w io.Writer) error {
	result := func() *serializationResult {
		// FIXME: Validate if fast path isn't necessary.
		// Addition is now fast - we just need a fast path with atomic.Value.
		// keep it in atomic.Value, Load() as fast path, only if not existing,
		// Lock() add-if-needed and Store.

		o.lock.Lock()
		defer o.lock.Unlock()

		result, exists := o.serializations[e.Encoder]
		if exists {
			return result
		}
		result = &serializationResult{}
		o.serializations[e.Encoder] = result
		return result
	}()
	result.once.Do(func() {
		buffer := bytes.NewBuffer(nil)
		result.err = e.Encoder.Encode(o.Object.DeepCopyObject(), buffer)
		result.raw = buffer.Bytes()
	})
	// Once invoked, fields of serialization will not change.
	if result.err != nil {
		return result.err
	}
	_, err := w.Write(result.raw)
	return err
}

// GetObjectKind implements runtime.Object.
func (o *VersionedObjectWithSerializations) GetObjectKind() schema.ObjectKind {
	return o.Object.GetObjectKind()
}

// DeepCopyObject implements runtime.Object.
func (o *VersionedObjectWithSerializations) DeepCopyObject() runtime.Object {
	// FIXME:
	panic("VersionedObjectWithSerializations shouldn't be deep-copied")
}

// ObjectWithSerializations is caching serialization results
// of all its versions (for which serialization happened).
//
// FIXME: Should this implement metav1.Object interface?
type ObjectWithSerializations struct {
	// Object is the object (potentially in the internal version)
	// for which serializations are cached when computed.
	Object runtime.Object

	lock sync.Mutex
	// Versioned captures serializations for each version of Object.
	//
	// FIXME: Is slice a good structure here? For majority of cases there
	// will be exactly one version, for almost all at most two. So sounds
	// fine but needs confirmation.
	//
	// FIXME: Do we need fast path for it? 
	Versioned []*VersionedObjectWithSerializations
}

func newObjectWithSerializations(object runtime.Object) *ObjectWithSerializations {
	return &ObjectWithSerializations{
		Object: object,
	}
}

// InterceptEncode implements runtime.CustomEncoder interface.
func (o *ObjectWithSerializations) InterceptEncode(e runtime.WithVersionEncoder, w io.Writer) error {
	gvks, isUnversioned, err := e.ObjectTyper.ObjectKinds(o.Object)
	if err != nil {
		klog.Errorf("AAAA: error: %v %v", err, o.Object)
		return err
	}
	encodeVersion := e.Version
	if isUnversioned {
		encodeVersion = nil
	}

	versioned, err := func() (*VersionedObjectWithSerializations, error) {
		// FIXME: Validate if fast path isn't necessary.

		o.lock.Lock()
		defer o.lock.Unlock()

		for _, object := range o.Versioned {
			if object.Versioner == e.Version {
				return object, nil
			}
		}

		versioned := &VersionedObjectWithSerializations{
			Versioner: encodeVersion,
		}
		if encodeVersion == nil {
			versioned.Object = o.Object.DeepCopyObject()
			versioned.Object.GetObjectKind().SetGroupVersionKind(gvks[0])
		} else {
			versioned.Object, err = e.ObjectConvertor.ConvertToVersion(o.Object.DeepCopyObject(), encodeVersion)
			if err != nil {
				klog.Errorf("AAAA: conversion error: %v %v", err, o.Object)
				return nil, err
			}
			// Conversion is responsible for setting the proper group, version, and kind onto the outgoing object
		}
		if ne, ok := versioned.Object.(runtime.NestedObjectEncoder); ok {
			if err := ne.EncodeNestedObjects(runtime.WithVersionEncoder{Version: encodeVersion, Encoder: e.Encoder, ObjectTyper: e.ObjectTyper}); err != nil {
				klog.Errorf("AAAA: nested encoder: %v %v", err, o.Object)
				return nil, err
			}
		}
		versioned.serializations = make(map[runtime.Encoder]*serializationResult)
		o.Versioned = append(o.Versioned, versioned)
		return versioned, nil
	}()
	if err != nil {
		return err
	}
	return e.Encoder.Encode(versioned, w)
}

// GetObjectKind implements runtime.Object.
func (o *ObjectWithSerializations) GetObjectKind() schema.ObjectKind {
	return o.Object.GetObjectKind()
}

// DeepCopyObject implements runtime.Object.
func (o *ObjectWithSerializations) DeepCopyObject() runtime.Object {
	// FIXME:
	panic("ObjectWithSerializations shouldn't be deep-copied")
}


// FIXME: Setting SelfLinks doesn't seem to work now.
