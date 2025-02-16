package http_server

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

var (
	ErrInvalidSignature = echo.NewHTTPError(403, "invalid signature")

	// TODO replace these
)

func getHMAC(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}

func getSHA256(data []byte) []byte {
	hash := sha256.New()
	hash.Write(data)
	return hash.Sum(nil)
}

func getCanonicalRequest(request *http.Request) string {
	s := ""
	s += request.Method + "\n"
	s += request.URL.EscapedPath() + "\n"
	s += request.URL.Query().Encode() + "\n"

	signedHeadersList, _ := lo.Find(strings.Split(request.Header.Get("Authorization"), ", "), func(item string) bool {
		return strings.HasPrefix(item, "SignedHeaders")
	})

	signedHeaders := strings.Split(strings.ReplaceAll(strings.ReplaceAll(signedHeadersList, "SignedHeaders=", ""), ",", ""), ";")
	sort.Strings(signedHeaders) // must be sorted alphabetically
	for _, header := range signedHeaders {
		if header == "host" {
			// For some reason the host header was blank (thanks echo?)
			s += strings.ToLower(header) + ":" + strings.TrimSpace(request.Host) + "\n"
			continue
		}
		s += strings.ToLower(header) + ":" + strings.TrimSpace(request.Header.Get(header)) + "\n"
	}

	s += "\n" // examples have this JESUS WHY DOCS FFS

	s += strings.Join(signedHeaders, ";") + "\n"

	shaHeader := request.Header.Get("x-amz-content-sha256")
	s += lo.Ternary(shaHeader == "", "UNSIGNED-PAYLOAD", shaHeader)

	return s
}

func getStringToSign(request *http.Request, canonicalRequest, region, service string) string {
	s := "AWS4-HMAC-SHA256" + "\n"
	s += request.Header.Get("X-Amz-Date") + "\n"

	scope := request.Header.Get("X-Amz-Date")[:8] + "/" + region + "/" + service + "/aws4_request"
	s += scope + "\n"
	s += fmt.Sprintf("%x", getSHA256([]byte(canonicalRequest)))

	return s
}

func getSigningKey(request *http.Request, password, region, service string) []byte {
	dateKey := getHMAC([]byte("AWS4"+password), []byte(request.Header.Get("X-Amz-Date")[:8]))
	dateRegionKey := getHMAC(dateKey, []byte(region))
	dateRegionServiceKey := getHMAC(dateRegionKey, []byte(service))
	signingKey := getHMAC(dateRegionServiceKey, []byte("aws4_request"))
	return signingKey
}

type (
	AWSAuthHeader struct {
		Credential    AWSAuthHeaderCredential
		SignedHeaders []string
		Signature     string
	}

	AWSAuthHeaderCredential struct {
		KeyID   string
		Date    string
		Region  string
		Service string
		// always "aws4_request"
		Request string
	}
)

// headers look like
//
//	Authorization: AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,  SignedHeaders=host;range;x-amz-date, Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024
func parseAuthHeader(header string) AWSAuthHeader {
	var authHeader AWSAuthHeader
	parts := strings.Split(header, " ")
	for _, part := range parts {
		// Remove the trailing `,`
		if part[len(part)-1] == ',' {
			part = part[:len(part)-1]
		}
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) != 2 {
			continue
		}

		key, value := keyValue[0], keyValue[1]
		switch key {
		case "Credential":
			credentialParts := strings.Split(value, "/")
			authHeader.Credential = AWSAuthHeaderCredential{
				KeyID:   credentialParts[0],
				Date:    credentialParts[1],
				Region:  credentialParts[2],
				Service: credentialParts[3],
				Request: credentialParts[4],
			}
		case "SignedHeaders":
			authHeader.SignedHeaders = strings.Split(value, ";")
		case "Signature":
			authHeader.Signature = value
		default:
			continue
		}
	}
	return authHeader
}

func verifyAWSRequestMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		logger := zerolog.Ctx(c.Request().Context())
		logger.Debug().Msg("verifying aws request")
		parsedHeader := parseAuthHeader(c.Request().Header.Get("Authorization"))

		signature := generateSigV4(c.Request(), parsedHeader, "")
		if signature != parsedHeader.Signature {
			return ErrInvalidSignature
		}

		cc, _ := c.(*CustomContext)
		cc.AWSCredentials = parsedHeader.Credential

		return next(c)
	}
}

func generateSigV4(r *http.Request, parsedHeader AWSAuthHeader, keySecret string) string {
	logger.Debug().Msg("verifying aws request")
	canonicalRequest := getCanonicalRequest(r)
	stringToSign := getStringToSign(r, canonicalRequest, parsedHeader.Credential.Region, parsedHeader.Credential.Service)

	signingKey := getSigningKey(r, keySecret, parsedHeader.Credential.Region, parsedHeader.Credential.Service)
	signature := fmt.Sprintf("%x", getHMAC(signingKey, []byte(stringToSign)))

	return signature
}
