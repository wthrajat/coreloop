package content

import "time"

const (
	PromptVersion   = "lesson-v1"
	SchemaVersion   = "lesson-draft-v1"
	CompilerVersion = "compiler-v1"
)

type Evidence struct {
	ID           string    `json:"id"`
	Publisher    string    `json:"publisher"`
	Title        string    `json:"title"`
	CanonicalURL string    `json:"url"`
	PublishedAt  time.Time `json:"published_at"`
	Passage      string    `json:"passage"`
}

type LessonContext struct {
	TopicID           string     `json:"topic_id"`
	Topic             string     `json:"topic"`
	Level             string     `json:"level"`
	Minutes           int        `json:"minutes"`
	Depth             string     `json:"depth"`
	ThemeID           string     `json:"theme_id"`
	Position          int        `json:"position"`
	Objectives        []string   `json:"objectives"`
	Prerequisites     []string   `json:"prerequisites,omitempty"`
	CoveredObjectives []string   `json:"covered_objectives,omitempty"`
	Evidence          []Evidence `json:"evidence,omitempty"`
}

type LessonDraft struct {
	Title             string   `json:"title"`
	EstimatedMinutes  int      `json:"estimated_minutes"`
	Motivation        string   `json:"motivation"`
	PriorApproaches   []string `json:"prior_approaches"`
	Definition        string   `json:"definition"`
	Mechanics         []string `json:"mechanics"`
	ProductionExample string   `json:"production_example"`
	Tradeoffs         []string `json:"tradeoffs"`
	FailureModes      []string `json:"failure_modes"`
	WhenNotToUse      []string `json:"when_not_to_use"`
	Alternatives      []string `json:"alternatives"`
	Security          string   `json:"security"`
	Reliability       string   `json:"reliability"`
	Performance       string   `json:"performance"`
	Cost              string   `json:"cost"`
	PresentMaturity   string   `json:"present_maturity"`
	FutureDirection   string   `json:"future_direction"`
	CareerRelevance   string   `json:"career_relevance"`
	InterviewAnswer   string   `json:"interview_answer"`
	RecallQuestion    string   `json:"recall_question"`
	Claims            []Claim  `json:"claims"`
	Sources           []Source `json:"sources"`
	Uncertainty       []string `json:"uncertainty"`
}

type Claim struct {
	Text      string   `json:"text"`
	SourceIDs []string `json:"source_ids"`
	Status    string   `json:"status"`
}

type Source struct {
	ID           string `json:"id"`
	Publisher    string `json:"publisher"`
	Title        string `json:"title"`
	CanonicalURL string `json:"url"`
	PublishedAt  string `json:"published_at"`
}

type Generated struct {
	Draft             LessonDraft
	Provider          string
	Model             string
	RequestID         string
	InputTokens       int
	OutputTokens      int
	ValidationErrors  []string
	VerificationState string
	Warning           string
}
