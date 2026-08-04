package radar

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

const RankerVersion = "deterministic-editorial-v3"

const MaximumItemAge = 10 * 24 * time.Hour

// MinimumDeliveryScore is a defensive release threshold. EditorialDecision
// applies more specific quality rules before a candidate reaches the queue,
// while the release store uses this value to avoid draining weak backlogs.
const MinimumDeliveryScore = 0.58

const (
	significanceWeight      = 0.25
	topicRelevanceWeight    = 0.25
	developerValueWeight    = 0.20
	authorityWeight         = 0.15
	communityInterestWeight = 0.10
	recencyWeight           = 0.05
)

var wordPattern = regexp.MustCompile(`[a-z0-9+#.]{2,}`)
var stopWords = map[string]bool{
	"about": true, "an": true, "as": true, "at": true, "be": true,
	"by": true, "engineering": true, "for": true, "foundations": true,
	"from": true, "in": true, "into": true, "is": true, "it": true,
	"of": true, "on": true, "or": true, "systems": true, "that": true,
	"the": true, "this": true, "to": true, "we": true, "with": true,
	"your": true,
}

var impactTerms = map[string]bool{
	"breaking": true, "critical": true, "deprecation": true,
	"deprecated": true, "exploit": true, "major": true,
	"migration": true, "outage": true, "standard": true,
	"vulnerability": true,
}

var urgentSecurityTerms = map[string]bool{
	"actively": true, "critical": true, "exploited": true,
	"ransomware": true, "rce": true, "remote": true,
	"wormable": true,
}

var cvePattern = regexp.MustCompile(`(?i)\bCVE-\d{4}-\d+\b`)
var routineSecurityPattern = regexp.MustCompile(`(?i)\b(?:security advisor(?:y|ies)|security bulletin|security update|ICS advisor(?:y|ies)|vulnerabilit(?:y|ies)(?: notice)?)\b`)
var urgentSecurityPattern = regexp.MustCompile(
	`(?i)\b(?:actively exploited|known exploited|exploitation observed|public exploit|` +
		`in the wild|critical(?:[- ]severity| vulnerability| flaw| bug| issue| CVE)|` +
		`rated critical|CVSS\s+(?:9|10)(?:\.\d)?|high[- ]severity|zero[- ]day|` +
		`remote code execution|arbitrary code execution|RCE|authentication bypass|` +
		`privilege escalation|emergency patch|ransomware|supply[- ]chain attack|wormable)\b`,
)
var regionalAvailabilityPattern = regexp.MustCompile(`(?i)(?:\b(?:now available|available)\s+in\b.{0,100}\bregions?\b|\bregional availability\b|\bnew regions?\b)`)

var developerTerms = map[string]bool{
	"ai": true, "api": true, "architecture": true, "benchmark": true,
	"bitcoin": true, "code": true, "compatibility": true,
	"cloud": true, "compiler": true, "database": true, "debugging": true,
	"deployment": true, "github": true, "inference": true,
	"infrastructure": true, "kubernetes": true, "latency": true,
	"lightning": true, "linux": true, "migration": true,
	"model": true, "performance": true, "postgres": true,
	"protocol": true, "reliability": true, "runtime": true, "rust": true,
	"sdk": true, "security": true, "testing": true,
	"training": true,
}

type ScoreInput struct {
	Title                     string
	Summary                   string
	TopicTerms                []string
	SourceTier                int
	PublishedAgeHours         float64
	Category                  Category
	CommunityPoints           int
	CommunityComments         int
	CommunitySignalsAvailable bool
}

// ScoreBreakdown exposes every ranking dimension so callers can persist and
// explain why an item was prominent. All values, including Total, are 0..1.
type ScoreBreakdown struct {
	Significance      float64 `json:"significance"`
	TopicRelevance    float64 `json:"topic_relevance"`
	DeveloperValue    float64 `json:"developer_value"`
	Authority         float64 `json:"authority"`
	CommunityInterest float64 `json:"community_interest"`
	Recency           float64 `json:"recency"`
	Total             float64 `json:"total"`
}

// EditorialDecision explains whether a ranked item is strong enough to enter
// the delivery backlog. A daily target is a ceiling, not a requirement to send
// weak items.
type EditorialDecision struct {
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason"`
}

// Score is retained for existing callers. New integrations should use
// CalculateScore and persist its individual dimensions.
func Score(title, summary string, topicTerms []string, sourceTier int, publishedAgeHours float64) float64 {
	return CalculateScore(ScoreInput{
		Title:             title,
		Summary:           summary,
		TopicTerms:        topicTerms,
		SourceTier:        sourceTier,
		PublishedAgeHours: publishedAgeHours,
	}).Total
}

func CalculateScore(input ScoreInput) ScoreBreakdown {
	category := input.Category
	if category == "" {
		category = Classify(input.Title, input.Summary)
	}
	document := terms(input.Title + " " + input.Summary)
	breakdown := ScoreBreakdown{
		Significance:      significance(category, document),
		TopicRelevance:    topicRelevance(document, input.TopicTerms),
		DeveloperValue:    developerValue(category, document),
		Authority:         sourceAuthority(input.SourceTier),
		CommunityInterest: communityInterest(input),
		Recency:           recency(input.PublishedAgeHours),
	}
	breakdown.Significance = rounded(breakdown.Significance)
	breakdown.TopicRelevance = rounded(breakdown.TopicRelevance)
	breakdown.DeveloperValue = rounded(breakdown.DeveloperValue)
	breakdown.Authority = rounded(breakdown.Authority)
	breakdown.CommunityInterest = rounded(breakdown.CommunityInterest)
	breakdown.Recency = rounded(breakdown.Recency)
	breakdown.Total = rounded(
		breakdown.Significance*significanceWeight +
			breakdown.TopicRelevance*topicRelevanceWeight +
			breakdown.DeveloperValue*developerValueWeight +
			breakdown.Authority*authorityWeight +
			breakdown.CommunityInterest*communityInterestWeight +
			breakdown.Recency*recencyWeight,
	)
	return breakdown
}

