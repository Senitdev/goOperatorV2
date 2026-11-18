/*
Copyright 2026.

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

package v2beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GoBackendSpec defines the desired state of GoBackend
type GoBackendSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	Replicas int32  `json:"replicas,omitempty"`
	Port     int32  `json:"port,omitempty"`
	Image    string `json:"image,omitempty"`
	// kubebuilder:default:="postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	// DatabaseUrl string `json:"databaseUrl"`
	// DatabaseRef est le nom de la ressource de type Database qui sera utilisée par le backend
	DatabaseRef corev1.SecretKeySelector `json:"databaseRef"`
	// L'url pour se connecter a Ollama, par example: http://
	OllamaUrl string `json:"ollamaUrl"`
	// +kubebuilder:default:="nomic-embed-text"
	EmbeddingModel string `json:"embeddingModel,omitempty"`
	// +kubebuilder:default:="qwen2.5:1.5b"
	LlmModel string `json:"llmModel,omitempty"`
	// AllowedOrigin correspond à l'origine CORS autorisée (ex: http://127.0.0.1:3000)
	// +kubebuilder:default:="http://127.0.0.1:3000"
	AllowedOrigin string              `json:"allowedOrigin,omitempty"`
	Autoscaling   *AutoscallingConfig `json:"autoscaling,omitempty"`
	Storage       StorageConfig       `json:"storage"`
}

// GoBackendStatus defines the observed state of GoBackend.
type GoBackendStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the GoBackend resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// definition class storage pour manipuler le PVC
type StorageConfig struct {
	// +kubebuilder:default:="1Gi"
	// +kubebuilder:validation:MinLength=1
	Size string `json:"size,omitempty"`
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadWriteMany;ReadOnlyMany
	// +kubebuilder:default:="ReadWriteOnce"
	AccessMode corev1.PersistentVolumeAccessMode `json:"accessMode,omitempty"`
	// storage facultative
	StorageClassName *string `json:"storageClassName,omitempty"`
}
type AutoscallingConfig struct {
	Enabled                        bool  `json:"enabled,omitempty"`
	MinReplicas                    int32 `json:"minReplicas,omitempty"`
	MaxReplicas                    int32 `json:"maxReplicas,omitempty"`
	TargetCPUUtilizationPercentage int32 `json:"targetCPUUtilizationPercentage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// GoBackend is the Schema for the gobackends API
type GoBackend struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of GoBackend
	// +required
	Spec GoBackendSpec `json:"spec"`

	// status defines the observed state of GoBackend
	// +optional
	Status GoBackendStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// GoBackendList contains a list of GoBackend
type GoBackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []GoBackend `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &GoBackend{}, &GoBackendList{})
		return nil
	})
}
