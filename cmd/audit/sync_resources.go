package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/aberoham/claude-compliance-api/cachesync"
	"github.com/aberoham/claude-compliance-api/compliance"
	"github.com/aberoham/claude-compliance-api/contentstore"
	"github.com/aberoham/claude-compliance-api/store"
)

func cmdSyncResources(args []string) {
	fs := flag.NewFlagSet("sync-resources", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "Path to SQLite database")
	orgID := fs.String("org", compliance.DefaultOrgID(), "Default organization UUID/ID")
	apiKey := fs.String("api-key", "", "Compliance API key (if unset, reads from 1Password)")
	content := fs.String("content", "none", "Content ingestion: none, text, or all")
	objectsDir := fs.String("objects-dir", "", "Local content-addressed object directory (default: beside database)")
	maxContentBytes := fs.Int64("max-content-bytes", 0, "Reject an individual download larger than this many bytes (0 = unlimited)")
	refreshContent := fs.Bool("refresh-content", false, "Re-download content already linked in SQLite")
	skipChats := fs.Bool("skip-chats", false, "Skip chat transcripts and their file/artifact discovery")
	statusOnly := fs.Bool("status", false, "Show cached resource counts without contacting the API")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}
	if *maxContentBytes < 0 {
		fatal("--max-content-bytes cannot be negative")
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fatal("opening database: %v", err)
	}
	defer db.Close()
	if *statusOnly {
		printResourceCoverage(db)
		return
	}

	client, err := buildResourceSyncClient(*apiKey, *orgID)
	if err != nil {
		fatal("creating Compliance API client: %v", err)
	}
	var objects contentstore.Store
	if cachesync.ContentMode(*content) == cachesync.ContentAll {
		root := *objectsDir
		if root == "" {
			root = filepath.Join(filepath.Dir(*dbPath), "objects")
		}
		local, err := contentstore.NewLocal(root)
		if err != nil {
			fatal("opening content store: %v", err)
		}
		objects = local
		fmt.Fprintf(os.Stderr, "Objects: %s\n", root)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(os.Stderr, "Database: %s\n", db.Path())
	syncer := cachesync.Syncer{
		Client:  client,
		DB:      db,
		Objects: objects,
		Options: cachesync.Options{
			Content:         cachesync.ContentMode(*content),
			RefreshContent:  *refreshContent,
			MaxContentBytes: *maxContentBytes,
			SkipChats:       *skipChats,
			Report:          func(format string, values ...any) { fmt.Fprintf(os.Stderr, "  "+format+"\n", values...) },
		},
	}
	if err := syncer.Run(ctx); err != nil {
		fatal("syncing Compliance resources: %v", err)
	}
	fmt.Fprintln(os.Stderr, "Resource sync complete.")
	printResourceCoverage(db)
}

func buildResourceSyncClient(apiKey, orgID string) (*compliance.Client, error) {
	if apiKey != "" {
		return compliance.NewClient(apiKey, orgID), nil
	}
	return compliance.NewClientFrom1Password("", "", orgID)
}

func printResourceCoverage(db *store.Store) {
	coverage, err := db.ResourceCoverage()
	if err != nil {
		fatal("reading resource coverage: %v", err)
	}
	fmt.Fprintln(os.Stdout, "Resource cache coverage:")
	for _, item := range coverage {
		fmt.Fprintf(os.Stdout, "  %-28s %8d\n", item.Resource, item.Count)
	}
}
