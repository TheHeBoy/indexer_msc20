package model

type Holder struct {
	BaseModel

	Tick    string `gorm:"column:tick;size:20" json:"tick"`
	Address string `gorm:"column:address;size:42" json:"number"`
	Amount  uint64 `gorm:"column:amount" json:"amount"`
}

func (holder *Holder) CreateHolder() {
	DB.Create(holder)
	return
}

func (holder *Holder) SavaHolder() {
	DB.Save(holder)
	return
}

func GetHolder(to string, tick string) (holder Holder) {
	DB.Where("tick = ?", tick).Where("address = ?", to).Find(&holder)
	return
}
