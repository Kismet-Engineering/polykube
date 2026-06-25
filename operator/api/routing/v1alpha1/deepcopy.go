package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (in *ServiceEndpoint) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec.Hostnames = append([]string(nil), in.Spec.Hostnames...)
	out.Status.Conditions = append([]metav1.Condition(nil), in.Status.Conditions...)
	out.Status.ResolvedHostnames = append([]string(nil), in.Status.ResolvedHostnames...)
	return &out
}

func (in *ServiceEndpointList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	out.ListMeta = in.ListMeta
	out.Items = append([]ServiceEndpoint(nil), in.Items...)
	return &out
}
