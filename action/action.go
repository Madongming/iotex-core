// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// SignatureLength indicates the length of signature generated by SECP256K1 crypto library
const SignatureLength = 65

var (
	// ErrAction indicates error for an action
	ErrAction = errors.New("action error")
	// ErrAddress indicates error of address
	ErrAddress = errors.New("address error")
)

// Action is the action can be Executed in protocols. The method is added to avoid mistakenly used empty interface as action.
type Action interface {
	SetEnvelopeContext(SealedEnvelope)
}

type actionPayload interface {
	ByteStream() []byte
	Cost() (*big.Int, error)
	IntrinsicGas() (uint64, error)
	SetEnvelopeContext(SealedEnvelope)
}

type hasDestination interface {
	Destination() string
}

// Envelope defines an envelope wrapped on action with some envelope metadata.
type Envelope struct {
	version  uint32
	nonce    uint64
	gasLimit uint64
	payload  actionPayload
	gasPrice *big.Int
}

// SealedEnvelope is a signed action envelope.
type SealedEnvelope struct {
	Envelope

	srcPubkey keypair.PublicKey
	signature []byte
}

// Version returns the version
func (elp *Envelope) Version() uint32 { return elp.version }

// Nonce returns the nonce
func (elp *Envelope) Nonce() uint64 { return elp.nonce }

// Destination returns the destination address
func (elp *Envelope) Destination() (string, bool) {
	r, ok := elp.payload.(hasDestination)
	if !ok {
		return "", false
	}

	return r.Destination(), true
}

// GasLimit returns the gas limit
func (elp *Envelope) GasLimit() uint64 { return elp.gasLimit }

// GasPrice returns the gas price
func (elp *Envelope) GasPrice() *big.Int {
	p := &big.Int{}
	if elp.gasPrice == nil {
		return p
	}
	return p.Set(elp.gasPrice)
}

// Cost returns cost of actions
func (elp *Envelope) Cost() (*big.Int, error) {
	return elp.payload.Cost()
}

// IntrinsicGas returns intrinsic gas of action.
func (elp *Envelope) IntrinsicGas() (uint64, error) {
	return elp.payload.IntrinsicGas()
}

// Action returns the action payload.
func (elp *Envelope) Action() Action { return elp.payload }

// Proto convert Envelope to protobuf format.
func (elp *Envelope) Proto() *iotextypes.ActionCore {
	actCore := &iotextypes.ActionCore{
		Version:  elp.version,
		Nonce:    elp.nonce,
		GasLimit: elp.gasLimit,
	}
	if elp.gasPrice != nil {
		actCore.GasPrice = elp.gasPrice.String()
	}

	// TODO assert each action
	act := elp.Action()
	switch act := act.(type) {
	case *Transfer:
		actCore.Action = &iotextypes.ActionCore_Transfer{Transfer: act.Proto()}
	case *Vote:
		actCore.Action = &iotextypes.ActionCore_Vote{Vote: act.Proto()}
	case *Execution:
		actCore.Action = &iotextypes.ActionCore_Execution{Execution: act.Proto()}
	case *PutBlock:
		actCore.Action = &iotextypes.ActionCore_PutBlock{PutBlock: act.Proto()}
	case *StartSubChain:
		actCore.Action = &iotextypes.ActionCore_StartSubChain{StartSubChain: act.Proto()}
	case *StopSubChain:
		actCore.Action = &iotextypes.ActionCore_StopSubChain{StopSubChain: act.Proto()}
	case *CreateDeposit:
		actCore.Action = &iotextypes.ActionCore_CreateDeposit{CreateDeposit: act.Proto()}
	case *SettleDeposit:
		actCore.Action = &iotextypes.ActionCore_SettleDeposit{SettleDeposit: act.Proto()}
	case *GrantReward:
		actCore.Action = &iotextypes.ActionCore_GrantReward{GrantReward: act.Proto()}
	case *SetReward:
		actCore.Action = &iotextypes.ActionCore_SetReward{SetReward: act.Proto()}
	case *ClaimFromRewardingFund:
		actCore.Action = &iotextypes.ActionCore_ClaimFromRewardingFund{ClaimFromRewardingFund: act.Proto()}
	case *DepositToRewardingFund:
		actCore.Action = &iotextypes.ActionCore_DepositToRewardingFund{DepositToRewardingFund: act.Proto()}
	case *PutPollResult:
		actCore.Action = &iotextypes.ActionCore_PutPollResult{PutPollResult: act.Proto()}
	default:
		log.S().Panicf("Cannot convert type of action %T.\r\n", act)
	}
	return actCore
}

