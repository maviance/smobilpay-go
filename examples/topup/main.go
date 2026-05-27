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

	topups, err := c.Masterdata.Topups(ctx, 20051)
	if err != nil {
		log.Fatal(err)
	}
	item := topups[0]

	quote, err := c.Initiate.Quote(ctx, smob.QuoteRequest{Amount: 500, PayItemID: item.PayItemID()})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := c.Confirm.Collect(ctx, smob.CollectionRequest{
		QuoteID:              quote.QuoteID,
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "customer@example.com",
		ServiceNumber:        "237699999999",
		TRID:                 "ORDER-EXAMPLE-TOPUP-001",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("PTN: %s, status: %s\n", resp.PTN, resp.Status)
}
