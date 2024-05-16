package model

type List struct {
	BaseModel

	Hash     string `json:"hash"`
	Owner    string `json:"owner"`
	Exchange string `json:"exchange"`
	Tick     string `json:"tick"`
	Amount   uint64 `json:"amount"`
}

func (data *List) CreateList() {
	DB.Create(data)
	return
}

func GetListByHash(hash string) (list *List) {
	DB.Where("hash = ?", hash).First(&list)
	return
}

func (data *List) Remove() {
	DB.Delete(data)
	return
}
