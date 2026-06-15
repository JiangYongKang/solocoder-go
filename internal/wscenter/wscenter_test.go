package wscenter

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockConn struct {
	id           string
	mu           sync.Mutex
	receivedMsgs []*Message
	sendErr      error
	closed       bool
	sendDelay    time.Duration
	sendCount    int32
}

func newMockConn(id string) *mockConn {
	return &mockConn{
		id:           id,
		receivedMsgs: make([]*Message, 0),
	}
}

func (m *mockConn) ID() string {
	return m.id
}

func (m *mockConn) SendMessage(msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendDelay > 0 {
		time.Sleep(m.sendDelay)
	}

	if m.sendErr != nil {
		return m.sendErr
	}

	m.receivedMsgs = append(m.receivedMsgs, msg)
	atomic.AddInt32(&m.sendCount, 1)
	return nil
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConn) getReceivedMessages() []*Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs := make([]*Message, len(m.receivedMsgs))
	copy(msgs, m.receivedMsgs)
	return msgs
}

func (m *mockConn) getSendCount() int32 {
	return atomic.LoadInt32(&m.sendCount)
}

func (m *mockConn) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *mockConn) clearMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.receivedMsgs = m.receivedMsgs[:0]
}

func TestNewWSCenter(t *testing.T) {
	cfg := DefaultConfig()
	ws := NewWSCenter(cfg)
	if ws == nil {
		t.Fatal("NewWSCenter returned nil")
	}
	if ws.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", ws.ClientCount())
	}
	if ws.RoomCount() != 0 {
		t.Errorf("expected 0 rooms, got %d", ws.RoomCount())
	}
	ws.Stop()
}

func TestNewWSCenterConfigDefaults(t *testing.T) {
	cfg := Config{}
	ws := NewWSCenter(cfg)
	if ws.cfg.PingInterval != defaultPingInterval {
		t.Errorf("expected default PingInterval %v, got %v", defaultPingInterval, ws.cfg.PingInterval)
	}
	if ws.cfg.PongTimeout != defaultPongTimeout {
		t.Errorf("expected default PongTimeout %v, got %v", defaultPongTimeout, ws.cfg.PongTimeout)
	}
	if ws.cfg.SendTimeout != defaultSendTimeout {
		t.Errorf("expected default SendTimeout %v, got %v", defaultSendTimeout, ws.cfg.SendTimeout)
	}
	if ws.cfg.ClientBufferSize != defaultClientBufferSize {
		t.Errorf("expected default ClientBufferSize %d, got %d", defaultClientBufferSize, ws.cfg.ClientBufferSize)
	}
	ws.Stop()

	cfg2 := Config{PingInterval: -1, PongTimeout: -1, SendTimeout: -1, ClientBufferSize: -1}
	ws2 := NewWSCenter(cfg2)
	if ws2.cfg.PingInterval != defaultPingInterval {
		t.Errorf("expected -1 to map to default PingInterval %v, got %v", defaultPingInterval, ws2.cfg.PingInterval)
	}
	if ws2.cfg.PongTimeout != defaultPongTimeout {
		t.Errorf("expected -1 to map to default PongTimeout %v, got %v", defaultPongTimeout, ws2.cfg.PongTimeout)
	}
	if ws2.cfg.SendTimeout != defaultSendTimeout {
		t.Errorf("expected -1 to map to default SendTimeout %v, got %v", defaultSendTimeout, ws2.cfg.SendTimeout)
	}
	if ws2.cfg.ClientBufferSize != defaultClientBufferSize {
		t.Errorf("expected -1 to map to default ClientBufferSize %d, got %d", defaultClientBufferSize, ws2.cfg.ClientBufferSize)
	}
	ws2.Stop()
}

func TestConnect(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn := newMockConn("client1")
	client, err := ws.Connect(conn)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
	if client.ID() != "client1" {
		t.Errorf("expected client id 'client1', got '%s'", client.ID())
	}
	if ws.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", ws.ClientCount())
	}

	_, err = ws.Connect(conn)
	if err != ErrClientExists {
		t.Errorf("expected ErrClientExists, got %v", err)
	}

	_, err = ws.Connect(nil)
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID for nil conn, got %v", err)
	}

	conn2 := newMockConn("")
	_, err = ws.Connect(conn2)
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID for empty id, got %v", err)
	}
}

