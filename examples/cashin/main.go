package main

import (
	"context"
	"fmt"
	"log"
	"os"

	smob "github.com/maviance/smobilpay-go"
)

func main() {
	cfg, err := smob.NewConfig(
		smob.WithBaseURL(os.Getenv("SMOBILPAY_BASE_URL")),
		smob.WithCredentials(os.Getenv("SMOBILPAY_PUBLIC_KEY"), os.Getenv("SMOBILPAY_SECRET_KEY")),
	)
	if err != nil {
		log.Fatal(err)
	}
	c, err := smob.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	cashins, err := c.Masterdata.Cashins(ctx, 50052)
	if err != nil {
		log.Fatal(err)
	}
	item := cashins[0]

	quote, err := c.Initiate.Quote(ctx, smob.QuoteRequest{Amount: 1000, PayItemID: item.PayItemID()})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := c.Confirm.Collect(ctx, smob.CollectionRequest{
		QuoteID:              quote.QuoteID,
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "customer@example.com",
		ServiceNumber:        "237699999999",
		TRID:                 "ORDER-EXAMPLE-CASHIN-001",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("PTN: %s, status: %s\n", resp.PTN, resp.Status)
}
