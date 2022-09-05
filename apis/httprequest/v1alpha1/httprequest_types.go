package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A ValueSelector is a selector for a configMap or a secret in an arbitrary namespace.
type ValueSelector struct {
	// Name of the secret.
	Name string `json:"name"`

	// Namespace of the secret.
	Namespace string `json:"namespace"`

	// The key to select.
	Key string `json:"key"`
}

type NamedValue struct {
	// Name: the name.
	Name string `json:"name"`

	// SecretRef: reference to a secret holding the value.
	// +optional
	SecretRef *ValueSelector `json:"secretRef,omitempty"`

	// ConfigMapRef: reference to a configMap holding the value.
	// +optional
	ConfigMapRef *ValueSelector `json:"configMapRef,omitempty"`

	// Value: the value.
	// +optional
	Value *string `json:"value,omitempty"`

	// Format: the value format.
	// +optional
	Format *string `json:"fmt,omitempty"`
}

type HttpRequestParams struct {
	// URL: the base url.
	// +immutable
	URL string `json:"url"`

	// Method: the request method.
	Method *string `json:"method,omitempty"`

	// Params: query string http request parameters.
	// +optional
	Params []NamedValue `json:"params,omitempty"`

	// Headers: http request headers.
	// +optional
	Headers []NamedValue `json:"headers,omitempty"`

	// WriteResponseToConfigMap: configMap to sink the http response.
	WriteResponseToConfigMap ValueSelector `json:"writeResponseToConfigMap"`
}

type HttpRequestObservation struct {
	// Target: http response content destination (CONFIGMAP, SECRET).
	Target *string `json:"target,omitempty"`

	// Name: name of target configmap or secret.
	Name *string `json:"name,omitempty"`

	// Namespace: name of target configmap or secret.
	Namespace *string `json:"namespace,omitempty"`

	// Key: name of target configmap key.
	Key *string `json:"key,omitempty"`
}

// A HttpRequestSpec defines the desired state of a GetRequest.
type HttpRequestSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       HttpRequestParams `json:"forProvider"`
}

// A HttpRequestStatus represents the observed state of a GetRequest.
type HttpRequestStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          HttpRequestObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A HttpRequest is a managed resource that represents an HTTP GET request
// +kubebuilder:printcolumn:name="TARGET",type="string",JSONPath=".status.atProvider.target"
// +kubebuilder:printcolumn:name="NAME",type="string",JSONPath=".status.atProvider.name"
// +kubebuilder:printcolumn:name="NAMESPACE",type="string",JSONPath=".status.atProvider.namespace"
// +kubebuilder:printcolumn:name="KEY",type="string",JSONPath=".status.atProvider.key"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",priority=1
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status",priority=1
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,krateo,http}
type HttpRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HttpRequestSpec   `json:"spec"`
	Status HttpRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HttpRequestList contains a list of HttpRequest.
type HttpRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HttpRequest `json:"items"`
}
