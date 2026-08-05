package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"coreloop/backend/internal/content"
)

func TestLessonPlanContextCarriesThemeProgression(t *testing.T) {
	plan := LessonPlan{
		Position:      5,
		Prerequisites: []string{"Transactions"},
		Topic: Topic{
			ID:         "topic_databases",
			Title:      "Database replication",
			Objectives: []string{"one", "two", "three", "four"},
		},
	}

	lessonContext := plan.Context()
	if !reflect.DeepEqual(lessonContext.Objectives, []string{"one"}) {
		t.Fatalf("selected objectives = %#v", lessonContext.Objectives)
	}
	if !reflect.DeepEqual(lessonContext.CoveredObjectives, []string{"one", "two", "three", "four"}) {
		t.Fatalf("covered objectives = %#v", lessonContext.CoveredObjectives)
	}
	if !reflect.DeepEqual(lessonContext.Prerequisites, []string{"Transactions"}) {
		t.Fatalf("prerequisites = %#v", lessonContext.Prerequisites)
	}
}

func TestSaveGeneratedLessonRejectsIncompleteContentBeforeDatabaseWrites(t *testing.T) {
	dataStore := &Store{}
	plan := LessonPlan{
		Preferences: Preferences{LessonMinutes: 30},
	}
	generated := content.Generated{Draft: content.LessonDraft{
		Title: "A partial lesson", EstimatedMinutes: 30,
		Motivation: "Only the opening was generated.",
	}}

	_, _, err := dataStore.SaveGeneratedLesson(
		context.Background(),
		plan,
		generated,
		[]string{"<b>A partial lesson</b>"},
		"cache-key",
		time.Now(),
	)
	if !errors.Is(err, ErrIncompleteLesson) {
		t.Fatalf("save error = %v, want ErrIncompleteLesson", err)
	}
}