func TestDisconnect(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn := newMockConn("client1")
	ws.Connect(conn)

	err := ws.Disconnect("client1")
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
	if ws.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", ws.ClientCount())
	}
	if !conn.isClosed() {
		t.Error("expected conn to be closed")
	}

	err = ws.Disconnect("client1")
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}

	err = ws.Disconnect("")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestCreateRoom(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	room, err := ws.CreateRoom("room1")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}
	if room == nil {
		t.Fatal("room is nil")
	}
	if room.ID() != "room1" {
		t.Errorf("expected room id 'room1', got '%s'", room.ID())
	}
	if ws.RoomCount() != 1 {
		t.Errorf("expected 1 room, got %d", ws.RoomCount())
	}
	if !ws.RoomExists("room1") {
		t.Error("expected room1 to exist")
	}

	_, err = ws.CreateRoom("room1")
	if err != ErrRoomExists {
		t.Errorf("expected ErrRoomExists, got %v", err)
	}

	_, err = ws.CreateRoom("")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestGetOrCreateRoom(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	room1, err := ws.GetOrCreateRoom("room1")
	if err != nil {
		t.Fatalf("GetOrCreateRoom failed: %v", err)
	}
	if room1 == nil {
		t.Fatal("room is nil")
	}
	if ws.RoomCount() != 1 {
		t.Errorf("expected 1 room, got %d", ws.RoomCount())
	}

	room2, err := ws.GetOrCreateRoom("room1")
	if err != nil {
		t.Fatalf("GetOrCreateRoom failed: %v", err)
	}
	if room1 != room2 {
		t.Error("expected same room instance")
	}
	if ws.RoomCount() != 1 {
		t.Errorf("expected 1 room, got %d", ws.RoomCount())
	}

	_, err = ws.GetOrCreateRoom("")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestJoinRoom(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.CreateRoom("room1")

	err := ws.JoinRoom("client1", "room1")
	if err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}

	err = ws.JoinRoom("client2", "room1")
	if err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}

	clients, err := ws.GetRoomClients("room1")
	if err != nil {
		t.Fatalf("GetRoomClients failed: %v", err)
	}
	if len(clients) != 2 {
		t.Errorf("expected 2 clients, got %d", len(clients))
	}

	err = ws.JoinRoom("client1", "room1")
	if err != ErrClientAlreadyInRoom {
		t.Errorf("expected ErrClientAlreadyInRoom, got %v", err)
	}

	err = ws.JoinRoom("nonexistent", "room1")
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}

	err = ws.JoinRoom("client1", "nonexistent")
	if err != ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}

	err = ws.JoinRoom("", "room1")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID for empty client, got %v", err)
	}

	err = ws.JoinRoom("client1", "")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID for empty room, got %v", err)
	}
}

func TestLeaveRoom(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	ws.Connect(conn1)
	ws.CreateRoom("room1")
	ws.JoinRoom("client1", "room1")

	err := ws.LeaveRoom("client1", "room1")
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}

	_, err = ws.GetRoomClients("room1")
	if err != ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound after room auto-destroyed, got %v", err)
	}
	if ws.RoomExists("room1") {
		t.Error("expected room1 to be auto-destroyed when empty")
	}

	err = ws.LeaveRoom("client1", "room1")
	if err != ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}

	ws.CreateRoom("room2")
	err = ws.LeaveRoom("client1", "room2")
	if err != ErrClientNotInRoom {
		t.Errorf("expected ErrClientNotInRoom, got %v", err)
	}

	err = ws.LeaveRoom("nonexistent", "room2")
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}

	err = ws.LeaveRoom("", "room2")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID for empty client, got %v", err)
	}

	err = ws.LeaveRoom("client1", "")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID for empty room, got %v", err)
	}
}

