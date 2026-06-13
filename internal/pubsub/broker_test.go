package pubsub

import (
	"sync"
	"testing"
	"time"
)

func TestNewBroker(t *testing.T) {
	b := NewBroker(DefaultConfig())
	if b == nil {
		t.Fatal("NewBroker returned nil")
	}
	if b.ConsumerCount() != 0 {
		t.Errorf("expected 0 consumers, got %d", b.ConsumerCount())
	}
	if b.SubscriptionCount() != 0 {
		t.Errorf("expected 0 subscriptions, got %d", b.SubscriptionCount())
	}
	b.Stop()
}

func TestNewBrokerConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	b := NewBroker(cfg)
	if b.cfg.AckTimeout != defaultAckTimeout {
		t.Errorf("expected default AckTimeout %v, got %v", defaultAckTimeout, b.cfg.AckTimeout)
	}
	if b.cfg.MaxRetry != defaultMaxRetry {
		t.Errorf("expected default MaxRetry %d, got %d", defaultMaxRetry, b.cfg.MaxRetry)
	}
	if b.cfg.MaxUnacked != defaultMaxUnacked {
		t.Errorf("expected default MaxUnacked %d, got %d", defaultMaxUnacked, b.cfg.MaxUnacked)
	}
	b.Stop()

	cfg2 := Config{MaxRetry: -1}
	b2 := NewBroker(cfg2)
	if b2.cfg.MaxRetry != defaultMaxRetry {
		t.Errorf("expected -1 to map to default MaxRetry %d, got %d", defaultMaxRetry, b2.cfg.MaxRetry)
	}
	b2.Stop()
}

func TestAddConsumer(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, err := b.AddConsumer("c1")
	if err != nil {
		t.Fatalf("AddConsumer failed: %v", err)
	}
	if ch == nil {
		t.Fatal("channel is nil")
	}
	if b.ConsumerCount() != 1 {
		t.Errorf("expected 1 consumer, got %d", b.ConsumerCount())
	}

	_, err = b.AddConsumer("c1")
	if err != ErrConsumerExists {
		t.Errorf("expected ErrConsumerExists, got %v", err)
	}

	_, err = b.AddConsumer("")
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound, got %v", err)
	}
}

func TestRemoveConsumer(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")
	err := b.RemoveConsumer("c1")
	if err != nil {
		t.Fatalf("RemoveConsumer failed: %v", err)
	}
	if b.ConsumerCount() != 0 {
		t.Errorf("expected 0 consumers, got %d", b.ConsumerCount())
	}

	err = b.RemoveConsumer("c1")
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound, got %v", err)
	}
}

func TestSubscribeAndPublish(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	err := b.Subscribe("c1", "test.topic")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if b.SubscriptionCount() != 1 {
		t.Errorf("expected 1 subscription, got %d", b.SubscriptionCount())
	}

	msgID, err := b.Publish("test.topic", "hello")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if msgID == "" {
		t.Error("expected non-empty message ID")
	}

	select {
	case msg := <-ch:
		if msg == nil {
			t.Fatal("received nil message")
		}
		if msg.Topic != "test.topic" {
			t.Errorf("expected topic 'test.topic', got '%s'", msg.Topic)
		}
		if msg.Payload != "hello" {
			t.Errorf("expected payload 'hello', got '%v'", msg.Payload)
		}
		if msg.ID == "" {
			t.Error("expected non-empty message ID in received message")
		}
		if msg.RetryCount != 0 {
			t.Errorf("expected RetryCount 0, got %d", msg.RetryCount)
		}
		err = b.Ack("c1", msg.ID)
		if err != nil {
			t.Fatalf("Ack failed: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for message")
	}
}

func TestPublishNoSubscribers(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	msgID, err := b.Publish("test.topic", "hello")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if msgID == "" {
		t.Error("expected non-empty message ID")
	}
}

func TestSubscribeInvalidFilter(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")

	err := b.Subscribe("c1", "")
	if err != ErrTopicInvalid {
		t.Errorf("expected ErrTopicInvalid, got %v", err)
	}

	err = b.Subscribe("c1", "a..b")
	if err != ErrTopicInvalid {
		t.Errorf("expected ErrTopicInvalid, got %v", err)
	}

	err = b.Subscribe("c1", "a.#.b")
	if err != ErrTopicInvalid {
		t.Errorf("expected ErrTopicInvalid for # in middle, got %v", err)
	}

	err = b.Subscribe("c1", "a.b*")
	if err != ErrTopicInvalid {
		t.Errorf("expected ErrTopicInvalid for mixed chars, got %v", err)
	}
}

func TestPublishInvalidTopic(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	_, err := b.Publish("", "data")
	if err != ErrTopicInvalid {
		t.Errorf("expected ErrTopicInvalid, got %v", err)
	}

	_, err = b.Publish("a..b", "data")
	if err != ErrTopicInvalid {
		t.Errorf("expected ErrTopicInvalid, got %v", err)
	}
}

func TestSubscribeConsumerNotFound(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	err := b.Subscribe("nonexistent", "test.topic")
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound, got %v", err)
	}
}

