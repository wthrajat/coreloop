package store

import "time"

type User struct {
	ID              string    `json:"id"`
	TelegramSubject string    `json:"-"`
	DisplayName     string    `json:"display_name"`
	Username        string    `json:"username"`
	AvatarURL       string    `json:"avatar_url"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

type Session struct {
	ID        string
	UserID    string
	CSRFHash  string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type Invite struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
	Consumed  bool      `json:"consumed"`
}

type AuthFlow struct {
	ID           string
	InviteID     string
	CodeVerifier string
	Nonce        string
	ReturnPath   string
	ExpiresAt    time.Time
}

type LearningProfile struct {
	CurrentLevel        string   `json:"current_level"`
	Goals               []string `json:"goals"`
	TargetRoles         []string `json:"target_roles"`
	CurrentTechnologies []string `json:"current_technologies"`
	TargetTechnologies  []string `json:"target_technologies"`
}

type Preferences struct {
	LessonMinutes        int        `json:"lesson_minutes"`
	ExplanationDepth     string     `json:"explanation_depth"`
	LessonsPerDay        int        `json:"lessons_per_day"`
	RadarEnabled         bool       `json:"radar_enabled"`
	RadarItemsPerDay     int        `json:"radar_items_per_day"`
	RadarWeekendsEnabled bool       `json:"radar_weekends_enabled"`
	RecallMode           string     `json:"recall_mode"`
	WeekendsEnabled      bool       `json:"weekends_enabled"`
	BundleMode           string     `json:"bundle_mode"`
	TimeZone             string     `json:"time_zone"`
	PausedUntil          *time.Time `json:"paused_until,omitempty"`
	DeliveryTimes        []string   `json:"delivery_times"`
	TopicIDs             []string   `json:"topic_ids"`
}

type Topic struct {
	ID         string   `json:"id"`
	Slug       string   `json:"slug"`
	Title      string   `json:"title"`
	Lane       string   `json:"lane"`
	Difficulty string   `json:"difficulty"`
	Objectives []string `json:"objectives"`
}

type Overview struct {
	ActiveTheme       string     `json:"active_theme"`
	ActiveTopic       string     `json:"active_topic"`
	NextDeliveryAt    *time.Time `json:"next_delivery_at,omitempty"`
	UnreadLessons     int        `json:"unread_lessons"`
	ReadLessons       int        `json:"read_lessons"`
	TelegramConnected bool       `json:"telegram_connected"`
	QueueState        string     `json:"queue_state"`
	QuotaBlockedCount int        `json:"quota_blocked_count"`
}

type AssignmentSummary struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Topic       string     `json:"topic"`
	State       string     `json:"state"`
	AssignedAt  time.Time  `json:"assigned_at"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
}

type Job struct {
	ID             string
	Sequence       int64
	UserID         string
	AssignmentID   string
	Type           string
	State          string
	DueAt          time.Time
	AttemptCount   int
	MaxAttempts    int
	IdempotencyKey string
	PayloadJSON    string
}
