package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
)

func (in *Workload) DeepCopyObject() kruntime.Object {
	if in == nil {
		return nil
	}
	out := *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec.ImagePullSecrets = append([]LocalObjectReference(nil), in.Spec.ImagePullSecrets...)
	out.Spec.Ports = append([]ContainerPort(nil), in.Spec.Ports...)
	out.Spec.Env = append([]EnvVar(nil), in.Spec.Env...)
	out.Spec.EnvFrom = append([]EnvFromSource(nil), in.Spec.EnvFrom...)
	out.Status.Conditions = append([]metav1.Condition(nil), in.Status.Conditions...)
	out.Status.Targets = append([]WorkloadTargetStatus(nil), in.Status.Targets...)
	return &out
}

func (in *WorkloadList) DeepCopyObject() kruntime.Object {
	if in == nil {
		return nil
	}
	out := *in
	out.ListMeta = in.ListMeta
	out.Items = append([]Workload(nil), in.Items...)
	return &out
}
