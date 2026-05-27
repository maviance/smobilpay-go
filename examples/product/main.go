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

	products, err := c.Masterdata.Products(ctx, 90006)
	if err != nil {
		log.Fatal(err)
	}
	product := products[0]
	amount := 0
	if product.AmountLocalCur() != nil {
		amount = int(*product.AmountLocalCur())
	}

	quote, err := c.Initiate.Quote(ctx, smob.QuoteRequest{Amount: amount, PayItemID: product.PayItemID()})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := c.Confirm.Collect(ctx, smob.CollectionRequest{
		QuoteID:              quote.QuoteID,
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "customer@example.com",
		TRID:                 "ORDER-EXAMPLE-PRODUCT-001",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("PTN: %s, status: %s\n", resp.PTN, resp.Status)
}
