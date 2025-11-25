package main

import (
	"easywallet"
	"errors"
	"flag"
	"fmt"
	"os"
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

	fmt.Println("Easy Wallet v 1.0.6")

	config, err := easywallet.ReadConfig("config.yaml")

	if err != nil {
		return
	}

	modeCmdParam := flag.String("mode", "", "Mode")
	commandCmdParam := flag.String("command", "", "Coin")
	coinCmdParam := flag.String("coin", "", "Coin")
	recipientAddressCmdParam := flag.String("address", "", "Address to send")
	amountCmdParam := flag.Float64("amount", 0.0, "Amount")

	flag.Parse()

	ew, err := easywallet.NewMultiWallet(config, os.Getenv("MNEMONIC"))

	if err != nil {
		fmt.Println(err)
		return
	}

	switch *modeCmdParam {
	case "i":
		runInteractive(ew)
	default:
		runParametrized(*commandCmdParam, ew, *coinCmdParam, *recipientAddressCmdParam, *amountCmdParam)
	}

}

func runInteractive(ew *easywallet.MultiWallet) {
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
	var amount float64
	_, err = fmt.Scan(&amount)
	if err != nil {
		return true
	}

	sendTransaction(ew, coin, recipientAddress, amount)
	return false
}

func runParametrized(commandCmdParam string, ew *easywallet.MultiWallet, coinCmdParam string, recipientAddressCmdParam string, amountCmdParam float64) {

	if coinCmdParam == "" {
		fmt.Println("coin is required")
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

func sendTransaction(ew *easywallet.MultiWallet, coin string, recipientAddress string, amount float64) {
	curAddress, _ := ew.GetAddress(coin)
	fmt.Println("Current address: ", curAddress)
	balance, _ := ew.GetBalance(coin)
	fmt.Println("Balance: ", balance.String())
	fmt.Println("Coin: ", coin)
	fmt.Println("Address: ", recipientAddress)

	tx, err := ew.Send(coin, recipientAddress, amount)
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
