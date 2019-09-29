package statepattern

// 参考 https://github.com/jackc/pgx/blob/master/internal/sanitize/sanitize.go#L101

import (
	"context"
	"errors"
)

var ErrIllegalStateTransfer = errors.New("illegal state transfer")

var stateHandlers = map[string]StateDesc{
	"init": {
		CanTransferTo: map[string]struct{}{
			"fail":      {},
			"auditPass": {},
			"auditing":  {},
		},
		Handler: new(InitStateHandler),
	},
	"auditing": {
		CanTransferTo: map[string]struct{}{
			"fail":      {},
			"auditPass": {},
		},
	},
	"auditPass": {
		CanTransferTo: map[string]struct{}{
			"fail":          {},
			"deductSuccess": {},
		},
		Handler: new(AuditPassStateHandler),
	},
	"deductSuccess": {
		CanTransferTo: map[string]struct{}{
			"success": {},
		},
		Handler: new(DeductSuccessStateHandler),
	},
}

type StateDesc struct {
	CanTransferTo map[string]struct{}
	Handler       TransferStateHandler
}

type TransferStateHandler interface {
	IdempotentHandle(ctx context.Context, transfer *Transfer) (stopped bool, err error)
}

// Transfer 转账记录
type Transfer struct {
	// 省略其它的信息
	State string

	stateHandlerMap map[string]StateDesc
}

func (t *Transfer) AfterLoad() {
	t.stateHandlerMap = make(map[string]StateDesc, len(stateHandlers))
	for _, desc := range stateHandlers {
		t.stateHandlerMap[t.State] = desc
	}
}

func (t *Transfer) TransitTo(state string) error {
	desc := t.stateHandlerMap[t.State]
	if _, exist := desc.CanTransferTo[state]; !exist {
		return ErrIllegalStateTransfer
	}
	t.State = state
	return nil

}

// Save itself
func (t *Transfer) Save() error {
	return nil
}

func (t *Transfer) getState() TransferStateHandler {
	return t.stateHandlerMap[t.State].Handler
}

func (t *Transfer) Process(ctx context.Context) error {
	for {
		handler := t.getState()
		if handler == nil {
			return nil
		}

		shouldStop, err := handler.IdempotentHandle(ctx, t)
		if err != nil {
			// TODO: maybe we can add some retry strategy
			return err
		}

		if err := t.Save(); err != nil {
			return err
		}

		if shouldStop {
			return nil
		}

	}
}

type InitStateHandler struct {
	initCase string
}

func (h *InitStateHandler) IdempotentHandle(ctx context.Context, transfer *Transfer) (bool, error) {
	switch h.initCase {
	case "fail":
		return true, transfer.TransitTo("fail")
	case "success":
		return false, transfer.TransitTo("auditPass")
	default: // pending
		return true, transfer.TransitTo("auditing")
	}
}

type AuditingStateHandler struct {
	auditingCase string
}

func (h *AuditingStateHandler) IdempotentHandle(ctx context.Context, transfer *Transfer) (bool, error) {
	switch h.auditingCase {
	case "fail":
		return true, transfer.TransitTo("fail")
	case "success":
		return false, transfer.TransitTo("auditPass")
	default: // still auditing
		return true, nil
	}
}

type AuditPassStateHandler struct {
	deductCase string
}

func (h *AuditPassStateHandler) IdempotentHandle(ctx context.Context, transfer *Transfer) (bool, error) {
	switch h.deductCase {
	case "fail":
		return false, transfer.TransitTo("fail")
	case "tempError":
		return true, nil
	default: // transfer success
		return true, transfer.TransitTo("deductSuccess")
	}

}

type DeductSuccessStateHandler struct {
}

func (h *DeductSuccessStateHandler) IdempotentHandle(ctx context.Context, transfer *Transfer) (bool, error) {

	return true, transfer.TransitTo("success")
}
