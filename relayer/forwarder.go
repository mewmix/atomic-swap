package relayer

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	rcommon "github.com/athanorlabs/go-relayer/common"
	"github.com/athanorlabs/go-relayer/impls/gsnforwarder"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	contracts "github.com/athanorlabs/atomic-swap/ethereum"
)

func createForwarderSignature(
	ctx context.Context,
	claimerEthKey *ecdsa.PrivateKey,
	ec *ethclient.Client,
	swapFactoryAddress ethcommon.Address,
	forwarderAddress ethcommon.Address,
	swap *contracts.SwapFactorySwap,
	secret *[32]byte,
) ([]byte, error) {

	if swap.Claimer != ethcrypto.PubkeyToAddress(claimerEthKey.PublicKey) {
		return nil, fmt.Errorf("signing key does not match claimer %s", swap.Claimer)
	}

	forwarder, domainSeparator, err := getForwarderAndDomainSeparator(ctx, ec, forwarderAddress)
	if err != nil {
		return nil, err
	}

	nonce, err := forwarder.GetNonce(&bind.CallOpts{Context: ctx}, swap.Claimer)
	if err != nil {
		return nil, err
	}

	forwarderReq, err := createForwarderRequest(
		nonce,
		swapFactoryAddress,
		swap,
		secret,
	)
	if err != nil {
		return nil, err
	}

	digest, err := rcommon.GetForwardRequestDigestToSign(forwarderReq, *domainSeparator, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get forward request digest: %w", err)
	}

	signature, err := rcommon.NewKeyFromPrivateKey(claimerEthKey).Sign(digest)
	if err != nil {
		return nil, fmt.Errorf("failed to sign forward request digest: %w", err)
	}

	return signature, nil
}

// createForwarderRequest creates the forwarder request, which we sign the digest of.
func createForwarderRequest(
	nonce *big.Int,
	swapFactoryAddress ethcommon.Address,
	swap *contracts.SwapFactorySwap,
	secret *[32]byte,
) (*gsnforwarder.IForwarderForwardRequest, error) {

	calldata, err := getClaimRelayerTxCalldata(FeeWei, swap, secret)
	if err != nil {
		return nil, err
	}

	req := &gsnforwarder.IForwarderForwardRequest{
		From:           swap.Claimer,
		To:             swapFactoryAddress,
		Value:          big.NewInt(0),
		Gas:            big.NewInt(relayedClaimGas),
		Nonce:          nonce,
		Data:           calldata,
		ValidUntilTime: big.NewInt(0),
	}

	return req, nil
}

// getClaimRelayerTxCalldata returns the call data to be used when invoking the
// claimRelayer method on the SwapFactory contract.
func getClaimRelayerTxCalldata(feeWei *big.Int, swap *contracts.SwapFactorySwap, secret *[32]byte) ([]byte, error) {
	return contracts.SwapFactoryParsedABI.Pack("claimRelayer", *swap, *secret, feeWei)
}

func getForwarderAndDomainSeparator(
	ctx context.Context,
	ec *ethclient.Client,
	forwarderAddress ethcommon.Address,
) (*gsnforwarder.Forwarder, *[32]byte, error) {
	chainID, err := ec.ChainID(ctx)
	if err != nil {
		return nil, nil, err
	}

	forwarder, err := gsnforwarder.NewForwarder(forwarderAddress, ec)
	if err != nil {
		return nil, nil, err
	}

	domainSeparator, err := rcommon.GetEIP712DomainSeparator(gsnforwarder.DefaultName,
		gsnforwarder.DefaultVersion, chainID, forwarderAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get EIP712 domain separator: %w", err)
	}

	return forwarder, &domainSeparator, nil
}
