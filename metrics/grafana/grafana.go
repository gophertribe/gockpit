package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"time"
)

type Grafana struct {
	baseURL    string
	htclient   http.Client
	dashboards map[string]string
}

func New(addr string) *Grafana {
	return &Grafana{
		htclient: http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:    addr,
		dashboards: make(map[string]string),
	}
}

type Datasource struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Access    string `json:"access"`
	IsDefault bool   `json:"isDefault"`
}

type InfluxDatasource struct {
	Id                int    `json:"id"`
	Uid               string `json:"uid"`
	OrgId             int    `json:"orgId"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	TypeLogoUrl       string `json:"typeLogoUrl"`
	Access            string `json:"access"`
	Url               string `json:"url"`
	Password          string `json:"password"`
	BasicAuth         bool   `json:"basicAuth"`
	BasicAuthUser     string `json:"basicAuthUser"`
	BasicAuthPassword string `json:"basicAuthPassword"`
	WithCredentials   bool   `json:"withCredentials"`
	IsDefault         bool   `json:"isDefault"`
	JsonData          struct {
		HttpMode      string `json:"httpMode"`
		Timeout       int    `json:"timeout"`
		Version       string `json:"version"`
		Organization  string `json:"organization"`
		DefaultBucket string `json:"defaultBucket"`
	} `json:"jsonData"`
	SecureJsonFields struct {
	} `json:"secureJsonFields"`
	Version        int  `json:"version"`
	ReadOnly       bool `json:"readOnly"`
	SecureJsonData struct {
		Token string `json:"token"`
	} `json:"secureJsonData"`
}

type DatasourceResponse struct {
	Datasource InfluxDatasource `json:"datasource"`
	Id         int              `json:"id"`
	Message    string           `json:"message"`
	Name       string           `json:"name"`
}

type DatasourceQuery struct {
	RefId         string `json:"refId"`
	Query         string `json:"query"`
	DatasourceId  int    `json:"datasourceId"`
	IntervalMs    int    `json:"intervalMs"`
	MaxDataPoints int    `json:"maxDataPoints"`
}

type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type RequestDatasourceQuery struct {
	Queries []DatasourceQuery `json:"queries"`
	Range   DateRange         `json:"range"`
	From    string            `json:"from"`
	To      string            `json:"to"`
}

type QueryResult struct {
	Frames []struct {
		Schema struct {
			RefId string `json:"refId"`
			Meta  struct {
				ExecutedQueryString string `json:"executedQueryString"`
			} `json:"meta"`
			Fields []struct {
				Name     string `json:"name"`
				Type     string `json:"type"`
				TypeInfo struct {
					Frame    string `json:"frame"`
					Nullable bool   `json:"nullable"`
				} `json:"typeInfo"`
				Labels struct {
					OrganizationID string `json:"organizationID"`
				} `json:"labels"`
			} `json:"fields"`
		} `json:"schema"`
		Data struct {
			Values [][]interface{} `json:"values"`
		} `json:"data"`
	} `json:"frames"`
}
type ResponseDatasourceQuery struct {
	Results map[string]QueryResult `json:"results"`
}

func (g *Grafana) SetupInfluxDatasource(ctx context.Context, name string, influxURL, bucket, org, token string) error {
	req, err := http.NewRequest(http.MethodGet, g.baseURL+"/api/datasources", nil)
	if err != nil {
		return err
	}
	req.Header.Add("X-WEBAUTH-USER", "admin")
	res, err := g.htclient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}
	dec := json.NewDecoder(res.Body)
	defer func() { _ = res.Body.Close() }()
	var ds []struct {
		Name string `json:"name"`
	}
	err = dec.Decode(&ds)
	if err != nil {
		return fmt.Errorf("could not decode datasources response: %w", err)
	}
	for _, d := range ds {
		if d.Name == name {
			return nil
		}
	}
	influxDs, err := g.CreateDatasource(ctx, Datasource{
		Name:      name,
		Type:      "influxdb",
		Access:    "proxy",
		IsDefault: true,
	})
	if err != nil {
		return fmt.Errorf("could not create datasource: %w", err)
	}
	slog.Info("created grafana datasource", "datasource", influxDs.Message)
	influxDs.Datasource.Url = influxURL
	influxDs.Datasource.JsonData.DefaultBucket = bucket
	influxDs.Datasource.JsonData.Version = "Flux"
	influxDs.Datasource.JsonData.Organization = org
	influxDs.Datasource.JsonData.HttpMode = "POST"
	influxDs.Datasource.JsonData.Timeout = 10
	influxDs.Datasource.SecureJsonData.Token = token
	influxDs.Datasource.Version = 1
	influxDs, err = g.UpdateInfluxDatasource(ctx, influxDs.Datasource)
	if err != nil {
		return fmt.Errorf("could not update datasource: %w", err)
	}
	slog.Info("updated grafana datasource", "datasource", influxDs.Message)
	_, err = g.QueryDatasource(ctx, RequestDatasourceQuery{
		Queries: []DatasourceQuery{{DatasourceId: influxDs.Datasource.Id, RefId: "test", Query: "buckets()", IntervalMs: 60000, MaxDataPoints: 423}},
		Range: DateRange{
			From: time.Time{},
			To:   time.Time{},
		},
		From: "1000",
		To:   "2000",
	})
	if err != nil {
		return fmt.Errorf("could not reach created datasource: %w", err)
	}
	return nil
}

func (g *Grafana) CreateDatasource(ctx context.Context, d Datasource) (*DatasourceResponse, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(d)
	if err != nil {
		return nil, fmt.Errorf("could not encode request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, g.baseURL+"/api/datasources", &body)
	if err != nil {
		return nil, fmt.Errorf("could not buid request: %w", err)
	}
	req.WithContext(ctx)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-WEBAUTH-USER", "admin")
	res, err := g.htclient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not perform request: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		dump, _ := httputil.DumpResponse(res, true)
		fmt.Println(string(dump))
		return nil, fmt.Errorf("unexpected status code; expecting %d got %d", http.StatusOK, res.StatusCode)
	}
	dec := json.NewDecoder(res.Body)
	defer func() { _ = res.Body.Close() }()
	var ds DatasourceResponse
	err = dec.Decode(&ds)
	if err != nil {
		return nil, fmt.Errorf("could not decode datasource response: %w", err)
	}
	return &ds, nil
}

func (g *Grafana) UpdateInfluxDatasource(ctx context.Context, d InfluxDatasource) (*DatasourceResponse, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(d)
	if err != nil {
		return nil, fmt.Errorf("could not encode request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/datasources/%d", g.baseURL, d.Id), &body)
	if err != nil {
		return nil, fmt.Errorf("could not buid request: %w", err)
	}
	req.WithContext(ctx)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-WEBAUTH-USER", "admin")
	res, err := g.htclient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not perform request: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		dump, _ := httputil.DumpResponse(res, true)
		fmt.Println(string(dump))
		return nil, fmt.Errorf("unexpected status code; expecting %d got %d", http.StatusOK, res.StatusCode)
	}
	dec := json.NewDecoder(res.Body)
	defer func() { _ = res.Body.Close() }()
	var ds DatasourceResponse
	err = dec.Decode(&ds)
	if err != nil {
		return nil, fmt.Errorf("could not decode datasource response: %w", err)
	}
	return &ds, nil
}

func (g *Grafana) QueryDatasource(ctx context.Context, d RequestDatasourceQuery) (*ResponseDatasourceQuery, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(d)
	if err != nil {
		return nil, fmt.Errorf("could not encode request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/ds/query", g.baseURL), &body)
	if err != nil {
		return nil, fmt.Errorf("could not buid request: %w", err)
	}
	req.WithContext(ctx)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-WEBAUTH-USER", "admin")
	res, err := g.htclient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not perform request: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		dump, _ := httputil.DumpResponse(res, true)
		fmt.Println(string(dump))
		return nil, fmt.Errorf("unexpected status code; expecting %d got %d", http.StatusOK, res.StatusCode)
	}
	dec := json.NewDecoder(res.Body)
	defer func() { _ = res.Body.Close() }()
	var ds ResponseDatasourceQuery
	err = dec.Decode(&ds)
	if err != nil {
		return nil, fmt.Errorf("could not decode query response: %w", err)
	}
	return &ds, nil
}

func (g *Grafana) EnsureDashboard(ctx context.Context, id string, title string) error {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/dashboards/uid/%s", g.baseURL, id), nil)
	if err != nil {
		return err
	}
	req.Header.Add("X-WEBAUTH-USER", "admin")
	res, err := g.htclient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusOK {
		return nil
	}
	if res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}
	err = g.CreateDashboard(ctx, CreateUpdateDashboard{
		Dashboard: Dashboard{
			Uid:           newString(id),
			Title:         title,
			Timezone:      "browser",
			SchemaVersion: 16,
			Version:       0,
			Refresh:       "15s",
		},
		Message:   "initial version",
		FolderId:  0,
		Overwrite: false,
	})
	if err != nil {
		return fmt.Errorf("could not create datasource: %w", err)
	}
	return nil
}

func (g *Grafana) ImportDashboard(ctx context.Context, dash DashboardImport) (*DashboardImportResponse, error) {
	d, err := ParseStructure(bytes.NewReader(dash.Dashboard))
	if err != nil {
		return nil, fmt.Errorf("could not parse dashboard structure: %w", err)
	}
	g.dashboards[d.Uid] = d.Title
	var body bytes.Buffer
	err = json.NewEncoder(&body).Encode(dash)
	if err != nil {
		return nil, fmt.Errorf("could not encode request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, g.baseURL+"/api/dashboards/import", &body)
	if err != nil {
		return nil, fmt.Errorf("could not buid request: %w", err)
	}
	req.WithContext(ctx)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-WEBAUTH-USER", "admin")
	res, err := g.htclient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not perform request: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		dump, _ := httputil.DumpResponse(res, true)
		fmt.Println(string(dump))
		return nil, fmt.Errorf("unexpected status code; expecting %d got %d", http.StatusOK, res.StatusCode)
	}
	var dir DashboardImportResponse
	defer func() { _ = res.Body.Close() }()
	err = json.NewDecoder(res.Body).Decode(&dir)
	if err != nil {
		return nil, fmt.Errorf("could not decode import response: %w", err)
	}
	return &dir, nil
}

func (g *Grafana) CreateDashboard(ctx context.Context, dashboard CreateUpdateDashboard) error {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(dashboard)
	if err != nil {
		return fmt.Errorf("could not encode request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, g.baseURL+"/api/dashboards/db", &body)
	if err != nil {
		return fmt.Errorf("could not buid request: %w", err)
	}
	req.WithContext(ctx)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-WEBAUTH-USER", "admin")
	res, err := g.htclient.Do(req)
	if err != nil {
		return fmt.Errorf("could not perform request: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		dump, _ := httputil.DumpResponse(res, true)
		fmt.Println(string(dump))
		return fmt.Errorf("unexpected status code; expecting %d got %d", http.StatusOK, res.StatusCode)
	}
	return nil
}

func newString(val string) *string {
	return &val
}
