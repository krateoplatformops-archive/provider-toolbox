package v1alpha1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	Group   = "toolbox.krateo.io"
	Version = "v1alpha1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

var (
	HttpRequestKind             = reflect.TypeOf(HttpRequest{}).Name()
	HttpRequestGroupKind        = schema.GroupKind{Group: Group, Kind: HttpRequestKind}.String()
	HttpRequestKindAPIVersion   = HttpRequestKind + "." + SchemeGroupVersion.String()
	HttpRequestGroupVersionKind = SchemeGroupVersion.WithKind(HttpRequestKind)
)

func init() {
	SchemeBuilder.Register(&HttpRequest{}, &HttpRequestList{})
}
