package main

import (
	"bufio"
	"easywallet"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/shopspring/decimal"
)

// Interactive mode command indexes
const (
	GetBalanceCommandIndex      int = 1
	SendTransactionCommandIndex     = 2
)

// Command names
const (
	balanceCmd         = "balance"
	sendTransactionCmd = "sendtx"
)

func checkFileExists(filePath string) bool {
	_, err := os.Stat(filePath) // Attempt to get file information
	if err == nil {
		return true // File exists, no error
	}
	if errors.Is(err, os.ErrNotExist) {
		return false // File does not exist
	}
	return false
}

func main() {
	fmt.Println("Easy Wallet v 1.0.9")

	config, err := easywallet.ReadConfig("config.yaml")

	if err != nil {
		return
	}

	modeCmdParam := flag.String("mode", "", "Mode")
	commandCmdParam := flag.String("command", "", "Coin")
	coinCmdParam := flag.String("coin", "", "Coin")
	recipientAddressCmdParam := flag.String("address", "", "Address to send")
	amountCmdParam := flag.String("amount", "", "Amount")
	seedEncryptionPasswordParam := flag.String("password", "", "Seed encryption password")

	flag.Parse()

	switch *modeCmdParam {
	case "create":
		createSeed()
	case "i":
		runInteractive(config)
	default:
		runParametrized(config,
			*seedEncryptionPasswordParam,
			*commandCmdParam,
			*coinCmdParam,
			*recipientAddressCmdParam,
			*amountCmdParam)
	}

}

func createSeed() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter master seed: ")
	mnemonic, _ := reader.ReadString('\n')
	mnemonic = strings.TrimSpace(mnemonic)

	fmt.Print("Enter seed encryption password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	if len(password) == 0 {
		fmt.Println("Warning! Seed is not encrypted!")
	}
	err := easywallet.GenerateAndSaveMasterSeed(mnemonic, password)
	if err != nil {
		fmt.Println(err)
	}
}
func runInteractive(config *easywallet.Config) {

	seedStorage, err := easywallet.ReadSeedStorage()

	if err != nil {
		fmt.Println(err)
		return
	}

	seedEncryptionPassword := ""
	if seedStorage.IsEncrypted {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter seed encryption password: ")
		seedEncryptionPassword, err = reader.ReadString('\n')

		if err != nil {
			fmt.Println(err)
			return
		}
		seedEncryptionPassword = strings.TrimSpace(seedEncryptionPassword)
	}

	ew, err := easywallet.NewMultiWallet(config, seedEncryptionPassword)

	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		fmt.Print("Enter command: \n" +
			"1: get balance\n" +
			"2: send transaction\n" +
			"3: exit\n")

		var commandIndex int
		_, err := fmt.Scan(&commandIndex)
		if err != nil {
			return
		}

		switch commandIndex {
		case GetBalanceCommandIndex:
			{
				if getBalanceCommand(ew) {
					return
				}
			}
		case SendTransactionCommandIndex:
			{
				if sendTransactionCommand(ew) {
					return
				}
			}
		default:
			return
		}
	}
}

func getBalanceCommand(ew *easywallet.MultiWallet) bool {
	fmt.Println("Enter coin:")
	var coin string
	_, err := fmt.Scan(&coin)
	if err != nil {
		return true
	}
	balance(ew, coin)
	return false
}

func sendTransactionCommand(ew *easywallet.MultiWallet) bool {
	fmt.Println("Enter coin:")
	var coin string
	_, err := fmt.Scan(&coin)
	if err != nil {
		return true
	}

	fmt.Println("Enter recipient address:")
	var recipientAddress string
	_, err = fmt.Scan(&recipientAddress)
	if err != nil {
		return true
	}

	fmt.Println("Enter amount:")
	var amount string
	_, err = fmt.Scan(&amount)
	if err != nil {
		return true
	}

	sendTransaction(ew, coin, recipientAddress, amount)
	return false
}

func runParametrized(config *easywallet.Config,
	seedEncryptionPasswordParam string,
	commandCmdParam string,
	coinCmdParam string,
	recipientAddressCmdParam string,
	amountCmdParam string) {

	if coinCmdParam == "" {
		fmt.Println("coin is required")
		return
	}

	ew, err := easywallet.NewMultiWallet(config, seedEncryptionPasswordParam)

	if err != nil {
		fmt.Println(err)
		return
	}

	switch commandCmdParam {
	case balanceCmd:
		balance(ew, coinCmdParam)
	case sendTransactionCmd:
		sendTransaction(ew, coinCmdParam, recipientAddressCmdParam, amountCmdParam)
	default:
		fmt.Println("Unknown commandCmdParam")
	}
}

func sendTransaction(ew *easywallet.MultiWallet, coin string, recipientAddress string, amount string) {
	curAddress, _ := ew.GetAddress(coin)
	fmt.Println("Current address: ", curAddress)
	balance, _ := ew.GetBalance(coin)
	fmt.Println("Balance: ", balance.String())
	fmt.Println("Coin: ", coin)
	fmt.Println("Address: ", recipientAddress)

	da, err := decimal.NewFromString(amount)
	if err != nil {
		fmt.Println(err)
		return
	}

	tx, err := ew.Send(coin, recipientAddress, da)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("Tx: ", tx)
	}
}

func balance(ew *easywallet.MultiWallet, coin string) {
	fmt.Println("Coin: ", coin)
	curAddress, _ := ew.GetAddress(coin)
	fmt.Println("Current address: ", curAddress)
	balance, _ := ew.GetBalance(coin)
	fmt.Println("Balance: ", balance.String())
}
