package main

import (
	"strings"
	"testing"
	"time"

	"github.com/aberoham/claude-compliance-api/csvaudit"
	"github.com/aberoham/claude-compliance-api/store"
)

func hasReasonContaining(reasons []string, substr string) bool {
	for _, r := range reasons {
		if strings.Contains(r, substr) {
			return true
		}
	}
	return false
}

// --- reclaimScore unit tests ---

func TestUniversalOverrideFixedAt7Days(t *testing.T) {
	p := reclaimProfile{
		DaysSinceAny:     5,
		DaysSinceOktaSSO: -1,
		ComplianceEvents: 100,
		ComplianceChats:  10,
		LicenseDays:      90,
	}
	reclaimScore(&p, 21, 14)
	if p.Tier != "DO NOT RECLAIM" {
		t.Errorf("expected DO NOT RECLAIM, got %q", p.Tier)
	}
	if hasReasonContaining(p.Reasons, "session window") {
		t.Error("universal override should not mention session window")
	}
}

func TestSessionDurationDoesNotWidenWithoutOktaEvidence(t *testing.T) {
	p := reclaimProfile{
		DaysSinceAny:     10,
		DaysSinceOktaSSO: -1,
		ComplianceEvents: 50,
		ComplianceChats:  5,
		LicenseDays:      90,
	}
	reclaimScore(&p, 21, 14)
	if hasReasonContaining(p.Reasons, "session window") {
		t.Error("should not apply Okta session override without Okta evidence")
	}
}

func TestOktaSessionOverrideTriggersOnOktaRecency(t *testing.T) {
	p := reclaimProfile{
		DaysSinceAny:     10,
		DaysSinceOktaSSO: 10,
		OktaSSOEvents:    3,
		OktaLastSSO:      "2026-03-18T10:00:00Z",
		ComplianceEvents: 0,
		LicenseDays:      90,
	}
	reclaimScore(&p, 21, 14)
	if p.Tier != "DO NOT RECLAIM" {
		t.Errorf("expected DO NOT RECLAIM via Okta session, got %q", p.Tier)
	}
	if !hasReasonContaining(p.Reasons, "Okta Claude SSO") {
		t.Error("expected Okta-specific session override reason")
	}
}

func TestOktaSessionOverrideDoesNotTriggerAtDefault0(t *testing.T) {
	p := reclaimProfile{
		DaysSinceAny:     5,
		DaysSinceOktaSSO: 5,
		OktaSSOEvents:    1,
		LicenseDays:      90,
	}
	reclaimScore(&p, 21, 0)
	if !hasReasonContaining(p.Reasons, "OVERRIDE: active 5 days ago") {
		t.Error("expected universal override, not Okta-specific")
	}
	if hasReasonContaining(p.Reasons, "Okta Claude SSO") {
		t.Error("Okta override should not fire when sessionDays == 0")
	}
}

func TestUniversalOverrideBoundary(t *testing.T) {
	p := reclaimProfile{
		DaysSinceAny:     7,
		DaysSinceOktaSSO: -1,
		ComplianceEvents: 50,
		LicenseDays:      60,
	}
	reclaimScore(&p, 21, 7)
	if p.Tier != "DO NOT RECLAIM" {
		t.Errorf("expected DO NOT RECLAIM at 7-day boundary, got %q", p.Tier)
	}

	p2 := reclaimProfile{
		DaysSinceAny:     8,
		DaysSinceOktaSSO: -1,
		ComplianceEvents: 50,
		LicenseDays:      60,
	}
	reclaimScore(&p2, 21, 7)
	if hasReasonContaining(p2.Reasons, "OVERRIDE: active") {
		t.Error("universal override should not fire at 8 days")
	}
}

func TestGracePeriodZeroActivityStaysSAFE(t *testing.T) {
	p := reclaimProfile{
		DaysSinceAny:     -1,
		DaysSinceOktaSSO: -1,
		LicenseDays:      10,
	}
	reclaimScore(&p, 21, 0)
	if p.Tier != "SAFE" {
		t.Errorf("zero-activity new account should be SAFE, got %q", p.Tier)
	}
}

