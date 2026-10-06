package service

import "kairo/internal/domain"

// Publisher verteilt Ereignisse an Echtzeit-Clients. Publish darf nicht blockieren.
type Publisher interface {
	Publish(e domain.Event)
}

// publisher ist in die Services eingebettet. Ohne SetPublisher passiert nichts.
type publisher struct{ pub Publisher }

// SetPublisher legt fest, wohin der Service Ereignisse meldet.
func (p *publisher) SetPublisher(pub Publisher) { p.pub = pub }

func (p *publisher) emit(es ...domain.Event) {
	if p.pub == nil {
		return
	}
	for _, e := range es {
		p.pub.Publish(e)
	}
}
