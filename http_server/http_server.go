package http_server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/quic-go/quic-go/http3"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"golang.org/x/net/http2"

	"github.com/danthegoodman1/IAMTheService/gologger"
	"github.com/danthegoodman1/IAMTheService/utils"
)

var logger = gologger.NewLogger()

type HTTPServer struct {
	Echo       *echo.Echo
	quicServer *http3.Server
}

type CustomValidator struct {
	validator *validator.Validate
}

func StartHTTPServer(port int) *HTTPServer {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.Error().Err(err).Msg("error creating tcp listener, exiting")
		os.Exit(1)
	}

	s := &HTTPServer{
		Echo: echo.New(),
	}
	s.Echo.HideBanner = true
	s.Echo.HidePort = true
	s.Echo.JSONSerializer = &utils.NoEscapeJSONSerializer{}
	s.Echo.Use(CreateReqContext)
	s.Echo.Use(LoggerMiddleware)
	s.Echo.Use(middleware.CORS())
	s.Echo.Validator = &CustomValidator{validator: validator.New()}
	s.Echo.HTTPErrorHandler = customHTTPErrorHandler

	internalRoutes := s.Echo.Group("/.internal")
	internalRoutes.GET("/hc", s.HealthCheck)

	// dummy route to test request verification
	s.Echo.Any("**", ccHandler(func(c *CustomContext) error {
		return c.JSON(http.StatusOK, c.AWSCredentials)
	}), verifyAWSRequestMiddleware)

	s.Echo.Listener = listener
	go func() {
		logger.Info().Msg("starting h2c server on " + listener.Addr().String())
		// this just basically creates a h2c.NewHandler(echo, &http2.Server{})
		err := s.Echo.StartH2CServer("", &http2.Server{})
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Msg("failed to start h2c server, exiting")
			os.Exit(1)
		}
	}()

	// Start http/3 server
	go func() {
		tlsCert, err := loadOrGenerateTLSCert()
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to generate self-signed cert")
		}

		// TLS configuration
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			NextProtos:   []string{"h3"},
		}

		// Create HTTP/3 server
		s.quicServer = &http3.Server{
			Addr:      listener.Addr().String(),
			Handler:   s.Echo,
			TLSConfig: tlsConfig,
		}

		logger.Info().Msg("starting h3 server on " + listener.Addr().String())
		err = s.quicServer.ListenAndServe()

		// Start the server
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Msg("failed to start h2c server, exiting")
			os.Exit(1)
		}
	}()

	return s
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

func ValidateRequest(c echo.Context, s interface{}) error {
	if err := c.Bind(s); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	// needed because POST doesn't have query param binding (https://echo.labstack.com/docs/binding#multiple-sources)
	if err := (&echo.DefaultBinder{}).BindQueryParams(c, s); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(s); err != nil {
		return err
	}
	return nil
}

func (*HTTPServer) HealthCheck(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	err := s.quicServer.Close()
	if err != nil {
		return fmt.Errorf("error in quicServer.Close: %w", err)
	}

	err = s.Echo.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("error shutting down echo: %w", err)
	}

	return nil
}

func LoggerMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		if err := next(c); err != nil {
			// default handler
			c.Error(err)
		}
		stop := time.Since(start)
		// Log otherwise
		logger := zerolog.Ctx(c.Request().Context())
		req := c.Request()
		res := c.Response()

		p := req.URL.Path
		if p == "" {
			p = "/"
		}

		cl := req.Header.Get(echo.HeaderContentLength)
		if cl == "" {
			cl = "0"
		}
		logger.Debug().Str("method", req.Method).Str("remote_ip", c.RealIP()).Str("req_uri", req.RequestURI).Str("handler_path", c.Path()).Str("path", p).Int("status", res.Status).Int64("latency_ns", int64(stop)).Str("protocol", req.Proto).Str("bytes_in", cl).Int64("bytes_out", res.Size).Msg("req recived")
		return nil
	}
}

// loadOrGenerateTLSCert will look for utils.TLSCert and utils.TLSKey on disk and load them.
// If both don't exist, it will generate a new pair and return those.
func loadOrGenerateTLSCert() (tls.Certificate, error) {
	// Check if certificate and key files exist
	if fileExists(utils.TLSCert) && fileExists(utils.TLSKey) {
		// Load existing certificate and key
		return tls.LoadX509KeyPair(utils.TLSCert, utils.TLSKey)
	}

	// Generate a new certificate and key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Example Co"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 180), // Valid for 180 days
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Save the certificate
	certOut, err := os.Create(utils.TLSCert)
	if err != nil {
		return tls.Certificate{}, err
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()

	// Save the key
	keyOut, err := os.Create(utils.TLSKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	keyOut.Close()

	// Load the newly created certificate and key
	return tls.LoadX509KeyPair(utils.TLSCert, utils.TLSKey)
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}
