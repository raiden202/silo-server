package historyimport

import "context"

type Observer interface {
	RunUpdated(Run)
}

func (s *Service) AddObserver(observer Observer) {
	if s == nil || observer == nil {
		return
	}

	s.runCancelsMu.Lock()
	s.observers = append(s.observers, observer)
	s.runCancelsMu.Unlock()
}

func (s *Service) notifyRun(run *Run) {
	if s == nil || run == nil {
		return
	}

	s.runCancelsMu.Lock()
	observers := append([]Observer(nil), s.observers...)
	s.runCancelsMu.Unlock()
	for _, observer := range observers {
		observer.RunUpdated(*run)
	}
}

func (s *Service) notifyRunByID(ctx context.Context, runID string) {
	if s == nil || runID == "" {
		return
	}
	run, err := s.repo.GetRunByID(ctx, runID)
	if err != nil {
		return
	}
	s.notifyRun(run)
}
