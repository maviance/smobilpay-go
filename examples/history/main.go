package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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

	today := time.Now().UTC()
	weekAgo := today.AddDate(0, 0, -7)
	rows, err := c.Verify.HistoryByDateRange(ctx, weekAgo, today)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("history rows: %d\n", len(rows))
	for i, r := range rows {
		if i >= 3 {
			break
		}
		fmt.Printf("  %d) PTN=%s status=%s trid=%s\n", i+1, r.PTN, r.Status, r.TRID)
	}
}
