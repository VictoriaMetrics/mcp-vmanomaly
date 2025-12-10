package vmanomaly

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	client := NewClient(server.URL, "", nil)
	return client, server
}

func assertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8490", "test-token", map[string]string{"X-Custom": "value"})

	if client.baseURL != "http://localhost:8490" {
		t.Errorf("baseURL = %v, want http://localhost:8490", client.baseURL)
	}
	if client.bearerToken != "test-token" {
		t.Errorf("bearerToken = %v, want test-token", client.bearerToken)
	}
	if len(client.customHeaders) != 1 || client.customHeaders["X-Custom"] != "value" {
		t.Errorf("customHeaders = %v, want map[X-Custom:value]", client.customHeaders)
	}
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", client.httpClient.Timeout)
	}
}

func TestClient_GetHealth(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   string
		wantErr    bool
		wantStatus string
	}{
		{
			name:       "success",
			statusCode: 200,
			response:   `{"status":"ok"}`,
			wantErr:    false,
			wantStatus: "ok",
		},
		{
			name:       "invalid json",
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
		{
			name:       "500 error",
			statusCode: 500,
			response:   `{"error":"internal error"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, r.URL.Path, "/health")
				assertEqual(t, r.Method, http.MethodGet)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.GetHealth(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetHealth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result["status"] != tt.wantStatus {
				t.Errorf("status = %v, want %v", result["status"], tt.wantStatus)
			}
		})
	}
}

func TestClient_ListModels(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   string
		wantErr    bool
		wantModels []string
	}{
		{
			name:       "success with models",
			statusCode: 200,
			response:   `{"models":["zscore","prophet","mad"]}`,
			wantErr:    false,
			wantModels: []string{"zscore", "prophet", "mad"},
		},
		{
			name:       "empty models",
			statusCode: 200,
			response:   `{"models":[]}`,
			wantErr:    false,
			wantModels: []string{},
		},
		{
			name:       "invalid json",
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
		{
			name:       "404 error",
			statusCode: 404,
			response:   `{"error":"not found"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, r.URL.Path, "/api/v1/models")
				assertEqual(t, r.Method, http.MethodGet)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.ListModels(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("ListModels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result.Models) != len(tt.wantModels) {
					t.Errorf("models count = %v, want %v", len(result.Models), len(tt.wantModels))
				}
				for i, model := range result.Models {
					if model != tt.wantModels[i] {
						t.Errorf("model[%d] = %v, want %v", i, model, tt.wantModels[i])
					}
				}
			}
		})
	}
}

func TestClient_GetModelSchema(t *testing.T) {
	tests := []struct {
		name       string
		modelClass string
		statusCode int
		response   string
		wantErr    bool
	}{
		{
			name:       "success",
			modelClass: "zscore",
			statusCode: 200,
			response:   `{"type":"object","properties":{"threshold":{"type":"number"}}}`,
			wantErr:    false,
		},
		{
			name:       "special characters in model class",
			modelClass: "isolation_forest_univariate",
			statusCode: 200,
			response:   `{"type":"object"}`,
			wantErr:    false,
		},
		{
			name:       "404 unknown model",
			modelClass: "unknown",
			statusCode: 404,
			response:   `{"error":"model not found"}`,
			wantErr:    true,
		},
		{
			name:       "invalid json",
			modelClass: "zscore",
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api/v1/model/schema"
				if r.URL.Path != expectedPath {
					t.Errorf("path = %v, want %v", r.URL.Path, expectedPath)
				}
				if r.URL.Query().Get("model_class") != tt.modelClass {
					t.Errorf("model_class = %v, want %v", r.URL.Query().Get("model_class"), tt.modelClass)
				}
				assertEqual(t, r.Method, http.MethodGet)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.GetModelSchema(context.Background(), tt.modelClass)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetModelSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result["type"] == nil {
				t.Error("expected schema to have 'type' field")
			}
		})
	}
}

