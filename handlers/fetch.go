package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"indexer/config"
	"indexer/model"
	"indexer/model/fetch"
	"indexer/utils"
	"strconv"
	"sync"
	"time"
)

var req *resty.Client
var fetchUrl string

var fetchInterrupt bool

// rpc中最新的区块号
var lastBlockNumber uint64

// 当前同步的区块号
var curSyncBlock uint64
var confirmBlockHeight uint64

func initFetch() {
	// 配置网络代理
	proxy := config.Cfg.Section("data-source").Key("proxy").MustString("")
	req = resty.New().SetTimeout(3 * time.Second)
	if proxy != "" {
		req = req.SetProxy(proxy)
	}
	synCfg := config.Cfg.Section("fetch")
	confirmBlockHeight = synCfg.Key("confirm").MustUint64(0)

	// 1. 配置起始区块号
	// 1.1 从配置文件获取起始区块号

	startBlock := synCfg.Key("start").MustUint64(0)
	// 1.2 从数据库获取保存的最新区块号
	latestTransaction := model.GetLatestTransaction()
	if latestTransaction.ID != 0 {
		startBlock = latestTransaction.Block
	}
	// 1.3 从rpc获取最新区块号
	var err error
	lastBlockNumber, err = fetchLastBlockNumber()
	if err != nil {
		panic("can't get latest block from rpc")
	}

	if startBlock > lastBlockNumber {
		panic(fmt.Sprintf("start block greater than last block number: %d > %d", startBlock, lastBlockNumber))
	}

	curSyncBlock = startBlock
	if curSyncBlock < 0 {
		panic("curSyncBlock num is error")
	}
}

func StartFetch() {
	fetchInterrupt = false
	for !fetchInterrupt {
		var txsResp fetch.BlockResponse
		var logsResp fetch.LogsResponse
		err := fetchData(curSyncBlock, &txsResp, &logsResp)
		if err != nil { // 已经拉取到最新区块了
			logger.Println("fetch error:", err.Error())
			time.Sleep(time.Duration(1) * time.Second)
		} else {
			// 开始事务
			tx := model.DB.Begin()
			err = saveData(&txsResp, &logsResp)

			if err != nil {
				// 回滚事务
				tx.Rollback()
				fmt.Printf("fetch: save error:%+v", err)
				QuitChan <- true
				break
			}
			// 提交事务
			tx.Commit()

			// 开始下一个区块
			curSyncBlock++
		}
	}

	logger.Println("fetch stopped")
}

func StopFetch() {
	fetchInterrupt = true
}

func fetchData(curSyncBlock uint64, blockResp *fetch.BlockResponse, logsResp *fetch.LogsResponse) error {
	if curSyncBlock > lastBlockNumber {
		lastBlock, err := fetchLastBlockNumber()
		if err != nil {
			return err
		}
		lastBlockNumber = lastBlock
	}

	if curSyncBlock > lastBlockNumber {
		errStr := fmt.Sprintf("no new blocks to be fetched, curSyncBlock: %d, lastBlockNumber %d", curSyncBlock, lastBlockNumber)
		return errors.New(errStr)
	}

	var wg sync.WaitGroup
	var err0 error
	var err1 error

	wg.Add(2)
	go func() {
		err0 = fetchTransactions(curSyncBlock, blockResp)
		wg.Done()
	}()
	go func() {
		err1 = fetchContractLogs(curSyncBlock, logsResp)
		wg.Done()
	}()

	wg.Wait()

	if err0 != nil {
		return err0
	}
	if err1 != nil {
		return err1
	}
	logger.Info("fetch data at #", curSyncBlock)
	return nil
}

func fetchLastBlockNumber() (uint64, error) {
	reqJson := fmt.Sprintf(`{"id": "indexer","jsonrpc": "2.0","method": "eth_blockNumber","params": []}`)
	resp, rerr := req.R().EnableTrace().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(reqJson).
		Post(fetchUrl)
	if rerr != nil {
		logger.Info("fetch url error:", rerr)
		return 0, rerr
	}

	var response fetch.NumberResponse
	uerr := json.Unmarshal(resp.Body(), &response)
	if uerr != nil {
		logger.Info("json parse error: ", uerr)
		fmt.Println(string(resp.Body()))
		return 0, rerr
	}
	if response.Error.Code != 0 && response.Error.Message != "" {
		return 0, errors.New(fmt.Sprintf("fetch error code: %d, msg: %s", response.Error.Code, response.Error.Message))
	}
	if response.Id != "indexer" || response.JsonRpc != "2.0" {
		return 0, errors.New("fetch error data")
	}

	blockNumber := utils.HexToUint64(response.Result) - confirmBlockHeight

	return blockNumber, nil
}

