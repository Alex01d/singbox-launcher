//go:build linux

package core

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"singbox-launcher/core/services"
	daemonpb "singbox-launcher/internal/daemonpb"
)

type classicTailnetTestService struct {
	daemonpb.UnimplementedStartedServiceServer
}

func (classicTailnetTestService) GetTailscaleStatus(context.Context, *daemonpb.TailscaleStatusRequest) (*daemonpb.TailscaleEndpointStatus, error) {
	return &daemonpb.TailscaleEndpointStatus{EndpointTag: "aws", BackendState: "Running", Transport: &daemonpb.TailscaleTransportStatus{ConfiguredMode: "awg3", AppliedMode: "awg3", Applied: true, ProfileID: "lab"}}, nil
}

func TestClassicTailscaleAuthenticatedLoopback(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		if len(md.Get("authorization")) != 1 || md.Get("authorization")[0] != "Bearer private-lab-token" {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization")
		}
		return handler(ctx, req)
	}))
	daemonpb.RegisterStartedServiceServer(server, classicTailnetTestService{})
	go server.Serve(listener)
	defer server.Stop()
	p := filepath.Join(t.TempDir(), "config.json")
	write := func(ip, secret string) {
		data, _ := json.Marshal(map[string]interface{}{"services": []interface{}{map[string]interface{}{"type": "api", "tag": "lx-desktop-api", "listen": ip, "listen_port": listener.Addr().(*net.TCPAddr).Port, "secret": secret}}})
		data = append([]byte("// Generated launcher JSONC\n"), data...)
		data = append(data, []byte("\n//ParserEND\n")...)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	b := &LegacyBackend{ac: &AppController{FileService: &services.FileService{ConfigPath: p}}}
	write("127.0.0.1", "private-lab-token")
	s, ok, err := b.TailscaleStatusNow("aws")
	if err != nil || !ok || s.Transport == nil || s.Transport.AppliedMode != "awg3" {
		t.Fatalf("status/conversion/auth failed: ok=%t err=%v", ok, err)
	}
	write("127.0.0.1", "wrong-token")
	if _, ok, err := b.TailscaleStatusNow("aws"); err == nil || ok {
		t.Fatalf("missing authentication refusal: %v", err)
	}
	for _, bad := range []struct{ ip, secret string }{{"192.0.2.1", "private-lab-token"}, {"127.0.0.1", ""}, {"127.0.0.1", "CHANGE_THIS_TO_YOUR_SECRET_TOKEN"}} {
		write(bad.ip, bad.secret)
		conn, _, err := b.classicTailscaleClient()
		if conn != nil {
			conn.Close()
		}
		if err == nil {
			t.Fatal("unsafe API endpoint was accepted")
		}
	}
	if generated := os.Getenv("LX_AWG143_BUNDLE_CONFIG"); generated != "" {
		b.ac.FileService.ConfigPath = generated
		conn, _, err := b.classicTailscaleClient()
		if err != nil {
			t.Fatalf("real generated JSONC was rejected: %v", err)
		}
		conn.Close()
	}

}