// LoadProto loads fields from protobuf format.
func (elp *Envelope) LoadProto(pbAct *iotextypes.ActionCore) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	if elp == nil {
		return errors.New("nil action to load proto")
	}
	*elp = Envelope{}
	elp.version = pbAct.GetVersion()
	elp.nonce = pbAct.GetNonce()
	elp.gasLimit = pbAct.GetGasLimit()
	elp.gasPrice = &big.Int{}
	elp.gasPrice.SetString(pbAct.GetGasPrice(), 10)

	switch {
	case pbAct.GetTransfer() != nil:
		act := &Transfer{}
		if err := act.LoadProto(pbAct.GetTransfer()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetVote() != nil:
		act := &Vote{}
		if err := act.LoadProto(pbAct.GetVote()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetExecution() != nil:
		act := &Execution{}
		if err := act.LoadProto(pbAct.GetExecution()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetPutBlock() != nil:
		act := &PutBlock{}
		if err := act.LoadProto(pbAct.GetPutBlock()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetStartSubChain() != nil:
		act := &StartSubChain{}
		if err := act.LoadProto(pbAct.GetStartSubChain()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetStopSubChain() != nil:
		act := &StopSubChain{}
		if err := act.LoadProto(pbAct.GetStopSubChain()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetCreateDeposit() != nil:
		act := &CreateDeposit{}
		if err := act.LoadProto(pbAct.GetCreateDeposit()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetSettleDeposit() != nil:
		act := &SettleDeposit{}
		if err := act.LoadProto(pbAct.GetSettleDeposit()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetGrantReward() != nil:
		act := &GrantReward{}
		if err := act.LoadProto(pbAct.GetGrantReward()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetSetReward() != nil:
		act := &SetReward{}
		if err := act.LoadProto(pbAct.GetSetReward()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetClaimFromRewardingFund() != nil:
		act := &ClaimFromRewardingFund{}
		if err := act.LoadProto(pbAct.GetClaimFromRewardingFund()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetDepositToRewardingFund() != nil:
		act := &DepositToRewardingFund{}
		if err := act.LoadProto(pbAct.GetDepositToRewardingFund()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetPutPollResult() != nil:
		act := &PutPollResult{}
		if err := act.LoadProto(pbAct.GetPutPollResult()); err != nil {
			return err
		}
		elp.payload = act
	default:
		return errors.Errorf("no applicable action to handle in action proto %+v", pbAct)
	}
	return nil
}

// ByteStream returns encoded binary.
func (elp *Envelope) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(elp.Proto()))
}

// Hash returns the hash value of SealedEnvelope.
func (elp *Envelope) Hash() hash.Hash256 {
	return blake2b.Sum256(elp.ByteStream())
}

// Hash returns the hash value of SealedEnvelope.
func (sealed *SealedEnvelope) Hash() hash.Hash256 {
	return blake2b.Sum256(byteutil.Must(proto.Marshal(sealed.Proto())))
}

// SrcPubkey returns the source public key
func (sealed *SealedEnvelope) SrcPubkey() keypair.PublicKey { return sealed.srcPubkey }

// Signature returns signature bytes
func (sealed *SealedEnvelope) Signature() []byte {
	sig := make([]byte, len(sealed.signature))
	copy(sig, sealed.signature)
	return sig
}

// Proto converts it to it's proto scheme.
func (sealed SealedEnvelope) Proto() *iotextypes.Action {
	return &iotextypes.Action{
		Core:         sealed.Envelope.Proto(),
		SenderPubKey: keypair.PublicKeyToBytes(sealed.srcPubkey),
		Signature:    sealed.signature,
	}
}

// LoadProto loads from proto scheme.
func (sealed *SealedEnvelope) LoadProto(pbAct *iotextypes.Action) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	srcPub, err := keypair.BytesToPublicKey(pbAct.GetSenderPubKey())
	if err != nil {
		return err
	}
	if sealed == nil {
		return errors.New("nil action to load proto")
	}
	*sealed = SealedEnvelope{}

	sealed.srcPubkey = srcPub
	sealed.signature = make([]byte, len(pbAct.GetSignature()))
	copy(sealed.signature, pbAct.GetSignature())
	if err := sealed.Envelope.LoadProto(pbAct.GetCore()); err != nil {
		return err
	}

	sealed.payload.SetEnvelopeContext(*sealed)
	return nil
}

// Sign signs the action using sender's private key
func Sign(act Envelope, sk keypair.PrivateKey) (SealedEnvelope, error) {
	sealed := SealedEnvelope{Envelope: act}

	sealed.srcPubkey = &sk.PublicKey

	hash := act.Hash()
	sig, err := crypto.Sign(hash[:], sk)
	if err != nil {
		return sealed, errors.Wrapf(ErrAction, "failed to sign action hash = %x", hash)
	}
	sealed.signature = sig
	sealed.payload.SetEnvelopeContext(sealed)
	return sealed, nil
}

// FakeSeal creates a SealedActionEnvelope without signature.
// This method should be only used in tests.
func FakeSeal(act Envelope, pubk keypair.PublicKey) SealedEnvelope {
	sealed := SealedEnvelope{
		Envelope:  act,
		srcPubkey: pubk,
	}
	sealed.payload.SetEnvelopeContext(sealed)
	return sealed
}

// AssembleSealedEnvelope assembles a SealedEnvelope use Envelope, Sender Address and Signature.
// This method should be only used in tests.
func AssembleSealedEnvelope(act Envelope, pk keypair.PublicKey, sig []byte) SealedEnvelope {
	sealed := SealedEnvelope{
		Envelope:  act,
		srcPubkey: pk,
		signature: sig,
	}
	sealed.payload.SetEnvelopeContext(sealed)
	return sealed
}

// Verify verifies the action using sender's public key
func Verify(sealed SealedEnvelope) error {
	hash := sealed.Envelope.Hash()
	if len(sealed.Signature()) != SignatureLength {
		return errors.New("incorrect length of signature")
	}
	if success := crypto.VerifySignature(keypair.PublicKeyToBytes(sealed.SrcPubkey()), hash[:],
		sealed.Signature()[:SignatureLength-1]); success {
		return nil
	}
	return errors.Wrapf(
		ErrAction,
		"failed to verify action hash = %x and signature = %x",
		hash,
		sealed.Signature(),
	)
}

// ClassifyActions classfies actions
func ClassifyActions(actions []SealedEnvelope) ([]*Transfer, []*Vote, []*Execution) {
	tsfs := make([]*Transfer, 0)
	votes := make([]*Vote, 0)
	exes := make([]*Execution, 0)
	for _, elp := range actions {
		act := elp.Action()
		switch act := act.(type) {
		case *Transfer:
			tsfs = append(tsfs, act)
		case *Vote:
			votes = append(votes, act)
		case *Execution:
			exes = append(exes, act)
		}
	}
	return tsfs, votes, exes
}
