package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-redis/redis"
)

var ctx = context.Background()

type Event struct {
	ContractEvent *ContractEvent
	ContractAbi   abi.ABI
}

// ContractEvent maps an event data in a struct
type ContractEvent struct {
	Name  string
	Count *big.Int
}

type TransferEvent struct {
	From   common.Address
	To     common.Address
	Tokens *big.Int
}

// checkEventLog check if all events emitted are in track in Mongo. If no, do a routine to update and
// forward non tracked events
func (e *Event) checkEventLog(logs []types.Log) {
	for i := 0; i < len(logs); i++ {
		e.forwardEvents(logs[i])
	}
	return
}

// forwaredEvents will forward the emitted events as they arrive
func (e *Event) forwardEvents(log types.Log) {

	e.checkLock(log)
	conf.BlockNumber = new(big.Int).SetUint64(log.BlockNumber)
	var buf bytes.Buffer
	cfg := toml.NewEncoder(&buf)
	cfg.Encode(conf)
	ioutil.WriteFile("./config.toml", buf.Bytes(), 0644)
}

//getContractAbi returs the contract ABI
func (e *Event) getContractAbi() {
	abiPath, _ := filepath.Abs(conf.AbiPath)
	file, err := ioutil.ReadFile(abiPath)
	if err != nil {
		fmt.Println("Failed to read file:", err)
	}
	e.ContractAbi, err = abi.JSON(strings.NewReader(string(file)))
	if err != nil {
		fmt.Println("Invalid abi:", err)
	}
}

func (e *Event) checkLock(log types.Log) error {
	e.getContractAbi()

	_, err := e.ContractAbi.Unpack(conf.EventName, log.Data)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	address := common.BytesToAddress(log.Topics[1].Bytes())
	rdb.Set(ctx, "TSHP_"+address.String(), 1, 0)
	rdb.Close()
	return nil
}
