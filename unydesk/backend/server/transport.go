package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/quic-go/quic-go/http3"
)

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) HTTP3Addr() string {
	return s.http3Addr
}

func (s *Server) NativeHTTP3Enabled() bool {
	return s.http3Server != nil
}

func (s *Server) ListenAndServe() error {
	if s.http3Server != nil {
		go func() {
			s.logger.Info("http3 listening", "addr", s.http3Addr)
			if err := s.http3Server.ListenAndServeTLS("", ""); !transportClosedError(err) {
				s.logger.Error("http3 stopped", "err", err)
			}
		}()
	}
	if s.redirectServer != nil {
		go func() {
			s.logger.Info("http redirect listening", "addr", s.redirectServer.Addr, "target", s.addr)
			if err := s.redirectServer.ListenAndServe(); !transportClosedError(err) {
				s.logger.Error("http redirect stopped", "err", err)
			}
		}()
	}

	if s.httpServer.TLSConfig != nil && len(s.httpServer.TLSConfig.Certificates) > 0 {
		err := s.httpServer.ListenAndServeTLS("", "")
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}

	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.redirectServer != nil {
		_ = s.redirectServer.Shutdown(ctx)
	}
	if s.http3Server != nil {
		_ = s.http3Server.Close()
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) configureTransport(handler http.Handler) http.Handler {
	s.addr = s.cfg.ListenAddr
	if !s.cfg.HTTP3.Enabled {
		s.httpServer = &http.Server{
			Addr:    s.cfg.ListenAddr,
			Handler: handler,
		}
		return handler
	}

	if strings.TrimSpace(s.cfg.HTTP3.CertFile) == "" || strings.TrimSpace(s.cfg.HTTP3.KeyFile) == "" {
		s.logger.Warn("http3 enabled without cert/key; falling back to plain HTTP", "listen_addr", s.cfg.ListenAddr)
		s.httpServer = &http.Server{
			Addr:    s.cfg.ListenAddr,
			Handler: handler,
		}
		return handler
	}

	cert, err := tls.LoadX509KeyPair(s.cfg.HTTP3.CertFile, s.cfg.HTTP3.KeyFile)
	if err != nil {
		s.logger.Error("http3 tls load failed; falling back to plain HTTP", "err", err, "cert", s.cfg.HTTP3.CertFile)
		s.httpServer = &http.Server{
			Addr:    s.cfg.ListenAddr,
			Handler: handler,
		}
		return handler
	}

	tlsAddr := replacePort(s.cfg.ListenAddr, s.cfg.HTTP3.Port)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h3", "h2", "http/1.1"},
	}

	handler = withAltSvcHeader(handler, s.cfg.HTTP3.Port)
	s.httpServer = &http.Server{
		Addr:      tlsAddr,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}
	s.addr = tlsAddr
	s.http3Addr = tlsAddr
	s.http3Server = &http3.Server{
		Addr:      tlsAddr,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	if s.cfg.HTTP3.RedirectHTTP && strings.TrimSpace(s.cfg.ListenAddr) != strings.TrimSpace(tlsAddr) {
		redirectTargetPort := s.cfg.HTTP3.Port
		s.redirectServer = &http.Server{
			Addr: s.cfg.ListenAddr,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, httpsRedirectTarget(r.Host, redirectTargetPort)+r.URL.RequestURI(), http.StatusMovedPermanently)
			}),
		}
	}

	s.logger.Info("native http3 configured",
		"https_addr", tlsAddr,
		"http3_addr", s.http3Addr,
		"cert_file", s.cfg.HTTP3.CertFile,
		"redirect_http", s.cfg.HTTP3.RedirectHTTP,
	)
	return handler
}

func withAltSvcHeader(next http.Handler, port int) http.Handler {
	altSvc := fmt.Sprintf(`h3=":%d"; ma=86400`, port)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Alt-Svc", altSvc)
		next.ServeHTTP(w, r)
	})
}

func replacePort(addr string, port int) string {
	host := strings.TrimSpace(addr)
	if host == "" {
		return fmt.Sprintf(":%d", port)
	}
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	} else if strings.HasPrefix(host, ":") {
		host = ""
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func httpsRedirectTarget(hostport string, port int) string {
	host := strings.TrimSpace(hostport)
	if host == "" {
		host = "localhost"
	}
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	} else if strings.HasPrefix(host, "[") && strings.Contains(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	if port == 443 {
		return "https://" + host
	}
	return "https://" + net.JoinHostPort(host, strconv.Itoa(port))
}

func transportClosedError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, http.ErrServerClosed) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "server closed") || strings.Contains(lower, "use of closed network connection")
}
