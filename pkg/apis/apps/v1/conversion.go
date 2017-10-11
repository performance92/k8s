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

package v1

import (
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8s_api_v1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	// Add non-generated conversion functions to handle the *int32 -> int32
	// conversion. A pointer is useful in the versioned type so we can default
	// it, but a plain int32 is more convenient in the internal type. These
	// functions are the same as the autogenerated ones in every other way.
	err := scheme.AddConversionFuncs(
		Convert_extensions_RollingUpdateDaemonSet_To_v1_RollingUpdateDaemonSet,
		Convert_v1_RollingUpdateDaemonSet_To_extensions_RollingUpdateDaemonSet,
		Convert_extensions_DaemonSet_To_v1_DaemonSet,
		Convert_v1_DaemonSet_To_extensions_DaemonSet,
		Convert_extensions_DaemonSetSpec_To_v1_DaemonSetSpec,
		Convert_v1_DaemonSetSpec_To_extensions_DaemonSetSpec,
		Convert_extensions_DaemonSetUpdateStrategy_To_v1_DaemonSetUpdateStrategy,
		Convert_v1_DaemonSetUpdateStrategy_To_extensions_DaemonSetUpdateStrategy,
		Convert_extensions_ReplicaSetSpec_To_v1_ReplicaSetSpec,
		Convert_v1_ReplicaSetSpec_To_extensions_ReplicaSetSpec,
		Convert_extensions_ScaleStatus_To_v1_ScaleStatus,
		Convert_v1_ScaleStatus_To_extensions_ScaleStatus,
	)
	if err != nil {
		return err
	}
	return nil
}

func Convert_extensions_RollingUpdateDaemonSet_To_v1_RollingUpdateDaemonSet(in *extensions.RollingUpdateDaemonSet, out *appsv1.RollingUpdateDaemonSet, s conversion.Scope) error {
	if out.MaxUnavailable == nil {
		out.MaxUnavailable = &intstr.IntOrString{}
	}
	if err := s.Convert(&in.MaxUnavailable, out.MaxUnavailable, 0); err != nil {
		return err
	}
	return nil
}

func Convert_v1_RollingUpdateDaemonSet_To_extensions_RollingUpdateDaemonSet(in *appsv1.RollingUpdateDaemonSet, out *extensions.RollingUpdateDaemonSet, s conversion.Scope) error {
	if err := s.Convert(in.MaxUnavailable, &out.MaxUnavailable, 0); err != nil {
		return err
	}
	return nil
}

func Convert_extensions_DaemonSet_To_v1_DaemonSet(in *extensions.DaemonSet, out *appsv1.DaemonSet, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if out.Annotations == nil {
		out.Annotations = make(map[string]string)
	}
	out.Annotations[appsv1.DeprecatedTemplateGeneration] = strconv.FormatInt(in.Spec.TemplateGeneration, 10)
	if err := Convert_extensions_DaemonSetSpec_To_v1_DaemonSetSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
		return err
	}
	return nil
}

func Convert_extensions_DaemonSetSpec_To_v1_DaemonSetSpec(in *extensions.DaemonSetSpec, out *appsv1.DaemonSetSpec, s conversion.Scope) error {
	out.Selector = in.Selector
	if err := k8s_api_v1.Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(&in.Template, &out.Template, s); err != nil {
		return err
	}
	if err := Convert_extensions_DaemonSetUpdateStrategy_To_v1_DaemonSetUpdateStrategy(&in.UpdateStrategy, &out.UpdateStrategy, s); err != nil {
		return err
	}
	out.MinReadySeconds = int32(in.MinReadySeconds)
	if in.RevisionHistoryLimit != nil {
		out.RevisionHistoryLimit = new(int32)
		*out.RevisionHistoryLimit = *in.RevisionHistoryLimit
	} else {
		out.RevisionHistoryLimit = nil
	}
	return nil
}

func Convert_extensions_DaemonSetUpdateStrategy_To_v1_DaemonSetUpdateStrategy(in *extensions.DaemonSetUpdateStrategy, out *appsv1.DaemonSetUpdateStrategy, s conversion.Scope) error {
	out.Type = appsv1.DaemonSetUpdateStrategyType(in.Type)
	if in.RollingUpdate != nil {
		out.RollingUpdate = &appsv1.RollingUpdateDaemonSet{}
		if err := Convert_extensions_RollingUpdateDaemonSet_To_v1_RollingUpdateDaemonSet(in.RollingUpdate, out.RollingUpdate, s); err != nil {
			return err
		}
	}
	return nil
}

