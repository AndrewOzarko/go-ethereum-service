package main

import (
	"context"
	"log"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/andrewozarko/go-ethereum-service/app/uniswap"
	"github.com/cheggaaa/pb"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"github.com/zenthangplus/goccm"
	"go.uber.org/zap"
)

const factoryAddress = "0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f"

var client = &ethclient.Client{}
var logger = &zap.SugaredLogger{}

type Pair struct {
	ID      int64
	Address string
}

var pairs []Pair
var unswp uniswap.Uniswap

func main() {
	l, _ := zap.NewProduction()
	logger = l.Sugar()

	if err := godotenv.Load(); err != nil {
		logger.Fatal("No .env")
	}

	blockchainUrl, exists := os.LookupEnv("BLOCKCHAIN_URL")
	log.Println(blockchainUrl)

	if !exists {
		logger.Fatal("need blockchain url")
	}

	client, err := ethclient.Dial(blockchainUrl)

	if err != nil {
		logger.Fatal(err)
	}

	defer client.Close()

	address := common.HexToAddress(factoryAddress)

	unswp, err := uniswap.NewUniswap(address, client)

	if err != nil {
		logger.Fatal(err)
	}

	pairsLength, err := unswp.AllPairsLength(&bind.CallOpts{})

	if err != nil {
		logger.Fatal(err)
	}

	log.Printf("found %d, pairs:", pairsLength)

	var mu sync.Mutex

	c := goccm.New(250)

	bar := pb.StartNew(int(pairsLength.Int64()))
	for i := int64(1); i <= int64(pairsLength.Int64()); i++ {
		c.Wait()

		go func(i int64) {
			start := time.Now()
			// fmt.Printf("Job %d is running\n", i)
			c.Done()
			addr, err := unswp.AllPairs(&bind.CallOpts{}, big.NewInt(i))
			if err != nil {
				logger.Warn(err)
				return
			}
			mu.Lock()
			pairs = append(pairs, Pair{
				ID:      i,
				Address: addr.String(),
			})
			mu.Unlock()
			elapsed := time.Since(start)
			log.Printf("Binomial took %s", elapsed)

		}(i)
	}
	bar.Finish()
	log.Printf("found %d, pairs:", pairsLength)

	c.WaitAllDone()

	var paddress = []common.Address{}

	for _, k := range pairs {
		paddress = append(paddress, common.HexToAddress(string(k.Address)))
	}

	query := ethereum.FilterQuery{
		Addresses: paddress,
	}
	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		logger.Warn(err)
		return
	}

	for {
		select {
		case err := <-sub.Err():
			logger.Warn(err)
		case vLog := <-logs:
			a := vLog.Address.String()

			log.Printf("liquidity chainges: %s", a)
			log.Println("")
		}
	}
	// log.Println("Finished")

}
