// +build !ignore_autogenerated

/*
Copyright 2016 The Kubernetes Authors.

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

// This file was autogenerated by openapi-gen. Do not edit it manually!

package v1

import (
	field "k8s.io/kubernetes/pkg/util/validation/field"
	validation "k8s.io/kubernetes/pkg/validation"
)

func (s Binding) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s ComponentStatus) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s ConfigMap) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s Endpoints) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s Event) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s LimitRange) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s Namespace) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s Node) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s ObjectMeta) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	{ // Validate: Len(5,100)
		v := &validation.Len{}
		errs := v.Init(op, &validation.FieldMeta{Path: meta.Child("Name"), Type: "string"}, 5, 100)
		if len(errs) == 0 {
			errs = v.Validate(s.Name)
		}
		if len(errs) != 0 {
			allErrs = append(allErrs, errs...)
		}
	}
	{ // Validate: OneOf(this.GenerateName)
		v := &validation.OneOf{}
		errs := v.Init(op, &validation.FieldMeta{Path: meta.Child("Name"), Type: "string"}, &validation.FieldMeta{Path: meta.Child("GenerateName"), Type: "string"})
		if len(errs) == 0 {
			errs = v.Validate(s.Name, s.GenerateName)
		}
		if len(errs) != 0 {
			allErrs = append(allErrs, errs...)
		}
	}
	return allErrs
}

func (s PersistentVolume) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s PersistentVolumeClaim) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s Pod) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s PodStatusResult) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s PodTemplate) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	if errs := s.Template.Validate(&validation.FieldMeta{Path: meta.Child("Template")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s PodTemplateSpec) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s RangeAllocation) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s ReplicationController) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s ResourceQuota) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s Secret) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s Service) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}

func (s ServiceAccount) Validate(meta *validation.FieldMeta, op validation.OperationType) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := s.ObjectMeta.Validate(&validation.FieldMeta{Path: meta.Child("ObjectMeta")}, op); len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return allErrs
}
