package main

import (
	"context"
	"fmt"
	"log"
	"os"

	smob "github.com/maviance/smobilpay-go"
)

func main() {
	cfg, _ := smob.NewConfig(
		smob.WithBaseURL(os.Getenv("SMOBILPAY_BASE_URL")),
		smob.WithCredentials(os.Getenv("SMOBILPAY_PUBLIC_KEY"), os.Getenv("SMOBILPAY_SECRET_KEY")),
	)
	c, _ := smob.New(cfg)
	ctx := context.Background()

	bills, err := c.Initiate.Bills(ctx, "ENEO", 10039, "203157530")
	if err != nil {
		log.Fatal(err)
	}
	bill := bills[0]
	amount := 0
	if bill.AmountLocalCur() != nil {
		amount = int(*bill.AmountLocalCur())
	}

	quote, err := c.Initiate.Quote(ctx, smob.QuoteRequest{Amount: amount, PayItemID: bill.PayItemID()})
	if err != nil {
		log.Fatal(err)
	}
	resp, err := c.Confirm.Collect(ctx, smob.CollectionRequest{
		QuoteID:              quote.QuoteID,
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "customer@example.com",
		ServiceNumber:        "203157530",
		CustomerName:         "Jane Doe",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("PTN: %s\n", resp.PTN)
}
