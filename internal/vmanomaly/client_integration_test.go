//go:build integration

package vmanomaly_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
)

func getTestClient(t *testing.T) *vmanomaly.Client {
	t.Helper()

	endpoint := os.Getenv("VMANOMALY_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8490"
	}

	client := vmanomaly.NewClient(endpoint, "", nil)

	waitForServer(t, client)

	return client
}

func waitForServer(t *testing.T, client *vmanomaly.Client) {
	t.Helper()

	ctx := context.Background()
	maxRetries := 20
	retryDelay := 500 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		_, err := client.GetHealth(ctx)
		if err == nil {
			return
		}

		if i == maxRetries-1 {
			t.Fatalf("server not ready after %d retries: %v", maxRetries, err)
		}

		time.Sleep(retryDelay)
	}
}

func TestIntegration_GetHealth(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	result, err := client.GetHealth(ctx)
	if err != nil {
		t.Fatalf("GetHealth() failed: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("status = %v, want ok", result["status"])
	}
}

func TestIntegration_GetBuildInfo(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	result, err := client.GetBuildInfo(ctx)
	if err != nil {
		t.Fatalf("GetBuildInfo() failed: %v", err)
	}

	if result["vmanomaly"] == nil {
		t.Error("expected vmanomaly field in build info")
	}
}

func TestIntegration_ListModels(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	result, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels() failed: %v", err)
	}

	if len(result.Models) == 0 {
		t.Error("expected at least one model")
	}

	expectedModels := []string{"zscore_online", "mad_online", "temporal_envelope"}
	for _, expected := range expectedModels {
		found := false
		for _, model := range result.Models {
			if model == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected model %s not found in list: %v", expected, result.Models)
		}
	}
}

func TestIntegration_GetModelSchema(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	schema, err := client.GetModelSchema(ctx, "zscore_online")
	if err != nil {
		t.Fatalf("GetModelSchema() failed: %v", err)
	}

	modelSchema, ok := schema["schema"].(map[string]any)
	if !ok {
		t.Fatalf("schema = %T, want object", schema["schema"])
	}
	if modelSchema["type"] != "object" {
		t.Errorf("schema type = %v, want object", modelSchema["type"])
	}
	if modelSchema["properties"] == nil {
		t.Error("expected schema to have properties field")
	}
}

func TestIntegration_ValidateModel(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	modelSpec := map[string]any{
		"class":       "zscore",
		"z_threshold": 3.0,
	}

	result, err := client.ValidateModel(ctx, modelSpec)
	if err != nil {
		t.Fatalf("ValidateModel() failed: %v", err)
	}

	if !result.Valid {
		t.Error("expected model spec to be valid")
	}

	if result.ModelSpec == nil {
		t.Error("expected validated model_spec in response")
	}
}

func TestIntegration_GenerateConfig(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	req := &vmanomaly.ConfigGenerationRequest{
		Step:          "1m",
		Query:         "up",
		DatasourceURL: "http://victoriametrics:8428",
		FitWindow:     "1h",
		FitEvery:      "30m",
		ModelSpec: map[string]any{
			"class":       "zscore",
			"z_threshold": 3.0,
		},
	}

	result, err := client.GenerateConfig(ctx, req)
	if err != nil {
		t.Fatalf("GenerateConfig() failed: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty config")
	}

	if !strings.Contains(result, "schedulers:") {
		t.Error("expected YAML config to contain 'schedulers:' section")
	}
}

func TestIntegration_Query(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	req := &vmanomaly.QueryRequest{
		Query:          "up",
		Step:           "1m",
		DatasourceType: "vm",
	}

	result, err := client.Query(ctx, req)
	if err != nil {
		t.Fatalf("Query() failed: %v", err)
	}

	if result["status"] == nil {
		t.Error("expected status field in query response")
	}
}

func TestIntegration_TimeseriesCharacteristics(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()
	end := float64(time.Now().Unix())
	start := end - float64((6 * time.Hour).Seconds())
	limit := 5

	result, err := client.TimeseriesCharacteristics(ctx, &vmanomaly.TimeseriesCharacteristicsRequest{
		Query:          "rand()",
		Start:          &start,
		End:            &end,
		Step:           "5m",
		DatasourceType: "vm",
		Verbose:        true,
		Limit:          &limit,
	})
	if err != nil {
		t.Fatalf("TimeseriesCharacteristics() failed: %v", err)
	}
	if result["status"] != "success" {
		t.Fatalf("status = %v, want success", result["status"])
	}
	data, ok := result["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %T, want object", result["data"])
	}
	if data["resultType"] != "characteristics" {
		t.Errorf("resultType = %v, want characteristics", data["resultType"])
	}
	if _, ok := data["batch"].(map[string]any); !ok {
		t.Errorf("batch = %T, want object", data["batch"])
	}
	stats, ok := result["stats"].(map[string]any)
	if !ok {
		t.Fatalf("stats = %T, want object", result["stats"])
	}
	if stats["seriesCharacteristicsIncluded"] != "false" {
		t.Errorf("seriesCharacteristicsIncluded = %v, want false", stats["seriesCharacteristicsIncluded"])
	}
}

