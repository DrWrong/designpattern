package persistablestatemachine

import (
	"context"
	"fmt"
)

type IllegalTransitError struct {
	SrcState string
	DstState string
}

func (e *IllegalTransitError) Error() string {
	return fmt.Sprintf("Illegal transit state src: %s dst: %s", e.SrcState, e.DstState)
}

type StateData interface {
	GetState() string
	SetState(state string)
}

type StateRepository interface {
	Save(ctx context.Context, data StateData) error
	FindUnfinished(ctx context.Context) ([]StateData, error)
}

type StateHandler interface {
	IdempotentHandle(ctx context.Context, stateContext *StateContext) (stopped bool, err error)
}

type StateDesc struct {
	CanTransitTo map[string]struct{}
	Handler      StateHandler
}

type StateContext struct {
	Data StateData

	stateHandlerMap map[string]StateDesc
	repository      StateRepository
}

func (s *StateContext) TransitTo(state string) error {
	desc := s.stateHandlerMap[s.Data.GetState()]
	if _, exist := desc.CanTransitTo[state]; !exist {

		return &IllegalTransitError{
			SrcState: s.Data.GetState(),
			DstState: state,
		}
	}
	s.Data.SetState(state)
	return nil
}

func (s *StateContext) getHandler() StateHandler {
	return s.stateHandlerMap[s.Data.GetState()].Handler
}

func (s *StateContext) Process(ctx context.Context) error {
	for {
		handler := s.getHandler()
		if handler == nil {
			return nil
		}
		shouldStop, err := handler.IdempotentHandle(ctx, s)
		if err != nil {
			// TODO: maybe we can add some retry strategy
			return err
		}
		if err := s.repository.Save(ctx, s.Data); err != nil {
			return err
		}

		if shouldStop {
			return nil
		}
	}
}

type StateService struct {
	Repository StateRepository
	HandlerMap map[string]StateDesc
}

func (s *StateService) Process(ctx context.Context, data StateData) error {
	if err := s.Repository.Save(ctx, data); err != nil {
		return err
	}

	stateContext := &StateContext{
		Data:            data,
		stateHandlerMap: s.HandlerMap,
		repository:      s.Repository,
	}
	return stateContext.Process(ctx)
}

func (s *StateService) FindAndRecover(ctx context.Context) error {
	records, err := s.Repository.FindUnfinished(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := s.Process(ctx, record); err != nil {
			return err
		}
	}

	return nil
}
