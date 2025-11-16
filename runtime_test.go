package aixgo

import (
	"sync"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
)

func TestNewSimpleRuntime(t *testing.T) {
	rt := NewSimpleRuntime()

	if rt == nil {
		t.Fatal("NewSimpleRuntime returned nil")
	}

	if rt.channels == nil {
		t.Error("channels map is nil")
	}

	if len(rt.channels) != 0 {
		t.Errorf("channels map length = %v, want 0", len(rt.channels))
	}
}

func TestSimpleRuntime_Send_CreateChannel(t *testing.T) {
	rt := NewSimpleRuntime()
	target := "test-channel"

	msg := &agent.Message{
		&pb.Message{
			Id:      "msg-1",
			Type:    "test",
			Payload: "test payload",
		},
	}

	err := rt.Send(target, msg)
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	// Verify channel was created
	rt.mu.RLock()
	ch, exists := rt.channels[target]
	rt.mu.RUnlock()

	if !exists {
		t.Error("channel was not created")
	}

	if ch == nil {
		t.Error("created channel is nil")
	}

	// Receive the message
	select {
	case receivedMsg := <-ch:
		if receivedMsg.Id != msg.Id {
			t.Errorf("received message Id = %v, want %v", receivedMsg.Id, msg.Id)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message")
	}
}

func TestSimpleRuntime_Send_ExistingChannel(t *testing.T) {
	rt := NewSimpleRuntime()
	target := "existing-channel"

	// Send first message to create channel
	msg1 := &agent.Message{&pb.Message{Id: "msg-1"}}
	err := rt.Send(target, msg1)
	if err != nil {
		t.Fatalf("First Send returned error: %v", err)
	}

	// Send second message to existing channel
	msg2 := &agent.Message{&pb.Message{Id: "msg-2"}}
	err = rt.Send(target, msg2)
	if err != nil {
		t.Fatalf("Second Send returned error: %v", err)
	}

	// Verify both messages are in the channel
	ch, _ := rt.Recv(target)

	select {
	case receivedMsg := <-ch:
		if receivedMsg.Id != "msg-1" {
			t.Errorf("first message Id = %v, want msg-1", receivedMsg.Id)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for first message")
	}

	select {
	case receivedMsg := <-ch:
		if receivedMsg.Id != "msg-2" {
			t.Errorf("second message Id = %v, want msg-2", receivedMsg.Id)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for second message")
	}
}

func TestSimpleRuntime_Send_FullChannel(t *testing.T) {
	rt := NewSimpleRuntime()
	target := "full-channel"

	// Fill the channel (capacity is 100)
	for i := 0; i < 100; i++ {
		msg := &agent.Message{&pb.Message{Id: string(rune(i))}}
		err := rt.Send(target, msg)
		if err != nil {
			t.Fatalf("Send %d returned error: %v", i, err)
		}
	}

	// Try to send one more message (should fail)
	msg := &agent.Message{&pb.Message{Id: "overflow"}}
	err := rt.Send(target, msg)

	if err == nil {
		t.Error("expected error when channel is full, got nil")
	}

	expectedErr := "channel full-channel is full"
	if err.Error() != expectedErr {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestSimpleRuntime_Recv_CreateChannel(t *testing.T) {
	rt := NewSimpleRuntime()
	source := "test-source"

	ch, err := rt.Recv(source)
	if err != nil {
		t.Fatalf("Recv returned error: %v", err)
	}

	if ch == nil {
		t.Error("Recv returned nil channel")
	}

	// Verify channel was created
	rt.mu.RLock()
	_, exists := rt.channels[source]
	rt.mu.RUnlock()

	if !exists {
		t.Error("channel was not created")
	}
}

func TestSimpleRuntime_Recv_ExistingChannel(t *testing.T) {
	rt := NewSimpleRuntime()
	source := "existing-source"

	// Create channel via Send
	msg := &agent.Message{&pb.Message{Id: "test-msg"}}
	err := rt.Send(source, msg)
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	// Get channel via Recv
	ch, err := rt.Recv(source)
	if err != nil {
		t.Fatalf("Recv returned error: %v", err)
	}

	// Should receive the message
	select {
	case receivedMsg := <-ch:
		if receivedMsg.Id != "test-msg" {
			t.Errorf("received message Id = %v, want test-msg", receivedMsg.Id)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message")
	}
}

func TestSimpleRuntime_Recv_SameChannelTwice(t *testing.T) {
	rt := NewSimpleRuntime()
	source := "same-source"

	ch1, err1 := rt.Recv(source)
	ch2, err2 := rt.Recv(source)

	if err1 != nil {
		t.Errorf("first Recv returned error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second Recv returned error: %v", err2)
	}

	// Both should return the same channel
	msg := &agent.Message{&pb.Message{Id: "test"}}
	rt.Send(source, msg)

	// Both ch1 and ch2 should receive the message
	select {
	case receivedMsg := <-ch1:
		if receivedMsg.Id != "test" {
			t.Errorf("ch1 received message Id = %v, want test", receivedMsg.Id)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message on ch1")
	}

	// Send another message
	msg2 := &agent.Message{&pb.Message{Id: "test2"}}
	rt.Send(source, msg2)

	select {
	case receivedMsg := <-ch2:
		if receivedMsg.Id != "test2" {
			t.Errorf("ch2 received message Id = %v, want test2", receivedMsg.Id)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message on ch2")
	}
}

func TestSimpleRuntime_ConcurrentSend(t *testing.T) {
	rt := NewSimpleRuntime()
	target := "concurrent-target"
	numSenders := 10
	messagesPerSender := 10

	var wg sync.WaitGroup
	wg.Add(numSenders)

	// Send messages concurrently
	for i := 0; i < numSenders; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerSender; j++ {
				msg := &agent.Message{&pb.Message{
					Id: string(rune('A'+id)) + string(rune('0'+j)),
				}}
				_ = rt.Send(target, msg)
			}
		}(i)
	}

	wg.Wait()

	// Verify all messages were sent
	ch, _ := rt.Recv(target)
	count := 0

	timeout := time.After(1 * time.Second)
	for count < numSenders*messagesPerSender {
		select {
		case <-ch:
			count++
		case <-timeout:
			t.Fatalf("timeout: received %d messages, want %d", count, numSenders*messagesPerSender)
		}
	}

	if count != numSenders*messagesPerSender {
		t.Errorf("received %d messages, want %d", count, numSenders*messagesPerSender)
	}
}

func TestSimpleRuntime_ConcurrentRecv(t *testing.T) {
	rt := NewSimpleRuntime()
	source := "concurrent-source"
	numReceivers := 5

	var wg sync.WaitGroup
	wg.Add(numReceivers)

	// Get channels concurrently
	channels := make([]<-chan *agent.Message, numReceivers)
	errors := make([]error, numReceivers)

	for i := 0; i < numReceivers; i++ {
		go func(id int) {
			defer wg.Done()
			ch, err := rt.Recv(source)
			channels[id] = ch
			errors[id] = err
		}(i)
	}

	wg.Wait()

	// Verify all succeeded
	for i, err := range errors {
		if err != nil {
			t.Errorf("receiver %d got error: %v", i, err)
		}
	}

	for i, ch := range channels {
		if ch == nil {
			t.Errorf("receiver %d got nil channel", i)
		}
	}
}

func TestSimpleRuntime_ConcurrentSendRecv(t *testing.T) {
	rt := NewSimpleRuntime()
	channel := "mixed-channel"
	numOperations := 20

	var wg sync.WaitGroup
	wg.Add(numOperations)

	// Mix Send and Recv operations
	for i := 0; i < numOperations; i++ {
		if i%2 == 0 {
			go func(id int) {
				defer wg.Done()
				msg := &agent.Message{&pb.Message{Id: string(rune('A' + id))}}
				_ = rt.Send(channel, msg)
			}(i)
		} else {
			go func(id int) {
				defer wg.Done()
				_, _ = rt.Recv(channel)
			}(i)
		}
	}

	wg.Wait()
}

func TestSimpleRuntime_MultipleChannels(t *testing.T) {
	rt := NewSimpleRuntime()

	channels := []string{"channel1", "channel2", "channel3", "channel4", "channel5"}

	// Send to each channel
	for i, ch := range channels {
		msg := &agent.Message{&pb.Message{
			Id:      string(rune('A' + i)),
			Payload: ch,
		}}
		err := rt.Send(ch, msg)
		if err != nil {
			t.Errorf("Send to %s returned error: %v", ch, err)
		}
	}

	// Receive from each channel
	for _, ch := range channels {
		recvCh, err := rt.Recv(ch)
		if err != nil {
			t.Errorf("Recv from %s returned error: %v", ch, err)
		}

		select {
		case msg := <-recvCh:
			if msg.Payload != ch {
				t.Errorf("channel %s: received payload %v, want %v", ch, msg.Payload, ch)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("timeout receiving from channel %s", ch)
		}
	}

	// Verify correct number of channels
	rt.mu.RLock()
	numChannels := len(rt.channels)
	rt.mu.RUnlock()

	if numChannels != len(channels) {
		t.Errorf("number of channels = %v, want %v", numChannels, len(channels))
	}
}

func TestSimpleRuntime_MessageOrdering(t *testing.T) {
	rt := NewSimpleRuntime()
	target := "ordered-channel"

	// Send messages in order
	numMessages := 10
	for i := 0; i < numMessages; i++ {
		msg := &agent.Message{&pb.Message{
			Id:      string(rune('0' + i)),
			Payload: string(rune('A' + i)),
		}}
		err := rt.Send(target, msg)
		if err != nil {
			t.Fatalf("Send message %d returned error: %v", i, err)
		}
	}

	// Receive and verify order
	ch, _ := rt.Recv(target)
	for i := 0; i < numMessages; i++ {
		select {
		case msg := <-ch:
			expectedId := string(rune('0' + i))
			if msg.Id != expectedId {
				t.Errorf("message %d: Id = %v, want %v (ordering violated)", i, msg.Id, expectedId)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timeout waiting for message %d", i)
		}
	}
}

func TestSimpleRuntime_EmptyMessages(t *testing.T) {
	rt := NewSimpleRuntime()
	target := "empty-msg-channel"

	// Send empty message
	msg := &agent.Message{&pb.Message{}}
	err := rt.Send(target, msg)
	if err != nil {
		t.Fatalf("Send empty message returned error: %v", err)
	}

	ch, _ := rt.Recv(target)
	select {
	case receivedMsg := <-ch:
		if receivedMsg.Id != "" || receivedMsg.Type != "" || receivedMsg.Payload != "" {
			t.Error("empty message fields were not preserved")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for empty message")
	}
}

func TestSimpleRuntime_NilMessage(t *testing.T) {
	rt := NewSimpleRuntime()
	target := "nil-msg-channel"

	// Send nil message (should not panic)
	err := rt.Send(target, nil)
	if err != nil {
		t.Fatalf("Send nil message returned error: %v", err)
	}

	ch, _ := rt.Recv(target)
	select {
	case receivedMsg := <-ch:
		if receivedMsg != nil {
			t.Errorf("received message = %v, want nil", receivedMsg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for nil message")
	}
}

func TestSimpleRuntime_ChannelCapacity(t *testing.T) {
	rt := NewSimpleRuntime()
	target := "capacity-test"

	// Verify channel capacity is 100
	// Send 100 messages (should all succeed)
	for i := 0; i < 100; i++ {
		msg := &agent.Message{&pb.Message{Id: string(rune(i))}}
		err := rt.Send(target, msg)
		if err != nil {
			t.Fatalf("Send message %d returned unexpected error: %v", i, err)
		}
	}

	// 101st message should fail
	msg := &agent.Message{&pb.Message{Id: "overflow"}}
	err := rt.Send(target, msg)
	if err == nil {
		t.Error("expected error on 101st message, got nil")
	}
}

func TestSimpleRuntime_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	rt := NewSimpleRuntime()
	numChannels := 10
	messagesPerChannel := 100

	var wg sync.WaitGroup

	// Send to multiple channels concurrently
	for c := 0; c < numChannels; c++ {
		wg.Add(1)
		go func(channelId int) {
			defer wg.Done()
			target := "stress-channel-" + string(rune('A'+channelId))
			for i := 0; i < messagesPerChannel; i++ {
				msg := &agent.Message{&pb.Message{
					Id: string(rune('0'+i%10)) + string(rune(channelId)),
				}}
				_ = rt.Send(target, msg)
			}
		}(c)
	}

	wg.Wait()

	// Verify all channels exist
	rt.mu.RLock()
	numCreatedChannels := len(rt.channels)
	rt.mu.RUnlock()

	if numCreatedChannels != numChannels {
		t.Errorf("created %d channels, want %d", numCreatedChannels, numChannels)
	}
}
