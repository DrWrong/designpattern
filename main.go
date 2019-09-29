package main

// type BroadCast struct {
//	Status string
//	fsm    *fsm.FSM
// }

// func (b *BroadCast) AfterLoad() {
//	b.fsm = fsm.NewFSM(
//		b.Status,
//		fsm.Events{
//			{Name: "init", Src: []string{"init"}, Dst: "checking"},
//			{Name: "refuse", Src: []string{"checking"}, Dst: "refused"},
//			{Name: "pass"},
//		},
//	)
// }