func TestUnsubscribe(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	err := b.Unsubscribe("c1", "test.topic")
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
	if b.SubscriptionCount() != 0 {
		t.Errorf("expected 0 subscriptions, got %d", b.SubscriptionCount())
	}

	err = b.Unsubscribe("c1", "test.topic")
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}

	err = b.Unsubscribe("nonexistent", "test.topic")
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestMultipleConsumersSameTopic(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch1, _ := b.AddConsumer("c1")
	ch2, _ := b.AddConsumer("c2")
	b.Subscribe("c1", "test.topic")
	b.Subscribe("c2", "test.topic")

	msgID, _ := b.Publish("test.topic", "broadcast")

	var mu sync.Mutex
	received := make(map[string]bool)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		select {
		case msg := <-ch1:
			mu.Lock()
			received["c1"] = true
			mu.Unlock()
			b.Ack("c1", msg.ID)
		case <-time.After(100 * time.Millisecond):
			t.Error("c1 timed out")
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case msg := <-ch2:
			mu.Lock()
			received["c2"] = true
			mu.Unlock()
			b.Ack("c2", msg.ID)
		case <-time.After(100 * time.Millisecond):
			t.Error("c2 timed out")
		}
	}()

	wg.Wait()

	if !received["c1"] {
		t.Error("c1 did not receive message")
	}
	if !received["c2"] {
		t.Error("c2 did not receive message")
	}

	_ = msgID
}

func TestWildcardSingleLevel(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "sports.*")

	b.Publish("sports.football", "goal")

	select {
	case msg := <-ch:
		if msg.Topic != "sports.football" {
			t.Errorf("expected topic sports.football, got %s", msg.Topic)
		}
		if msg.Payload != "goal" {
			t.Errorf("expected payload goal, got %v", msg.Payload)
		}
		b.Ack("c1", msg.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for wildcard match")
	}
}

func TestWildcardSingleLevelNoMatch(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "sports.*")

	b.Publish("news.politics.usa", "news")

	select {
	case <-ch:
		t.Error("should not have received message for non-matching topic")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestWildcardMultiLevel(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "sports.#")

	b.Publish("sports.football.worldcup.final", "champions")

	select {
	case msg := <-ch:
		if msg.Topic != "sports.football.worldcup.final" {
			t.Errorf("expected topic sports.football.worldcup.final, got %s", msg.Topic)
		}
		b.Ack("c1", msg.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for multi-level wildcard")
	}
}

func TestWildcardMultiLevelShort(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "sports.#")

	b.Publish("sports", "root")

	select {
	case msg := <-ch:
		if msg.Topic != "sports" {
			t.Errorf("expected topic sports, got %s", msg.Topic)
		}
		b.Ack("c1", msg.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for multi-level wildcard (short match)")
	}
}

func TestWildcardHashOnly(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "#")

	b.Publish("any.arbitrary.topic.name", "data")

	select {
	case msg := <-ch:
		if msg.Topic != "any.arbitrary.topic.name" {
			t.Errorf("expected any.arbitrary.topic.name, got %s", msg.Topic)
		}
		b.Ack("c1", msg.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for # match")
	}
}

func TestMatchTopicDirect(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	tests := []struct {
		filter string
		topic  string
		want   bool
	}{
		{"a.b.c", "a.b.c", true},
		{"a.*.c", "a.b.c", true},
		{"a.*.c", "a.x.c", true},
		{"a.*.c", "a.b", false},
		{"a.*.c", "a.b.c.d", false},
		{"a.#", "a.b.c", true},
		{"a.#", "a", true},
		{"#", "anything", true},
		{"a.b.#", "a.b.c.d", true},
		{"a.b.#", "a.b", true},
		{"a.b.#", "a.c.b", false},
		{"*.b.*", "a.b.c", true},
		{"*.b.*", "a.b", false},
		{"*", "single", true},
		{"*", "a.b", false},
		{"a.*", "a.b", true},
		{"a.*", "a.b.c", false},
	}

	for _, tt := range tests {
		got := b.matchTopic(tt.filter, tt.topic)
		if got != tt.want {
			t.Errorf("matchTopic(%q, %q) = %v, want %v", tt.filter, tt.topic, got, tt.want)
		}
	}
}

func TestNack(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRetry = 2
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "nack-test")

	msg := <-ch
	if msg.RetryCount != 0 {
		t.Errorf("expected RetryCount 0, got %d", msg.RetryCount)
	}

	err := b.Nack("c1", msg.ID)
	if err != nil {
		t.Fatalf("Nack failed: %v", err)
	}

	msg2 := <-ch
	if msg2.RetryCount != 1 {
		t.Errorf("expected RetryCount 1, got %d", msg2.RetryCount)
	}
	b.Ack("c1", msg2.ID)
}

func TestNackMaxRetryDeadLetter(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRetry = 1
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "deadletter")

	msg1 := <-ch
	b.Nack("c1", msg1.ID)

	msg2 := <-ch
	b.Nack("c1", msg2.ID)

	dl := b.GetDeadLetters()
	if len(dl) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(dl))
	}
	if dl[0].RetryCount != 2 {
		t.Errorf("expected RetryCount 2 in dead letter, got %d", dl[0].RetryCount)
	}
	if b.DeadLetterCount() != 1 {
		t.Errorf("expected DeadLetterCount 1, got %d", b.DeadLetterCount())
	}
}

func TestClearDeadLetters(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRetry = 0
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "x")
	msg := <-ch
	b.Nack("c1", msg.ID)

	if b.DeadLetterCount() != 1 {
		t.Fatalf("expected 1 dead letter, got %d", b.DeadLetterCount())
	}

	b.ClearDeadLetters()
	if b.DeadLetterCount() != 0 {
		t.Errorf("expected 0 dead letters after clear, got %d", b.DeadLetterCount())
	}
}

