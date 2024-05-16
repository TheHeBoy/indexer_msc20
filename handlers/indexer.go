package handlers

import (
	"encoding/hex"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"indexer/model"
	"indexer/mq"
	"indexer/utils"
	"sort"
	"strings"
	"time"
)

func mixRecords(txs []*model.Transaction, logs []*model.EvmLog) []*model.Record {
	var records []*model.Record
	for _, tx := range txs {
		var record model.Record
		record.IsLog = false
		record.Transaction = tx
		record.Block = tx.Block
		record.TransactionIndex = tx.Idx
		record.LogIndex = 0
		records = append(records, &record)
	}
	for _, log := range logs {
		var record model.Record
		record.IsLog = true
		record.EvmLog = log
		record.Block = log.Block
		record.TransactionIndex = log.TxIndex
		record.LogIndex = log.LogIndex
		records = append(records, &record)
	}

	// tx 的顺序在 log 前面
	sort.SliceStable(records, func(i, j int) bool {
		record0 := records[i]
		record1 := records[j]
		if record0.Block == record1.Block {
			if record0.TransactionIndex == record1.TransactionIndex {
				return record0.LogIndex-utils.BoolToUint32(record0.IsLog) < record1.LogIndex-utils.BoolToUint32(record1.IsLog)
			}
			return record0.TransactionIndex < record1.TransactionIndex
		}
		return record0.Block < record1.Block
	})
	return records
}