func TestClient_ValidateModel(t *testing.T) {
	tests := []struct {
		name       string
		modelSpec  map[string]any
		statusCode int
		response   string
		wantErr    bool
		wantValid  bool
	}{
		{
			name:       "valid model",
			modelSpec:  map[string]any{"class": "zscore", "threshold": 3.0},
			statusCode: 200,
			response:   `{"valid":true,"model_spec":{"class":"zscore","threshold":3.0}}`,
			wantErr:    false,
			wantValid:  true,
		},
		{
			name:       "invalid model",
			modelSpec:  map[string]any{"class": "zscore", "threshold": -1},
			statusCode: 200,
			response:   `{"valid":false,"model_spec":{}}`,
			wantErr:    false,
			wantValid:  false,
		},
		{
			name:       "400 bad request",
			modelSpec:  map[string]any{},
			statusCode: 400,
			response:   `{"error":"invalid spec"}`,
			wantErr:    true,
		},
		{
			name:       "invalid json response",
			modelSpec:  map[string]any{"class": "zscore"},
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, r.URL.Path, "/api/v1/model/validate")
				assertEqual(t, r.Method, http.MethodPost)
				assertEqual(t, r.Header.Get("Content-Type"), "application/json")

				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.ValidateModel(context.Background(), tt.modelSpec)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result.Valid != tt.wantValid {
				t.Errorf("valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func TestClient_GenerateConfig(t *testing.T) {
	tenantID := "tenant-1"
	inferEvery := "30s"

	tests := []struct {
		name       string
		request    *ConfigGenerationRequest
		statusCode int
		response   string
		wantErr    bool
	}{
		{
			name: "success with all params",
			request: &ConfigGenerationRequest{
				Step:          "1m",
				Query:         "up{job='app'}",
				DatasourceURL: "http://localhost:8428",
				TenantID:      &tenantID,
				FitWindow:     "1d",
				FitEvery:      "1d",
				InferEvery:    &inferEvery,
				ModelSpec:     map[string]any{"class": "zscore"},
			},
			statusCode: 200,
			response:   "schedulers:\n  model: zscore\n",
			wantErr:    false,
		},
		{
			name: "success with optional params nil",
			request: &ConfigGenerationRequest{
				Step:          "1m",
				Query:         "up",
				DatasourceURL: "http://localhost:8428",
				FitWindow:     "1d",
				FitEvery:      "1d",
				ModelSpec:     map[string]any{"class": "prophet"},
			},
			statusCode: 200,
			response:   "schedulers:\n  model: prophet\n",
			wantErr:    false,
		},
		{
			name: "500 error",
			request: &ConfigGenerationRequest{
				Step:          "1m",
				Query:         "invalid",
				DatasourceURL: "http://localhost:8428",
				FitWindow:     "1d",
				FitEvery:      "1d",
				ModelSpec:     map[string]any{},
			},
			statusCode: 500,
			response:   "internal error",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, r.URL.Path, "/api/vmanomaly/config.yaml")
				assertEqual(t, r.Method, http.MethodGet)

				query := r.URL.Query()
				assertEqual(t, query.Get("step"), tt.request.Step)
				assertEqual(t, query.Get("query"), tt.request.Query)
				assertEqual(t, query.Get("datasource_url"), tt.request.DatasourceURL)

				if tt.request.TenantID != nil {
					assertEqual(t, query.Get("tenant_id"), *tt.request.TenantID)
				}
				if tt.request.InferEvery != nil {
					assertEqual(t, query.Get("infer_every"), *tt.request.InferEvery)
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.GenerateConfig(context.Background(), tt.request)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != tt.response {
				t.Errorf("config = %v, want %v", result, tt.response)
			}
		})
	}
}

func TestClient_CreateDetectionTask(t *testing.T) {
	tests := []struct {
		name       string
		request    *AnomalyDetectionTaskRequest
		statusCode int
		response   string
		wantErr    bool
		wantTaskID string
	}{
		{
			name: "success",
			request: &AnomalyDetectionTaskRequest{
				Query:            "up",
				Step:             "1m",
				FitWindow:        "1d",
				FitEvery:         "1d",
				AnomalyThreshold: 1.0,
				DatasourceType:   "vm",
			},
			statusCode: 200,
			response:   `{"task_id":"task-123","status":"running"}`,
			wantErr:    false,
			wantTaskID: "task-123",
		},
		{
			name: "400 bad request",
			request: &AnomalyDetectionTaskRequest{
				Query: "",
			},
			statusCode: 400,
			response:   `{"error":"query required"}`,
			wantErr:    true,
		},
		{
			name: "invalid json response",
			request: &AnomalyDetectionTaskRequest{
				Query:            "up",
				Step:             "1m",
				FitWindow:        "1d",
				FitEvery:         "1d",
				AnomalyThreshold: 1.0,
				DatasourceType:   "vm",
			},
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, r.URL.Path, "/api/v1/anomaly_detection/tasks")
				assertEqual(t, r.Method, http.MethodPost)
				assertEqual(t, r.Header.Get("Content-Type"), "application/json")

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.CreateDetectionTask(context.Background(), tt.request)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDetectionTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result.TaskID != tt.wantTaskID {
				t.Errorf("task_id = %v, want %v", result.TaskID, tt.wantTaskID)
			}
		})
	}
}

