package csvaudit

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AuditEvent represents a single row from the manually-exported CSV audit log.
type AuditEvent struct {
	CreatedAt      time.Time
	Event          string
	ClientPlatform string
	ActorName      string
	ActorEmail     string
	ActorUUID      string
}

// UserSummary aggregates events per user from the CSV export.
type UserSummary struct {
	Name       string
	Email      string
	UUID       string
	EventCount int
	EventTypes map[string]int
	FirstSeen  time.Time
	LastSeen   time.Time
	ActiveDays map[string]bool
}

// ParseCSV reads a CSV or ZIP file and returns parsed audit events.
// For ZIP files, the largest CSV inside the archive is used.
func ParseCSV(path string) ([]AuditEvent, error) {
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		return parseZip(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseReader(f)
}

func parseZip(path string) ([]AuditEvent, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()
	// Find the largest CSV in the archive.
	var best *zip.File
	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
			if best == nil || f.UncompressedSize64 > best.UncompressedSize64 {
				best = f
			}
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no CSV files found in %s", filepath.Base(path))
	}

	rc, err := best.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return parseReader(rc)
}

func parseReader(r io.Reader) ([]AuditEvent, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}

	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[h] = i
	}

	// Verify required columns exist.
	for _, col := range []string{"created_at", "event", "actor_info"} {
		if _, ok := idx[col]; !ok {
			return nil, fmt.Errorf("missing required column %q in CSV", col)
		}
	}

	var events []AuditEvent
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return events, fmt.Errorf("reading CSV row: %w", err)
		}

		ts, err := time.Parse(time.RFC3339Nano, strings.Replace(record[idx["created_at"]], "Z", "+00:00", 1))
		if err != nil {
			// Try parsing as-is if the replacement didn't help.
			ts, err = time.Parse(time.RFC3339Nano, record[idx["created_at"]])
			if err != nil {
				continue
			}
		}

		name, email, uuid := parseActorInfo(record[idx["actor_info"]])

		var platform string
		if i, ok := idx["client_platform"]; ok {
			platform = record[i]
		}

		events = append(events, AuditEvent{
			CreatedAt:      ts,
			Event:          record[idx["event"]],
			ClientPlatform: platform,
			ActorName:      name,
			ActorEmail:     strings.ToLower(email),
			ActorUUID:      uuid,
		})
	}

	return events, nil
}

// parseActorInfo extracts name, email, and UUID from the Python dict literal
// format used in the CSV export. The field looks like:
//
//	{'name': 'Jane Doe', 'uuid': 'abc-123', 'metadata': {'email_address': 'jane@example.com'}}
//
// Names may contain apostrophes (e.g., "Reece O'Sullivan"), so a simple
// quote-replacement strategy won't work. Instead we do key-specific extraction.
func parseActorInfo(s string) (name, email, uuid string) {
	name = extractValue(s, "'name': '")
	uuid = extractValue(s, "'uuid': '")
	email = extractValue(s, "'email_address': '")
	return
}

// extractValue pulls the string value following the given key prefix in a
// Python dict literal. It handles escaped single quotes within values by
// looking for the closing pattern "'" followed by either , or }.
func extractValue(s, key string) string {
	i := strings.Index(s, key)
	if i < 0 {
		return ""
	}
	start := i + len(key)
	// Walk forward looking for an unescaped closing quote. In practice the
	// Python export doesn't escape quotes — names with apostrophes just have
	// a bare ' in the middle. We rely on the fact that the closing delimiter
	// is always "', " or "'}" to distinguish value-internal apostrophes.
	for j := start; j < len(s); j++ {
		if s[j] == '\'' {
			// Check if this quote is followed by a delimiter.
			rest := s[j+1:]
			if len(rest) == 0 || rest[0] == ',' || rest[0] == '}' {
				return s[start:j]
			}
		}
	}
	// If no clean delimiter found, take everything to the end.
	return s[start:]
}

// SummarizeByUser groups parsed CSV events by email and computes per-user stats.
func SummarizeByUser(events []AuditEvent) map[string]*UserSummary {
	summaries := make(map[string]*UserSummary)

	for _, e := range events {
		if e.ActorEmail == "" {
			continue
		}

		s, ok := summaries[e.ActorEmail]
		if !ok {
			s = &UserSummary{
				Name:       e.ActorName,
				Email:      e.ActorEmail,
				UUID:       e.ActorUUID,
				EventTypes: make(map[string]int),
				ActiveDays: make(map[string]bool),
			}
			summaries[e.ActorEmail] = s
		}

		s.EventCount++
		s.EventTypes[e.Event]++

		if s.FirstSeen.IsZero() || e.CreatedAt.Before(s.FirstSeen) {
			s.FirstSeen = e.CreatedAt
		}
		if e.CreatedAt.After(s.LastSeen) {
			s.LastSeen = e.CreatedAt
		}
		s.ActiveDays[e.CreatedAt.Format("2006-01-02")] = true
	}

	return summaries
}
