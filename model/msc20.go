package model

type Msc20 struct {
	BaseModel

	Tick      string `gorm:"column:tick;size:18" json:"tick"`
	From      string `gorm:"column:from;size:42" json:"from"`
	To        string `gorm:"column:to;size:42" json:"to"`
	Operation string `gorm:"column:operation;size:20" json:"operation"`
	Limit     uint64 `gorm:"column:limit" json:"limit"`
	Amount    uint64 `gorm:"column:amount" json:"amount"`
	Hash      string `gorm:"column:hash;size:66" json:"hash"`
	Block     uint64 `gorm:"column:block" json:"block"`
	Timestamp uint64 `gorm:"column:timestamp" json:"timestamp"`
	Valid     int8   `gorm:"column:valid" json:"valid"`
}

func (data *Msc20) CreateMsc20() {
	DB.Create(data)
	return
}
