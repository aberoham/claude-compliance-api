package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
)

// cmdCompliance exposes the Compliance API operations that do not belong to
// the existing cached activity/chat/project workflows.
func cmdCompliance(args []string) {
	if len(args) < 2 {
		printComplianceUsage()
		return
	}
	switch args[0] {
	case "list":
		cmdComplianceList(args[1], args[2:])
	case "get":
		cmdComplianceGet(args[1], args[2:])
	case "download":
		cmdComplianceDownload(args[1], args[2:])
	case "delete":
		cmdComplianceDelete(args[1], args[2:])
	default:
		fatal("unknown compliance action %q (use list, get, download, or delete)", args[0])
	}
}

type complianceCommonFlags struct {
	apiKey *string
	orgID  *string
}

func addComplianceCommonFlags(fs *flag.FlagSet) complianceCommonFlags {
	return complianceCommonFlags{
		apiKey: fs.String("api-key", "", "Compliance API key (if unset, reads from 1Password)"),
		orgID:  fs.String("org", compliance.DefaultOrgID(), "Default organization UUID/ID"),
	}
}

func (f complianceCommonFlags) client() *compliance.Client {
	if *f.apiKey != "" {
		return compliance.NewClient(*f.apiKey, *f.orgID)
	}
	client, err := compliance.NewClientFrom1Password("", "", *f.orgID)
	if err != nil {
		fatal("creating Compliance API client: %v", err)
	}
	return client
}

func cmdComplianceList(resource string, args []string) {
	fs := flag.NewFlagSet("compliance list "+resource, flag.ExitOnError)
	common := addComplianceCommonFlags(fs)
	limit := fs.Int("limit", 0, "Per-page result limit (all pages are fetched)")
	namePrefix := fs.String("name-prefix", "", "Filter group names by prefix")
	var organizationIDs stringListFlag
	var userIDs stringListFlag
	fs.Var(&organizationIDs, "organization-id", "Filter Code Artifacts by organization (repeatable)")
	fs.Var(&userIDs, "user-id", "Filter Code Artifacts by owner user (repeatable)")
	updatedGTE := fs.String("updated-at-gte", "", "Filter Code Artifacts updated at/after RFC3339 time")
	updatedLT := fs.String("updated-at-lt", "", "Filter Code Artifacts updated before RFC3339 time")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}
	client := common.client()
	ctx := context.Background()
	positionals := fs.Args()
	var result any
	var err error

	switch resource {
	case "organizations":
		requireArgs(resource, positionals, 0)
		result, err = client.FetchOrganizations(ctx, compliance.ListOptions{Limit: *limit})
	case "users":
		orgUUID := positionalOrDefault(positionals, *common.orgID, "organization UUID")
		result, err = client.FetchOrganizationUsers(ctx, orgUUID, compliance.ListOptions{Limit: *limit})
	case "roles":
		orgUUID := positionalOrDefault(positionals, *common.orgID, "organization UUID")
		result, err = client.FetchRoles(ctx, orgUUID, compliance.ListOptions{Limit: *limit})
	case "role-permissions":
		requireArgs(resource, positionals, 2)
		result, err = client.FetchRolePermissions(ctx, positionals[0], positionals[1], compliance.ListOptions{Limit: *limit})
	case "groups":
		requireArgs(resource, positionals, 0)
		result, err = client.FetchGroups(ctx, compliance.GroupQuery{Limit: *limit, NamePrefix: *namePrefix})
	case "group-members":
		requireArgs(resource, positionals, 1)
		result, err = client.FetchGroupMembers(ctx, positionals[0], compliance.ListOptions{Limit: *limit})
	case "project-attachments":
		requireArgs(resource, positionals, 1)
		result, err = client.FetchProjectAttachments(ctx, positionals[0], compliance.ListOptions{Limit: *limit})
	case "project-collaborators":
		requireArgs(resource, positionals, 1)
		result, err = client.FetchProjectCollaborators(ctx, positionals[0], compliance.ListOptions{Limit: *limit})
	case "code-artifacts":
		requireArgs(resource, positionals, 0)
		gte := parseOptionalRFC3339(*updatedGTE, "--updated-at-gte")
		lt := parseOptionalRFC3339(*updatedLT, "--updated-at-lt")
		result, err = client.FetchCodeArtifacts(ctx, compliance.CodeArtifactQuery{
			Limit:           *limit,
			OrganizationIDs: organizationIDs,
			UserIDs:         userIDs,
			UpdatedAtGTE:    gte,
			UpdatedAtLT:     lt,
		})
	default:
		fatal("unknown compliance list resource %q", resource)
	}
	if err != nil {
		fatal("listing %s: %v", resource, err)
	}
	writeComplianceJSON(result)
}

