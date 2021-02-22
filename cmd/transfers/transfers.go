package main

import (
	"context"
	"flag"
	"log"
	"strconv"
	"time"

	luno_go_examples "luno-go-examples"

	"github.com/luno/luno-go"
	"github.com/luno/luno-go/decimal"
)

var apiKey = flag.String("api_key", "", "The Luno API key id")
var apiSecretPath = flag.String("api_secret_path", "./secret", "The Luno API secret file path")

// This date is inclusive
var startDate = time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

// This date is exclusive
var endDate = time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

func main() {
	flag.Parse()

	cl := luno.NewClient()
	cl.SetDebug(false)
	cl.SetAuth(*apiKey, luno_go_examples.ReadSecret(*apiSecretPath))

	ctx := context.Background()
	res, err := cl.GetBalances(ctx, new(luno.GetBalancesRequest))
	if err != nil {
		log.Fatal(err)
	}

	for _, b := range res.Balance {
		err := accountSummary(ctx, cl, b, startDate, endDate)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func accountSummary(ctx context.Context, cl *luno.Client, b luno.AccountBalance, start, end time.Time) error {
	accID, err := strconv.ParseInt(b.AccountId, 10, 64)
	if err != nil {
		return err
	}

	batchLimit := 100
	before := end.UnixNano() / 1e6
	var inboundSum, outboundSum decimal.Decimal
	for {
		req := &luno.ListTransfersRequest{
			AccountId: accID,
			Before:    before,
			Limit:     int64(batchLimit),
		}
		res, err := cl.ListTransfers(ctx, req)
		if err != nil {
			return err
		}

		for _, t := range res.Transfers {
			createdAt := t.CreatedAt.AsTime()

			if createdAt.Before(start) {
				break
			}

			if t.Inbound {
				inboundSum = inboundSum.Add(t.Amount)
			} else {
				outboundSum = outboundSum.Add(t.Amount)
			}
		}

		if len(res.Transfers) < batchLimit {
			break
		}

		last := res.Transfers[batchLimit-1]
		if last.CreatedAt.AsTime().Before(start) {
			break
		}

		before = last.CreatedAt.AsTime().UnixNano() / 1e6
	}

	log.Printf("Account: %s (%s)", b.AccountId, b.Asset)
	log.Printf("\tCredited: %s\tDebited: %s", inboundSum, outboundSum)

	return nil
}
