package compliance

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// ActivityQuery specifies filters for fetching activities.
type ActivityQuery struct {
	CreatedAtGte  *time.Time                // created_at.gte filter
	CreatedAtLt   *time.Time                // created_at.lt filter
	ActorIDs      []string                  // actor_ids[] filter
	ActivityTypes []string                  // activity_types[] filter
	Limit         int                       // per-page limit (default 5000, API max 5000)
	BeforeID      string                    // for incremental fetch: get activities newer than this ID
	AfterID       string                    // resume backfill: start pagination from this cursor
	OnPage        func(PageResult) error    // called after each page; use to persist incrementally
}

// PageResult is passed to the OnPage callback after each API response page.
type PageResult struct {
	Activities []Activity // filtered activities from this page (no compliance_api_accessed)
	Page       int        // 1-indexed page number
	Total      int        // running total of activities fetched so far
	OldestDate string     // date (YYYY-MM-DD) of the oldest activity on this page
	NewestDate string     // date (YYYY-MM-DD) of the newest activity on this page
	TargetDate string     // target date we're fetching toward (from CreatedAtGte), or ""
	Done       bool       // true if this is the last page
}

// FetchActivities retrieves all activities matching the query, paginating
// automatically. Results are returned in reverse chronological order (newest
// first). Compliance API access events are filtered out since they represent
// our own footprint rather than user activity.
//
// If opts.OnPage is non-nil, it is called after each page is fetched. This
// allows the caller to persist results incrementally rather than waiting for
// the entire fetch to complete. If OnPage returns an error, pagination stops
// and that error is returned.
func (c *Client) FetchActivities(ctx context.Context, opts ActivityQuery) ([]Activity, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 5000 {
		limit = 5000
	}

	var all []Activity
	totalCount := 0
	page := 0
	afterID := opts.AfterID // cursor for backward pagination (older results); may resume from a previous run

	// Forward mode paginates toward the present using before_id/first_id.
	// Backward mode (default) paginates into the past using after_id/last_id.
	forwardMode := opts.BeforeID != ""
	beforeID := opts.BeforeID

	targetDate := ""
	if opts.CreatedAtGte != nil {
		targetDate = opts.CreatedAtGte.Format("2006-01-02")
	}

	for {
		params := url.Values{}
		params.Set("limit", fmt.Sprintf("%d", limit))

		if c.orgID != "" {
			params.Add("organization_ids[]", c.orgID)
		}
		if opts.CreatedAtGte != nil {
			params.Set("created_at.gte", opts.CreatedAtGte.Format(time.RFC3339))
		}
		if opts.CreatedAtLt != nil {
			params.Set("created_at.lt", opts.CreatedAtLt.Format(time.RFC3339))
		}
		for _, id := range opts.ActorIDs {
			params.Add("actor_ids[]", id)
		}
		for _, t := range opts.ActivityTypes {
			params.Add("activity_types[]", t)
		}

		if forwardMode && beforeID != "" {
			params.Set("before_id", beforeID)
		}
		if afterID != "" {
			params.Set("after_id", afterID)
		}

		var resp ActivitiesResponse
		if err := c.get(ctx, "/v1/compliance/activities", params, &resp); err != nil {
			return all, fmt.Errorf("page %d: %w", page, err)
		}

		var pageActivities []Activity
		for i := range resp.Data {
			if resp.Data[i].Type == "compliance_api_accessed" {
				continue
			}
			pageActivities = append(pageActivities, resp.Data[i])
		}
		totalCount += len(pageActivities)

		// When an OnPage callback is provided, the caller is responsible for
		// persisting results and we avoid accumulating the full history in
		// memory. Without a callback we collect everything for the return value.
		if opts.OnPage == nil {
			all = append(all, pageActivities...)
		}

		page++

		oldestDate := ""
		newestDate := ""
		if len(resp.Data) > 0 {
			if ts := resp.Data[len(resp.Data)-1].CreatedAt; len(ts) >= 10 {
				oldestDate = ts[:10]
			}
			if ts := resp.Data[0].CreatedAt; len(ts) >= 10 {
				newestDate = ts[:10]
			}
		}

		var done bool
		if forwardMode {
			done = !resp.HasMore || resp.FirstID == ""
		} else {
			done = !resp.HasMore || resp.LastID == ""
		}

		// Progress output: show the frontier date for the current direction.
		reachedDate := oldestDate
		if forwardMode {
			reachedDate = newestDate
		}
		targetStr := ""
		if targetDate != "" {
			targetStr = fmt.Sprintf(", target %s", targetDate)
		}
		fmt.Fprintf(os.Stderr, "  Page %d: %d activities, reached %s%s\n",
			page, totalCount, reachedDate, targetStr)

		if opts.OnPage != nil && len(pageActivities) > 0 {
			pr := PageResult{
				Activities: pageActivities,
				Page:       page,
				Total:      totalCount,
				OldestDate: oldestDate,
				NewestDate: newestDate,
				TargetDate: targetDate,
				Done:       done,
			}
			if err := opts.OnPage(pr); err != nil {
				return all, fmt.Errorf("OnPage callback (page %d): %w", page, err)
			}
		}

		if done {
			break
		}

		if forwardMode {
			beforeID = resp.FirstID
		} else {
			afterID = resp.LastID
		}
	}

	return all, nil
}

// SummarizeByUser groups a slice of activities by actor email and computes
// per-user aggregate metrics. This is a pure function with no I/O.
func SummarizeByUser(activities []Activity) map[string]*UserActivitySummary {
	summaries := make(map[string]*UserActivitySummary)

	for _, a := range activities {
		email := actorEmail(&a.Actor)
		if email == "" {
			continue
		}
		email = strings.ToLower(email)

		s, ok := summaries[email]
		if !ok {
			s = &UserActivitySummary{
				Email:      email,
				EventTypes: make(map[string]int),
				ActiveDays: make(map[string]bool),
			}
			summaries[email] = s
		}

		if a.Actor.UserID != nil && s.UserID == "" {
			s.UserID = *a.Actor.UserID
		}

		s.EventCount++
		s.EventTypes[a.Type]++

		t, err := a.CreatedAtTime()
		if err != nil {
			continue
		}
		if s.FirstSeen.IsZero() || t.Before(s.FirstSeen) {
			s.FirstSeen = t
		}
		if t.After(s.LastSeen) {
			s.LastSeen = t
		}
		s.ActiveDays[t.Format("2006-01-02")] = true
	}

	return summaries
}

// actorEmail extracts the best available email from an actor, regardless of
// actor type.
func actorEmail(a *Actor) string {
	if a.EmailAddress != nil {
		return *a.EmailAddress
	}
	if a.UnauthenticatedEmailAddress != nil {
		return *a.UnauthenticatedEmailAddress
	}
	return ""
}
