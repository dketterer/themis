package xhttpserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"time"
	"xlog/xloghttp"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/justinas/alice"
)

const (
	defaultTCPKeepAlivePeriod time.Duration = 3 * time.Minute // the value used internally by net/http
)

var (
	ErrNoAddress                      = errors.New("A server bind address must be specified")
	ErrTlsCertificateRequired         = errors.New("Both a certificateFile and keyFile are required")
	ErrUnableToAddClientCACertificate = errors.New("Unable to add client CA certificate")
)

type Tls struct {
	CertificateFile         string
	KeyFile                 string
	ClientCACertificateFile string
	ServerName              string
	NextProtos              []string
	MinVersion              uint16
	MaxVersion              uint16
}

type Options struct {
	Name    string
	Address string
	Tls     *Tls

	LogConnectionState    bool
	DisableHTTPKeepAlives bool
	MaxHeaderBytes        int

	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration

	DisableTCPKeepAlives bool
	TCPKeepAlivePeriod   time.Duration

	Header               http.Header
	DisableTracking      bool
	DisableHandlerLogger bool
	DisableParseForm     bool
}

// Interface is the expected behavior of a server
type Interface interface {
	Serve(l net.Listener) error
	ServeTLS(l net.Listener, cert, key string) error
	Shutdown(context.Context) error
}

type tcpKeepAliveListener struct {
	*net.TCPListener
	period time.Duration
}

func NewTlsConfig(t *Tls) (*tls.Config, error) {
	if t == nil {
		return nil, nil
	}

	if len(t.CertificateFile) == 0 || len(t.KeyFile) == 0 {
		return nil, ErrTlsCertificateRequired
	}

	var nextProtos []string
	if len(t.NextProtos) > 0 {
		for _, np := range t.NextProtos {
			nextProtos = append(nextProtos, np)
		}
	} else {
		// assume http/1.1 by default
		nextProtos = append(nextProtos, "http/1.1")
	}

	tc := &tls.Config{
		MinVersion: t.MinVersion,
		MaxVersion: t.MaxVersion,
		ServerName: t.ServerName,
		NextProtos: nextProtos,
	}

	if cert, err := tls.LoadX509KeyPair(t.CertificateFile, t.KeyFile); err != nil {
		return nil, err
	} else {
		tc.Certificates = []tls.Certificate{cert}
	}

	if len(t.ClientCACertificateFile) > 0 {
		caCert, err := ioutil.ReadFile(t.ClientCACertificateFile)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, ErrUnableToAddClientCACertificate
		}

		tc.ClientCAs = caCertPool
		tc.ClientAuth = tls.RequireAndVerifyClientCert
	}

	tc.BuildNameToCertificate()
	return tc, nil
}

func NewListener(o Options, ctx context.Context, lcfg net.ListenConfig) (net.Listener, error) {
	address := o.Address
	if len(address) == 0 {
		address = ":http"
	}

	tc, err := NewTlsConfig(o.Tls)
	if err != nil {
		return nil, err
	}

	l, err := lcfg.Listen(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}

	if tc != nil {
		l = tls.NewListener(l, tc)
	}

	if !o.DisableTCPKeepAlives {
		period := o.TCPKeepAlivePeriod
		if period <= 0 {
			period = defaultTCPKeepAlivePeriod
		}

		l = tcpKeepAliveListener{
			TCPListener: l.(*net.TCPListener),
			period:      period,
		}
	}

	return l, nil
}

// NewServerLogger returns a go-kit Logger enriched with information about the server.
func NewServerLogger(o Options, base log.Logger, extra ...interface{}) log.Logger {
	address := o.Address
	if len(address) == 0 {
		address = ":http"
	}

	parameters := []interface{}{AddressKey(), address}
	if len(o.Name) > 0 {
		parameters = append(parameters, ServerKey(), o.Name)
	}

	return log.WithPrefix(base, append(parameters, extra...)...)
}

// NewServerChain produces the standard constructor chain for a server, primarily using configuration.
func NewServerChain(o Options, l log.Logger, pb ...xloghttp.ParameterBuilder) alice.Chain {
	chain := alice.New(
		ResponseHeaders{Header: o.Header}.Then,
	)

	if !o.DisableTracking {
		chain = chain.Append(UseTrackingWriter)
	}

	if !o.DisableHandlerLogger {
		chain = chain.Append(
			xloghttp.Logging{Base: l, Builders: pb}.Then,
		)
	}

	return chain
}

// New constructs a basic HTTP server instance.  The supplied logger is enriched with information
// about the server and returned for use by higher-level code.
func New(o Options, l log.Logger, h http.Handler) Interface {
	if len(o.Address) == 0 {
		o.Address = ":http"
	}

	s := &http.Server{
		// we don't need this technically, because we create a listener
		// it's here for other code to inspect
		Addr:    o.Address,
		Handler: h,

		MaxHeaderBytes:    o.MaxHeaderBytes,
		IdleTimeout:       o.IdleTimeout,
		ReadHeaderTimeout: o.ReadHeaderTimeout,
		ReadTimeout:       o.ReadTimeout,
		WriteTimeout:      o.WriteTimeout,

		ErrorLog: xloghttp.NewErrorLog(
			o.Address,
			log.WithPrefix(l, level.Key(), level.ErrorValue()),
		),
	}

	if o.LogConnectionState {
		s.ConnState = xloghttp.NewConnStateLogger(
			l,
			"connState",
			level.DebugValue(),
		)
	}

	if o.DisableHTTPKeepAlives {
		s.SetKeepAlivesEnabled(false)
	}

	return s
}
