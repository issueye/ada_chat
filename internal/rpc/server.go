package rpc

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type server struct {
	ln    Listener
	ctx   context.Context
	opts  ServerOptions
	conf  config
	modes []string

	mu      sync.RWMutex
	methods map[string]Handler

	pubMu sync.Mutex
	conns map[*conn]struct{}
	seq   map[string]uint64

	peerSeq  atomic.Uint64
	frames   atomic.Uint64
	out      atomic.Uint64
	events   atomic.Uint64
	inflight atomic.Int64

	wg        sync.WaitGroup
	done      chan struct{}
	closeOnce sync.Once
}

func newServer(ctx context.Context, ln Listener, opts ServerOptions, conf config) *server {
	return &server{
		ln: ln, ctx: ctx, opts: opts, conf: conf,
		modes:   []string{string(opts.Auth.Mode)},
		methods: make(map[string]Handler),
		conns:   make(map[*conn]struct{}),
		seq:     make(map[string]uint64),
		done:    make(chan struct{}),
	}
}

func (s *server) Handle(method string, h Handler) (func(), error) {
	if !validMethod(method) {
		return nil, Invalid("方法名非法: " + method)
	}
	if h == nil {
		return nil, Invalid("handler 不能为 nil")
	}
	s.mu.Lock()
	if _, ok := s.methods[method]; ok {
		s.mu.Unlock()
		return nil, Invalid("方法已注册: " + method)
	}
	s.methods[method] = h
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.methods, method)
		s.mu.Unlock()
	}, nil
}

func (s *server) Publish(topic string, data any) error {
	if topic == "" {
		return Invalid("topic 不能为空")
	}
	b, err := json.Marshal(data)
	if err != nil {
		return Invalid("事件编码失败: " + err.Error())
	}
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	s.mu.Lock()
	s.seq[topic]++
	seq := s.seq[topic]
	targets := make([]*conn, 0, len(s.conns))
	for c := range s.conns {
		targets = append(targets, c)
	}
	s.mu.Unlock()
	ev := event{V: protocolVersion, Topic: topic, Seq: seq, Data: b}
	for _, c := range targets {
		if !c.subscribed(topic) {
			continue
		}
		if err := c.send(kindEvent, ev); err != nil {
			c.close()
			continue
		}
		s.events.Add(1)
	}
	return nil
}

func (s *server) Serve(ctx context.Context) error {
	s.ctx = ctx
	for {
		st, err := s.ln.Accept(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				_ = s.Close()
				return nil
			case <-s.done:
				return nil
			default:
			}
			return Unavailable("accept 失败: " + err.Error())
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handle(st)
		}()
	}
}

func (s *server) Stats() Stats {
	s.mu.RLock()
	conns := len(s.conns)
	s.mu.RUnlock()
	return Stats{
		Listeners: []Address{s.ln.Addr()},
		Conns:     conns, Inflight: int(s.inflight.Load()),
		FramesIn: s.frames.Load(), FramesOut: s.out.Load(), EventsOut: s.events.Load(),
	}
}

func (s *server) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		_ = s.ln.Close()
		s.mu.Lock()
		for c := range s.conns {
			c.close()
		}
		s.mu.Unlock()
	})
	s.wg.Wait()
	return nil
}

func (s *server) handle(stream Stream) {
	s.mu.RLock()
	n := len(s.conns)
	s.mu.RUnlock()
	if n >= s.conf.maxConns {
		_ = writeFrame(stream, kindBye, byeMsg{V: protocolVersion, Code: string(ErrLimit), Message: "连接数超过上限"}, s.conf.maxFrameBytes)
		_ = stream.Close()
		return
	}
	c := &conn{srv: s, stream: stream, rd: bufio.NewReader(stream), cancels: make(map[string]context.CancelFunc)}
	defer c.close()

	peerName, err := s.handshake(c, stream)
	if err != nil {
		_ = c.send(kindBye, byeMsg{V: protocolVersion, Code: string(ErrAuth), Message: "鉴权失败"})
		return
	}
	c.peer = peerName
	if err := c.send(kindReady, ready{V: protocolVersion, Peer: peerName, Features: []string{"cancel", "events", "ping"}}); err != nil {
		return
	}
	s.mu.Lock()
	s.conns[c] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
	}()
	c.readLoop()
}

func (s *server) handshake(c *conn, stream Stream) (string, error) {
	nonce, err := randomHex(16)
	if err != nil {
		return "", Internal("生成 nonce 失败")
	}
	if err := c.send(kindChallenge, challenge{V: protocolVersion, Nonce: nonce, Modes: s.modes}); err != nil {
		return "", err
	}
	// Named pipes do not support deadlines; close the stream after 5s as a backstop.
	timer := time.AfterFunc(5*time.Second, func() { _ = stream.Close() })
	defer timer.Stop()
	_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
	kind, body, err := readFrame(c.rd, s.conf.maxFrameBytes)
	_ = stream.SetDeadline(time.Time{})
	if err != nil || kind != kindAuth {
		return "", AuthFailed("握手期望 auth")
	}
	var a authMsg
	if err := json.Unmarshal(body, &a); err != nil || a.V != protocolVersion {
		return "", AuthFailed("auth 非法")
	}
	if !validClient(a.Client) {
		return "", AuthFailed("客户端名非法")
	}
	if !s.peerAllowed(a.Client) {
		return "", AuthFailed("客户端不在允许表: " + a.Client)
	}
	if s.opts.Auth.Mode == AuthToken {
		want := proof(s.opts.Auth.Token, nonce, a.Client)
		if subtle.ConstantTimeCompare([]byte(a.Proof), []byte(want)) != 1 {
			return "", AuthFailed("token 不匹配")
		}
	}
	return fmt.Sprintf("%s#%d", a.Client, s.peerSeq.Add(1)), nil
}

