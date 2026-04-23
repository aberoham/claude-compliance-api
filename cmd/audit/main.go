package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/aberoham/claude-compliance-api/analytics"
	"github.com/aberoham/claude-compliance-api/compliance"
	"github.com/aberoham/claude-compliance-api/csvaudit"
	"github.com/aberoham/claude-compliance-api/okta"
	"github.com/aberoham/claude-compliance-api/store"
)

func main() {
	loadDotenv()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "fetch":
		cmdFetch(os.Args[2:])
	case "users":
		cmdUsers(os.Args[2:])
	case "compare":
		cmdCompare(os.Args[2:])
	case "projects":
		cmdProjects(os.Args[2:])
	case "project":
		cmdProject(os.Args[2:])
	case "chats":
		cmdChats(os.Args[2:])
	case "chat":
		cmdChat(os.Args[2:])
	case "file":
		cmdFile(os.Args[2:])
	case "chatanalysis":
		cmdChatAnalysis(os.Args[2:])
	case "classify":
		cmdClassify(os.Args[2:])
	case "rank":
		cmdRank(os.Args[2:])
	case "analytics-users":
		cmdAnalyticsUsers(os.Args[2:])
	case "analytics-summary":
		cmdAnalyticsSummary(os.Args[2:])
	case "usage-report":
		cmdUsageReport(os.Args[2:])
	case "user-agents":
		cmdUserAgents(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// loadDotenv reads KEY=VALUE pairs from a .env file in the working directory
// and sets them as environment variables. Existing variables are not
// overwritten, so explicit env vars and shell exports always take precedence.
// Missing file is silently ignored.
func loadDotenv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = stripQuotes(val)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: audit <command> [flags]

Commands:
  fetch               Fetch activities from Compliance API (incremental)
  users               List licensed users
  compare <csv>       Compare API activities against CSV export
  projects            List projects
  project <id>        Show project details
  chats               List chat conversations
  chat <id>           Export chat transcript
  chatanalysis <email>  Generate analysis prompt (--cmd to pipe through LLM)
  file <id>           Download file attachment
  rank                Stack-ranked engagement table (--reclaim for seat analysis)
  analytics-users     Fetch per-user daily analytics from Analytics API
  analytics-summary   Show org-level DAU/WAU/MAU table
  classify            Classify chat messages by usage taxonomy (--cmd for LLM)
  usage-report        Generate usage classification report
  user-agents         List unique user agent strings from stored activities

Run "audit <command> -h" for command-specific help.
`)
}

// cmdFetch pulls activities from the API and stores them locally. A fetch
// consists of up to two phases:
//
//  1. Forward (incremental): if we have a high-water mark, fetch any activities
//     newer than what we've already seen.
//  2. Backfill: if our oldest stored activity doesn't reach the target date
//     (--days), continue paginating backwards from where we left off.
//
// Both phases commit each page to SQLite immediately, so progress survives
// Ctrl+C and the next invocation picks up where it left off.
func cmdFetch(args []string) {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	days := fs.Int("days", 30, "Number of days of history to fetch")
	refresh := fs.Bool("refresh", false, "Drop local cache and re-fetch from scratch")
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	orgID := fs.String("org", compliance.DefaultOrgID(), "Organization ID")
	apiKey := fs.String("api-key", "", "API key (if unset, reads from 1Password)")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	ctx := context.Background()
	client, err := buildClient(*apiKey, *orgID)
	if err != nil {
		fatal("creating API client: %v", err)
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()
	fmt.Fprintf(os.Stderr, "Database: %s\n", db.Path())

	if *refresh {
		count, _ := db.ActivityCount()
		fmt.Fprintf(os.Stderr,
			"WARNING: --refresh will DROP all %d cached activities and re-fetch from scratch.\n"+
				"This is slow and hits the API heavily. You almost certainly want a plain\n"+
				"'audit fetch' (incremental) instead.\n\n"+
				"Type PROCEED to continue: ", count)
		var confirm string
		if _, err := fmt.Fscanln(os.Stdin, &confirm); err != nil || confirm != "PROCEED" {
			fatal("aborted (expected PROCEED, got %q)", confirm)
		}
		fmt.Fprintln(os.Stderr, "Resetting local activity cache...")
		if err := db.Reset(); err != nil {
			fatal("resetting database: %v", err)
		}
	}

	since := time.Now().UTC().AddDate(0, 0, -*days)
	targetDate := since.Format("2006-01-02")
	prevCount, _ := db.ActivityCount()
	totalFetched := 0
	totalInserted := 0

	// onPage commits each page to the store immediately. When updateHWM is
	// true and the page contains data, the high-water mark advances to the
	// newest activity on the page. For forward fetches each successive page
	// is newer, so updating on every page means the HWM tracks progress and
	// survives Ctrl+C.
	onPage := func(updateHWM bool) func(compliance.PageResult) error {
		return func(pr compliance.PageResult) error {
			n, err := db.InsertActivities(pr.Activities)
			if err != nil {
				return err
			}
			totalFetched += len(pr.Activities)
			totalInserted += n

			if updateHWM && len(pr.Activities) > 0 {
				if err := db.SetHighWaterMark(pr.Activities[0].ID); err != nil {
					return err
				}
			}
			return nil
		}
	}

	// Phase 1: forward (incremental) fetch for new activities.
	hwm, err := db.HighWaterMark()
	if err != nil {
		fatal("reading high water mark: %v", err)
	}

	if hwm != "" {
		fmt.Fprintln(os.Stderr, "Phase 1: checking for new activities...")
		query := compliance.ActivityQuery{
			BeforeID: hwm,
			OnPage:   onPage(true),
		}
		if _, err := client.FetchActivities(ctx, query); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: forward fetch failed: %v\n", err)
		} else if totalFetched > 0 {
			fmt.Fprintf(os.Stderr, "  %d new activities found\n", totalFetched)
		} else {
			fmt.Fprintln(os.Stderr, "  No new activities")
		}
	}

	// Phase 2: backfill if we haven't reached the target date yet. We query
	// the DB for the oldest activity we have; if it's newer than the target,
	// we resume pagination from that point rather than re-fetching from the
	// top. This is derived from the data itself, not a separate cursor, so
	// it's always consistent even after Ctrl+C.
	oldestID, oldestDate, _ := db.OldestActivity()
	needsBackfill := oldestDate == "" || oldestDate > targetDate

	if needsBackfill {
		if oldestDate == "" {
			fmt.Fprintf(os.Stderr, "Fetching activities back to %s...\n", targetDate)
		} else {
			fmt.Fprintf(os.Stderr, "Backfilling: have data to %s, need %s...\n",
				fmtDate(oldestDate), targetDate)
		}

		query := compliance.ActivityQuery{
			CreatedAtGte: &since,
			OnPage:       onPage(hwm == ""),
		}
		// Resume from the oldest activity we already have so we don't
		// re-download everything.
		if oldestID != "" {
			query.AfterID = oldestID
		}

		if _, err := client.FetchActivities(ctx, query); err != nil {
			fmt.Fprintf(os.Stderr, "\nBackfill interrupted: %d activities fetched, %d newly stored\n",
				totalFetched, totalInserted)
			printStoreSummary(db)
			fatal("fetching activities: %v", err)
		}
	}

	if totalFetched == 0 {
		fmt.Fprintln(os.Stderr, "No new activities found.")
		printStoreSummary(db)
		return
	}

	if err := db.SetLastFetchedAt(time.Now().UTC()); err != nil {
		fatal("updating last fetched time: %v", err)
	}

	fmt.Fprintf(os.Stderr, "\nFetched %d activities from API, %d newly stored (%d previously stored)\n",
		totalFetched, totalInserted, prevCount)

	printStoreSummary(db)
}

// cmdUsers lists licensed users from the API.
func cmdUsers(args []string) {
	fs := flag.NewFlagSet("users", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	orgID := fs.String("org", compliance.DefaultOrgID(), "Organization ID")
	apiKey := fs.String("api-key", "", "API key (if unset, reads from 1Password)")
	refreshFlag := fs.Bool("refresh", false, "Force refresh from API (ignore TTL)")
	jsonFlag := fs.Bool("json", false, "Output as JSON (includes user IDs)")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	ctx := context.Background()

	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	// Check cache staleness (4-hour TTL).
	const ttl = 4 * time.Hour
	fetchedAt, _ := db.UsersFetchedAt()
	cacheValid := !fetchedAt.IsZero() && time.Since(fetchedAt) < ttl

	if cacheValid && !*refreshFlag {
		fmt.Fprintf(os.Stderr, "Using cached user list (fetched %s ago)\n", time.Since(fetchedAt).Round(time.Minute))
	} else {
		client, err := buildClient(*apiKey, *orgID)
		if err != nil {
			fatal("creating API client: %v", err)
		}

		fmt.Fprintln(os.Stderr, "Fetching licensed users from API...")
		users, err := client.FetchUsers(ctx)
		if err != nil {
			fatal("fetching users: %v", err)
		}
		if err := db.InsertUsers(users, time.Now().UTC()); err != nil {
			fatal("storing users: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Cached %d users\n", len(users))
	}

	users, err := db.Users()
	if err != nil {
		fatal("reading users: %v", err)
	}

	if *jsonFlag {
		type jsonUser struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			FullName  string `json:"full_name"`
			CreatedAt string `json:"created_at"`
		}
		var out []jsonUser
		for _, u := range users {
			out = append(out, jsonUser{
				ID:        u.ID,
				Email:     u.Email,
				FullName:  u.FullName,
				CreatedAt: u.CreatedAt,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fatal("encoding JSON: %v", err)
		}
		return
	}

	fmt.Printf("%-40s %-25s %s\n", "Email", "Name", "Created")
	fmt.Println(strings.Repeat("-", 80))
	for _, u := range users {
		name := u.FullName
		if name == "" {
			name = "(no name)"
		}
		created := fmtDate(u.CreatedAt)
		fmt.Printf("%-40s %-25s %s\n", u.Email, truncateName(name, 25), created)
	}
	fmt.Fprintf(os.Stderr, "\nTotal: %d licensed users\n", len(users))
}

// cmdCompare loads both API-sourced activities and a CSV export, then reports
// on the overlap and differences between the two datasets.
func cmdCompare(args []string) {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: audit compare [flags] <csv-path>")
		os.Exit(1)
	}
	csvPath := fs.Arg(0)

	// Load CSV events.
	fmt.Fprintf(os.Stderr, "Parsing CSV: %s\n", csvPath)
	csvEvents, err := csvaudit.ParseCSV(csvPath)
	if err != nil {
		fatal("parsing CSV: %v", err)
	}
	csvSummary := csvaudit.SummarizeByUser(csvEvents)
	fmt.Fprintf(os.Stderr, "CSV: %d events, %d users\n", len(csvEvents), len(csvSummary))

	// Load stored API activities covering the same date range as the CSV.
	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	var csvMin, csvMax time.Time
	for _, e := range csvEvents {
		if csvMin.IsZero() || e.CreatedAt.Before(csvMin) {
			csvMin = e.CreatedAt
		}
		if e.CreatedAt.After(csvMax) {
			csvMax = e.CreatedAt
		}
	}

	apiActivities, err := db.Activities(store.QueryOpts{
		Since: &csvMin,
		Until: timePtr(csvMax.Add(time.Second)),
	})
	if err != nil {
		fatal("querying stored activities: %v", err)
	}
	apiSummary := compliance.SummarizeByUser(apiActivities)
	fmt.Fprintf(os.Stderr, "API: %d activities, %d users (in same date range)\n\n",
		len(apiActivities), len(apiSummary))

	// Classify users into overlap buckets.
	both, csvOnly, apiOnly := compareClassifyUsers(csvSummary, apiSummary)

	comparePrintUserOverlap(both, csvOnly, apiOnly, csvSummary, apiSummary)
	comparePrintCSVOnlyDiagnostic(csvOnly, csvSummary)
	comparePrintEventCounts(both, csvSummary, apiSummary)

	csvTypeTotals := aggregateEventTypes(csvSummary)
	apiTypeTotals := aggregateAPIEventTypes(apiSummary)

	comparePrintEventTypeTotals(csvTypeTotals, apiTypeTotals)
	comparePrintMappedTypes(csvTypeTotals, apiTypeTotals)
	comparePrintUnmappedCSVTypes(csvTypeTotals)
	comparePrintMappedGaps(csvTypeTotals, apiTypeTotals)
	comparePrintAPIOnlyTypes(apiTypeTotals)
}

func compareClassifyUsers(
	csvSummary map[string]*csvaudit.UserSummary,
	apiSummary map[string]*compliance.UserActivitySummary,
) (both, csvOnly, apiOnly []string) {
	allEmails := make(map[string]bool)
	for email := range csvSummary {
		allEmails[email] = true
	}
	for email := range apiSummary {
		allEmails[email] = true
	}
	for email := range allEmails {
		inCSV := csvSummary[email] != nil
		inAPI := apiSummary[email] != nil
		switch {
		case inCSV && inAPI:
			both = append(both, email)
		case inCSV:
			csvOnly = append(csvOnly, email)
		default:
			apiOnly = append(apiOnly, email)
		}
	}
	sort.Strings(both)
	sort.Strings(csvOnly)
	sort.Strings(apiOnly)
	return
}

func comparePrintUserOverlap(
	both, csvOnly, apiOnly []string,
	csvSummary map[string]*csvaudit.UserSummary,
	apiSummary map[string]*compliance.UserActivitySummary,
) {
	fmt.Printf("=== User Overlap ===\n")
	fmt.Printf("Both:     %d users\n", len(both))
	fmt.Printf("CSV only: %d users\n", len(csvOnly))
	fmt.Printf("API only: %d users\n\n", len(apiOnly))

	if len(apiOnly) > 0 {
		fmt.Printf("Users in API only:\n")
		for _, e := range apiOnly {
			fmt.Printf("  %s (%d events)\n", e, apiSummary[e].EventCount)
		}
		fmt.Println()
	}
}

// comparePrintCSVOnlyDiagnostic breaks down each CSV-only user's events by
// type and annotates whether each type has a known API equivalent. This
// reveals whether a "missing" user is genuinely anomalous or simply had
// only event types the Compliance API doesn't track.
func comparePrintCSVOnlyDiagnostic(
	csvOnly []string,
	csvSummary map[string]*csvaudit.UserSummary,
) {
	if len(csvOnly) == 0 {
		return
	}

	mappedTypes := csvaudit.MappedCSVTypes()
	csvToAPI := csvaudit.CSVToAPIMap()

	fmt.Printf("=== CSV-Only Users (%d) ===\n", len(csvOnly))
	fmt.Println("Users with activity in the CSV export but zero events in the Compliance API.")
	fmt.Println()

	unmappedOnly := 0
	for _, email := range csvOnly {
		s := csvSummary[email]
		fmt.Printf("  %s (%d events)\n", email, s.EventCount)

		hasMapped := false
		types := sortedKeys(s.EventTypes)
		for _, t := range types {
			count := s.EventTypes[t]
			if mappedTypes[t] {
				hasMapped = true
				fmt.Printf("    %-35s %4d  -> %s (MAPPED)\n", t, count, csvToAPI[t])
			} else {
				fmt.Printf("    %-35s %4d  (NO API EQUIVALENT)\n", t, count)
			}
		}
		if !hasMapped {
			unmappedOnly++
		}
		fmt.Println()
	}

	hasMapped := len(csvOnly) - unmappedOnly
	fmt.Printf("Diagnosis: %d of %d have ONLY unmapped event types (expected).\n",
		unmappedOnly, len(csvOnly))
	if hasMapped > 0 {
		fmt.Printf("           %d have mapped types — these warrant investigation.\n",
			hasMapped)
	}
	fmt.Println()
}

func comparePrintEventCounts(
	both []string,
	csvSummary map[string]*csvaudit.UserSummary,
	apiSummary map[string]*compliance.UserActivitySummary,
) {
	if len(both) == 0 {
		return
	}
	fmt.Printf("=== Event Counts (overlapping users) ===\n")
	fmt.Printf("%-40s %8s %8s %8s\n", "Email", "CSV", "API", "API-CSV")
	fmt.Println(strings.Repeat("-", 70))
	for _, email := range both {
		c := csvSummary[email].EventCount
		a := apiSummary[email].EventCount
		fmt.Printf("%-40s %8d %8d %+8d\n", email, c, a, a-c)
	}
	fmt.Println()
}

func aggregateEventTypes(
	csvSummary map[string]*csvaudit.UserSummary,
) map[string]int {
	totals := make(map[string]int)
	for _, s := range csvSummary {
		for t, c := range s.EventTypes {
			totals[t] += c
		}
	}
	return totals
}

func aggregateAPIEventTypes(
	apiSummary map[string]*compliance.UserActivitySummary,
) map[string]int {
	totals := make(map[string]int)
	for _, s := range apiSummary {
		for t, c := range s.EventTypes {
			totals[t] += c
		}
	}
	return totals
}

func comparePrintEventTypeTotals(
	csvTypeTotals, apiTypeTotals map[string]int,
) {
	fmt.Printf("=== Event Type Totals ===\n")
	fmt.Printf("\nCSV event types:\n")
	printTypeCounts(csvTypeTotals)
	fmt.Printf("\nAPI activity types:\n")
	printTypeCounts(apiTypeTotals)
}

func comparePrintMappedTypes(
	csvTypeTotals, apiTypeTotals map[string]int,
) {
	fmt.Printf("\n=== Mapped Event Type Comparison ===\n")
	fmt.Printf("%-55s %8s %8s\n", "CSV / API", "CSV", "API")
	fmt.Println(strings.Repeat("-", 75))
	for _, m := range csvaudit.CSVToAPIMappings {
		fmt.Printf("%-55s %8d %8d\n",
			m.CSV+" / "+m.API,
			csvTypeTotals[m.CSV],
			apiTypeTotals[m.API])
	}
}

// comparePrintUnmappedCSVTypes shows CSV event types that have no known
// API equivalent — these are blind spots in the Compliance API.
func comparePrintUnmappedCSVTypes(csvTypeTotals map[string]int) {
	mappedTypes := csvaudit.MappedCSVTypes()
	var unmapped []string
	for t := range csvTypeTotals {
		if !mappedTypes[t] {
			unmapped = append(unmapped, t)
		}
	}
	if len(unmapped) == 0 {
		return
	}
	sort.Strings(unmapped)
	fmt.Printf("\n=== Unmapped CSV Event Types (no API equivalent) ===\n")
	for _, t := range unmapped {
		fmt.Printf("  %-45s %8d\n", t, csvTypeTotals[t])
	}
}

// comparePrintMappedGaps flags mapped event type pairs where the CSV count
// exceeds the API count, indicating potential data gaps.
func comparePrintMappedGaps(csvTypeTotals, apiTypeTotals map[string]int) {
	var gaps []string
	for _, m := range csvaudit.CSVToAPIMappings {
		csvCount := csvTypeTotals[m.CSV]
		apiCount := apiTypeTotals[m.API]
		if csvCount > apiCount && apiCount > 0 {
			pct := float64(csvCount-apiCount) / float64(apiCount) * 100
			gaps = append(gaps, fmt.Sprintf(
				"  %-55s CSV %d, API %d (CSV %.0f%% higher)",
				m.CSV+" / "+m.API, csvCount, apiCount, pct))
		} else if csvCount > 0 && apiCount == 0 {
			gaps = append(gaps, fmt.Sprintf(
				"  %-55s CSV %d, API 0 (missing from API)",
				m.CSV+" / "+m.API, csvCount))
		}
	}
	if len(gaps) == 0 {
		return
	}
	fmt.Printf("\n=== Mapped Types Where CSV > API ===\n")
	for _, g := range gaps {
		fmt.Println(g)
	}
}

func comparePrintAPIOnlyTypes(apiTypeTotals map[string]int) {
	mappedTypes := csvaudit.MappedAPITypes()
	var apiOnly []string
	for t := range apiTypeTotals {
		if !mappedTypes[t] {
			apiOnly = append(apiOnly, t)
		}
	}
	if len(apiOnly) == 0 {
		return
	}
	sort.Strings(apiOnly)
	fmt.Printf("\n=== API-Only Event Types (no CSV equivalent) ===\n")
	for _, t := range apiOnly {
		fmt.Printf("  %-45s %8d\n", t, apiTypeTotals[t])
	}
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// cmdProjects lists projects from the API.
func cmdProjects(args []string) {
	fs := flag.NewFlagSet("projects", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	orgID := fs.String("org", compliance.DefaultOrgID(), "Organization ID")
	apiKey := fs.String("api-key", "", "API key (if unset, reads from 1Password)")
	refreshFlag := fs.Bool("refresh", false, "Force refresh from API (ignore TTL)")
	jsonFlag := fs.Bool("json", false, "Output as JSON")
	userFilter := fs.String("user", "", "Filter by creator email")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	ctx := context.Background()

	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	// Check cache staleness (4-hour TTL).
	const ttl = 4 * time.Hour
	fetchedAt, _ := db.ProjectsFetchedAt()
	cacheValid := !fetchedAt.IsZero() && time.Since(fetchedAt) < ttl

	if cacheValid && !*refreshFlag {
		fmt.Fprintf(os.Stderr, "Using cached project list (fetched %s ago)\n", time.Since(fetchedAt).Round(time.Minute))
	} else {
		client, err := buildClient(*apiKey, *orgID)
		if err != nil {
			fatal("creating API client: %v", err)
		}

		fmt.Fprintln(os.Stderr, "Fetching projects from API...")
		projects, err := client.FetchProjects(ctx, compliance.ProjectQuery{})
		if err != nil {
			fatal("fetching projects: %v", err)
		}
		if err := db.InsertProjects(projects, time.Now().UTC()); err != nil {
			fatal("storing projects: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Cached %d projects\n", len(projects))
	}

	projects, err := db.Projects(store.ProjectQueryOpts{CreatorEmail: *userFilter})
	if err != nil {
		fatal("reading projects: %v", err)
	}

	if *jsonFlag {
		type jsonProject struct {
			ID           string  `json:"id"`
			Name         string  `json:"name"`
			Description  string  `json:"description,omitempty"`
			Instructions string  `json:"instructions,omitempty"`
			CreatorID    string  `json:"creator_id"`
			CreatorEmail *string `json:"creator_email,omitempty"`
			CreatedAt    string  `json:"created_at"`
			UpdatedAt    string  `json:"updated_at"`
			ArchivedAt   *string `json:"archived_at,omitempty"`
		}
		var out []jsonProject
		for _, p := range projects {
			out = append(out, jsonProject{
				ID:           p.ID,
				Name:         p.Name,
				Description:  p.Description,
				Instructions: p.Instructions,
				CreatorID:    p.CreatorID,
				CreatorEmail: p.CreatorEmail,
				CreatedAt:    p.CreatedAt,
				UpdatedAt:    p.UpdatedAt,
				ArchivedAt:   p.ArchivedAt,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fatal("encoding JSON: %v", err)
		}
		return
	}

	fmt.Printf("%-30s %-30s %-12s %s\n", "Project ID", "Creator", "Created", "Name")
	fmt.Println(strings.Repeat("-", 100))
	for _, p := range projects {
		creator := "(unknown)"
		if p.CreatorEmail != nil {
			creator = *p.CreatorEmail
		}
		fmt.Printf("%-30s %-30s %-12s %s\n",
			truncateID(p.ID, 30), truncateName(creator, 30), fmtDate(p.CreatedAt), truncateName(p.Name, 40))
	}
	fmt.Fprintf(os.Stderr, "\nTotal: %d projects\n", len(projects))
}

// cmdProject shows details for a single project.
func cmdProject(args []string) {
	fs := flag.NewFlagSet("project", flag.ExitOnError)
	orgID := fs.String("org", compliance.DefaultOrgID(), "Organization ID")
	apiKey := fs.String("api-key", "", "API key (if unset, reads from 1Password)")
	jsonFlag := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: audit project [flags] <project-id>")
		os.Exit(1)
	}
	projectID := fs.Arg(0)

	ctx := context.Background()
	client, err := buildClient(*apiKey, *orgID)
	if err != nil {
		fatal("creating API client: %v", err)
	}

	project, err := client.GetProject(ctx, projectID)
	if err != nil {
		fatal("fetching project: %v", err)
	}

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(project); err != nil {
			fatal("encoding JSON: %v", err)
		}
		return
	}

	fmt.Printf("Project ID:   %s\n", project.ID)
	fmt.Printf("Name:         %s\n", project.Name)
	fmt.Printf("Created:      %s\n", project.CreatedAt)
	fmt.Printf("Updated:      %s\n", project.UpdatedAt)
	if project.ArchivedAt != nil {
		fmt.Printf("Archived:     %s\n", *project.ArchivedAt)
	}
	fmt.Printf("Creator ID:   %s\n", project.CreatorID)
	if project.Creator != nil {
		fmt.Printf("Creator:      %s (%s)\n", project.Creator.FullName, project.Creator.EffectiveEmail())
	}
	if project.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", project.Description)
	}
	if project.Instructions != "" {
		fmt.Printf("\nCustom Instructions:\n%s\n", project.Instructions)
	}
}

// cmdChats lists chat conversations from the API.
func cmdChats(args []string) {
	fs := flag.NewFlagSet("chats", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	orgID := fs.String("org", compliance.DefaultOrgID(), "Organization ID")
	apiKey := fs.String("api-key", "", "API key (if unset, reads from 1Password)")
	refreshFlag := fs.Bool("refresh", false, "Force refresh from API (ignore TTL)")
	jsonFlag := fs.Bool("json", false, "Output as JSON")
	userFilter := fs.String("user", "", "Filter by user email")
	projectFilter := fs.String("project", "", "Filter by project ID")
	sinceFlag := fs.String("since", "", "Filter by created_at >= date (YYYY-MM-DD)")
	untilFlag := fs.String("until", "", "Filter by created_at < date (YYYY-MM-DD)")
	limitFlag := fs.Int("limit", 100, "Maximum number of chats to display")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	ctx := context.Background()

	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	// Parse date filters.
	var since, until *time.Time
	if *sinceFlag != "" {
		t, err := time.Parse("2006-01-02", *sinceFlag)
		if err != nil {
			fatal("invalid --since date: %v", err)
		}
		since = &t
	}
	if *untilFlag != "" {
		t, err := time.Parse("2006-01-02", *untilFlag)
		if err != nil {
			fatal("invalid --until date: %v", err)
		}
		until = &t
	}

	// Check cache staleness (4-hour TTL).
	const ttl = 4 * time.Hour
	fetchedAt, _ := db.ChatsFetchedAt()
	cacheValid := !fetchedAt.IsZero() && time.Since(fetchedAt) < ttl

	// The chats API requires user_ids[], so we need to convert email to user ID.
	var userIDs []string
	if *userFilter != "" {
		user, err := db.UserByEmail(*userFilter)
		if err != nil {
			fatal("user not found in cache: %s (run 'audit users --refresh' first)", *userFilter)
		}
		userIDs = append(userIDs, user.ID)
	}

	if cacheValid && !*refreshFlag {
		fmt.Fprintf(os.Stderr, "Using cached chat list (fetched %s ago)\n", time.Since(fetchedAt).Round(time.Minute))
	} else {
		if len(userIDs) == 0 {
			fatal("--user is required for fetching chats from API (chats API requires user_ids[])")
		}

		client, err := buildClient(*apiKey, *orgID)
		if err != nil {
			fatal("creating API client: %v", err)
		}

		fmt.Fprintln(os.Stderr, "Fetching chats from API...")
		query := compliance.ChatQuery{
			UserIDs:      userIDs,
			CreatedAtGte: since,
			CreatedAtLt:  until,
		}
		chats, err := client.FetchChats(ctx, query)
		if err != nil {
			fatal("fetching chats: %v", err)
		}
		if err := db.InsertChats(chats, time.Now().UTC()); err != nil {
			fatal("storing chats: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Cached %d chats\n", len(chats))
	}

	chats, err := db.Chats(store.ChatQueryOpts{
		UserEmail: *userFilter,
		ProjectID: *projectFilter,
		Since:     since,
		Until:     until,
		Limit:     *limitFlag,
	})
	if err != nil {
		fatal("reading chats: %v", err)
	}

	if *jsonFlag {
		type jsonChat struct {
			ID        string  `json:"id"`
			Name      string  `json:"name"`
			UserID    string  `json:"user_id"`
			UserEmail string  `json:"user_email"`
			ProjectID *string `json:"project_id,omitempty"`
			CreatedAt string  `json:"created_at"`
			UpdatedAt string  `json:"updated_at"`
			DeletedAt *string `json:"deleted_at,omitempty"`
		}
		var out []jsonChat
		for _, c := range chats {
			out = append(out, jsonChat{
				ID:        c.ID,
				Name:      c.Name,
				UserID:    c.UserID,
				UserEmail: c.UserEmail,
				ProjectID: c.ProjectID,
				CreatedAt: c.CreatedAt,
				UpdatedAt: c.UpdatedAt,
				DeletedAt: c.DeletedAt,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fatal("encoding JSON: %v", err)
		}
		return
	}

	fmt.Printf("%-30s %-30s %-12s %-12s %s\n", "Chat ID", "User", "Created", "Updated", "Name")
	fmt.Println(strings.Repeat("-", 120))
	for _, c := range chats {
		fmt.Printf("%-30s %-30s %-12s %-12s %s\n",
			truncateID(c.ID, 30), truncateName(c.UserEmail, 30),
			fmtDate(c.CreatedAt), fmtDate(c.UpdatedAt), truncateName(c.Name, 40))
	}
	fmt.Fprintf(os.Stderr, "\nShowing %d chats\n", len(chats))
}

// cmdChat exports a single chat transcript.
func cmdChat(args []string) {
	fs := flag.NewFlagSet("chat", flag.ExitOnError)
	orgID := fs.String("org", compliance.DefaultOrgID(), "Organization ID")
	apiKey := fs.String("api-key", "", "API key (if unset, reads from 1Password)")
	formatFlag := fs.String("format", "json", "Output format: json or markdown")
	outputFlag := fs.String("output", "", "Output file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: audit chat [flags] <chat-id>")
		os.Exit(1)
	}
	chatID := fs.Arg(0)

	ctx := context.Background()
	client, err := buildClient(*apiKey, *orgID)
	if err != nil {
		fatal("creating API client: %v", err)
	}

	chat, err := client.GetChat(ctx, chatID)
	if err != nil {
		fatal("fetching chat: %v", err)
	}

	var out *os.File
	if *outputFlag != "" {
		out, err = os.Create(*outputFlag)
		if err != nil {
			fatal("creating output file: %v", err)
		}
		defer out.Close()
	} else {
		out = os.Stdout
	}

	switch *formatFlag {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(chat); err != nil {
			fatal("encoding JSON: %v", err)
		}
	case "markdown":
		writeChatMarkdown(out, chat)
	default:
		fatal("unknown format: %s (use json or markdown)", *formatFlag)
	}
}

func writeChatMarkdown(out *os.File, chat *compliance.ChatDetail) {
	_, _ = fmt.Fprintf(out, "# %s\n\n", chat.Name)
	_, _ = fmt.Fprintf(out, "**User:** %s\n", chat.User.EmailAddress)
	_, _ = fmt.Fprintf(out, "**Created:** %s\n", chat.CreatedAt)
	_, _ = fmt.Fprintf(out, "**Chat ID:** `%s`\n", chat.ID)
	if chat.ProjectID != nil {
		_, _ = fmt.Fprintf(out, "**Project ID:** `%s`\n", *chat.ProjectID)
	}
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "---")
	_, _ = fmt.Fprintln(out)

	for _, msg := range chat.ChatMessages {
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}
		_, _ = fmt.Fprintf(out, "## %s (%s)\n\n", role, fmtDateTime(msg.CreatedAt))

		for _, c := range msg.Content {
			if c.Type == "text" {
				_, _ = fmt.Fprintln(out, c.Text)
				_, _ = fmt.Fprintln(out)
			}
		}

		if len(msg.Files) > 0 {
			for _, f := range msg.Files {
				_, _ = fmt.Fprintf(out, "📎 **Attached:** `%s` (`%s`)\n\n", f.Filename, f.ID)
			}
		}

		for _, a := range msg.Artifacts {
			title := "(untitled)"
			if a.Title != nil {
				title = *a.Title
			}
			_, _ = fmt.Fprintf(out, "🧩 **Artifact:** %s (`%s`)\n\n", title, a.ID)
		}

		_, _ = fmt.Fprintln(out, "---")
		_, _ = fmt.Fprintln(out)
	}
}

// cmdFile downloads a file attachment.
func cmdFile(args []string) {
	fs := flag.NewFlagSet("file", flag.ExitOnError)
	orgID := fs.String("org", compliance.DefaultOrgID(), "Organization ID")
	apiKey := fs.String("api-key", "", "API key (if unset, reads from 1Password)")
	outputFlag := fs.String("output", "", "Output file path (default: original filename)")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: audit file [flags] <file-id>")
		os.Exit(1)
	}
	fileID := fs.Arg(0)

	ctx := context.Background()
	client, err := buildClient(*apiKey, *orgID)
	if err != nil {
		fatal("creating API client: %v", err)
	}

	body, filename, err := client.DownloadFile(ctx, fileID)
	if err != nil {
		fatal("downloading file: %v", err)
	}
	defer body.Close()

	outputPath := filename
	if *outputFlag != "" {
		outputPath = *outputFlag
	}

	out, err := os.Create(outputPath)
	if err != nil {
		fatal("creating output file: %v", err)
	}
	defer out.Close()

	n, err := copyWithProgress(out, body)
	if err != nil {
		fatal("writing file: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Downloaded %s (%d bytes)\n", outputPath, n)
}

func copyWithProgress(dst *os.File, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// ChatAnalysisData holds all data needed to render the analysis prompt template.
type ChatAnalysisData struct {
	Email                 string
	FirstSeen             string
	LastSeen              string
	DaysSinceLastActivity int
	TotalEvents           int
	ChatsCreated          int
	ChatsUpdated          int
	ProjectsCreated       int
	SnapshotsShared       int
	SessionsShared        int
	IntegrationsUsed      int
	CustomizationsCreated int
	FilesUploaded         int
	PrimaryClient         string
	EventTypes            []EventTypeCount
	WeeklyActivity        []WeeklyActivityData
	VelocityPattern       string
	Chats                 []ChatSummary
	SamplingNote          string
}

// WeeklyActivityData holds activity metrics for a single week.
type WeeklyActivityData struct {
	WeekStart  string
	Events     int
	ActiveDays int
	Trend      string // "▲", "▼", "—"
}

// EventTypeCount holds a single event type and its count.
type EventTypeCount struct {
	Type  string
	Count int
}

// ChatSummary represents a chat for display in the analysis prompt.
type ChatSummary struct {
	ID        string
	Name      string
	CreatedAt string
	UpdatedAt string
	Messages  []MessageSummary
}

// MessageSummary represents a single message in a chat.
type MessageSummary struct {
	Role      string
	CreatedAt string
	Text      string
	Files     []string
}

const defaultAnalysisPrompt = `You are analyzing Claude Enterprise usage data for a specific user to understand their engagement patterns and identify why they may have stopped using Claude (if applicable).

## User: {{.Email}}

### Activity Summary
- **First seen:** {{.FirstSeen}}
- **Last seen:** {{.LastSeen}} ({{.DaysSinceLastActivity}} days ago)
- **Total events:** {{.TotalEvents}}
- **Chats created:** {{.ChatsCreated}}
- **Chats with follow-up messages:** {{.ChatsUpdated}}
- **Projects created:** {{.ProjectsCreated}}
- **Chat snapshots shared:** {{.SnapshotsShared}}
- **Sessions shared:** {{.SessionsShared}}
- **Files uploaded:** {{.FilesUploaded}}
- **Integrations connected:** {{.IntegrationsUsed}}
- **Customizations created (skills/commands/plugins):** {{.CustomizationsCreated}}
- **Primary client:** {{.PrimaryClient}}
{{- if .SamplingNote}}
- **Note:** {{.SamplingNote}}
{{- end}}

### Activity Breakdown by Type
{{range .EventTypes -}}
- {{.Type}}: {{.Count}}
{{end}}
### Activity Velocity (by week)
| Week Starting | Events | Active Days | Trend |
|---------------|--------|-------------|-------|
{{range .WeeklyActivity -}}
| {{.WeekStart}} | {{.Events}} | {{.ActiveDays}} | {{.Trend}} |
{{end}}
**Pattern:** {{.VelocityPattern}}

### Chat Transcripts
{{range .Chats}}
#### Chat: "{{.Name}}" ({{.CreatedAt}})
{{range .Messages -}}
**{{.Role}}:** {{.Text}}
{{if .Files}}
{{- range .Files}}📎 *Attached:* ` + "`{{.}}`" + `
{{end -}}
{{end}}
{{end}}---
{{end}}
## Analysis Request

Based on the data above, analyze this user's Claude usage and provide a structured assessment. Pay attention to:
- **Sharing behavior:** Chat snapshots and session shares indicate users who find value worth sharing with colleagues.
- **Integration usage:** Connected integrations suggest deeper workflow embedding.
- **Customization activity:** Skills, commands, and plugins indicate power users investing in personalization.
- **Project usage:** Project creation shows structured, recurring use cases.

### Example Output Format

Here's an example of the expected analysis format (for a different user):

---

**Usage Pattern Summary**

Sarah used Claude sporadically over a 3-week period, primarily via the web browser. She created 5 chats but only followed up on 2 of them, suggesting she was testing capabilities rather than integrating Claude into her workflow.

**Capability Assessment**

| Use Case | Worked Well | Issues |
|----------|-------------|--------|
| Email drafting | ✅ Claude produced professional emails | None observed |
| Data analysis | ⚠️ Partial | Claude couldn't access her Excel files directly |
| Meeting scheduling | ❌ Failed | Claude has no calendar integration |

**Friction Points**

1. **SharePoint access:** Sarah tried to share a SharePoint link in chat 3, but Claude couldn't access it directly. She abandoned the conversation.
2. **Expectation mismatch:** In chat 5, she asked Claude to "check my calendar" — Claude explained it doesn't have calendar access, and she didn't respond.

**Engagement Classification:** Exploratory user (tried it out, hit capability gaps)

**Root Cause Analysis**

Sarah's core workflow requires direct access to SharePoint files and calendar integration — capabilities Claude doesn't have. Her use cases (data analysis, scheduling) require integrations that aren't available, while the capabilities that worked (email drafting) aren't central to her job.

**Recommendations**

1. **No immediate action needed** — This is a capability gap, not a training issue
2. **Future consideration:** If SharePoint/Excel integration becomes available, reach out to Sarah as she's a good candidate for re-engagement
3. **License status:** Consider for reallocation if licenses are constrained

---

Now analyze the user data provided above using this same format.
`

// cmdChatAnalysis generates a prompt for analyzing a user's Claude usage.
func cmdChatAnalysis(args []string) {
	fs := flag.NewFlagSet("chatanalysis", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	orgID := fs.String("org", compliance.DefaultOrgID(), "Organization ID")
	apiKey := fs.String("api-key", "", "API key (if unset, reads from 1Password)")
	promptStdin := fs.Bool("prompt-stdin", false, "Read custom prompt template from stdin")
	promptFile := fs.String("prompt-file", "", "Read custom prompt template from file")
	outputFlag := fs.String("output", "", "Output file (default: stdout)")
	maxTokens := fs.Int("max-tokens", 150000, "Maximum tokens in output (approx 4 chars/token)")
	cmdFlag := fs.String("cmd", "", "Shell command to pipe the prompt through (e.g., 'claude --print')")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: audit chatanalysis [flags] <user-email>")
		fmt.Fprintln(os.Stderr, "\nGenerates a prompt suitable for analyzing a user's Claude usage patterns.")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		fs.PrintDefaults()
		os.Exit(1)
	}
	userEmail := strings.ToLower(fs.Arg(0))

	ctx := context.Background()

	// Open the database to get activity data.
	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	// Lookup the user to get their ID.
	user, err := db.UserByEmail(userEmail)
	if err != nil {
		fatal("user not found in cache: %s (run 'audit users --refresh' first)", userEmail)
	}
	fmt.Fprintf(os.Stderr, "User: %s (%s)\n", user.Email, user.FullName)

	// Query user's activities from the database.
	activities, err := db.Activities(store.QueryOpts{Email: userEmail})
	if err != nil {
		fatal("querying activities: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Found %d activities in database\n", len(activities))

	// Build activity summary.
	data := buildActivitySummary(activities, userEmail)

	// Create API client for fetching chats.
	client, err := buildClient(*apiKey, *orgID)
	if err != nil {
		fatal("creating API client: %v", err)
	}

	// Fetch user's chats via API.
	fmt.Fprintln(os.Stderr, "Fetching chats from API...")
	chatMetas, err := client.FetchChats(ctx, compliance.ChatQuery{
		UserIDs: []string{user.ID},
	})
	if err != nil {
		fatal("fetching chats: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Found %d chats\n", len(chatMetas))

	// Cache the chat metadata.
	if err := db.InsertChats(chatMetas, time.Now().UTC()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cache chats: %v\n", err)
	}

	// Select chats within token budget and fetch their full content.
	data.Chats, data.SamplingNote = selectAndFetchChats(ctx, client, chatMetas, *maxTokens)

	// Read custom prompt template if provided.
	var customPrompt string
	if *promptStdin {
		promptBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			fatal("reading prompt from stdin: %v", err)
		}
		customPrompt = string(promptBytes)
	} else if *promptFile != "" {
		promptBytes, err := os.ReadFile(*promptFile)
		if err != nil {
			fatal("reading prompt file: %v", err)
		}
		customPrompt = string(promptBytes)
	}

	// Execute the template.
	var tmplText string
	if customPrompt != "" {
		tmplText = buildCustomTemplate(data, customPrompt)
	} else {
		tmplText = defaultAnalysisPrompt
	}

	tmpl, err := template.New("analysis").Parse(tmplText)
	if err != nil {
		fatal("parsing template: %v", err)
	}

	// Determine output destination.
	var out *os.File
	if *outputFlag != "" {
		out, err = os.Create(*outputFlag)
		if err != nil {
			fatal("creating output file: %v", err)
		}
		defer out.Close()
	} else {
		out = os.Stdout
	}

	if *cmdFlag != "" {
		// Render the template into a buffer, then pipe through the command.
		var buf strings.Builder
		if err := tmpl.Execute(&buf, data); err != nil {
			fatal("executing template: %v", err)
		}
		runPromptThroughCmd(ctx, *cmdFlag, buf.String(), out)
	} else {
		if err := tmpl.Execute(out, data); err != nil {
			fatal("executing template: %v", err)
		}
	}

	if *outputFlag != "" {
		fmt.Fprintf(os.Stderr, "\nWritten to %s\n", *outputFlag)
	}
}

// runPromptThroughCmd pipes prompt text through a shell command, writing the
// command's stdout to out and its stderr to os.Stderr.
func runPromptThroughCmd(
	ctx context.Context,
	shellCmd string,
	prompt string,
	out io.Writer,
) {
	cmd := exec.CommandContext(ctx, "sh", "-c", shellCmd)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fatal("command %q failed: %v", shellCmd, err)
	}
}

// buildActivitySummary aggregates activity data into the analysis data structure.
func buildActivitySummary(activities []compliance.Activity, email string) ChatAnalysisData {
	data := ChatAnalysisData{
		Email:       email,
		TotalEvents: len(activities),
	}

	if len(activities) == 0 {
		data.FirstSeen = "(no activity)"
		data.LastSeen = "(no activity)"
		data.PrimaryClient = "(unknown)"
		data.VelocityPattern = "No activity recorded"
		return data
	}

	// Activities are returned newest-first, so last element is oldest.
	data.LastSeen = fmtDate(activities[0].CreatedAt)
	data.FirstSeen = fmtDate(activities[len(activities)-1].CreatedAt)

	// Calculate days since last activity.
	if lastTime, err := activities[0].CreatedAtTime(); err == nil {
		data.DaysSinceLastActivity = int(time.Since(lastTime).Hours() / 24)
	}

	// Count event types, detect primary client, and build weekly activity data.
	typeCounts := make(map[string]int)
	clientCounts := make(map[string]int)
	weeklyEvents := make(map[string]int)    // week start date -> event count
	weeklyDays := make(map[string]map[string]bool) // week start date -> set of active days

	for _, a := range activities {
		typeCounts[a.Type]++

		// Count activity types across Rev E categories.
		switch a.Type {
		case "claude_chat_created":
			data.ChatsCreated++
		case "claude_chat_updated":
			data.ChatsUpdated++
		case "claude_project_created":
			data.ProjectsCreated++
		case "claude_chat_snapshot_created":
			data.SnapshotsShared++
		case "session_share_created":
			data.SessionsShared++
		case "integration_user_connected":
			data.IntegrationsUsed++
		case "claude_skill_created",
			"claude_command_created",
			"claude_plugin_created":
			data.CustomizationsCreated++
		case "claude_file_uploaded":
			data.FilesUploaded++
		}

		// Detect client from user agent.
		if a.Actor.UserAgent != nil {
			ua := *a.Actor.UserAgent
			client := detectClient(ua)
			clientCounts[client]++
		}

		// Track weekly activity.
		if t, err := a.CreatedAtTime(); err == nil {
			weekStart := getWeekStart(t)
			weeklyEvents[weekStart]++
			day := t.Format("2006-01-02")
			if weeklyDays[weekStart] == nil {
				weeklyDays[weekStart] = make(map[string]bool)
			}
			weeklyDays[weekStart][day] = true
		}
	}

	// Sort event types by count descending.
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range typeCounts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
	for _, kv := range sorted {
		data.EventTypes = append(data.EventTypes, EventTypeCount{Type: kv.k, Count: kv.v})
	}

	// Determine primary client.
	data.PrimaryClient = "(unknown)"
	maxCount := 0
	for client, count := range clientCounts {
		if count > maxCount {
			maxCount = count
			data.PrimaryClient = client
		}
	}

	// Build weekly activity summary sorted chronologically.
	var weeks []string
	for w := range weeklyEvents {
		weeks = append(weeks, w)
	}
	sort.Strings(weeks)

	var prevEvents int
	var peakWeek string
	var peakEvents int
	for i, w := range weeks {
		events := weeklyEvents[w]
		activeDays := len(weeklyDays[w])

		trend := "—"
		if i > 0 {
			if events > prevEvents {
				trend = "▲"
			} else if events < prevEvents {
				trend = "▼"
			}
		}

		if events > peakEvents {
			peakEvents = events
			peakWeek = w
		}

		data.WeeklyActivity = append(data.WeeklyActivity, WeeklyActivityData{
			WeekStart:  w,
			Events:     events,
			ActiveDays: activeDays,
			Trend:      trend,
		})
		prevEvents = events
	}

	// Determine velocity pattern.
	data.VelocityPattern = analyzeVelocityPattern(data.WeeklyActivity, peakWeek, data.DaysSinceLastActivity)

	return data
}

// getWeekStart returns the Monday of the week containing t, formatted as YYYY-MM-DD.
func getWeekStart(t time.Time) string {
	// Go's Weekday: Sunday=0, Monday=1, ..., Saturday=6
	// We want Monday as start of week.
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday becomes 7
	}
	monday := t.AddDate(0, 0, -(weekday - 1))
	return monday.Format("2006-01-02")
}

// analyzeVelocityPattern generates a human-readable description of the activity trajectory.
func analyzeVelocityPattern(weeks []WeeklyActivityData, peakWeek string, daysSinceLastActivity int) string {
	if len(weeks) == 0 {
		return "No activity recorded"
	}
	if len(weeks) == 1 {
		return "Single week of activity — tried it briefly"
	}

	lastWeek := weeks[len(weeks)-1]
	firstWeek := weeks[0]

	// Check if peak was the last week.
	isPeakAtEnd := peakWeek == lastWeek.WeekStart
	isPeakNearEnd := len(weeks) >= 2 &&
		peakWeek == weeks[len(weeks)-2].WeekStart

	// Calculate overall trend.
	var upWeeks, downWeeks int
	for _, w := range weeks {
		switch w.Trend {
		case "▲":
			upWeeks++
		case "▼":
			downWeeks++
		}
	}

	// Determine pattern.
	switch {
	case isPeakAtEnd && daysSinceLastActivity > 14:
		return "Peak usage in final week then stopped — likely project completion or got busy"
	case isPeakNearEnd && daysSinceLastActivity > 14:
		return "Peak usage near end then stopped — suggests workflow completion, not dissatisfaction"
	case downWeeks > upWeeks*2 && lastWeek.Events < firstWeek.Events/2:
		return "Steady decline over time — possible loss of interest or found alternative"
	case upWeeks > downWeeks && daysSinceLastActivity > 14:
		return "Was ramping up before stopping — external factor likely (vacation, role change, project end)"
	case lastWeek.Events <= 5 && daysSinceLastActivity > 14:
		return "Usage tapered off to minimal before stopping — gradual disengagement"
	case daysSinceLastActivity <= 7:
		return "Recently active — still engaged"
	case daysSinceLastActivity <= 14:
		return "Active within last 2 weeks — may still be engaged"
	default:
		return "Mixed usage pattern — review chat content for friction points"
	}
}

// detectClient extracts a readable client name from user agent. Product-
// specific clients are checked first so they aren't misidentified as generic
// browsers by the Chrome/Safari/Edge fallbacks. The Claude Desktop app embeds
// Chromium (Electron) and identifies itself as either "ClaudeNest/X.Y.Z"
// (pre-GA) or "Claude/X.Y.Z" (post-GA) in the product token.
func detectClient(ua string) string {
	uaLower := strings.ToLower(ua)
	switch {
	case strings.Contains(uaLower, "claudenest/"):
		return "Claude Desktop"
	case strings.Contains(uaLower, " claude/"):
		return "Claude Desktop"
	case strings.Contains(uaLower, "claude-code") ||
		strings.Contains(uaLower, "claudecode"):
		return "Claude Code"
	case strings.Contains(uaLower, "claude-chrome") ||
		strings.Contains(uaLower, "crx-claude"):
		return "Claude for Chrome"
	case strings.Contains(uaLower, "claude-excel") ||
		(strings.Contains(uaLower, "office") &&
			strings.Contains(uaLower, "excel")):
		return "Claude for Excel"
	case strings.Contains(uaLower, "claude-powerpoint") ||
		(strings.Contains(uaLower, "office") &&
			strings.Contains(uaLower, "powerpoint")):
		return "Claude for PowerPoint"
	case strings.Contains(uaLower, "firefox"):
		return "Firefox"
	case strings.Contains(uaLower, "edg/"):
		return "Edge"
	case strings.Contains(uaLower, "chrome"):
		return "Chrome"
	case strings.Contains(uaLower, "safari"):
		return "Safari"
	default:
		return "Browser"
	}
}

// selectAndFetchChats selects chats within the token budget, fetches their full
// content, and returns the summaries plus a sampling note if applicable.
func selectAndFetchChats(ctx context.Context, client *compliance.Client, allChats []compliance.Chat, maxTokens int) ([]ChatSummary, string) {
	const charsPerToken = 4
	const reservedTokens = 6000 // for prompt template + activity summary
	budgetChars := (maxTokens - reservedTokens) * charsPerToken

	if len(allChats) == 0 {
		return nil, ""
	}

	// Sort by date ascending (oldest first) for selection logic.
	sort.Slice(allChats, func(i, j int) bool {
		return allChats[i].CreatedAt < allChats[j].CreatedAt
	})

	// If few chats, fetch all of them.
	if len(allChats) <= 10 {
		return fetchChatsWithBudget(ctx, client, allChats, budgetChars, len(allChats))
	}

	// Always include first and last chat, then sample from the middle.
	var selected []compliance.Chat
	selected = append(selected, allChats[0])              // oldest
	selected = append(selected, allChats[len(allChats)-1]) // newest

	// Shuffle middle chats for random sampling.
	middle := make([]compliance.Chat, len(allChats)-2)
	copy(middle, allChats[1:len(allChats)-1])
	rand.Shuffle(len(middle), func(i, j int) {
		middle[i], middle[j] = middle[j], middle[i]
	})

	// Combine: first, sampled middle, last. We'll fetch in order and stop when budget exhausted.
	toFetch := append(selected[:1], middle...)
	toFetch = append(toFetch, selected[1])

	results, note := fetchChatsWithBudget(ctx, client, toFetch, budgetChars, len(allChats))

	// Sort results chronologically for output.
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt < results[j].CreatedAt
	})

	return results, note
}

// fetchChatsWithBudget fetches chat transcripts up to the character budget.
func fetchChatsWithBudget(ctx context.Context, client *compliance.Client, chats []compliance.Chat, budgetChars int, totalChats int) ([]ChatSummary, string) {
	var results []ChatSummary
	usedChars := 0

	for i, meta := range chats {
		fmt.Fprintf(os.Stderr, "Fetching chat %d/%d: %s\n", i+1, len(chats), truncateName(meta.Name, 40))

		chat, err := client.GetChat(ctx, meta.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to fetch chat %s: %v\n", meta.ID, err)
			continue
		}

		summary := convertChat(chat)
		chatChars := estimateChatChars(summary)

		// Stop if we've exceeded the budget (but keep at least 2 chats).
		if usedChars+chatChars > budgetChars && len(results) >= 2 {
			fmt.Fprintf(os.Stderr, "  Stopping: budget exceeded (%d chars used of %d)\n", usedChars, budgetChars)
			break
		}

		results = append(results, summary)
		usedChars += chatChars
	}

	var note string
	if len(results) < totalChats {
		note = fmt.Sprintf("%d of %d chats sampled to fit context window", len(results), totalChats)
	}

	fmt.Fprintf(os.Stderr, "Included %d chats (~%d tokens)\n", len(results), usedChars/4)
	return results, note
}

// convertChat converts a ChatDetail to a ChatSummary.
func convertChat(chat *compliance.ChatDetail) ChatSummary {
	summary := ChatSummary{
		ID:        chat.ID,
		Name:      chat.Name,
		CreatedAt: fmtDateTime(chat.CreatedAt),
		UpdatedAt: fmtDateTime(chat.UpdatedAt),
	}

	for _, msg := range chat.ChatMessages {
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}

		var text strings.Builder
		for _, c := range msg.Content {
			if c.Type == "text" {
				if text.Len() > 0 {
					text.WriteString("\n")
				}
				text.WriteString(c.Text)
			}
		}

		var files []string
		for _, f := range msg.Files {
			files = append(files, f.Filename)
		}

		summary.Messages = append(summary.Messages, MessageSummary{
			Role:      role,
			CreatedAt: fmtDateTime(msg.CreatedAt),
			Text:      text.String(),
			Files:     files,
		})
	}

	return summary
}

// estimateChatChars estimates the character count of a chat summary.
func estimateChatChars(chat ChatSummary) int {
	chars := len(chat.Name) + len(chat.CreatedAt) + 50 // header overhead
	for _, msg := range chat.Messages {
		chars += len(msg.Role) + len(msg.CreatedAt) + len(msg.Text) + 20
		for _, f := range msg.Files {
			chars += len(f) + 20
		}
	}
	return chars
}

// buildCustomTemplate wraps a custom analysis request with the standard data sections.
func buildCustomTemplate(data ChatAnalysisData, customPrompt string) string {
	var sb strings.Builder

	sb.WriteString(`## User: {{.Email}}

### Activity Summary
- **First seen:** {{.FirstSeen}}
- **Last seen:** {{.LastSeen}} ({{.DaysSinceLastActivity}} days ago)
- **Total events:** {{.TotalEvents}}
- **Chats created:** {{.ChatsCreated}}
- **Chats with follow-up messages:** {{.ChatsUpdated}}
- **Projects created:** {{.ProjectsCreated}}
- **Chat snapshots shared:** {{.SnapshotsShared}}
- **Sessions shared:** {{.SessionsShared}}
- **Files uploaded:** {{.FilesUploaded}}
- **Integrations connected:** {{.IntegrationsUsed}}
- **Customizations created (skills/commands/plugins):** {{.CustomizationsCreated}}
- **Primary client:** {{.PrimaryClient}}
{{- if .SamplingNote}}
- **Note:** {{.SamplingNote}}
{{- end}}

### Activity Breakdown by Type
{{range .EventTypes -}}
- {{.Type}}: {{.Count}}
{{end}}
### Activity Velocity (by week)
| Week Starting | Events | Active Days | Trend |
|---------------|--------|-------------|-------|
{{range .WeeklyActivity -}}
| {{.WeekStart}} | {{.Events}} | {{.ActiveDays}} | {{.Trend}} |
{{end}}
**Pattern:** {{.VelocityPattern}}

### Chat Transcripts
{{range .Chats}}
#### Chat: "{{.Name}}" ({{.CreatedAt}})
{{range .Messages -}}
**{{.Role}}:** {{.Text}}
{{if .Files}}
{{- range .Files}}📎 *Attached:* ` + "`{{.}}`" + `
{{end -}}
{{end}}
{{end}}---
{{end}}
## Analysis Request

`)
	sb.WriteString(customPrompt)

	return sb.String()
}

// cmdClassify classifies chat messages according to the usage taxonomy from
// "How People Use ChatGPT" (Chatterji et al., 2025).
func cmdClassify(args []string) {
	fs := flag.NewFlagSet("classify", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	orgID := fs.String("org", compliance.DefaultOrgID(), "Organization ID")
	apiKey := fs.String("api-key", "", "API key (if unset, reads from 1Password)")
	taxonomyFlag := fs.String("taxonomy", "all", "Taxonomy to classify: work, intent, topic, or all")
	modelFlag := fs.String("model", "haiku", "Classifier model: haiku (fast/cheap) or sonnet (accurate)")
	classifierCmd := fs.String("cmd", "", "Shell command to pipe prompts through (e.g., 'claude --print --model sonnet')")
	limitFlag := fs.Int("limit", 0, "Maximum chats to classify (0 = no limit)")
	outputFlag := fs.String("output", "", "Output file (default: stdout)")
	formatFlag := fs.String("format", "summary", "Output format: summary, json, csv, or sql")
	jsonFlag := fs.Bool("json", false, "Shorthand for --format=json")
	stdinFlag := fs.Bool("stdin", false, "Read chat IDs from stdin (one per line)")
	compareFlag := fs.Bool("compare", false, "Show comparison to ChatGPT baseline after classification")
	noStoreFlag := fs.Bool("no-store", false, "Don't store classifications in database (dry run)")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	// Handle stdin mode for batch chat classification.
	if *stdinFlag {
		if fs.NArg() > 0 {
			fatal("--stdin cannot be combined with positional arguments")
		}
	} else if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, `Usage: audit classify [flags] <subcommand> <target>

Subcommands:
  chat <chat-id>    Classify messages in a single chat
  user <email>      Classify messages in all chats for a user

Flags:`)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, `
Examples:
  # Classify a user's chats (uses API)
  audit classify user alice@example.com

  # Classify using Claude CLI (Sonnet recommended for speed)
  audit classify --cmd='claude --print --model sonnet' user alice@example.com

  # Classify and output JSON
  audit classify --json user alice@example.com

  # Classify and save CSV (database format)
  audit classify --format=csv --output=results.csv user alice@example.com

  # Classify chat IDs from stdin
  echo "claude_chat_01abc..." | audit classify --stdin

  # Classify and compare to ChatGPT baseline
  audit classify --compare user alice@example.com`)
		os.Exit(1)
	}

	var subcommand, target string
	if !*stdinFlag {
		subcommand = fs.Arg(0)
		target = fs.Arg(1)
	} else {
		subcommand = "chats-stdin"
	}

	// Determine taxonomies to run.
	var taxonomies []string
	switch *taxonomyFlag {
	case "all":
		taxonomies = compliance.AllTaxonomies()
	case "work", "intent", "topic":
		taxonomies = []string{*taxonomyFlag}
	default:
		fatal("unknown taxonomy: %s (use work, intent, topic, or all)", *taxonomyFlag)
	}

	ctx := context.Background()

	// Build compliance client.
	complianceClient, err := buildClient(*apiKey, *orgID)
	if err != nil {
		fatal("creating compliance client: %v", err)
	}

	// Build classifier - either via shell command or API.
	var classifier *compliance.Classifier
	if *classifierCmd != "" {
		// Use shell command for classification (Unix pipes).
		classifier = compliance.NewClassifierWithCmd(*classifierCmd, "claude-cli")
		fmt.Fprintf(os.Stderr, "Using classifier command: %s\n", *classifierCmd)
	} else {
		// Use API for classification.
		var model string
		switch *modelFlag {
		case "haiku":
			model = "claude-3-5-haiku-20241022"
		case "sonnet":
			model = "claude-sonnet-4-20250514"
		default:
			fatal("unknown model: %s (use haiku or sonnet)", *modelFlag)
		}

		classifierKey := *apiKey
		if classifierKey == "" {
			opItem := compliance.DefaultOPItem()
			if opItem == "" {
				fatal("ANTHROPIC_OP_ITEM not set (configure in .env or pass --api-key)")
			}
			opField := compliance.DefaultOPField()
			if opField == "" {
				fatal("COMPLIANCE_OP_FIELD not set (configure in .env or pass --api-key)")
			}
			out, err := exec.Command("op", "item", "get", opItem, "--field", opField, "--reveal").Output()
			if err != nil {
				fatal("reading API key from 1Password: %v", err)
			}
			classifierKey = strings.TrimSpace(string(out))
		}
		classifier = compliance.NewClassifier(classifierKey, model)
		fmt.Fprintf(os.Stderr, "Using classifier model: %s\n", classifier.Model())
	}
	fmt.Fprintf(os.Stderr, "Taxonomies: %s\n", strings.Join(taxonomies, ", "))

	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	// Determine output format.
	outputFormat := *formatFlag
	if *jsonFlag {
		outputFormat = "json"
	}

	var allClassifications []*compliance.Classification

	switch subcommand {
	case "chat":
		allClassifications = classifyChat(ctx, complianceClient, classifier, db, target, taxonomies, *noStoreFlag)

	case "user":
		allClassifications = classifyUser(ctx, complianceClient, classifier, db, target, taxonomies, *limitFlag, *noStoreFlag)

	case "chats-stdin":
		allClassifications = classifyChatsFromStdin(ctx, complianceClient, classifier, db, taxonomies, *noStoreFlag)

	default:
		fatal("unknown subcommand: %s (use chat or user)", subcommand)
	}

	// Output results in requested format.
	outputClassifications(allClassifications, outputFormat, *outputFlag)

	// Show comparison to ChatGPT baseline if requested.
	if *compareFlag {
		printChatGPTComparison(allClassifications)
	}
}

func classifyChat(ctx context.Context, client *compliance.Client, classifier *compliance.Classifier, db *store.Store, chatID string, taxonomies []string, noStore bool) []*compliance.Classification {
	fmt.Fprintf(os.Stderr, "Fetching chat %s...\n", chatID)
	chat, err := client.GetChat(ctx, chatID)
	if err != nil {
		fatal("fetching chat: %v", err)
	}

	// Count user messages.
	userMsgCount := 0
	for _, msg := range chat.ChatMessages {
		if msg.Role == "user" {
			userMsgCount++
		}
	}
	fmt.Fprintf(os.Stderr, "Chat has %d messages (%d from user)\n", len(chat.ChatMessages), userMsgCount)

	if userMsgCount == 0 {
		fmt.Fprintln(os.Stderr, "No user messages to classify.")
		return nil
	}

	fmt.Fprintln(os.Stderr, "Classifying messages...")
	classifications, err := classifier.ClassifyChat(ctx, chat, taxonomies)
	if err != nil {
		fatal("classifying chat: %v", err)
	}

	// Store results unless --no-store was specified.
	if !noStore {
		inserted, err := db.InsertClassifications(classifications)
		if err != nil {
			fatal("storing classifications: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Classified %d messages, stored %d\n", len(classifications), inserted)
	} else {
		fmt.Fprintf(os.Stderr, "Classified %d messages (not stored, --no-store)\n", len(classifications))
	}
	return classifications
}

func classifyUser(ctx context.Context, client *compliance.Client, classifier *compliance.Classifier, db *store.Store, email string, taxonomies []string, limit int, noStore bool) []*compliance.Classification {
	email = strings.ToLower(email)

	// Lookup user ID.
	user, err := db.UserByEmail(email)
	if err != nil {
		fatal("user not found in cache: %s (run 'audit users --refresh' first)", email)
	}
	fmt.Fprintf(os.Stderr, "User: %s (%s)\n", user.Email, user.FullName)

	// Fetch user's chats.
	fmt.Fprintln(os.Stderr, "Fetching chats...")
	chats, err := client.FetchChats(ctx, compliance.ChatQuery{
		UserIDs: []string{user.ID},
	})
	if err != nil {
		fatal("fetching chats: %v", err)
	}

	if len(chats) == 0 {
		fmt.Fprintln(os.Stderr, "No chats found for user.")
		return nil
	}

	if limit > 0 && len(chats) > limit {
		fmt.Fprintf(os.Stderr, "Limiting to %d of %d chats\n", limit, len(chats))
		chats = chats[:limit]
	}

	fmt.Fprintf(os.Stderr, "Found %d chats, classifying...\n\n", len(chats))

	var allClassifications []*compliance.Classification
	for i, chatMeta := range chats {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s: %s\n", i+1, len(chats), truncateID(chatMeta.ID, 20), truncateName(chatMeta.Name, 40))

		chat, err := client.GetChat(ctx, chatMeta.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to fetch chat: %v\n", err)
			continue
		}

		classifications, err := classifier.ClassifyChat(ctx, chat, taxonomies)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: classification failed: %v\n", err)
			continue
		}

		if len(classifications) == 0 {
			fmt.Fprintln(os.Stderr, "  No user messages")
			continue
		}

		// Store results incrementally unless --no-store.
		if !noStore {
			inserted, err := db.InsertClassifications(classifications)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: failed to store: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "  Classified %d messages, stored %d\n", len(classifications), inserted)
			}
		} else {
			fmt.Fprintf(os.Stderr, "  Classified %d messages\n", len(classifications))
		}
		allClassifications = append(allClassifications, classifications...)
	}

	fmt.Fprintf(os.Stderr, "\n=== Completed for %s ===\n", email)
	return allClassifications
}

// classifyChatsFromStdin reads chat IDs from stdin and classifies each one.
func classifyChatsFromStdin(ctx context.Context, client *compliance.Client, classifier *compliance.Classifier, db *store.Store, taxonomies []string, noStore bool) []*compliance.Classification {
	scanner := bufio.NewScanner(os.Stdin)
	var chatIDs []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			chatIDs = append(chatIDs, line)
		}
	}
	if err := scanner.Err(); err != nil {
		fatal("reading stdin: %v", err)
	}

	if len(chatIDs) == 0 {
		fmt.Fprintln(os.Stderr, "No chat IDs provided on stdin.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Classifying %d chats from stdin...\n\n", len(chatIDs))

	var allClassifications []*compliance.Classification
	for i, chatID := range chatIDs {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s\n", i+1, len(chatIDs), chatID)

		chat, err := client.GetChat(ctx, chatID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to fetch chat: %v\n", err)
			continue
		}

		classifications, err := classifier.ClassifyChat(ctx, chat, taxonomies)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: classification failed: %v\n", err)
			continue
		}

		if len(classifications) == 0 {
			fmt.Fprintln(os.Stderr, "  No user messages")
			continue
		}

		if !noStore {
			inserted, err := db.InsertClassifications(classifications)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: failed to store: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "  Classified %d messages, stored %d\n", len(classifications), inserted)
			}
		} else {
			fmt.Fprintf(os.Stderr, "  Classified %d messages\n", len(classifications))
		}
		allClassifications = append(allClassifications, classifications...)
	}

	return allClassifications
}

// outputClassifications writes classifications in the specified format.
func outputClassifications(classifications []*compliance.Classification, format, outputPath string) {
	if len(classifications) == 0 {
		fmt.Fprintln(os.Stderr, "No classifications to output.")
		return
	}

	var out io.Writer = os.Stdout
	var outFile *os.File
	if outputPath != "" {
		var err error
		outFile, err = os.Create(outputPath)
		if err != nil {
			fatal("creating output file: %v", err)
		}
		defer outFile.Close()
		out = outFile
	}

	switch format {
	case "summary":
		printClassificationResults(classifications)

	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(classifications); err != nil {
			fatal("encoding JSON: %v", err)
		}

	case "csv":
		// CSV format matching the database schema.
		_, _ = fmt.Fprintln(out, "message_id,chat_id,user_email,message_created,work_related,intent,topic_fine,topic_coarse,classified_at,classifier_model")
		for _, c := range classifications {
			workRelated := ""
			if c.WorkRelated != nil {
				if *c.WorkRelated {
					workRelated = "1"
				} else {
					workRelated = "0"
				}
			}
			_, _ = fmt.Fprintf(out, "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
				csvEscape(c.MessageID),
				csvEscape(c.ChatID),
				csvEscape(c.UserEmail),
				csvEscape(c.MessageCreated),
				workRelated,
				csvEscape(string(c.Intent)),
				csvEscape(string(c.TopicFine)),
				csvEscape(string(c.TopicCoarse)),
				csvEscape(c.ClassifiedAt),
				csvEscape(c.ClassifierModel))
		}

	case "sql":
		// SQL INSERT statements for the classifications table.
		_, _ = fmt.Fprintln(out, "-- Classifications INSERT statements")
		_, _ = fmt.Fprintln(out, "-- Table: classifications (message_id, chat_id, user_email, message_created, work_related, intent, topic_fine, topic_coarse, classified_at, classifier_model)")
		_, _ = fmt.Fprintln(out)
		for _, c := range classifications {
			workRelated := "NULL"
			if c.WorkRelated != nil {
				if *c.WorkRelated {
					workRelated = "1"
				} else {
					workRelated = "0"
				}
			}
			intent := "NULL"
			if c.Intent != "" {
				intent = fmt.Sprintf("'%s'", sqlEscape(string(c.Intent)))
			}
			topicFine := "NULL"
			if c.TopicFine != "" {
				topicFine = fmt.Sprintf("'%s'", sqlEscape(string(c.TopicFine)))
			}
			topicCoarse := "NULL"
			if c.TopicCoarse != "" {
				topicCoarse = fmt.Sprintf("'%s'", sqlEscape(string(c.TopicCoarse)))
			}
			_, _ = fmt.Fprintf(out, "INSERT OR REPLACE INTO classifications VALUES ('%s', '%s', '%s', '%s', %s, %s, %s, %s, '%s', '%s');\n",
				sqlEscape(c.MessageID),
				sqlEscape(c.ChatID),
				sqlEscape(c.UserEmail),
				sqlEscape(c.MessageCreated),
				workRelated,
				intent,
				topicFine,
				topicCoarse,
				sqlEscape(c.ClassifiedAt),
				sqlEscape(c.ClassifierModel))
		}

	default:
		fatal("unknown format: %s (use summary, json, csv, or sql)", format)
	}

	if outputPath != "" {
		fmt.Fprintf(os.Stderr, "Wrote %d classifications to %s (%s format)\n", len(classifications), outputPath, format)
	}
}

// csvEscape escapes a string for CSV output.
func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// sqlEscape escapes a string for SQL output.
func sqlEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// ChatGPT baseline statistics from "How People Use ChatGPT" (Chatterji et al., 2025).
var chatGPTBaseline = struct {
	WorkPct     float64
	NonWorkPct  float64
	AskingPct   float64
	DoingPct    float64
	ExpressPct  float64
	TopicGroups map[string]float64
}{
	WorkPct:    27.0,
	NonWorkPct: 73.0,
	AskingPct:  49.0,
	DoingPct:   40.0,
	ExpressPct: 11.0,
	TopicGroups: map[string]float64{
		"practical_guidance":  29.0,
		"writing":             24.0,
		"seeking_information": 24.0,
		"technical_help":      7.0,
		"multimedia":          7.0,
		"self_expression":     2.4,
		"other":               6.6,
	},
}

// printChatGPTComparison shows how the classified data compares to ChatGPT baseline.
func printChatGPTComparison(classifications []*compliance.Classification) {
	if len(classifications) == 0 {
		return
	}

	fmt.Println("\n=== Comparison to ChatGPT Baseline ===")
	fmt.Println("(From \"How People Use ChatGPT\", Chatterji et al., 2025)")
	fmt.Println()

	// Calculate our percentages.
	var workCount, nonWorkCount int
	intentCounts := make(map[string]int)
	coarseCounts := make(map[string]int)

	for _, c := range classifications {
		if c.WorkRelated != nil {
			if *c.WorkRelated {
				workCount++
			} else {
				nonWorkCount++
			}
		}
		if c.Intent != "" && c.Intent != compliance.IntentUnknown {
			intentCounts[string(c.Intent)]++
		}
		if c.TopicCoarse != "" {
			coarseCounts[string(c.TopicCoarse)]++
		}
	}

	total := len(classifications)
	workTotal := workCount + nonWorkCount
	intentTotal := intentCounts["asking"] + intentCounts["doing"] + intentCounts["expressing"]
	var topicTotal int
	for _, v := range coarseCounts {
		topicTotal += v
	}

	// Work vs Non-Work comparison.
	if workTotal > 0 {
		ourWorkPct := pct(workCount, workTotal)
		ourNonWorkPct := pct(nonWorkCount, workTotal)
		fmt.Println("## Work vs Non-Work")
		fmt.Printf("  %-20s %6.1f%% vs %5.1f%% (ChatGPT)  %+6.1f%%\n",
			"Work-related:", ourWorkPct, chatGPTBaseline.WorkPct, ourWorkPct-chatGPTBaseline.WorkPct)
		fmt.Printf("  %-20s %6.1f%% vs %5.1f%% (ChatGPT)  %+6.1f%%\n",
			"Non-work:", ourNonWorkPct, chatGPTBaseline.NonWorkPct, ourNonWorkPct-chatGPTBaseline.NonWorkPct)
		fmt.Println()
	}

	// Intent comparison.
	if intentTotal > 0 {
		fmt.Println("## User Intent")
		intents := []struct {
			name     string
			baseline float64
		}{
			{"asking", chatGPTBaseline.AskingPct},
			{"doing", chatGPTBaseline.DoingPct},
			{"expressing", chatGPTBaseline.ExpressPct},
		}
		for _, i := range intents {
			ourPct := pct(intentCounts[i.name], intentTotal)
			fmt.Printf("  %-20s %6.1f%% vs %5.1f%% (ChatGPT)  %+6.1f%%\n",
				i.name+":", ourPct, i.baseline, ourPct-i.baseline)
		}
		fmt.Println()
	}

	// Topic group comparison.
	if topicTotal > 0 {
		fmt.Println("## Topic Groups")
		topicOrder := []string{
			"practical_guidance", "writing", "seeking_information",
			"technical_help", "multimedia", "self_expression", "other",
		}
		for _, topic := range topicOrder {
			ourPct := pct(coarseCounts[topic], topicTotal)
			baseline := chatGPTBaseline.TopicGroups[topic]
			if ourPct > 0 || baseline > 0 {
				fmt.Printf("  %-22s %6.1f%% vs %5.1f%% (ChatGPT)  %+6.1f%%\n",
					topic+":", ourPct, baseline, ourPct-baseline)
			}
		}
		fmt.Println()
	}

	// Summary interpretation.
	fmt.Println("## Interpretation")
	if workTotal > 0 {
		ourWorkPct := pct(workCount, workTotal)
		if ourWorkPct > chatGPTBaseline.WorkPct+10 {
			fmt.Printf("  • Work usage is significantly HIGHER than ChatGPT average (%.0f%% vs %.0f%%)\n",
				ourWorkPct, chatGPTBaseline.WorkPct)
			fmt.Println("    This suggests strong enterprise/professional adoption.")
		} else if ourWorkPct < chatGPTBaseline.WorkPct-10 {
			fmt.Printf("  • Work usage is LOWER than ChatGPT average (%.0f%% vs %.0f%%)\n",
				ourWorkPct, chatGPTBaseline.WorkPct)
		}
	}

	if coarseCounts["technical_help"] > 0 && topicTotal > 0 {
		ourTechPct := pct(coarseCounts["technical_help"], topicTotal)
		if ourTechPct > chatGPTBaseline.TopicGroups["technical_help"]+10 {
			fmt.Printf("  • Technical/coding usage is significantly HIGHER (%.0f%% vs %.0f%%)\n",
				ourTechPct, chatGPTBaseline.TopicGroups["technical_help"])
			fmt.Println("    Common for developer-heavy organizations.")
		}
	}

	fmt.Printf("\n  Total messages analyzed: %d\n", total)
}

func printClassificationResults(classifications []*compliance.Classification) {
	if len(classifications) == 0 {
		return
	}

	// Calculate statistics.
	var workCount, nonWorkCount int
	intentCounts := make(map[compliance.Intent]int)
	topicCounts := make(map[compliance.TopicFine]int)
	coarseCounts := make(map[compliance.TopicCoarse]int)

	for _, c := range classifications {
		if c.WorkRelated != nil {
			if *c.WorkRelated {
				workCount++
			} else {
				nonWorkCount++
			}
		}
		if c.Intent != "" && c.Intent != compliance.IntentUnknown {
			intentCounts[c.Intent]++
		}
		if c.TopicFine != "" {
			topicCounts[c.TopicFine]++
			coarseCounts[c.TopicCoarse]++
		}
	}

	total := len(classifications)
	fmt.Printf("\nTotal messages classified: %d\n", total)

	// Work/Non-work.
	if workCount+nonWorkCount > 0 {
		fmt.Printf("\n## Work vs Non-Work\n")
		fmt.Printf("  Work-related:     %3d (%5.1f%%)\n", workCount, pct(workCount, total))
		fmt.Printf("  Non-work:         %3d (%5.1f%%)\n", nonWorkCount, pct(nonWorkCount, total))
	}

	// Intent.
	if len(intentCounts) > 0 {
		fmt.Printf("\n## User Intent\n")
		for _, intent := range []compliance.Intent{compliance.IntentAsking, compliance.IntentDoing, compliance.IntentExpressing} {
			count := intentCounts[intent]
			fmt.Printf("  %-15s %3d (%5.1f%%)\n", intent+":", count, pct(count, total))
		}
	}

	// Coarse topics.
	if len(coarseCounts) > 0 {
		fmt.Printf("\n## Topic Groups\n")
		coarseOrder := []compliance.TopicCoarse{
			compliance.TopicCoarsePracticalGuidance,
			compliance.TopicCoarseWriting,
			compliance.TopicCoarseSeekingInfo,
			compliance.TopicCoarseTechnicalHelp,
			compliance.TopicCoarseMultimedia,
			compliance.TopicCoarseSelfExpression,
			compliance.TopicCoarseOther,
		}
		for _, topic := range coarseOrder {
			count := coarseCounts[topic]
			if count > 0 {
				fmt.Printf("  %-22s %3d (%5.1f%%)\n", topic+":", count, pct(count, total))
			}
		}
	}

	// Fine topics (top 10).
	if len(topicCounts) > 0 {
		fmt.Printf("\n## Top Topics (fine-grained)\n")
		type kv struct {
			k compliance.TopicFine
			v int
		}
		var sorted []kv
		for k, v := range topicCounts {
			sorted = append(sorted, kv{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })

		shown := 10
		if len(sorted) < shown {
			shown = len(sorted)
		}
		for i := 0; i < shown; i++ {
			fmt.Printf("  %-35s %3d (%5.1f%%)\n", sorted[i].k+":", sorted[i].v, pct(sorted[i].v, total))
		}
	}
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100 / float64(total)
}

// printChatGPTComparisonFromSummary shows baseline comparison from a stored ClassificationSummary.
func printChatGPTComparisonFromSummary(summary *store.ClassificationSummary) {
	fmt.Println("\n=== Comparison to ChatGPT Baseline ===")
	fmt.Println("(From \"How People Use ChatGPT\", Chatterji et al., 2025)")
	fmt.Println()

	workTotal := summary.WorkRelated + summary.NonWorkRelated
	intentTotal := summary.IntentAsking + summary.IntentDoing + summary.IntentExpressing
	var topicTotal int
	for _, v := range summary.CoarseTopicCounts {
		topicTotal += v
	}

	if workTotal > 0 {
		fmt.Println("## Work vs Non-Work")
		ourWorkPct := pct(summary.WorkRelated, workTotal)
		ourNonWorkPct := pct(summary.NonWorkRelated, workTotal)
		fmt.Printf("  %-20s %6.1f%% vs %5.1f%% (ChatGPT)  %+6.1f%%\n",
			"Work-related:", ourWorkPct, chatGPTBaseline.WorkPct, ourWorkPct-chatGPTBaseline.WorkPct)
		fmt.Printf("  %-20s %6.1f%% vs %5.1f%% (ChatGPT)  %+6.1f%%\n",
			"Non-work:", ourNonWorkPct, chatGPTBaseline.NonWorkPct, ourNonWorkPct-chatGPTBaseline.NonWorkPct)
		fmt.Println()
	}

	if intentTotal > 0 {
		fmt.Println("## User Intent")
		intents := []struct {
			name     string
			count    int
			baseline float64
		}{
			{"asking", summary.IntentAsking, chatGPTBaseline.AskingPct},
			{"doing", summary.IntentDoing, chatGPTBaseline.DoingPct},
			{"expressing", summary.IntentExpressing, chatGPTBaseline.ExpressPct},
		}
		for _, i := range intents {
			ourPct := pct(i.count, intentTotal)
			fmt.Printf("  %-20s %6.1f%% vs %5.1f%% (ChatGPT)  %+6.1f%%\n",
				i.name+":", ourPct, i.baseline, ourPct-i.baseline)
		}
		fmt.Println()
	}

	if topicTotal > 0 {
		fmt.Println("## Topic Groups")
		topicOrder := []string{
			"practical_guidance", "writing", "seeking_information",
			"technical_help", "multimedia", "self_expression", "other",
		}
		for _, topic := range topicOrder {
			ourPct := pct(summary.CoarseTopicCounts[topic], topicTotal)
			baseline := chatGPTBaseline.TopicGroups[topic]
			if ourPct > 0 || baseline > 0 {
				fmt.Printf("  %-22s %6.1f%% vs %5.1f%% (ChatGPT)  %+6.1f%%\n",
					topic+":", ourPct, baseline, ourPct-baseline)
			}
		}
		fmt.Println()
	}

	fmt.Printf("Total messages analyzed: %d\n", summary.TotalMessages)
}

// buildAnalyticsClient creates an Analytics API client from an explicit key
// or via 1Password fallback.
func buildAnalyticsClient(apiKey string) (*analytics.Client, error) {
	if apiKey != "" {
		return analytics.NewClient(apiKey), nil
	}
	return analytics.NewClientFrom1Password("", "")
}

func buildOktaClient(apiKey string) (*okta.Client, error) {
	domain := okta.DefaultDomain()
	if domain == "" {
		return nil, fmt.Errorf(
			"OKTA_DOMAIN not set (configure in .env)",
		)
	}
	if apiKey != "" {
		return okta.NewClient(apiKey, domain), nil
	}
	return okta.NewClientFrom1Password("", "", domain)
}

// cmdAnalytics fetches per-user daily analytics and stores them locally.
// It computes missing dates and fetches only those (unless --refresh).
func cmdAnalyticsUsers(args []string) {
	fs := flag.NewFlagSet("analytics-users", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	apiKey := fs.String("analytics-api-key", "", "Analytics API key (if unset, reads from 1Password)")
	days := fs.Int("days", 30, "Number of days of history")
	refreshFlag := fs.Bool("refresh", false, "Re-fetch all dates")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	ctx := context.Background()

	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	client, err := buildAnalyticsClient(*apiKey)
	if err != nil {
		fatal("creating analytics client: %v", err)
	}

	// The Analytics API has a 3-day data lag.
	endDate := time.Now().UTC().AddDate(0, 0, -3)
	startDate := endDate.AddDate(0, 0, -*days)

	// Determine which dates we already have cached.
	var alreadyFetched map[string]bool
	if !*refreshFlag {
		dates, err := db.AnalyticsFetchedDates()
		if err != nil {
			fatal("reading cached dates: %v", err)
		}
		alreadyFetched = make(map[string]bool, len(dates))
		for _, d := range dates {
			alreadyFetched[d] = true
		}
	}

	totalUsers := 0
	datesFetched := 0
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		if !*refreshFlag && alreadyFetched[dateStr] {
			continue
		}

		metrics, err := client.FetchUserMetrics(ctx, dateStr)
		if err != nil {
			fatal("fetching %s: %v", dateStr, err)
		}

		n, err := db.InsertUserDailyMetrics(
			metrics, dateStr, time.Now().UTC(),
		)
		if err != nil {
			fatal("storing metrics for %s: %v", dateStr, err)
		}

		fmt.Fprintf(os.Stderr, "Fetching %s... %d users\n", dateStr, n)
		totalUsers += n
		datesFetched++
	}

	if err := db.SetAnalyticsLastFetchedAt(time.Now().UTC()); err != nil {
		fatal("updating last fetched time: %v", err)
	}

	if datesFetched == 0 {
		fmt.Fprintln(os.Stderr, "All dates already cached (use --refresh to re-fetch)")
	} else {
		fmt.Fprintf(os.Stderr,
			"\nDone: %d dates fetched, %d total user-day records stored\n",
			datesFetched, totalUsers,
		)
	}
}

// cmdAnalyticsSummary fetches and displays org-level DAU/WAU/MAU data.
func cmdAnalyticsSummary(args []string) {
	fs := flag.NewFlagSet("analytics-summary", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	apiKey := fs.String("analytics-api-key", "", "Analytics API key (if unset, reads from 1Password)")
	days := fs.Int("days", 30, "Number of days of history")
	jsonFlag := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	ctx := context.Background()

	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	client, err := buildAnalyticsClient(*apiKey)
	if err != nil {
		fatal("creating analytics client: %v", err)
	}

	endDate := time.Now().UTC().AddDate(0, 0, -3)
	startDate := endDate.AddDate(0, 0, -*days)
	startStr := startDate.Format("2006-01-02")
	endStr := endDate.Format("2006-01-02")

	// The summaries endpoint's ending_date is exclusive, so add one day
	// to include endDate in the results. The API enforces a max 31-day
	// span, so chunk larger ranges.
	fetchEnd := endDate.AddDate(0, 0, 1)
	chunkStart := startDate
	for chunkStart.Before(fetchEnd) {
		chunkEnd := chunkStart.AddDate(0, 0, 31)
		if chunkEnd.After(fetchEnd) {
			chunkEnd = fetchEnd
		}
		summaries, err := client.FetchSummaries(
			ctx,
			chunkStart.Format("2006-01-02"),
			chunkEnd.Format("2006-01-02"),
		)
		if err != nil {
			fatal("fetching summaries: %v", err)
		}
		if err := db.InsertOrgDailySummaries(summaries, time.Now().UTC()); err != nil {
			fatal("storing summaries: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Fetched %d days of summaries (%s to %s)\n",
			len(summaries), chunkStart.Format("2006-01-02"),
			chunkEnd.AddDate(0, 0, -1).Format("2006-01-02"))
		chunkStart = chunkEnd
	}

	stored, err := db.OrgDailySummaries(startStr, endStr)
	if err != nil {
		fatal("reading summaries: %v", err)
	}

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(stored); err != nil {
			fatal("encoding JSON: %v", err)
		}
		return
	}

	fmt.Printf("%-12s %5s %5s %5s %6s %8s\n",
		"Date", "DAU", "WAU", "MAU", "Seats", "Pending")
	fmt.Println(strings.Repeat("\u2500", 48))

	for _, s := range stored {
		fmt.Printf("%-12s %5d %5d %5d %6d %8d\n",
			s.Date, s.DailyActiveUsers, s.WeeklyActiveUsers,
			s.MonthlyActiveUsers, s.AssignedSeats, s.PendingInvites,
		)
	}

	fmt.Fprintf(os.Stderr, "\n%d days (%s to %s)\n",
		len(stored), startStr, endStr)
}

// cmdUsageReport generates an aggregated classification report.
// cmdRank produces a stack-ranked engagement table of licensed users.
// It auto-fetches fresh data when the local cache is stale (>1 hour).
func cmdRank(args []string) {
	fs := flag.NewFlagSet("rank", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	orgID := fs.String("org", compliance.DefaultOrgID(), "Organization ID")
	apiKey := fs.String("api-key", "",
		"Compliance API key (if unset, reads from 1Password)")
	analyticsKey := fs.String("analytics-api-key", "",
		"Analytics API key (if unset, reads from 1Password)")
	days := fs.Int("days", 30, "Number of days of history")
	refreshFlag := fs.Bool("refresh", false, "Force re-fetch before ranking")
	jsonFlag := fs.Bool("json", false, "Output as JSON")
	reclaimFlag := fs.Bool("reclaim", false,
		"Seat reclamation safety analysis mode")
	csvPath := fs.String("csv", "",
		"CSV/ZIP audit log for cross-reference (requires --reclaim)")
	graceDays := fs.Int("grace", 21,
		"Grace period for new accounts in days (requires --reclaim)")
	tierFilter := fs.String("tier", "",
		"Filter: safe, investigate, dnr (requires --reclaim)")
	oktaFlag := fs.Bool("okta", false,
		"Cross-reference with Okta Claude SSO logs (requires --reclaim)")
	oktaAPIKey := fs.String("okta-api-key", "",
		"Okta API token (if unset, reads from 1Password)")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	// Reject reclaim-only flags when not in reclaim mode.
	if !*reclaimFlag {
		if *csvPath != "" {
			fatal("--csv requires --reclaim")
		}
		if *tierFilter != "" {
			fatal("--tier requires --reclaim")
		}
		if *oktaFlag {
			fatal("--okta requires --reclaim")
		}
		graceExplicit := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "grace" {
				graceExplicit = true
			}
		})
		if graceExplicit {
			fatal("--grace requires --reclaim")
		}
	}
	if *oktaAPIKey != "" && !*oktaFlag {
		fatal("--okta-api-key requires --okta")
	}
	if *oktaFlag && *days > 90 {
		fatal("--okta supports at most 90 days because Okta System Log retention is 90 days")
	}
	if *tierFilter != "" {
		switch strings.ToLower(*tierFilter) {
		case "safe", "investigate", "dnr":
		default:
			fatal(
				"invalid --tier %q (expected safe, investigate, or dnr)",
				*tierFilter,
			)
		}
	}

	ctx := context.Background()
	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	in := loadRankInputs(
		ctx, db, *apiKey, *orgID, *analyticsKey,
		*days, *refreshFlag, *reclaimFlag,
	)

	if *reclaimFlag {
		rankRunReclaim(db, in, reclaimOptions{
			csvPath:    *csvPath,
			graceDays:  *graceDays,
			tierFilter: *tierFilter,
			jsonOutput: *jsonFlag,
			days:       *days,
			oktaFlag:   *oktaFlag,
			oktaAPIKey: *oktaAPIKey,
		})
		return
	}

	// Normal rank mode below.
	type rankedUser struct {
		Email         string `json:"email"`
		Events        int    `json:"events"`
		Chats         int    `json:"chats"`
		Projects      int    `json:"projects"`
		Shares        int    `json:"shares"`
		Files         int    `json:"files"`
		DaysActive    int    `json:"days_active"`
		DaysSinceLast int    `json:"days_since_last"`
		LicenseDays   int    `json:"license_days"`
		Conversations int    `json:"conversations"`
		Messages      int    `json:"messages"`
		CCSessions    int    `json:"cc_sessions"`
		Category      string `json:"category"`
	}

	now := time.Now().UTC()
	var ranked []rankedUser
	hasAnalytics := in.hasAnalytics

	for _, u := range in.users {
		s, found := in.summaryMap[u.Email]
		as := in.analyticsMap[u.Email]

		licDays := 0
		if ct, ok := in.userCreated[u.Email]; ok {
			licDays = int(now.Sub(ct).Hours()/24) + 1
		}

		if !found {
			cat := "ZERO"
			if hasAnalyticsActivity(as) {
				cat = "CODE ONLY"
			}
			ranked = append(ranked, rankedUser{
				Email:         u.Email,
				DaysSinceLast: -1,
				LicenseDays:   licDays,
				Conversations: as.Conversations,
				Messages:      as.Messages,
				CCSessions:    as.CCSessions,
				Category:      cat,
			})
			continue
		}

		daysSince := -1
		if s.LastSeen != "" {
			if lastT, err := time.Parse(
				time.RFC3339Nano, s.LastSeen,
			); err == nil {
				daysSince = int(now.Sub(lastT).Hours() / 24)
			}
		}

		cat := rankCategory(
			s.EventCount, s.ChatsCreated, s.ActiveDays, daysSince,
		)
		ranked = append(ranked, rankedUser{
			Email:         s.Email,
			Events:        s.EventCount,
			Chats:         s.ChatsCreated,
			Projects:      s.ProjectsCreated,
			Shares:        s.Shares,
			Files:         s.FilesUploaded,
			DaysActive:    s.ActiveDays,
			DaysSinceLast: daysSince,
			LicenseDays:   licDays,
			Conversations: as.Conversations,
			Messages:      as.Messages,
			CCSessions:    as.CCSessions,
			Category:      cat,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		pi := categoryPriority(ranked[i].Category)
		pj := categoryPriority(ranked[j].Category)
		if pi != pj {
			return pi < pj
		}
		if ranked[i].DaysSinceLast != ranked[j].DaysSinceLast {
			return ranked[i].DaysSinceLast > ranked[j].DaysSinceLast
		}
		return ranked[i].Events < ranked[j].Events
	})

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(ranked); err != nil {
			fatal("encoding JSON: %v", err)
		}
		return
	}

	if hasAnalytics {
		fmt.Printf(
			"%-5s %-40s %7s %6s %5s %5s %5s %5s %-8s %4s %5s %5s %4s %s\n",
			"Rank", "Email", "Events", "Chats", "Proj", "Share",
			"Files", "Days", "Last", "Lic",
			"Conv", "Msgs", "CC", "Category")
		fmt.Println(strings.Repeat("\u2500", 135))
	} else {
		fmt.Printf(
			"%-5s %-40s %7s %6s %5s %5s %5s %5s %-8s %4s %s\n",
			"Rank", "Email", "Events", "Chats", "Proj", "Share",
			"Files", "Days", "Last", "Lic", "Category")
		fmt.Println(strings.Repeat("\u2500", 110))
	}

	catCounts := make(map[string]int)
	for i, u := range ranked {
		lastStr := "never"
		if u.DaysSinceLast == 0 {
			lastStr = "today"
		} else if u.DaysSinceLast > 0 {
			lastStr = fmt.Sprintf("%dd ago", u.DaysSinceLast)
		}

		if hasAnalytics {
			fmt.Printf(
				"%-5d %-40s %7d %6d %5d %5d %5d %5d %-8s %4d %5d %5d %4d %s\n",
				i+1, u.Email, u.Events, u.Chats,
				u.Projects, u.Shares, u.Files,
				u.DaysActive, lastStr, u.LicenseDays,
				u.Conversations, u.Messages, u.CCSessions,
				u.Category)
		} else {
			fmt.Printf(
				"%-5d %-40s %7d %6d %5d %5d %5d %5d %-8s %4d %s\n",
				i+1, u.Email, u.Events, u.Chats,
				u.Projects, u.Shares, u.Files,
				u.DaysActive, lastStr, u.LicenseDays, u.Category)
		}
		catCounts[u.Category]++
	}

	var parts []string
	cats := []string{
		"ZERO", "CODE ONLY", "VIEW ONLY", "BARELY TRIED",
		"DORMANT", "MINIMAL", "OCCASIONAL", "REGULAR+",
	}
	for _, cat := range cats {
		if n, ok := catCounts[cat]; ok {
			parts = append(parts,
				fmt.Sprintf("%d %s", n, strings.ToLower(cat)))
		}
	}
	fmt.Fprintf(os.Stderr, "\nTotal: %d licensed users (%s)\n",
		len(ranked), strings.Join(parts, ", "))

	if hasAnalytics {
		fmt.Fprintf(os.Stderr,
			"Note: Analytics data has a 3-day lag; compliance data is near real-time.\n")
	}
}

// rankEnsureFreshData auto-fetches activities and users when the local cache
// is stale (activities >1 hour old, users >4 hours old). The --refresh flag
// forces a full re-fetch.
func rankEnsureFreshData(
	ctx context.Context, db *store.Store,
	apiKey, orgID string, days int, refresh bool,
) {
	const activityTTL = 1 * time.Hour
	const userTTL = 4 * time.Hour

	lastFetch, _ := db.LastFetchedAt()
	activitiesStale := lastFetch.IsZero() || time.Since(lastFetch) > activityTTL

	usersFetched, _ := db.UsersFetchedAt()
	usersStale := usersFetched.IsZero() || time.Since(usersFetched) > userTTL

	if !activitiesStale && !usersStale && !refresh {
		return
	}

	client, err := buildClient(apiKey, orgID)
	if err != nil {
		fatal("creating API client: %v", err)
	}

	if activitiesStale || refresh {
		if refresh {
			fmt.Fprintln(os.Stderr, "Refreshing activity data...")
		} else {
			fmt.Fprintln(os.Stderr, "Activity data stale, fetching...")
		}
		rankFetchActivities(ctx, client, db, days)
	}

	if usersStale || refresh {
		fmt.Fprintln(os.Stderr, "Fetching licensed users...")
		users, err := client.FetchUsers(ctx)
		if err != nil {
			fatal("fetching users: %v", err)
		}
		if err := db.InsertUsers(users, time.Now().UTC()); err != nil {
			fatal("storing users: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Cached %d users\n", len(users))
	}
}

// rankFetchActivities runs the same two-phase fetch used by cmdFetch, but
// with no extra output beyond progress lines from FetchActivities itself.
func rankFetchActivities(
	ctx context.Context, client *compliance.Client, db *store.Store, days int,
) {
	since := time.Now().UTC().AddDate(0, 0, -days)
	totalInserted := 0

	onPage := func(updateHWM bool) func(compliance.PageResult) error {
		return func(pr compliance.PageResult) error {
			n, err := db.InsertActivities(pr.Activities)
			if err != nil {
				return err
			}
			totalInserted += n
			if updateHWM && len(pr.Activities) > 0 {
				return db.SetHighWaterMark(pr.Activities[0].ID)
			}
			return nil
		}
	}

	forwardOK := true
	hwm, _ := db.HighWaterMark()
	if hwm != "" {
		query := compliance.ActivityQuery{
			BeforeID: hwm,
			OnPage:   onPage(true),
		}
		if _, err := client.FetchActivities(ctx, query); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: forward fetch failed: %v\n", err)
			forwardOK = false
		}
	}

	targetDate := since.Format("2006-01-02")
	oldestID, oldestDate, _ := db.OldestActivity()
	if oldestDate == "" || oldestDate > targetDate {
		query := compliance.ActivityQuery{
			CreatedAtGte: &since,
			OnPage:       onPage(hwm == ""),
		}
		if oldestID != "" {
			query.AfterID = oldestID
		}
		if _, err := client.FetchActivities(ctx, query); err != nil {
			fatal("backfill fetch: %v", err)
		}
	}

	if totalInserted > 0 {
		fmt.Fprintf(os.Stderr, "%d new activities stored\n", totalInserted)
	}

	// Only mark data as fresh if the forward fetch succeeded. A failed
	// forward fetch means we may have missed new activities, so leaving
	// lastFetchedAt stale ensures the next run retries.
	if forwardOK {
		if err := db.SetLastFetchedAt(time.Now().UTC()); err != nil {
			fatal("updating last fetched time: %v", err)
		}
	}
}

// rankEnsureFreshAnalytics auto-fetches analytics data when stale (>1 hour).
// Errors are logged as warnings rather than fatal — analytics is supplementary.
func rankEnsureFreshAnalytics(
	ctx context.Context, db *store.Store,
	analyticsKey string, days int, refresh, required bool,
) {
	const analyticsTTL = 1 * time.Hour

	lastFetch, _ := db.AnalyticsLastFetchedAt()
	if !lastFetch.IsZero() && time.Since(lastFetch) <= analyticsTTL && !refresh {
		return
	}

	client, err := buildAnalyticsClient(analyticsKey)
	if err != nil {
		if required {
			fatal("analytics required but unavailable: %v", err)
		}
		fmt.Fprintf(os.Stderr,
			"Warning: analytics unavailable (%v), skipping\n", err)
		return
	}

	endDate := time.Now().UTC().AddDate(0, 0, -3)
	startDate := endDate.AddDate(0, 0, -days)

	var alreadyFetched map[string]bool
	if !refresh {
		dates, err := db.AnalyticsFetchedDates()
		if err == nil {
			alreadyFetched = make(map[string]bool, len(dates))
			for _, d := range dates {
				alreadyFetched[d] = true
			}
		}
	}

	datesFetched := 0
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		if !refresh && alreadyFetched[dateStr] {
			continue
		}

		metrics, err := client.FetchUserMetrics(ctx, dateStr)
		if err != nil {
			if required {
				fatal("analytics fetch failed for %s: %v", dateStr, err)
			}
			fmt.Fprintf(os.Stderr,
				"Warning: analytics fetch failed for %s: %v\n", dateStr, err)
			return
		}

		if _, err := db.InsertUserDailyMetrics(
			metrics, dateStr, time.Now().UTC(),
		); err != nil {
			if required {
				fatal("storing analytics for %s: %v", dateStr, err)
			}
			fmt.Fprintf(os.Stderr,
				"Warning: storing analytics for %s: %v\n", dateStr, err)
			return
		}
		datesFetched++
	}

	if datesFetched > 0 {
		fmt.Fprintf(os.Stderr,
			"Fetched analytics for %d dates\n", datesFetched)
	}
	if err := db.SetAnalyticsLastFetchedAt(time.Now().UTC()); err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: could not persist analytics fetch timestamp: %v\n", err)
	}
}

// rankInputs holds the shared data loaded by both rank and reclaim modes.
type rankInputs struct {
	summaryMap   map[string]store.StoredUserSummary
	analyticsMap map[string]store.AnalyticsUserSummary
	hasAnalytics bool
	users        []store.CachedUser
	userCreated  map[string]time.Time
}

// loadRankInputs fetches fresh compliance and analytics data, then loads
// the summaries and user list needed by both rank and reclaim modes. When
// analyticsRequired is true, any analytics failure is fatal.
func loadRankInputs(
	ctx context.Context, db *store.Store,
	apiKey, orgID, analyticsKey string,
	days int, refresh, analyticsRequired bool,
) rankInputs {
	rankEnsureFreshData(ctx, db, apiKey, orgID, days, refresh)
	rankEnsureFreshAnalytics(
		ctx, db, analyticsKey, days, refresh, analyticsRequired,
	)

	since := time.Now().UTC().AddDate(0, 0, -days)
	summaries, err := db.UserSummaries(since)
	if err != nil {
		fatal("querying user summaries: %v", err)
	}
	summaryMap := make(
		map[string]store.StoredUserSummary, len(summaries),
	)
	for _, s := range summaries {
		summaryMap[s.Email] = s
	}

	endDate := time.Now().UTC().AddDate(0, 0, -3)
	startDate := endDate.AddDate(0, 0, -days)
	asList, err := db.AnalyticsUserSummaries(
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"),
	)
	analyticsLoaded := err == nil
	if err != nil {
		if analyticsRequired {
			fatal("loading analytics summaries: %v", err)
		}
		fmt.Fprintf(os.Stderr,
			"Warning: could not load analytics: %v\n", err)
	}
	analyticsMap := make(
		map[string]store.AnalyticsUserSummary, len(asList),
	)
	for _, a := range asList {
		analyticsMap[a.Email] = a
	}

	users, err := db.Users()
	if err != nil {
		fatal("reading users: %v", err)
	}

	userCreated := make(map[string]time.Time, len(users))
	for _, u := range users {
		if t, err := time.Parse(
			time.RFC3339Nano, u.CreatedAt,
		); err == nil {
			userCreated[u.Email] = t
		}
	}

	return rankInputs{
		summaryMap:   summaryMap,
		analyticsMap: analyticsMap,
		hasAnalytics: analyticsLoaded,
		users:        users,
		userCreated:  userCreated,
	}
}

// hasAnalyticsActivity returns true if any analytics metric is non-zero.
func hasAnalyticsActivity(as store.AnalyticsUserSummary) bool {
	return as.Conversations > 0 || as.Messages > 0 ||
		as.CCCommits > 0 || as.CCSessions > 0 ||
		as.CCPullRequests > 0 || as.CCLinesAdded > 0 ||
		as.WebSearches > 0 || as.ConnectorsUsed > 0 ||
		as.SkillsUsed > 0 || as.ArtifactsCreated > 0 ||
		as.ThinkingMessages > 0 || as.ProjectsCreated > 0 ||
		as.FilesUploaded > 0
}

// rankCategory assigns an engagement category based on activity metrics.
// Dormancy takes precedence over low-engagement categories so that users
// who briefly tried Claude and then went silent are flagged as dormant.
func rankCategory(events, chats, activeDays, daysSinceLast int) string {
	if events == 0 {
		return "ZERO"
	}
	if chats == 0 {
		return "VIEW ONLY"
	}
	if daysSinceLast > 14 {
		return "DORMANT"
	}
	if activeDays <= 2 && chats <= 3 {
		return "BARELY TRIED"
	}
	if activeDays <= 5 && chats <= 5 {
		return "MINIMAL"
	}
	if activeDays <= 10 {
		return "OCCASIONAL"
	}
	return "REGULAR+"
}

// categoryPriority returns a sort key for engagement categories, with the
// least engaged (most reclaimable) categories first.
func categoryPriority(cat string) int {
	switch cat {
	case "ZERO":
		return 0
	case "CODE ONLY":
		return 1
	case "VIEW ONLY":
		return 2
	case "DORMANT":
		return 3
	case "BARELY TRIED":
		return 4
	case "MINIMAL":
		return 5
	case "OCCASIONAL":
		return 6
	case "REGULAR+":
		return 7
	default:
		return 8
	}
}

func cmdUsageReport(args []string) {
	fs := flag.NewFlagSet("usage-report", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	userFilter := fs.String("user", "", "Filter by user email")
	periodFlag := fs.Int("period", 0, "Filter by last N days (0 = all time)")
	jsonFlag := fs.Bool("json", false, "Output as JSON")
	compareFlag := fs.Bool("compare", false, "Show comparison to ChatGPT baseline")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	opts := store.ClassificationQueryOpts{
		UserEmail: *userFilter,
	}
	if *periodFlag > 0 {
		since := time.Now().UTC().AddDate(0, 0, -*periodFlag)
		opts.Since = &since
	}

	summary, err := db.GetClassificationSummary(opts)
	if err != nil {
		fatal("getting classification summary: %v", err)
	}

	if summary.TotalMessages == 0 {
		fmt.Fprintln(os.Stderr, "No classifications found. Run 'audit classify' first.")
		return
	}

	if *jsonFlag {
		printUsageReportJSON(summary, *userFilter, *periodFlag)
		return
	}

	printUsageReportText(summary, *userFilter, *periodFlag)

	if *compareFlag {
		printChatGPTComparisonFromSummary(summary)
	}
}

func printUsageReportText(summary *store.ClassificationSummary, user string, period int) {
	fmt.Println("=== Claude Usage Classification Report ===")
	fmt.Println()

	if user != "" {
		fmt.Printf("User: %s\n", user)
	}
	if period > 0 {
		fmt.Printf("Period: last %d days\n", period)
	}
	fmt.Printf("Total messages: %d\n", summary.TotalMessages)
	fmt.Println()

	// Work vs Non-Work.
	workTotal := summary.WorkRelated + summary.NonWorkRelated
	if workTotal > 0 {
		fmt.Println("## Work vs Non-Work")
		fmt.Printf("  Work-related:     %4d (%5.1f%%)\n", summary.WorkRelated, pct(summary.WorkRelated, workTotal))
		fmt.Printf("  Non-work:         %4d (%5.1f%%)\n", summary.NonWorkRelated, pct(summary.NonWorkRelated, workTotal))
		if summary.WorkUnknown > 0 {
			fmt.Printf("  Unknown:          %4d\n", summary.WorkUnknown)
		}
		fmt.Println()
	}

	// Intent.
	intentTotal := summary.IntentAsking + summary.IntentDoing + summary.IntentExpressing
	if intentTotal > 0 {
		fmt.Println("## User Intent")
		fmt.Printf("  Asking:           %4d (%5.1f%%)\n", summary.IntentAsking, pct(summary.IntentAsking, intentTotal))
		fmt.Printf("  Doing:            %4d (%5.1f%%)\n", summary.IntentDoing, pct(summary.IntentDoing, intentTotal))
		fmt.Printf("  Expressing:       %4d (%5.1f%%)\n", summary.IntentExpressing, pct(summary.IntentExpressing, intentTotal))
		if summary.IntentUnknown > 0 {
			fmt.Printf("  Unknown:          %4d\n", summary.IntentUnknown)
		}
		fmt.Println()
	}

	// Topic Groups.
	if len(summary.CoarseTopicCounts) > 0 {
		fmt.Println("## Topic Groups")
		coarseTotal := 0
		for _, v := range summary.CoarseTopicCounts {
			coarseTotal += v
		}
		coarseOrder := []string{
			"practical_guidance", "writing", "seeking_information",
			"technical_help", "multimedia", "self_expression", "other",
		}
		for _, topic := range coarseOrder {
			count := summary.CoarseTopicCounts[topic]
			if count > 0 {
				fmt.Printf("  %-22s %4d (%5.1f%%)\n", topic+":", count, pct(count, coarseTotal))
			}
		}
		fmt.Println()
	}

	// Fine Topics (top 10).
	if len(summary.TopicCounts) > 0 {
		fmt.Println("## Top Topics (fine-grained)")
		type kv struct {
			k string
			v int
		}
		var sorted []kv
		topicTotal := 0
		for k, v := range summary.TopicCounts {
			sorted = append(sorted, kv{k, v})
			topicTotal += v
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })

		shown := 10
		if len(sorted) < shown {
			shown = len(sorted)
		}
		for i := 0; i < shown; i++ {
			fmt.Printf("  %-35s %4d (%5.1f%%)\n", sorted[i].k+":", sorted[i].v, pct(sorted[i].v, topicTotal))
		}
	}
}

func printUsageReportJSON(summary *store.ClassificationSummary, user string, period int) {
	report := map[string]interface{}{
		"total_messages": summary.TotalMessages,
		"work_non_work": map[string]int{
			"work":    summary.WorkRelated,
			"non_work": summary.NonWorkRelated,
			"unknown": summary.WorkUnknown,
		},
		"intent": map[string]int{
			"asking":     summary.IntentAsking,
			"doing":      summary.IntentDoing,
			"expressing": summary.IntentExpressing,
			"unknown":    summary.IntentUnknown,
		},
		"topic_groups":     summary.CoarseTopicCounts,
		"topics_fine":      summary.TopicCounts,
	}
	if user != "" {
		report["user"] = user
	}
	if period > 0 {
		report["period_days"] = period
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fatal("encoding JSON: %v", err)
	}
}

func fmtDateTime(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339Nano, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.Format("2006-01-02 15:04:05")
}

func printTypeCounts(counts map[string]int) {
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
	for _, kv := range sorted {
		fmt.Printf("  %-45s %8d\n", kv.k, kv.v)
	}
}

func printStoreSummary(db *store.Store) {
	total, _ := db.ActivityCount()
	users, _ := db.UniqueUserCount()
	earliest, latest, _ := db.DateRange()
	typeCounts, _ := db.EventTypeCounts()

	fmt.Printf("\nTotal: %d activities (%s to %s)\n", total, fmtDate(earliest), fmtDate(latest))
	fmt.Printf("%d unique users with activity\n", users)

	if len(typeCounts) > 0 {
		fmt.Println("\nEvent type breakdown:")
		type kv struct {
			k string
			v int
		}
		var sorted []kv
		for k, v := range typeCounts {
			sorted = append(sorted, kv{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
		for _, kv := range sorted {
			fmt.Printf("  %-45s %6d\n", kv.k, kv.v)
		}
	}
}

func buildClient(apiKey, orgID string) (*compliance.Client, error) {
	if orgID == "" {
		orgID = compliance.DefaultOrgID()
	}
	if orgID == "" {
		return nil, fmt.Errorf("ANTHROPIC_ORG_ID not set (configure in .env or pass --org)")
	}
	if apiKey != "" {
		return compliance.NewClient(apiKey, orgID), nil
	}
	return compliance.NewClientFrom1Password("", "", orgID)
}

func fmtDate(rfc3339 string) string {
	if len(rfc3339) >= 10 {
		return rfc3339[:10]
	}
	return rfc3339
}

func truncateName(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func truncateID(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// For IDs, show beginning and end with ellipsis in middle.
	half := (maxLen - 3) / 2
	return s[:half] + "..." + s[len(s)-half:]
}

// cmdUserAgents lists unique user agent strings from stored activities,
// grouped by the client name returned by detectClient. Useful for discovering
// new client identifiers (e.g., Claude for Chrome, Excel, PowerPoint).
func cmdUserAgents(args []string) {
	fs := flag.NewFlagSet("user-agents", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	userFilter := fs.String("user", "", "Filter by user email")
	jsonFlag := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()

	uaCounts, err := db.UserAgents(*userFilter)
	if err != nil {
		fatal("querying user agents: %v", err)
	}

	if len(uaCounts) == 0 {
		fmt.Fprintln(os.Stderr, "No user agent strings found.")
		return
	}

	if *jsonFlag {
		type jsonEntry struct {
			UserAgent string `json:"user_agent"`
			Client    string `json:"client"`
			Count     int    `json:"count"`
		}
		var entries []jsonEntry
		for _, uc := range uaCounts {
			entries = append(entries, jsonEntry{
				UserAgent: uc.UserAgent,
				Client:    detectClient(uc.UserAgent),
				Count:     uc.Count,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			fatal("encoding JSON: %v", err)
		}
		return
	}

	// Group by detected client name for readability.
	type uaEntry struct {
		ua    string
		count int
	}
	grouped := make(map[string][]uaEntry)
	for _, uc := range uaCounts {
		client := detectClient(uc.UserAgent)
		grouped[client] = append(grouped[client], uaEntry{
			ua: uc.UserAgent, count: uc.Count,
		})
	}

	// Sort client groups by total count descending.
	type clientGroup struct {
		name    string
		entries []uaEntry
		total   int
	}
	var groups []clientGroup
	for name, entries := range grouped {
		total := 0
		for _, e := range entries {
			total += e.count
		}
		groups = append(groups, clientGroup{
			name: name, entries: entries, total: total,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].total > groups[j].total
	})

	for _, g := range groups {
		fmt.Printf("=== %s (%d activities) ===\n", g.name, g.total)
		for _, e := range g.entries {
			// Truncate long user agents for display.
			ua := e.ua
			if len(ua) > 120 {
				ua = ua[:117] + "..."
			}
			fmt.Printf("  %8d  %s\n", e.count, ua)
		}
		fmt.Println()
	}

	fmt.Fprintf(os.Stderr,
		"%d unique user agent strings across %d detected clients\n",
		len(uaCounts), len(groups))
}

// reclaimProfile aggregates all data sources into a single reclamation
// assessment for one licensed user.
type reclaimProfile struct {
	Email            string   `json:"email"`
	Tier             string   `json:"tier"`
	Score            int      `json:"score"`
	LastSeenAny      string   `json:"last_seen_any"`
	DaysSinceAny     int      `json:"days_since_any"`
	ComplianceEvents int      `json:"compliance_events"`
	ComplianceChats  int      `json:"compliance_chats"`
	ComplianceDays   int      `json:"compliance_days_active"`
	AnalyticsConv    int      `json:"analytics_conversations"`
	AnalyticsMsgs    int      `json:"analytics_messages"`
	CCSessions       int      `json:"cc_sessions"`
	CCCommits        int      `json:"cc_commits"`
	Connectors       int      `json:"connectors_used"`
	HasIntegration   bool     `json:"has_integration"`
	CSVEvents        int      `json:"csv_events,omitempty"`
	OktaSSOEvents    int      `json:"okta_sso_events,omitempty"`
	OktaLastSSO      string   `json:"okta_last_sso,omitempty"`
	DaysSinceOktaSSO int      `json:"days_since_okta_sso,omitempty"`
	LicenseDays      int      `json:"license_days"`
	Reasons          []string `json:"reasons"`
}

// reclaimScore computes a 0-100 safety score where higher means safer to
// reclaim. It also populates the Reasons slice with human-readable
// explanations of each scoring component.
func reclaimScore(p *reclaimProfile, gracePeriodDays, sessionDays int) {
	score := 0
	var reasons []string

	// Recency (0-40): how long since any activity across all sources.
	switch {
	case p.DaysSinceAny < 0:
		score += 40
		reasons = append(reasons, "never active in any source (+40)")
	case p.DaysSinceAny > 60:
		score += 40
		reasons = append(reasons,
			fmt.Sprintf("last seen %d days ago across all sources (+40)",
				p.DaysSinceAny))
	case p.DaysSinceAny > 30:
		score += 30
		reasons = append(reasons,
			fmt.Sprintf("last seen %d days ago (+30)", p.DaysSinceAny))
	case p.DaysSinceAny > 14:
		score += 15
		reasons = append(reasons,
			fmt.Sprintf("last seen %d days ago (+15)", p.DaysSinceAny))
	case p.DaysSinceAny > 7:
		score += 5
		reasons = append(reasons,
			fmt.Sprintf("last seen %d days ago (+5)", p.DaysSinceAny))
	default:
		reasons = append(reasons, "active within last 7 days (+0)")
	}

	// Inactivity (0-25): how much activity exists across all sources.
	totalActivity := p.ComplianceEvents + p.AnalyticsConv +
		p.AnalyticsMsgs + p.CCSessions + p.CCCommits + p.Connectors +
		p.OktaSSOEvents
	switch {
	case totalActivity == 0 && p.CSVEvents == 0:
		score += 25
		reasons = append(reasons, "zero activity across all sources (+25)")
	case p.ComplianceChats == 0 && p.AnalyticsConv == 0 &&
		p.CCSessions == 0 && p.Connectors == 0 && p.OktaSSOEvents > 0:
		score += 5
		reasons = append(reasons,
			fmt.Sprintf(
				"no visible activity but %d successful Claude SSO logins (+5)",
				p.OktaSSOEvents))
	case p.ComplianceChats == 0 && p.AnalyticsConv == 0 &&
		p.CCSessions == 0 && p.Connectors == 0 && p.OktaSSOEvents == 0:
		score += 20
		reasons = append(reasons,
			"no chats, conversations, code, or connectors (+20)")
	case p.ComplianceDays <= 2 && p.ComplianceChats <= 3 &&
		p.AnalyticsConv == 0 && p.CCSessions == 0:
		score += 15
		reasons = append(reasons,
			"barely tried with no analytics activity (+15)")
	case totalActivity > 0 && totalActivity <= 20:
		score += 5
		reasons = append(reasons, "low total activity (+5)")
	default:
		reasons = append(reasons,
			fmt.Sprintf("substantial activity (%d total events) (+0)",
				totalActivity))
	}

	// Shadow channels (0-20): penalize hidden activity that the
	// Compliance API cannot see.
	shadowScore := 20
	if p.CCSessions > 0 || p.CCCommits > 0 {
		shadowScore -= 20
		reasons = append(reasons,
			fmt.Sprintf("Claude Code: %d sessions, %d commits (-20 shadow)",
				p.CCSessions, p.CCCommits))
	}
	if p.Connectors > 0 {
		shadowScore -= 15
		reasons = append(reasons,
			fmt.Sprintf("connectors/MCP: %d invocations — possible Cowork (-15 shadow)",
				p.Connectors))
	}
	if shadowScore < 0 {
		shadowScore = 0
	}
	if shadowScore == 20 {
		reasons = append(reasons,
			"no shadow channel activity detected (+20)")
	}
	score += shadowScore

	// Integrations (0-10): penalize active integrations whose ongoing
	// usage generates no compliance events.
	if p.HasIntegration {
		reasons = append(reasons,
			"active integration (GitHub/GDrive) — ongoing use invisible (+0)")
	} else {
		score += 10
		reasons = append(reasons, "no active integrations (+10)")
	}

	// Account age (0-5): grace period for new accounts.
	switch {
	case p.LicenseDays > 30:
		score += 5
		reasons = append(reasons, "account >30 days old (+5)")
	case p.LicenseDays > 14:
		score += 2
		reasons = append(reasons,
			fmt.Sprintf("account %d days old (+2)", p.LicenseDays))
	default:
		reasons = append(reasons,
			fmt.Sprintf("new account (%d days) — grace period (+0)",
				p.LicenseDays))
	}

	p.Score = score
	p.Reasons = reasons

	switch {
	case score >= 75:
		p.Tier = "SAFE"
	case score >= 40:
		p.Tier = "INVESTIGATE"
	default:
		p.Tier = "DO NOT RECLAIM"
	}

	// Hard override: recently active users are never reclaimable
	// regardless of their composite score.
	if p.DaysSinceAny >= 0 && p.DaysSinceAny <= 7 {
		p.Tier = "DO NOT RECLAIM"
		p.Reasons = append(p.Reasons,
			fmt.Sprintf("OVERRIDE: active %d days ago — not reclaimable",
				p.DaysSinceAny))
	}

	// Okta session override: a successful Claude SSO within the
	// configurable session window means the user may hold an active
	// web session. Disabled when sessionDays is 0 (unlimited sessions)
	// or <=7 (already covered by the universal override above).
	if sessionDays > 7 &&
		p.DaysSinceOktaSSO >= 0 && p.DaysSinceOktaSSO <= sessionDays {
		p.Tier = "DO NOT RECLAIM"
		p.Reasons = append(p.Reasons,
			fmt.Sprintf(
				"OVERRIDE: Okta Claude SSO %d days ago (within %d-day session window) — not reclaimable",
				p.DaysSinceOktaSSO, sessionDays))
	}

	// Grace period override: newly provisioned accounts with some
	// activity get a grace period to onboard. If the user has zero
	// activity across every source, there is nothing to protect and
	// the seat is safe to reclaim regardless of account age.
	if p.LicenseDays <= gracePeriodDays && p.Tier == "SAFE" &&
		(totalActivity > 0 || p.CSVEvents > 0) {
		p.Tier = "INVESTIGATE"
		p.Reasons = append(p.Reasons,
			fmt.Sprintf("OVERRIDE: account only %d days old with some activity — onboarding grace period",
				p.LicenseDays))
	}
}

// reclaimData holds all inputs needed to build reclaim profiles. This
// is separate from reclaimOptions to make the profile-building logic
// testable without a live store or CLI flags.
type reclaimData struct {
	rank                rankInputs
	analyticsLastActive map[string]string
	activeIntegrations  map[string]bool
	csvSummaries        map[string]*csvaudit.UserSummary
	oktaSummaries       map[string]store.OktaSSOSummary
	now                 time.Time
	graceDays           int
	sessionDays         int
}

// buildReclaimProfiles constructs and scores a reclaimProfile for each
// licensed user by merging all available data sources.
func buildReclaimProfiles(rd reclaimData) []reclaimProfile {
	var profiles []reclaimProfile

	for _, u := range rd.rank.users {
		cs := rd.rank.summaryMap[u.Email]
		as := rd.rank.analyticsMap[u.Email]
		csvSu := rd.csvSummaries[u.Email]

		licDays := 0
		if ct, ok := rd.rank.userCreated[u.Email]; ok {
			licDays = int(rd.now.Sub(ct).Hours()/24) + 1
		}

		var lastSeen time.Time
		if cs.LastSeen != "" {
			if t, err := time.Parse(
				time.RFC3339Nano, cs.LastSeen,
			); err == nil && t.After(lastSeen) {
				lastSeen = t
			}
		}
		if d, ok := rd.analyticsLastActive[u.Email]; ok {
			if t, err := time.Parse("2006-01-02", d); err == nil {
				if t.After(lastSeen) {
					lastSeen = t
				}
			}
		}

		var csvEvents int
		if csvSu != nil {
			csvEvents = csvSu.EventCount
			if !csvSu.LastSeen.IsZero() && csvSu.LastSeen.After(lastSeen) {
				lastSeen = csvSu.LastSeen
			}
		}

		var oktaSSOEvents int
		var oktaLastSSOStr string
		daysSinceOktaSSO := -1
		if oktaSu, ok := rd.oktaSummaries[u.Email]; ok {
			oktaSSOEvents = oktaSu.EventCount
			oktaLastSSOStr = oktaSu.LastSSO
			if t, err := time.Parse(
				time.RFC3339, oktaSu.LastSSO,
			); err == nil {
				daysSinceOktaSSO = int(rd.now.Sub(t).Hours() / 24)
				if t.After(lastSeen) {
					lastSeen = t
				}
			}
		}

		daysSince := -1
		lastStr := "never"
		if !lastSeen.IsZero() {
			daysSince = int(rd.now.Sub(lastSeen).Hours() / 24)
			lastStr = lastSeen.Format("2006-01-02")
		}

		p := reclaimProfile{
			Email:            u.Email,
			LastSeenAny:      lastStr,
			DaysSinceAny:     daysSince,
			ComplianceEvents: cs.EventCount,
			ComplianceChats:  cs.ChatsCreated,
			ComplianceDays:   cs.ActiveDays,
			AnalyticsConv:    as.Conversations,
			AnalyticsMsgs:    as.Messages,
			CCSessions:       as.CCSessions,
			CCCommits:        as.CCCommits,
			Connectors:       as.ConnectorsUsed,
			HasIntegration:   rd.activeIntegrations[u.Email],
			CSVEvents:        csvEvents,
			OktaSSOEvents:    oktaSSOEvents,
			OktaLastSSO:      oktaLastSSOStr,
			DaysSinceOktaSSO: daysSinceOktaSSO,
			LicenseDays:      licDays,
		}
		reclaimScore(&p, rd.graceDays, rd.sessionDays)
		profiles = append(profiles, p)
	}
	return profiles
}

// reclaimOptions holds flags specific to reclaim mode.
type reclaimOptions struct {
	csvPath    string
	graceDays  int
	tierFilter string
	jsonOutput bool
	days       int
	oktaFlag   bool
	oktaAPIKey string
}

// rankRunReclaim runs seat-reclamation safety analysis using shared rank
// inputs plus reclaim-specific data (last-active dates, integrations, CSV).
func rankRunReclaim(
	db *store.Store, in rankInputs, opts reclaimOptions,
) {
	now := time.Now().UTC()
	endDate := now.AddDate(0, 0, -3)
	startDate := endDate.AddDate(0, 0, -opts.days)
	sinceStr := startDate.Format("2006-01-02")
	untilStr := endDate.Format("2006-01-02")

	analyticsLastActive, err := db.AnalyticsLastActiveDates(
		sinceStr, untilStr,
	)
	if err != nil {
		fatal("loading analytics last-active dates: %v", err)
	}

	activeIntegrations, err := db.UsersWithActiveIntegrations()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: could not check active integrations: %v\n", err)
		activeIntegrations = make(map[string]bool)
	}

	csvSummaries := make(map[string]*csvaudit.UserSummary)
	if opts.csvPath != "" {
		records, err := csvaudit.ParseCSV(opts.csvPath)
		if err != nil {
			fatal("parsing CSV: %v", err)
		}
		csvSummaries = csvaudit.SummarizeByUser(records)
		fmt.Fprintf(os.Stderr,
			"Loaded %d users from CSV export\n", len(csvSummaries))
	}

	oktaSummaries := make(map[string]store.OktaSSOSummary)
	if opts.oktaFlag {
		oktaClient, err := buildOktaClient(opts.oktaAPIKey)
		if err != nil {
			fatal("creating Okta client: %v", err)
		}

		// Okta window is independent of the analytics 3-day lag.
		// The half-open interval [start, until) must span exactly
		// opts.days calendar days so --days 90 stays within Okta's
		// 90-day retention. Anchor start to until, not to today.
		today := time.Date(
			now.Year(), now.Month(), now.Day(),
			0, 0, 0, 0, time.UTC,
		)
		oktaUntil := today.AddDate(0, 0, 1)
		oktaStart := oktaUntil.AddDate(0, 0, -opts.days)

		fmt.Fprintf(os.Stderr,
			"Fetching Okta SSO events %s to %s...\n",
			oktaStart.Format("2006-01-02"),
			today.Format("2006-01-02"))

		events, err := oktaClient.FetchClaudeSSOEvents(
			context.Background(),
			oktaStart,
			oktaUntil,
			okta.DefaultClaudeAppID(),
			okta.DefaultClaudeAppName(),
		)
		if err != nil {
			fatal("fetching Okta SSO events: %v", err)
		}

		n, err := db.InsertOktaSSOEvents(events, time.Now().UTC())
		if err != nil {
			fatal("storing Okta SSO events: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Cached %d Okta Claude SSO events\n", n)

		oktaSummaries, err = db.OktaSSOSummaries(oktaStart, oktaUntil)
		if err != nil {
			fatal("loading Okta SSO summaries: %v", err)
		}
		sessionDays := okta.DefaultSessionDurationDays()
		fmt.Fprintf(os.Stderr,
			"Okta SSO: %d users with successful Claude authentication\n",
			len(oktaSummaries))
		if sessionDays > 0 {
			fmt.Fprintf(os.Stderr,
				"Okta evidence covers at most the last 90 days; "+
					"Claude session duration assumed: %d days\n",
				sessionDays)
		} else {
			fmt.Fprintf(os.Stderr,
				"Okta evidence covers at most the last 90 days; "+
					"Claude sessions are unlimited (no session-based override)\n")
		}
	}

	// Session duration is only configurable when --okta is active.
	// Default is 0 (unlimited) since Claude sessions no longer expire.
	sessionDays := 0
	if opts.oktaFlag {
		sessionDays = okta.DefaultSessionDurationDays()
	}

	rd := reclaimData{
		rank:                in,
		analyticsLastActive: analyticsLastActive,
		activeIntegrations:  activeIntegrations,
		csvSummaries:        csvSummaries,
		oktaSummaries:       oktaSummaries,
		now:                 now,
		graceDays:           opts.graceDays,
		sessionDays:         sessionDays,
	}
	profiles := buildReclaimProfiles(rd)

	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Score != profiles[j].Score {
			return profiles[i].Score > profiles[j].Score
		}
		return profiles[i].Email < profiles[j].Email
	})

	if opts.tierFilter != "" {
		target := strings.ToUpper(opts.tierFilter)
		if target == "DNR" {
			target = "DO NOT RECLAIM"
		}
		var filtered []reclaimProfile
		for _, p := range profiles {
			if p.Tier == target {
				filtered = append(filtered, p)
			}
		}
		profiles = filtered
	}

	if opts.jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(profiles); err != nil {
			fatal("encoding JSON: %v", err)
		}
		return
	}

	if opts.oktaFlag {
		fmt.Printf(
			"%-5s %-40s %-16s %5s %-10s %6s %6s %5s %5s %4s %4s %4s %-10s %s\n",
			"Rank", "Email", "Tier", "Score", "Last Seen",
			"CmplEv", "AnConv", "AnMsg", "CC", "Conn", "Intg",
			"SSO", "LastSSO", "Top Reason")
		fmt.Println(strings.Repeat("\u2500", 160))
	} else {
		fmt.Printf(
			"%-5s %-40s %-16s %5s %-10s %6s %6s %5s %5s %4s %4s %s\n",
			"Rank", "Email", "Tier", "Score", "Last Seen",
			"CmplEv", "AnConv", "AnMsg", "CC", "Conn", "Intg",
			"Top Reason")
		fmt.Println(strings.Repeat("\u2500", 140))
	}

	tierCounts := make(map[string]int)
	for i, p := range profiles {
		intgStr := ""
		if p.HasIntegration {
			intgStr = "yes"
		}
		topReason := ""
		if len(p.Reasons) > 0 {
			topReason = p.Reasons[0]
		}
		if opts.oktaFlag {
			lastSSO := ""
			if len(p.OktaLastSSO) >= 10 {
				lastSSO = p.OktaLastSSO[:10]
			}
			fmt.Printf(
				"%-5d %-40s %-16s %5d %-10s %6d %6d %5d %5d %4d %-4s %4d %-10s %s\n",
				i+1, p.Email, p.Tier, p.Score, p.LastSeenAny,
				p.ComplianceEvents, p.AnalyticsConv, p.AnalyticsMsgs,
				p.CCSessions, p.Connectors, intgStr,
				p.OktaSSOEvents, lastSSO, topReason)
		} else {
			fmt.Printf(
				"%-5d %-40s %-16s %5d %-10s %6d %6d %5d %5d %4d %-4s %s\n",
				i+1, p.Email, p.Tier, p.Score, p.LastSeenAny,
				p.ComplianceEvents, p.AnalyticsConv, p.AnalyticsMsgs,
				p.CCSessions, p.Connectors, intgStr, topReason)
		}
		tierCounts[p.Tier]++
	}

	fmt.Fprintf(os.Stderr,
		"\nTotal: %d licensed users — %d safe, %d investigate, %d do not reclaim\n",
		len(profiles),
		tierCounts["SAFE"],
		tierCounts["INVESTIGATE"],
		tierCounts["DO NOT RECLAIM"])

	if opts.csvPath == "" {
		fmt.Fprintf(os.Stderr,
			"Tip: pass --csv <export.zip> to cross-reference with audit log export\n")
	}
	if !opts.oktaFlag {
		fmt.Fprintf(os.Stderr,
			"Tip: pass --okta to cross-reference with Okta SSO login data\n")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