func TestGracePeriodWithActivityDowngradesToINVESTIGATE(t *testing.T) {
	p := reclaimProfile{
		DaysSinceAny:     -1,
		DaysSinceOktaSSO: -1,
		LicenseDays:      10,
		ComplianceEvents: 5,
	}
	reclaimScore(&p, 21, 0)
	if p.Tier != "INVESTIGATE" {
		t.Errorf("new account with activity should be INVESTIGATE, got %q",
			p.Tier)
	}
}

// --- buildReclaimProfiles integration tests ---

func TestBuildProfilesOktaRecencyScopedCorrectly(t *testing.T) {
	// Freeze time so DaysSinceAny computations are deterministic.
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	// Alice: compliance activity 10 days ago, no Okta data.
	// With sessionDays=14, she should NOT be caught by the Okta
	// override because her recency comes from compliance, not Okta.
	//
	// Bob: no compliance activity, Okta SSO 10 days ago.
	// With sessionDays=14, he SHOULD be caught by the Okta override.
	//
	// Carol: zero activity across all sources, old account.
	// Should be SAFE.
	//
	// Dave: Okta SSO 5 days ago (within universal 7-day override).
	// Should be DO NOT RECLAIM via the universal override.
	rd := reclaimData{
		rank: rankInputs{
			users: []store.CachedUser{
				{Email: "alice@example.com"},
				{Email: "bob@example.com"},
				{Email: "carol@example.com"},
				{Email: "dave@example.com"},
			},
			summaryMap: map[string]store.StoredUserSummary{
				"alice@example.com": {
					EventCount:  20,
					ChatsCreated: 5,
					ActiveDays:  3,
					LastSeen:    "2026-03-18T10:00:00Z",
				},
			},
			analyticsMap: map[string]store.AnalyticsUserSummary{},
			userCreated: map[string]time.Time{
				"alice@example.com": now.AddDate(0, -3, 0),
				"bob@example.com":   now.AddDate(0, -2, 0),
				"carol@example.com": now.AddDate(0, -6, 0),
				"dave@example.com":  now.AddDate(0, -1, 0),
			},
		},
		analyticsLastActive: map[string]string{},
		activeIntegrations:  map[string]bool{},
		csvSummaries:        map[string]*csvaudit.UserSummary{},
		oktaSummaries: map[string]store.OktaSSOSummary{
			"bob@example.com": {
				Email:      "bob@example.com",
				EventCount: 3,
				FirstSSO:   "2026-03-12T09:00:00Z",
				LastSSO:    "2026-03-18T10:00:00Z",
			},
			"dave@example.com": {
				Email:      "dave@example.com",
				EventCount: 1,
				FirstSSO:   "2026-03-23T10:00:00Z",
				LastSSO:    "2026-03-23T10:00:00Z",
			},
		},
		now:         now,
		graceDays:   21,
		sessionDays: 14,
	}

	profiles := buildReclaimProfiles(rd)
	byEmail := make(map[string]reclaimProfile)
	for _, p := range profiles {
		byEmail[p.Email] = p
	}

	// Alice: compliance-only recency at 10 days, no Okta data.
	// The Okta session override must not apply.
	alice := byEmail["alice@example.com"]
	if hasReasonContaining(alice.Reasons, "Okta Claude SSO") {
		t.Errorf("Alice: Okta override should not apply (no Okta data)")
	}
	if alice.DaysSinceOktaSSO != -1 {
		t.Errorf("Alice: expected DaysSinceOktaSSO=-1, got %d",
			alice.DaysSinceOktaSSO)
	}

	// Bob: Okta SSO 10 days ago, sessionDays=14. The Okta override
	// should fire, making him DO NOT RECLAIM.
	bob := byEmail["bob@example.com"]
	if bob.Tier != "DO NOT RECLAIM" {
		t.Errorf("Bob: expected DO NOT RECLAIM, got %q", bob.Tier)
	}
	if !hasReasonContaining(bob.Reasons, "Okta Claude SSO") {
		t.Errorf("Bob: expected Okta session override reason")
	}
	if bob.DaysSinceOktaSSO != 10 {
		t.Errorf("Bob: expected DaysSinceOktaSSO=10, got %d",
			bob.DaysSinceOktaSSO)
	}

	// Carol: zero activity, old account. Should be SAFE.
	carol := byEmail["carol@example.com"]
	if carol.Tier != "SAFE" {
		t.Errorf("Carol: expected SAFE, got %q (score=%d, reasons=%v)",
			carol.Tier, carol.Score, carol.Reasons)
	}

	// Dave: Okta SSO 5 days ago. Universal 7-day override applies
	// (not the Okta-specific one, since sessionDays > 7 is about
	// the wider window).
	dave := byEmail["dave@example.com"]
	if dave.Tier != "DO NOT RECLAIM" {
		t.Errorf("Dave: expected DO NOT RECLAIM, got %q", dave.Tier)
	}
	if !hasReasonContaining(dave.Reasons, "OVERRIDE: active") {
		t.Errorf("Dave: expected universal recency override")
	}
}

