package wscenter

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrCenterStopped      = errors.New("wscenter: center is stopped")
	ErrClientNotFound     = errors.New("wscenter: client not found")
	ErrClientExists       = errors.New("wscenter: client already exists")
	ErrRoomNotFound       = errors.New("wscenter: room not found")
	ErrRoomExists         = errors.New("wscenter: room already exists")
	ErrClientNotInRoom    = errors.New("wscenter: client not in room")
	ErrClientAlreadyInRoom = errors.New("wscenter: client already in room")
	ErrInvalidID          = errors.New("wscenter: invalid id")
	ErrSendTimeout        = errors.New("wscenter: send timeout")
	ErrClientOffline      = errors.New("wscenter: client is offline")
)

const (
	defaultPingInterval     = 30 * time.Second
	defaultPongTimeout      = 10 * time.Second
	defaultSendTimeout      = 5 * time.Second
	defaultClientBufferSize = 256
)

type MessageType int

const (
	MessageTypeText MessageType = iota
	MessageTypeBinary
	MessageTypePing
	MessageTypePong
	MessageTypeJoin
	MessageTypeLeave
	MessageTypeBroadcast
	MessageTypeDirect
)

type Message struct {
	Type      MessageType
	RoomID    string
	From      string
	To        string
	Payload   []byte
	Timestamp time.Time
}

type Conn interface {
	SendMessage(msg *Message) error
	Close() error
	ID() string
}

type Config struct {
	PingInterval     time.Duration
	PongTimeout      time.Duration
	SendTimeout      time.Duration
	ClientBufferSize int
	Logger           *log.Logger
}

func DefaultConfig() Config {
	return Config{
		PingInterval:     defaultPingInterval,
		PongTimeout:      defaultPongTimeout,
		SendTimeout:      defaultSendTimeout,
		ClientBufferSize: defaultClientBufferSize,
		Logger:           log.Default(),
	}
}

type Client struct {
	id           string
	conn         Conn
	rooms        map[string]*Room
	sendCh       chan *Message
	lastPong     time.Time
	lastPingSent time.Time
	mu           sync.RWMutex
	closeOnce    sync.Once
	disconnect   bool
}

func newClient(id string, conn Conn, bufferSize int) *Client {
	return &Client{
		id:       id,
		conn:     conn,
		rooms:    make(map[string]*Room),
		sendCh:   make(chan *Message, bufferSize),
		lastPong: time.Now(),
	}
}

func (c *Client) ID() string {
	return c.id
}

func (c *Client) send(msg *Message, timeout time.Duration) error {
	if c.isDisconnected() {
		return ErrClientOffline
	}
	select {
	case c.sendCh <- msg:
		return nil
	case <-time.After(timeout):
		return ErrSendTimeout
	}
}

func (c *Client) isDisconnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.disconnect
}

func (c *Client) updatePong() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPong = time.Now()
}

func (c *Client) getLastPong() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastPong
}

func (c *Client) updatePingSent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPingSent = time.Now()
}

func (c *Client) getLastPingSent() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastPingSent
}

func (c *Client) hasPendingPing() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.lastPingSent.IsZero() && c.lastPingSent.After(c.lastPong)
}

func (c *Client) addRoom(room *Room) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rooms[room.id] = room
}

func (c *Client) removeRoom(roomID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rooms, roomID)
}

func (c *Client) getRooms() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rooms := make([]string, 0, len(c.rooms))
	for id := range c.rooms {
		rooms = append(rooms, id)
	}
	return rooms
}

func (c *Client) close() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.disconnect = true
		close(c.sendCh)
		c.mu.Unlock()
		c.conn.Close()
	})
}

type Room struct {
	id       string
	clients  map[string]*Client
	mu       sync.RWMutex
	createAt time.Time
}

func newRoom(id string) *Room {
	return &Room{
		id:       id,
		clients:  make(map[string]*Client),
		createAt: time.Now(),
	}
}

func (r *Room) ID() string {
	return r.id
}

func (r *Room) addClient(client *Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.clients[client.id]; exists {
		return ErrClientAlreadyInRoom
	}
	r.clients[client.id] = client
	return nil
}

func (r *Room) removeClient(clientID string) (*Client, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	client, exists := r.clients[clientID]
	if !exists {
		return nil, false
	}
	delete(r.clients, clientID)
	return client, true
}

func (r *Room) hasClient(clientID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.clients[clientID]
	return exists
}

