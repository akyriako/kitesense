package kube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/klog/v2"
)

// PortForwardSession manages a port-forward connection to a pod.
// It encapsulates the complexity of SPDY negotiation, port allocation,
// and connection lifecycle — similar to how TerminalSession manages exec sessions.
//
// Usage pattern (mirrors TerminalSession):
//
//	session := kube.NewPortForwardSession(cfg, namespace, podName, podPort)
//	if err := session.Start(ctx); err != nil {
//		// handle error
//	}
//	defer session.Close()
//	localAddr := session.LocalAddr() // e.g., "127.0.0.1:54321"
//	// make HTTP request to http://127.0.0.1:54321/your/path
type PortForwardSession struct {
	// Configuration
	restConfig *rest.Config
	namespace  string
	podName    string
	podPort    string

	// Lifecycle channels and state
	stop      chan struct{} // signal to stop the port-forward
	ready     chan struct{} // signaled when port-forward is ready
	pf        *portforward.PortForwarder
	localPort int

	// Output streams for logging (similar to how TerminalSession uses websocket)
	out    io.Writer // stdout-like stream for port-forward logs
	errOut io.Writer // stderr-like stream for port-forward errors
}

// NewPortForwardSession creates a new port-forward session.
//
// Parameters:
//   - cfg: Kubernetes rest.Config (contains API server host, auth, etc.)
//   - namespace: target pod's namespace
//   - podName: target pod's name
//   - podPort: port on the pod to forward to (e.g., "8808")
//   - out: io.Writer to capture port-forward stdout logs (e.g., os.Stdout, &strings.Builder{})
//   - errOut: io.Writer to capture port-forward stderr/errors (e.g., os.Stderr, &strings.Builder{})
//
// Example:
//
//	cfg, _ := rest.InClusterConfig()
//	session := kube.NewPortForwardSession(cfg, "default", "my-pod", "8808", os.Stdout, os.Stderr)
func NewPortForwardSession(cfg *rest.Config, namespace, podName, podPort string, out, errOut io.Writer) *PortForwardSession {
	return &PortForwardSession{
		restConfig: cfg,
		namespace:  namespace,
		podName:    podName,
		podPort:    podPort,
		out:        out,
		errOut:     errOut,
		stop:       make(chan struct{}, 1), // buffered so Close() won't block
		ready:      make(chan struct{}),    // unbuffered; receives once when ready
		localPort:  -1,                     // sentinel: not yet allocated
	}
}