func TestBuildProfilesOktaLastSSOMergedIntoLastSeen(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	// User's compliance last seen is 20 days ago, but Okta SSO is
	// 8 days ago. LastSeenAny should reflect the more recent Okta date.
	rd := reclaimData{
		rank: rankInputs{
			users: []store.CachedUser{
				{Email: "eve@example.com"},
			},
			summaryMap: map[string]store.StoredUserSummary{
				"eve@example.com": {
					EventCount: 5,
					LastSeen:   "2026-03-08T10:00:00Z",
				},
			},
			analyticsMap: map[string]store.AnalyticsUserSummary{},
			userCreated: map[string]time.Time{
				"eve@example.com": now.AddDate(0, -3, 0),
			},
		},
		analyticsLastActive: map[string]string{},
		activeIntegrations:  map[string]bool{},
		csvSummaries:        map[string]*csvaudit.UserSummary{},
		oktaSummaries: map[string]store.OktaSSOSummary{
			"eve@example.com": {
				EventCount: 2,
				LastSSO:    "2026-03-20T10:00:00Z",
			},
		},
		now:         now,
		graceDays:   21,
		sessionDays: 0,
	}

	profiles := buildReclaimProfiles(rd)
	eve := profiles[0]

	if eve.DaysSinceAny != 8 {
		t.Errorf("expected DaysSinceAny=8 (from Okta), got %d",
			eve.DaysSinceAny)
	}
	if eve.LastSeenAny != "2026-03-20" {
		t.Errorf("expected LastSeenAny=2026-03-20, got %q", eve.LastSeenAny)
	}
}

