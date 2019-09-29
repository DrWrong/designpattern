package fsm

import "github.com/looplab/fsm"

type Transfer struct {
	State string
	fsm   *fsm.FSM
}

func (t *Transfer) AfterLoad() {
	t.fsm = fsm.NewFSM(
		t.State,
		fsm.Events{
			{Name: "auditFail", Src: []string{"init", "auditing"}, Dst: "fail"},
			{Name: "auditSuccess", Src: []string{"init", "auditing"}, Dst: "auditPass"},
			{Name: "auditingProcessing", Src: []string{"init"}, Dst: "auditing"},
			{Name: "deductSuccess", Src: []string{"auditPass"}, Dst: "deductSuccess"},
			{Name: "deductFail", Src: []string{"auditPass"}, Dst: "fail"},
			{Name: "addSuccess", Src: []string{"auditPass"}, Dst: "success"},
		},
		fsm.Callbacks{},
	)
}

func (t *Transfer) Process() {
	t.fsm.Event()
}
