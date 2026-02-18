package database

import (
	"os"
	"testing"
	"time"

	"github.com/truefoundry/cruisekube/pkg/ports"
	"github.com/truefoundry/cruisekube/pkg/types"
)

func TestSQLiteStorage(t *testing.T) {
	// Create a temporary database file
	dbPath := "./test_cruisekube.db"
	defer os.Remove(dbPath)

	config := DatabaseConfig{
		Type:     "sqlite",
		Database: dbPath,
	}

	storage, err := NewDatabase(config)
	if err != nil {
		t.Fatalf("Failed to create SQLite storage: %v", err)
	}
	defer storage.Close()

	testStorage(t, storage)
}

func testStorage(t *testing.T, storage ports.Database) {
	clusterID := "test-cluster"
	workloadID := "test-workload"

	// Test UpsertStat
	stat := types.WorkloadStat{
		WorkloadIdentifier:         workloadID,
		Kind:                       "Deployment",
		Namespace:                  "default",
		Name:                       "test-app",
		CreationTime:               time.Now(),
		UpdatedAt:                  time.Now(),
		Replicas:                   1,
		EvictionRanking:            types.EvictionRankingMedium,
		ContainerStats:             []types.ContainerStats{},
		OriginalContainerResources: []types.OriginalContainerResources{},
	}

	err := storage.UpsertStat(clusterID, workloadID, stat, time.Now())
	if err != nil {
		t.Fatalf("Failed to upsert stat: %v", err)
	}

	// Test HasStatForWorkload
	exists, err := storage.HasStatForWorkload(clusterID, workloadID)
	if err != nil {
		t.Fatalf("Failed to check if stat exists: %v", err)
	}
	if !exists {
		t.Error("Expected stat to exist after upsert")
	}

	// Test GetStatCountForCluster
	count, err := storage.GetStatCountForCluster(clusterID)
	if err != nil {
		t.Fatalf("Failed to get stat count: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count to be 1, got %d", count)
	}

	// Test GetStatForWorkload
	retrievedStat, err := storage.GetStatForWorkload(clusterID, workloadID)
	if err != nil {
		t.Fatalf("Failed to get stat: %v", err)
	}
	if retrievedStat.WorkloadIdentifier != workloadID {
		t.Errorf("Expected workload ID %s, got %s", workloadID, retrievedStat.WorkloadIdentifier)
	}
	if retrievedStat.Kind != "Deployment" {
		t.Errorf("Expected kind Deployment, got %s", retrievedStat.Kind)
	}

	// Test DeleteStatForWorkload
	err = storage.DeleteStatForWorkload(clusterID, workloadID)
	if err != nil {
		t.Fatalf("Failed to delete stat: %v", err)
	}

	// Verify deletion
	exists, err = storage.HasStatForWorkload(clusterID, workloadID)
	if err != nil {
		t.Fatalf("Failed to check if stat exists after deletion: %v", err)
	}
	if exists {
		t.Error("Expected stat to not exist after deletion")
	}

	rows := []types.PodResourceRecommendationRow{
		{WorkloadID: "Deployment/default/test-app/nginx", NodeName: "node-1", Namespace: "default", Pod: "test-app-abc", Container: "nginx", Recommendation: `{"cpu_request":0.5,"memory_request":512,"cpu_limit":1,"memory_limit":1024,"to_be_evicted":false}`},
		{WorkloadID: "Deployment/default/test-app/sidecar", NodeName: "node-1", Namespace: "default", Pod: "test-app-abc", Container: "sidecar", Recommendation: ""},
	}
	err = storage.SavePodRecommendations(clusterID, rows)
	if err != nil {
		t.Fatalf("Failed to save pod recommendations: %v", err)
	}
	err = storage.SavePodRecommendations(clusterID, nil)
	if err == nil {
		t.Error("SavePodRecommendations with nil rows should return error")
	}
}
