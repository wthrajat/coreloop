"use client";

import { useEffect, useMemo, useState } from "react";

import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { api } from "@/lib/api-client";
import type { LearningProfile, Preferences, Topic } from "@/lib/api-types";

const timePresets: Record<number, string[]> = {
  1: ["20:30"],
  2: ["08:30", "20:30"],
  3: ["08:30", "13:00", "20:30"],
  4: ["08:30", "12:00", "16:30", "20:30"],
  5: ["08:00", "11:00", "14:00", "17:30", "20:30"],
  6: ["08:00", "10:30", "13:00", "15:30", "18:00", "20:30"],
};

const emptyProfile: LearningProfile = {
  current_level: "intermediate",
  goals: [],
  target_roles: [],
  current_technologies: [],
  target_technologies: [],
};

export function LearningSettings({
  onboarding = false,
}: {
  onboarding?: boolean;
}) {
  const [profile, setProfile] = useState<LearningProfile>(emptyProfile);
  const [preferences, setPreferences] = useState<Preferences | null>(null);
  const [topics, setTopics] = useState<Topic[]>([]);
  const [state, setState] = useState<
    "loading" | "ready" | "saving" | "saved" | "error"
  >("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    Promise.all([
      api<{ profile: LearningProfile; preferences: Preferences }>("/profile"),
      api<{ topics: Topic[] }>("/topics"),
    ])
      .then(([profileData, topicData]) => {
        setProfile(profileData.profile);
        setPreferences(profileData.preferences);
        setTopics(topicData.topics ?? []);
        setState("ready");
      })
      .catch((reason: Error) => {
        setMessage(reason.message);
        setState("error");
      });
  }, []);

  const lanes = useMemo(() => {
    const values = new Map<string, Topic[]>();
    for (const topic of topics)
      values.set(topic.lane, [...(values.get(topic.lane) ?? []), topic]);
    return [...values.entries()];
  }, [topics]);

  if (!preferences || state === "loading")
    return (
      <div className="page-stack">
        <span className="loading-line" />
        <p className="muted-copy">Loading your private configuration…</p>
      </div>
    );

  function update<K extends keyof Preferences>(key: K, value: Preferences[K]) {
    setPreferences((current) =>
      current ? { ...current, [key]: value } : current,
    );
    setState("ready");
  }
  function setList(
    key: keyof Pick<
      LearningProfile,
      "goals" | "target_roles" | "current_technologies" | "target_technologies"
    >,
    value: string,
  ) {
    setProfile((current) => ({
      ...current,
      [key]: value
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean),
    }));
    setState("ready");
  }
  function changeCount(count: number) {
    setPreferences((current) =>
      current
        ? {
            ...current,
            lessons_per_day: count,
            delivery_times: timePresets[count],
          }
        : current,
    );
    setState("ready");
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
    setState("ready");
  }
  async function save(event: React.FormEvent) {
    event.preventDefault();
    setState("saving");
    setMessage("");
    try {
      await api<LearningProfile>("/profile", {
        method: "PUT",
        body: JSON.stringify(profile),
      });
      await api<Preferences>("/preferences", {
        method: "PUT",
        body: JSON.stringify(preferences),
      });
      setState("saved");
      setMessage("Your profile, topics, and delivery rhythm are saved.");
      if (onboarding) window.location.assign("/overview");
    } catch (reason) {
      setState("error");
      setMessage(
        reason instanceof Error
          ? reason.message
          : "Settings could not be saved.",
      );
    }
  }

  return (
    <form
      className="page-stack settings-page"
      onSubmit={(event) => void save(event)}
    >
      <PageHeader
        title={onboarding ? "Set up Coreloop" : "Learning settings"}
        description={
          onboarding
            ? "Start with useful defaults. Every choice can be changed later without losing queued lessons."
            : "Control lesson depth, topic selection, delivery rhythm, radar, and your curriculum context."
        }
        action={
          <button
            className="button button-primary"
            disabled={state === "saving"}
            type="submit"
          >
            {state === "saving"
              ? "Saving…"
              : onboarding
                ? "Save and start"
                : "Save changes"}
          </button>
        }
      />
      {message ? (
        <section
          className={`notice ${state === "error" ? "notice-error" : "notice-success"}`}
          role={state === "error" ? "alert" : "status"}
        >
          <div>
            <strong>
              {state === "error" ? "Could not save" : "Configuration saved"}
            </strong>
            <p>{message}</p>
          </div>
        </section>
      ) : null}
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
              Plain technical English, usefulness first, mostly theoretical.
            </small>
          </label>
          <label className="field">
            <span>Current level</span>
            <select
              value={profile.current_level}
              onChange={(event) =>
                setProfile({
                  ...profile,
                  current_level: event.target
                    .value as LearningProfile["current_level"],
                })
              }
            >
              <option value="beginner">Beginner</option>
              <option value="intermediate">Intermediate</option>
              <option value="advanced">Advanced</option>
            </select>
          </label>
          <label className="field">
            <span>Recall mode</span>
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
              <option value="light">Light</option>
              <option value="standard">Standard</option>
            </select>
          </label>
        </div>
      </section>
      <section className="settings-section">
        <div className="settings-heading">
          <span className="section-number">02</span>
          <h2>Delivery rhythm</h2>
          <p>
            Times use India Standard Time. Full lessons arrive as ordered
            Telegram messages.
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
              <small>Keep lesson delivery active on Saturday and Sunday.</small>
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
              <strong>Current-tech radar</strong>
              <small>
                Receive source-backed technology updates independently of
                lessons.
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
            <span>Radar updates per day</span>
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
              <strong>Send Radar updates on weekends</strong>
              <small>
                Keep technology news active on Saturday and Sunday, independent
                of lesson delivery.
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
            Choose the areas the planner may use. It continues a coherent theme
            and stores decisions to avoid frequent repetition.
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
            <p className="field-error">
              Choose at least one topic before saving.
            </p>
          ) : null}
        </div>
      </section>
      <section className="settings-section">
        <div className="settings-heading">
          <span className="section-number">04</span>
          <h2>Curriculum context</h2>
          <p>
            Short lists help the deterministic planner choose useful starting
            points. They are never included in shared lesson cache keys.
          </p>
        </div>
        <div className="form-grid">
          <label className="field field-wide">
            <span>Goals</span>
            <input
              value={profile.goals.join(", ")}
              onChange={(event) => setList("goals", event.target.value)}
              placeholder="Cloud engineering, stronger system design interviews"
            />
            <small>Separate items with commas.</small>
          </label>
          <label className="field">
            <span>Target roles</span>
            <input
              value={profile.target_roles.join(", ")}
              onChange={(event) => setList("target_roles", event.target.value)}
              placeholder="Backend engineer"
            />
          </label>
          <label className="field">
            <span>Current technologies</span>
            <input
              value={profile.current_technologies.join(", ")}
              onChange={(event) =>
                setList("current_technologies", event.target.value)
              }
              placeholder="Go, Bitcoin, PostgreSQL"
            />
          </label>
          <label className="field field-wide">
            <span>Technologies to learn</span>
            <input
              value={profile.target_technologies.join(", ")}
              onChange={(event) =>
                setList("target_technologies", event.target.value)
              }
              placeholder="Terraform, Kubernetes, production AI"
            />
          </label>
        </div>
      </section>
      <section className="save-bar">
        <div>
          <StatusPill
            tone={
              state === "saved"
                ? "ready"
                : state === "error"
                  ? "error"
                  : "neutral"
            }
          >
            {state === "saved"
              ? "Saved"
              : state === "error"
                ? "Needs attention"
                : "Ready to save"}
          </StatusPill>
          <p>
            Automatic AI routing is Groq → Gemini. OpenAI always requires a
            separate owner action.
          </p>
        </div>
        <button
          className="button button-primary"
          disabled={state === "saving" || preferences.topic_ids.length === 0}
          type="submit"
        >
          {state === "saving" ? "Saving…" : "Save configuration"}
        </button>
      </section>
    </form>
  );
}