func fetchTransactions(blockNumber uint64, response *fetch.BlockResponse) (err error) {
	block := strconv.FormatUint(blockNumber, 16)
	reqJson := fmt.Sprintf(`{"id": "indexer","jsonrpc": "2.0","method": "eth_getBlockByNumber","params": ["0x%s", %t]}`, block, true)
	resp, rerr := req.R().EnableTrace().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(reqJson).Post(fetchUrl)
	if rerr != nil {
		logger.Info("fetch url error:", rerr)
		err = rerr
		return
	}

	uerr := json.Unmarshal(resp.Body(), &response)
	if uerr != nil {
		logger.Info("json parse error: ", uerr)
		fmt.Println(string(resp.Body()))
		err = uerr
		return
	}
	if response.Error.Code != 0 && response.Error.Message != "" {
		err = errors.New(fmt.Sprintf("fetch error code: %d, msg: %s", response.Error.Code, response.Error.Message))
		return
	}
	if response.Id != "indexer" || response.JsonRpc != "2.0" || response.Result == nil {
		err = errors.New("fetch error data")
		return
	}

	return
}

func fetchContractLogs(blockNumber uint64, response *fetch.LogsResponse) (err error) {
	block := strconv.FormatUint(blockNumber, 16)
	reqJson := fmt.Sprintf(`{"id": "indexer","jsonrpc": "2.0","method": "eth_getLogs","params": [{"fromBlock": "0x%s","toBlock": "0x%s"}]}`, block, block)
	resp, rerr := req.R().EnableTrace().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(reqJson).
		Post(fetchUrl)
	if rerr != nil {
		logger.Info("fetch url error:", rerr)
		err = rerr
		return
	}

	uerr := json.Unmarshal(resp.Body(), &response)
	if uerr != nil {
		logger.Info("json parse error: ", uerr)
		fmt.Println(string(resp.Body()))
		err = uerr
		return
	}
	if response.Error.Code != 0 && response.Error.Message != "" {
		err = errors.New(fmt.Sprintf("fetch error code: %d, msg: %s", response.Error.Code, response.Error.Message))
		return
	}
	if response.Id != "indexer" || response.JsonRpc != "2.0" {
		err = errors.New("fetch error data")
		return
	}

	return
}

func saveData(blockResp *fetch.BlockResponse, logsResp *fetch.LogsResponse) error {
	var err error

	var blockNumber = utils.HexToUint64(blockResp.Result.Number)
	var timestamp = utils.HexToUint64(blockResp.Result.Timestamp)

	// save txs
	txs := make([]*model.Transaction, len(blockResp.Result.Transactions))
	for ti := range blockResp.Result.Transactions {
		_tx := blockResp.Result.Transactions[ti]
		tx := &model.Transaction{
			Hash:      _tx.Hash,
			From:      _tx.From,
			To:        _tx.To,
			Block:     blockNumber,
			Idx:       utils.HexToUint32(_tx.TransactionIndex),
			Timestamp: timestamp,
			Input:     _tx.Input,
		}
		txs[ti] = tx
	}

	// save logs
	logs := make([]*model.EvmLog, len(logsResp.Result))
	for li := range logsResp.Result {
		_log := logsResp.Result[li]
		log := &model.EvmLog{
			Hash:      _log.TransactionHash,
			Address:   _log.Address,
			Topics:    _log.Topics,
			Data:      _log.Data,
			Block:     blockNumber,
			TxIndex:   utils.HexToUint32(_log.TransactionIndex),
			LogIndex:  utils.HexToUint32(_log.LogIndex),
			Timestamp: timestamp,
		}
		logs[li] = log
	}

	// 对 log 和 tx 进行排序并合并
	records := mixRecords(txs, logs)

	// 生成 inscription
	err = processRecords(records)
	if err != nil {
		return err
	}

	// save tx and log to db
	err = model.CreateBatchesTransaction(txs, len(txs))
	if err != nil {
		return err
	}
	err = model.CreateBatchesEvmLog(logs, len(logs))
	if err != nil {
		return err
	}

	return nil
}
