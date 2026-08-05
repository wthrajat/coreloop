package store

import (
	"reflect"
	"testing"
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
