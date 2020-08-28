package adaptors

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"

	"github.com/Gravity-Tech/gravity-core/common/account"
	"github.com/Gravity-Tech/gravity-core/common/client"
	"github.com/Gravity-Tech/gravity-core/common/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	Int64  SubType = 0
	String SubType = 1
	Bytes  SubType = 2
)

type SubType uint8
type EthereumAdaptor struct {
	privKey *ecdsa.PrivateKey

	ghClient  *client.GravityClient
	ethClient *ethclient.Client

	gravityContract *contracts.Gravity
}
type EthereumAdapterOption func(*EthereumAdaptor) error

func WithEthereumGravityContract(address string) EthereumAdapterOption {
	return func(h *EthereumAdaptor) error {
		hexAddress, err := hexutil.Decode(address)
		if err != nil {
			return err
		}
		ethContractAddress := common.Address{}
		ethContractAddress.SetBytes(hexAddress)
		h.gravityContract, err = contracts.NewGravity(ethContractAddress, h.ethClient)
		if err != nil {
			return err
		}

		return nil
	}
}
func EthAdapterWithGhClient(ghClient *client.GravityClient) EthereumAdapterOption {
	return func(h *EthereumAdaptor) error {
		h.ghClient = ghClient
		return nil
	}
}

func NewEthereumAdaptor(privKey []byte, nodeUrl string, ctx context.Context, opts ...EthereumAdapterOption) (*EthereumAdaptor, error) {
	ethClient, err := ethclient.DialContext(ctx, nodeUrl)
	if err != nil {
		return nil, err
	}

	ethPrivKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: secp256k1.S256(),
		},
		D: new(big.Int),
	}
	ethPrivKey.D.SetBytes(privKey)
	ethPrivKey.PublicKey.X, ethPrivKey.PublicKey.Y = ethPrivKey.PublicKey.Curve.ScalarBaseMult(privKey)

	adapter := &EthereumAdaptor{
		privKey:   ethPrivKey,
		ethClient: ethClient,
	}
	for _, opt := range opts {
		err := opt(adapter)
		if err != nil {
			return nil, err
		}
	}

	return adapter, nil
}

func (adaptor *EthereumAdaptor) GetHeight(ctx context.Context) (uint64, error) {
	tcHeightRq, err := adaptor.ethClient.BlockByNumber(ctx, nil)
	if err != nil {
		return 0, err
	}

	return tcHeightRq.NumberU64(), nil
}
func (adaptor *EthereumAdaptor) Sign(msg []byte) ([]byte, error) {
	sig, err := crypto.Sign(msg, adaptor.privKey)
	if err != nil {
		return nil, err
	}

	return sig, nil
}
func (adaptor *EthereumAdaptor) WaitTx(id string, ctx context.Context) error {
	return nil
}
func (adaptor *EthereumAdaptor) PubKey() account.OraclesPubKey {
	var oraclePubKey account.OraclesPubKey
	copy(oraclePubKey[:], crypto.PubkeyToAddress(adaptor.privKey.PublicKey).Bytes())
	return oraclePubKey
}

