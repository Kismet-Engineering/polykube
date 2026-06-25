package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (in *ClusterMember) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec.Labels = copyStringMap(in.Spec.Labels)
	out.Status.Conditions = append([]metav1.Condition(nil), in.Status.Conditions...)
	return &out
}

func (in *ClusterMemberList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	out.ListMeta = in.ListMeta
	out.Items = append([]ClusterMember(nil), in.Items...)
	return &out
}

func (in *Federation) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec.Members = append([]FederationMemberReference(nil), in.Spec.Members...)
	out.Status.Members = append([]FederationMemberStatus(nil), in.Status.Members...)
	out.Status.Conditions = append([]metav1.Condition(nil), in.Status.Conditions...)
	return &out
}

func (in *FederationList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	out.ListMeta = in.ListMeta
	out.Items = append([]Federation(nil), in.Items...)
	return &out
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