func (s *server) peerAllowed(client string) bool {
	for _, id := range s.opts.PeerIDs {
		if id == client {
			return true
		}
	}
	return false
}

func (s *server) handler(method string) (Handler, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.methods[method]
	return h, ok
}

func (s *server) countIn() { s.frames.Add(1) }

func (s *server) countOut() { s.out.Add(1) }

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type conn struct {
	srv    *server
	stream Stream
	rd     *bufio.Reader
	peer   string

	writeMu sync.Mutex
	mu      sync.Mutex
	topics  []string
	cancels map[string]context.CancelFunc

	sem    chan struct{}
	closed atomic.Bool
}

func (c *conn) subscribed(topic string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.topics {
		if topicMatch(p, topic) {
			return true
		}
	}
	return false
}

func (c *conn) send(kind byte, body any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed.Load() {
		return Unavailable("连接已关闭")
	}
	err := writeFrame(c.stream, kind, body, c.srv.conf.maxFrameBytes)
	if err == nil {
		c.srv.countOut()
	}
	return err
}

func (c *conn) close() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}
	_ = c.stream.Close()
	c.mu.Lock()
	for _, cancel := range c.cancels {
		cancel()
	}
	c.cancels = map[string]context.CancelFunc{}
	c.mu.Unlock()
}

func (c *conn) readLoop() {
	c.sem = make(chan struct{}, c.srv.conf.maxInflight)
	for {
		kind, body, err := readFrame(c.rd, c.srv.conf.maxFrameBytes)
		if err != nil {
			return
		}
		c.srv.countIn()
		switch kind {
		case kindRequest:
			c.onRequest(body)
		case kindCancel:
			var m cancelMsg
			if json.Unmarshal(body, &m) == nil {
				c.cancel(m.ID)
			}
		case kindSubscribe:
			var m subscribeMsg
			if json.Unmarshal(body, &m) != nil || len(m.Topics) == 0 {
				return
			}
			c.mu.Lock()
			c.topics = append(c.topics, m.Topics...)
			c.mu.Unlock()
		case kindPing:
			_ = c.send(kindPong, pingMsg{V: protocolVersion, Ts: time.Now().UnixMilli()})
		case kindPong:
		case kindBye:
			return
		default:
			_ = c.send(kindBye, byeMsg{V: protocolVersion, Code: string(ErrInvalid), Message: "未知帧"})
			return
		}
	}
}

func (c *conn) cancel(id string) {
	c.mu.Lock()
	cancel := c.cancels[id]
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *conn) onRequest(body []byte) {
	var req request
	if err := json.Unmarshal(body, &req); err != nil || req.V != protocolVersion || req.ID == "" {
		_ = c.send(kindBye, byeMsg{V: protocolVersion, Code: string(ErrInvalid), Message: "request 非法"})
		return
	}
	if !validMethod(req.Method) {
		c.reply(req, nil, Invalid("方法名非法"))
		return
	}
	if len(req.Params) > c.srv.conf.maxParamsBytes {
		c.reply(req, nil, Limit("参数超过上限"))
		return
	}
	h, ok := c.srv.handler(req.Method)
	if !ok {
		c.reply(req, nil, Unimplemented("未知方法: "+req.Method))
		return
	}
	select {
	case c.sem <- struct{}{}:
	default:
		c.reply(req, nil, Limit("在飞请求超过上限"))
		return
	}
	// A connection may not reuse an active id; responses would cross wires.
	c.mu.Lock()
	if _, dup := c.cancels[req.ID]; dup {
		c.mu.Unlock()
		<-c.sem
		c.reply(req, nil, Invalid("请求 id 重复"))
		return
	}
	ctx, cancel := context.WithCancel(c.srv.ctx)
	if c.srv.opts.Timeout > 0 {
		cancel()
		ctx, cancel = context.WithTimeout(c.srv.ctx, c.srv.opts.Timeout)
	}
	c.cancels[req.ID] = cancel
	c.mu.Unlock()
	c.srv.inflight.Add(1)

	go func() {
		defer func() {
			cancel()
			c.mu.Lock()
			delete(c.cancels, req.ID)
			c.mu.Unlock()
			c.srv.inflight.Add(-1)
			<-c.sem
		}()
		result, herr := safeCall(ctx, h, req.Params)
		if req.NoReply {
			return
		}
		c.reply(req, result, herr)
	}()
}

// safeCall turns a handler panic into EINTERNAL; the detail stays local.
func safeCall(ctx context.Context, h Handler, params json.RawMessage) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			result, err = nil, Internal("handler 执行失败")
		}
	}()
	return h(ctx, params)
}

func (c *conn) reply(req request, result any, err error) {
	if req.NoReply {
		return
	}
	r := response{V: protocolVersion, ID: req.ID, Error: toWire(err)}
	if err == nil && result != nil {
		b, mErr := json.Marshal(result)
		if mErr != nil {
			r.Result, r.Error = nil, toWire(Internal("结果编码失败"))
		} else {
			r.Result = b
		}
	}
	_ = c.send(kindResponse, r)
}