// DecideEditorialEligibility suppresses deterministic low-signal patterns and
// then applies the general score threshold. It deliberately does not require
// AI, so Radar quality remains stable when provider quota is unavailable.
func DecideEditorialEligibility(input ScoreInput, score ScoreBreakdown) EditorialDecision {
	documentText := strings.TrimSpace(input.Title + " " + input.Summary)
	category := input.Category
	if category == "" {
		category = Classify(input.Title, input.Summary)
	}
	if input.PublishedAgeHours > MaximumItemAge.Hours() {
		return EditorialDecision{Reason: "outside_freshness_window"}
	}

	if category == CategorySecurity &&
		(cvePattern.MatchString(documentText) || routineSecurityPattern.MatchString(documentText)) &&
		!urgentSecurityPattern.MatchString(documentText) {
		return EditorialDecision{Reason: "routine_security_bulletin"}
	}
	if category == CategoryProductUpdate && regionalAvailabilityPattern.MatchString(documentText) {
		return EditorialDecision{Reason: "routine_regional_availability"}
	}
	if score.Total < MinimumDeliveryScore {
		return EditorialDecision{Reason: "below_editorial_threshold"}
	}
	return EditorialDecision{Eligible: true, Reason: "high_editorial_value"}
}

func Keywords(values ...string) []string {
	set := terms(strings.Join(values, " "))
	output := make([]string, 0, len(set))
	for value := range set {
		output = append(output, value)
	}
	sort.Strings(output)
	return output
}
func terms(value string) map[string]bool {
	output := map[string]bool{}
	for _, word := range wordPattern.FindAllString(strings.ToLower(value), -1) {
		word = strings.Trim(word, ".")
		if !stopWords[word] {
			output[word] = true
		}
	}
	return output
}

func significance(category Category, document map[string]bool) float64 {
	base := map[Category]float64{
		CategorySecurity:      0.62,
		CategoryResearch:      0.75,
		CategoryRelease:       0.72,
		CategoryEngineering:   0.68,
		CategoryIndustry:      0.62,
		CategoryFunding:       0.58,
		CategoryProductUpdate: 0.56,
		CategoryPricing:       0.54,
		CategoryPartnership:   0.52,
		CategoryDiscussion:    0.50,
	}[category]
	if base == 0 {
		base = 0.5
	}
	impactBonus := math.Min(0.2, float64(countTerms(document, impactTerms))*0.05)
	if category == CategorySecurity {
		impactBonus += math.Min(0.25, float64(countTerms(document, urgentSecurityTerms))*0.05)
	}
	return clamp(base + impactBonus)
}

func topicRelevance(document map[string]bool, topicTerms []string) float64 {
	wanted := terms(strings.Join(topicTerms, " "))
	if len(wanted) == 0 {
		return 0.5
	}
	matches := 0
	for term := range wanted {
		if document[term] {
			matches++
		}
	}
	return math.Min(1, float64(matches)/math.Min(6, float64(len(wanted))))
}

func developerValue(category Category, document map[string]bool) float64 {
	base := map[Category]float64{
		CategoryEngineering:   0.72,
		CategorySecurity:      0.70,
		CategoryResearch:      0.62,
		CategoryRelease:       0.60,
		CategoryDiscussion:    0.50,
		CategoryProductUpdate: 0.43,
		CategoryPricing:       0.40,
		CategoryIndustry:      0.38,
		CategoryFunding:       0.34,
		CategoryPartnership:   0.32,
	}[category]
	if base == 0 {
		base = 0.35
	}
	return clamp(base + math.Min(0.28, float64(countTerms(document, developerTerms))*0.04))
}

func sourceAuthority(sourceTier int) float64 {
	switch sourceTier {
	case 1:
		return 1
	case 2:
		return 0.8
	case 3:
		return 0.6
	default:
		return 0.5
	}
}

func communityInterest(input ScoreInput) float64 {
	if !input.CommunitySignalsAvailable {
		return 0.5
	}
	points := math.Max(0, float64(input.CommunityPoints))
	comments := math.Max(0, float64(input.CommunityComments))
	engagement := points + comments*2
	return clamp(math.Log1p(engagement) / math.Log1p(500))
}

func recency(ageHours float64) float64 {
	if math.IsNaN(ageHours) || ageHours < 0 {
		return 1
	}
	switch {
	case ageHours <= 6:
		return 1
	case ageHours <= 24:
		return 0.9
	case ageHours <= 72:
		return 0.75
	case ageHours <= 24*7:
		return 0.55
	case ageHours <= 24*30:
		return 0.25
	default:
		return 0.1
	}
}

func countTerms(document, candidates map[string]bool) int {
	count := 0
	for term := range candidates {
		if document[term] {
			count++
		}
	}
	return count
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}

func rounded(value float64) float64 {
	return math.Round(clamp(value)*1000) / 1000
}