func cmdComplianceGet(resource string, args []string) {
	fs := flag.NewFlagSet("compliance get "+resource, flag.ExitOnError)
	common := addComplianceCommonFlags(fs)
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}
	client := common.client()
	ctx := context.Background()
	positionals := fs.Args()
	var result any
	var err error

	switch resource {
	case "role":
		requireArgs(resource, positionals, 2)
		result, err = client.GetRole(ctx, positionals[0], positionals[1])
	case "group":
		requireArgs(resource, positionals, 1)
		result, err = client.GetGroup(ctx, positionals[0])
	case "settings":
		organizationID := positionalOrDefault(positionals, *common.orgID, "organization UUID")
		result, err = client.GetOrganizationSettings(ctx, organizationID)
	case "file":
		requireArgs(resource, positionals, 1)
		result, err = client.GetFileMetadata(ctx, positionals[0])
	case "generated-file":
		requireArgs(resource, positionals, 1)
		result, err = client.GetGeneratedFileMetadata(ctx, positionals[0])
	case "artifact":
		requireArgs(resource, positionals, 1)
		result, err = client.GetChatArtifactMetadata(ctx, positionals[0])
	case "project-document":
		requireArgs(resource, positionals, 1)
		result, err = client.GetProjectDocument(ctx, positionals[0])
	case "project-document-metadata":
		requireArgs(resource, positionals, 1)
		result, err = client.GetProjectDocumentMetadata(ctx, positionals[0])
	default:
		fatal("unknown compliance get resource %q", resource)
	}
	if err != nil {
		fatal("getting %s: %v", resource, err)
	}
	writeComplianceJSON(result)
}

func cmdComplianceDownload(resource string, args []string) {
	fs := flag.NewFlagSet("compliance download "+resource, flag.ExitOnError)
	common := addComplianceCommonFlags(fs)
	output := fs.String("output", "", "Output path (default: server filename or resource ID; '-' for stdout)")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}
	client := common.client()
	ctx := context.Background()
	positionals := fs.Args()
	var download *compliance.Download
	var err error

	switch resource {
	case "file":
		requireArgs(resource, positionals, 1)
		download, err = client.DownloadFileContent(ctx, positionals[0])
	case "generated-file":
		requireArgs(resource, positionals, 1)
		download, err = client.DownloadGeneratedFile(ctx, positionals[0])
	case "artifact":
		requireArgs(resource, positionals, 1)
		download, err = client.DownloadChatArtifact(ctx, positionals[0])
	case "code-artifact":
		requireArgs(resource, positionals, 2)
		download, err = client.DownloadCodeArtifactVersion(ctx, positionals[0], positionals[1])
	default:
		fatal("unknown compliance download resource %q", resource)
	}
	if err != nil {
		fatal("downloading %s: %v", resource, err)
	}
	defer download.Body.Close()
	writeComplianceDownload(download, *output)
}

