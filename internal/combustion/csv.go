package combustion

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func WriteCSV(path string, dump ProbeDump) error {
	start := dump.SessionMeta.StartedAt
	if start == "" && len(dump.Rows) > 0 {
		start = dump.Rows[0].SampledAt
	}
	startTime, err := time.Parse(time.RFC3339Nano, start)
	if err != nil {
		return fmt.Errorf("parse session start time %q: %w", start, err)
	}
	var lines []string
	lines = append(lines,
		"Combustion Inc. Probe Data",
		"Source: Combustion cloud API chunked dump",
		"CSV version: 4",
		"Probe S/N: "+dump.Probe.Serial,
		fmt.Sprintf("Sample Period: %d", dump.Status.SamplePeriod),
		"Created: "+time.Now().UTC().Format(time.RFC3339Nano),
		"",
		"Timestamp,SessionID,SequenceNumber,T1,T2,T3,T4,T5,T6,T7,T8,VirtualCoreTemperature,VirtualSurfaceTemperature,VirtualAmbientTemperature,EstimatedCoreTemperature,PredictionSetPoint,VirtualCoreSensor,VirtualSurfaceSensor,VirtualAmbientSensor,PredictionState,PredictionMode,PredictionType,PredictionValueSeconds",
	)
	for _, row := range dump.Rows {
		line, err := rowCSV(startTime, dump.Status.SessionID, row)
		if err != nil {
			return err
		}
		lines = append(lines, line)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func rowCSV(start time.Time, sessionID int64, row DataRow) (string, error) {
	t, err := time.Parse(time.RFC3339Nano, row.SampledAt)
	if err != nil {
		return "", fmt.Errorf("parse sampled_at for sequence %d: %w", row.SequenceNumber, err)
	}
	seconds := t.Sub(start).Seconds()
	cells := []string{
		fmt.Sprintf("%.3f", seconds),
		strconv.FormatInt(sessionID, 10),
		strconv.Itoa(row.SequenceNumber),
		f(row.T1), f(row.T2), f(row.T3), f(row.T4), f(row.T5), f(row.T6), f(row.T7), f(row.T8),
		f(row.VirtualCore), f(row.VirtualSurface), f(row.VirtualAmbient), f(row.EstimatedCoreTemperature), f(row.PredictionSetPoint),
		i(row.VirtualCoreSensor), i(row.VirtualSurfaceSensor), i(row.VirtualAmbientSensor),
		i(row.PredictionState), i(row.PredictionMode), i(row.PredictionType), i(row.PredictionValueSeconds),
	}
	return strings.Join(cells, ","), nil
}

func f(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *v)
}

func i(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}
