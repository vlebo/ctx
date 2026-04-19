// SPDX-FileCopyrightText: 2026 Enrique Alonso <enrialonso@gmail.com>
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/vlebo/ctx/internal/config"
)

func TestCheckSSMDependencies_MissingAWS(t *testing.T) {
	// Put an empty PATH so neither aws nor session-manager-plugin is found
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir)

	err := checkSSMDependencies()
	if err == nil {
		t.Fatal("expected error when aws CLI is missing, got nil")
	}
}

func TestCheckSSMDependencies_MissingPlugin(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake aws binary
	awsBin := filepath.Join(tmpDir, "aws")
	if err := os.WriteFile(awsBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", tmpDir)

	err := checkSSMDependencies()
	if err == nil {
		t.Fatal("expected error when session-manager-plugin is missing, got nil")
	}
}

func TestCheckSSMDependencies_BothPresent(t *testing.T) {
	tmpDir := t.TempDir()

	for _, bin := range []string{"aws", "session-manager-plugin"} {
		p := filepath.Join(tmpDir, bin)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("PATH", tmpDir)

	if err := checkSSMDependencies(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestBuildSSMArgs(t *testing.T) {
	tunnel := config.SSMTunnelConfig{
		Name:       "postgres",
		SSMTarget:  "i-0abc123def456789a",
		RemoteHost: "db.internal.vpc",
		RemotePort: 5432,
		LocalPort:  5432,
	}

	args := buildSSMArgs(tunnel)

	expected := []string{
		"ssm", "start-session",
		"--target", "i-0abc123def456789a",
		"--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
		"--parameters", `{"host":["db.internal.vpc"],"portNumber":["5432"],"localPortNumber":["5432"]}`,
	}

	if len(args) != len(expected) {
		t.Fatalf("got %d args, want %d: %v", len(args), len(expected), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("arg[%d]: got %q, want %q", i, a, expected[i])
		}
	}
}

func TestBuildSSMArgs_PortSubstitution(t *testing.T) {
	tunnel := config.SSMTunnelConfig{
		SSMTarget:  "i-0abc123def456789a",
		RemoteHost: "cache.internal",
		RemotePort: 6379,
		LocalPort:  16379, // remapped port
	}

	args := buildSSMArgs(tunnel)

	// Find --parameters arg
	var params string
	for i, a := range args {
		if a == "--parameters" && i+1 < len(args) {
			params = args[i+1]
			break
		}
	}

	want := `{"host":["cache.internal"],"portNumber":["6379"],"localPortNumber":["16379"]}`
	if params != want {
		t.Errorf("params: got %q, want %q", params, want)
	}
}

func TestValidateSSMTarget_Found(t *testing.T) {
	tmpDir := t.TempDir()

	// Fake aws binary that returns a non-empty InstanceInformationList
	response := map[string]any{
		"InstanceInformationList": []map[string]string{
			{"InstanceId": "i-0abc123def456789a"},
		},
	}
	out, _ := json.Marshal(response)
	script := fmt.Sprintf("#!/bin/sh\necho '%s'\n", string(out))
	if err := os.WriteFile(filepath.Join(tmpDir, "aws"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", tmpDir)
	awsEnv := append(os.Environ(), "PATH="+tmpDir)

	if err := validateSSMTarget("i-0abc123def456789a", awsEnv); err != nil {
		t.Fatalf("expected no error for registered instance, got: %v", err)
	}
}

func TestValidateSSMTarget_NotRegistered(t *testing.T) {
	tmpDir := t.TempDir()

	// Fake aws binary that returns an empty list
	response := map[string]any{
		"InstanceInformationList": []any{},
	}
	out, _ := json.Marshal(response)
	script := fmt.Sprintf("#!/bin/sh\necho '%s'\n", string(out))
	if err := os.WriteFile(filepath.Join(tmpDir, "aws"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", tmpDir)
	awsEnv := append(os.Environ(), "PATH="+tmpDir)

	err := validateSSMTarget("i-0deadbeef00000000", awsEnv)
	if err == nil {
		t.Fatal("expected error for unregistered instance, got nil")
	}
}

func TestBuildAWSEnv_WithVaultCreds(t *testing.T) {
	awsCfg := &config.AWSConfig{
		Region:  "eu-west-1",
		Profile: "my-profile",
	}
	creds := &config.AWSCredentials{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		SessionToken:    "token123",
	}

	env := buildAWSEnv(awsCfg, creds)

	check := func(key, want string) {
		t.Helper()
		for _, e := range env {
			if len(e) > len(key)+1 && e[:len(key)+1] == key+"=" {
				got := e[len(key)+1:]
				if got != want {
					t.Errorf("%s: got %q, want %q", key, got, want)
				}
				return
			}
		}
		t.Errorf("missing env var %s", key)
	}

	check("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	check("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	check("AWS_SESSION_TOKEN", "token123")
	check("AWS_REGION", "eu-west-1")
	check("AWS_DEFAULT_REGION", "eu-west-1")
}

func TestBuildAWSEnv_WithProfile(t *testing.T) {
	awsCfg := &config.AWSConfig{
		Region:  "us-east-1",
		Profile: "sso-prod",
	}

	env := buildAWSEnv(awsCfg, nil)

	hasProfile := false
	hasNoKey := true
	for _, e := range env {
		if e == "AWS_PROFILE=sso-prod" {
			hasProfile = true
		}
		if len(e) > 17 && e[:17] == "AWS_ACCESS_KEY_ID" {
			hasNoKey = false
		}
	}
	if !hasProfile {
		t.Error("AWS_PROFILE not set when no vault creds")
	}
	if !hasNoKey {
		t.Error("AWS_ACCESS_KEY_ID should not be set when using profile")
	}
}

func TestBuildAWSEnv_CustomConfigFile(t *testing.T) {
	awsCfg := &config.AWSConfig{
		Profile: "myproject",
		Region:  "eu-west-1",
		Config:  "/home/user/.aws/config_myproject",
	}

	env := buildAWSEnv(awsCfg, nil)

	hasConfigFile := false
	for _, e := range env {
		if e == "AWS_CONFIG_FILE=/home/user/.aws/config_myproject" {
			hasConfigFile = true
		}
	}
	if !hasConfigFile {
		t.Error("AWS_CONFIG_FILE not set when aws.config is configured")
	}
}

// writeTunnelStateFile writes a tunnelState JSON file under stateDir/tunnels/<context>.json.
func writeTunnelStateFile(t *testing.T, stateDir, contextName string, state *tunnelState) string {
	t.Helper()
	tunnelDir := filepath.Join(stateDir, "tunnels")
	if err := os.MkdirAll(tunnelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(tunnelDir, contextName+".json")
	if err := saveTunnelState(stateFile, state); err != nil {
		t.Fatal(err)
	}
	return stateFile
}

// startSleepProcess starts a long-running sleep subprocess in its own process group
// (Setsid: true), mirroring how SSM tunnels are launched. This is required so that
// killSSMProcessGroup (which uses Kill(-pid, SIGTERM)) actually reaches the process.
// Returns the PID and a reap function to call after killing to avoid zombies.
func startSleepProcess(t *testing.T) (pid int, reap func()) {
	t.Helper()
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return cmd.Process.Pid, func() { _ = cmd.Wait() }
}

func TestStopContextTunnels_StopsSSMTunnels(t *testing.T) {
	stateDir := t.TempDir()
	pid, reap := startSleepProcess(t)

	state := &tunnelState{
		TunnelPIDs: make(map[string]tunnelEntry),
		SSMTunnelPIDs: map[string]ssmTunnelEntry{
			"postgres": {
				PID:       pid,
				StartedAt: time.Now(),
				Config: config.SSMTunnelConfig{
					Name:       "postgres",
					SSMTarget:  "i-0abc123def456789a",
					RemoteHost: "db.internal.vpc",
					RemotePort: 5432,
					LocalPort:  5432,
				},
			},
		},
	}
	stateFile := writeTunnelStateFile(t, stateDir, "test-ctx", state)

	stopped, err := stopContextTunnels(stateDir, "test-ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stopped != 1 {
		t.Errorf("expected 1 tunnel stopped, got %d", stopped)
	}

	// Reap the zombie so kill -0 no longer sees it
	reap()
	if isProcessRunning(pid) {
		t.Error("SSM tunnel process should have been stopped")
	}
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Error("state file should have been removed")
	}
}

func TestStopContextTunnels_StopsBothSSHAndSSM(t *testing.T) {
	stateDir := t.TempDir()
	sshPID, reapSSH := startSleepProcess(t)
	ssmPID, reapSSM := startSleepProcess(t)

	state := &tunnelState{
		TunnelPIDs: map[string]tunnelEntry{
			"legacy": {PID: sshPID, StartedAt: time.Now()},
		},
		SSMTunnelPIDs: map[string]ssmTunnelEntry{
			"postgres": {
				PID:       ssmPID,
				StartedAt: time.Now(),
				Config:    config.SSMTunnelConfig{Name: "postgres"},
			},
		},
	}
	stateFile := writeTunnelStateFile(t, stateDir, "test-ctx", state)

	stopped, err := stopContextTunnels(stateDir, "test-ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stopped != 2 {
		t.Errorf("expected 2 tunnels stopped, got %d", stopped)
	}

	reapSSH()
	reapSSM()
	if isProcessRunning(sshPID) {
		t.Error("SSH tunnel process should have been stopped")
	}
	if isProcessRunning(ssmPID) {
		t.Error("SSM tunnel process should have been stopped")
	}
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Error("state file should have been removed")
	}
}

func TestStopContextTunnels_NoStateFile(t *testing.T) {
	stateDir := t.TempDir()

	stopped, err := stopContextTunnels(stateDir, "nonexistent-ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stopped != 0 {
		t.Errorf("expected 0 tunnels stopped, got %d", stopped)
	}
}
