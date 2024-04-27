package greenstalk

import (
	"context"
	"testing"
	"time"

	"github.com/jbcpollak/greenstalk/core"
	"github.com/jbcpollak/greenstalk/util"
	"github.com/rs/zerolog/log"

	// Use dot imports to make a tree definition look nice.
	// Be careful when doing this! These packages export
	// common word identifiers such as "Fail" and "Sequence".
	. "github.com/jbcpollak/greenstalk/common/action"
	. "github.com/jbcpollak/greenstalk/common/composite"
	. "github.com/jbcpollak/greenstalk/common/decorator"
)

type TestBlackboard struct {
	id    int
	count uint
}

var n = 0

func untilTwo(status core.ResultDetails) bool {
	n++
	return n == 2
}

var synchronousRoot = Sequence[TestBlackboard](
	RepeatUntil(RepeatUntilParams{
		BaseParams: "RepeatUntilTwo",
		Until:      untilTwo,
	}, Fail[TestBlackboard](FailParams{})),
	Succeed[TestBlackboard](SucceedParams{}),
)

func TestUpdate(t *testing.T) {
	log.Info().Msg("Testing synchronous tree...")

	// Synchronous, so does not need to be cancelled.
	ctx := context.Background()

	tree, err := NewBehaviorTree[TestBlackboard](
		synchronousRoot,
		TestBlackboard{id: 42},
		WithContext[TestBlackboard](ctx),
		WithVisitor(util.PrintTreeInColor[TestBlackboard]),
	)
	if err != nil {
		panic(err)
	}

	for {
		evt := core.DefaultEvent{}
		status := tree.Update(evt)
		if status == core.StatusSuccess {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	log.Info().Msg("Done!")
}

var countChan = make(chan uint)

var delay = 100
var asynchronousRoot = Sequence(
	// Repeater(core.Params{"n": 2}, Fail[TestBlackboard](nil, nil)),
	AsyncDelayer[TestBlackboard](
		AsyncDelayerParams{
			BaseParams: core.BaseParams("First"),
			Delay:      time.Duration(delay) * time.Millisecond,
		},
		Counter[TestBlackboard](CounterParams{
			BaseParams: "First Counter",
			Limit:      10,
			CountChan:  countChan,
		}),
	),
	AsyncDelayer[TestBlackboard](
		AsyncDelayerParams{
			BaseParams: core.BaseParams("Second"),
			Delay:      time.Duration(delay) * time.Millisecond,
		},
		Counter[TestBlackboard](CounterParams{
			BaseParams: "Second Counter",
			Limit:      10,
			CountChan:  countChan,
		}),
	),
)

func getCount(d time.Duration) (uint, bool) {
	select {
	case c := <-countChan:
		log.Info().Msgf("got count %v", c)
		return c, true
	case <-time.After(d):
		log.Info().Msgf("Timeout after delaying %v", d)
		return 0, false
	}
}

func TestEventLoop(t *testing.T) {
	log.Info().Msg("Testing asynchronous tree...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bb := TestBlackboard{id: 42, count: 0}
	tree, err := NewBehaviorTree(
		asynchronousRoot, bb,
		WithContext[TestBlackboard](ctx),
		WithVisitor(util.PrintTreeInColor[TestBlackboard]),
	)
	if err != nil {
		panic(err)
	}

	evt := core.DefaultEvent{}
	go tree.EventLoop(evt)

	// Wait half the delay and verify no value sent
	first_halfway, ok := getCount(time.Duration(delay/2) * time.Millisecond)
	if ok {
		t.Errorf("Unexpectedly got count %d", first_halfway)
	} else {
		log.Info().Msg("Halfway through first delay counter correctly is 0")
	}

	// Sleep a bit more
	first_after, ok := getCount(time.Duration(delay/2+10) * time.Millisecond)
	if ok && first_after != 1 {
		t.Errorf("Expected count to be 1, got %d", first_after)
	} else {
		log.Info().Msg("After first delay, counter is 1")
	}

	// Wait half the delay and verify value is 0
	second_halfway, ok := getCount(time.Duration(delay/2) * time.Millisecond)
	if ok {
		t.Errorf("Unexpectedly got count %d", second_halfway)
	} else {
		log.Info().Msg("Halfway through second delay counter correctly is 1")
	}

	// Shut it _down_
	log.Info().Msg("Shutting down...")
	cancel()

	after_cancel, ok := getCount(time.Duration(delay/2) * time.Millisecond)

	// Ensure we shut down before the second tick
	if ok {
		t.Errorf("Expected to shut down before second tick but got %d", after_cancel)
	}

	log.Info().Msg("Done!")
}
