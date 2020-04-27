package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/howeyc/gopass"
	"github.com/skip2/go-qrcode"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
)

var full *keypair.Full

func prompt(s string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(s)
	text, _ := reader.ReadString('\n')
	return strings.Trim(text, "\r\n ")
}

func accountInfo() (horizon.Account, error) {
	client := horizonclient.DefaultPublicNetClient

	accountRequest := horizonclient.AccountRequest{AccountID: full.Address()}
	return client.AccountDetail(accountRequest)
}

func printWalletInfo() {
	ai, err := accountInfo()
	if err != nil {
		fmt.Println("Error obtaining wallet info")
		return
	}
	fmt.Printf("Wallet address: %s\n", full.Address())
	fmt.Printf("Balances:\n")
	for _, balance := range ai.Balances {
		code := balance.Code
		typ := balance.Type
		if typ == "native" {
			code = "XLM"
		}
		fmt.Printf("%s: %s\n", code, balance.Balance)
	}
}

func printHelp() {
	m := `
Wallet: %s
(h) Help
(s) Send
(r) Receive (Wallet QR)
(i) Wallet info
(spk) Show private key (CAUTION!!!)
(q) Quit

`
	fmt.Printf(m, full.Address())
}

func showQR(s string) error {
	png, err := qrcode.Encode(full.Address(), qrcode.Medium, -4)
	if err != nil {
		return err
	}

	cmd := exec.Command("magick", "display", "-")
	cmd.Stdin = strings.NewReader(string(png))
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func send() error {
	destAddress := prompt("Destination address:")
	if destAddress == "" {
		return fmt.Errorf("Abort")
	}
	_, err := keypair.Parse(destAddress)
	if err != nil {
		return fmt.Errorf("Invalid Stellar address")
	}
	sourceAccount, err := accountInfo()
	if err != nil {
		return fmt.Errorf("Error getting account info")
	}
	amount := prompt("amount:")
	if amount == "" {
		return fmt.Errorf("Abort")
	}
	_, err = strconv.ParseFloat(amount, 64)
	if err != nil {
		return fmt.Errorf("Invalid amount")
	}
	memo := prompt("Text Memo (left blank for none):")
	op := txnbuild.Payment{
		Destination: destAddress,
		Amount:      amount,
		Asset:       txnbuild.NativeAsset{},
	}

	tx := txnbuild.Transaction{
		SourceAccount: &sourceAccount,
		Operations:    []txnbuild.Operation{&op},
		Timebounds:    txnbuild.NewTimeout(3600), // One hour timeout
		Network:       network.PublicNetworkPassphrase,
	}
	if memo != "" {
		tx.Memo = txnbuild.MemoText(memo)
	}

	_, err = tx.BuildSignEncode(full)
	if err != nil {
		return fmt.Errorf("Fail building transaction")
	}
	submit := prompt("Submit transaction? (y/N):")
	if strings.ToLower(submit) != "y" {
		return fmt.Errorf("Abort")
	}
	client := horizonclient.DefaultPublicNetClient
	_, err = client.SubmitTransaction(tx)
	if err != nil {
		return fmt.Errorf("Fail Submit transaction")
	}
	return nil
}

func mainMenu() {

	printHelp()
	for {
		input := prompt(">")
		input = strings.ToLower(input)
		switch input {
		case "":
		case "h":
			printHelp()
		case "s":
			err := send()
			if err != nil {
				fmt.Printf("Send error: %s\n", err.Error())
			} else {
				fmt.Printf("Succesful\n")
			}
		case "i":
			printWalletInfo()
		case "r":
			err := showQR(full.Address())
			if err != nil {
				fmt.Printf("Please install Image Magick display command\n")
			}
		case "spk":
			fmt.Printf("Seed: %s\n", full.Seed())
		case "q":
			return
		default:
			fmt.Println("Invalid command. (h) for help")
		}
	}
}

func main() {

	fmt.Print("Enter pass-phrase: ")
	pass, err := gopass.GetPasswdMasked()
	if err != nil {
		panic(err)
	}
	kp := keypair.Master(string(pass))
	full = kp.(*keypair.Full)

	mainMenu()
}
