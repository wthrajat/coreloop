package radar

import (
	"regexp"
	"strings"
)

type Category string

const (
	CategoryResearch      Category = "research"
	CategoryRelease       Category = "release"
	CategorySecurity      Category = "security"
	CategoryFunding       Category = "funding"
	CategoryPartnership   Category = "partnership"
	CategoryPricing       Category = "pricing"
	CategoryIndustry      Category = "industry"
	CategoryDiscussion    Category = "discussion"
	CategoryProductUpdate Category = "product_update"
	CategoryEngineering   Category = "engineering"
)

type categoryRule struct {
	category Category
	pattern  *regexp.Regexp
}

var categoryRules = []categoryRule{
	{CategorySecurity, regexp.MustCompile(`(?i)\b(?:CVE-\d{4}-\d+|security advisory|security (?:update|patch|release|fix)|vulnerabilit(?:y|ies)|zero[- ]day|exploit|ransomware|data breach|security incident|malware|supply[- ]chain attack|credential leak)\b`)},
	{CategoryDiscussion, regexp.MustCompile(`(?i)(?:\bask HN\b|\bdiscussion\b|\bcommunity thread\b|\bQ(?:&|and)A\b|\bRFC discussion\b|\bwhat do you think\b)`)},
	{CategoryFunding, regexp.MustCompile(`(?i)(?:\b(?:seed|series [a-z]|venture|funding|fundraise|investment) round\b|\brais(?:e|es|ed)\s+(?:\$|€|£)|\b(?:secured|received)\s+(?:\$|€|£).+funding\b)`)},
	{CategoryPartnership, regexp.MustCompile(`(?i)\b(?:partnership|partners with|partnered with|collaboration|collaborates with|teams up with|strategic alliance)\b`)},
	{CategoryPricing, regexp.MustCompile(`(?i)\b(?:pricing|price change|price reduction|price increase|subscription|free tier|billing|usage[- ]based pricing|cost reduction)\b`)},
	{CategoryResearch, regexp.MustCompile(`(?i)\b(?:research|research paper|paper|arXiv|technical report|study|model card|evaluation|benchmark study|new dataset)\b`)},
	{CategoryEngineering, regexp.MustCompile(`(?i)\b(?:architecture|postmortem|post-mortem|deep dive|how we (?:built|scaled|migrated)|internals|under the hood|engineering blog|implementation|performance analysis|migration guide|technical tutorial|case study)\b`)},
	{CategoryProductUpdate, regexp.MustCompile(`(?i)\b(?:now available in|available in .+ regions?|regional availability|new regions?|feature update|product update|console update|dashboard update|adds support for|now supports)\b`)},
	{CategoryRelease, regexp.MustCompile(`(?i)(?:\b(?:released?|launch(?:es|ed)?|general availability|generally available|stable release|preview|beta|release candidate|changelog)\b|\bv?\d+\.\d+(?:\.\d+)?\b)`)},
	{CategoryIndustry, regexp.MustCompile(`(?i)\b(?:acquisition|acquires?|merger|regulation|regulator|antitrust|lawsuit|leadership change|appoints?|steps down|layoffs?|policy change|open standard|standardization|market share|industry)\b`)},
}

// Classify assigns a neutral editorial label without excluding any category.
func Classify(title, summary string) Category {
	content := strings.TrimSpace(title + " " + summary)
	for _, rule := range categoryRules {
		if rule.pattern.MatchString(content) {
			return rule.category
		}
	}
	return CategoryProductUpdate
}

func (category Category) Label() string {
	switch category {
	case CategoryResearch:
		return "Research"
	case CategoryRelease:
		return "Release"
	case CategorySecurity:
		return "Security"
	case CategoryFunding:
		return "Funding"
	case CategoryPartnership:
		return "Partnership"
	case CategoryPricing:
		return "Pricing"
	case CategoryIndustry:
		return "Industry"
	case CategoryDiscussion:
		return "Discussion"
	case CategoryEngineering:
		return "Engineering"
	default:
		return "Product update"
	}
}
