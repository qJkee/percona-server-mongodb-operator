package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PerconaServerMongoDBBackupSpec defines the desired state of PerconaServerMongoDBBackup
type PerconaServerMongoDBBackupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	PSMDBCluster string `json:"psmdbCluster,omitempty"`
	StorageName  string `json:"storageName,omitempty"`
}

// PerconaServerMongoDBBackupStatus defines the observed state of PerconaServerMongoDBBackup
type PerconaServerMongoDBBackupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	Name          string               `json:"name,omitempty"`
	State         string               `json:"state,omitempty"`
	StartAt       *metav1.Time         `json:"start,omitempty"`
	CompletedAt   *metav1.Time         `json:"completed,omitempty"`
	LastScheduled *metav1.Time         `json:"lastscheduled,omitempty"`
	Destination   string               `json:"destination,omitempty"`
	StorageName   string               `json:"storageName,omitempty"`
	S3            *BackupStorageS3Spec `json:"s3,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PerconaServerMongoDBBackup is the Schema for the perconaservermongodbbackups API
// +k8s:openapi-gen=true
type PerconaServerMongoDBBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PerconaServerMongoDBBackupSpec   `json:"spec,omitempty"`
	Status PerconaServerMongoDBBackupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PerconaServerMongoDBBackupList contains a list of PerconaServerMongoDBBackup
type PerconaServerMongoDBBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PerconaServerMongoDBBackup `json:"items"`
}

type PSMDBBackupState string

const (
	BackupStarting  PSMDBBackupState = "Starting"
	BackupFailed                     = "Failed"
	BackupSucceeded                  = "Ready"
)

func init() {
	SchemeBuilder.Register(&PerconaServerMongoDBBackup{}, &PerconaServerMongoDBBackupList{})
}
