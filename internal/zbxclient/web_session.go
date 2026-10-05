package zbxclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (c *Client) DownloadCombinedGraph(
	ctx context.Context,
	itemIDs []string,
	fromTime, toTime string,
	width, height int,
	outputPath string,
) error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("creating cookiejar: %w", err)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !c.profile.VerifySSL,
		},
	}

	client := &http.Client{
		Jar:       jar,
		Transport: tr,
		Timeout:   60 * time.Second,
	}

	serverBase := strings.TrimRight(c.profile.URL, "/")
	if strings.HasSuffix(serverBase, ".php") {
		u, err := url.Parse(serverBase)
		if err == nil {
			dir := path.Dir(u.Path)
			if dir == "/" || dir == "." {
				dir = ""
			}
			serverBase = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, dir)
		}
	}

	// 1. Login to web frontend if username/password are provided
	if c.profile.User != "" && c.profile.Password != "" {
		loginURL := serverBase + "/index.php"
		form := url.Values{
			"name":      {c.profile.User},
			"password":  {c.profile.Password},
			"autologin": {"1"},
			"enter":     {"Sign in"},
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
		if err != nil {
			return fmt.Errorf("creating login request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("web login failed: %w", err)
		}
		_ = resp.Body.Close()

		// 2. Sync timeselector
		syncURL := serverBase + "/zabbix.php?action=timeselector.update"
		syncForm := url.Values{
			"method": {"rangechange"},
			"from":   {fromTime},
			"to":     {toTime},
			"idx":    {"web.item.graph.filter"},
			"idx2":   {"0"},
		}

		syncReq, err := http.NewRequestWithContext(ctx, http.MethodPost, syncURL, strings.NewReader(syncForm.Encode()))
		if err == nil {
			syncReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if syncResp, err := client.Do(syncReq); err == nil {
				_ = syncResp.Body.Close()
			}
		}
	}

	// 3. Fetch chart PNG
	chartURL := serverBase + "/chart.php"
	params := url.Values{
		"from":        {fromTime},
		"to":          {toTime},
		"profileIdx":  {"web.item.graph.filter"},
		"profileIdx2": {"0"},
		"width":       {strconv.Itoa(width)},
		"height":      {strconv.Itoa(height)},
	}
	for i, id := range itemIDs {
		params.Set(fmt.Sprintf("itemids[%d]", i), id)
	}

	fullURL := chartURL + "?" + params.Encode()
	chartReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("creating chart request: %w", err)
	}
	if c.profile.Token != "" {
		chartReq.Header.Set("Authorization", "Bearer "+c.profile.Token)
	}

	chartResp, err := client.Do(chartReq)
	if err != nil {
		return fmt.Errorf("fetching chart: %w", err)
	}
	defer chartResp.Body.Close()

	if chartResp.StatusCode >= 400 {
		return fmt.Errorf("chart http status: %d", chartResp.StatusCode)
	}

	data, err := io.ReadAll(chartResp.Body)
	if err != nil {
		return fmt.Errorf("reading chart data: %w", err)
	}

	// Validate PNG magic bytes (\x89PNG)
	if len(data) < 4 || !bytes.Equal(data[:4], []byte{0x89, 'P', 'N', 'G'}) {
		return fmt.Errorf("zabbix chart export returned invalid image (not a PNG)")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("saving chart file: %w", err)
	}

	return nil
}
