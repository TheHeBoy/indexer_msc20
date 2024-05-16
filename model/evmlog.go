package model

type EvmLog struct {
	BaseModel

	Hash      string   `json:"hash"`
	Address   string   `json:"address"`
	Topics    []string `gorm:"serializer:json" json:"topics"`
	Data      string   `json:"data"`
	Block     uint64   `json:"block"`
	TxIndex   uint32   `json:"txIndex"`
	LogIndex  uint32   `json:"logIndex"`
	Timestamp uint64   `json:"timestamp"`
}

func CreateBatchesEvmLog(data []*EvmLog, chunkSize int) error {
	return DB.CreateInBatches(data, chunkSize).Error
}
