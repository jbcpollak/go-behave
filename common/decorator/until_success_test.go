package decorator

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jbcpollak/greenstalk"
	"github.com/jbcpollak/greenstalk/common/action"
	"github.com/jbcpollak/greenstalk/common/composite"
	"github.com/jbcpollak/greenstalk/core"
	"github.com/jbcpollak/greenstalk/util"
	"github.com/rs/zerolog/log"
)

func TestUntilSuccess(t *testing.T) {
	var wg sync.WaitGroup

	// Synchronous, so does not need to be cancelled.
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan bool)

	countChan := make(chan uint)

	child := action.Counter[core.EmptyBlackboard](action.CounterParams{
		BaseParams: "Counter",
		Limit:      3,
		CountChan:  countChan,
	})

	untilFailure := UntilSuccess(Inverter(child))

	params := action.SignallerParams[bool]{
		BaseParams: "Signaller",

		Channel: sigChan,
		Signal:  true,
	}
	signaller := action.Signaller[core.EmptyBlackboard](params)

	var testSequence = composite.Sequence(
		untilFailure,
		action.Succeed[core.EmptyBlackboard](action.SucceedParams{
			BaseParams: "Success",
		}),
		signaller,
	)

	tree, err := greenstalk.NewBehaviorTree(
		testSequence,
		core.EmptyBlackboard{},
		greenstalk.WithContext[core.EmptyBlackboard](ctx),
		greenstalk.WithVisitor(util.PrintTreeInColor[core.EmptyBlackboard]),
	)
	if err != nil {
		panic(err)
	}

	evt := core.DefaultEvent{}
	wg.Add(1)
	go func() {
		tree.EventLoop(evt)
		wg.Done()
	}()

	d := time.Duration(100) * time.Millisecond

LOOP:
	for {
		select {
		case c := <-countChan:
			log.Info().Msgf("got count %v", c)
		case c := <-sigChan:
			log.Info().Msgf("loop is finished %v", c)

			break LOOP
		case <-time.After(d):
			t.Errorf("Timeout after delaying %v", d)
		}
	}

	cancel()
	wg.Wait()
	status := tree.Root.Result().Status()
	if status != core.StatusSuccess {
		t.Errorf("Unexpectedly got %v", status)
	}
}