func TestAckNotFound(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")
	err := b.Ack("c1", "nonexistent-msg")
	if err != ErrMessageNotFound {
		t.Errorf("expected ErrMessageNotFound, got %v", err)
	}

	err = b.Ack("nonexistent-consumer", "msg")
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound, got %v", err)
	}
}

func TestNackNotFound(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")
	err := b.Nack("c1", "nonexistent-msg")
	if err != ErrMessageNotFound {
		t.Errorf("expected ErrMessageNotFound, got %v", err)
	}
}

func TestProcessTimeouts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AckTimeout = 10 * time.Millisecond
	cfg.MaxRetry = 1
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "timeout-test")

	msg := <-ch
	firstID := msg.ID
	_ = firstID

	time.Sleep(50 * time.Millisecond)
	b.ProcessTimeouts()

	select {
	case msg2 := <-ch:
		if msg2.RetryCount != 1 {
			t.Errorf("expected RetryCount 1 after timeout, got %d", msg2.RetryCount)
		}
		b.Ack("c1", msg2.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for redelivery after timeout")
	}
}

func TestTimeoutLoop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AckTimeout = 10 * time.Millisecond
	cfg.MaxRetry = 0
	b := NewBroker(cfg)
	b.Start()
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "autotimeout")

	msg := <-ch
	_ = msg

	time.Sleep(200 * time.Millisecond)

	if b.DeadLetterCount() != 1 {
		t.Errorf("expected 1 dead letter after auto timeout, got %d", b.DeadLetterCount())
	}
}

func TestDurableSubscriptionOffline(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.SubscribeDurable("c1", "test.topic", true)

	b.DisconnectConsumer("c1")

	b.Publish("test.topic", "durmsg1")
	b.Publish("test.topic", "durmsg2")

	pending, _ := b.PendingCount("c1")
	if pending != 2 {
		t.Errorf("expected 2 pending messages, got %d", pending)
	}

	b.ReconnectConsumer("c1")

	count := 0
loop:
	for {
		select {
		case msg := <-ch:
			b.Ack("c1", msg.ID)
			count++
			if count >= 2 {
				break loop
			}
		case <-time.After(100 * time.Millisecond):
			break loop
		}
	}

	if count != 2 {
		t.Errorf("expected to receive 2 messages after reconnect, got %d", count)
	}
}

func TestDurableSubscriptionNotDurableLost(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.DisconnectConsumer("c1")

	b.Publish("test.topic", "lost")

	pending, _ := b.PendingCount("c1")
	if pending != 0 {
		t.Errorf("expected 0 pending for non-durable, got %d", pending)
	}

	b.ReconnectConsumer("c1")

	select {
	case <-ch:
		t.Error("should not receive message for non-durable subscription")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDisconnectReconnectConsumer(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")

	err := b.DisconnectConsumer("nonexistent")
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound, got %v", err)
	}

	err = b.ReconnectConsumer("nonexistent")
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound, got %v", err)
	}

	b.DisconnectConsumer("c1")
	err = b.DisconnectConsumer("c1")
	if err != nil {
		t.Errorf("disconnect already disconnected should be nil, got %v", err)
	}

	b.ReconnectConsumer("c1")
	err = b.ReconnectConsumer("c1")
	if err != nil {
		t.Errorf("reconnect already connected should be nil, got %v", err)
	}
}

func TestBackpressure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxUnacked = 2
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumerWithOptions("c1", 2)
	b.SubscribeDurable("c1", "test.topic", true)

	b.Publish("test.topic", "msg1")
	b.Publish("test.topic", "msg2")
	b.Publish("test.topic", "msg3")

	msg1 := <-ch
	msg2 := <-ch

	unacked, _ := b.UnackedCount("c1")
	if unacked != 2 {
		t.Errorf("expected 2 unacked, got %d", unacked)
	}

	pending, _ := b.PendingCount("c1")
	if pending != 1 {
		t.Errorf("expected 1 pending in durable buffer, got %d", pending)
	}

	b.Ack("c1", msg1.ID)

	select {
	case msg3 := <-ch:
		if msg3.Payload != "msg3" {
			t.Errorf("expected msg3 after ack, got %v", msg3.Payload)
		}
		b.Ack("c1", msg2.ID)
		b.Ack("c1", msg3.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for msg3 after ack released backpressure")
	}

	unacked, _ = b.UnackedCount("c1")
	if unacked != 0 {
		t.Errorf("expected 0 unacked after all acks, got %d", unacked)
	}

	pending, _ = b.PendingCount("c1")
	if pending != 0 {
		t.Errorf("expected 0 pending after all delivered, got %d", pending)
	}
}

func TestBrokerStopped(t *testing.T) {
	b := NewBroker(DefaultConfig())
	b.Stop()

	_, err := b.AddConsumer("c1")
	if err != ErrBrokerStopped {
		t.Errorf("expected ErrBrokerStopped, got %v", err)
	}

	_, err = b.Publish("test", "data")
	if err != ErrBrokerStopped {
		t.Errorf("expected ErrBrokerStopped for publish, got %v", err)
	}

	err = b.RemoveConsumer("x")
	if err != ErrBrokerStopped {
		t.Errorf("expected ErrBrokerStopped for remove, got %v", err)
	}
}

