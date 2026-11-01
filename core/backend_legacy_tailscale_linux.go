//go:build linux

package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/muhammadmuzzammil1998/jsonc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"singbox-launcher/core/services"
	daemonpb "singbox-launcher/internal/daemonpb"
)

// classicTailscaleClient only accepts an authenticated loopback service from
// this launcher's generated config. It never connects to a system tailscaled.
func (b *LegacyBackend) classicTailscaleClient() (*grpc.ClientConn, daemonpb.StartedServiceClient, error) {
	if b.ac == nil || b.ac.FileService == nil {
		return nil, nil, errors.New("Local core API is not configured")
	}
	data, err := os.ReadFile(b.ac.FileService.ConfigPath)
	if err != nil {
		return nil, nil, errors.New("Local core API configuration is unavailable")
	}
	var config struct {
		Services []struct {
			Type   string `json:"type"`
			Tag    string `json:"tag"`
			Listen string `json:"listen"`
			Port   uint16 `json:"listen_port"`
			Secret string `json:"secret"`
		} `json:"services"`
	}
	if json.Unmarshal(jsonc.ToJSON(data), &config) != nil {
		return nil, nil, errors.New("Local core API configuration is invalid")
	}
	for _, s := range config.Services {
		if s.Type != "api" || s.Tag != "lx-desktop-api" {
			continue
		}
		ip := net.ParseIP(s.Listen)
		if ip == nil || !ip.IsLoopback() || s.Port == 0 || s.Secret == "" || s.Secret == "CHANGE_THIS_TO_YOUR_SECRET_TOKEN" {
			return nil, nil, errors.New("Local core API requires a loopback address and a private secret")
		}
		token := s.Secret
		conn, err := grpc.NewClient(net.JoinHostPort(s.Listen, strconv.Itoa(int(s.Port))), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoke grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoke(metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token), method, req, reply, cc, opts...)
		}), grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token), desc, cc, method, opts...)
		}))
		if err != nil {
			return nil, nil, err
		}
		return conn, daemonpb.NewStartedServiceClient(conn), nil
	}
	return nil, nil, errors.New("Enable the local core API to use Tailscale controls")
}

func (b *LegacyBackend) startTailscaleAPI() {
	ctx, cancel := context.WithCancel(context.Background())
	b.tailCancel = cancel
	go func() {
		defer b.tailCache.MarkDead()
		for ctx.Err() == nil {
			if b.ac.RunningState != nil && b.ac.RunningState.IsRunning() {
				conn, client, err := b.classicTailscaleClient()
				b.tailEnabled.Store(err == nil)
				if err == nil {
					func() {
						defer conn.Close()
						defer b.tailCache.MarkDead()
						stream, err := client.SubscribeTailscaleStatus(ctx, &emptypb.Empty{})
						if err != nil {
							return
						}
						for {
							update, err := stream.Recv()
							if err != nil {
								return
							}
							b.tailCache.Apply(update)
						}
					}()
				}
			} else {
				b.tailEnabled.Store(false)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
}

func (b *LegacyBackend) TailscaleAPIEnabled() bool { return b.tailEnabled.Load() }
func (b *LegacyBackend) TailscaleStatus(tag string) (services.TailscaleStatus, bool) {
	return b.tailCache.Get(tag)
}
func (b *LegacyBackend) TailscaleLive() bool { return b.tailCache.Live() }
func (b *LegacyBackend) TailscaleStatusNow(tag string) (services.TailscaleStatus, bool, error) {
	conn, client, err := b.classicTailscaleClient()
	if err != nil {
		return services.TailscaleStatus{}, false, err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return services.TailscaleStatusRPC(ctx, client, tag)
}
func (b *LegacyBackend) TailscaleLogout(tag string) error {
	conn, client, err := b.classicTailscaleClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = client.TailscaleLogout(ctx, &daemonpb.TailscaleLogoutRequest{EndpointTag: tag})
	return err
}
func (b *LegacyBackend) TailscaleSetExitNode(tag, id string) error {
	conn, client, err := b.classicTailscaleClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = client.SetTailscaleExitNode(ctx, &daemonpb.SetTailscaleExitNodeRequest{EndpointTag: tag, StableID: id})
	return err
}
func (b *LegacyBackend) TailscalePing(ctx context.Context, tag, peer string, onReply func(services.TailscalePingResult)) error {
	conn, client, err := b.classicTailscaleClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	stream, err := client.StartTailscalePing(ctx, &daemonpb.TailscalePingRequest{EndpointTag: tag, PeerIP: peer})
	if err != nil {
		return err
	}
	for n := 0; n < services.TailscalePingMaxReplies; n++ {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) || ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		onReply(services.TailscalePingResult{LatencyMs: resp.GetLatencyMs(), IsDirect: resp.GetIsDirect(), Endpoint: resp.GetEndpoint(), DERPRegionCode: resp.GetDerpRegionCode(), Error: resp.GetError()})
	}
	return nil
}
