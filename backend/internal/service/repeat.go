package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"kairo/internal/domain"
)

// repeatPrefix kennzeichnet die SourceKey der Wiederholungen: repeat:<task_id>@<YYYY-MM-DD>+<Tage>.
// Der Tag im Schlüssel macht einen zweiten Klick am selben Tag wirkungslos; an einem späteren Tag
// beginnt eine neue Runde. CancelUntouched(explainPrefix) trifft sie nicht: verpasste bleiben offen.
const repeatPrefix = "repeat:"

// RepeatDays sind die Abstände der Wiederholungen in Tagen ab heute.
var RepeatDays = []int{1, 3, 7, 14}

// PlanRepetitions legt zu einer Task Wiederholungen „Wiederholen: <Titel>“ an, je eine für den
// Lerntag + RepeatDays, im selben Projekt. Lerntag ist heute, bei einer noch nicht begonnenen, erst
// später geplanten Task deren Tag (wer sie schon begonnen oder erledigt hat, hat heute gelernt). Zurück kommen nur die neu angelegten; leer heißt: schon geplant.
func (s *TaskService) PlanRepetitions(ctx context.Context, id string) ([]domain.Task, error) {
	src, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if src.SourceKey != nil && strings.HasPrefix(*src.SourceKey, repeatPrefix) {
		return nil, fmt.Errorf("%w: eine Wiederholung lässt sich nicht noch einmal wiederholen", domain.ErrInvalid)
	}
	now := s.now()
	today := now.In(s.loc)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, s.loc)
	if src.PlannedDate != nil && (src.Status == domain.TaskBacklog || src.Status == domain.TaskPlanned) {
		if d, err := time.ParseInLocation("2006-01-02", *src.PlannedDate, s.loc); err == nil && d.After(today) {
			today = d
		}
	}
	title := "Wiederholen: " + strings.TrimPrefix(src.Title, explainTitle)
	var created []domain.Task
	for _, n := range RepeatDays {
		// time.Date statt Add(24h*n): sonst verrutscht der Tag bei der Zeitumstellung.
		date := time.Date(today.Year(), today.Month(), today.Day()+n, 0, 0, 0, 0, s.loc).Format("2006-01-02")
		key := repeatPrefix + src.ID + "@" + today.Format("2006-01-02") + "+" + strconv.Itoa(n)
		t := domain.Task{
			ID:               domain.NewID(),
			Title:            title,
			Status:           domain.TaskPlanned,
			Priority:         domain.PriorityMedium,
			EstimatedMinutes: 10,
			PlannedDate:      &date,
			ProjectID:        src.ProjectID,
			SourceKey:        &key,
			CreatedAt:        now.UTC(),
			UpdatedAt:        now.UTC(),
		}
		ok, err := s.CreateGenerated(ctx, t)
		if err != nil {
			return created, err
		}
		if ok {
			created = append(created, t)
		}
	}
	return created, nil
}
