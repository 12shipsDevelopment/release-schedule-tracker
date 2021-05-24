package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-redis/redis"
	"strings"
)

// Provider has the variables needed to communicate with the Ethereum RPC
type Provider struct {
	client          *ethclient.Client
	contractAddress common.Address
}

// setUp is responsible for initializing the needed vars
func (p *Provider) setUp() {
	pClient, err := ethclient.Dial(conf.ProviderURL)
	if err != nil {
		log.Fatal(err)
	}
	p.client = pClient

	if err != nil {
		log.Fatal(err)
	}
	p.contractAddress = common.HexToAddress(conf.ContractAddress) //Contract Address
	if err != nil {
		log.Fatal(err)
	}
}

// listenToEvent is the function responsible for keep listening an event
func (p *Provider) loadEvent() {
	from := conf.BlockNumber
	to := new(big.Int).SetInt64(12339355)
	for {
		if from.Cmp(to) > 0 {
			break
		}
		_to := new(big.Int).Add(from, new(big.Int).SetInt64(1000))
		query := ethereum.FilterQuery{
			Addresses: []common.Address{p.contractAddress},
			FromBlock: new(big.Int).Sub(from, new(big.Int).SetInt64(1)),
			ToBlock:   _to,
		}
		fmt.Printf("from %d to %d\n", from, _to)
		from = _to
		ctx := context.Background()

		var eventCh = make(chan types.Log)

		_, err := p.client.SubscribeFilterLogs(ctx, query, eventCh)
		if err != nil {
			log.Println("Subscribe Failed: ", err)
			return
		}
		// Check events logs
		logs, err := p.client.FilterLogs(ctx, query)
		if err != nil {
			fmt.Println("Filter Logs: ", err)
		}

		event := new(Event)
		if err != nil {
			fmt.Println("Failed to start Mongo session:", err)
		}
		event.checkEventLog(logs)
	}
}

func (p *Provider) fetchLock() {

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	keys, _ := rdb.Keys(ctx, "TSHP_*").Result()
	latest, _ := p.client.BlockNumber(ctx)
	method := "d26c4a76"
	index := "0000000000000000000000000000000000000000000000000000000000000000"
	to := common.HexToAddress(conf.ContractAddress)
	decimal := new(big.Int).Exp(big.NewInt(10), big.NewInt(0), nil)
	now := new(big.Int).SetInt64(time.Now().Unix())
	totalLocked := new(big.Int).SetInt64(0)
	for _, v := range keys {
		addr := common.HexToAddress(strings.Replace(v, "TSHP_", "", 1))
		fmt.Println("checking ", addr.String())
		data, _ := hex.DecodeString(method + strings.Replace(addr.Hash().String(), "0x", "", 1) + index)
		msg := ethereum.CallMsg{
			From:     common.HexToAddress("0x0000000000000000000000000000000000000000"),
			To:       &to,
			Gas:      0,
			GasPrice: new(big.Int).SetInt64(0),
			Value:    new(big.Int).SetInt64(0),
			Data:     data,
		}
		locked, err := p.client.CallContract(ctx, msg, new(big.Int).SetUint64(latest))
		if err != nil {
			rdb.Del(ctx, v)
			continue
		}
		releaseTime := new(big.Int).SetBytes(locked[:32])
		//amount := new(big.Int).Div(new(big.Int).SetBytes(locked[32:64]), decimal)
		remaining := new(big.Int).Div(new(big.Int).SetBytes(locked[64:96]), decimal)
		termOfRound := new(big.Int).SetBytes(locked[96:128])
		amountPerRound := new(big.Int).Div(new(big.Int).SetBytes(locked[128:]), decimal)
		day := new(big.Int).Div(new(big.Int).Sub(now, releaseTime), termOfRound)
		if day.Cmp(new(big.Int).SetInt64(0)) < 0 {
			day = new(big.Int).SetInt64(0)
		}
		unlocked := day.Mul(day, amountPerRound)
		if unlocked.Cmp(remaining) < 0 {
			locked := new(big.Int).Sub(remaining, unlocked)
			totalLocked = new(big.Int).Add(totalLocked, locked)
			fmt.Println("locked", locked)
		} else {
			rdb.Del(ctx, v)
		}
	}
	fmt.Println("totalLocked", totalLocked)
	rdb.Set(ctx, "locked", totalLocked.String(), 88400*time.Second)
}
