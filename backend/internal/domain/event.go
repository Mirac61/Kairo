package domain

// EventType benennt ein Ereignis für Echtzeit-Clients.
type EventType string

const (
	EventTaskCreated   EventType = "TASK_CREATED"
	EventTaskUpdated   EventType = "TASK_UPDATED"
	EventTaskDeleted   EventType = "TASK_DELETED"
	EventTaskStarted   EventType = "TASK_STARTED"
	EventTaskPaused    EventType = "TASK_PAUSED"
	EventTaskCompleted EventType = "TASK_COMPLETED"

	EventTimerStarted EventType = "TIMER_STARTED"
	EventTimerStopped EventType = "TIMER_STOPPED"

	EventProjectCreated EventType = "PROJECT_CREATED"
	EventProjectUpdated EventType = "PROJECT_UPDATED"
	EventProjectDeleted EventType = "PROJECT_DELETED"

	EventHabitCreated     EventType = "HABIT_CREATED"
	EventHabitUpdated     EventType = "HABIT_UPDATED"
	EventHabitDeleted     EventType = "HABIT_DELETED"
	EventHabitCompleted   EventType = "HABIT_COMPLETED"
	EventHabitUncompleted EventType = "HABIT_UNCOMPLETED"

	EventCalendarEventCreated EventType = "CALENDAR_EVENT_CREATED"
	EventCalendarEventUpdated EventType = "CALENDAR_EVENT_UPDATED"
	EventCalendarEventDeleted EventType = "CALENDAR_EVENT_DELETED"
)

// Event meldet, dass sich etwas geändert hat. Es trägt nur IDs; Clients
// lesen den neuen Stand per REST nach.
type Event struct {
	Type   EventType
	ID     string // das betroffene Objekt (bei Timer-Events der Zeiteintrag)
	TaskID string // bei Timer-Events die zugehörige Task
}