func TestRoomAutoDestroy(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.CreateRoom("room1")

	ws.JoinRoom("client1", "room1")
	ws.JoinRoom("client2", "room1")

	if !ws.RoomExists("room1") {
		t.Error("expected room1 to exist")
	}

	ws.LeaveRoom("client1", "room1")
	if !ws.RoomExists("room1") {
		t.Error("expected room1 to exist with 1 client")
	}

	ws.LeaveRoom("client2", "room1")
	if ws.RoomExists("room1") {
		t.Error("expected room1 to be auto-destroyed when empty")
	}
}

func TestGetRoomClients(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.CreateRoom("room1")
	ws.JoinRoom("client1", "room1")
	ws.JoinRoom("client2", "room1")

	clients, err := ws.GetRoomClients("room1")
	if err != nil {
		t.Fatalf("GetRoomClients failed: %v", err)
	}
	if len(clients) != 2 {
		t.Errorf("expected 2 clients, got %d", len(clients))
	}

	_, err = ws.GetRoomClients("nonexistent")
	if err != ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}

	_, err = ws.GetRoomClients("")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestBroadcastToRoom(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	conn3 := newMockConn("client3")
	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.Connect(conn3)
	ws.CreateRoom("room1")
	ws.JoinRoom("client1", "room1")
	ws.JoinRoom("client2", "room1")
	ws.JoinRoom("client3", "room1")

	time.Sleep(100 * time.Millisecond)
	conn1.clearMessages()
	conn2.clearMessages()
	conn3.clearMessages()

	payload := []byte("hello everyone")
	successCount, err := ws.BroadcastToRoom("room1", payload, "client1")
	if err != nil {
		t.Fatalf("BroadcastToRoom failed: %v", err)
	}
	if successCount != 2 {
		t.Errorf("expected 2 successful sends, got %d", successCount)
	}

	time.Sleep(100 * time.Millisecond)

	conn1Msgs := conn1.getReceivedMessages()
	conn2Msgs := conn2.getReceivedMessages()
	conn3Msgs := conn3.getReceivedMessages()

	conn1BroadcastCount := 0
	for _, msg := range conn1Msgs {
		if msg.Type == MessageTypeBroadcast {
			conn1BroadcastCount++
		}
	}
	conn2BroadcastCount := 0
	for _, msg := range conn2Msgs {
		if msg.Type == MessageTypeBroadcast {
			conn2BroadcastCount++
		}
	}
	conn3BroadcastCount := 0
	for _, msg := range conn3Msgs {
		if msg.Type == MessageTypeBroadcast {
			conn3BroadcastCount++
		}
	}

	if conn1BroadcastCount != 0 {
		t.Errorf("expected sender to not receive broadcast, got %d broadcast messages", conn1BroadcastCount)
	}
	if conn2BroadcastCount != 1 {
		t.Errorf("expected client2 to receive 1 broadcast message, got %d", conn2BroadcastCount)
	}
	if conn3BroadcastCount != 1 {
		t.Errorf("expected client3 to receive 1 broadcast message, got %d", conn3BroadcastCount)
	}

	msgs := conn2.getReceivedMessages()
	if len(msgs) > 0 {
		if msgs[0].Type != MessageTypeBroadcast {
			t.Errorf("expected MessageTypeBroadcast, got %d", msgs[0].Type)
		}
		if msgs[0].From != "client1" {
			t.Errorf("expected From 'client1', got '%s'", msgs[0].From)
		}
		if string(msgs[0].Payload) != "hello everyone" {
			t.Errorf("expected payload 'hello everyone', got '%s'", string(msgs[0].Payload))
		}
		if msgs[0].RoomID != "room1" {
			t.Errorf("expected RoomID 'room1', got '%s'", msgs[0].RoomID)
		}
	}

	_, err = ws.BroadcastToRoom("nonexistent", payload, "client1")
	if err != ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}

	_, err = ws.BroadcastToRoom("", payload, "client1")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestBroadcastToRoomWithSendFailure(t *testing.T) {
	cfg := Config{
		SendTimeout:      30 * time.Millisecond,
		ClientBufferSize: 1,
	}
	ws := NewWSCenter(cfg)
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	conn3 := newMockConn("client3")
	conn3.sendDelay = 200 * time.Millisecond

	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.Connect(conn3)
	ws.CreateRoom("room1")
	ws.JoinRoom("client1", "room1")
	ws.JoinRoom("client2", "room1")
	ws.JoinRoom("client3", "room1")

	time.Sleep(100 * time.Millisecond)
	conn1.clearMessages()
	conn2.clearMessages()
	conn3.clearMessages()

	_, err := ws.BroadcastToRoom("room1", []byte("fill-buffer-1"), "")
	if err != nil {
		t.Fatalf("first BroadcastToRoom failed: %v", err)
	}
	_, err = ws.BroadcastToRoom("room1", []byte("fill-buffer-2"), "")
	if err != nil {
		t.Fatalf("second BroadcastToRoom failed: %v", err)
	}

	payload := []byte("hello everyone")
	successCount, err := ws.BroadcastToRoom("room1", payload, "client1")
	if err != nil {
		t.Fatalf("BroadcastToRoom failed: %v", err)
	}
	if successCount != 1 {
		t.Errorf("expected 1 successful send (client2 only, client3 buffer full), got %d", successCount)
	}

	time.Sleep(100 * time.Millisecond)

	conn2Msgs := conn2.getReceivedMessages()
	conn2BroadcastCount := 0
	for _, msg := range conn2Msgs {
		if msg.Type == MessageTypeBroadcast {
			conn2BroadcastCount++
		}
	}
	if conn2BroadcastCount < 1 {
		t.Errorf("expected client2 to receive at least 1 broadcast message, got %d", conn2BroadcastCount)
	}
}