func processRecords(records []*model.Record) error {
	if len(records) == 0 {
		return nil
	}
	logger.Println("process ", len(records), " records")

	var err error
	for _, record := range records {
		if record.IsLog {
			err = indexLog(record.EvmLog)
		} else {
			err = indexTransaction(record.Transaction)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func indexTransaction(tx *model.Transaction) error {
	// filter
	if ignoreHashes[tx.Hash] {
		return nil
	}
	// data:text/plain;rule=esip6, current only support json token
	if !strings.HasPrefix(tx.Input, "0x646174613a746578742f706c61696e3b72756c653d65736970362c") {
		return nil
	}

	bytes, err := hex.DecodeString(tx.Input[2:])
	if err != nil {
		logger.Warn("inscribe err", err, " at block ", tx.Block, ":", tx.Idx)
		return nil
	}
	input := string(bytes)

	sepIdx := strings.Index(input, ",")
	if sepIdx == -1 || sepIdx == len(input)-1 {
		return nil
	}
	content := input[sepIdx+1:]

	// save inscription
	var inscription model.Inscription
	inscription.From = tx.From
	inscription.To = tx.To
	inscription.Block = tx.Block
	inscription.Hash = tx.Hash
	inscription.Idx = tx.Idx
	inscription.Timestamp = tx.Timestamp
	inscription.ContentType = "text/plain"
	inscription.Content = content

	if tx.To != "" {
		if err := handleTransaction(&inscription); err != nil {
			logger.Infof("error at %+v", inscription)
			return err
		}
	}

	inscription.CreateInscription()
	if inscription.ID == 0 {
		return errors.New("failed to save inscription")
	}

	return nil
}

func indexLog(log *model.EvmLog) error {
	if len(log.Topics) < 3 {
		return nil
	}
	var topicType uint8
	if log.Topics[0] == "0x62759d0ae69f633fdc0c23409a4ece704de005c608c4937553e08b3fea047114" {
		// scriptions_protocol_TransferMSC20Token(address,address,string,uint256)
		topicType = 1
	} else if log.Topics[0] == "0x95286129b3b09012858ff6203aff3142c9faa519554c271012ad3f3a1483ea26" {
		// scriptions_protocol_TransferMSC20TokenForListing(address,address,bytes32)
		topicType = 2
	} else {
		return nil
	}

	var msc20 model.Msc20
	msc20.From = utils.TopicToAddress(log.Topics[1])
	msc20.To = utils.TopicToAddress(log.Topics[2])
	msc20.Block = log.Block
	msc20.Timestamp = log.Timestamp
	msc20.Hash = log.Hash
	if topicType == 1 { // transfer
		logger.Info("current not support transfer")
		return nil
	} else { // exchange
		msc20.Operation = "exchange"

		list := model.GetListByHash(log.Data)
		if list.ID != 0 {
			//  list.Exchange 为 合约地址
			if list.Owner == msc20.From && list.Exchange == log.Address {
				msc20.Tick = list.Tick
				msc20.Amount = list.Amount
				// update from to exchange
				msc20.From = log.Address

				var err error
				msc20.Valid, err = exchangeToken(list, msc20.To)
				if err != nil {
					return err
				}
			} else {
				if list.Owner != msc20.From {
					msc20.Valid = -54
					logger.Warningln("failed to validate transfer from:", msc20.From, list.Owner)
				} else {
					msc20.Valid = -55
					logger.Warningln("failed to validate exchange:", log.Address, list.Exchange)
				}
			}

		} else {
			msc20.Valid = -53
			logger.Warningln("failed to transfer, list not found, id:", log.Data)
		}
	}

	msc20.CreateMsc20()
	if msc20.ID == 0 {
		return errors.New("failed to save msc20")
	}
	return nil
}

func handleTransaction(inscription *model.Inscription) error {
	content := strings.TrimSpace(inscription.Content)
	if len(content) > 0 && content[0] == '{' {
		var protoData map[string]string
		err := json.Unmarshal([]byte(content), &protoData)
		if err != nil {
			return nil
		}

		protocol, ok := protoData["p"]
		if ok && strings.TrimSpace(protocol) != "" {
			p := strings.ToLower(protocol)
			if p == "msc-20" {
				var msc20 model.Msc20
				msc20.From = inscription.From
				msc20.To = inscription.To
				msc20.Block = inscription.Block
				msc20.Timestamp = inscription.Timestamp
				msc20.Hash = inscription.Hash

				// check tick
				if tick, ok := protoData["tick"]; ok {
					// trim
					msc20.Tick = strings.ToLower(strings.TrimSpace(tick))
				}

				if op, ok := protoData["op"]; ok {
					msc20.Operation = op
				}

				var err error
				if msc20.Tick == "" {
					msc20.Valid = -1 // empty tick
				} else if len(msc20.Tick) > 18 {
					msc20.Valid = -2 // too long tick
				} else {
					switch msc20.Operation {
					case "deploy":
						msc20.Valid, err = deployToken(&msc20, protoData)
					case "mint":
						msc20.Valid, err = mintToken(&msc20, protoData)
					case "transfer":
						msc20.Valid, err = transferToken(&msc20, protoData)
					case "list":
						msc20.Valid, err = listToken(&msc20, protoData)
					default:
						msc20.Valid = -3 // wrong operation
					}
				}

				msc20.CreateMsc20()
				if msc20.ID == 0 {
					return errors.New("failed to save msc20")
				}

				if err != nil {
					return err
				}
				return nil
			}
		}
	}
	return nil
}

func deployToken(msc20 *model.Msc20, params map[string]string) (int8, error) {

	value, ok := params["max"]
	if !ok {
		return -11, nil
	}
	max, err := utils.ParseUint64(value)
	if err != nil {
		return -12, nil
	}

	value, ok = params["lim"]
	if !ok {
		return -13, nil
	}
	limit, err := utils.ParseUint64(value)
	if err != nil {
		return -14, nil
	}

	if max <= 0 || limit <= 0 || max < limit {
		return -15, nil
	}

	msc20.Amount = max
	msc20.Limit = limit

	// 已经 deploy
	t := model.GetTokenByTick(msc20.Tick)
	if t.ID != 0 {
		logger.Info("token ", msc20.Tick, " has deployed at ", msc20.ID)
		return -16, nil
	}

	logger.Info("token ", msc20.Tick, " deployed at ", msc20.ID)

	token := &model.Token{
		Tick:        msc20.Tick,
		Max:         max,
		Limit:       limit,
		Minted:      uint64(0),
		Progress:    "",
		DeployAt:    msc20.Timestamp,
		CompletedAt: uint64(0),
	}

	token.CreateToken()
	if token.ID == 0 {
		return -17, errors.New("failed to save token")
	}

	return 1, nil
}

func mintToken(msc20 *model.Msc20, params map[string]string) (int8, error) {
	value, ok := params["amt"]
	if !ok {
		return -21, nil
	}
	amt, err := utils.ParseUint64(value)
	if err != nil {
		return -22, nil
	}
	msc20.Amount = amt
	if amt <= 0 {
		return -25, nil
	}

	// 检查 token 是否存在

	token := model.GetTokenByTick(msc20.Tick)
	if token.ID == 0 {
		logger.Info("token ", msc20.Tick, " hasn't deployed")
		return -23, nil
	}

	if amt > token.Limit {
		return -26, nil
	}

	// 剩余可 mint 的数量
	var left = token.Max - token.Minted
	if left <= 0 || amt > left {
		logger.Info("token ", msc20.Tick, " minted exceed max")
		return -27, nil
	}

	err = addBalance(msc20.To, msc20.Tick, amt)
	if err != nil {
		return 0, err
	}

	// update token
	token.Minted = token.Minted + amt
	token.Txs++

	// 计算mint百分比
	a := decimal.NewFromFloat(float64(token.Minted))
	b := decimal.NewFromInt(int64(token.Max))
	hundred := decimal.NewFromInt(100)
	token.Progress = a.Div(b).Mul(hundred).Round(2).String()

	if token.Minted == token.Max {
		token.CompletedAt = uint64(time.Now().Unix())
	}

	token.SaveToken()
	return 1, nil
}

func transferToken(asc20 *model.Msc20, params map[string]string) (int8, error) {
	value, ok := params["amt"]
	if !ok {
		return -31, nil
	}
	amt, err := utils.ParseUint64(value)
	if err != nil {
		return -32, nil
	}

	asc20.Amount = amt

	return _transferToken(asc20)
}

func listToken(asc20 *model.Msc20, params map[string]string) (int8, error) {
	value, ok := params["amt"]
	if !ok {
		return -41, nil
	}
	amt, err := utils.ParseUint64(value)
	if err != nil {
		return -42, nil
	}

	asc20.Amount = amt

	return _listToken(asc20)
}

func _listToken(asc20 *model.Msc20) (int8, error) {
	tick := asc20.Tick
	token := model.GetTokenByTick(tick)
	if token.ID == 0 {
		return -43, nil
	}
	asc20.Tick = tick

	if asc20.Amount <= 0 {
		return -44, nil
	}

	if asc20.From == asc20.To {
		// list to self
		return -45, nil
	}

	// sub balance
	err := subBalance(asc20.From, tick, asc20.Amount)
	if err != nil {
		if err.Error() == "insufficient balance" {
			return -47, nil
		}
		return 0, err
	}

	var list model.List
	list.Hash = asc20.Hash
	list.Owner = asc20.From
	list.Exchange = asc20.To
	list.Tick = token.Tick
	list.Amount = asc20.Amount

	list.CreateList()
	if list.ID == 0 {
		return -47, errors.New("failed to save list")
	}

	if mq.IsEnable() {
		mq.SendListMessage(list)
	}

	token.Txs++
	return 1, err
}

func exchangeToken(list *model.List, sendTo string) (int8, error) {
	// add balance
	err := addBalance(sendTo, list.Tick, list.Amount)
	if err != nil {
		return 0, err
	}

	// update token
	lowerTick := strings.ToLower(list.Tick)

	token := model.GetTokenByTick(lowerTick)
	if token.ID == 0 {
		return -33, nil
	}
	token.Txs++

	list.Remove()

	return 1, err
}

func _transferToken(asc20 *model.Msc20) (int8, error) {
	tick := asc20.Tick

	token := model.GetTokenByTick(tick)
	if token.ID == 0 {
		return -33, nil

	}
	asc20.Tick = tick

	if asc20.Amount <= 0 {
		return -34, nil
	}

	if asc20.From == "" || asc20.To == "" {
		return -35, nil
	}
	if asc20.From == asc20.To {
		// send to self
		return -36, nil
	}

	// From
	err := subBalance(asc20.From, tick, asc20.Amount)
	if err != nil {
		if err.Error() == "insufficient balance" {
			return -37, nil
		}
		return 0, err
	}

	// To
	err = addBalance(asc20.To, tick, asc20.Amount)
	if err != nil {
		return 0, err
	}

	token.Txs++

	return 1, nil
}

func subBalance(owner string, tick string, amount uint64) error {
	holder := model.GetHolder(owner, tick)
	if holder.Amount < amount {
		return errors.New("insufficient balance")
	}
	holder.Amount -= amount
	holder.SavaHolder()
	if holder.ID == 0 {
		return errors.New("failed to save holder")
	}
	return nil
}

func addBalance(owner string, tick string, amount uint64) error {
	holder := model.GetHolder(owner, tick)
	holder.Amount += amount
	holder.Tick = tick
	holder.Address = owner
	holder.SavaHolder()
	if holder.ID == 0 {
		return errors.New("failed to save holder")
	}
	return nil
}