func TestClient_GetTaskStatus(t *testing.T) {
	tests := []struct {
		name         string
		taskID       string
		statusCode   int
		response     string
		wantErr      bool
		wantProgress int
	}{
		{
			name:         "running task",
			taskID:       "task-123",
			statusCode:   200,
			response:     `{"task_id":"task-123","status":"running","progress":50,"message":"Processing","updated_at":"2025-01-01T00:00:00Z","metrics":{}}`,
			wantErr:      false,
			wantProgress: 50,
		},
		{
			name:         "completed task",
			taskID:       "task-456",
			statusCode:   200,
			response:     `{"task_id":"task-456","status":"done","progress":100,"message":"Complete","updated_at":"2025-01-01T00:00:00Z","metrics":{}}`,
			wantErr:      false,
			wantProgress: 100,
		},
		{
			name:       "404 not found",
			taskID:     "unknown",
			statusCode: 404,
			response:   `{"error":"task not found"}`,
			wantErr:    true,
		},
		{
			name:       "invalid json",
			taskID:     "task-123",
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api/v1/anomaly_detection/tasks/" + tt.taskID
				assertEqual(t, r.URL.Path, expectedPath)
				assertEqual(t, r.Method, http.MethodGet)

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.GetTaskStatus(context.Background(), tt.taskID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetTaskStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result.Progress != tt.wantProgress {
				t.Errorf("progress = %v, want %v", result.Progress, tt.wantProgress)
			}
		})
	}
}

func TestClient_ListTasks(t *testing.T) {
	statusRunning := "running"

	tests := []struct {
		name       string
		limit      int
		status     *string
		statusCode int
		response   string
		wantErr    bool
		wantCount  int
	}{
		{
			name:       "success with status filter",
			limit:      10,
			status:     &statusRunning,
			statusCode: 200,
			response:   `{"tasks":[{"task_id":"task-1","status":"running","progress":50,"message":"Processing","updated_at":"2025-01-01T00:00:00Z","metrics":{}}]}`,
			wantErr:    false,
			wantCount:  1,
		},
		{
			name:       "success without status filter",
			limit:      20,
			status:     nil,
			statusCode: 200,
			response:   `{"tasks":[{"task_id":"task-1","status":"running","progress":50,"message":"Processing","updated_at":"2025-01-01T00:00:00Z","metrics":{}},{"task_id":"task-2","status":"done","progress":100,"message":"Complete","updated_at":"2025-01-01T00:00:00Z","metrics":{}}]}`,
			wantErr:    false,
			wantCount:  2,
		},
		{
			name:       "empty tasks",
			limit:      10,
			status:     nil,
			statusCode: 200,
			response:   `{"tasks":[]}`,
			wantErr:    false,
			wantCount:  0,
		},
		{
			name:       "invalid json",
			limit:      10,
			status:     nil,
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, r.URL.Path, "/api/v1/anomaly_detection/tasks")
				assertEqual(t, r.Method, http.MethodGet)

				query := r.URL.Query()
				if query.Get("limit") == "" {
					t.Error("limit query param missing")
				}
				if tt.status != nil && query.Get("status") != *tt.status {
					t.Errorf("status = %v, want %v", query.Get("status"), *tt.status)
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.ListTasks(context.Background(), tt.limit, tt.status)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListTasks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(result.Tasks) != tt.wantCount {
				t.Errorf("tasks count = %v, want %v", len(result.Tasks), tt.wantCount)
			}
		})
	}
}

