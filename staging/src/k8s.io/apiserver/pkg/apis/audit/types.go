/*
Copyright 2017 The Kubernetes Authors.

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

package audit

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// Level defines the amount of information logged during auditing
type Level string

// Valid audit levels
const (
	// LevelNone disables auditing
	LevelNone Level = "None"
	// LevelMetadata provides the basic level of auditing.
	LevelMetadata Level = "Metadata"
	// LevelRequestObject provides Metadata level of auditing, and additionally
	// logs the request object (does not apply for non-resource requests).
	LevelRequestObject Level = "RequestObject"
	// LevelResponseObject provides RequestObject level of auditing, and additionally
	// logs the response object (does not apply for non-resource requests).
	LevelResponseObject Level = "ResponseObject"
)

// Event captures all the information that can be included in an API audit log.
type Event struct {
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// AuditLevel at which event was generated
	Level Level

	// Time the request reached the apiserver.
	Timestamp metav1.Time
	// Unique audit ID, generated for each request.
	// +optional
	AuditID types.UID
	// RequestURI is the request URI as sent by the client to a server.
	RequestURI string
	// Verb is the kubernetes verb associated with the request.
	// For non-resource requests, this is identical to HttpMethod.
	Verb string
	// Authenticated user information.
	User UserInfo
	// Impersonated user information.
	// +optional
	Impersonate *UserInfo
	// Source IP, from where the request originates.
	// +optional
	SourceIP string
	// Object reference this request is targeted at.
	// Does not apply for List-type requests, or non-resource requests.
	// +optional
	ObjectRef *ObjectReference
	// The response status, populated even when the ResponseObject is not a Status type.
	// For successful responses, this will only include the Code and StatusSuccess.
	// For non-status type error responses, this will be auto-populated with the error Message.
	// +optional
	ResponseStatus *metav1.Status

	// API object from the request. The RequestObject is recorded as-is in the request, prior to
	// version conversion, defaulting, admission or merging. It is an external versioned object type,
	// and may not be a valid object on its own.  Omitted for non-resource requests.  Only logged at
	// RequestObject Level and higher.
	// +optional
	RequestObject runtime.Object
	// API object returned in the response. The ResponseObject is recorded after conversion to the
	// external type.  Omitted for non-resource requests.  Only logged at ResponseObject Level and
	// higher.
	// +optional
	ResponseObject runtime.Object
}

// EventList is a list of audit Events.
type EventList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	Items []Event
}

// Policy defines the configuration of audit logging, and the rules for how different request
// categories are logged.
type Policy struct {
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// Rules specify the audit Level a request should be recorded at.
	// A request may match multiple rules, in which case the FIRST matching rule is used.
	// The default audit level is None, but can be overridden by a catch-all rule at the end of the list.
	Rules []PolicyRule
}

// PolicyRule maps requests based off metadata to an audit Level.
// Requests must match the rules of every field (an intersection of rules).
type PolicyRule struct {
	// The Level that requests matching this rule are recorded at.
	Level Level

	// The users (by authenticated user name) this rule applies to.
	// An empty list implies every user.
	// +optional
	Users []string
	// The user groups this rule applies to. If a user is considered matching
	// if the are a member of any of these groups
	// An empty list implies every user group.
	// +optional
	UserGroups []string

	// The verbs that match this rule.
	// An empty list implies every verb.
	// +optional
	Verbs []string

	// Rules can apply to API resources (such as "pods" or "secrets"),
	// non-resource URL paths (such as "/api"), or neither, but not both.
	// If neither is specified, the rule is treated as a default for all URLs.

	// Resource kinds that this rule matches. An empty list implies all kinds in all API groups.
	// +optional
	ResourceKinds []GroupKinds
	// Namespaces that this rule matches.
	// The empty string "" matches non-namespaced resources.
	// An empty list implies every namespace.
	// +optional
	Namespaces []string

	// NonResourceURLs is a set of URL paths that should be audited.
	// *s are allowed, but only as the full, final step in the path.
	// Examples:
	//  "/metrics" - Log requests for apiserver metrics
	//  "/healthz*" - Log all health checks
	// +optional
	NonResourceURLs []string
}

// PolicyList is a list of audit Policies.
type PolicyList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	Items []Policy
}

// GroupKinds represents resource kinds in an API group.
type GroupKinds struct {
	// Group is the name of the API group that contains the resources.
	// The empty string represents the core API group.
	// +optional
	Group string
	// Kinds is a list of kinds of resources within the API group.
	// Any empty list implies every resource kind in the API group.
	// +optional
	Kinds []string
}

// ObjectReference contains enough information to let you inspect or modify the referred object.
type ObjectReference struct {
	// +optional
	Kind string
	// +optional
	Namespace string
	// +optional
	Name string
	// +optional
	UID types.UID
	// +optional
	APIVersion string
	// +optional
	ResourceVersion string
}

// UserInfo holds the information about the user needed to implement the
// user.Info interface.
type UserInfo struct {
	// The name that uniquely identifies this user among all active users.
	Username string
	// A unique value that identifies this user across time. If this user is
	// deleted and another user by the same name is added, they will have
	// different UIDs.
	UID string
	// The names of groups this user is a part of.
	Groups []string
	// Any additional information provided by the authenticator.
	Extra map[string]ExtraValue
}

// ExtraValue masks the value so protobuf can generate
type ExtraValue []string
