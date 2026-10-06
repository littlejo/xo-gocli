package vm

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

// readyVMResponse is a VM that satisfies the default gate: Running with a
// main IP address.
const readyVMResponse = `{
	"id": "550e8400-e29b-41d4-a716-446655440001",
	"uuid": "550e8400-e29b-41d4-a716-446655440001",
	"type": "vm",
	"name_label": "web-03",
	"power_state": "Running",
	"memory": {"size": 1073741824},
	"CPUs": {"number": 1},
	"mainIpAddress": "10.0.0.50"
}`

const waitVMID = "550e8400-e29b-41d4-a716-446655440001"

// closedPort returns a port that is not listening (listen, remember, close).
func closedPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot reserve a port: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("cannot split port: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("cannot close listener: %v", err)
	}
	return port
}

func TestVMWaitReady(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	server.vmResponse = readyVMResponse

	out, err := runVM(t, "vm", "wait", waitVMID)
	if err != nil {
		t.Fatalf("vm wait: %v\n%s", err, out)
	}
	if !strings.Contains(out, `VM "web-03" is ready`) {
		t.Fatalf("expected the ready marker:\n%s", out)
	}
	if !strings.Contains(out, "10.0.0.50") {
		t.Fatalf("expected the main IP:\n%s", out)
	}
	// Without --ssh the ssh gate is off: no connect hint.
	if strings.Contains(out, "Connect with") {
		t.Fatalf("no ssh gate requested, so no connect hint:\n%s", out)
	}
}

// TestVMWaitSSH gates on a local listener standing in for the guest's sshd:
// the command checks TCP reachability only, so any accepting port works.
func TestVMWaitSSH(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot start the fake sshd listener: %v", err)
	}
	defer ln.Close()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("cannot split port: %v", err)
	}

	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	server.vmResponse = fmt.Sprintf(`{
		"id": %q,
		"type": "vm",
		"name_label": "web-03",
		"power_state": "Running",
		"memory": {"size": 1073741824},
		"CPUs": {"number": 1},
		"mainIpAddress": "127.0.0.1"
	}`, waitVMID)

	out, err := runVM(t, "vm", "wait", waitVMID, "--ssh", "--port", port, "--output", "json")
	if err != nil {
		t.Fatalf("vm wait --ssh: %v\n%s", err, out)
	}
	for _, want := range []string{`"ip": "127.0.0.1"`, `"ssh": true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %s in the readiness document:\n%s", want, out)
		}
	}
}

// TestVMWaitSSHNotYetReachable: a running VM whose port is not (yet) open must
// hit the --timeout deadline, not return ready.
func TestVMWaitSSHNotYetReachable(t *testing.T) {
	port := closedPort(t)

	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	server.vmResponse = fmt.Sprintf(`{
		"id": %q,
		"type": "vm",
		"name_label": "web-03",
		"power_state": "Running",
		"memory": {"size": 1073741824},
		"CPUs": {"number": 1},
		"mainIpAddress": "127.0.0.1"
	}`, waitVMID)

	if _, err := runVM(t, "vm", "wait", waitVMID, "--ssh", "--port", port, "--timeout", "1s"); err == nil {
		t.Fatal("expected the wait to time out on an unreachable port")
	} else if !strings.Contains(err.Error(), "not ready within") {
		t.Fatalf("expected a deadline error, got: %v", err)
	}
}

// TestVMWaitStillStarting: the raw power_state lags at Halted while the start
// operation is in flight, so the derived state is "Starting" and the gate must
// not pass.
func TestVMWaitStillStarting(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	server.vmResponse = fmt.Sprintf(`{
		"id": %q,
		"type": "vm",
		"name_label": "web-03",
		"power_state": "Halted",
		"memory": {"size": 1073741824},
		"CPUs": {"number": 1},
		"current_operations": {"00000000-0000-0000-0000-0000000000aa": "start"}
	}`, waitVMID)

	out, err := runVM(t, "vm", "wait", waitVMID, "--timeout", "1s")
	if err == nil {
		t.Fatalf("expected the wait to time out while the VM is starting:\n%s", out)
	}
	if !strings.Contains(err.Error(), "state is Starting") {
		t.Fatalf("expected the blocker to report the starting state, got: %v", err)
	}
}

// TestVMWaitNoIP: a running VM that has not been given an address yet must not
// pass the gate.
func TestVMWaitNoIP(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	server.vmResponse = fmt.Sprintf(`{
		"id": %q,
		"type": "vm",
		"name_label": "web-03",
		"power_state": "Running",
		"memory": {"size": 1073741824},
		"CPUs": {"number": 1}
	}`, waitVMID)

	out, err := runVM(t, "vm", "wait", waitVMID, "--timeout", "1s")
	if err == nil {
		t.Fatalf("expected the wait to time out without an IP:\n%s", out)
	}
	if !strings.Contains(err.Error(), "no main IP address yet") {
		t.Fatalf("expected the missing-IP blocker, got: %v", err)
	}
}

func TestVMWaitNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	server.vm404 = true

	if _, err := runVM(t, "vm", "wait", waitVMID); err == nil {
		t.Fatal("expected a not-found error for a missing VM")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected a not-found error, got: %v", err)
	}
}

func TestVMWaitQuery(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	server.vmResponse = readyVMResponse

	out, err := runVM(t, "vm", "wait", waitVMID, "--output", "json", "--query", "ip")
	if err != nil {
		t.Fatalf("vm wait --query: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"10.0.0.50"`) {
		t.Fatalf("expected the projected IP:\n%s", out)
	}
}
