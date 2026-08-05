export type User = {
  id: string;
  display_name: string;
  username: string;
  avatar_url: string;
  status: string;
  created_at: string;
};

export type SessionPayload = { user: User; owner: boolean };

export type Topic = {
  id: string;
  slug: string;
  title: string;
  lane: string;
  difficulty: "beginner" | "intermediate" | "advanced";
  objectives: string[];
};

export type LearningProfile = {
  current_level: "beginner" | "intermediate" | "advanced";
  goals: string[];
  target_roles: string[];
  current_technologies: string[];
  target_technologies: string[];
};

export type Preferences = {
  lesson_minutes: 15 | 30;
  explanation_depth: "foundation" | "standard" | "detailed";
  lessons_per_day: number;
  radar_enabled: boolean;
  radar_items_per_day: number;
  radar_weekends_enabled: boolean;
  recall_mode: "off" | "light" | "standard";
  weekends_enabled: boolean;
  bundle_mode: "complete" | "continue_after_intro";
  time_zone: "Asia/Kolkata";
  paused_until?: string;
  delivery_times: string[];
  topic_ids: string[];
};

export type Overview = {
  active_theme: string;
  active_topic: string;
  next_delivery_at?: string;
  unread_lessons: number;
  read_lessons: number;
  telegram_connected: boolean;
  queue_state: "healthy" | "quota_exhausted";
  quota_blocked_count: number;
};

export type Assignment = {
  id: string;
  title: string;
  topic: string;
  state: "queued" | "delivered" | "read" | "skipped" | "superseded";
  assigned_at: string;
  delivered_at?: string;
  read_at?: string;
};

export type Operations = {
  queued: number;
  leased: number;
  failed: number;
  blocked_quota: number;
  users: number;
  sources: number;
  blocked_jobs: { id: string; created_at: string; attempt_count: number }[];
};

export type ManualLesson = {
  job_id: string;
  state:
    | "queued"
    | "generating"
    | "delivering"
    | "delivered"
    | "quota_blocked"
    | "failed";
  message: string;
};

export type ManualRadar = {
  batch_id: string;
  state: "queued" | "delivering" | "delivered" | "failed";
  profile_target: number;
  requested_count: number;
  selected_count: number;
  delivered_count: number;
  failed_count: number;
  message: string;
};
