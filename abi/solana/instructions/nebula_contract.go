package instructions

import "github.com/portto/solana-go-sdk/common"

type DataType uint8
type Subscription struct {
	Sender           common.PublicKey
	ContractAddress  common.PublicKey
	MinConfirmations uint8
	Reward           uint64
}
type Pulse struct {
	DataHash []byte
	Height   uint64
}

/*
   pub rounds_dict: HashMap<PulseID, bool>,
   subscriptions_queue: NebulaQueue<SubscriptionID>,
   pub oracles: Vec<Pubkey>,

   pub bft: u8,
   pub multisig_account: Pubkey,
   pub gravity_contract: Pubkey,
   pub data_type: DataType,
   pub last_round: PulseID,

   subscription_ids: Vec<SubscriptionID>,
   pub last_pulse_id: PulseID,

   subscriptions_map: HashMap<SubscriptionID, Subscription>,
   pulses_map: HashMap<PulseID, Pulse>,
   is_pulse_sent: HashMap<PulseID, bool>,

   pub is_initialized: bool,
   pub initializer_pubkey: Pubkey,
*/
type NebulaContract struct {
	RoundsDict         map[uint64]byte
	SubscriptionsQueue [][16]uint8
	Oracles            []common.PublicKey
	Bft                uint8
	MultisigAccount    common.PublicKey
	GravityContract    common.PublicKey
	DataType           DataType
	LastRound          uint64
	LastPulseId        uint64
	SubscriptionsMap   map[[16]uint8]Subscription
	PulsesMap          map[uint64]Pulse
	IsPulseSent        map[uint64]byte
	IsInitialized      byte
	InitializerPubkey  common.PublicKey
}

func NewNebulaContract() NebulaContract {
	c := NebulaContract{}
	c.RoundsDict = make(map[uint64]byte)
	c.SubscriptionsQueue = make([][16]uint8, 0)
	c.Oracles = make([]common.PublicKey, 0)
	c.SubscriptionsMap = make(map[[16]uint8]Subscription)
	c.PulsesMap = make(map[uint64]Pulse)
	c.IsPulseSent = make(map[uint64]byte)
	return c
}