package combustion

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	project          = "combustion-production-apps"
	userKeyNamespace = "c6639a3c-0b0a-4dd9-8cc1-046a2da8a5f1"
	dataAPI          = "https://data-api.combustion.inc"
)

type Client struct {
	http   *http.Client
	apiKey string
	auth   Auth
}

func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	authBundle, err := readAuthBundle(cfg)
	if err != nil {
		return nil, err
	}
	apiKey := firstNonEmpty(cfg.APIKey, os.Getenv("CPT_FIREBASE_API_KEY"), authBundle.APIKey)
	if apiKey == "" {
		return nil, errors.New("set --api-key, CPT_FIREBASE_API_KEY, or apiKey in the Firebase auth bundle")
	}
	if authBundle.RefreshToken == "" {
		return nil, errors.New("Firebase auth bundle is missing refreshToken")
	}
	auth, err := refreshAuth(ctx, apiKey, authBundle)
	if err != nil {
		return nil, err
	}
	return &Client{http: &http.Client{Timeout: 30 * time.Second}, apiKey: apiKey, auth: auth}, nil
}

func readAuthBundle(cfg Config) (AuthBundle, error) {
	if cfg.AuthAgePath == "" {
		cfg.AuthAgePath = os.Getenv("CPT_FIREBASE_AUTH_AGE")
	}
	if cfg.AgeIdentity == "" {
		cfg.AgeIdentity = os.Getenv("CPT_AGE_IDENTITY")
	}
	if cfg.AgeIdentity == "" {
		cfg.AgeIdentity = os.Getenv("HOME") + "/.ssh/id_ed25519"
	}
	if cfg.AuthAgePath != "" {
		cmd := exec.Command("age", "-d", "-i", cfg.AgeIdentity, cfg.AuthAgePath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return AuthBundle{}, fmt.Errorf("age decrypt failed: %w: %s", err, strings.TrimSpace(string(out)))
		}
		var auth AuthBundle
		return auth, json.Unmarshal(out, &auth)
	}

	p := os.Getenv("CPT_FIREBASE_AUTH_JSON")
	if p == "" {
		return AuthBundle{}, errors.New("set --auth-age, CPT_FIREBASE_AUTH_AGE, or CPT_FIREBASE_AUTH_JSON")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return AuthBundle{}, err
	}
	var auth AuthBundle
	return auth, json.Unmarshal(b, &auth)
}

