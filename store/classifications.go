package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
)

// InsertClassification upserts a single classification. Returns true if a new row
// was inserted, false if an existing row was updated.
func (s *Store) InsertClassification(c *compliance.Classification) (bool, error) {
	var workRelated *int
	if c.WorkRelated != nil {
		val := 0
		if *c.WorkRelated {
			val = 1
		}
		workRelated = &val
	}

	result, err := s.db.Exec(`
		INSERT INTO classifications (
			message_id, chat_id, user_email, message_created,
			work_related, intent, topic_fine, topic_coarse,
			classified_at, classifier_model
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(message_id) DO UPDATE SET
			work_related = COALESCE(excluded.work_related, classifications.work_related),
			intent = COALESCE(excluded.intent, classifications.intent),
			topic_fine = COALESCE(excluded.topic_fine, classifications.topic_fine),
			topic_coarse = COALESCE(excluded.topic_coarse, classifications.topic_coarse),
			classified_at = excluded.classified_at,
			classifier_model = excluded.classifier_model
	`, c.MessageID, c.ChatID, strings.ToLower(c.UserEmail), c.MessageCreated,
		workRelated, string(c.Intent), string(c.TopicFine), string(c.TopicCoarse),
		c.ClassifiedAt, c.ClassifierModel)

	if err != nil {
		return false, fmt.Errorf("inserting classification: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("checking rows affected: %w", err)
	}
	return rows > 0, nil
}

// InsertClassifications batch inserts multiple classifications.
// Returns the number of newly inserted rows.
func (s *Store) InsertClassifications(classifications []*compliance.Classification) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO classifications (
			message_id, chat_id, user_email, message_created,
			work_related, intent, topic_fine, topic_coarse,
			classified_at, classifier_model
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(message_id) DO UPDATE SET
			work_related = COALESCE(excluded.work_related, classifications.work_related),
			intent = COALESCE(excluded.intent, classifications.intent),
			topic_fine = COALESCE(excluded.topic_fine, classifications.topic_fine),
			topic_coarse = COALESCE(excluded.topic_coarse, classifications.topic_coarse),
			classified_at = excluded.classified_at,
			classifier_model = excluded.classifier_model
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, c := range classifications {
		var workRelated *int
		if c.WorkRelated != nil {
			val := 0
			if *c.WorkRelated {
				val = 1
			}
			workRelated = &val
		}

		res, err := stmt.Exec(
			c.MessageID, c.ChatID, strings.ToLower(c.UserEmail), c.MessageCreated,
			workRelated, string(c.Intent), string(c.TopicFine), string(c.TopicCoarse),
			c.ClassifiedAt, c.ClassifierModel)
		if err != nil {
			return inserted, fmt.Errorf("inserting classification %s: %w", c.MessageID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return inserted, fmt.Errorf("checking rows affected for %s: %w", c.MessageID, err)
		}
		inserted += int(n)
	}

	return inserted, tx.Commit()
}

// StoredClassification represents a classification as stored in the database.
type StoredClassification struct {
	MessageID       string
	ChatID          string
	UserEmail       string
	MessageCreated  string
	WorkRelated     *bool
	Intent          string
	TopicFine       string
	TopicCoarse     string
	ClassifiedAt    string
	ClassifierModel string
}

// GetClassification retrieves a single classification by message ID.
func (s *Store) GetClassification(messageID string) (*StoredClassification, error) {
	var c StoredClassification
	var workRelated *int

	err := s.db.QueryRow(`
		SELECT message_id, chat_id, user_email, message_created,
			   work_related, intent, topic_fine, topic_coarse,
			   classified_at, classifier_model
		FROM classifications
		WHERE message_id = ?
	`, messageID).Scan(
		&c.MessageID, &c.ChatID, &c.UserEmail, &c.MessageCreated,
		&workRelated, &c.Intent, &c.TopicFine, &c.TopicCoarse,
		&c.ClassifiedAt, &c.ClassifierModel)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if workRelated != nil {
		val := *workRelated == 1
		c.WorkRelated = &val
	}

	return &c, nil
}

// ClassificationQueryOpts specifies filters for querying classifications.
type ClassificationQueryOpts struct {
	ChatID    string
	UserEmail string
	Since     *time.Time
	Until     *time.Time
	Limit     int
}

// GetClassifications retrieves classifications with optional filters.
func (s *Store) GetClassifications(opts ClassificationQueryOpts) ([]StoredClassification, error) {
	var clauses []string
	var args []interface{}

	if opts.ChatID != "" {
		clauses = append(clauses, "chat_id = ?")
		args = append(args, opts.ChatID)
	}
	if opts.UserEmail != "" {
		clauses = append(clauses, "user_email = ?")
		args = append(args, strings.ToLower(opts.UserEmail))
	}
	if opts.Since != nil {
		clauses = append(clauses, "message_created >= ?")
		args = append(args, opts.Since.Format(time.RFC3339Nano))
	}
	if opts.Until != nil {
		clauses = append(clauses, "message_created < ?")
		args = append(args, opts.Until.Format(time.RFC3339Nano))
	}

	q := `SELECT message_id, chat_id, user_email, message_created,
	             work_related, intent, topic_fine, topic_coarse,
	             classified_at, classifier_model
	      FROM classifications`
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY message_created DESC"
	if opts.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []StoredClassification
	for rows.Next() {
		var c StoredClassification
		var workRelated *int
		if err := rows.Scan(
			&c.MessageID, &c.ChatID, &c.UserEmail, &c.MessageCreated,
			&workRelated, &c.Intent, &c.TopicFine, &c.TopicCoarse,
			&c.ClassifiedAt, &c.ClassifierModel,
		); err != nil {
			return results, err
		}
		if workRelated != nil {
			val := *workRelated == 1
			c.WorkRelated = &val
		}
		results = append(results, c)
	}

	return results, rows.Err()
}

// ClassificationSummary holds aggregated classification statistics.
type ClassificationSummary struct {
	TotalMessages   int
	WorkRelated     int
	NonWorkRelated  int
	WorkUnknown     int
	IntentAsking    int
	IntentDoing     int
	IntentExpressing int
	IntentUnknown   int
	TopicCounts     map[string]int
	CoarseTopicCounts map[string]int
}

// GetClassificationSummary returns aggregated statistics for classifications.
func (s *Store) GetClassificationSummary(opts ClassificationQueryOpts) (*ClassificationSummary, error) {
	classifications, err := s.GetClassifications(opts)
	if err != nil {
		return nil, err
	}

	summary := &ClassificationSummary{
		TotalMessages:     len(classifications),
		TopicCounts:       make(map[string]int),
		CoarseTopicCounts: make(map[string]int),
	}

	for _, c := range classifications {
		// Work/non-work
		if c.WorkRelated != nil {
			if *c.WorkRelated {
				summary.WorkRelated++
			} else {
				summary.NonWorkRelated++
			}
		} else {
			summary.WorkUnknown++
		}

		// Intent
		switch c.Intent {
		case "asking":
			summary.IntentAsking++
		case "doing":
			summary.IntentDoing++
		case "expressing":
			summary.IntentExpressing++
		default:
			if c.Intent != "" {
				summary.IntentUnknown++
			}
		}

		// Topics
		if c.TopicFine != "" {
			summary.TopicCounts[c.TopicFine]++
		}
		if c.TopicCoarse != "" {
			summary.CoarseTopicCounts[c.TopicCoarse]++
		}
	}

	return summary, nil
}

// ClassificationCount returns the total number of stored classifications.
func (s *Store) ClassificationCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM classifications").Scan(&count)
	return count, err
}

// ClassifiedChatCount returns the number of distinct chats with classifications.
func (s *Store) ClassifiedChatCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(DISTINCT chat_id) FROM classifications").Scan(&count)
	return count, err
}

// ClassifiedUserCount returns the number of distinct users with classifications.
func (s *Store) ClassifiedUserCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(DISTINCT user_email) FROM classifications").Scan(&count)
	return count, err
}

// UnclassifiedMessageCount returns the number of user messages that haven't been classified
// for a given chat. This requires knowing the total user messages in the chat.
func (s *Store) ClassifiedMessageCountForChat(chatID string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM classifications WHERE chat_id = ?", chatID).Scan(&count)
	return count, err
}
