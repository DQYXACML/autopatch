package node

import (
	"errors"
	"fmt"
	"github.com/DQYXACML/autopatch/common/bigint"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
)

var (
	ErrHeaderTraversalAheadOfProvider            = errors.New("the HeaderTraversal's internal state is ahead of the provider")
	ErrHeaderTraversalAndProviderMismatchedState = errors.New("the HeaderTraversal and provider have diverged in state")
)

type HeaderTraversal struct {
	ethClient EthClient
	chainId   uint

	latestHeader        *types.Header
	lastTraversedHeader *types.Header

	blockConfirmationDepth *big.Int
}

func NewHeaderTraversal(ethClient EthClient, fromHeader *types.Header, blockConfirmationDepth *big.Int, chainId uint) *HeaderTraversal {
	return &HeaderTraversal{
		ethClient:              ethClient,
		chainId:                chainId,
		blockConfirmationDepth: blockConfirmationDepth,
		lastTraversedHeader:    fromHeader,
	}
}

func (f *HeaderTraversal) LatestHeader() *types.Header {
	return f.latestHeader
}

func (f *HeaderTraversal) LastTraversedHeader() *types.Header {
	return f.lastTraversedHeader
}

func (f *HeaderTraversal) NextHeaders(maxSize uint64) ([]types.Header, error) {
	latestHeader, err := f.ethClient.BlockHeaderByNumber(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to query latest block: %w", err)
	} else if latestHeader == nil {
		return nil, fmt.Errorf("latest header unreported")
	} else {
		f.latestHeader = latestHeader
	}

	endHeight := new(big.Int).Sub(latestHeader.Number, f.blockConfirmationDepth)
	log.Info("endHeight after sub", "endHeight", endHeight, "latestHeader", latestHeader.Number, "blockConfirmationDepth", f.blockConfirmationDepth, "lastTraversedHeader Number", f.lastTraversedHeader.Number)
	if endHeight.Sign() < 0 {
		return nil, nil
	}

	if f.lastTraversedHeader != nil {
		cmp := f.lastTraversedHeader.Number.Cmp(endHeight)
		if cmp == 0 {
			return nil, nil
		} else if cmp > 0 {
			return nil, ErrHeaderTraversalAheadOfProvider
		}
	}

	nextHeight := big.NewInt(0)
	if f.lastTraversedHeader != nil {
		nextHeight = new(big.Int).Add(f.lastTraversedHeader.Number, big.NewInt(1))
	}

	endHeight = bigint.Clamp(nextHeight, endHeight, maxSize)
	log.Info("endHeight after clamp", "endHeight", endHeight, "nextHeight", nextHeight, "maxSize", maxSize)
	headers, err := f.ethClient.BlockHeadersByRange(nextHeight, endHeight, f.chainId)
	if err != nil {
		return nil, fmt.Errorf("error querying blocks by range: %w", err)
	}

	numHeaders := len(headers)
	log.Info("number of headers fetched", "numHeaders", numHeaders, "nextHeight", nextHeight, "endHeight", endHeight)
	if numHeaders == 0 {
		return nil, nil
	} else if f.lastTraversedHeader != nil && headers[0].ParentHash != f.lastTraversedHeader.Hash() {
		//fmt.Println(f.lastTraversedHeader.Number)
		//fmt.Println(headers[0].Number)
		//fmt.Println(len(headers))
		return nil, ErrHeaderTraversalAndProviderMismatchedState
	}
	f.lastTraversedHeader = &headers[numHeaders-1]
	return headers, nil
}
