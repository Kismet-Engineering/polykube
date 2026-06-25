package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (in *DatastoreBinding) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Status.Conditions = append([]metav1.Condition(nil), in.Status.Conditions...)
	return &out
}

func (in *DatastoreBindingList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	out.ListMeta = in.ListMeta
	out.Items = append([]DatastoreBinding(nil), in.Items...)
	return &out
}
