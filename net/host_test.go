package net

import (
	"context"
	"path"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	logging "github.com/ipfs/go-log"
	"github.com/stretchr/testify/require"

	"github.com/athanorlabs/atomic-swap/common/types"
	"github.com/athanorlabs/atomic-swap/net/message"
)

func init() {
	logging.SetLogLevel("net", "debug")
	logging.SetLogLevel("p2pnet", "debug")
}

var (
	testID        = types.Hash{99}
	mockEthTXHash = ethcommon.Hash{33}
)

type mockMakerHandler struct {
	t  *testing.T
	id types.Hash
}

func (h *mockMakerHandler) GetOffers() []*types.Offer {
	return []*types.Offer{}
}

func (h *mockMakerHandler) HandleInitiateMessage(msg *message.SendKeysMessage) (s SwapState, resp Message, err error) {
	if (h.id != types.Hash{}) {
		return &mockSwapState{h.id}, createSendKeysMessage(h.t), nil
	}
	return &mockSwapState{}, msg, nil
}

type mockTakerHandler struct {
	t *testing.T
}

func (h *mockTakerHandler) HandleRelayClaimRequest(_ *RelayClaimRequest) (*RelayClaimResponse, error) {
	return &RelayClaimResponse{
		TxHash: mockEthTXHash,
	}, nil
}

type mockSwapState struct {
	id types.Hash
}

func (s *mockSwapState) ID() types.Hash {
	if (s.id != types.Hash{}) {
		return s.id
	}

	return testID
}

func (s *mockSwapState) HandleProtocolMessage(_ Message) error {
	return nil
}

func (s *mockSwapState) Exit() error {
	return nil
}

func basicTestConfig(t *testing.T) *Config {
	// t.TempDir() is unique on every call. Don't reuse this config with multiple hosts.
	tmpDir := t.TempDir()
	return &Config{
		Ctx:        context.Background(),
		DataDir:    tmpDir,
		Port:       0, // OS randomized libp2p port
		KeyFile:    path.Join(tmpDir, "node.key"),
		Bootnodes:  nil,
		ProtocolID: "/testid",
		ListenIP:   "127.0.0.1",
		IsRelayer:  false,
	}
}

func newHost(t *testing.T, cfg *Config) *Host {
	h, err := NewHost(cfg)
	require.NoError(t, err)
	h.SetHandlers(&mockMakerHandler{t: t}, &mockTakerHandler{t: t})
	t.Cleanup(func() {
		err = h.Stop()
		require.NoError(t, err)
	})
	return h
}
