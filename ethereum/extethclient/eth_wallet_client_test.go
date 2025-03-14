package extethclient

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/athanorlabs/atomic-swap/common"
)

func Test_validateChainID_devSuccess(t *testing.T) {
	err := validateChainID(common.Development, big.NewInt(common.GanacheChainID))
	require.NoError(t, err)
}

func Test_validateChainID_mismatchedEnv(t *testing.T) {
	err := validateChainID(common.Mainnet, big.NewInt(common.GanacheChainID))
	require.Error(t, err)
	assert.ErrorContains(t, err, "expected Mainnet chain ID (1), but found 1337")

	err = validateChainID(common.Stagenet, big.NewInt(common.GanacheChainID))
	require.Error(t, err)
	assert.ErrorContains(t, err, "expected Goerli chain ID (5), but found 1337")
}
