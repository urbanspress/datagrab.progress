package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const CodeGrabE402 = "GRAB-E402"

// ExactOrigin normalizes a URL to scheme, host and effective port.
func ExactOrigin(u *url.URL) (string, error) {
	if u == nil || u.User != nil || u.Opaque != "" || u.Hostname() == "" ||
		strings.ContainsAny(u.Host, "%\\") || strings.HasSuffix(u.Host, ":") {
		return "", NewError(CodeGrabE402, 403, "invalid origin", nil)
	}
	port := u.Port()
	switch u.Scheme {
	case "https":
		if port == "" {
			port = "443"
		}
	case "http":
		if port == "" {
			port = "80"
		}
	default:
		return "", NewError(CodeGrabE402, 403, "invalid origin scheme", nil)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
		return "", NewError(CodeGrabE402, 403, "invalid origin port", err)
	}
	return u.Scheme + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port), nil
}

type ServiceOrigin struct {
	base *url.URL
	key  string
}

func ParseServiceOrigin(raw string, allowLocalWorker bool) (ServiceOrigin, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return ServiceOrigin{}, NewError(CodeGrabE402, 403, "invalid configured service origin", nil)
	}
	key, err := ExactOrigin(u)
	if err != nil {
		return ServiceOrigin{}, err
	}
	if (u.Path != "" && u.Path != "/") || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
		(u.Scheme != "https" && !(allowLocalWorker && u.Hostname() == "127.0.0.1")) {
		return ServiceOrigin{}, NewError(CodeGrabE402, 403, "configure an exact HTTPS origin or an explicit loopback worker", nil)
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !serviceAddressAllowed(ip, u.Scheme == "http") {
		return ServiceOrigin{}, NewError(CodeGrabE402, 403, "service address is outside the allowed network", nil)
	}
	u, _ = url.Parse(key)
	return ServiceOrigin{base: u, key: key}, nil
}

func (o ServiceOrigin) Identity() string { return o.key }

func (o ServiceOrigin) URL(path string) string {
	u := *o.base
	u.Path = path
	return u.String()
}

// Transport connects to the validated DNS result directly. The original host
// remains in the request for TLS certificate verification and SNI.
func (o ServiceOrigin) Transport() *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Transport{
		TLSHandshakeTimeout:    5 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr != o.base.Host {
				return nil, NewError(CodeGrabE402, 403, "outbound service origin mismatch", nil)
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", o.base.Hostname())
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("service DNS resolution failed")
			}
			for _, ip := range ips {
				if !serviceAddressAllowed(ip, o.base.Scheme == "http") {
					return nil, NewError(CodeGrabE402, 403, "service DNS resolved to a disallowed address", nil)
				}
			}
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), o.base.Port()))
				if err == nil {
					return conn, nil
				}
			}
			return nil, fmt.Errorf("configured service is unreachable")
		},
	}
}

func serviceAddressAllowed(ip netip.Addr, local bool) bool {
	ip = ip.Unmap()
	if local {
		return ip == netip.AddrFrom4([4]byte{127, 0, 0, 1})
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	// Limit remote IPv6 to global unicast and exclude IPv4 translation ranges.
	if ip.Is6() {
		return netip.MustParsePrefix("2000::/3").Contains(ip) &&
			!netip.MustParsePrefix("2002::/16").Contains(ip) &&
			!netip.MustParsePrefix("2001::/23").Contains(ip)
	}
	for _, p := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4"} {
		if netip.MustParsePrefix(p).Contains(ip) {
			return false
		}
	}
	return true
}