func Convert_v1_DaemonSet_To_extensions_DaemonSet(in *appsv1.DaemonSet, out *extensions.DaemonSet, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1_DaemonSetSpec_To_extensions_DaemonSetSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if value, ok := in.Annotations[appsv1.DeprecatedTemplateGeneration]; ok {
		if value64, err := strconv.ParseInt(value, 10, 64); err != nil {
			return err
		} else {
			out.Spec.TemplateGeneration = value64
			delete(out.Annotations, appsv1.DeprecatedTemplateGeneration)
		}
	}
	if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
		return err
	}
	return nil
}

func Convert_v1_DaemonSetSpec_To_extensions_DaemonSetSpec(in *appsv1.DaemonSetSpec, out *extensions.DaemonSetSpec, s conversion.Scope) error {
	out.Selector = in.Selector
	if err := k8s_api_v1.Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(&in.Template, &out.Template, s); err != nil {
		return err
	}
	if err := Convert_v1_DaemonSetUpdateStrategy_To_extensions_DaemonSetUpdateStrategy(&in.UpdateStrategy, &out.UpdateStrategy, s); err != nil {
		return err
	}
	if in.RevisionHistoryLimit != nil {
		out.RevisionHistoryLimit = new(int32)
		*out.RevisionHistoryLimit = *in.RevisionHistoryLimit
	} else {
		out.RevisionHistoryLimit = nil
	}
	out.MinReadySeconds = in.MinReadySeconds
	return nil
}

func Convert_v1_DaemonSetUpdateStrategy_To_extensions_DaemonSetUpdateStrategy(in *appsv1.DaemonSetUpdateStrategy, out *extensions.DaemonSetUpdateStrategy, s conversion.Scope) error {
	out.Type = extensions.DaemonSetUpdateStrategyType(in.Type)
	if in.RollingUpdate != nil {
		out.RollingUpdate = &extensions.RollingUpdateDaemonSet{}
		if err := Convert_v1_RollingUpdateDaemonSet_To_extensions_RollingUpdateDaemonSet(in.RollingUpdate, out.RollingUpdate, s); err != nil {
			return err
		}
	}
	return nil
}

func Convert_extensions_ReplicaSetSpec_To_v1_ReplicaSetSpec(in *extensions.ReplicaSetSpec, out *appsv1.ReplicaSetSpec, s conversion.Scope) error {
	out.Replicas = new(int32)
	*out.Replicas = int32(in.Replicas)
	out.MinReadySeconds = in.MinReadySeconds
	out.Selector = in.Selector
	if err := k8s_api_v1.Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(&in.Template, &out.Template, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_ReplicaSetSpec_To_extensions_ReplicaSetSpec(in *appsv1.ReplicaSetSpec, out *extensions.ReplicaSetSpec, s conversion.Scope) error {
	if in.Replicas != nil {
		out.Replicas = *in.Replicas
	}
	out.MinReadySeconds = in.MinReadySeconds
	out.Selector = in.Selector
	if err := k8s_api_v1.Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(&in.Template, &out.Template, s); err != nil {
		return err
	}
	return nil
}

func Convert_extensions_ScaleStatus_To_v1_ScaleStatus(in *extensions.ScaleStatus, out *appsv1.ScaleStatus, s conversion.Scope) error {
	out.Replicas = int32(in.Replicas)

	out.Selector = nil
	out.TargetSelector = ""
	if in.Selector != nil {
		if in.Selector.MatchExpressions == nil || len(in.Selector.MatchExpressions) == 0 {
			out.Selector = in.Selector.MatchLabels
		}

		selector, err := metav1.LabelSelectorAsSelector(in.Selector)
		if err != nil {
			return fmt.Errorf("invalid label selector: %v", err)
		}
		out.TargetSelector = selector.String()
	}
	return nil
}

func Convert_v1_ScaleStatus_To_extensions_ScaleStatus(in *appsv1.ScaleStatus, out *extensions.ScaleStatus, s conversion.Scope) error {
	out.Replicas = in.Replicas

	// Normally when 2 fields map to the same internal value we favor the old field, since
	// old clients can't be expected to know about new fields but clients that know about the
	// new field can be expected to know about the old field (though that's not quite true, due
	// to kubectl apply). However, these fields are readonly, so any non-nil value should work.
	if in.TargetSelector != "" {
		labelSelector, err := metav1.ParseToLabelSelector(in.TargetSelector)
		if err != nil {
			out.Selector = nil
			return fmt.Errorf("failed to parse target selector: %v", err)
		}
		out.Selector = labelSelector
	} else if in.Selector != nil {
		out.Selector = new(metav1.LabelSelector)
		selector := make(map[string]string)
		for key, val := range in.Selector {
			selector[key] = val
		}
		out.Selector.MatchLabels = selector
	} else {
		out.Selector = nil
	}
	return nil
}