func TestBrokerStopIdempotent(t *testing.T) {
	b := NewBroker(DefaultConfig())
	b.Stop()
	b.Stop()
}

func TestUnackedCount(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	_, err := b.UnackedCount("nonexistent")
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound, got %v", err)
	}

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "t")

	unacked, _ := b.UnackedCount("c1")
	if unacked != 0 {
		t.Errorf("expected 0 unacked, got %d", unacked)
	}

	b.Publish("t", "x")
	msg := <-ch
	_ = msg

	unacked, _ = b.UnackedCount("c1")
	if unacked != 1 {
		t.Errorf("expected 1 unacked, got %d", unacked)
	}

	b.Ack("c1", msg.ID)

	unacked, _ = b.UnackedCount("c1")
	if unacked != 0 {
		t.Errorf("expected 0 unacked after ack, got %d", unacked)
	}
}

func TestPendingCount(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	_, err := b.PendingCount("nonexistent")
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound, got %v", err)
	}

	b.AddConsumer("c1")
	pending, _ := b.PendingCount("c1")
	if pending != 0 {
		t.Errorf("expected 0 pending, got %d", pending)
	}
}

func TestConsumerWithMultipleSubscriptions(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "a.b")
	b.Subscribe("c1", "a.c")

	b.Publish("a.b", "ab")
	b.Publish("a.c", "ac")
	b.Publish("a.d", "ad")

	received := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case msg := <-ch:
			received[msg.Topic] = true
			b.Ack("c1", msg.ID)
		case <-time.After(100 * time.Millisecond):
		}
	}

	if !received["a.b"] {
		t.Error("did not receive a.b")
	}
	if !received["a.c"] {
		t.Error("did not receive a.c")
	}
	if received["a.d"] {
		t.Error("should not receive a.d")
	}
}

func TestRemoveConsumerClosesChannel(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.RemoveConsumer("c1")

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after RemoveConsumer")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("channel not closed after RemoveConsumer")
	}
}

func TestStartRestart(t *testing.T) {
	b := NewBroker(DefaultConfig())
	b.Stop()
	b.Start()
	time.Sleep(50 * time.Millisecond)
	b.Stop()
}

func TestSubscribeIdempotent(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")
	err := b.Subscribe("c1", "test.topic")
	if err != nil {
		t.Fatalf("first subscribe failed: %v", err)
	}
	err = b.Subscribe("c1", "test.topic")
	if err != nil {
		t.Fatalf("duplicate subscribe should be nil, got %v", err)
	}
	if b.SubscriptionCount() != 1 {
		t.Errorf("expected 1 subscription after duplicate, got %d", b.SubscriptionCount())
	}
}

func TestPublishMultipleWildcards(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch1, _ := b.AddConsumer("c1")
	ch2, _ := b.AddConsumer("c2")
	ch3, _ := b.AddConsumer("c3")
	b.Subscribe("c1", "a.b.c")
	b.Subscribe("c2", "a.*.c")
	b.Subscribe("c3", "#")

	b.Publish("a.b.c", "triple")

	var wg sync.WaitGroup
	wg.Add(3)
	count := 0
	var mu sync.Mutex

	go func() { defer wg.Done(); <-ch1; mu.Lock(); count++; mu.Unlock() }()
	go func() { defer wg.Done(); <-ch2; mu.Lock(); count++; mu.Unlock() }()
	go func() { defer wg.Done(); <-ch3; mu.Lock(); count++; mu.Unlock() }()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
	}

	mu.Lock()
	defer mu.Unlock()
	if count != 3 {
		t.Errorf("expected 3 consumers to get message, got %d", count)
	}
}

func TestConcurrency(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch1, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.*")

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)

	go func() {
		defer wg.Done()
		for i := 0; i < n-1; i++ {
			go func(i int) {
				defer wg.Done()
				b.Publish("test.x", i)
			}(i)
		}
	}()

	received := 0
	timeout := time.After(2 * time.Second)
loop:
	for received < n {
		select {
		case msg := <-ch1:
			b.Ack("c1", msg.ID)
			received++
		case <-timeout:
			break loop
		}
	}

	wg.Wait()

	if received < n/2 {
		t.Errorf("expected at least %d/2 messages, got %d", n, received)
	}
}

func TestWildcardComplexMatching(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	tests := []struct {
		filter string
		topic  string
		want   bool
	}{
		{"a.b.c.d", "a.b.c.d", true},
		{"a.*.c.d", "a.b.c.d", true},
		{"a.*.*.d", "a.b.c.d", true},
		{"*.*.*.*", "a.b.c.d", true},
		{"a.#", "a.b.c.d.e", true},
		{"a.b.#", "a.b", true},
		{"a.b.#", "a.b.c", true},
		{"*.b.*", "a.b.c", true},
		{"*.b.*", "x.b.y", true},
		{"*.b.*", "a.b", false},
		{"*", "a", true},
		{"*", "a.b", false},
		{"#", "a", true},
		{"#", "a.b.c.d.e", true},
		{"a.b.*.d", "a.b.c.d", true},
		{"a.b.*.d", "a.b.x.d", true},
		{"a.b.*.d", "a.b.c.e", false},
	}

	for _, tt := range tests {
		got := b.matchTopic(tt.filter, tt.topic)
		if got != tt.want {
			t.Errorf("matchTopic(%q, %q) = %v, want %v", tt.filter, tt.topic, got, tt.want)
		}
	}
}