func TestSendToClient(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)

	payload := []byte("direct message")
	err := ws.SendToClient("client1", "client2", payload)
	if err != nil {
		t.Fatalf("SendToClient failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if conn2.getSendCount() != 1 {
		t.Errorf("expected client2 to receive 1 message, got %d", conn2.getSendCount())
	}
	if conn1.getSendCount() != 0 {
		t.Errorf("expected sender to not receive message, got %d", conn1.getSendCount())
	}

	msgs := conn2.getReceivedMessages()
	if len(msgs) > 0 {
		if msgs[0].Type != MessageTypeDirect {
			t.Errorf("expected MessageTypeDirect, got %d", msgs[0].Type)
		}
		if msgs[0].From != "client1" {
			t.Errorf("expected From 'client1', got '%s'", msgs[0].From)
		}
		if msgs[0].To != "client2" {
			t.Errorf("expected To 'client2', got '%s'", msgs[0].To)
		}
		if string(msgs[0].Payload) != "direct message" {
			t.Errorf("expected payload 'direct message', got '%s'", string(msgs[0].Payload))
		}
	}

	err = ws.SendToClient("client1", "nonexistent", payload)
	if err != ErrClientOffline {
		t.Errorf("expected ErrClientOffline, got %v", err)
	}

	err = ws.SendToClient("nonexistent", "client2", payload)
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}

	err = ws.SendToClient("", "client2", payload)
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID for empty from, got %v", err)
	}

	err = ws.SendToClient("client1", "", payload)
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID for empty to, got %v", err)
	}
}

func TestSendToClientOffline(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)

	ws.Disconnect("client2")

	err := ws.SendToClient("client1", "client2", []byte("test"))
	if err != ErrClientOffline {
		t.Errorf("expected ErrClientOffline for disconnected client, got %v", err)
	}
}

func TestHandlePong(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn := newMockConn("client1")
	ws.Connect(conn)

	err := ws.HandlePong("client1")
	if err != nil {
		t.Fatalf("HandlePong failed: %v", err)
	}

	err = ws.HandlePong("nonexistent")
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}

	err = ws.HandlePong("")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestHeartbeatTimeout(t *testing.T) {
	cfg := Config{
		PingInterval: 50 * time.Millisecond,
		PongTimeout:  80 * time.Millisecond,
		SendTimeout:  10 * time.Millisecond,
	}
	ws := NewWSCenter(cfg)
	ws.Start()
	defer ws.Stop()

	conn := newMockConn("client1")
	ws.Connect(conn)

	if ws.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", ws.ClientCount())
	}

	time.Sleep(200 * time.Millisecond)

	if ws.ClientCount() != 0 {
		t.Errorf("expected client to be disconnected due to pong timeout, got %d clients", ws.ClientCount())
	}
	if !conn.isClosed() {
		t.Error("expected conn to be closed after timeout")
	}
}

