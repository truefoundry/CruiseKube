package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	metricsprovider "github.com/truefoundry/cruisekube/pkg/adapters/metricsprovider/prometheus"
	"github.com/truefoundry/cruisekube/pkg/buildmetadata"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/task"
	taskutils "github.com/truefoundry/cruisekube/pkg/task/utils"
)

func TestGetConfigHandlerKloudfuseUsesQueryHealthAndDoesNotExposeBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const bearerToken = "secret-kloudfuse-token"
	var buildInfoHits int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/query":
			if got := r.Header.Get("Authorization"); got != "Bearer "+bearerToken {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if got := r.Form.Get("query"); got != "vector(1)" {
				http.Error(w, "unexpected query", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1700000000,"1"]}]}}`))
		default:
			if r.URL.Path == "/api/v1/status/buildinfo" {
				buildInfoHits++
			}
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	promClient := newTestPrometheusClient(t, server.URL, bearerToken)
	deps := HandlerDependencies{
		ClusterManager: testClusterManager{clients: &cluster.ClusterClients{PrometheusClient: promClient}},
		Config: &config.Config{
			ControllerMode: config.ClusterModeLocal,
			Dependencies: config.Dependencies{
				Local: config.LocalDeps{
					PrometheusURL: "http://legacy-prometheus.example",
					MetricsProvider: config.MetricsProviderConfig{
						Type:        config.MetricsProviderTypeKloudfuse,
						URL:         server.URL,
						BearerToken: bearerToken,
					},
				},
			},
		},
	}

	w := executeConfigRequest(deps)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), bearerToken) {
		t.Fatalf("response body exposed bearer token: %s", w.Body.String())
	}
	if buildInfoHits != 0 {
		t.Fatalf("expected kloudfuse health check not to call buildinfo, got %d calls", buildInfoHits)
	}

	body := decodeConfigResponse(t, w)

	if body.Provider != string(config.MetricsProviderTypeKloudfuse) {
		t.Fatalf("expected provider=%q, got %q", config.MetricsProviderTypeKloudfuse, body.Provider)
	}
	if body.URL != server.URL {
		t.Fatalf("expected url=%q, got %q", server.URL, body.URL)
	}
	if !body.Connected {
		t.Fatal("expected connected=true")
	}
	if body.Version != buildmetadata.Version {
		t.Fatalf("expected controller version %q, got %q", buildmetadata.Version, body.Version)
	}
	if body.ProviderVersion != "" {
		t.Fatalf("expected kloudfuse providerVersion to be empty, got %q", body.ProviderVersion)
	}
	if body.Error != "" {
		t.Fatalf("expected no error, got %q", body.Error)
	}
}

func TestGetConfigHandlerPrometheusKeepsControllerVersionAndReturnsProviderVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/query":
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1700000000,"1"]}]}}`))
		case "/api/v1/status/buildinfo":
			_, _ = w.Write([]byte(`{"status":"success","data":{"version":"2.51.0","revision":"test","branch":"main","buildUser":"test","buildDate":"2024-01-01","goVersion":"go1.22"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	deps := HandlerDependencies{
		ClusterManager: testClusterManager{clients: &cluster.ClusterClients{PrometheusClient: newTestPrometheusClient(t, server.URL, "")}},
		Config: &config.Config{
			ControllerMode: config.ClusterModeLocal,
			Dependencies: config.Dependencies{
				Local: config.LocalDeps{
					MetricsProvider: config.MetricsProviderConfig{
						Type: config.MetricsProviderTypePrometheus,
						URL:  server.URL,
					},
				},
			},
		},
	}

	w := executeConfigRequest(deps)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeConfigResponse(t, w)
	if body.Version != buildmetadata.Version {
		t.Fatalf("expected controller version %q, got %q", buildmetadata.Version, body.Version)
	}
	if body.ProviderVersion != "2.51.0" {
		t.Fatalf("expected providerVersion=2.51.0, got %q", body.ProviderVersion)
	}
}

func TestKloudfusePrometheusCompatibleIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const bearerToken = "test-token"
	fixture := newKloudfusePrometheusFixture(t, bearerToken)
	defer fixture.Close()

	provider := newTestKloudfuseProvider(t, fixture.URL, bearerToken)
	deps := HandlerDependencies{
		ClusterManager: testClusterManager{clients: &cluster.ClusterClients{PrometheusClient: provider.GetClient()}},
		Config: &config.Config{
			ControllerMode: config.ClusterModeLocal,
			Dependencies: config.Dependencies{
				Local: config.LocalDeps{
					PrometheusURL: "http://legacy-prometheus.example",
					MetricsProvider: config.MetricsProviderConfig{
						Type:        config.MetricsProviderTypeKloudfuse,
						URL:         fixture.URL,
						BearerToken: bearerToken,
					},
				},
			},
		},
	}

	w := executeConfigRequest(deps)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), bearerToken) {
		t.Fatalf("response body exposed bearer token: %s", w.Body.String())
	}

	body := decodeConfigResponse(t, w)
	if body.Provider != string(config.MetricsProviderTypeKloudfuse) {
		t.Fatalf("expected provider=%q, got %q", config.MetricsProviderTypeKloudfuse, body.Provider)
	}
	if body.URL != fixture.URL {
		t.Fatalf("expected url=%q, got %q", fixture.URL, body.URL)
	}
	if !body.Connected {
		t.Fatal("expected connected=true")
	}
	if body.Version != buildmetadata.Version {
		t.Fatalf("expected controller version %q, got %q", buildmetadata.Version, body.Version)
	}
	if body.ProviderVersion != "" {
		t.Fatalf("expected kloudfuse providerVersion to be empty, got %q", body.ProviderVersion)
	}
	if body.Error != "" {
		t.Fatalf("expected no error, got %q", body.Error)
	}

	_, _, err := provider.ExecuteQueryWithRetry(context.Background(), cluster.SingleClusterID, taskutils.BuildClusterCPUUtilizationExpression(), "cluster-cpu-utilization")
	if err != nil {
		t.Fatalf("expected representative instant query to succeed: %v", err)
	}

	predictions, err := taskutils.PredictSimpleStatsFromTimeSeriesModel(context.Background(), []string{"default"}, provider.GetClient(), "cpu", false)
	if err != nil {
		t.Fatalf("expected representative create-stats range query path to succeed: %v", err)
	}
	if len(predictions["default"]) == 0 {
		t.Fatalf("expected predictions from representative range query, got %#v", predictions)
	}

	if fixture.BuildInfoHits() != 0 {
		t.Fatalf("expected kloudfuse health not to call buildinfo, got %d calls", fixture.BuildInfoHits())
	}
	if fixture.RepresentativeInstantHits() == 0 {
		t.Fatal("expected fixture to receive representative instant query")
	}
	if fixture.QueryRangeHits() == 0 {
		t.Fatal("expected fixture to receive representative query_range request")
	}
}

func TestGetConfigHandlerKloudfuseRedactsBearerTokenFromQueryErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const bearerToken = "secret-kloudfuse-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "upstream rejected token "+bearerToken, http.StatusUnauthorized)
	}))
	defer server.Close()

	promClient := newTestPrometheusClient(t, server.URL, bearerToken)
	deps := HandlerDependencies{
		ClusterManager: testClusterManager{clients: &cluster.ClusterClients{PrometheusClient: promClient}},
		Config: &config.Config{
			ControllerMode: config.ClusterModeLocal,
			Dependencies: config.Dependencies{
				Local: config.LocalDeps{
					MetricsProvider: config.MetricsProviderConfig{
						Type:        config.MetricsProviderTypeKloudfuse,
						URL:         server.URL,
						BearerToken: bearerToken,
					},
				},
			},
		},
	}

	w := executeConfigRequest(deps)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), bearerToken) {
		t.Fatalf("response body exposed bearer token: %s", w.Body.String())
	}

	body := decodeConfigResponse(t, w)
	if body.Connected {
		t.Fatal("expected connected=false")
	}
	if body.Error == "" {
		t.Fatal("expected redacted error")
	}
}

type configResponse struct {
	URL             string `json:"url"`
	Provider        string `json:"provider"`
	Connected       bool   `json:"connected"`
	Version         string `json:"version"`
	ProviderVersion string `json:"providerVersion"`
	Error           string `json:"error"`
}

func executeConfigRequest(deps HandlerDependencies) *httptest.ResponseRecorder {
	router := gin.New()
	router.GET("/clusters/:clusterID/config", deps.GetConfigHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/clusters/default/config", nil)
	router.ServeHTTP(w, req)
	return w
}

func decodeConfigResponse(t *testing.T, w *httptest.ResponseRecorder) configResponse {
	t.Helper()
	var body configResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return body
}

func newTestPrometheusClient(t *testing.T, url, bearerToken string) promv1.API {
	t.Helper()
	apiClient, err := promapi.NewClient(promapi.Config{
		Address: url,
		Client: &http.Client{
			Transport: &metricsprovider.BearerTokenRoundTripper{
				BearerToken: bearerToken,
				Proxied:     http.DefaultTransport,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create prometheus client: %v", err)
	}
	return promv1.NewAPI(apiClient)
}

func newTestKloudfuseProvider(t *testing.T, url, bearerToken string) *metricsprovider.PrometheusProvider {
	t.Helper()
	provider, err := metricsprovider.NewPrometheusProvider(context.Background(), &metricsprovider.PrometheusClientConfig{
		PrometheusURL:        url,
		ProviderName:         string(config.MetricsProviderTypeKloudfuse),
		BearerToken:          bearerToken,
		QueryTimeout:         time.Second,
		MaxConnsPerHost:      2,
		MaxIdleConns:         2,
		IdleConnTimeout:      time.Second,
		ResponseTimeout:      time.Second,
		DialTimeout:          time.Second,
		KeepAlive:            time.Second,
		TLSHandshakeTimeout:  time.Second,
		MaxQueryRetries:      1,
		RetryBackoffBase:     time.Millisecond,
		MaxConcurrentQueries: 1,
	})
	if err != nil {
		t.Fatalf("failed to create kloudfuse provider: %v", err)
	}
	return provider
}

type kloudfusePrometheusFixture struct {
	*httptest.Server
	token                     string
	buildInfoHits             int
	representativeInstantHits int
	queryRangeHits            int
}

func newKloudfusePrometheusFixture(t *testing.T, bearerToken string) *kloudfusePrometheusFixture {
	t.Helper()
	fixture := &kloudfusePrometheusFixture{token: bearerToken}
	fixture.Server = httptest.NewServer(http.HandlerFunc(fixture.ServeHTTP))
	return fixture
}

func (f *kloudfusePrometheusFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if got := r.Header.Get("Authorization"); got != "Bearer "+f.token {
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return
	}

	switch r.URL.Path {
	case "/api/v1/query":
		f.handleQuery(w, r)
	case "/api/v1/query_range":
		f.handleQueryRange(w, r)
	case "/api/v1/status/buildinfo":
		f.buildInfoHits++
		http.NotFound(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *kloudfusePrometheusFixture) handleQuery(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query := r.Form.Get("query")
	w.Header().Set("Content-Type", "application/json")

	if query == "vector(1)" {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1700000000,"1"]}]}}`))
		return
	}

	if strings.Contains(query, "node_cpu_seconds_total") {
		f.representativeInstantHits++
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"node":"node-1"},"value":[1700000000,"2"]}]}}`))
		return
	}

	http.Error(w, "unexpected instant query", http.StatusBadRequest)
}

func (f *kloudfusePrometheusFixture) handleQueryRange(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query := r.Form.Get("query")
	if !strings.Contains(query, "container_cpu_usage_seconds_total") || !strings.Contains(query, `namespace="default"`) {
		http.Error(w, "unexpected range query", http.StatusBadRequest)
		return
	}

	f.queryRangeHits++
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"namespace":"default","pod":"demo-abc","container":"app","created_by_kind":"Deployment","created_by_name":"demo"},"values":[[1700000000,"0.10"],[1700003600,"0.20"]]}]}}`))
}

