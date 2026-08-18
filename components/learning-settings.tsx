"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { api } from "@/lib/api-client";
import type { LearningProfile, Preferences, Topic } from "@/lib/api-types";
import { lessonTimePresets } from "@/lib/schedule";

const emptyProfile: LearningProfile = {
  current_level: "intermediate",
  goals: [],
  target_roles: [],
  current_technologies: [],
  target_technologies: [],
};

async function fetchConfiguration() {
  const [profileData, topicData] = await Promise.all([
    api<{ profile: LearningProfile; preferences: Preferences }>("/profile"),
    api<{ topics: Topic[] }>("/topics"),
  ]);
  return { profileData, topicData };
}

export function LearningSettings({
  onboarding = false,
}: {
  onboarding?: boolean;
}) {
  const [profile, setProfile] = useState<LearningProfile>(emptyProfile);
  const [preferences, setPreferences] = useState<Preferences | null>(null);
  const [topics, setTopics] = useState<Topic[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [loadError, setLoadError] = useState("");
  const [saveState, setSaveState] = useState<
    "idle" | "dirty" | "saving" | "saved" | "error"
  >("idle");
  const [saveMessage, setSaveMessage] = useState("");

  const loadConfiguration = useCallback(async () => {
    try {
      const { profileData, topicData } = await fetchConfiguration();
      setProfile(profileData.profile);
      setPreferences(profileData.preferences);
      setTopics(topicData.topics ?? []);
      setSaveState("idle");
      setSaveMessage("");
      setLoadState("ready");
    } catch (reason) {
      setLoadError(
        reason instanceof Error
          ? reason.message
          : "Your configuration could not be loaded.",
      );
      setLoadState("error");
    }
  }, []);

  useEffect(() => {
    let active = true;
    fetchConfiguration()
      .then(({ profileData, topicData }) => {
        if (!active) return;
        setProfile(profileData.profile);
        setPreferences(profileData.preferences);
        setTopics(topicData.topics ?? []);
        setSaveState("idle");
        setSaveMessage("");
        setLoadState("ready");
      })
      .catch((reason: unknown) => {
        if (!active) return;
        setLoadError(
          reason instanceof Error
            ? reason.message
            : "Your configuration could not be loaded.",
        );
        setLoadState("error");
      });
    return () => {
      active = false;
    };
  }, []);

  function retryConfiguration() {
    setLoadState("loading");
    setLoadError("");
    void loadConfiguration();
  }

  const lanes = useMemo(() => {
    const values = new Map<string, Topic[]>();
    for (const topic of topics)
      values.set(topic.lane, [...(values.get(topic.lane) ?? []), topic]);
    return [...values.entries()];
  }, [topics]);

  if (loadState === "loading")
    return (
      <div className="page-stack" role="status">
        <span aria-hidden="true" className="loading-line" />
        <p className="muted-copy">Loading your private configuration…</p>
      </div>
    );

  if (loadState === "error" || !preferences) {
    return (
      <div className="page-stack">
        <PageHeader
          title={onboarding ? "Set up Coreloop" : "Learning settings"}
          description="Your saved configuration has not been changed."
        />
        <section className="notice notice-error" role="alert">
          <div>
            <strong>Configuration unavailable</strong>
            <p>{loadError || "Your configuration could not be loaded."}</p>
          </div>
          <button
            className="button button-secondary"
            onClick={retryConfiguration}
            type="button"
          >
            Try again
          </button>
        </section>
      </div>
    );
  }

  function markDirty() {
    setSaveState("dirty");
    setSaveMessage("");
  }

  function update<K extends keyof Preferences>(key: K, value: Preferences[K]) {
    setPreferences((current) =>
      current ? { ...current, [key]: value } : current,
    );
    markDirty();
  }
  function changeCount(count: number) {
    setPreferences((current) =>
      current
        ? {
            ...current,
            lessons_per_day: count,
            delivery_times: [...lessonTimePresets[count]],
          }
        : current,
    );
    markDirty();
  }
  function toggleTopic(id: string) {
    setPreferences((current) => {
      if (!current) return current;
      const selected = current.topic_ids.includes(id);
      return {
        ...current,
        topic_ids: selected
          ? current.topic_ids.filter((value) => value !== id)
          : [...current.topic_ids, id],
      };
    });
    markDirty();
  }
  async function save(event: React.FormEvent) {
    event.preventDefault();
    const preferencesToSave = preferences;
    if (!preferencesToSave || preferencesToSave.topic_ids.length === 0) {
      setSaveState("error");
      setSaveMessage("Choose at least one topic before saving.");
      return;
    }
    setSaveState("saving");
    setSaveMessage("");
    try {
      await api<{ profile: LearningProfile; preferences: Preferences }>(
        "/configuration",
        {
          method: "PUT",
          body: JSON.stringify({ profile, preferences: preferencesToSave }),
        },
      );
      setSaveState("saved");
      setSaveMessage("Your topics and delivery loop are saved.");
      if (onboarding) window.location.assign("/overview");
    } catch (reason) {
      setSaveState("error");
      const detail =
        reason instanceof Error
          ? reason.message
          : "Settings could not be saved.";
      setSaveMessage(detail);
    }
  }

  return (
    <form
      aria-busy={saveState === "saving"}
      className="page-stack settings-page"
      onSubmit={(event) => void save(event)}
    >
      <PageHeader
        title={onboarding ? "Set up Coreloop" : "Learning settings"}
        description={
          onboarding
            ? "Start with useful defaults. Every choice can be changed later without losing queued lessons."
            : "Choose lesson depth, topics, delivery rhythm, and News Radar."
        }
        action={
          <button
            className="button button-primary"
            disabled={
              saveState === "saving" || preferences.topic_ids.length === 0
            }
            type="submit"
          >
            {saveState === "saving"
              ? "Saving…"
              : onboarding
                ? "Save and start"
                : "Save changes"}
          </button>
        }
      />
      {saveMessage ? (
        <section
          className={`notice ${saveState === "error" ? "notice-error" : "notice-success"}`}
          role={saveState === "error" ? "alert" : "status"}
        >
          <div>
            <strong>
              {saveState === "error" ? "Could not save" : "Configuration saved"}
            </strong>
            <p>{saveMessage}</p>
          </div>
        </section>
      ) : null}
      <fieldset className="settings-fields" disabled={saveState === "saving"}>
        <legend className="sr-only">Learning configuration</legend>
        <section className="settings-section">
          <div className="settings-heading">
            <span className="section-number">01</span>
            <h2>Lesson shape</h2>
            <p>
              Every preset remains detailed. Duration controls breadth; depth
              controls how far trade-offs and internals go.
            </p>
          </div>
          <div className="form-grid">
            <fieldset>
              <legend>Reading time</legend>
              <div className="segmented-control">
                {([15, 30] as const).map((minutes) => (
                  <label key={minutes}>
                    <input
                      checked={preferences.lesson_minutes === minutes}
                      name="minutes"
                      onChange={() => update("lesson_minutes", minutes)}
                      type="radio"
                    />
                    <span>{minutes} min</span>
                  </label>
                ))}
              </div>
            </fieldset>
            <label className="field">
              <span>Explanation depth</span>
              <select
                value={preferences.explanation_depth}
                onChange={(event) =>
                  update(
                    "explanation_depth",
                    event.target.value as Preferences["explanation_depth"],
                  )
                }
              >
                <option value="foundation">Detailed foundation</option>
                <option value="standard">Detailed standard</option>
                <option value="detailed">Very detailed</option>
              </select>
              <small>
                Plain English, with unfamiliar subject terms explained when they
                first appear.
              </small>
            </label>
            <label className="field">
              <span>Current level</span>
              <select
                value={profile.current_level}
                onChange={(event) => {
                  setProfile({
                    ...profile,
                    current_level: event.target
                      .value as LearningProfile["current_level"],
                  });
                  markDirty();
                }}
              >
                <option value="beginner">Beginner</option>
                <option value="intermediate">Intermediate</option>
                <option value="advanced">Advanced</option>
              </select>
            </label>
            <label className="field">
              <span>Spaced recall</span>
              <select
                value={preferences.recall_mode}
                onChange={(event) =>
                  update(
                    "recall_mode",
                    event.target.value as Preferences["recall_mode"],
                  )
                }
              >
                <option value="off">Off</option>
                <option value="light">Light · after about 3 days</option>
                <option value="standard">Standard · after about 1 day</option>
              </select>
              <small>
                Adds at most one earlier recall question to the start of a later
                lesson. It never creates a separate notification.
              </small>
            </label>
          </div>
        </section>
        <section className="settings-section">
          <div className="settings-heading">
            <span className="section-number">02</span>
            <h2>Delivery rhythm</h2>
            <p>
              Times use this instance&apos;s configured time zone. Full lessons
              arrive as ordered Telegram messages.
            </p>
          </div>
          <div className="form-grid">
            <label className="field">
              <span>Lessons per day</span>
              <select
                value={preferences.lessons_per_day}
                onChange={(event) => changeCount(Number(event.target.value))}
              >
                {[1, 2, 3, 4, 5, 6].map((count) => (
                  <option key={count} value={count}>
                    {count}
                  </option>
                ))}
              </select>
            </label>
            <div className="field field-wide">
              <span>Delivery times · Asia/Kolkata</span>
              <div className="time-inputs">
                {preferences.delivery_times.map((value, index) => (
                  <label key={index}>
                    <span className="sr-only">Delivery {index + 1}</span>
                    <input
                      type="time"
                      value={value}
                      onChange={(event) =>
                        update(
                          "delivery_times",
                          preferences.delivery_times.map((time, timeIndex) =>
                            timeIndex === index ? event.target.value : time,
                          ),
                        )
                      }
                    />
                  </label>
                ))}
              </div>
            </div>
            <label className="toggle-row">
              <span>
                <strong>Send lessons on weekends</strong>
                <small>
                  Keep lesson delivery active on Saturday and Sunday.
                </small>
              </span>
              <input
                checked={preferences.weekends_enabled}
                onChange={(event) =>
                  update("weekends_enabled", event.target.checked)
                }
                type="checkbox"
              />
            </label>
            <label className="toggle-row">
              <span>
                <strong>News Radar</strong>
                <small>
                  Receive source-backed updates independently of lessons.
                </small>
              </span>
              <input
                checked={preferences.radar_enabled}
                onChange={(event) =>
                  update("radar_enabled", event.target.checked)
                }
                type="checkbox"
              />
            </label>
            <label className="field">
              <span>News updates per day</span>
              <input
                max={50}
                min={0}
                onChange={(event) =>
                  update("radar_items_per_day", Number(event.target.value))
                }
                step={1}
                type="number"
                value={preferences.radar_items_per_day}
              />
              <small>Choose 1–50, or set 0 for unlimited updates.</small>
            </label>
            <label className="toggle-row">
              <span>
                <strong>Send news on weekends</strong>
                <small>
                  Keep source-backed news active on Saturday and Sunday,
                  independently of lesson delivery.
                </small>
              </span>
              <input
                checked={preferences.radar_weekends_enabled}
                onChange={(event) =>
                  update("radar_weekends_enabled", event.target.checked)
                }
                type="checkbox"
              />
            </label>
          </div>
        </section>
        <section className="settings-section">
          <div className="settings-heading">
            <span className="section-number">03</span>
            <h2>Topics</h2>
            <p>
              Choose the areas the planner may use. It continues a coherent
              theme and stores decisions to avoid frequent repetition.
            </p>
          </div>
          <div className="topic-groups">
            {lanes.map(([lane, laneTopics]) => (
              <fieldset className="topic-group" key={lane}>
                <legend>{lane}</legend>
                {laneTopics.map((topic) => (
                  <label className="topic-option" key={topic.id}>
                    <input
                      checked={preferences.topic_ids.includes(topic.id)}
                      onChange={() => toggleTopic(topic.id)}
                      type="checkbox"
                    />
                    <span>
                      <strong>{topic.title}</strong>
                      <small>{topic.objectives.slice(0, 2).join(" · ")}</small>
                    </span>
                  </label>
                ))}
              </fieldset>
            ))}
            {preferences.topic_ids.length === 0 ? (
              <p className="field-error" role="alert">
                Choose at least one topic before saving.
              </p>
            ) : null}
          </div>
        </section>
      </fieldset>
      <section className="save-bar">
        <div>
          <StatusPill
            tone={
              saveState === "saved"
                ? "ready"
                : saveState === "error"
                  ? "error"
                  : "neutral"
            }
          >
            {saveState === "saved"
              ? "Saved"
              : saveState === "error"
                ? "Needs attention"
                : saveState === "saving"
                  ? "Saving"
                  : saveState === "dirty"
                    ? "Unsaved changes"
                    : "Up to date"}
          </StatusPill>
          <p>
            Automatic AI routing is Groq → Gemini. OpenAI always requires a
            separate owner action.
          </p>
        </div>
        <button
          className="button button-primary"
          disabled={
            saveState === "saving" || preferences.topic_ids.length === 0
          }
          type="submit"
        >
          {saveState === "saving" ? "Saving…" : "Save configuration"}
        </button>
      </section>
    </form>
  );
}
