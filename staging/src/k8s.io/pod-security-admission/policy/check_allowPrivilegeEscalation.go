/*
Copyright 2021 The Kubernetes Authors.

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

package policy

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/pod-security-admission/api"
)

/*
Privilege escalation (such as via set-user-ID or set-group-ID file mode) should not be allowed.

**Restricted Fields:**

spec.containers[*].securityContext.allowPrivilegeEscalation
spec.initContainers[*].securityContext.allowPrivilegeEscalation

**Allowed Values:** false
*/

func init() {
	addCheck(CheckAllowPrivilegeEscalation)
}

// CheckAllowPrivilegeEscalation returns a restricted level check
// that requires allowPrivilegeEscalation=false in 1.8+
func CheckAllowPrivilegeEscalation() Check {
	return Check{
		ID:    "allowPrivilegeEscalation",
		Level: api.LevelRestricted,
		Versions: []VersionedCheck{
			{
				// Field added in 1.8:
				// https://github.com/kubernetes/kubernetes/blob/v1.8.0/staging/src/k8s.io/api/core/v1/types.go#L4797-L4804
				MinimumVersion: api.MajorMinorVersion(1, 8),
				CheckPod:       withOptions(allowPrivilegeEscalationV1Dot8),
			},
			{
				// Starting 1.25, windows pods would be exempted from this check using pod.spec.os field when set to windows.
				MinimumVersion: api.MajorMinorVersion(1, 25),
				CheckPod:       withOptions(allowPrivilegeEscalationV1Dot25),
			},
		},
	}
}

func allowPrivilegeEscalationV1Dot8(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec, opts options) CheckResult {
	badContainers := NewViolations(opts.withFieldErrors)

	visitContainers(podSpec, opts, func(container *corev1.Container, path *field.Path) {
		if opts.withFieldErrors {
			path = path.Child("securityContext", "allowPrivilegeEscalation")
			if container.SecurityContext == nil {
				badContainers.Add(container.Name, required(path))
			} else if container.SecurityContext.AllowPrivilegeEscalation == nil {
				badContainers.Add(container.Name, withBadValue(forbidden(path), "nil"))
			} else if *container.SecurityContext.AllowPrivilegeEscalation {
				badContainers.Add(container.Name, withBadValue(forbidden(path), true))
			}
		} else if container.SecurityContext == nil || container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation {
			badContainers.Add(container.Name)
		}
	})

	if !badContainers.Empty() {
		return CheckResult{
			Allowed:         false,
			ForbiddenReason: "allowPrivilegeEscalation != false",
			ForbiddenDetail: fmt.Sprintf(
				"%s %s must set securityContext.allowPrivilegeEscalation=false",
				pluralize("container", "containers", badContainers.Len()),
				joinQuote(badContainers.Data()),
			),
			ErrList: badContainers.Errs(),
		}
	}
	return CheckResult{Allowed: true}
}

func allowPrivilegeEscalationV1Dot25(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec, opts options) CheckResult {
	// Pod API validation would have failed if podOS == Windows and if privilegeEscalation has been set.
	// We can admit the Windows pod even if privilegeEscalation has not been set.
	if podSpec.OS != nil && podSpec.OS.Name == corev1.Windows {
		return CheckResult{Allowed: true}
	}
	return allowPrivilegeEscalationV1Dot8(podMetadata, podSpec, opts)
}