func (r *Room) getClients() []*Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clients := make([]*Client, 0, len(r.clients))
	for _, c := range r.clients {
		clients = append(clients, c)
	}
	return clients
}

func (r *Room) getClientIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.clients))
	for id := range r.clients {
		ids = append(ids, id)
	}
	return ids
}

func (r *Room) clientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

func (r *Room) isEmpty() bool {
	return r.clientCount() == 0
}

type WSCenter struct {
	cfg          Config
	mu           sync.RWMutex
	clients      map[string]*Client
	knownClients map[string]bool
	rooms        map[string]*Room
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	logger       *log.Logger
	nextMsgID    uint64
}

func NewWSCenter(cfg Config) *WSCenter {
	if cfg.PingInterval <= 0 {
		cfg.PingInterval = defaultPingInterval
	}
	if cfg.PongTimeout <= 0 {
		cfg.PongTimeout = defaultPongTimeout
	}
	if cfg.SendTimeout <= 0 {
		cfg.SendTimeout = defaultSendTimeout
	}
	if cfg.ClientBufferSize <= 0 {
		cfg.ClientBufferSize = defaultClientBufferSize
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}

	return &WSCenter{
		cfg:          cfg,
		clients:      make(map[string]*Client),
		knownClients: make(map[string]bool),
		rooms:        make(map[string]*Room),
		stopCh:       make(chan struct{}),
		running:      true,
		logger:       cfg.Logger,
	}
}

func (ws *WSCenter) generateMsgID() string {
	id := atomic.AddUint64(&ws.nextMsgID, 1)
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), id)
}

func (ws *WSCenter) Start() {
	ws.mu.Lock()
	if !ws.running {
		ws.running = true
		ws.stopCh = make(chan struct{})
	}
	ws.mu.Unlock()

	ws.wg.Add(1)
	go ws.pingLoop()
}

func (ws *WSCenter) Stop() {
	ws.mu.Lock()
	if !ws.running {
		ws.mu.Unlock()
		return
	}
	ws.running = false
	close(ws.stopCh)

	for _, client := range ws.clients {
		client.close()
	}
	ws.clients = make(map[string]*Client)
	ws.knownClients = make(map[string]bool)
	ws.rooms = make(map[string]*Room)
	ws.mu.Unlock()

	ws.wg.Wait()
}