func cmdComplianceDelete(resource string, args []string) {
	fs := flag.NewFlagSet("compliance delete "+resource, flag.ExitOnError)
	common := addComplianceCommonFlags(fs)
	yes := fs.Bool("yes", false, "Permanently delete without interactive confirmation")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}
	positionals := fs.Args()
	requireArgs(resource, positionals, 1)
	id := positionals[0]
	confirmComplianceDeletion(resource, id, *yes)
	client := common.client()
	ctx := context.Background()
	var result *compliance.DeletionReceipt
	var err error

	switch resource {
	case "chat":
		result, err = client.DeleteChat(ctx, id)
	case "file":
		result, err = client.DeleteFile(ctx, id)
	case "project":
		result, err = client.DeleteProject(ctx, id)
	case "project-document":
		result, err = client.DeleteProjectDocument(ctx, id)
	case "code-artifact":
		result, err = client.DeleteCodeArtifact(ctx, id)
	default:
		fatal("unknown compliance delete resource %q", resource)
	}
	if err != nil {
		fatal("deleting %s: %v", resource, err)
	}
	writeComplianceJSON(result)
}

func writeComplianceJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fatal("encoding JSON: %v", err)
	}
}

func writeComplianceDownload(download *compliance.Download, requestedPath string) {
	if requestedPath == "-" {
		if _, err := io.Copy(os.Stdout, download.Body); err != nil {
			fatal("writing download: %v", err)
		}
		return
	}
	outputPath := requestedPath
	if outputPath == "" {
		outputPath = filepath.Base(download.Filename)
	}
	out, err := os.Create(outputPath)
	if err != nil {
		fatal("creating output file: %v", err)
	}
	n, copyErr := io.Copy(out, download.Body)
	closeErr := out.Close()
	if copyErr != nil {
		fatal("writing download: %v", copyErr)
	}
	if closeErr != nil {
		fatal("closing output file: %v", closeErr)
	}
	fmt.Fprintf(os.Stderr, "Downloaded %s (%d bytes)\n", outputPath, n)
}

func confirmComplianceDeletion(resource, id string, yes bool) {
	if yes {
		return
	}
	fmt.Fprintf(os.Stderr, "WARNING: this permanently deletes %s %s.\nType the resource ID to continue: ", resource, id)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		fatal("reading confirmation: %v", err)
	}
	if strings.TrimSpace(line) != id {
		fatal("aborted: confirmation did not match resource ID")
	}
}

func requireArgs(resource string, args []string, count int) {
	if len(args) != count {
		fatal("%s expects %d positional argument(s), got %d", resource, count, len(args))
	}
}

func positionalOrDefault(args []string, fallback, label string) string {
	if len(args) > 1 {
		fatal("expected at most one %s", label)
	}
	if len(args) == 1 {
		return args[0]
	}
	if fallback == "" {
		fatal("%s is required (pass it positionally or with --org)", label)
	}
	return fallback
}

func parseOptionalRFC3339(value, flagName string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		fatal("invalid %s: %v", flagName, err)
	}
	return &parsed
}

type stringListFlag []string

func (s *stringListFlag) String() string { return strings.Join(*s, ",") }
func (s *stringListFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func printComplianceUsage() {
	fmt.Fprint(os.Stderr, `Usage: audit compliance <action> <resource> [flags] [ids]

List:
  list organizations
  list users [organization-uuid]
  list roles [organization-uuid]
  list role-permissions <organization-uuid> <role-id>
  list groups [--name-prefix PREFIX]
  list group-members <group-id>
  list project-attachments <project-id>
  list project-collaborators <project-id>
  list code-artifacts [--organization-id ID] [--user-id ID]

Get:
  get role <organization-uuid> <role-id>
  get group <group-id>
  get settings [organization-uuid]
  get file <file-id>
  get generated-file <generated-file-id>
  get artifact <artifact-version-id>
  get project-document <document-id>
  get project-document-metadata <document-id>

Download:
  download file <file-id>
  download generated-file <generated-file-id>
  download artifact <artifact-version-id>
  download code-artifact <artifact-id> <version-id>

Delete (permanent; interactive unless --yes is supplied):
  delete chat|file|project|project-document|code-artifact <id>

Flags must precede positional IDs. Metadata and list results are emitted as JSON.
`)
}
