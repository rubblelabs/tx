package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/rubblelabs/ripple/crypto"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
	"io/ioutil"
	"os"
	"strings"
)

func checkErr(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func parseSeed(s string) *crypto.RootDeterministicKey {
	seed, err := crypto.NewRippleHashCheck(s, crypto.RIPPLE_FAMILY_SEED)
	checkErr(err)
	key, err := crypto.GenerateRootDeterministicKey(seed.Payload())
	checkErr(err)
	return key
}

func parseAccount(s string) *data.Account {
	account, err := data.NewAccountFromAddress(s)
	checkErr(err)
	return account
}

func parseAmount(s string) *data.Amount {
	amount, err := data.NewAmount(s)
	checkErr(err)
	return amount
}

func parsePaths(s string) *data.PathSet {
	ps := data.PathSet{}
	for _, pathStr := range strings.Split(s, ",") {
		path, err := data.NewPath(pathStr)
		checkErr(err)
		ps = append(ps, path)
	}
	return &ps
}

func sign(c *cli.Context, tx data.Transaction, sequence int32) {
	priv, err := key.GenerateAccountKey(sequence)
	checkErr(err)
	id, err := key.GenerateAccountId(sequence)
	checkErr(err)
	pub, err := priv.PublicAccountKey()
	checkErr(err)
	base := tx.GetBase()
	base.Sequence = uint32(c.GlobalInt("sequence"))
	base.SigningPubKey = new(data.PublicKey)
	if c.GlobalInt("lastledger") > 0 {
		base.LastLedgerSequence = new(uint32)
		*base.LastLedgerSequence = uint32(c.GlobalInt("lastledger"))
	}
	if base.Flags == nil {
		base.Flags = new(data.TransactionFlag)
	}
	copy(base.Account[:], id.Payload())
	copy(base.SigningPubKey[:], pub.Payload())
	if c.GlobalString("fee") != "" {
		fee, err := data.NewNativeValue(int64(c.GlobalInt("fee")))
		checkErr(err)
		base.Fee = *fee
	}
	tx.GetBase().TxnSignature = &data.VariableLength{}
	checkErr(data.Sign(tx, priv))
}

func submitTx(tx data.Transaction) {
	r, err := websockets.NewRemote("wss://s-east.ripple.com:443")
	checkErr(err)
	result, err := r.Submit(tx)
	checkErr(err)
	fmt.Printf("%s: %s\n", result.EngineResult, result.EngineResultMessage)
	os.Exit(0)
}

func outputTx(c *cli.Context, tx data.Transaction) {
	if !c.GlobalBool("json") {
		hash, raw, err := data.Raw(tx)
		checkErr(err)

		if c.GlobalBool("binary") {
			os.Stdout.Write(raw)
		} else {
			fmt.Printf("Hash: %X\nRaw: %X\n", hash, raw)
		}
	}

	if c.GlobalBool("json") || !c.GlobalBool("binary") {
		// Print it in JSON
		out, err := json.Marshal(tx)
		checkErr(err)
		fmt.Println(string(out))
	}

	if c.GlobalBool("submit") {
		submitTx(tx)
	}
}

func payment(c *cli.Context) {
	// Validate and parse required fields
	if c.String("dest") == "" || c.String("amount") == "" || key == nil {
		fmt.Println("Destination, amount, and seed are required")
		os.Exit(1)
	}
	destination, amount := parseAccount(c.String("dest")), parseAmount(c.String("amount"))

	// Create payment and sign it
	payment := &data.Payment{
		Destination: *destination,
		Amount:      *amount,
	}
	payment.TransactionType = data.PAYMENT

	if c.String("paths") != "" {
		payment.Paths = parsePaths(c.String("paths"))
	}

	if c.String("sendmax") != "" {
		payment.SendMax = parseAmount(c.String("sendmax"))
	}

	payment.Flags = new(data.TransactionFlag)
	if c.Bool("nodirect") {
		*payment.Flags = *payment.Flags | data.TxNoDirectRipple
	}
	if c.Bool("partial") {
		*payment.Flags = *payment.Flags | data.TxPartialPayment
	}
	if c.Bool("limit") {
		*payment.Flags = *payment.Flags | data.TxLimitQuality
	}

	sign(c, payment, 0)
	outputTx(c, payment)
}

func trust(c *cli.Context) {
	// Validate and parse required fields
	if c.String("amount") == "" || key == nil {
		fmt.Println("Amount and seed are required")
		os.Exit(1)
	}
	amount := parseAmount(c.String("amount"))

	// Create tx and sign it
	tx := &data.TrustSet{
		LimitAmount:      *amount,
	}
	tx.TransactionType = data.TRUST_SET

	tx.QualityOut = new(uint32)
	*tx.QualityOut = uint32(c.Float64("quality-out")*1000000000)

	tx.QualityIn = new(uint32)
	*tx.QualityIn = uint32(c.Float64("quality-in")*1000000000)

	tx.Flags = new(data.TransactionFlag)
	if c.Bool("auth") {
		*tx.Flags = *tx.Flags | data.TxSetAuth
	}
	if c.Bool("noripple") {
		*tx.Flags = *tx.Flags | data.TxSetNoRipple
	}
	if c.Bool("clear-noripple") {
		*tx.Flags = *tx.Flags | data.TxClearNoRipple
	}
	if c.Bool("freeze") {
		*tx.Flags = *tx.Flags | data.TxSetFreeze
	}
	if c.Bool("clear-freeze") {
		*tx.Flags = *tx.Flags | data.TxClearFreeze
	}

	sign(c, tx, 0)
	outputTx(c, tx)
}

func submit(c *cli.Context) {
	bs, err := ioutil.ReadAll(os.Stdin)
	checkErr(err)

	tx, err := data.ReadTransaction(bytes.NewReader(bs))
	checkErr(err)

	outputTx(c, tx)
}

func common(c *cli.Context) error {
	if c.GlobalString("seed") != "" {
		key = parseSeed(c.String("seed"))
	}

	return nil
}

var key *crypto.RootDeterministicKey

func main() {
	app := cli.NewApp()
	app.Name = "tx"
	app.Usage = "create a Ripple transaction. Sequence and seed must be specified for every command."
	app.Version = "0.1"
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "seed,s", Value: "", Usage: "the seed for the submitting account"},
		cli.IntFlag{Name: "fee,f", Value: 10, Usage: "the fee you want to pay"},
		cli.IntFlag{Name: "sequence,q", Value: 0, Usage: "the sequence for the transaction"},
		cli.IntFlag{Name: "lastledger,l", Value: 0, Usage: "highest ledger number that the transaction can appear in"},
		cli.BoolFlag{Name: "submit,t", Usage: "submits the transaction via websocket"},
		cli.BoolFlag{Name: "binary,b", Usage: "raw output in binary"},
		cli.BoolFlag{Name: "json,j", Usage: "output only the resulting JSON"},
	}
	app.Before = common
	app.Commands = []cli.Command{{
		Name:        "payment",
		ShortName:   "p",
		Usage:       "create a payment",
		Description: "seed, sequence, destination and amount are required",
		Action:      payment,
		Flags: []cli.Flag{
			cli.StringFlag{Name: "dest,d", Value: "", Usage: "destination account"},
			cli.StringFlag{Name: "amount,a", Value: "", Usage: "amount to send"},
			cli.IntFlag{Name: "tag,t", Value: 0, Usage: "destination tag"},
			cli.StringFlag{Name: "invoice,i", Value: "", Usage: "invoice id (will be passed through SHA512Half)"},
			cli.StringFlag{Name: "paths", Value: "", Usage: "paths"},
			cli.StringFlag{Name: "sendmax,m", Value: "", Usage: "maximum to send"},
			cli.BoolFlag{Name: "nodirect,r", Usage: "do not look for direct path"},
			cli.BoolFlag{Name: "partial,p", Usage: "permit partial payment"},
			cli.BoolFlag{Name: "limit,l", Usage: "limit quality"},
		},
	}, {
		Name:        "trust",
		ShortName:   "t",
		Usage:       "set trust",
		Description: "seed, sequence, destination and amount are required",
		Action:      trust,
		Flags: []cli.Flag{
			cli.StringFlag{Name: "amount,a", Value: "", Usage: "trust limit"},
			cli.Float64Flag{Name: "quality-out,q", Value: 1.0, Usage: "> 1.0 to charge a fee"},
			cli.Float64Flag{Name: "quality-in,Q", Value: 1.0, Usage: "< 1.0 to charge a fee"},
			cli.BoolFlag{Name: "auth,A", Usage: "SetAuth"},
			cli.BoolFlag{Name: "noripple,n", Usage: "no rippling on this trustline"},
			cli.BoolFlag{Name: "clear-noripple,N", Usage: "re-enable rippling on this trustline"},
			cli.BoolFlag{Name: "freeze,f", Usage: "freeze this trustline"},
			cli.BoolFlag{Name: "clear-freeze,F", Usage: "unfreeze this trustline"},
		},
	}, {
		Name:        "submit",
		ShortName:   "s",
		Usage:       "submit a transaction",
		Description: "pass a transaction on stdin",
		Action:      submit,
	}}
	app.Run(os.Args)
}