func TestHeartbeatWithPongResponse(t *testing.T) {
	cfg := Config{
		PingInterval: 50 * time.Millisecond,
		PongTimeout:  150 * time.Millisecond,
		SendTimeout:  10 * time.Millisecond,
	}
	ws := NewWSCenter(cfg)
	ws.Start()
	defer ws.Stop()

	conn := newMockConn("client1")
	ws.Connect(conn)

	go func() {
		for {
			time.Sleep(60 * time.Millisecond)
			ws.HandlePong("client1")
		}
	}()

	time.Sleep(200 * time.Millisecond)

	if ws.ClientCount() != 1 {
		t.Errorf("expected client to still be connected with pong responses, got %d clients", ws.ClientCount())
	}
}

func TestDisconnectNotification(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	conn3 := newMockConn("client3")
	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.Connect(conn3)
	ws.CreateRoom("room1")
	ws.JoinRoom("client1", "room1")
	ws.JoinRoom("client2", "room1")
	ws.JoinRoom("client3", "room1")

	conn1.clearMessages()
	conn2.clearMessages()
	conn3.clearMessages()

	err := ws.Disconnect("client1")
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if conn2.getSendCount() < 1 {
		t.Errorf("expected client2 to receive leave notification, got %d messages", conn2.getSendCount())
	}
	if conn3.getSendCount() < 1 {
		t.Errorf("expected client3 to receive leave notification, got %d messages", conn3.getSendCount())
	}

	msgs := conn2.getReceivedMessages()
	foundLeave := false
	for _, msg := range msgs {
		if msg.Type == MessageTypeLeave && msg.From == "client1" {
			foundLeave = true
			if msg.RoomID != "room1" {
				t.Errorf("expected RoomID 'room1', got '%s'", msg.RoomID)
			}
			break
		}
	}
	if !foundLeave {
		t.Error("expected to find leave notification message")
	}
}

func TestJoinNotification(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.CreateRoom("room1")
	ws.JoinRoom("client1", "room1")

	conn1.clearMessages()
	conn2.clearMessages()

	err := ws.JoinRoom("client2", "room1")
	if err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if conn1.getSendCount() < 1 {
		t.Errorf("expected client1 to receive join notification, got %d messages", conn1.getSendCount())
	}

	msgs := conn1.getReceivedMessages()
	foundJoin := false
	for _, msg := range msgs {
		if msg.Type == MessageTypeJoin && msg.From == "client2" {
			foundJoin = true
			break
		}
	}
	if !foundJoin {
		t.Error("expected to find join notification message")
	}
}

func TestLeaveNotification(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.CreateRoom("room1")
	ws.JoinRoom("client1", "room1")
	ws.JoinRoom("client2", "room1")

	conn1.clearMessages()
	conn2.clearMessages()

	err := ws.LeaveRoom("client2", "room1")
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if conn1.getSendCount() < 1 {
		t.Errorf("expected client1 to receive leave notification, got %d messages", conn1.getSendCount())
	}

	msgs := conn1.getReceivedMessages()
	foundLeave := false
	for _, msg := range msgs {
		if msg.Type == MessageTypeLeave && msg.From == "client2" {
			foundLeave = true
			break
		}
	}
	if !foundLeave {
		t.Error("expected to find leave notification message")
	}
}

