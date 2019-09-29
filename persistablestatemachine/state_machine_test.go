package persistablestatemachine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/onsi/gomega"
)

type FakeStateData struct {
	ID            string
	ShouldFailure bool
	ShouldHang    bool

	State string
}

func (data *FakeStateData) GetState() string {
	return data.State
}

func (data *FakeStateData) SetState(state string) {
	data.State = state
}

func (data *FakeStateData) IsFinished() bool {
	return data.State == "success" || data.State == "fail"
}

type InMemoryStateRepository map[string]*FakeStateData

func (r InMemoryStateRepository) Save(ctx context.Context, data StateData) error {
	fakeStateData, ok := data.(*FakeStateData)
	if !ok {
		return errors.New("illegal repository")
	}
	r[fakeStateData.ID] = fakeStateData
	return nil
}

func (r InMemoryStateRepository) FindUnfinished(ctx context.Context) ([]StateData, error) {
	records := make([]StateData, 0, len(r))
	for _, data := range r {
		if !data.IsFinished() {
			records = append(records, data)
		}
	}

	return records, nil
}

type StepAHandler struct {
}

func (s *StepAHandler) IdempotentHandle(ctx context.Context, stateContext *StateContext) (stopped bool, err error) {
	fakeData, ok := stateContext.Data.(*FakeStateData)
	if !ok {
		return true, fmt.Errorf("not prop data")
	}
	if fakeData.ShouldFailure {
		fakeData.ShouldFailure = false
		return true, fmt.Errorf("mock failure")
	}
	return false, stateContext.TransitTo("stepB")
}

type StepBHandler struct {
}

func (s *StepBHandler) IdempotentHandle(ctx context.Context, stateContext *StateContext) (stopped bool, err error) {
	fakeData, ok := stateContext.Data.(*FakeStateData)
	fmt.Println("process", fakeData)
	if !ok {
		return true, fmt.Errorf("not prop data")
	}
	if fakeData.ShouldHang {
		fakeData.ShouldHang = false
		return true, nil
	}
	fmt.Println("going to transit to success")
	return false, stateContext.TransitTo("success")

}

func TestStateMachine(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	s := &StateService{
		Repository: make(InMemoryStateRepository),
		HandlerMap: map[string]StateDesc{
			"stepA": {
				CanTransitTo: map[string]struct{}{
					"stepB": {},
				},
				Handler: new(StepAHandler),
			},
			"stepB": {
				CanTransitTo: map[string]struct{}{
					"success": {},
				},
				Handler: new(StepBHandler),
			},
		},
	}
	data1 := &FakeStateData{
		ID:            "1",
		ShouldFailure: true,
		ShouldHang:    true,
		State:         "stepA",
	}

	data2 := &FakeStateData{
		ID:            "2",
		ShouldFailure: false,
		ShouldHang:    true,
		State:         "stepA",
	}

	data3 := &FakeStateData{
		ID:            "3",
		ShouldFailure: false,
		ShouldHang:    false,
		State:         "stepA",
	}

	err := s.Process(context.Background(), data1)
	g.Expect(err).Should(gomega.HaveOccurred())
	g.Expect(data1.State).Should(gomega.Equal("stepA"))

	err = s.Process(context.Background(), data2)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(data2.State).Should(gomega.Equal("stepB"))

	err = s.Process(context.Background(), data3)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(data3.State).Should(gomega.Equal("success"))

	err = s.FindAndRecover(context.Background())
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(data1.State).Should(gomega.Equal("stepB"))
	g.Expect(data2.State).Should(gomega.Equal("success"))
}
