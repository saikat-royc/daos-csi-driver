package metadata

import (
	"fmt"

	"cloud.google.com/go/compute/metadata"
	"sigs.k8s.io/daos-csi-driver/pkg/cloud_provider/clientset"
)

type Service interface {
	GetProjectID() string
}

type metadataServiceManager struct {
	projectID string
}

var _ Service = &metadataServiceManager{}

func NewMetadataService(clientset clientset.Interface) (Service, error) {
	projectID, err := metadata.ProjectID()
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return &metadataServiceManager{
		projectID: projectID,
	}, nil
}

func (manager *metadataServiceManager) GetProjectID() string {
	return manager.projectID
}