func TestGetClientRooms(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn := newMockConn("client1")
	ws.Connect(conn)
	ws.CreateRoom("room1")
	ws.CreateRoom("room2")
	ws.CreateRoom("room3")

	ws.JoinRoom("client1", "room1")
	ws.JoinRoom("client1", "room2")

	rooms, err := ws.GetClientRooms("client1")
	if err != nil {
		t.Fatalf("GetClientRooms failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(rooms))
	}

	ws.LeaveRoom("client1", "room1")

	rooms, err = ws.GetClientRooms("client1")
	if err != nil {
		t.Fatalf("GetClientRooms failed: %v", err)
	}
	if len(rooms) != 1 {
		t.Errorf("expected 1 room, got %d", len(rooms))
	}
	if rooms[0] != "room2" {
		t.Errorf("expected room2, got %s", rooms[0])
	}

	_, err = ws.GetClientRooms("nonexistent")
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}

	_, err = ws.GetClientRooms("")
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestClientAndRoomExists(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	if ws.ClientExists("client1") {
		t.Error("expected client1 to not exist")
	}
	if ws.RoomExists("room1") {
		t.Error("expected room1 to not exist")
	}

	conn := newMockConn("client1")
	ws.Connect(conn)
	ws.CreateRoom("room1")

	if !ws.ClientExists("client1") {
		t.Error("expected client1 to exist")
	}
	if !ws.RoomExists("room1") {
		t.Error("expected room1 to exist")
	}

	ws.Disconnect("client1")
	ws.LeaveRoom("client1", "room1")

	if ws.ClientExists("client1") {
		t.Error("expected client1 to not exist after disconnect")
	}
}

func TestStop(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	ws.Start()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.CreateRoom("room1")
	ws.JoinRoom("client1", "room1")
	ws.JoinRoom("client2", "room1")

	ws.Stop()

	if ws.ClientCount() != 0 {
		t.Errorf("expected 0 clients after Stop, got %d", ws.ClientCount())
	}
	if ws.RoomCount() != 0 {
		t.Errorf("expected 0 rooms after Stop, got %d", ws.RoomCount())
	}
	if !conn1.isClosed() {
		t.Error("expected conn1 to be closed after Stop")
	}
	if !conn2.isClosed() {
		t.Error("expected conn2 to be closed after Stop")
	}

	ws.Stop()
}

func TestConnectAfterStop(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	ws.Stop()

	conn := newMockConn("client1")
	_, err := ws.Connect(conn)
	if err != ErrCenterStopped {
		t.Errorf("expected ErrCenterStopped, got %v", err)
	}
}

func TestCreateRoomAfterStop(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	ws.Stop()

	_, err := ws.CreateRoom("room1")
	if err != ErrCenterStopped {
		t.Errorf("expected ErrCenterStopped, got %v", err)
	}
}

func TestGetOrCreateRoomAfterStop(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	ws.Stop()

	_, err := ws.GetOrCreateRoom("room1")
	if err != ErrCenterStopped {
		t.Errorf("expected ErrCenterStopped, got %v", err)
	}
}

func TestConcurrentOperations(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	var wg sync.WaitGroup
	clientCount := 10
	roomCount := 5

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn := newMockConn(fmt.Sprintf("client%d", idx))
			ws.Connect(conn)
		}(i)
	}

	for i := 0; i < roomCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ws.CreateRoom(fmt.Sprintf("room%d", idx))
		}(i)
	}

	wg.Wait()

	if ws.ClientCount() != clientCount {
		t.Errorf("expected %d clients, got %d", clientCount, ws.ClientCount())
	}
	if ws.RoomCount() != roomCount {
		t.Errorf("expected %d rooms, got %d", roomCount, ws.RoomCount())
	}

	var wg2 sync.WaitGroup
	for c := 0; c < clientCount; c++ {
		for r := 0; r < roomCount; r++ {
			wg2.Add(1)
			go func(cIdx, rIdx int) {
				defer wg2.Done()
				ws.JoinRoom(fmt.Sprintf("client%d", cIdx), fmt.Sprintf("room%d", rIdx))
			}(c, r)
		}
	}
	wg2.Wait()

	for r := 0; r < roomCount; r++ {
		clients, err := ws.GetRoomClients(fmt.Sprintf("room%d", r))
		if err != nil {
			t.Errorf("GetRoomClients failed: %v", err)
		}
		if len(clients) != clientCount {
			t.Errorf("expected %d clients in room%d, got %d", clientCount, r, len(clients))
		}
	}

	var wg3 sync.WaitGroup
	for c := 0; c < clientCount; c++ {
		wg3.Add(1)
		go func(cIdx int) {
			defer wg3.Done()
			for r := 0; r < roomCount; r++ {
				ws.BroadcastToRoom(fmt.Sprintf("room%d", r), []byte("test"), fmt.Sprintf("client%d", cIdx))
			}
		}(c)
	}
	wg3.Wait()
}