func TestIntegration_AutotuneTaskLifecycle(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()
	end := float64(time.Now().Unix())
	start := end - float64((6 * time.Hour).Seconds())
	limit := 1
	useProfileHints := true

	created, err := client.CreateAutotuneTask(ctx, &vmanomaly.AutotuneTaskRequest{
		Query:             "rand()",
		TunedClassName:    "mad_online",
		AnomalyPercentage: 0.02,
		Start:             &start,
		End:               &end,
		Step:              "5m",
		DatasourceType:    "vm",
		Limit:             &limit,
		UseProfileHints:   &useProfileHints,
		OptimizationParams: map[string]any{
			"n_trials":        1,
			"timeout":         2,
			"n_splits":        1,
			"train_val_ratio": 2.5,
			"beta":            0,
			"exact":           true,
		},
	})
	if err != nil {
		t.Fatalf("CreateAutotuneTask() failed: %v", err)
	}
	if created.TaskID == "" || created.Status != "running" {
		t.Fatalf("created = %#v, want non-empty task_id and running status", created)
	}

	status, err := client.GetAutotuneTask(ctx, created.TaskID)
	if err != nil {
		t.Fatalf("GetAutotuneTask() failed: %v", err)
	}
	if status.TaskID != created.TaskID {
		t.Errorf("task_id = %q, want %q", status.TaskID, created.TaskID)
	}
	if status.Status == "" {
		t.Error("expected non-empty autotune task status")
	}

	canceled, err := client.CancelAutotuneTask(ctx, created.TaskID)
	if err != nil {
		t.Fatalf("CancelAutotuneTask() failed: %v", err)
	}
	if !canceled["ok"] {
		t.Fatalf("cancellation response = %#v, want ok=true", canceled)
	}
}

func TestIntegration_GetDetectionLimits(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	limits, err := client.GetDetectionLimits(ctx)
	if err != nil {
		t.Fatalf("GetDetectionLimits() failed: %v", err)
	}

	if limits.MaxConcurrent <= 0 {
		t.Errorf("expected max_concurrent > 0, got %d", limits.MaxConcurrent)
	}

	if limits.Running < 0 {
		t.Errorf("expected running >= 0, got %d", limits.Running)
	}

	if limits.Available < 0 {
		t.Errorf("expected available >= 0, got %d", limits.Available)
	}
}

func TestIntegration_TaskLifecycle(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	var taskID string

	t.Run("create task", func(t *testing.T) {
		datasourceURL := "http://victoriametrics:8428"
		req := &vmanomaly.AnomalyDetectionTaskRequest{
			Query:            "rand()",
			Step:             "1m",
			FitWindow:        "1h",
			FitEvery:         "30m",
			AnomalyThreshold: 1.5,
			ModelSpec:        map[string]any{"class": "mad_online"},
			DatasourceURL:    &datasourceURL,
			DatasourceType:   "vm",
			Exact:            false,
			PassAuthHeaders:  false,
		}

		result, err := client.CreateDetectionTask(ctx, req)
		if err != nil {
			t.Fatalf("CreateDetectionTask() failed: %v", err)
		}

		if result.TaskID == "" {
			t.Fatal("expected non-empty task_id")
		}

		if result.Status == "" {
			t.Error("expected status field in response")
		}

		taskID = result.TaskID
		t.Logf("Created task: %s (status: %s)", taskID, result.Status)
	})

	t.Run("get task status", func(t *testing.T) {
		if taskID == "" {
			t.Skip("no task ID from create test")
		}

		time.Sleep(1 * time.Second)

		status, err := client.GetTaskStatus(ctx, taskID)
		if err != nil {
			t.Fatalf("GetTaskStatus() failed: %v", err)
		}

		if status.TaskID != taskID {
			t.Errorf("task_id = %v, want %v", status.TaskID, taskID)
		}

		if status.Status == "" {
			t.Error("expected status field")
		}

		if status.Progress < 0 || status.Progress > 100 {
			t.Errorf("progress = %d, expected 0-100", status.Progress)
		}

		t.Logf("Task status: %s (progress: %d%%)", status.Status, status.Progress)
	})

	t.Run("list tasks", func(t *testing.T) {
		if taskID == "" {
			t.Skip("no task ID from create test")
		}

		result, err := client.ListTasks(ctx, 20, nil)
		if err != nil {
			t.Fatalf("ListTasks() failed: %v", err)
		}

		found := false
		for _, task := range result.Tasks {
			if task.TaskID == taskID {
				found = true
				t.Logf("Found task in list: %s (status: %s)", task.TaskID, task.Status)
				break
			}
		}

		if !found {
			t.Errorf("created task %s not found in task list", taskID)
		}
	})

	t.Run("cancel task", func(t *testing.T) {
		if taskID == "" {
			t.Skip("no task ID from create test")
		}

		result, err := client.CancelTask(ctx, taskID)
		if err != nil {
			t.Logf("CancelTask() warning: %v (task may have completed)", err)
			return
		}

		if !result["ok"] {
			t.Errorf("expected ok=true, got %v", result["ok"])
		}

		t.Logf("Task canceled successfully: %s", taskID)
	})
}
