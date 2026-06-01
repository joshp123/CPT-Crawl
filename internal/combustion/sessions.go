package combustion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

type Session struct {
	ID                   string         `json:"id"`
	UID                  string         `json:"uid"`
	DeviceSerialNumber   string         `json:"device_serial_number"`
	DeviceType           int            `json:"device_type"`
	DeviceSessionID      string         `json:"device_session_id"`
	SamplePeriod         int64          `json:"sample_period"`
	StartedAt            string         `json:"started_at"`
	EndedAt              *string        `json:"ended_at"`
	SequenceNumberRanges [][]int        `json:"sequence_number_ranges"`
	Source               map[string]any `json:"source,omitempty"`
}

func (c *Client) Sessions(ctx context.Context, probe Probe) ([]Session, error) {
	const pageSize = 100
	const maxPages = 100
	var sessions []Session
	for page := 1; page <= maxPages; page++ {
		u, _ := url.Parse(dataAPI + "/v1/sessions")
		q := u.Query()
		q.Set("uid", c.auth.LocalID)
		q.Set("device_serial_number", probe.Serial)
		q.Set("device_type", "1")
		q.Set("page", strconv.Itoa(page))
		q.Set("page_size", strconv.Itoa(pageSize))
		u.RawQuery = q.Encode()

		var resp struct {
			Page       int               `json:"page"`
			PageSize   int               `json:"page_size"`
			TotalPages int               `json:"total_pages"`
			Sessions   []json.RawMessage `json:"sessions"`
		}
		if err := c.getJSON(ctx, u.String(), &resp); err != nil {
			return nil, err
		}
		if resp.Page != 0 && resp.Page != page {
			return nil, fmt.Errorf("sessions page mismatch for %s: got page %d, want %d", probe.Serial, resp.Page, page)
		}
		for _, raw := range resp.Sessions {
			session, err := decodeSession(raw)
			if err != nil {
				return nil, fmt.Errorf("decode session index item for %s page %d: %w", probe.Serial, page, err)
			}
			sessions = append(sessions, session)
		}
		if resp.TotalPages <= 0 {
			if len(resp.Sessions) == pageSize {
				return nil, fmt.Errorf("sessions response for %s omitted total_pages on a full page", probe.Serial)
			}
			break
		}
		if page >= resp.TotalPages {
			break
		}
		if page == maxPages {
			return nil, fmt.Errorf("sessions for %s exceeded max page guard %d", probe.Serial, maxPages)
		}
	}
	return sessions, nil
}

func decodeSession(raw json.RawMessage) (Session, error) {
	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return Session{}, err
	}
	if err := json.Unmarshal(raw, &session.Source); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (c *Client) DumpSession(ctx context.Context, probe Probe, sessionID int64, samplePeriod int64) (ProbeDump, []ChunkLog, error) {
	meta, err := c.SessionMeta(ctx, probe.Serial, sessionID)
	if err != nil {
		return ProbeDump{}, nil, err
	}
	rows, chunks, err := c.SessionData(ctx, probe.Serial, sessionID, meta.SequenceNumberRanges)
	if err != nil {
		return ProbeDump{}, nil, err
	}
	status := Status{SessionID: sessionID, SamplePeriod: samplePeriod}
	return ProbeDump{Probe: probe, Status: status, SessionMeta: meta, Rows: rows}, chunks, nil
}