func (ws *WSCenter) Connect(conn Conn) (*Client, error) {
	if conn == nil {
		return nil, ErrInvalidID
	}
	id := conn.ID()
	if id == "" {
		return nil, ErrInvalidID
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.running {
		return nil, ErrCenterStopped
	}

	if _, exists := ws.clients[id]; exists {
		return nil, ErrClientExists
	}

	client := newClient(id, conn, ws.cfg.ClientBufferSize)
	ws.clients[id] = client
	ws.knownClients[id] = true

	ws.wg.Add(1)
	go ws.clientWriteLoop(client)

	return client, nil
}

func (ws *WSCenter) Disconnect(clientID string) error {
	if clientID == "" {
		return ErrInvalidID
	}

	ws.mu.Lock()
	client, exists := ws.clients[clientID]
	if !exists {
		ws.mu.Unlock()
		return ErrClientNotFound
	}

	client.mu.Lock()
	if client.disconnect {
		client.mu.Unlock()
		ws.mu.Unlock()
		return nil
	}
	client.disconnect = true
	client.mu.Unlock()

	client.mu.RLock()
	roomObjs := make([]*Room, 0, len(client.rooms))
	for _, room := range client.rooms {
		roomObjs = append(roomObjs, room)
	}
	client.mu.RUnlock()
	ws.mu.Unlock()

	for _, room := range roomObjs {
		removedClient, ok := room.removeClient(clientID)
		if ok {
			removedClient.removeRoom(room.id)
			ws.broadcastLeave(room, removedClient)
		}
		if room.isEmpty() {
			ws.mu.Lock()
			if room.isEmpty() {
				delete(ws.rooms, room.id)
			}
			ws.mu.Unlock()
		}
	}

	client.close()

	ws.mu.Lock()
	delete(ws.clients, clientID)
	ws.mu.Unlock()

	return nil
}

func (ws *WSCenter) CreateRoom(roomID string) (*Room, error) {
	if roomID == "" {
		return nil, ErrInvalidID
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.running {
		return nil, ErrCenterStopped
	}

	if _, exists := ws.rooms[roomID]; exists {
		return nil, ErrRoomExists
	}

	room := newRoom(roomID)
	ws.rooms[roomID] = room
	return room, nil
}

func (ws *WSCenter) GetOrCreateRoom(roomID string) (*Room, error) {
	if roomID == "" {
		return nil, ErrInvalidID
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.running {
		return nil, ErrCenterStopped
	}

	if room, exists := ws.rooms[roomID]; exists {
		return room, nil
	}

	room := newRoom(roomID)
	ws.rooms[roomID] = room
	return room, nil
}

func (ws *WSCenter) JoinRoom(clientID, roomID string) error {
	if clientID == "" || roomID == "" {
		return ErrInvalidID
	}

	ws.mu.Lock()
	client, clientExists := ws.clients[clientID]
	room, roomExists := ws.rooms[roomID]
	if !clientExists {
		ws.mu.Unlock()
		return ErrClientNotFound
	}
	if !roomExists {
		ws.mu.Unlock()
		return ErrRoomNotFound
	}
	if client.isDisconnected() {
		ws.mu.Unlock()
		return ErrClientOffline
	}

	if err := room.addClient(client); err != nil {
		ws.mu.Unlock()
		return err
	}
	client.addRoom(room)
	ws.mu.Unlock()

	ws.broadcastJoin(room, client)
	return nil
}

func (ws *WSCenter) LeaveRoom(clientID, roomID string) error {
	if clientID == "" || roomID == "" {
		return ErrInvalidID
	}

	ws.mu.RLock()
	_, clientExists := ws.clients[clientID]
	room, roomExists := ws.rooms[roomID]
	ws.mu.RUnlock()

	if !clientExists {
		return ErrClientNotFound
	}
	if !roomExists {
		return ErrRoomNotFound
	}

	removedClient, ok := room.removeClient(clientID)
	if !ok {
		return ErrClientNotInRoom
	}
	removedClient.removeRoom(roomID)

	ws.broadcastLeave(room, removedClient)

	if room.isEmpty() {
		ws.mu.Lock()
		if room.isEmpty() {
			delete(ws.rooms, roomID)
		}
		ws.mu.Unlock()
	}

	return nil
}

func (ws *WSCenter) GetRoomClients(roomID string) ([]string, error) {
	if roomID == "" {
		return nil, ErrInvalidID
	}

	ws.mu.RLock()
	room, exists := ws.rooms[roomID]
	ws.mu.RUnlock()

	if !exists {
		return nil, ErrRoomNotFound
	}

	return room.getClientIDs(), nil
}

func (ws *WSCenter) RoomExists(roomID string) bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	_, exists := ws.rooms[roomID]
	return exists
}

func (ws *WSCenter) ClientExists(clientID string) bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	_, exists := ws.clients[clientID]
	return exists
}

func (ws *WSCenter) BroadcastToRoom(roomID string, payload []byte, fromClientID string) (int, error) {
	if roomID == "" {
		return 0, ErrInvalidID
	}

	ws.mu.RLock()
	room, exists := ws.rooms[roomID]
	ws.mu.RUnlock()

	if !exists {
		return 0, ErrRoomNotFound
	}

	clients := room.getClients()
	successCount := 0

	for _, client := range clients {
		if client.id == fromClientID {
			continue
		}

		msg := &Message{
			Type:      MessageTypeBroadcast,
			RoomID:    roomID,
			From:      fromClientID,
			To:        client.id,
			Payload:   payload,
			Timestamp: time.Now(),
		}

		if err := client.send(msg, ws.cfg.SendTimeout); err != nil {
			ws.logger.Printf("wscenter: broadcast to client %s failed: %v", client.id, err)
		} else {
			successCount++
		}
	}

	return successCount, nil
}

func (ws *WSCenter) SendToClient(fromClientID, toClientID string, payload []byte) error {
	if fromClientID == "" || toClientID == "" {
		return ErrInvalidID
	}

	ws.mu.RLock()
	fromClient, fromExists := ws.clients[fromClientID]
	toClient, toExists := ws.clients[toClientID]
	fromKnown := ws.knownClients[fromClientID]
	toKnown := ws.knownClients[toClientID]
	ws.mu.RUnlock()

	if !fromExists {
		if fromKnown {
			return ErrClientOffline
		}
		return ErrClientNotFound
	}
	if !toExists {
		if toKnown {
			return ErrClientOffline
		}
		return ErrClientNotFound
	}
	if fromClient.isDisconnected() {
		return ErrClientOffline
	}
	if toClient.isDisconnected() {
		return ErrClientOffline
	}

	msg := &Message{
		Type:      MessageTypeDirect,
		From:      fromClientID,
		To:        toClientID,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	return toClient.send(msg, ws.cfg.SendTimeout)
}

func (ws *WSCenter) HandlePong(clientID string) error {
	if clientID == "" {
		return ErrInvalidID
	}

	ws.mu.RLock()
	client, exists := ws.clients[clientID]
	ws.mu.RUnlock()

	if !exists {
		return ErrClientNotFound
	}

	client.updatePong()
	return nil
}

func (ws *WSCenter) ClientCount() int {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return len(ws.clients)
}

func (ws *WSCenter) RoomCount() int {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return len(ws.rooms)
}

func (ws *WSCenter) GetClientRooms(clientID string) ([]string, error) {
	if clientID == "" {
		return nil, ErrInvalidID
	}

	ws.mu.RLock()
	client, exists := ws.clients[clientID]
	ws.mu.RUnlock()

	if !exists {
		return nil, ErrClientNotFound
	}

	return client.getRooms(), nil
}

func (ws *WSCenter) pingLoop() {
	defer ws.wg.Done()

	ticker := time.NewTicker(ws.cfg.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ws.stopCh:
			return
		case <-ticker.C:
			ws.checkTimeouts()
			ws.sendPings()
		}
	}
}

func (ws *WSCenter) sendPings() {
	ws.mu.RLock()
	clients := make([]*Client, 0, len(ws.clients))
	for _, c := range ws.clients {
		clients = append(clients, c)
	}
	ws.mu.RUnlock()

	for _, client := range clients {
		if client.isDisconnected() {
			continue
		}
		msg := &Message{
			Type:      MessageTypePing,
			To:        client.id,
			Timestamp: time.Now(),
		}
		if err := client.send(msg, ws.cfg.SendTimeout); err != nil {
			ws.logger.Printf("wscenter: send ping to client %s failed: %v", client.id, err)
		} else {
			client.updatePingSent()
		}
	}
}

func (ws *WSCenter) checkTimeouts() {
	ws.mu.RLock()
	clients := make([]*Client, 0, len(ws.clients))
	for _, c := range ws.clients {
		clients = append(clients, c)
	}
	ws.mu.RUnlock()

	now := time.Now()

	for _, client := range clients {
		if client.isDisconnected() {
			continue
		}

		if !client.hasPendingPing() {
			continue
		}

		lastPingSent := client.getLastPingSent()
		timeSincePing := now.Sub(lastPingSent)
		if timeSincePing >= ws.cfg.PongTimeout {
			ws.logger.Printf("wscenter: client %s pong timeout, disconnecting", client.id)
			ws.Disconnect(client.id)
		}
	}
}

func (ws *WSCenter) clientWriteLoop(client *Client) {
	defer ws.wg.Done()

	for {
		select {
		case <-ws.stopCh:
			return
		case msg, ok := <-client.sendCh:
			if !ok {
				return
			}
			if err := client.conn.SendMessage(msg); err != nil {
				ws.logger.Printf("wscenter: write to client %s failed: %v", client.id, err)
			}
		}
	}
}

func (ws *WSCenter) broadcastJoin(room *Room, client *Client) {
	clients := room.getClients()
	payload := []byte(fmt.Sprintf("client %s joined room %s", client.id, room.id))

	for _, c := range clients {
		if c.id == client.id {
			continue
		}
		msg := &Message{
			Type:      MessageTypeJoin,
			RoomID:    room.id,
			From:      client.id,
			To:        c.id,
			Payload:   payload,
			Timestamp: time.Now(),
		}
		if err := c.send(msg, ws.cfg.SendTimeout); err != nil {
			ws.logger.Printf("wscenter: broadcast join to client %s failed: %v", c.id, err)
		}
	}
}

func (ws *WSCenter) broadcastLeave(room *Room, client *Client) {
	clients := room.getClients()
	payload := []byte(fmt.Sprintf("client %s left room %s", client.id, room.id))

	for _, c := range clients {
		msg := &Message{
			Type:      MessageTypeLeave,
			RoomID:    room.id,
			From:      client.id,
			To:        c.id,
			Payload:   payload,
			Timestamp: time.Now(),
		}
		if err := c.send(msg, ws.cfg.SendTimeout); err != nil {
			ws.logger.Printf("wscenter: broadcast leave to client %s failed: %v", c.id, err)
		}
	}
}
