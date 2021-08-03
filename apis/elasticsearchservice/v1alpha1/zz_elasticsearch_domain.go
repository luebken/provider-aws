/*
Copyright 2021 The Crossplane Authors.

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

// Code generated by ack-generate. DO NOT EDIT.

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ElasticsearchDomainParameters defines the desired state of ElasticsearchDomain
type ElasticsearchDomainParameters struct {
	// Region is which region the ElasticsearchDomain will be created.
	// +kubebuilder:validation:Required
	Region string `json:"region"`
	// IAM access policy as a JSON-formatted string.
	AccessPolicies *string `json:"accessPolicies,omitempty"`
	// Option to allow references to indices in an HTTP request body. Must be false
	// when configuring access to individual sub-resources. By default, the value
	// is true. See Configuration Advanced Options (http://docs.aws.amazon.com/elasticsearch-service/latest/developerguide/es-createupdatedomains.html#es-createdomain-configure-advanced-options)
	// for more information.
	AdvancedOptions map[string]*string `json:"advancedOptions,omitempty"`
	// Specifies advanced security options.
	AdvancedSecurityOptions *AdvancedSecurityOptionsInput `json:"advancedSecurityOptions,omitempty"`
	// Options to specify the Cognito user and identity pools for Kibana authentication.
	// For more information, see Amazon Cognito Authentication for Kibana (http://docs.aws.amazon.com/elasticsearch-service/latest/developerguide/es-cognito-auth.html).
	CognitoOptions *CognitoOptions `json:"cognitoOptions,omitempty"`
	// Options to specify configuration that will be applied to the domain endpoint.
	DomainEndpointOptions *DomainEndpointOptions `json:"domainEndpointOptions,omitempty"`
	// The name of the Elasticsearch domain that you are creating. Domain names
	// are unique across the domains owned by an account within an AWS region. Domain
	// names must start with a lowercase letter and can contain the following characters:
	// a-z (lowercase), 0-9, and - (hyphen).
	// +kubebuilder:validation:Required
	DomainName *string `json:"domainName"`
	// Options to enable, disable and specify the type and size of EBS storage volumes.
	EBSOptions *EBSOptions `json:"ebsOptions,omitempty"`
	// Configuration options for an Elasticsearch domain. Specifies the instance
	// type and number of instances in the domain cluster.
	ElasticsearchClusterConfig *ElasticsearchClusterConfig `json:"elasticsearchClusterConfig,omitempty"`
	// String of format X.Y to specify version for the Elasticsearch domain eg.
	// "1.5" or "2.3". For more information, see Creating Elasticsearch Domains
	// (http://docs.aws.amazon.com/elasticsearch-service/latest/developerguide/es-createupdatedomains.html#es-createdomains)
	// in the Amazon Elasticsearch Service Developer Guide.
	ElasticsearchVersion *string `json:"elasticsearchVersion,omitempty"`
	// Specifies the Encryption At Rest Options.
	EncryptionAtRestOptions *EncryptionAtRestOptions `json:"encryptionAtRestOptions,omitempty"`
	// Map of LogType and LogPublishingOption, each containing options to publish
	// a given type of Elasticsearch log.
	LogPublishingOptions map[string]*LogPublishingOption `json:"logPublishingOptions,omitempty"`
	// Specifies the NodeToNodeEncryptionOptions.
	NodeToNodeEncryptionOptions *NodeToNodeEncryptionOptions `json:"nodeToNodeEncryptionOptions,omitempty"`
	// Option to set time, in UTC format, of the daily automated snapshot. Default
	// value is 0 hours.
	SnapshotOptions *SnapshotOptions `json:"snapshotOptions,omitempty"`
	// Options to specify the subnets and security groups for VPC endpoint. For
	// more information, see Creating a VPC (http://docs.aws.amazon.com/elasticsearch-service/latest/developerguide/es-vpc.html#es-creating-vpc)
	// in VPC Endpoints for Amazon Elasticsearch Service Domains
	VPCOptions                          *VPCOptions `json:"vpcOptions,omitempty"`
	CustomElasticsearchDomainParameters `json:",inline"`
}

// ElasticsearchDomainSpec defines the desired state of ElasticsearchDomain
type ElasticsearchDomainSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ElasticsearchDomainParameters `json:"forProvider"`
}

// ElasticsearchDomainObservation defines the observed state of ElasticsearchDomain
type ElasticsearchDomainObservation struct {
	// The Amazon resource name (ARN) of an Elasticsearch domain. See Identifiers
	// for IAM Entities (http://docs.aws.amazon.com/IAM/latest/UserGuide/index.html?Using_Identifiers.html)
	// in Using AWS Identity and Access Management for more information.
	ARN *string `json:"arn,omitempty"`
	// The domain creation status. True if the creation of an Elasticsearch domain
	// is complete. False if domain creation is still in progress.
	Created *bool `json:"created,omitempty"`
	// The domain deletion status. True if a delete request has been received for
	// the domain but resource cleanup is still in progress. False if the domain
	// has not been deleted. Once domain deletion is complete, the status of the
	// domain is no longer returned.
	Deleted *bool `json:"deleted,omitempty"`
	// The unique identifier for the specified Elasticsearch domain.
	DomainID *string `json:"domainID,omitempty"`
	// The Elasticsearch domain endpoint that you use to submit index and search
	// requests.
	Endpoint *string `json:"endpoint,omitempty"`
	// Map containing the Elasticsearch domain endpoints used to submit index and
	// search requests. Example key, value: 'vpc','vpc-endpoint-h2dsd34efgyghrtguk5gt6j2foh4.us-east-1.es.amazonaws.com'.
	Endpoints map[string]*string `json:"endpoints,omitempty"`
	// The status of the Elasticsearch domain configuration. True if Amazon Elasticsearch
	// Service is processing configuration changes. False if the configuration is
	// active.
	Processing *bool `json:"processing,omitempty"`
	// The current status of the Elasticsearch domain's service software.
	ServiceSoftwareOptions *ServiceSoftwareOptions `json:"serviceSoftwareOptions,omitempty"`
	// The status of an Elasticsearch domain version upgrade. True if Amazon Elasticsearch
	// Service is undergoing a version upgrade. False if the configuration is active.
	UpgradeProcessing *bool `json:"upgradeProcessing,omitempty"`
}

// ElasticsearchDomainStatus defines the observed state of ElasticsearchDomain.
type ElasticsearchDomainStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ElasticsearchDomainObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// ElasticsearchDomain is the Schema for the ElasticsearchDomains API
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,aws}
type ElasticsearchDomain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ElasticsearchDomainSpec   `json:"spec"`
	Status            ElasticsearchDomainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ElasticsearchDomainList contains a list of ElasticsearchDomains
type ElasticsearchDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElasticsearchDomain `json:"items"`
}

// Repository type metadata.
var (
	ElasticsearchDomainKind             = "ElasticsearchDomain"
	ElasticsearchDomainGroupKind        = schema.GroupKind{Group: Group, Kind: ElasticsearchDomainKind}.String()
	ElasticsearchDomainKindAPIVersion   = ElasticsearchDomainKind + "." + GroupVersion.String()
	ElasticsearchDomainGroupVersionKind = GroupVersion.WithKind(ElasticsearchDomainKind)
)

func init() {
	SchemeBuilder.Register(&ElasticsearchDomain{}, &ElasticsearchDomainList{})
}