func TestDeadLetterMessageContent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRetry = 0
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "important-data")

	msg := <-ch
	originalID := msg.ID
	originalTopic := msg.Topic
	originalPayload := msg.Payload

	b.Nack("c1", msg.ID)

	dl := b.GetDeadLetters()
	if len(dl) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(dl))
	}

	if dl[0].ID != originalID {
		t.Errorf("expected ID %s, got %s", originalID, dl[0].ID)
	}
	if dl[0].Topic != originalTopic {
		t.Errorf("expected topic %s, got %s", originalTopic, dl[0].Topic)
	}
	if dl[0].Payload != originalPayload {
		t.Errorf("expected payload %v, got %v", originalPayload, dl[0].Payload)
	}
	if dl[0].RetryCount != 1 {
		t.Errorf("expected RetryCount 1, got %d", dl[0].RetryCount)
	}
}

func TestMessageIDUniqueness(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id, _ := b.Publish("test", i)
		if ids[id] {
			t.Errorf("duplicate message ID found: %s", id)
		}
		ids[id] = true
	}
}

func TestMultipleWildcardsDifferentConsumers(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch1, _ := b.AddConsumer("c1")
	ch2, _ := b.AddConsumer("c2")
	ch3, _ := b.AddConsumer("c3")

	b.Subscribe("c1", "sports.football")
	b.Subscribe("c2", "sports.*")
	b.Subscribe("c3", "#")

	b.Publish("sports.football", "match result")

	var wg sync.WaitGroup
	wg.Add(3)

	count := 0
	var mu sync.Mutex

	go func() {
		defer wg.Done()
		select {
		case msg := <-ch1:
			mu.Lock()
			count++
			mu.Unlock()
			b.Ack("c1", msg.ID)
		case <-time.After(100 * time.Millisecond):
			t.Error("c1 did not receive message")
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case msg := <-ch2:
			mu.Lock()
			count++
			mu.Unlock()
			b.Ack("c2", msg.ID)
		case <-time.After(100 * time.Millisecond):
			t.Error("c2 did not receive message")
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case msg := <-ch3:
			mu.Lock()
			count++
			mu.Unlock()
			b.Ack("c3", msg.ID)
		case <-time.After(100 * time.Millisecond):
			t.Error("c3 did not receive message")
		}
	}()

	wg.Wait()

	if count != 3 {
		t.Errorf("expected all 3 consumers to receive message, got %d", count)
	}
}

func TestConsumerChannelFullDurable(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ConsumerBuffer = 1
	cfg.MaxUnacked = 100
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.SubscribeDurable("c1", "test.topic", true)

	b.Publish("test.topic", "msg1")
	b.Publish("test.topic", "msg2")
	b.Publish("test.topic", "msg3")

	select {
	case msg := <-ch:
		b.Ack("c1", msg.ID)
	case <-time.After(50 * time.Millisecond):
		t.Fatal("did not receive msg1")
	}

	pending, _ := b.PendingCount("c1")
	if pending < 0 {
		t.Errorf("expected pending >= 0, got %d", pending)
	}

	received := make(map[string]bool)
	timeout := time.After(300 * time.Millisecond)
loop:
	for len(received) < 2 {
		select {
		case msg := <-ch:
			received[msg.Payload.(string)] = true
			b.Ack("c1", msg.ID)
		case <-timeout:
			break loop
		}
	}

	if !received["msg2"] {
		t.Error("did not receive msg2")
	}
	if !received["msg3"] {
		t.Error("did not receive msg3")
	}
}

func TestReconnectWithPendingMessages(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.SubscribeDurable("c1", "test.topic", true)

	b.Publish("test.topic", "msg1")
	<-ch

	b.DisconnectConsumer("c1")

	b.Publish("test.topic", "msg2")
	b.Publish("test.topic", "msg3")

	pending, _ := b.PendingCount("c1")
	if pending != 3 {
		t.Errorf("expected 3 pending messages (msg1 from unacked + msg2, msg3), got %d", pending)
	}

	b.ReconnectConsumer("c1")

	received := make(map[string]bool)
	timeout := time.After(300 * time.Millisecond)
loop:
	for len(received) < 3 {
		select {
		case msg := <-ch:
			received[msg.Payload.(string)] = true
			b.Ack("c1", msg.ID)
		case <-timeout:
			break loop
		}
	}

	if !received["msg1"] {
		t.Error("did not receive msg1 (redelivered) after reconnect")
	}
	if !received["msg2"] {
		t.Error("did not receive msg2 after reconnect")
	}
	if !received["msg3"] {
		t.Error("did not receive msg3 after reconnect")
	}

	unacked, _ := b.UnackedCount("c1")
	if unacked != 0 {
		t.Errorf("expected 0 unacked after all acks, got %d", unacked)
	}

	pending, _ = b.PendingCount("c1")
	if pending != 0 {
		t.Errorf("expected 0 pending after all delivered, got %d", pending)
	}
}