func (f *kloudfusePrometheusFixture) BuildInfoHits() int { return f.buildInfoHits }

func (f *kloudfusePrometheusFixture) RepresentativeInstantHits() int {
	return f.representativeInstantHits
}

func (f *kloudfusePrometheusFixture) QueryRangeHits() int { return f.queryRangeHits }

type testClusterManager struct {
	clients *cluster.ClusterClients
	err     error
}

func (m testClusterManager) RefreshClusters(ctx context.Context) error { return nil }
func (m testClusterManager) GetAllClusters() map[string]*cluster.ClusterClients {
	return map[string]*cluster.ClusterClients{cluster.SingleClusterID: m.clients}
}
func (m testClusterManager) GetClusterIDs() []string { return []string{cluster.SingleClusterID} }
func (m testClusterManager) GetClusterClients(clusterID string) (*cluster.ClusterClients, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.clients == nil {
		return nil, errors.New("cluster clients not configured")
	}
	return m.clients, nil
}
func (m testClusterManager) GetPrometheusConnectionInfo(clusterID string) (*cluster.PrometheusConnectionInfo, error) {
	return nil, nil
}
func (m testClusterManager) GetClusterMode() cluster.ClusterMode { return cluster.ClusterModeSingle }
func (m testClusterManager) AddTask(task task.Task)              {}
func (m testClusterManager) GetTask(taskName string) (task.Task, error) {
	return nil, errors.New("task not configured")
}
func (m testClusterManager) ScheduleAllTasks(ctx context.Context) error { return nil }
func (m testClusterManager) StopScheduler(ctx context.Context)          {}

var _ cluster.Manager = testClusterManager{}