func TestClient_CancelTask(t *testing.T) {
	tests := []struct {
		name         string
		taskID       string
		statusCode   int
		response     string
		wantErr      bool
		wantCanceled bool
	}{
		{
			name:         "success",
			taskID:       "task-123",
			statusCode:   200,
			response:     `{"canceled":true}`,
			wantErr:      false,
			wantCanceled: true,
		},
		{
			name:       "404 not found",
			taskID:     "unknown",
			statusCode: 404,
			response:   `{"error":"task not found"}`,
			wantErr:    true,
		},
		{
			name:       "invalid json",
			taskID:     "task-123",
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api/v1/anomaly_detection/tasks/" + tt.taskID
				assertEqual(t, r.URL.Path, expectedPath)
				assertEqual(t, r.Method, http.MethodDelete)

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.CancelTask(context.Background(), tt.taskID)

			if (err != nil) != tt.wantErr {
				t.Errorf("CancelTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result["canceled"] != tt.wantCanceled {
				t.Errorf("canceled = %v, want %v", result["canceled"], tt.wantCanceled)
			}
		})
	}
}

func TestClient_GetDetectionLimits(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		response      string
		wantErr       bool
		wantMaxConcur int
	}{
		{
			name:          "success",
			statusCode:    200,
			response:      `{"max_concurrent":5,"running":2,"available":3}`,
			wantErr:       false,
			wantMaxConcur: 5,
		},
		{
			name:       "invalid json",
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
		{
			name:       "500 error",
			statusCode: 500,
			response:   `{"error":"internal error"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, r.URL.Path, "/api/v1/anomaly_detection/limits")
				assertEqual(t, r.Method, http.MethodGet)

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.GetDetectionLimits(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDetectionLimits() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result.MaxConcurrent != tt.wantMaxConcur {
				t.Errorf("max_concurrent = %v, want %v", result.MaxConcurrent, tt.wantMaxConcur)
			}
		})
	}
}

func TestClient_Query(t *testing.T) {
	tests := []struct {
		name       string
		request    *QueryRequest
		statusCode int
		response   string
		wantErr    bool
	}{
		{
			name: "success",
			request: &QueryRequest{
				Query:          "up",
				Step:           "1m",
				DatasourceType: "vm",
			},
			statusCode: 200,
			response:   `{"status":"success","data":{"resultType":"vector","result":[]}}`,
			wantErr:    false,
		},
		{
			name: "400 bad query",
			request: &QueryRequest{
				Query:          "invalid{",
				Step:           "1m",
				DatasourceType: "vm",
			},
			statusCode: 400,
			response:   `{"error":"parse error"}`,
			wantErr:    true,
		},
		{
			name: "invalid json response",
			request: &QueryRequest{
				Query:          "up",
				Step:           "1m",
				DatasourceType: "vm",
			},
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, r.URL.Path, "/api/v1/query")
				assertEqual(t, r.Method, http.MethodPost)
				assertEqual(t, r.Header.Get("Content-Type"), "application/json")

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.Query(context.Background(), tt.request)

			if (err != nil) != tt.wantErr {
				t.Errorf("Query() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result["status"] == nil {
				t.Error("expected result to have 'status' field")
			}
		})
	}
}

func TestClient_GetBuildInfo(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		response    string
		wantErr     bool
		wantVersion string
	}{
		{
			name:        "success",
			statusCode:  200,
			response:    `{"version":"1.0.0","build_time":"2025-01-01"}`,
			wantErr:     false,
			wantVersion: "1.0.0",
		},
		{
			name:       "invalid json",
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
		{
			name:       "500 error",
			statusCode: 500,
			response:   `{"error":"internal error"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, r.URL.Path, "/api/v1/status/buildinfo")
				assertEqual(t, r.Method, http.MethodGet)

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.GetBuildInfo(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetBuildInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result["version"] != tt.wantVersion {
				t.Errorf("version = %v, want %v", result["version"], tt.wantVersion)
			}
		})
	}
}

