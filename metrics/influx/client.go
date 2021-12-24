package influx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
)

type Logger interface {
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
}

type CallOpts struct {
	Verbose bool
}

type Client struct {
	httpClient http.Client
	addr       string
	token      string
}

func (c *Client) Ready(ctx context.Context) error {
	_, err := c.getWithBackoff(ctx, c.addr+"/api/v2/ready")
	if err != nil {
		return err
	}
	// no error means http.StatusOK
	return nil
}

func (c *Client) Health(ctx context.Context) error {
	_, err := c.getWithBackoff(ctx, c.addr+"/api/v2/health")
	if err != nil {
		return err
	}
	// no error means http.StatusOK
	return nil
}

func (c *Client) getWithBackoff(ctx context.Context, url string) (*http.Response, error) {
	back := backoff.WithContext(backoff.WithMaxRetries(backoff.NewConstantBackOff(3*time.Second), 20), ctx)
	var res *http.Response
	err := backoff.Retry(func() error {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		if c.token != "" {
			req.Header.Add("Authorization", fmt.Sprintf("Token %s", c.token))
		}
		response, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code (%d)", response.StatusCode)
		}
		res = response
		return nil
	}, back)
	return res, err
}

func (c *Client) NeedSetup(ctx context.Context) (bool, error) {
	url := c.addr + "/api/v2/setup"
	resp, err := c.getWithBackoff(ctx, url)
	if err != nil {
		return false, fmt.Errorf("error during setup check: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected setup check response status code (%d)", resp.StatusCode)
	}
	status := struct {
		Allowed bool `json:"allowed"`
	}{}
	dec := json.NewDecoder(resp.Body)
	defer func() { _ = resp.Body.Close() }()
	err = dec.Decode(&status)
	if err != nil {
		return false, fmt.Errorf("could not decode response: %w", err)
	}
	return status.Allowed, nil
}

func (c *Client) Setup(username, password, org, bucket string, retention time.Duration, opts ...CallOpts) (string, error) {
	verbose := false
	if len(opts) > 0 {
		verbose = opts[0].Verbose
	}
	setup := struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Org      string `json:"org"`
		Bucket   string `json:"bucket"`
		//RetentionPeriod int64  `json:"retentionPeriodSeconds"`
		RetentionPeriod time.Duration `json:"retentionPeriodHrs"`
	}{
		Username:        username,
		Password:        password,
		Org:             org,
		Bucket:          bucket,
		RetentionPeriod: retention,
	}
	body, _ := json.Marshal(setup)
	req, _ := http.NewRequest(http.MethodPost, c.addr+"/api/v2/setup", bytes.NewBuffer(body))
	req.Header.Add("Content-MsgType", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Token %s", c.token))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error during setup call: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		if verbose {
			defer func() { _ = resp.Body.Close() }()
			_, _ = io.Copy(os.Stderr, resp.Body)
		}
		return "", fmt.Errorf("unexpected setup call response status code (%d)", resp.StatusCode)
	}
	token := struct {
		Auth struct {
			Token string `json:"token"`
		} `json:"auth"`
	}{}
	dec := json.NewDecoder(resp.Body)
	defer func() { _ = resp.Body.Close() }()
	err = dec.Decode(&token)
	if err != nil {
		return "", fmt.Errorf("could not decode auth token: %w", err)
	}
	return token.Auth.Token, nil
}

func (c *Client) SignOut() {
	// do nothing
}

func (c *Client) WriteMeasurement(_ context.Context, org, bucket, measurement string, fields map[string]interface{}, tags map[string]string, timestamp time.Time, logger Logger, opts ...CallOpts) error {
	verbose := false
	if len(opts) > 0 {
		verbose = opts[0].Verbose
	}
	var builder strings.Builder
	builder.WriteString(measurement)
	for key, val := range tags {
		builder.WriteString(",")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(formatValue(val))
	}
	builder.WriteString(" ")
	first := true
	for field, val := range fields {
		if !first {
			builder.WriteString(",")
		} else {
			first = false
		}
		builder.WriteString(field)
		builder.WriteString("=")
		builder.WriteString(formatValue(val))
	}
	builder.WriteString(" ")
	builder.WriteString(strconv.Itoa(int(timestamp.UnixNano())))
	builder.WriteString("\n")
	logger.Debugf("writing protocol line: %s", builder.String())
	req, _ := http.NewRequest(http.MethodPost, c.addr+"/api/v2/write", bytes.NewBufferString(builder.String()))
	q := req.URL.Query()
	q.Add("bucket", bucket)
	q.Add("org", org)
	req.URL.RawQuery = q.Encode()
	req.Header.Add("Content-MsgType", "text/plain; charset=utf-8")
	req.Header.Add("Content-Encoding", "identity")
	req.Header.Add("Authorization", fmt.Sprintf("Token %s", c.token))
	req.Header.Add("Accept", "application/json")
	//rd, _ := httputil.DumpRequest(req, true)
	//os.Stderr.WriteString(string(rd))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error during setup call: %w", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		if verbose {
			defer func() { _ = resp.Body.Close() }()
			_, _ = io.Copy(os.Stderr, resp.Body)
		}
		return fmt.Errorf("unexpected status code (%d)", resp.StatusCode)
	}
	return nil
}

func formatValue(val interface{}) string {
	switch typed := val.(type) {
	case int:
		return fmt.Sprintf("%di", typed)
	case string:
		return fmt.Sprintf(`"%s"`, typed)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', 2, 32)
	case float64:
		return strconv.FormatFloat(typed, 'f', 2, 32)
	case bool:
		if typed {
			return "T"
		}
		return "F"
	default:
		return fmt.Sprintf("%v", val)
	}
}
