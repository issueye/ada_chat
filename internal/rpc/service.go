package rpc

import (
	"context"
	"time"
)

const (
	defaultHeartbeat  = 30 * time.Second
	defaultMaxParams  = 1 << 20
	defaultInflight   = 32
	defaultEventQueue = 1024
	defaultMaxConns   = 64
	minFrame          = 4 << 10
	maxFrameCap       = 64 << 20
)

type config struct {
	heartbeat      time.Duration
	maxFrameBytes  int
	maxParamsBytes int
	maxInflight    int
	maxEventQueue  int
	maxConns       int
}

func defaultConfig() config {
	return config{
		heartbeat:      defaultHeartbeat,
		maxFrameBytes:  defaultMaxFrame,
		maxParamsBytes: defaultMaxParams,
		maxInflight:    defaultInflight,
		maxEventQueue:  defaultEventQueue,
		maxConns:       defaultMaxConns,
	}
}

func (c *config) normalize() {
	if c.heartbeat <= 0 {
		c.heartbeat = defaultHeartbeat
	}
	if c.maxFrameBytes <= 0 {
		c.maxFrameBytes = defaultMaxFrame
	}
	if c.maxFrameBytes < minFrame {
		c.maxFrameBytes = minFrame
	}
	if c.maxFrameBytes > maxFrameCap {
		c.maxFrameBytes = maxFrameCap
	}
	if c.maxParamsBytes <= 0 {
		c.maxParamsBytes = defaultMaxParams
	}
	if c.maxInflight <= 0 {
		c.maxInflight = defaultInflight
	}
	if c.maxEventQueue <= 0 {
		c.maxEventQueue = defaultEventQueue
	}
	if c.maxConns <= 0 {
		c.maxConns = defaultMaxConns
	}
}

func (c *config) applyLimits(l Limits) {
	if l.MaxFrameBytes > 0 {
		c.maxFrameBytes = l.MaxFrameBytes
	}
	if l.MaxParamsBytes > 0 {
		c.maxParamsBytes = l.MaxParamsBytes
	}
	if l.MaxInflight > 0 {
		c.maxInflight = l.MaxInflight
	}
	if l.MaxEventQueue > 0 {
		c.maxEventQueue = l.MaxEventQueue
	}
	if l.MaxConns > 0 {
		c.maxConns = l.MaxConns
	}
	c.normalize()
}

// Dial connects to addr and completes the v1 handshake.
func (s *Service) Dial(ctx context.Context, addr Address, opts DialOptions) (Peer, error) {
	tr, ok := s.transports[addr.Network]
	if !ok {
		return nil, Unimplemented("没有注册传输 " + string(addr.Network))
	}
	st, err := tr.Dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	conf := defaultConfig()
	conf.normalize()
	p, err := newPeer(ctx, st, opts, conf)
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	return p, nil
}

// Listen binds addr and returns a server; Serve must be called to accept.
func (s *Service) Listen(ctx context.Context, addr Address, opts ServerOptions) (Server, error) {
	tr, ok := s.transports[addr.Network]
	if !ok {
		return nil, Unimplemented("没有注册传输 " + string(addr.Network))
	}
	if opts.Auth.Mode == "" {
		if addr.Network == TCP {
			opts.Auth.Mode = AuthToken
		} else {
			opts.Auth.Mode = AuthOS
		}
	}
	switch opts.Auth.Mode {
	case AuthOS:
		if addr.Network == TCP {
			return nil, Invalid("tcp 不能用 os 鉴权")
		}
	case AuthToken:
		if len(opts.Auth.Token) == 0 {
			return nil, AuthFailed("token 鉴权缺少 token")
		}
	case AuthNone:
		if !opts.Auth.AllowInsecure {
			return nil, AuthFailed("none 鉴权必须显式 AllowInsecure")
		}
	default:
		return nil, Invalid("未知鉴权模式: " + string(opts.Auth.Mode))
	}
	if len(opts.PeerIDs) == 0 {
		return nil, Invalid("必须配置至少一个允许的客户端名")
	}
	ln, err := tr.Listen(ctx, addr, ListenOptions{Perm: 0o600})
	if err != nil {
		return nil, err
	}
	conf := defaultConfig()
	conf.applyLimits(opts.Limits)
	srv := newServer(ctx, ln, opts, conf)
	srv.modes = []string{string(opts.Auth.Mode)}
	return srv, nil
}