func TestClient_ContextHandling(t *testing.T) {
	t.Run("canceled context", func(t *testing.T) {
		client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := client.GetHealth(ctx)
		if err == nil {
			t.Error("expected error with canceled context")
		}
	})

	t.Run("timeout context", func(t *testing.T) {
		client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.GetHealth(ctx)
		if err == nil {
			t.Error("expected timeout error")
		}
	})

	t.Run("deadline exceeded", func(t *testing.T) {
		client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		defer server.Close()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Hour))
		defer cancel()

		_, err := client.GetHealth(ctx)
		if err == nil {
			t.Error("expected deadline exceeded error")
		}
	})
}

func TestClient_Compatibility(t *testing.T) {
	tests := []struct {
		name           string
		versionTo      *string
		statusCode     int
		response       string
		wantErr        bool
		wantCompatible bool
		wantHasState   bool
	}{
		{
			name:       "compatible with state",
			versionTo:  nil,
			statusCode: 200,
			response: `{
				"runtime_version": "1.18.0",
				"stored_version": "1.17.0",
				"global_check": {
					"has_state": true,
					"is_compatible": true,
					"drop_everything": false
				}
			}`,
			wantErr:        false,
			wantCompatible: true,
			wantHasState:   true,
		},
		{
			name:       "no persisted state",
			versionTo:  nil,
			statusCode: 200,
			response: `{
				"runtime_version": "1.18.0",
				"global_check": {
					"has_state": false,
					"is_compatible": true,
					"drop_everything": false
				}
			}`,
			wantErr:        false,
			wantCompatible: true,
			wantHasState:   false,
		},
		{
			name:       "incompatible - requires full purge",
			versionTo:  nil,
			statusCode: 200,
			response: `{
				"runtime_version": "2.0.0",
				"stored_version": "1.0.0",
				"global_check": {
					"has_state": true,
					"is_compatible": false,
					"reason": "Major version incompatibility",
					"drop_everything": true
				}
			}`,
			wantErr:        false,
			wantCompatible: false,
			wantHasState:   true,
		},
		{
			name:       "with version_to parameter",
			versionTo:  func() *string { v := "2.0.0"; return &v }(),
			statusCode: 200,
			response: `{
				"runtime_version": "2.0.0",
				"global_check": {
					"has_state": true,
					"is_compatible": false,
					"drop_everything": false
				},
				"component_assessment": {
					"issues": [{
						"component": "model",
						"subcomponent": "prophet",
						"requirement": {
							"runtime_version": "2.0.0",
							"origin_version": "1.17.0",
							"min_state_version": "1.18.0",
							"description": "Prophet model state format changed"
						},
						"affected_entities": ["prophet_1", "prophet_2"]
					}],
					"models_to_purge": ["prophet_1", "prophet_2"],
					"should_purge_reader_data": false
				}
			}`,
			wantErr:        false,
			wantCompatible: false,
			wantHasState:   true,
		},
		{
			name:       "server error",
			versionTo:  nil,
			statusCode: 500,
			response:   `{"error":"internal error"}`,
			wantErr:    true,
		},
		{
			name:       "invalid json response",
			versionTo:  nil,
			statusCode: 200,
			response:   `{invalid}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, r.URL.Path, "/api/v1/compatibility")
				assertEqual(t, r.Method, http.MethodGet)

				if tt.versionTo != nil {
					assertEqual(t, r.URL.Query().Get("version_to"), *tt.versionTo)
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			})
			defer server.Close()

			result, err := client.Compatibility(context.Background(), tt.versionTo)

			if (err != nil) != tt.wantErr {
				t.Errorf("Compatibility() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result.GlobalCheck.IsCompatible != tt.wantCompatible {
					t.Errorf("is_compatible = %v, want %v", result.GlobalCheck.IsCompatible, tt.wantCompatible)
				}
				if result.GlobalCheck.HasState != tt.wantHasState {
					t.Errorf("has_state = %v, want %v", result.GlobalCheck.HasState, tt.wantHasState)
				}
			}
		})
	}
}
