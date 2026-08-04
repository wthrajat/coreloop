package radar

import "testing"

func TestCalculateScoreExposesAllDimensions(t *testing.T) {
	result := CalculateScore(ScoreInput{
		Title:                     "Critical PostgreSQL security release",
		Summary:                   "The database runtime fixes a vulnerability and improves reliability.",
		TopicTerms:                []string{"PostgreSQL", "database", "reliability"},
		SourceTier:                1,
		PublishedAgeHours:         2,
		Category:                  CategorySecurity,
		CommunityPoints:           250,
		CommunityComments:         80,
		CommunitySignalsAvailable: true,
	})
	values := map[string]float64{
		"significance": result.Significance, "topic relevance": result.TopicRelevance,
		"developer value": result.DeveloperValue, "authority": result.Authority,
		"community interest": result.CommunityInterest, "recency": result.Recency,
		"total": result.Total,
	}
	for name, value := range values {
		if value < 0 || value > 1 {
			t.Fatalf("%s = %f, want value in 0..1", name, value)
		}
	}
	if result.TopicRelevance != 1 || result.Authority != 1 || result.Recency != 1 {
		t.Fatalf("unexpected high-signal breakdown: %#v", result)
	}
}

func TestCalculateScoreRewardsRelevantDeveloperNews(t *testing.T) {
	base := ScoreInput{
		Summary:           "Architecture, API compatibility, database performance, and migration details.",
		TopicTerms:        []string{"PostgreSQL", "database", "performance"},
		SourceTier:        1,
		PublishedAgeHours: 4,
		Category:          CategoryEngineering,
	}
	relevant := base
	relevant.Title = "PostgreSQL database performance deep dive"
	irrelevant := base
	irrelevant.Title = "New design palettes"
	irrelevant.Summary = "A collection of visual color themes."
	if CalculateScore(relevant).Total <= CalculateScore(irrelevant).Total {
		t.Fatal("relevant developer news should have a higher score")
	}
}

func TestCalculateScoreRecencyDecaysWithoutHidingOldNews(t *testing.T) {
	input := ScoreInput{Title: "Runtime release", SourceTier: 1, Category: CategoryRelease}
	input.PublishedAgeHours = 1
	fresh := CalculateScore(input)
	input.PublishedAgeHours = 24 * 45
	old := CalculateScore(input)
	if fresh.Recency != 1 || old.Recency != 0.1 {
		t.Fatalf("fresh=%#v old=%#v", fresh, old)
	}
	if fresh.Total <= old.Total || old.Total == 0 {
		t.Fatalf("recency should affect prominence, not erase news: fresh=%f old=%f", fresh.Total, old.Total)
	}
}

func TestCalculateScoreUsesCommunityInterestOnlyWhenAvailable(t *testing.T) {
	input := ScoreInput{Title: "Release", SourceTier: 2, CommunitySignalsAvailable: true}
	withoutEngagement := CalculateScore(input)
	input.CommunityPoints = 400
	input.CommunityComments = 100
	withEngagement := CalculateScore(input)
	if withEngagement.CommunityInterest <= withoutEngagement.CommunityInterest || withEngagement.Total <= withoutEngagement.Total {
		t.Fatalf("community engagement did not increase prominence: without=%#v with=%#v", withoutEngagement, withEngagement)
	}
	input.CommunitySignalsAvailable = false
	unknown := CalculateScore(input)
	if unknown.CommunityInterest != 0.5 {
		t.Fatalf("unknown community signal = %f, want neutral 0.5", unknown.CommunityInterest)
	}
}

func TestEditorialDecisionSuppressesRoutineCVEBulletins(t *testing.T) {
	input := ScoreInput{
		Title:             "CVE-2026-12345 security advisory",
		Summary:           "A security update is available for Example Runtime 3.2.",
		TopicTerms:        []string{"security", "runtime", "backend engineering"},
		SourceTier:        1,
		PublishedAgeHours: 2,
		Category:          CategorySecurity,
	}
	decision := DecideEditorialEligibility(input, CalculateScore(input))
	if decision.Eligible || decision.Reason != "routine_security_bulletin" {
		t.Fatalf("routine CVE decision = %#v", decision)
	}
}

func TestEditorialDecisionSuppressesRoutineAdvisoryDigests(t *testing.T) {
	input := ScoreInput{
		Title:             "CISA releases seven industrial control systems advisories",
		Summary:           "The advisories describe vulnerabilities and updates for critical infrastructure operators.",
		TopicTerms:        []string{"security", "infrastructure", "reliability"},
		SourceTier:        1,
		PublishedAgeHours: 2,
		Category:          CategorySecurity,
	}
	decision := DecideEditorialEligibility(input, CalculateScore(input))
	if decision.Eligible || decision.Reason != "routine_security_bulletin" {
		t.Fatalf("routine advisory decision = %#v", decision)
	}
}

func TestEditorialDecisionKeepsUrgentSecurityIncidents(t *testing.T) {
	input := ScoreInput{
		Title:   "Critical CVE-2026-12345 is actively exploited",
		Summary: "The vulnerability permits remote code execution in a widely used runtime.",
		TopicTerms: []string{
			"security", "runtime", "backend engineering", "reliability",
		},
		SourceTier:        1,
		PublishedAgeHours: 2,
		Category:          CategorySecurity,
	}
	score := CalculateScore(input)
	decision := DecideEditorialEligibility(input, score)
	if !decision.Eligible {
		t.Fatalf("urgent security incident was suppressed: score=%#v decision=%#v", score, decision)
	}
}

func TestEditorialDecisionSuppressesRoutineRegionalAvailability(t *testing.T) {
	input := ScoreInput{
		Title:             "Compute instances now available in Europe (Paris) region",
		Summary:           "The existing instance type adds another cloud region.",
		TopicTerms:        []string{"cloud", "infrastructure", "backend engineering"},
		SourceTier:        1,
		PublishedAgeHours: 2,
		Category:          CategoryProductUpdate,
	}
	decision := DecideEditorialEligibility(input, CalculateScore(input))
	if decision.Eligible || decision.Reason != "routine_regional_availability" {
		t.Fatalf("regional availability decision = %#v", decision)
	}
}

func TestEditorialDecisionKeepsHighValueEngineeringWork(t *testing.T) {
	input := ScoreInput{
		Title:   "How we redesigned database replication after a production outage",
		Summary: "A postmortem explains the architecture, failure mode, migration, observability, and reliability trade-offs.",
		TopicTerms: []string{
			"database", "architecture", "reliability", "backend engineering",
		},
		SourceTier:        2,
		PublishedAgeHours: 8,
		Category:          CategoryEngineering,
	}
	score := CalculateScore(input)
	decision := DecideEditorialEligibility(input, score)
	if !decision.Eligible {
		t.Fatalf("high-value engineering item was suppressed: score=%#v decision=%#v", score, decision)
	}
}

func TestEditorialDecisionRejectsItemsOlderThanTenDays(t *testing.T) {
	input := ScoreInput{
		Title:             "Major runtime architecture release",
		Summary:           "The release changes the compiler, performance, and deployment model.",
		TopicTerms:        []string{"runtime", "compiler", "performance", "deployment"},
		SourceTier:        1,
		PublishedAgeHours: MaximumItemAge.Hours() + 1,
		Category:          CategoryRelease,
	}
	decision := DecideEditorialEligibility(input, CalculateScore(input))
	if decision.Eligible || decision.Reason != "outside_freshness_window" {
		t.Fatalf("stale item decision = %#v", decision)
	}
}
