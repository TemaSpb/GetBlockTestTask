package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type blockTransactions struct {
	id           int
	transactions *interface{}
}

const (
	url         = "https://eth.getblock.io/mainnet/"
	apiKey      = "1b2c2094-787d-425c-9422-bd653957fddb"
	concurrency = 3
)

func main() {
	depthCount := flag.Int("depthCount", 3, "depthCount")
	windowSize := flag.Int("windowSize", 100, "windowSize")
	flag.Parse()

	*depthCount--
	totalCount := *depthCount + *windowSize
	transactionsList := make(chan blockTransactions)
	var initialNumber uint64
	go func(initialNumber *uint64) {
		value := requestTransactions(transactionsList, totalCount)
		*initialNumber = value
		close(transactionsList)
	}(&initialNumber)

	transactionsResult := make([]*big.Int, totalCount)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for transactions := range transactionsList {
				calculateTransactionSum(transactions, transactionsResult)
			}
			wg.Done()
		}()
	}

	wg.Wait()
	calculateMax(transactionsResult, *windowSize, *depthCount, initialNumber)
}

func calculateTransactionSum(transactions blockTransactions, transactionsResult []*big.Int) {
	transactionList := (*transactions.transactions).([]interface{})
	sum := big.NewInt(0)
	for _, transaction := range transactionList {
		val := transaction.(map[string]interface{})["value"]
		n := new(big.Int)
		n.SetString(val.(string), 0)
		sum.Add(sum, n)
	}
	transactionsResult[transactions.id] = sum
}

func calculateMax(transactionsResult []*big.Int, windowSize, depthCount int, initialNumber uint64) {
	sum := big.NewInt(0)
	max := big.NewInt(0)
	maxIndex := 0
	for i := 0; i < windowSize+depthCount; i++ {
		sum.Add(sum, transactionsResult[i])
		if i >= windowSize {
			sum.Sub(sum, transactionsResult[i-windowSize])
		}
		if i >= windowSize-1 {
			if sum.Cmp(max) >= 0 {
				max.Set(sum)
				maxIndex = i - windowSize + 1
			}
		}
	}
	fmt.Printf("Block number: 0x%x\nTotal transactions value:%d", initialNumber-uint64(maxIndex), max)
}

func makePostRequest(client *http.Client, body map[string]interface{}) *http.Response {
	marshaledBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(marshaledBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	resp, _ := client.Do(req)
	return resp
}

func hexStrToInt(s string) uint64 {
	numberStr := strings.Replace(s, "0x", "", -1)
	n, err := strconv.ParseUint(numberStr, 16, 64)
	if err != nil {
		panic(err)
	}
	return n
}

func getBodyTemplate(methodName string) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  methodName,
		"id":      "getblock.io",
	}
}

func requestTransactions(transactions chan blockTransactions, totalCount int) uint64 {
	client := &http.Client{}
	body := getBodyTemplate("eth_blockNumber")
	body["params"] = nil
	var headBlock map[string]string
	resp := makePostRequest(client, body)
	err := json.NewDecoder(resp.Body).Decode(&headBlock)
	if err != nil {
		panic(err)
	}
	initialNumber := hexStrToInt(headBlock["result"])

	bodyTemplate := getBodyTemplate("eth_getBlockByNumber")
	var responseBlock map[string]interface{}
	for i := 0; i < totalCount; i++ {
		bodyTemplate["params"] = []interface{}{fmt.Sprintf("0x%x", initialNumber-uint64(i)), true}
		resp := makePostRequest(client, bodyTemplate)
		err := json.NewDecoder(resp.Body).Decode(&responseBlock)
		if err != nil {
			panic(err)
		}
		responseTransactions := responseBlock["result"].(map[string]interface{})["transactions"]
		transactions <- blockTransactions{
			id:           i,
			transactions: &responseTransactions,
		}
	}
	return initialNumber
}
