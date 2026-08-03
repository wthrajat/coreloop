package radar

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

const RankerVersion = "keyword-authority-recency-v1"

var wordPattern = regexp.MustCompile(`[a-z0-9+#.]{3,}`)
var stopWords = map[string]bool{"this": true, "that": true, "with": true, "from": true, "into": true, "your": true, "about": true, "engineering": true, "foundations": true, "systems": true}

func Score(title, summary string, topicTerms []string, sourceTier int, publishedAgeHours float64) float64 {
	document := terms(title + " " + summary)
	wanted := terms(strings.Join(topicTerms, " "))
	matches := 0
	for term := range wanted {
		if document[term] {
			matches++
		}
	}
	coverage := 0.0
	if len(wanted) > 0 {
		coverage = math.Min(1, float64(matches)/math.Min(6, float64(len(wanted))))
	}
	authority := 1 - float64(sourceTier-1)*0.2
	if authority < 0.5 {
		authority = 0.5
	}
	recency := 1.0
	if publishedAgeHours > 24 {
		recency = math.Max(0.35, 1-(publishedAgeHours-24)/(24*30))
	}
	return math.Round((coverage*0.65+authority*0.2+recency*0.15)*1000) / 1000
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
		if !stopWords[word] {
			output[word] = true
		}
	}
	return output
}
