package model

type Transaction struct {
	BaseModel
	Hash      string `json:"hash"`
	From      string `json:"from"`
	To        string `json:"to"`
	Block     uint64 `json:"block"`
	Idx       uint32 `json:"idx"`
	Timestamp uint64 `json:"timestamp"`
	Input     string `json:"input"`
}

func CreateBatchesTransaction(data []*Transaction, chunkSize int) error {
	return DB.CreateInBatches(data, chunkSize).Error
}

func GetLatestTransaction() (transaction Transaction) {
	DB.Order("id desc").First(&transaction)
	return
}