func TestAckTriggersNewDelivery(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxUnacked = 1
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumerWithOptions("c1", 1)
	b.SubscribeDurable("c1", "test.topic", true)

	b.Publish("test.topic", "msg1")
	b.Publish("test.topic", "msg2")

	msg1 := <-ch

	select {
	case <-ch:
		t.Error("should not receive msg2 before acking msg1 due to backpressure")
	case <-time.After(50 * time.Millisecond):
	}

	b.Ack("c1", msg1.ID)

	select {
	case msg2 := <-ch:
		if msg2.Payload != "msg2" {
			t.Errorf("expected msg2, got %v", msg2.Payload)
		}
		b.Ack("c1", msg2.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive msg2 after acking msg1")
	}
}

func TestMixedDurableNonDurable(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch1, _ := b.AddConsumer("c1")
	ch2, _ := b.AddConsumer("c2")

	b.SubscribeDurable("c1", "test.topic", true)
	b.Subscribe("c2", "test.topic")

	b.DisconnectConsumer("c1")
	b.DisconnectConsumer("c2")

	b.Publish("test.topic", "data")

	pending1, _ := b.PendingCount("c1")
	if pending1 != 1 {
		t.Errorf("expected 1 pending for durable consumer, got %d", pending1)
	}

	pending2, _ := b.PendingCount("c2")
	if pending2 != 0 {
		t.Errorf("expected 0 pending for non-durable consumer, got %d", pending2)
	}

	b.ReconnectConsumer("c1")
	b.ReconnectConsumer("c2")

	select {
	case msg := <-ch1:
		b.Ack("c1", msg.ID)
	case <-time.After(100 * time.Millisecond):
		t.Error("durable consumer should receive message after reconnect")
	}

	select {
	case <-ch2:
		t.Error("non-durable consumer should not receive old message")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestExactAndWildcardSameTopic(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch1, _ := b.AddConsumer("c1")
	ch2, _ := b.AddConsumer("c2")

	b.Subscribe("c1", "a.b.c")
	b.Subscribe("c2", "a.*.c")

	b.Publish("a.b.c", "test")

	var wg sync.WaitGroup
	wg.Add(2)

	received := 0
	var mu sync.Mutex

	go func() {
		defer wg.Done()
		select {
		case msg := <-ch1:
			mu.Lock()
			received++
			mu.Unlock()
			b.Ack("c1", msg.ID)
		case <-time.After(100 * time.Millisecond):
			t.Error("c1 (exact) did not receive")
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case msg := <-ch2:
			mu.Lock()
			received++
			mu.Unlock()
			b.Ack("c2", msg.ID)
		case <-time.After(100 * time.Millisecond):
			t.Error("c2 (wildcard) did not receive")
		}
	}()

	wg.Wait()

	if received != 2 {
		t.Errorf("expected both consumers to receive, got %d", received)
	}
}

func TestMaxRetryZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRetry = 0
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "fail-fast")

	msg := <-ch
	b.Nack("c1", msg.ID)

	if b.DeadLetterCount() != 1 {
		t.Errorf("expected immediate dead letter with MaxRetry=0, got %d", b.DeadLetterCount())
	}

	dl := b.GetDeadLetters()
	if dl[0].RetryCount != 1 {
		t.Errorf("expected RetryCount 1, got %d", dl[0].RetryCount)
	}
}

func TestWildcardOneAtDifferentLevels(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "a.*.c.*.e")

	tests := []struct {
		topic string
		want  bool
	}{
		{"a.b.c.d.e", true},
		{"a.x.c.y.e", true},
		{"a.b.c.d", false},
		{"a.b.c.d.e.f", false},
		{"a.b.c", false},
	}

	for _, tt := range tests {
		b.Publish(tt.topic, "data")

		select {
		case msg := <-ch:
			if !tt.want {
				t.Errorf("unexpected message for topic %s", tt.topic)
			}
			b.Ack("c1", msg.ID)
		case <-time.After(50 * time.Millisecond):
			if tt.want {
				t.Errorf("expected message for topic %s, got none", tt.topic)
			}
		}
	}
}

func TestPendingMessagesAfterReconnectOrder(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.SubscribeDurable("c1", "test.topic", true)

	b.DisconnectConsumer("c1")

	for i := 1; i <= 5; i++ {
		b.Publish("test.topic", i)
	}

	b.ReconnectConsumer("c1")

	received := make([]int, 0, 5)
	timeout := time.After(200 * time.Millisecond)
loop:
	for len(received) < 5 {
		select {
		case msg := <-ch:
			received = append(received, msg.Payload.(int))
			b.Ack("c1", msg.ID)
		case <-timeout:
			break loop
		}
	}

	if len(received) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(received))
	}

	for i, v := range received {
		if v != i+1 {
			t.Errorf("expected order %d, got %d at position %d", i+1, v, i)
		}
	}
}

func TestUnsubscribeAndResubscribe(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "msg1")
	msg := <-ch
	b.Ack("c1", msg.ID)

	b.Unsubscribe("c1", "test.topic")

	b.Publish("test.topic", "msg2")
	select {
	case <-ch:
		t.Error("should not receive after unsubscribe")
	case <-time.After(50 * time.Millisecond):
	}

	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "msg3")
	select {
	case msg := <-ch:
		if msg.Payload != "msg3" {
			t.Errorf("expected msg3, got %v", msg.Payload)
		}
		b.Ack("c1", msg.ID)
	case <-time.After(100 * time.Millisecond):
		t.Error("should receive msg3 after resubscribe")
	}
}

func TestConsumerOptions(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, err := b.AddConsumerWithOptions("c1", 5)
	if err != nil {
		t.Fatalf("AddConsumerWithOptions failed: %v", err)
	}
	if ch == nil {
		t.Fatal("channel is nil")
	}

	_, err = b.AddConsumerWithOptions("c1", 10)
	if err != ErrConsumerExists {
		t.Errorf("expected ErrConsumerExists, got %v", err)
	}

	ch2, err := b.AddConsumerWithOptions("c2", 0)
	if err != nil {
		t.Fatalf("AddConsumerWithOptions with 0 should use default, got error: %v", err)
	}
	if ch2 == nil {
		t.Fatal("channel is nil")
	}

	_, err = b.AddConsumerWithOptions("", 5)
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound for empty ID, got %v", err)
	}
}

