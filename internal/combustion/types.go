package combustion

import (
	"encoding/json"
	"time"
)

type Config struct {
	APIKey      string
	AuthAgePath string
	AgeIdentity string
	OutDir      string
	Serials     map[string]bool
}

type AuthBundle struct {
	APIKey       string `json:"apiKey"`
	LocalID      string `json:"localId"`
	RefreshToken string `json:"refreshToken"`
	Email        string `json:"email"`
}

type Auth struct {
	LocalID      string
	IDToken      string
	RefreshToken string
	Email        string
	ExpiresAt    time.Time
}

type Probe struct {
	Serial    string `json:"serial"`
	DeviceKey string `json:"device_key"`
}

type Status struct {
	SessionID    int64 `json:"session_id"`
	SamplePeriod int64 `json:"sample_period"`
}

type SessionMeta struct {
	StartedAt            string          `json:"started_at"`
	SequenceNumberRanges [][]int         `json:"sequence_number_ranges"`
	Raw                  json.RawMessage `json:"-"`
}

type DataRow struct {
	SampledAt                string   `json:"sampled_at"`
	SequenceNumber           int      `json:"sequence_number"`
	T1                       *float64 `json:"t1"`
	T2                       *float64 `json:"t2"`
	T3                       *float64 `json:"t3"`
	T4                       *float64 `json:"t4"`
	T5                       *float64 `json:"t5"`
	T6                       *float64 `json:"t6"`
	T7                       *float64 `json:"t7"`
	T8                       *float64 `json:"t8"`
	VirtualCore              *float64 `json:"virtual_core"`
	VirtualSurface           *float64 `json:"virtual_surface"`
	VirtualAmbient           *float64 `json:"virtual_ambient"`
	EstimatedCoreTemperature *float64 `json:"estimated_core_temperature"`
	PredictionSetPoint       *float64 `json:"prediction_set_point"`
	VirtualCoreSensor        *int     `json:"virtual_core_sensor"`
	VirtualSurfaceSensor     *int     `json:"virtual_surface_sensor"`
	VirtualAmbientSensor     *int     `json:"virtual_ambient_sensor"`
	PredictionState          *int     `json:"prediction_state"`
	PredictionMode           *int     `json:"prediction_mode"`
	PredictionType           *int     `json:"prediction_type"`
	PredictionValueSeconds   *int     `json:"prediction_value_seconds"`
}

type ProbeDump struct {
	Probe       Probe       `json:"probe"`
	Status      Status      `json:"status"`
	SessionMeta SessionMeta `json:"meta"`
	Rows        []DataRow   `json:"rows"`
}

type ProbeSummary struct {
	Serial       string     `json:"serial"`
	SessionID    int64      `json:"session_id"`
	Ranges       [][]int    `json:"ranges"`
	ExpectedRows int        `json:"expected_rows"`
	Rows         int        `json:"rows"`
	MinSeq       int        `json:"min_seq"`
	MaxSeq       int        `json:"max_seq"`
	MissingCount int        `json:"missing_count"`
	Missing      []int      `json:"missing"`
	Chunks       []ChunkLog `json:"chunks"`
	JSON         string     `json:"json,omitempty"`
	CSV          string     `json:"csv,omitempty"`
}

type ChunkLog struct {
	Range [2]int `json:"range"`
	Rows  int    `json:"rows"`
}

type WindowProbe struct {
	Serial       string    `json:"serial"`
	SessionID    int64     `json:"session_id"`
	Rows         []DataRow `json:"rows"`
	RowsReturned int       `json:"rows_returned"`
	RowsInWindow int       `json:"rows_in_window"`
	Range        [2]int    `json:"range"`
	Ranges       [][]int   `json:"ranges"`
	Latest       *DataRow  `json:"latest,omitempty"`
}

type WindowSnapshot struct {
	GeneratedAt string        `json:"generated_at"`
	Minutes     float64       `json:"minutes"`
	Probes      []WindowProbe `json:"probes"`
}
