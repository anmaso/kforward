package cmd

import (
	"context"
	"fmt"

	"github.com/amarin/kforward/internal/selector"
)

func runMenu(ctx context.Context) error {
	action, err := selector.ChooseAction()
	if err != nil {
		return err
	}

	switch action {
	case selector.ActionStatus:
		return runStatus(ctx, true)
	case selector.ActionAdd:
		return runAdd(ctx, nil, true)
	case selector.ActionRemove:
		return runRemove()
	}
	return fmt.Errorf("unknown action %q", action)
}