func TestGetDeadLettersCopy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRetry = 0
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "test")
	msg := <-ch
	b.Nack("c1", msg.ID)

	dl1 := b.GetDeadLetters()
	if len(dl1) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(dl1))
	}

	dl1[0].Payload = "modified"

	dl2 := b.GetDeadLetters()
	if dl2[0].Payload == "modified" {
		t.Error("GetDeadLetters should return a copy, but modifications affected internal state")
	}
}

func TestNoDuplicateMessagesMultipleSubs(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "a.b.c")
	b.Subscribe("c1", "a.*.c")
	b.Subscribe("c1", "#")

	b.Publish("a.b.c", "data")

	received := 0
	timeout := time.After(200 * time.Millisecond)
loop:
	for {
		select {
		case msg := <-ch:
			received++
			b.Ack("c1", msg.ID)
		case <-timeout:
			break loop
		}
	}

	if received != 1 {
		t.Errorf("expected exactly 1 message (no duplicates), got %d", received)
	}
}

func TestWildcardOneAndStarChildSameNode(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "a.*")
	b.Subscribe("c1", "a.*.c")

	b.Publish("a.b", "short")

	receivedShort := 0
	select {
	case msg := <-ch:
		receivedShort++
		b.Ack("c1", msg.ID)
	case <-time.After(100 * time.Millisecond):
	}
	if receivedShort != 1 {
		t.Errorf("expected 1 message for a.b, got %d", receivedShort)
	}

	b.Publish("a.b.c", "long")

	receivedLong := 0
	select {
	case msg := <-ch:
		receivedLong++
		b.Ack("c1", msg.ID)
	case <-time.After(100 * time.Millisecond):
	}
	if receivedLong != 1 {
		t.Errorf("expected 1 message for a.b.c, got %d", receivedLong)
	}
}

func TestNackWithBackpressure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxUnacked = 1
	cfg.MaxRetry = 2
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumerWithOptions("c1", 1)
	b.SubscribeDurable("c1", "test.topic", true)

	b.Publish("test.topic", "msg1")
	b.Publish("test.topic", "msg2")

	msg1 := <-ch
	if msg1.Payload != "msg1" {
		t.Errorf("expected msg1, got %v", msg1.Payload)
	}

	b.Nack("c1", msg1.ID)

	select {
	case msg := <-ch:
		if msg.Payload != "msg1" {
			t.Errorf("expected redelivered msg1, got %v", msg.Payload)
		}
		if msg.RetryCount != 1 {
			t.Errorf("expected RetryCount 1, got %d", msg.RetryCount)
		}
		b.Ack("c1", msg.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for redelivered msg1")
	}

	select {
	case msg2 := <-ch:
		if msg2.Payload != "msg2" {
			t.Errorf("expected msg2 after ack, got %v", msg2.Payload)
		}
		b.Ack("c1", msg2.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for msg2")
	}
}

func TestDisconnectNonDurableLosesPending(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "msg1")

	msg := <-ch
	_ = msg

	b.DisconnectConsumer("c1")

	pending, _ := b.PendingCount("c1")
	if pending != 0 {
		t.Errorf("expected 0 pending for non-durable after disconnect, got %d", pending)
	}

	unacked, _ := b.UnackedCount("c1")
	if unacked != 1 {
		t.Errorf("expected 1 unacked for non-durable after disconnect, got %d", unacked)
	}
}

func TestTopicValidationEdgeCases(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	invalidTopics := []string{
		"",
		".",
		"a.",
		".a",
		"a..b",
		"a...b",
	}

	for _, topic := range invalidTopics {
		_, err := b.Publish(topic, "data")
		if err != ErrTopicInvalid {
			t.Errorf("expected ErrTopicInvalid for topic %q, got %v", topic, err)
		}
	}

	b.AddConsumer("c1")

	invalidFilters := []string{
		"",
		".",
		"a.",
		".a",
		"a..b",
		"a.#.b",
		"#.a",
		"a#",
		"a*b",
	}

	for _, filter := range invalidFilters {
		err := b.Subscribe("c1", filter)
		if err != ErrTopicInvalid {
			t.Errorf("expected ErrTopicInvalid for filter %q, got %v", filter, err)
		}
	}
}

