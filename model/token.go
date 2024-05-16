package model

type Token struct {
	BaseModel

	Tick        string `gorm:"column:tick;size:18;unique" json:"tick"`
	Max         uint64 `gorm:"column:max" json:"max"`
	Limit       uint64 `gorm:"column:limit" json:"limit"`
	Minted      uint64 `gorm:"column:minted" json:"minted"`
	Progress    string `gorm:"column:progress;size:5;default:'0'" json:"progress"`
	Txs         uint32 `gorm:"column:txs" json:"txs"`
	CompletedAt uint64 `gorm:"column:completed_at" json:"completedAt"`
	DeployAt    uint64 `gorm:"column:deploy_at" json:"deployAt"`
}

func (data *Token) CreateToken() {
	DB.Create(data)
	return
}

func (data *Token) SaveToken() {
	DB.Save(data)
	return
}

func GetALlToken() (tokens []*Token) {
	DB.Find(&tokens)
	return
}

func GetTokenByTick(tick string) (token *Token) {
	DB.Where("tick = ?", tick).First(&token)
	return
}