func TestBroadcastToRoomWithTimeout(t *testing.T) {
	cfg := Config{
		SendTimeout:      30 * time.Millisecond,
		ClientBufferSize: 1,
	}
	ws := NewWSCenter(cfg)
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	conn2.sendDelay = 200 * time.Millisecond

	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.CreateRoom("room1")
	ws.JoinRoom("client1", "room1")
	ws.JoinRoom("client2", "room1")

	time.Sleep(50 * time.Millisecond)
	conn1.clearMessages()
	conn2.clearMessages()

	_, err := ws.BroadcastToRoom("room1", []byte("msg1"), "")
	if err != nil {
		t.Fatalf("first BroadcastToRoom failed: %v", err)
	}

	_, err = ws.BroadcastToRoom("room1", []byte("msg2"), "")
	if err != nil {
		t.Fatalf("second BroadcastToRoom failed: %v", err)
	}

	start := time.Now()
	successCount, err := ws.BroadcastToRoom("room1", []byte("msg3"), "")
	if err != nil {
		t.Fatalf("third BroadcastToRoom failed: %v", err)
	}

	duration := time.Since(start)
	if duration > 100*time.Millisecond {
		t.Errorf("expected broadcast to complete quickly with timeout, took %v", duration)
	}
	if successCount != 1 {
		t.Errorf("expected 1 successful send (client1, client2 buffer full), got %d", successCount)
	}
}

func TestSendBufferFull(t *testing.T) {
	cfg := Config{
		ClientBufferSize: 1,
		SendTimeout:      10 * time.Millisecond,
	}
	ws := NewWSCenter(cfg)
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	conn2.sendDelay = 100 * time.Millisecond

	ws.Connect(conn1)
	ws.Connect(conn2)

	err := ws.SendToClient("client1", "client2", []byte("msg1"))
	if err != nil {
		t.Fatalf("first send failed: %v", err)
	}

	err = ws.SendToClient("client1", "client2", []byte("msg2"))
	if err != nil {
		t.Fatalf("second send (buffer) failed: %v", err)
	}

	err = ws.SendToClient("client1", "client2", []byte("msg3"))
	if err != ErrSendTimeout {
		t.Errorf("expected ErrSendTimeout for buffer full, got %v", err)
	}
}

func TestDisconnectFromMultipleRooms(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)

	for i := 0; i < 5; i++ {
		roomID := fmt.Sprintf("room%d", i)
		ws.CreateRoom(roomID)
		ws.JoinRoom("client1", roomID)
		ws.JoinRoom("client2", roomID)
	}

	time.Sleep(100 * time.Millisecond)
	conn2.clearMessages()

	err := ws.Disconnect("client1")
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	msgs := conn2.getReceivedMessages()
	leaveCount := 0
	for _, msg := range msgs {
		if msg.Type == MessageTypeLeave {
			leaveCount++
		}
	}
	if leaveCount != 5 {
		t.Errorf("expected 5 leave notifications (one per room), got %d", leaveCount)
	}

	for i := 0; i < 5; i++ {
		roomID := fmt.Sprintf("room%d", i)
		if !ws.RoomExists(roomID) {
			t.Errorf("expected room %s to still exist with client2 in it", roomID)
		}
	}

	ws.Disconnect("client2")
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 5; i++ {
		roomID := fmt.Sprintf("room%d", i)
		if ws.RoomExists(roomID) {
			t.Errorf("expected room %s to be destroyed after all clients left", roomID)
		}
	}
}

func TestPingLoopSendsPing(t *testing.T) {
	cfg := Config{
		PingInterval: 30 * time.Millisecond,
		PongTimeout:  100 * time.Millisecond,
		SendTimeout:  10 * time.Millisecond,
	}
	ws := NewWSCenter(cfg)
	ws.Start()
	defer ws.Stop()

	conn := newMockConn("client1")
	ws.Connect(conn)

	time.Sleep(80 * time.Millisecond)

	msgs := conn.getReceivedMessages()
	pingCount := 0
	for _, msg := range msgs {
		if msg.Type == MessageTypePing {
			pingCount++
		}
	}
	if pingCount < 1 {
		t.Errorf("expected at least 1 ping message, got %d", pingCount)
	}
}