func TestSingleSegmentWildcards(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch1, _ := b.AddConsumer("c1")
	ch2, _ := b.AddConsumer("c2")
	ch3, _ := b.AddConsumer("c3")

	b.Subscribe("c1", "*")
	b.Subscribe("c2", "#")
	b.Subscribe("c3", "test")

	b.Publish("single", "data")

	var wg sync.WaitGroup
	wg.Add(2)

	c1Received := false
	c2Received := false

	go func() {
		defer wg.Done()
		select {
		case msg := <-ch1:
			c1Received = true
			b.Ack("c1", msg.ID)
		case <-time.After(100 * time.Millisecond):
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case msg := <-ch2:
			c2Received = true
			b.Ack("c2", msg.ID)
		case <-time.After(100 * time.Millisecond):
		}
	}()

	wg.Wait()

	if !c1Received {
		t.Error("c1 (*) should receive single-segment topic")
	}
	if !c2Received {
		t.Error("c2 (#) should receive single-segment topic")
	}

	select {
	case <-ch3:
		t.Error("c3 (test) should not receive 'single' topic")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDeepTopicMatching(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "a.b.c.d.e.f.g")

	b.Publish("a.b.c.d.e.f.g", "deep")

	select {
	case msg := <-ch:
		if msg.Payload != "deep" {
			t.Errorf("expected deep, got %v", msg.Payload)
		}
		b.Ack("c1", msg.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for deep topic message")
	}
}

func TestHashWildcardOnlyRoot(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "#")

	topics := []string{
		"a",
		"a.b",
		"a.b.c",
		"x.y.z.w",
		"very.deep.topic.with.many.levels",
	}

	for _, topic := range topics {
		b.Publish(topic, topic)
	}

	received := make(map[string]bool)
	timeout := time.After(300 * time.Millisecond)
loop:
	for len(received) < len(topics) {
		select {
		case msg := <-ch:
			received[msg.Topic] = true
			b.Ack("c1", msg.ID)
		case <-timeout:
			break loop
		}
	}

	if len(received) != len(topics) {
		t.Errorf("expected %d messages for # subscription, got %d", len(topics), len(received))
	}

	for _, topic := range topics {
		if !received[topic] {
			t.Errorf("missing message for topic %s", topic)
		}
	}
}

func TestSubscribeAfterPublishDurable(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")

	b.Publish("test.topic", "before-sub")

	b.SubscribeDurable("c1", "test.topic", true)

	select {
	case <-ch:
		t.Error("should not receive message published before subscription")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMultipleUnsubscribes(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")
	b.Subscribe("c1", "topic1")
	b.Subscribe("c1", "topic2")
	b.Subscribe("c1", "topic3")

	if b.SubscriptionCount() != 3 {
		t.Errorf("expected 3 subscriptions, got %d", b.SubscriptionCount())
	}

	b.Unsubscribe("c1", "topic2")
	if b.SubscriptionCount() != 2 {
		t.Errorf("expected 2 subscriptions after unsubscribe, got %d", b.SubscriptionCount())
	}

	b.Unsubscribe("c1", "topic1")
	b.Unsubscribe("c1", "topic3")
	if b.SubscriptionCount() != 0 {
		t.Errorf("expected 0 subscriptions after all unsubscribed, got %d", b.SubscriptionCount())
	}

	err := b.Unsubscribe("c1", "topic1")
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestConsumerChannelClosedOnStop(t *testing.T) {
	b := NewBroker(DefaultConfig())

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Stop()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after Stop")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("channel not closed after Stop")
	}
}

func TestPublishAfterConsumerRemoved(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")
	b.RemoveConsumer("c1")

	msgID, err := b.Publish("test.topic", "data")
	if err != nil {
		t.Fatalf("Publish should succeed even with no consumers, got %v", err)
	}
	if msgID == "" {
		t.Error("expected non-empty message ID")
	}
}

func TestAckAfterConsumerRemoved(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	ch, _ := b.AddConsumer("c1")
	b.Subscribe("c1", "test.topic")

	b.Publish("test.topic", "data")
	msg := <-ch

	b.RemoveConsumer("c1")

	err := b.Ack("c1", msg.ID)
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound, got %v", err)
	}
}

func TestProcessTimeoutsWhenStopped(t *testing.T) {
	b := NewBroker(DefaultConfig())
	b.Stop()

	b.ProcessTimeouts()
}

func TestDurableBufferOrderAfterAck(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxUnacked = 1
	b := NewBroker(cfg)
	defer b.Stop()

	ch, _ := b.AddConsumerWithOptions("c1", 1)
	b.SubscribeDurable("c1", "test.topic", true)

	for i := 1; i <= 5; i++ {
		b.Publish("test.topic", i)
	}

	received := make([]int, 0, 5)
	for i := 0; i < 5; i++ {
		select {
		case msg := <-ch:
			received = append(received, msg.Payload.(int))
			b.Ack("c1", msg.ID)
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timed out waiting for message %d", i+1)
		}
	}

	if len(received) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(received))
	}

	for i, v := range received {
		if v != i+1 {
			t.Errorf("expected order %d, got %d at position %d", i+1, v, i)
		}
	}
}

func TestWildcardHashAtRootWithEmptyTopic(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")
	err := b.Subscribe("c1", "#")
	if err != nil {
		t.Fatalf("Subscribe to # should succeed, got %v", err)
	}

	_, err = b.Publish("", "data")
	if err != ErrTopicInvalid {
		t.Errorf("expected ErrTopicInvalid for empty topic, got %v", err)
	}
}

func TestFindMatchingSubsEmptyTopic(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	subs := b.findMatchingSubs("")
	if len(subs) != 0 {
		t.Errorf("expected 0 matching subs for empty topic, got %d", len(subs))
	}
}

func TestReconnectAfterRemove(t *testing.T) {
	b := NewBroker(DefaultConfig())
	defer b.Stop()

	b.AddConsumer("c1")
	b.SubscribeDurable("c1", "test.topic", true)
	b.RemoveConsumer("c1")

	err := b.ReconnectConsumer("c1")
	if err != ErrConsumerNotFound {
		t.Errorf("expected ErrConsumerNotFound after RemoveConsumer, got %v", err)
	}
}