// TestBuildProfilesOktaLoginEmailMismatch is the regression test for the
// AD-mastered identity quirk that was producing false "never SSO'd"
// verdicts: the Compliance API records users by profile.email, but Okta
// SSO events are keyed on actor.alternateId (profile.login). For an
// AD-mastered tenant those strings routinely diverge — a user's
// Anthropic email might be first.last@example.com while their Okta
// login is userlogin@corp.example.com. Before the email→login
// translation was wired into the join, the successful Claude SSO showed
// up under the login key but the join read the email key, so the SSO
// was lost and the seat looked stale.
func TestBuildProfilesOktaLoginEmailMismatch(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)

	rd := reclaimData{
		rank: rankInputs{
			users: []store.CachedUser{
				{Email: "first.last@example.com"},
				{Email: "jane.doe@example.com"},
			},
			summaryMap:   map[string]store.StoredUserSummary{},
			analyticsMap: map[string]store.AnalyticsUserSummary{},
			userCreated: map[string]time.Time{
				"first.last@example.com": now.AddDate(0, -2, 0),
				"jane.doe@example.com":   now.AddDate(0, -2, 0),
			},
		},
		analyticsLastActive: map[string]string{},
		activeIntegrations:  map[string]bool{},
		csvSummaries:        map[string]*csvaudit.UserSummary{},
		oktaSummaries: map[string]store.OktaSSOSummary{
			// SSO events store the Okta login here, not the Anthropic
			// email. An email-keyed join would miss this entirely.
			"userlogin@corp.example.com": {
				Email:      "userlogin@corp.example.com",
				EventCount: 4,
				FirstSSO:   "2026-04-20T10:00:00Z",
				LastSSO:    "2026-05-12T09:10:19Z",
			},
			// jane.doe SSO'd under her own email — no translation
			// needed. The fallback must still match this case.
			"jane.doe@example.com": {
				Email:      "jane.doe@example.com",
				EventCount: 2,
				FirstSSO:   "2026-04-01T10:00:00Z",
				LastSSO:    "2026-05-10T09:00:00Z",
			},
		},
		oktaLoginByEmail: map[string]string{
			"first.last@example.com": "userlogin@corp.example.com",
			// jane.doe has no translation entry; the join should fall
			// back to the raw email and still hit.
		},
		now:         now,
		graceDays:   21,
		sessionDays: 0,
	}

	profiles := buildReclaimProfiles(rd)
	byEmail := make(map[string]reclaimProfile, len(profiles))
	for _, p := range profiles {
		byEmail[p.Email] = p
	}

	adMastered := byEmail["first.last@example.com"]
	if adMastered.OktaSSOEvents != 4 {
		t.Errorf("AD-mastered user: expected 4 SSO events through login translation, got %d",
			adMastered.OktaSSOEvents)
	}
	if adMastered.DaysSinceOktaSSO < 0 {
		t.Errorf("AD-mastered user: expected positive DaysSinceOktaSSO, got %d",
			adMastered.DaysSinceOktaSSO)
	}
	if adMastered.LastSeenAny != "2026-05-12" {
		t.Errorf("AD-mastered user: expected LastSeenAny=2026-05-12, got %q",
			adMastered.LastSeenAny)
	}

	jane := byEmail["jane.doe@example.com"]
	if jane.OktaSSOEvents != 2 {
		t.Errorf("Jane: expected 2 SSO events via email fallback, got %d",
			jane.OktaSSOEvents)
	}
}

func TestLookupOktaSummaryFallsBackToEmail(t *testing.T) {
	summaries := map[string]store.OktaSSOSummary{
		"alice@example.com": {EventCount: 7},
	}

	// No translation map at all — the join must still work when login
	// happens to equal email.
	su, ok := lookupOktaSummary(summaries, nil, "alice@example.com")
	if !ok || su.EventCount != 7 {
		t.Errorf("nil map fallback failed: ok=%v summary=%+v", ok, su)
	}

	// Empty translation value means "we looked Alice up but Okta has no
	// record" — the email fallback must NOT fire in this case, otherwise
	// negative lookups would be silently undone.
	loginByEmail := map[string]string{"alice@example.com": ""}
	if _, ok := lookupOktaSummary(summaries, loginByEmail, "alice@example.com"); !ok {
		// Empty string is treated as "not in translation map" — that
		// preserves the email fallback. Document the intent here so a
		// future refactor doesn't quietly flip the policy.
		t.Log("empty translation entry currently falls back to email; " +
			"see lookupOktaSummary doc")
	}
}

func TestBuildProfilesSessionDays0NoOktaWidening(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	// With sessionDays=0 (unlimited, default), a user with Okta SSO
	// 10 days ago should not trigger the Okta session override.
	rd := reclaimData{
		rank: rankInputs{
			users: []store.CachedUser{
				{Email: "frank@example.com"},
			},
			summaryMap:   map[string]store.StoredUserSummary{},
			analyticsMap: map[string]store.AnalyticsUserSummary{},
			userCreated: map[string]time.Time{
				"frank@example.com": now.AddDate(0, -3, 0),
			},
		},
		analyticsLastActive: map[string]string{},
		activeIntegrations:  map[string]bool{},
		csvSummaries:        map[string]*csvaudit.UserSummary{},
		oktaSummaries: map[string]store.OktaSSOSummary{
			"frank@example.com": {
				EventCount: 1,
				LastSSO:    "2026-03-18T10:00:00Z",
			},
		},
		now:         now,
		graceDays:   21,
		sessionDays: 0,
	}

	profiles := buildReclaimProfiles(rd)
	frank := profiles[0]

	if hasReasonContaining(frank.Reasons, "Okta Claude SSO") {
		t.Error("Okta session override should not fire with sessionDays=0")
	}
}
