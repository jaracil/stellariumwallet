package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
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

func accountInfo(address string) (horizon.Account, error) {
	client := horizonclient.DefaultPublicNetClient

	if address == "" {
		address = full.Address()
	}

	accountRequest := horizonclient.AccountRequest{AccountID: address}
	return client.AccountDetail(accountRequest)
}

func printWalletInfo() {
	ai, err := accountInfo("")
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
		fmt.Printf("asset: %-12s balance: %s\n", code, balance.Balance)
	}
}

func pay() {
	ai, err := accountInfo("")
	if err != nil {
		fmt.Println("Error obtaining wallet info")
		return
	}

	var assets []*horizon.Asset
	var balances []string

	for _, balance := range ai.Balances {
		asset := &horizon.Asset{Code: balance.Code, Type: balance.Type, Issuer: balance.Issuer}
		assets = append(assets, asset)
		balances = append(balances, balance.Balance)
	}

	for i := range assets {
		assetCode := assets[i].Code
		if assets[i].Type == "native" {
			assetCode = "XLM"
		}
		fmt.Printf("%d - asset: %-12s balance: %s\n", i+1, assetCode, balances[i])
	}
	idx := prompt(fmt.Sprintf("Asset [1 - %d]?", len(assets)))
	if idx == "" {
		fmt.Println("Pay exit")
		return
	}
	assetIndex, err := strconv.Atoi(idx)
	if err != nil || (assetIndex < 1 || assetIndex > len(assets)) {
		fmt.Println("Invalid asset index")
		return
	}
	assetIndex--
	assetCode := assets[assetIndex].Code
	if assets[assetIndex].Type == "native" {
		assetCode = "XLM"
	}

	ammount := prompt(fmt.Sprintf("%s ammount?", assetCode))
	if ammount == "" {
		fmt.Println("Pay exit")
		return
	}
	assetAmmount, err := strconv.ParseFloat(ammount, 64)
	if err != nil || (assetIndex < 1 || assetIndex > len(assets)) {
		fmt.Println("Invalid ammount")
		return
	}

	memo := prompt("Memo?")

	destinationAddress := prompt("Destination address?")
	if destinationAddress == "" {
		fmt.Println("Pay exit")
		return
	}

	_, err = keypair.ParseAddress(destinationAddress)
	if err != nil {
		fmt.Println("Invalid destination address")
		return
	}

	createAccount := false

	destAccountInfo, err := accountInfo(destinationAddress)
	if err != nil {
		hce := err.(*horizonclient.Error)
		if hce.Problem.Status == 404 {
			createAccount = true
		} else {
			fmt.Printf("Invalid destination address, error: %v\n", err)
			return
		}
	}

	fmt.Printf("Destination account info:%v\ncreate:%v\n", destAccountInfo, createAccount)

	fmt.Printf("Asset ammount: %f Memo:%s Address:%s\n", assetAmmount, memo, destinationAddress)
}

func printHelp() {
	m := `
Wallet: %s
(h) Help
(p) Pay
(s) Sign transaction
(i) Wallet info
(qr) Show account public address QR
(qr_private_key) Show account private key QR
(print_private_key) Print account private key
(q) quit

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

func asset2str(asset txnbuild.Asset) string {
	code := asset.GetCode()
	if code == "" {
		code = "XLM"
	}
	return code
}

func op2str(op txnbuild.Operation) string {

	switch v := op.(type) {
	case *txnbuild.Payment:
		return fmt.Sprintf("Send %s %s to %s", v.Amount, asset2str(v.Asset), v.Destination)
	case *txnbuild.CreateAccount:
		return fmt.Sprintf("Create account %s and fund with %s XLM", v.Destination, v.Amount)
	case *txnbuild.ChangeTrust:
		limit, _ := strconv.ParseFloat(v.Limit, 64)
		if limit == 0 {
			return fmt.Sprintf("Remove trust line %s issuer: %s", asset2str(v.Line), v.Line.GetIssuer())
		}
		return fmt.Sprintf("Change trust line %s issuer: %s limit: %s", asset2str(v.Line), v.Line.GetIssuer(), v.Limit)

	case *txnbuild.ManageBuyOffer:
		return fmt.Sprintf("Manage buy offer %s %s @ %s %s/%s", v.Amount, asset2str(v.Buying), v.Price, asset2str(v.Selling), asset2str(v.Buying))
	case *txnbuild.ManageSellOffer:
		return fmt.Sprintf("Manage sell offer %s %s @ %s %s/%s", v.Amount, asset2str(v.Selling), v.Price, asset2str(v.Buying), asset2str(v.Selling))
	case *txnbuild.CreatePassiveSellOffer:
		return fmt.Sprintf("Create passive sell offer %s %s @ %s %s/%s", v.Amount, asset2str(v.Selling), v.Price, asset2str(v.Buying), asset2str(v.Selling))
	case *txnbuild.PathPaymentStrictReceive:
		return fmt.Sprintf("Path payment strict receive: %+v", v)
	case *txnbuild.PathPaymentStrictSend:
		return fmt.Sprintf("Path payment strict send: %+v", v)
	case *txnbuild.AllowTrust:
		return fmt.Sprintf("Allow trust to %s asset: %s authoroize: %t", v.Trustor, asset2str(v.Type), v.Authorize)
	case *txnbuild.AccountMerge:
		return fmt.Sprintf("Account merge. Destination account: %s", v.Destination)
	case *txnbuild.ManageData:
		return fmt.Sprintf("Manage data Name: %s Value: %s", strconv.Quote(v.Name), strconv.Quote(string(v.Value)))
	case *txnbuild.BumpSequence:
		return fmt.Sprintf("Bump sequence to %d", v.BumpTo)
	default:
		return fmt.Sprintf("Unknown operation: %v", v)

	}
}

func memo2str(memo txnbuild.Memo) string {
	switch v := memo.(type) {
	case txnbuild.MemoText:
		return fmt.Sprintf("Text: %s", v)
	case txnbuild.MemoID:
		return fmt.Sprintf("ID: %d", v)
	case txnbuild.MemoHash:
		return fmt.Sprintf("HASH: %s", hex.EncodeToString(v[:]))
	case txnbuild.MemoReturn:
		return fmt.Sprintf("RETURN: %s", hex.EncodeToString(v[:]))
	}
	return ""
}

func sign() error {
	raw := prompt("Enter raw transaction:")
	fmt.Printf("\n")
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

	memo := memo2str(txn.Memo)
	if memo != "" {
		fmt.Printf("Transaction memo %s\n", memo)
	}

	fmt.Printf("Operations in transaction:\n")
	for i, op := range txn.Operations {
		fmt.Printf("(%d) %s\n", i+1, op2str(op))
	}
	fmt.Printf("\n")

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
		case "p":
			pay()
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
	var err error

	fmt.Print("Enter pass-phrase: ")
	pass, err := gopass.GetPasswdMasked()
	if err != nil {
		panic(err)
	}

	full, err = keypair.ParseFull(string(pass))
	if err != nil {
		full = keypair.Master(string(pass)).(*keypair.Full)
	}

	mainMenu()
}