// Start initiates the port-forward connection to the pod.
// It spawns a background goroutine that keeps the SPDY tunnel open.
// The function returns when the port-forward is ready (or timeout/error).
//
// This mirrors TerminalSession.Start(ctx, subResource) but for port-forwarding instead of exec.
//
// Flow:
//  1. Build Kubernetes API URL for port-forward subresource
//  2. Create SPDY transport (SPDY is the protocol for tunneling in Kubernetes)
//  3. Create a port-forward dialer (speaks SPDY)
//  4. Tell port-forward to forward local ephemeral port (0) to pod's podPort
//  5. Start the forwarder in a goroutine (blocks indefinitely until Close())
//  6. Wait for the "ready" signal or timeout
//  7. Extract the actual local port that was allocated
//  8. Return (connection now ready for HTTP requests to localhost:localPort)
//
// Error cases:
//   - context cancelled before ready → returns context error
//   - SPDY setup fails → returns SPDY error
//   - Port-forward setup fails → returns port-forward error
//   - Timeout (10 seconds) → returns timeout error
func (s *PortForwardSession) Start(ctx context.Context) error {
	// --- Step 1: Build the Kubernetes API URL ---
	// We need to call the Kubernetes API server's port-forward endpoint:
	// POST /api/v1/namespaces/{ns}/pods/{podName}/portforward
	// The API server will then proxy our SPDY connection to the pod's kubelet.

	// Extract the API server host from rest.Config, removing the "https://" scheme
	// because url.URL will add it back in the Scheme field.
	host := strings.TrimPrefix(s.restConfig.Host, "https://")

	// Build the full request URL
	reqURL := &url.URL{
		Scheme: "https",
		Host:   host,
		Path:   path.Join("/api/v1/namespaces", s.namespace, "pods", s.podName, "portforward"),
	}

	klog.V(2).Infof("Starting port-forward to %s/%s:%s via %s", s.namespace, s.podName, s.podPort, reqURL.String())

	// --- Step 2: Create SPDY transport and upgrader ---
	// SPDY is a multiplexed binary protocol used by Kubernetes for tunneling.
	// It's how kubectl port-forward, kubectl exec, and kubectl logs all work under the hood.
	// RoundTripperFor gives us a transport that speaks SPDY and uses the credentials from rest.Config.

	transport, upgrader, err := spdy.RoundTripperFor(s.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create SPDY transport: %w", err)
	}

	// --- Step 3: Create a SPDY dialer ---
	// The dialer will initiate the SPDY connection to the API server.
	// It needs:
	//   - upgrader: handles SPDY protocol upgrade
	//   - HTTP client with the SPDY transport
	//   - HTTP method (port-forward uses POST)
	//   - target URL (the port-forward endpoint we built above)

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", reqURL)

	// --- Step 4: Specify which ports to forward ---
	// Format: "localPort:podPort"
	// Using "0" for localPort tells the OS: "pick any free port for me"
	// The actual port will be assigned and we extract it later.

	ports := []string{"0:" + s.podPort}

	// --- Step 5: Create the port-forwarder ---
	// This object will manage the SPDY tunnel and forward traffic.
	// Parameters:
	//   - dialer: our configured SPDY dialer
	//   - ports: which ports to forward (localPort:podPort)
	//   - stop: channel we'll close to signal shutdown
	//   - ready: channel the forwarder will close when ready
	//   - out, errOut: streams to write logs/errors to

	pf, err := portforward.New(dialer, ports, s.stop, s.ready, s.out, s.errOut)
	if err != nil {
		return fmt.Errorf("failed to create port-forwarder: %w", err)
	}

	s.pf = pf

	// --- Step 6: Start the forwarder in a background goroutine ---
	// ForwardPorts() is a blocking call that runs until:
	//   - we close the 'stop' channel (in Close()), or
	//   - an error occurs
	// We run it in a goroutine so it doesn't block our caller.

	go func() {
		if err := pf.ForwardPorts(); err != nil {
			// Only log if it's not due to context cancellation or stop signal
			if err != context.Canceled {
				klog.Errorf("Port-forward ended with error: %v", err)
			}
		}
	}()

	// --- Step 7: Wait for the port-forward to be ready or timeout ---
	// The forwarder sends a signal on s.ready when it successfully
	// connected and negotiated with the pod's kubelet.

	select {
	case <-s.ready:
		// Port-forward is ready! Extract the actual local port.
		ports, err := pf.GetPorts()
		if err != nil {
			return fmt.Errorf("port-forward failed to report the ports forwared: %w", err)
		}

		for _, p := range ports {
			s.localPort = int(p.Local)
			klog.V(2).Infof("Port-forward ready: %s/%s:%s -> localhost:%d",
				s.namespace, s.podName, s.podPort, s.localPort)
			return nil
		}
		// Should not reach here if ready was signaled, but safeguard
		return fmt.Errorf("port-forward ready but no ports allocated")

	case <-ctx.Done():
		// Caller's context was cancelled before port-forward was ready
		return fmt.Errorf("port-forward start cancelled: %w", ctx.Err())

	case <-time.After(10 * time.Second):
		// Safety timeout: if port-forward doesn't become ready in 10 seconds, give up
		return fmt.Errorf("port-forward ready timeout (10s)")
	}
}

// LocalAddr returns the local address where the port-forward is listening.
// This address is valid only after Start() has returned successfully.
//
// Format: "127.0.0.1:<localPort>"
//
// Example usage:
//
//	err := session.Start(ctx)
//	if err != nil { ... }
//	url := "http://" + session.LocalAddr() + "/readyz"
//	resp, _ := http.Get(url)
//
// Note: LocalAddr() will return "127.0.0.1:-1" if called before Start() completes.
func (s *PortForwardSession) LocalAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", s.localPort)
}

// Close gracefully terminates the port-forward session.
// It signals the background port-forward goroutine to stop and waits for cleanup.
// Safe to call multiple times (after first call, subsequent calls are no-ops).
//
// This mirrors TerminalSession.Close() — both clean up their underlying
// connection/stream resources.
//
// Important: Always call Close() (preferably via defer) to avoid leaving
// the port-forward goroutine running in the background.
//
// Example:
//
//	session := kube.NewPortForwardSession(...)
//	defer session.Close()  // <-- always do this
//	if err := session.Start(ctx); err != nil { ... }
func (s *PortForwardSession) Close() {
	// Close the 'stop' channel to signal the background goroutine to shut down.
	// Using a buffered channel (created with make(..., 1)) ensures this won't block
	// even if the goroutine isn't listening yet.
	select {
	case s.stop <- struct{}{}:
		klog.V(2).Infof("Sent stop signal to port-forward %s/%s:%s",
			s.namespace, s.podName, s.podPort)
	default:
		// Already sent; this is fine (idempotent)
	}
}
