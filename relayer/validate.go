package relayer

import (
	"context"
	"fmt"

	"github.com/athanorlabs/go-relayer/impls/gsnforwarder"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/athanorlabs/atomic-swap/coins"
	"github.com/athanorlabs/atomic-swap/common/types"
	contracts "github.com/athanorlabs/atomic-swap/ethereum"
	"github.com/athanorlabs/atomic-swap/net/message"
)

func validateClaimRequest(
	ctx context.Context,
	request *message.RelayClaimRequest,
	ec *ethclient.Client,
	ourSFContractAddr ethcommon.Address,
) error {
	err := validateClaimValues(ctx, request, ec, ourSFContractAddr)
	if err != nil {
		return err
	}

	return validateClaimSignature(ctx, ec, request)
}

// validateClaimValues validates the non-signature aspects of the claim request:
//  1. the claim request's swap factory and forwarder contract bytecode matches ours
//  2. the swap is for ETH and not an ERC20 token
//  3. the swap value is strictly greater than the relayer fee
//  4. TODO: Validate that the swap exists and is in a claimable state?
func validateClaimValues(
	ctx context.Context,
	req *message.RelayClaimRequest,
	ec *ethclient.Client,
	ourSwapFactoryAddr ethcommon.Address,
) error {
	// Validate the deployed SwapFactory contract, if it is not at the same address
	// as our own. The CheckSwapFactoryContractCode method validates both the
	// SwapFactory bytecode and the Forwarder bytecode.
	if req.SwapFactoryAddress != ourSwapFactoryAddr {
		_, err := contracts.CheckSwapFactoryContractCode(ctx, ec, req.SwapFactoryAddress)
		if err != nil {
			return err
		}
	}

	asset := types.EthAsset(req.Swap.Asset)
	if asset != types.EthAssetETH {
		return fmt.Errorf("relaying for ETH Asset %s is not supported", asset)
	}

	// The relayer fee must be strictly less than the swap value
	if FeeWei.Cmp(req.Swap.Value) >= 0 {
		return fmt.Errorf("swap value of %s ETH is too low to support %s ETH relayer fee",
			coins.FmtWeiAsETH(req.Swap.Value), coins.FmtWeiAsETH(FeeWei))
	}

	return nil
}

// validateClaimSignature validates the claim signature. It is assumed that the
// request fields have already been validated.
func validateClaimSignature(
	ctx context.Context,
	ec *ethclient.Client,
	req *message.RelayClaimRequest,
) error {
	callOpts := &bind.CallOpts{
		Context: ctx,
		From:    ethcommon.Address{0xFF}, // can be any value but zero, which will validate all signatures
	}

	swapFactory, err := contracts.NewSwapFactory(req.SwapFactoryAddress, ec)
	if err != nil {
		return err
	}

	forwarderAddr, err := swapFactory.TrustedForwarder(&bind.CallOpts{Context: ctx})
	if err != nil {
		return err
	}

	forwarder, domainSeparator, err := getForwarderAndDomainSeparator(ctx, ec, forwarderAddr)
	if err != nil {
		return err
	}

	nonce, err := forwarder.GetNonce(callOpts, req.Swap.Claimer)
	if err != nil {
		return err
	}

	secret := (*[32]byte)(req.Secret)

	forwarderRequest, err := createForwarderRequest(
		nonce,
		req.SwapFactoryAddress,
		req.Swap,
		secret,
	)
	if err != nil {
		return err
	}

	err = forwarder.Verify(
		callOpts,
		*forwarderRequest,
		*domainSeparator,
		gsnforwarder.ForwardRequestTypehash,
		nil,
		req.Signature,
	)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	return nil
}