func refreshAuth(ctx context.Context, apiKey string, auth AuthBundle) (Auth, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", auth.RefreshToken)
	u := "https://securetoken.googleapis.com/v1/token?key=" + apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return Auth{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Auth{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Auth{}, fmt.Errorf("token refresh failed: %s: %s", res.Status, string(body))
	}
	var r struct {
		UserID       string `json:"user_id"`
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    string `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return Auth{}, err
	}
	if r.IDToken == "" {
		return Auth{}, errors.New("token refresh response is missing id_token")
	}
	expiresIn, _ := strconv.Atoi(r.ExpiresIn)
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	localID := first(r.UserID, auth.LocalID)
	if localID == "" {
		return Auth{}, errors.New("token refresh response is missing user_id/localId")
	}
	return Auth{
		LocalID:      localID,
		IDToken:      r.IDToken,
		RefreshToken: first(r.RefreshToken, auth.RefreshToken),
		Email:        auth.Email,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

func (c *Client) Probes(ctx context.Context, serials map[string]bool) ([]Probe, error) {
	userKey, err := uuidV5(userKeyNamespace, c.auth.LocalID)
	if err != nil {
		return nil, err
	}
	docURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/(default)/documents/users/%s", project, userKey)
	var doc firestoreDoc
	if err := c.getJSON(ctx, docURL, &doc); err != nil {
		return nil, err
	}
	user := firestoreObject(doc.Fields)
	raw, ok := user["associations"].([]any)
	if !ok {
		return nil, errors.New("user document has no associations array")
	}
	var probes []Probe
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok || m["type"] != "PROBE" {
			continue
		}
		serial, _ := m["serial_number"].(string)
		if len(serials) > 0 && !serials[serial] {
			continue
		}
		deviceKey, _ := m["device_key"].(string)
		if serial == "" || deviceKey == "" {
			return nil, fmt.Errorf("probe association is missing serial_number or device_key: %#v", m)
		}
		probes = append(probes, Probe{Serial: serial, DeviceKey: deviceKey})
	}
	sort.Slice(probes, func(i, j int) bool { return probes[i].Serial < probes[j].Serial })
	return probes, nil
}

func (c *Client) Status(ctx context.Context, probe Probe) (Status, error) {
	u := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/(default)/documents/probes/%s/probe_status/current", project, probe.DeviceKey)
	var doc firestoreDoc
	if err := c.getJSON(ctx, u, &doc); err != nil {
		return Status{}, err
	}
	m := firestoreObject(doc.Fields)
	sessionID, err := requiredNumber(m, "session_id")
	if err != nil {
		return Status{}, err
	}
	samplePeriod, err := requiredNumber(m, "sample_period")
	if err != nil {
		return Status{}, err
	}
	return Status{SessionID: int64(sessionID), SamplePeriod: int64(samplePeriod)}, nil
}

func (c *Client) SessionMeta(ctx context.Context, serial string, sessionID int64) (SessionMeta, error) {
	u, _ := url.Parse(dataAPI + "/v1/session")
	q := u.Query()
	q.Set("uid", c.auth.LocalID)
	q.Set("device_serial_number", serial)
	q.Set("device_type", "1")
	q.Set("device_session_id", strconv.FormatInt(sessionID, 10))
	u.RawQuery = q.Encode()
	body, err := c.get(ctx, u.String())
	if err != nil {
		return SessionMeta{}, err
	}
	var meta SessionMeta
	if err := json.Unmarshal(body, &meta); err != nil {
		return SessionMeta{}, err
	}
	meta.Raw = append([]byte(nil), body...)
	return meta, nil
}

func (c *Client) SessionData(ctx context.Context, serial string, sessionID int64, ranges [][]int) ([]DataRow, []ChunkLog, error) {
	bySeq := map[int]DataRow{}
	var logs []ChunkLog
	for _, r := range rangeChunks(ranges, 1000) {
		rows, err := c.sessionDataChunk(ctx, serial, sessionID, r)
		if err != nil {
			return nil, nil, err
		}
		for _, row := range rows {
			if !sequenceInRanges([][]int{r}, row.SequenceNumber) {
				return nil, nil, fmt.Errorf("session_data returned out-of-range sequence %d for requested range [%d,%d]", row.SequenceNumber, r[0], r[1])
			}
			if _, ok := bySeq[row.SequenceNumber]; ok {
				return nil, nil, fmt.Errorf("session_data returned duplicate sequence %d", row.SequenceNumber)
			}
			bySeq[row.SequenceNumber] = row
		}
		logs = append(logs, ChunkLog{Range: [2]int{r[0], r[1]}, Rows: len(rows)})
	}
	rows := make([]DataRow, 0, len(bySeq))
	for _, row := range bySeq {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].SequenceNumber < rows[j].SequenceNumber })
	return rows, logs, nil
}

func (c *Client) DumpProbe(ctx context.Context, probe Probe) (ProbeDump, []ChunkLog, error) {
	status, err := c.Status(ctx, probe)
	if err != nil {
		return ProbeDump{}, nil, err
	}
	return c.DumpSession(ctx, probe, status.SessionID, status.SamplePeriod)
}

func (c *Client) Window(ctx context.Context, minutes float64, serials map[string]bool) (WindowSnapshot, error) {
	probes, err := c.Probes(ctx, serials)
	if err != nil {
		return WindowSnapshot{}, err
	}
	cutoff := time.Now().Add(-time.Duration(minutes * float64(time.Minute)))
	snapshot := WindowSnapshot{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Minutes: minutes}
	for _, probe := range probes {
		status, err := c.Status(ctx, probe)
		if err != nil {
			return WindowSnapshot{}, err
		}
		meta, err := c.SessionMeta(ctx, probe.Serial, status.SessionID)
		if err != nil {
			return WindowSnapshot{}, err
		}
		ranges := latestRanges(meta.SequenceNumberRanges, status.SamplePeriod, minutes)
		wp := WindowProbe{Serial: probe.Serial, SessionID: status.SessionID, Ranges: ranges, Range: collapseRanges(ranges)}
		if len(ranges) == 0 {
			snapshot.Probes = append(snapshot.Probes, wp)
			continue
		}
		rows, _, err := c.SessionData(ctx, probe.Serial, status.SessionID, ranges)
		if err != nil {
			return WindowSnapshot{}, err
		}
		rowsReturned := len(rows)
		rows = filterRowsSince(rows, cutoff)
		wp.Rows = rows
		wp.RowsReturned = rowsReturned
		wp.RowsInWindow = len(rows)
		if len(rows) > 0 {
			latest := rows[len(rows)-1]
			wp.Latest = &latest
		}
		snapshot.Probes = append(snapshot.Probes, wp)
	}
	return snapshot, nil
}

func (c *Client) sessionDataChunk(ctx context.Context, serial string, sessionID int64, r []int) ([]DataRow, error) {
	rangesJSON, _ := json.Marshal([][]int{{r[0], r[1]}})
	u, _ := url.Parse(dataAPI + "/v1/session_data")
	q := u.Query()
	q.Set("uid", c.auth.LocalID)
	q.Set("device_serial_number", serial)
	q.Set("device_type", "1")
	q.Set("device_session_id", strconv.FormatInt(sessionID, 10))
	q.Set("sequence_number_ranges", string(rangesJSON))
	q.Set("page", "1")
	u.RawQuery = q.Encode()
	var resp struct {
		Data map[string]DataRow `json:"data"`
	}
	if err := c.getJSON(ctx, u.String(), &resp); err != nil {
		return nil, err
	}
	rows := make([]DataRow, 0, len(resp.Data))
	for _, row := range resp.Data {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].SequenceNumber < rows[j].SequenceNumber })
	return rows, nil
}

func (c *Client) getJSON(ctx context.Context, u string, v any) error {
	body, err := c.get(ctx, u)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	return c.getWithRetry(ctx, u, true)
}

func (c *Client) getWithRetry(ctx context.Context, u string, allowRefresh bool) ([]byte, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.auth.IDToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CI-AppVersion", "v3.2.4")
	req.Header.Set("CI-OSVersion", "35")
	req.Header.Set("CI-Locale", "en-US")
	req.Header.Set("CI-DateTime", time.Now().UTC().Format(time.RFC3339Nano))
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusUnauthorized && allowRefresh {
		auth, err := refreshAuth(ctx, c.apiKey, AuthBundle{LocalID: c.auth.LocalID, RefreshToken: c.auth.RefreshToken, Email: c.auth.Email})
		if err != nil {
			return nil, err
		}
		c.auth = auth
		return c.getWithRetry(ctx, u, false)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s failed: %s: %s", urlWithoutQuery(u), res.Status, string(body))
	}
	return body, nil
}

func urlWithoutQuery(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<unparseable-url>"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func (c *Client) ensureAuth(ctx context.Context) error {
	if time.Until(c.auth.ExpiresAt) > 5*time.Minute {
		return nil
	}
	auth, err := refreshAuth(ctx, c.apiKey, AuthBundle{LocalID: c.auth.LocalID, RefreshToken: c.auth.RefreshToken, Email: c.auth.Email})
	if err != nil {
		return err
	}
	c.auth = auth
	return nil
}

func rangeChunks(ranges [][]int, size int) [][]int {
	var out [][]int
	for _, r := range ranges {
		for start := r[0]; start <= r[1]; start += size {
			end := start + size - 1
			if end > r[1] {
				end = r[1]
			}
			out = append(out, []int{start, end})
		}
	}
	return out
}

func Summary(dump ProbeDump, chunks []ChunkLog, jsonPath, csvPath string) ProbeSummary {
	missing := Missing(dump.SessionMeta.SequenceNumberRanges, dump.Rows)
	minSeq, maxSeq := 0, 0
	if len(dump.Rows) > 0 {
		minSeq = dump.Rows[0].SequenceNumber
		maxSeq = dump.Rows[len(dump.Rows)-1].SequenceNumber
	}
	return ProbeSummary{
		Serial: dump.Probe.Serial, SessionID: dump.Status.SessionID, Ranges: dump.SessionMeta.SequenceNumberRanges,
		ExpectedRows: ExpectedCount(dump.SessionMeta.SequenceNumberRanges), Rows: len(dump.Rows),
		MinSeq: minSeq, MaxSeq: maxSeq, MissingCount: len(missing), Missing: missing,
		Chunks: chunks, JSON: jsonPath, CSV: csvPath,
	}
}

func ExpectedCount(ranges [][]int) int {
	n := 0
	for _, r := range ranges {
		n += r[1] - r[0] + 1
	}
	return n
}

func Missing(ranges [][]int, rows []DataRow) []int {
	seen := map[int]bool{}
	for _, row := range rows {
		seen[row.SequenceNumber] = true
	}
	missing := []int{}
	for _, r := range ranges {
		for i := r[0]; i <= r[1]; i++ {
			if !seen[i] {
				missing = append(missing, i)
			}
		}
	}
	return missing
}

func latestRange(ranges [][]int, samplePeriodMS int64, minutes float64) [2]int {
	return collapseRanges(latestRanges(ranges, samplePeriodMS, minutes))
}

func latestRanges(ranges [][]int, samplePeriodMS int64, minutes float64) [][]int {
	if samplePeriodMS <= 0 {
		samplePeriodMS = 5000
	}
	valid := make([][]int, 0, len(ranges))
	for _, r := range ranges {
		if len(r) < 2 || r[0] > r[1] {
			continue
		}
		valid = append(valid, []int{r[0], r[1]})
	}
	if len(valid) == 0 {
		return nil
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i][1] > valid[j][1] })

	needed := int((minutes*60*1000)/float64(samplePeriodMS)) + 12
	if needed <= 0 {
		needed = 1
	}
	selected := [][]int{}
	for _, r := range valid {
		if needed <= 0 {
			break
		}
		count := r[1] - r[0] + 1
		start := r[0]
		if count > needed {
			start = r[1] - needed + 1
			count = needed
		}
		selected = append(selected, []int{start, r[1]})
		needed -= count
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i][0] < selected[j][0] })
	return selected
}

func collapseRanges(ranges [][]int) [2]int {
	if len(ranges) == 0 {
		return [2]int{}
	}
	start, end := ranges[0][0], ranges[0][1]
	for _, r := range ranges[1:] {
		if r[0] < start {
			start = r[0]
		}
		if r[1] > end {
			end = r[1]
		}
	}
	return [2]int{start, end}
}

func sequenceInRanges(ranges [][]int, seq int) bool {
	for _, r := range ranges {
		if len(r) >= 2 && seq >= r[0] && seq <= r[1] {
			return true
		}
	}
	return false
}

func filterRowsSince(rows []DataRow, cutoff time.Time) []DataRow {
	var out []DataRow
	for _, row := range rows {
		t, err := time.Parse(time.RFC3339Nano, row.SampledAt)
		if err == nil && !t.Before(cutoff) {
			out = append(out, row)
		}
	}
	return out
}

func requiredNumber(m map[string]any, key string) (float64, error) {
	v, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("missing required numeric field %s", key)
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case int64:
		return float64(n), nil
	case int:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("field %s is not numeric: %T", key, v)
	}
}

type firestoreDoc struct {
	Fields map[string]firestoreValue `json:"fields"`
}

type firestoreValue struct {
	StringValue    *string           `json:"stringValue,omitempty"`
	IntegerValue   *string           `json:"integerValue,omitempty"`
	DoubleValue    *float64          `json:"doubleValue,omitempty"`
	BooleanValue   *bool             `json:"booleanValue,omitempty"`
	TimestampValue *string           `json:"timestampValue,omitempty"`
	ArrayValue     *firestoreArray   `json:"arrayValue,omitempty"`
	MapValue       *firestoreObjectV `json:"mapValue,omitempty"`
}

type firestoreArray struct {
	Values []firestoreValue `json:"values"`
}

type firestoreObjectV struct {
	Fields map[string]firestoreValue `json:"fields"`
}

func firestoreObject(fields map[string]firestoreValue) map[string]any {
	out := map[string]any{}
	for k, v := range fields {
		out[k] = firestoreAny(v)
	}
	return out
}

func firestoreAny(v firestoreValue) any {
	switch {
	case v.StringValue != nil:
		return *v.StringValue
	case v.IntegerValue != nil:
		n, _ := strconv.ParseFloat(*v.IntegerValue, 64)
		return n
	case v.DoubleValue != nil:
		return *v.DoubleValue
	case v.BooleanValue != nil:
		return *v.BooleanValue
	case v.TimestampValue != nil:
		return *v.TimestampValue
	case v.ArrayValue != nil:
		out := make([]any, 0, len(v.ArrayValue.Values))
		for _, item := range v.ArrayValue.Values {
			out = append(out, firestoreAny(item))
		}
		return out
	case v.MapValue != nil:
		return firestoreObject(v.MapValue.Fields)
	default:
		return nil
	}
}

func uuidV5(namespace, name string) (string, error) {
	ns, err := hex.DecodeString(strings.ReplaceAll(namespace, "-", ""))
	if err != nil {
		return "", err
	}
	h := sha1.New()
	h.Write(ns)
	h.Write([]byte(name))
	sum := h.Sum(nil)[:16]
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80
	var b bytes.Buffer
	fmt.Fprintf(&b, "%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:])
	return strings.ToUpper(b.String()), nil
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func number(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	default:
		return 0
	}
}