func (adaptor *EthereumAdaptor) SendDataResult(nebulaId account.NebulaId, tcHeight uint64, validators []account.OraclesPubKey, hash []byte, ctx context.Context) (string, error) {
	nebula, err := contracts.NewNebula(common.BytesToAddress(nebulaId), adaptor.ethClient)
	if err != nil {
		return "", err
	}

	data, err := nebula.Pulses(nil, big.NewInt(int64(tcHeight)))
	if err != nil {
		return "", err
	}

	if bytes.Equal(data[:], make([]byte, 32, 32)) != true {
		return "", nil
	}

	bft, err := nebula.BftValue(nil)
	if err != nil {
		return "", err
	}

	realSignCount := 0

	oracles, err := nebula.GetOracles(nil)
	if err != nil {
		return "", err
	}
	var r [5][32]byte
	var s [5][32]byte
	var v [5]uint8
	for _, validator := range validators {
		pubKey, err := crypto.DecompressPubkey(validator.ToBytes(account.Ethereum))
		if err != nil {
			return "", err
		}
		validatorAddress := crypto.PubkeyToAddress(*pubKey)
		position := 0
		isExist := false
		for i, address := range oracles {
			if validatorAddress == address {
				position = i
				isExist = true
				break
			}
		}
		if !isExist {
			continue
		}

		sign, err := adaptor.ghClient.Result(account.Ethereum, nebulaId, int64(tcHeight), validator)
		if err != nil {
			r[position] = [32]byte{}
			s[position] = [32]byte{}
			v[position] = byte(0)
			continue
		}
		copy(r[position][:], sign[:32])
		copy(s[position][:], sign[32:64])
		v[position] = sign[64] + 27

		realSignCount++
	}

	if realSignCount < int(bft.Uint64()) {
		return "", nil
	}

	var resultBytes32 [32]byte
	copy(resultBytes32[:], hash)
	tx, err := nebula.SendHashValue(bind.NewKeyedTransactor(adaptor.privKey), resultBytes32, v[:], r[:], s[:])
	if err != nil {
		return "", err
	}

	fmt.Printf("Tx finilize: %s \n", tx.Hash().String())

	return tx.Hash().String(), nil
}
func (adaptor *EthereumAdaptor) SendDataToSubs(nebulaId account.NebulaId, tcHeight uint64, value interface{}, ctx context.Context) error {
	var err error

	nebula, err := contracts.NewNebula(common.BytesToAddress(nebulaId), adaptor.ethClient)
	if err != nil {
		return err
	}

	ids, err := nebula.GetSubscribersIds(nil)
	if err != nil {
		return err
	}

	for _, id := range ids {
		t, err := nebula.DataType(nil)
		if err != nil {
			return err
		}

		transactOpt := bind.NewKeyedTransactor(adaptor.privKey)

		switch SubType(t) {
		case Int64:
			_, err = nebula.SendValueToSubInt(transactOpt, value.(int64), big.NewInt(int64(tcHeight)), id)
			if err != nil {
				return err
			}
		case String:
			_, err = nebula.SendValueToSubString(transactOpt, value.(string), big.NewInt(int64(tcHeight)), id)
			if err != nil {
				return err
			}
		case Bytes:
			_, err = nebula.SendValueToSubByte(transactOpt, value.([]byte), big.NewInt(int64(tcHeight)), id)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (adaptor *EthereumAdaptor) SendOraclesToNebula(nebulaId account.NebulaId, oracles []account.OraclesPubKey, signs [][]byte, round int64, ctx context.Context) (string, error) {
	nebula, err := contracts.NewNebula(common.BytesToAddress(nebulaId), adaptor.ethClient)
	if err != nil {
		return "", err
	}

	lastRound, err := nebula.Rounds(nil, big.NewInt(round))
	if err != nil {
		return "", err
	}

	if lastRound {
		return "", err
	}

	var oraclesAddresses []common.Address
	for _, v := range oracles {
		oraclesAddresses = append(oraclesAddresses, common.BytesToAddress(v.ToBytes(account.Ethereum)))
	}

	var r [][32]byte
	var s [][32]byte
	var v []uint8
	for _, sign := range signs {
		var bytes32R [32]byte
		copy(bytes32R[:], sign[:32])
		var bytes32S [32]byte
		copy(bytes32S[:], sign[32:64])

		r = append(r, bytes32R)
		s = append(s, bytes32S)
		v = append(v, sign[64:][0]+27)
	}

	tx, err := nebula.UpdateOracles(bind.NewKeyedTransactor(adaptor.privKey), oraclesAddresses, v, r, s, big.NewInt(round))
	if err != nil {
		return "", err
	}

	return tx.Hash().Hex(), nil
}
func (adaptor *EthereumAdaptor) SendConsulsToGravityContract(newConsulsAddresses []account.OraclesPubKey, signs [][]byte, round int64, ctx context.Context) (string, error) {
	lastRound, err := adaptor.gravityContract.Rounds(nil, big.NewInt(round))
	if err != nil {
		return "", err
	}

	if lastRound {
		return "", nil
	}

	var r [][32]byte
	var s [][32]byte
	var v []uint8
	for _, sign := range signs {
		var bytes32R [32]byte
		copy(bytes32R[:], sign[:32])
		var bytes32S [32]byte
		copy(bytes32S[:], sign[32:64])

		r = append(r, bytes32R)
		s = append(s, bytes32S)
		v = append(v, sign[64:][0]+27)
	}

	var consulsAddress []common.Address

	for _, v := range newConsulsAddresses {
		consulsAddress = append(consulsAddress, common.BytesToAddress(v.ToBytes(account.Ethereum)))
	}

	tx, err := adaptor.gravityContract.UpdateConsuls(bind.NewKeyedTransactor(adaptor.privKey), consulsAddress, v, r, s, big.NewInt(round))
	if err != nil {
		return "", nil
	}

	return tx.Hash().Hex(), nil
}
func (adaptor *EthereumAdaptor) SignConsuls(consulsAddresses []account.OraclesPubKey) ([]byte, error) {
	var oraclesAddresses []common.Address
	for _, v := range consulsAddresses {
		oraclesAddresses = append(oraclesAddresses, common.BytesToAddress(v.ToBytes(account.Ethereum)))
	}
	hash, err := adaptor.gravityContract.HashNewConsuls(nil, oraclesAddresses)
	if err != nil {
		return nil, err
	}

	sign, err := adaptor.Sign(hash[:])
	if err != nil {
		return nil, err
	}

	return sign, nil
}
func (adaptor *EthereumAdaptor) SignOracles(nebulaId account.NebulaId, oracles []account.OraclesPubKey) ([]byte, error) {
	nebula, err := contracts.NewNebula(common.BytesToAddress(nebulaId), adaptor.ethClient)
	if err != nil {
		return nil, err
	}

	var oraclesAddresses []common.Address
	for _, v := range oracles {
		oraclesAddresses = append(oraclesAddresses, common.BytesToAddress(v.ToBytes(account.Ethereum)))
	}

	hash, err := nebula.HashNewOracles(nil, oraclesAddresses)
	if err != nil {
		return nil, err
	}

	sign, err := adaptor.Sign(hash[:])
	if err != nil {
		return nil, err
	}

	return sign, nil
}
