package fetcher

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"

	"github.com/mitchellh/mapstructure"
)

func StructToContracts(input interface{}) map[string]string {
	var contracts map[string]string
	mapstructure.Decode(input, &contracts)
	return contracts
}

func Fetch(etherscanKey string, contracts map[string]string, outputDir string) {
	// clean output directory
	err := os.RemoveAll(outputDir)
	if err != nil {
		log.Fatal(err)
	}

	// create and reuse http client
	client := &http.Client{}

	// create wait group in order to wait for all fetches to finish
	var wg sync.WaitGroup
	wg.Add(len(contracts))

	// fetch all contracts in parallel
	for name, address := range contracts {
		go func(name, address string) {
			fetchContract(etherscanKey, name, address, outputDir, client)
			wg.Done()
		}(name, address)
	}

	// wait for every contract to be fetched
	wg.Wait()
}

func fetchContract(etherscanKey, name, address, outputDir string, client *http.Client) {
	abi, err := fetchABI(etherscanKey, address, client)
	if err != nil {
		log.Fatal(err)
	}

	lowercaseName := strings.ToLower(name)
	abiDir := path.Join(outputDir, lowercaseName)
	abiFileName := lowercaseName + ".json"

	ensureDirAndWriteToFile(abiDir, abiFileName, abi)

	cmd := exec.Command(
		"abigen",
		"--abi="+path.Join(abiDir, abiFileName),
		"--pkg="+lowercaseName,
		"--out="+path.Join(abiDir, lowercaseName+".go"),
	)
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Fetched %v abi from %v\n", name, address)
}

func fetchABI(etherscanKey, address string, client *http.Client) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.etherscan.io/api", nil)
	if err != nil {
		log.Fatal(err)
	}

	// appending to existing query args
	q := req.URL.Query()
	q.Add("module", "contract")
	q.Add("action", "getabi")
	q.Add("address", address)
	q.Add("apikey", etherscanKey)

	// assign encoded query string to http request
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Errored when sending request to the server")
		return "", err
	}

	defer resp.Body.Close()

	var response struct {
		Result string `json:"result"`
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Fatal(err)
	}

	return response.Result, nil
}

func ensureDirAndWriteToFile(dir, fileName, data string) {
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Create(path.Join(dir, fileName))
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	_, err = file.WriteString(data)
	if err != nil {
		log.Fatal(err)
	}
}
