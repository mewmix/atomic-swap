package types

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/cockroachdb/apd/v3"
	"golang.org/x/crypto/sha3"

	"github.com/athanorlabs/atomic-swap/coins"
	"github.com/athanorlabs/atomic-swap/common/vjson"
)

var (
	// CurOfferVersion is the latest supported version of a serialised Offer struct
	CurOfferVersion, _ = semver.NewVersion("1.0.0")

	errOfferVersionMissing = errors.New(`required "version" field missing in offer`)
	errOfferIDNotSet       = errors.New(`"offerID" is not set`)
	errExchangeRateNil     = errors.New(`"exchangeRate" is not set`)
	errMinGreaterThanMax   = errors.New(`"minAmount" must be less than or equal to "maxAmount"`)
)

// Offer represents a swap offer
type Offer struct {
	Version      semver.Version      `json:"version"`
	ID           Hash                `json:"offerID" validate:"required"`
	Provides     coins.ProvidesCoin  `json:"provides" validate:"required"`
	MinAmount    *apd.Decimal        `json:"minAmount" validate:"required"` // Min XMR amount
	MaxAmount    *apd.Decimal        `json:"maxAmount" validate:"required"` // Max XMR amount
	ExchangeRate *coins.ExchangeRate `json:"exchangeRate" validate:"required"`
	EthAsset     EthAsset            `json:"ethAsset"`
	Nonce        uint64              `json:"nonce" validate:"required"`
}

// NewOffer creates and returns an Offer with an initialised ID and Version fields
func NewOffer(
	coin coins.ProvidesCoin,
	minAmount *apd.Decimal,
	maxAmount *apd.Decimal,
	exRate *coins.ExchangeRate,
	ethAsset EthAsset,
) *Offer {
	var n [8]byte
	if _, err := rand.Read(n[:]); err != nil {
		panic(err)
	}

	// We want the coefficients of apd decimals to be reduced before computing the
	// hash at the end. Otherwise an apd value like apd.New(10, -2) will print 0.10
	// instead of 0.1. The reduced form is apd.New(1, -1).
	_, _ = minAmount.Reduce(minAmount)
	_, _ = maxAmount.Reduce(maxAmount)
	_, _ = exRate.Decimal().Reduce(exRate.Decimal())

	offer := &Offer{
		Version:      *CurOfferVersion,
		Provides:     coin,
		MinAmount:    minAmount,
		MaxAmount:    maxAmount,
		ExchangeRate: exRate,
		EthAsset:     ethAsset,
		Nonce:        binary.BigEndian.Uint64(n[:]),
	}

	offer.setID()
	return offer
}

func (o *Offer) setID() {
	if !IsHashZero(o.ID) {
		panic("offer ID is already set")
	}

	o.ID = o.hash()
}

func (o *Offer) hash() Hash {
	b := append([]byte(o.Version.String()), []byte(o.Provides)...)
	b = append(b, []byte(",")...)
	b = append(b, []byte(o.MinAmount.Text('f'))...)
	b = append(b, []byte(",")...)
	b = append(b, []byte(o.MaxAmount.Text('f'))...)
	b = append(b, []byte(",")...)
	b = append(b, []byte(o.ExchangeRate.String())...)
	b = append(b, []byte(",")...)
	b = append(b, []byte(o.EthAsset.String())...)
	b = append(b, []byte(",")...)
	b = append(b, []byte(fmt.Sprintf("%d", o.Nonce))...)
	return sha3.Sum256(b)
}

// String ...
func (o *Offer) String() string {
	return fmt.Sprintf("OfferID:%s Provides:%s MinAmount:%s MaxAmount:%s ExchangeRate:%s EthAsset:%s Nonce:%d",
		o.ID,
		o.Provides,
		o.MinAmount.String(),
		o.MaxAmount.String(),
		o.ExchangeRate.String(),
		o.EthAsset,
		o.Nonce,
	)
}

// IsSet returns true if the offer's fields are all set.
func (o *Offer) IsSet() bool {
	return !IsHashZero(o.ID) &&
		o.Provides != "" &&
		o.MinAmount != nil &&
		o.MaxAmount != nil &&
		o.ExchangeRate != nil
}

func (o *Offer) validate() error {
	if IsHashZero(o.ID) {
		return errOfferIDNotSet
	}

	if err := coins.ValidatePositive("minAmount", coins.NumMoneroDecimals, o.MinAmount); err != nil {
		return err
	}
	if err := coins.ValidatePositive("maxAmount", coins.NumMoneroDecimals, o.MaxAmount); err != nil {
		return err
	}

	if o.MinAmount.Cmp(o.MaxAmount) > 0 {
		return errMinGreaterThanMax
	}

	// The JSON decoder for ExchangeRate does validation, but it can't check for nil, as
	// it won't get invoked when the value is not present.
	if o.ExchangeRate == nil {
		return errExchangeRateNil
	}

	if o.ID != o.hash() {
		return errors.New("hash of offer fields does not match offer ID")
	}

	return nil
}

// OfferExtra represents extra data that is passed when an offer is made.
type OfferExtra struct {
	StatusCh   chan Status `json:"-"`
	UseRelayer bool        `json:"useRelayer,omitempty"`
}

// UnmarshalOffer deserializes a JSON offer, checking the version for compatibility before
// attempting to deserialize the whole blob.
func UnmarshalOffer(jsonData []byte) (*Offer, error) {
	// First unmarshal into a struct that only has the version. Then, if we ever
	// have to support multiple versions, you can use the version to pick which
	// offer structure to deserialize the full data into.
	ov := struct {
		Version *semver.Version `json:"version"`
	}{}
	if err := json.Unmarshal(jsonData, &ov); err != nil {
		return nil, err
	}

	if ov.Version == nil {
		return nil, errOfferVersionMissing
	}

	if ov.Version.GreaterThan(CurOfferVersion) {
		return nil, fmt.Errorf("offer version %q not supported, latest is %q", ov.Version, CurOfferVersion)
	}

	o := new(Offer)
	if err := vjson.UnmarshalStruct(jsonData, o); err != nil {
		return nil, err
	}

	return o, nil
}

// MarshalJSON provides JSON marshalling for the Offer type
func (o *Offer) MarshalJSON() ([]byte, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}
	// Do standard JSON marshal without recursion
	type _Offer Offer
	return vjson.MarshalStruct((*_Offer)(o))
}

// UnmarshalJSON provides JSON unmarshalling the Offer type
func (o *Offer) UnmarshalJSON(data []byte) error {
	// Do standard JSON marshal without recursion
	type _Offer Offer
	if err := vjson.UnmarshalStruct(data, (*_Offer)(o)); err != nil {
		return err
	}
	return o.validate()
}