func TestMessageTimestamp(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)
	ws.CreateRoom("room1")
	ws.JoinRoom("client1", "room1")
	ws.JoinRoom("client2", "room1")

	before := time.Now()
	ws.BroadcastToRoom("room1", []byte("test"), "client1")
	after := time.Now()

	time.Sleep(50 * time.Millisecond)

	msgs := conn2.getReceivedMessages()
	if len(msgs) > 0 {
		if msgs[0].Timestamp.Before(before) || msgs[0].Timestamp.After(after) {
			t.Errorf("expected timestamp to be between %v and %v, got %v", before, after, msgs[0].Timestamp)
		}
	}
}

func TestRoomCreationTime(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	before := time.Now()
	room, err := ws.CreateRoom("room1")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}
	after := time.Now()

	if room.createAt.Before(before) || room.createAt.After(after) {
		t.Errorf("expected room createAt to be between %v and %v, got %v", before, after, room.createAt)
	}
}

func TestMultipleClientsLeaveRoom(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	numClients := 5
	conns := make([]*mockConn, numClients)

	for i := 0; i < numClients; i++ {
		conns[i] = newMockConn(fmt.Sprintf("client%d", i))
		ws.Connect(conns[i])
	}

	ws.CreateRoom("room1")
	for i := 0; i < numClients; i++ {
		ws.JoinRoom(fmt.Sprintf("client%d", i), "room1")
	}

	clients, err := ws.GetRoomClients("room1")
	if err != nil {
		t.Fatalf("GetRoomClients failed: %v", err)
	}
	if len(clients) != numClients {
		t.Errorf("expected %d clients, got %d", numClients, len(clients))
	}

	for i := 0; i < numClients-1; i++ {
		ws.LeaveRoom(fmt.Sprintf("client%d", i), "room1")
		if !ws.RoomExists("room1") {
			t.Errorf("expected room1 to exist with %d clients left", numClients-i-1)
		}
	}

	ws.LeaveRoom(fmt.Sprintf("client%d", numClients-1), "room1")
	if ws.RoomExists("room1") {
		t.Error("expected room1 to be destroyed after all clients left")
	}
}

func TestSendToClientDisconnectedSender(t *testing.T) {
	ws := NewWSCenter(DefaultConfig())
	defer ws.Stop()

	conn1 := newMockConn("client1")
	conn2 := newMockConn("client2")
	ws.Connect(conn1)
	ws.Connect(conn2)

	ws.Disconnect("client1")

	err := ws.SendToClient("client1", "client2", []byte("test"))
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound for disconnected sender, got %v", err)
	}
}

func TestDefaultConfigValues(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.PingInterval != defaultPingInterval {
		t.Errorf("expected default PingInterval %v, got %v", defaultPingInterval, cfg.PingInterval)
	}
	if cfg.PongTimeout != defaultPongTimeout {
		t.Errorf("expected default PongTimeout %v, got %v", defaultPongTimeout, cfg.PongTimeout)
	}
	if cfg.SendTimeout != defaultSendTimeout {
		t.Errorf("expected default SendTimeout %v, got %v", defaultSendTimeout, cfg.SendTimeout)
	}
	if cfg.ClientBufferSize != defaultClientBufferSize {
		t.Errorf("expected default ClientBufferSize %d, got %d", defaultClientBufferSize, cfg.ClientBufferSize)
	}
	if cfg.Logger == nil {
		t.Error("expected default Logger to be set")
	}
}

func TestNewWSCenterCustomLogger(t *testing.T) {
	logger := log.New(nil, "test", 0)
	cfg := Config{
		Logger: logger,
	}
	ws := NewWSCenter(cfg)
	defer ws.Stop()
	if ws.cfg.Logger != logger {
		t.Error("expected custom logger to be used")
	}
}
