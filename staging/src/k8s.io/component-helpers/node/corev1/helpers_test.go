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

package corev1

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdatePodCondition(t *testing.T) {
	time := metav1.Now()

	podStatus := v1.PodStatus{
		Conditions: []v1.PodCondition{
			{
				Type:               v1.PodReady,
				Status:             v1.ConditionTrue,
				Reason:             "successfully",
				Message:            "sync pod successfully",
				LastProbeTime:      time,
				LastTransitionTime: metav1.NewTime(time.Add(1000)),
			},
		},
	}
	tests := []struct {
		status     *v1.PodStatus
		conditions v1.PodCondition
		expected   bool
		desc       string
	}{
		{
			status: &podStatus,
			conditions: v1.PodCondition{
				Type:               v1.PodReady,
				Status:             v1.ConditionTrue,
				Reason:             "successfully",
				Message:            "sync pod successfully",
				LastProbeTime:      time,
				LastTransitionTime: metav1.NewTime(time.Add(1000))},
			expected: false,
			desc:     "all equal, no update",
		},
		{
			status: &podStatus,
			conditions: v1.PodCondition{
				Type:               v1.PodScheduled,
				Status:             v1.ConditionTrue,
				Reason:             "successfully",
				Message:            "sync pod successfully",
				LastProbeTime:      time,
				LastTransitionTime: metav1.NewTime(time.Add(1000))},
			expected: true,
			desc:     "not equal Type, should get updated",
		},
		{
			status: &podStatus,
			conditions: v1.PodCondition{
				Type:               v1.PodReady,
				Status:             v1.ConditionFalse,
				Reason:             "successfully",
				Message:            "sync pod successfully",
				LastProbeTime:      time,
				LastTransitionTime: metav1.NewTime(time.Add(1000))},
			expected: true,
			desc:     "not equal Status, should get updated",
		},
	}

	for _, test := range tests {
		resultStatus := UpdatePodCondition(test.status, &test.conditions)

		if diff := cmp.Diff(test.expected, resultStatus); diff != "" {
			t.Fatalf("unexpected PodCondition update:\n%s", diff)
		}
	}
}
