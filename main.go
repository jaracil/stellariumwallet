package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
(s) Sign transaction
(i) Wallet info
(qr) Show account public address QR
(qr_secret_key) Show account private key QR
(print_secret_key) Print account private key

`
	fmt.Printf(m, full.Address())
}

func showQR(s string) error {
	png, err := qrcode.Encode(s, qrcode.Medium, -4)
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

func sign() error {
	raw := prompt("Enter raw transaction:")
	if raw == "" {
		return fmt.Errorf("Abort")
	}
	txn, err := txnbuild.TransactionFromXDR(raw)
	if err != nil {
		return fmt.Errorf("Invalid transaction")
	}

	if txn.SourceAccount.GetAccountID() != full.Address() {
		fmt.Printf("%s != %s\n", txn.SourceAccount.GetAccountID(), full.Address())
		return fmt.Errorf("Source address don't match")
	}
	txn.Network = network.PublicNetworkPassphrase
	err = txn.Sign(full)
	if err != nil {
		return fmt.Errorf("Fail signing transaction: %v", err)
	}
	txnStr, err := txn.Base64()
	if err != nil {
		return fmt.Errorf("Fail encoding transaction")
	}
	r := prompt("(Submit/Print/QR) signed transaction (s/p/q)?")
	switch strings.ToLower(r) {
	case "s":
		client := horizonclient.DefaultPublicNetClient
		_, err = client.SubmitTransaction(txn)
		if err != nil {
			return fmt.Errorf("Fail Submit transaction")
		}
	case "p":
		fmt.Println("======= START SIGNED TRANSACTION ========")
		fmt.Println(txnStr)
		fmt.Println("======== END SIGNED TRANSACTION =========")
	case "q":
		showQR(txnStr)
	default:
		return fmt.Errorf("Abort")
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
			err := sign()
			if err != nil {
				fmt.Printf("Sign error: %s\n", err.Error())
			}
		case "i":
			printWalletInfo()
		case "qr":
			err := showQR(full.Address())
			if err != nil {
				fmt.Printf("Please install Image Magick\n")
			}
		case "qr_private_key":
			err := showQR(full.Seed())
			if err != nil {
				fmt.Printf("Please install Image Magick\n")
			}
		case "print_private_key":
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
