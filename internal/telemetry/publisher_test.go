package telemetry

import "testing"

func TestLivePublisherSequenceReplayDedupeAndShutdown(t *testing.T) {
	publisher := NewLivePublisher(2, 2)
	if got := publisher.Publish(LiveEvent{Event: "area_changed", AreaID: 1, Area: "Town"}); got != 1 {
		t.Fatalf("first sequence = %d", got)
	}
	if got := publisher.Publish(LiveEvent{Event: "area_changed", AreaID: 1, Area: "Town"}); got != 1 {
		t.Fatalf("deduplicated sequence = %d", got)
	}
	publisher.Publish(LiveEvent{Event: "run_started", RunID: "run-1"})
	publisher.Publish(LiveEvent{Event: "run_completed", RunID: "run-1"})
	replay, subscription := publisher.Subscribe(1)
	defer subscription.Close()
	if len(replay) != 2 || replay[0].Sequence != 2 || replay[1].Sequence != 3 {
		t.Fatalf("replay = %+v", replay)
	}
	publisher.Close()
	if _, ok := <-subscription.Events; ok {
		t.Fatal("subscription remained open after shutdown")
	}
}

func TestLivePublisherDropsOnlySlowClient(t *testing.T) {
	publisher := NewLivePublisher(8, 1)
	_, slow := publisher.Subscribe(publisher.Sequence())
	publisher.Publish(LiveEvent{Event: "one"})
	publisher.Publish(LiveEvent{Event: "two"})
	if publisher.Sequence() != 2 {
		t.Fatalf("publisher stopped at sequence %d", publisher.Sequence())
	}
	first, ok := <-slow.Events
	if !ok || first.Event != "one" {
		t.Fatalf("slow first event = %+v open=%t", first, ok)
	}
	if _, ok := <-slow.Events; ok {
		t.Fatal("slow client was not disconnected after overflow")
	}
}
