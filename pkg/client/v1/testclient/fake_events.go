/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package testclient

import (
	v1api "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
)

// FakeEvents implements EventInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the method you want to test easier.
type FakeEvents struct {
	Fake *Fake
}

// Create makes a new event. Returns the copy of the event the server returns, or an error.
func (c *FakeEvents) Create(event *v1api.Event) (*v1api.Event, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-event", Value: event.Name}, &v1api.Event{})
	return obj.(*v1api.Event), err
}

// Update replaces an existing event. Returns the copy of the event the server returns, or an error.
func (c *FakeEvents) Update(event *v1api.Event) (*v1api.Event, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-event", Value: event.Name}, &v1api.Event{})
	return obj.(*v1api.Event), err
}

// List returns a list of events matching the selectors.
func (c *FakeEvents) List(label labels.Selector, field fields.Selector) (*v1api.EventList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-events"}, &v1api.EventList{})
	return obj.(*v1api.EventList), err
}

// Get returns the given event, or an error.
func (c *FakeEvents) Get(id string) (*v1api.Event, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-endpoints", Value: id}, &v1api.Event{})
	return obj.(*v1api.Event), err
}

// Watch starts watching for events matching the given selectors.
func (c *FakeEvents) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-events", Value: resourceVersion})
	return c.Fake.Watch, c.Fake.Err
}

// Search returns a list of events matching the specified object.
func (c *FakeEvents) Search(objOrRef runtime.Object) (*v1api.EventList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "search-events"}, &v1api.EventList{})
	return obj.(*v1api.EventList), err
}

func (c *FakeEvents) Delete(name string) error {
	_, err := c.Fake.Invokes(FakeAction{Action: "delete-event", Value: name}, &v1api.Event{})
	return err
}

func (c *FakeEvents) GetFieldSelector(involvedObjectName, involvedObjectNamespace, involvedObjectKind, involvedObjectUID *string) fields.Selector {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-field-selector"})
	return fields.Everything()
}
