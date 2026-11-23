package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"groxy-desk/internal/deskexec"
	"groxy-desk/internal/server"

	"github.com/wailsapp/go-webview2/pkg/edge"
)

func TestNativeWebView2Isolation(t *testing.T) {
	if os.Getenv("GROXY_DESK_TEST_WEBVIEW2") != "1" {
		t.Skip("set GROXY_DESK_TEST_WEBVIEW2=1 on an interactive Windows desktop with WebView2")
	}
	const (
		bearer = "desk-native-fixture-bearer"
		csrf   = "desk-native-fixture-csrf"
	)
	if base := os.Getenv("GROXY_DESK_TEST_WORKBENCH"); base != "" {
		root := os.Getenv("GROXY_DESK_TEST_WEBVIEW_ROOT")
		session := server.SessionCredentials{URL: base, BearerToken: bearer, CSRFToken: csrf}
		report := func(event string) {
			res, err := (&http.Client{Timeout: 2 * time.Second}).Get(base + "/signal/" + event)
			if err == nil {
				_ = res.Body.Close()
			}
		}
		controllers := 0
		newController := func() *edge.Chromium {
			controllers++
			pane := edge.NewChromium()
			pane.AdditionalBrowserArgs = []string{
				"--ignore-certificate-errors-spki-list=" + os.Getenv("GROXY_DESK_TEST_WEBVIEW_PIN"),
			}
			if controllers == 2 {
				pane.MessageCallback = func(string, *edge.ICoreWebView2, *edge.ICoreWebView2WebMessageReceivedEventArgs) {
					report("research-message-dispatched")
				}
				pane.NavigationCompletedCallback = func(view *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) {
					source, _ := view.GetSource()
					if !strings.HasPrefix(source, "https://localhost:") {
						return
					}
					settings, err := pane.GetSettings()
					if err != nil {
						report("research-settings-unavailable")
						return
					}
					defer settings.Release()
					messages, msgErr := settings.GetIsWebMessageEnabled()
					objects, objErr := settings.GetAreHostObjectsAllowed()
					if messages || objects || msgErr != nil || objErr != nil {
						report("research-settings-invalid")
						return
					}
					report("research-settings-verified")
				}
			}
			return pane
		}
		if err := launchWebViewWindowWithController(session, filepath.Join(root, "state"), root, false, newController); err != nil {
			t.Fatal(err)
		}
		return
	}

	events := make(chan string, 32)
	var loaded atomic.Bool
	send := func(event string) {
		select {
		case events <- event:
		default:
		}
	}
	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Host, "localhost:") || r.URL.Path == "/invalid-command" {
			send("blocked HTTPS request: " + r.Host + r.URL.Path + " (" + r.Header.Get("Sec-Fetch-Dest") + ")")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-CSRF-Token") != "" {
			send("research received workbench credentials")
		}
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><p id="selection">Research fixture</p>
<script>
const report = name => fetch('/signal/' + name);
try {
  window.chrome?.webview?.addEventListener('message', () => report('research-received-host-message'));
  window.chrome?.webview?.postMessage(JSON.stringify({type:'groxy-desk-ready'}));
} catch {}
const selection = document.createRange();
selection.selectNodeContents(document.getElementById('selection'));
window.getSelection().addRange(selection);
const blocked = location.href.replace('localhost', '127.0.0.1');
window.open(blocked + 'popup', '_blank');
const frame = document.createElement('iframe');
frame.src = blocked + 'research-frame';
document.body.append(frame);
const image = document.createElement('img');
image.addEventListener('error', () => report('research-loaded'), { once: true });
image.src = blocked + 'image';
document.body.append(image);
</script>`)
		case "/redirect":
			http.Redirect(w, r, "https://"+strings.Replace(r.Host, "localhost", "127.0.0.1", 1)+"/redirect-target", http.StatusFound)
			send("same-origin redirect requested")
		case "/signal/research-loaded":
			loaded.Store(true)
			send("research loaded")
		case "/signal/research-received-host-message":
			send(r.URL.Path)
		}
	}))
	defer tls.Close()
	research := strings.Replace(tls.URL, "127.0.0.1", "localhost", 1)
	blocked := tls.URL
	researchURL, blockedURL := jsTestString(research), jsTestString(blocked)
	workbench := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><p>Workbench fixture</p>
<script>
const report = name => fetch('/signal/' + name);
const post = payload => chrome.webview.postMessage(JSON.stringify(payload));
let receivedSession = false;
chrome.webview.addEventListener('message', event => {
  const result = JSON.parse(event.data);
  if (result.type === 'groxy-desk-session' && !receivedSession) {
    receivedSession = true;
    if (result.bearer_token !== %s || result.csrf_token !== %s) return;
    report('workbench-session');
    post({type:'groxy-browser-open', url:%s, script:'window.deskFixtureUnexpected = true'});
    post({type:'groxy-browser-open', url:%s});
  } else if (result.type === 'groxy-browser-status' && result.message.includes('GRAB-E402')) {
    report('origin-rejected');
    post({type:'groxy-browser-open', url:%s});
  } else if (result.type === 'groxy-browser-selection' && result.text === 'Research fixture') {
    report('fixed-capture');
    post({type:'groxy-browser-open', url:%s + '/redirect'});
    const frame = document.createElement('iframe');
    frame.src = '/frame';
    document.body.append(frame);
    location.assign(%s + '/workbench-top');
  }
});
post({type:'groxy-desk-ready'});
const poll = setInterval(async () => {
  if ((await fetch('/capture-ready')).status !== 204) return;
  clearInterval(poll);
  post({type:'groxy-browser-capture'});
}, 250);
</script>`, jsTestString(bearer), jsTestString(csrf), jsTestString(research+"/invalid-command"), blockedURL, researchURL, researchURL, blockedURL)
		case "/capture-ready":
			// The browser fixture's origin and page must be established before
			// the workbench requests the fixed selection operation.
			if !loaded.Load() {
				w.WriteHeader(http.StatusTooEarly)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "/frame":
			send("workbench child frame received a request")
		case "/favicon.ico":
			w.WriteHeader(http.StatusNoContent)
		default:
			send(strings.TrimPrefix(r.URL.Path, "/signal/"))
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer workbench.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp("", "groxy-desk-webview-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var err error
		for range 30 {
			if err = os.RemoveAll(root); err == nil {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Errorf("remove native test profiles: %v", err)
	})
	cmd := deskexec.CommandContext(ctx, exe, "-test.run=^TestNativeWebView2Isolation$", "-test.v")
	pin := sha256.Sum256(tls.Certificate().RawSubjectPublicKeyInfo)
	cmd.Env = append(os.Environ(),
		"GROXY_DESK_TEST_WORKBENCH="+workbench.URL,
		"GROXY_DESK_TEST_WEBVIEW_ROOT="+root,
		"GROXY_DESK_BROWSER_ORIGINS="+research,
		"GROXY_DESK_CMS_ORIGIN=",
		"GROXY_DESK_TEST_WEBVIEW_PIN="+base64.StdEncoding.EncodeToString(pin[:]),
	)
	finished := make(chan string, 1)
	go func() {
		out, err := cmd.CombinedOutput()
		finished <- fmt.Sprintf("%v\n%s", err, out)
	}()
	defer func() {
		cancel()
		if finished != nil {
			result := <-finished
			if t.Failed() {
				t.Log(result)
			}
		}
	}()
	seen := map[string]bool{}
	const requiredEvents = 6
	for len(seen) < requiredEvents {
		select {
		case event := <-events:
			switch event {
			case "workbench-session", "origin-rejected", "research loaded", "fixed-capture", "same-origin redirect requested", "research-settings-verified":
				seen[event] = true
			default:
				t.Fatalf("unexpected native event %q", event)
			}
		case result := <-finished:
			finished = nil
			t.Fatalf("launcher exited after %v: %s", seen, result)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for native WebView2 events: %v", seen)
		}
	}

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-events:
			if event != "research-settings-verified" {
				t.Fatalf("unexpected native event after isolation checks: %q", event)
			}
		case <-timer.C:
			return
		}
	}
}

func jsTestString(text string) string {
	value, _ := json.Marshal(text)
	return string(value)
}
