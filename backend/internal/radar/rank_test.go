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
