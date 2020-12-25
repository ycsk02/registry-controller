/*
Copyright 2020.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RegistrySpec defines the desired state of Registry

type RegistryServer struct {
	Server   string `json:"server,omitempty"`
	UserName string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
}

// RegistrySpec defines the desired state of Registry
type RegistrySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	SecretName      string           `json:"secretname,omitempty"`
	TargetNamespace []string         `json:"targetnamespace,omitempty"`
	RegistryServers []RegistryServer `json:"registryservers,omitempty"`
}

// RegistryStatus defines the observed state of Registry
type RegistryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Secret",type=string,JSONPath=`.spec.secretname`
// +kubebuilder:printcolumn:name="TargetNamespace",type=string,JSONPath=`.spec.targetnamespace`

// Registry is the Schema for the registries API
type Registry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistrySpec   `json:"spec,omitempty"`
	Status RegistryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RegistryList contains a list of Registry
type RegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Registry `json:"items"`
}

// {"auths":{"https://192.168.1.111":{"username":"sukai","password":"sukai","email":"ycsk02@hotmail.com",
// 	"auth":"c3VrYWk6c3VrYWk="}}}

// Auths struct contains an embedded RegistriesStruct of name auths
type Auths struct {
	Registries RegistriesStruct `json:"auths"`
}

// RegistriesStruct is a map of registries to their credentials
type RegistriesStruct map[string]RegistryCredentials

// RegistryCredentials defines the fields stored per registry in an docker config secret
type RegistryCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Auth     string `json:"auth"`
}

func init() {
	SchemeBuilder.Register(&Registry{}, &RegistryList{})
}
