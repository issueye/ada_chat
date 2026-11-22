package rpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"time"
)

type netTransport struct{}

// RegisterNet registers the tcp and unix transports.
func RegisterNet(s *Service) error {
	t := netTransport{}
	if err := s.Register(TCP, t); err != nil {
		return err
	}
	return s.Register(Unix, t)
}

func (netTransport) Dial(ctx context.Context, addr Address) (Stream, error) {
	if addr.Network != TCP && addr.Network != Unix {
		return nil, Unimplemented("rpc-net 不支持传输 " + string(addr.Network))
	}
	var d net.Dialer
	c, err := d.DialContext(ctx, string(addr.Network), addr.Name)
	if err != nil {
		return nil, Unavailable("拨号 " + addr.String() + " 失败: " + err.Error())
	}
	return c, nil
}

func (netTransport) Listen(ctx context.Context, addr Address, opts ListenOptions) (Listener, error) {
	if err := ctx.Err(); err != nil {
		return nil, New(ErrCanceled, "rpc-net", "监听已取消", false)
	}
	if addr.Network != TCP && addr.Network != Unix {
		return nil, Unimplemented("rpc-net 不支持传输 " + string(addr.Network))
	}
	path := ""
	if addr.Network == Unix {
		if err := prepareUnix(addr.Name, opts); err != nil {
			return nil, err
		}
		path = addr.Name
	}
	ln, err := net.Listen(string(addr.Network), addr.Name)
	if err != nil {
		return nil, Unavailable("监听 " + addr.String() + " 失败: " + err.Error())
	}
	if path != "" && runtime.GOOS != "windows" {
		perm := opts.Perm
		if perm == 0 {
			perm = 0o600
		}
		if err := os.Chmod(path, perm); err != nil {
			_ = ln.Close()
			_ = os.Remove(path)
			return nil, Unavailable("设置 socket 权限失败: " + err.Error())
		}
	}
	actual := addr
	if addr.Network == TCP {
		actual = Address{Network: TCP, Name: ln.Addr().String()}
	}
	return &netListener{ln: ln, addr: actual, path: path}, nil
}

// prepareUnix checks the parent directory and any stale socket.
func prepareUnix(path string, opts ListenOptions) error {
	dir := dirOf(path)
	info, err := os.Stat(dir)
	if err != nil {
		return Invalid("unix socket 目录不存在: " + dir)
	}
	if !info.IsDir() {
		return Invalid("unix socket 父路径不是目录: " + dir)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return Denied("unix socket 目录权限必须 ≤ 0700: " + dir)
	}
	if li, err := os.Lstat(path); err == nil {
		if li.Mode()&os.ModeSymlink != 0 || li.Mode()&os.ModeSocket == 0 {
			return Invalid("路径已存在且不是 socket: " + path)
		}
		if !opts.RemoveStale {
			return Invalid("socket 已存在(未允许清理): " + path)
		}
		if err := os.Remove(path); err != nil {
			return Unavailable("清理 stale socket 失败: " + err.Error())
		}
	} else if !os.IsNotExist(err) {
		return Unavailable("检查 socket 失败: " + err.Error())
	}
	return nil
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			if i == 0 {
				return string(path[0])
			}
			return path[:i]
		}
	}
	return "."
}

type netListener struct {
	ln   net.Listener
	addr Address
	path string

	closeOnce sync.Once
}

func (l *netListener) Accept(ctx context.Context) (Stream, error) {
	dl, ok := l.ln.(interface{ SetDeadline(time.Time) error })
	if !ok {
		return nil, Internal("监听器不支持 deadline")
	}
	stop := context.AfterFunc(ctx, func() { _ = dl.SetDeadline(time.Now()) })
	c, err := l.ln.Accept()
	stop()
	_ = dl.SetDeadline(time.Time{})
	if err != nil {
		if ctx.Err() != nil {
			return nil, New(ErrCanceled, "rpc-net", "accept 已取消", false)
		}
		return nil, err
	}
	return c, nil
}

func (l *netListener) Addr() Address { return l.addr }

func (l *netListener) Close() error {
	var err error
	l.closeOnce.Do(func() {
		err = l.ln.Close()
		if l.path != "" {
			_ = os.Remove(l.path)
		}
	})
	if err != nil {
		return fmt.Errorf("rpc-net: 关闭监听器失败: %w", err)
	}
	return nil
}
