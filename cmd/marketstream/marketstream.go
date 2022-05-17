package main

import (
	"context"
	"flag"
	"github.com/luno/luno-go"
	"github.com/luno/luno-go/decimal"
	"github.com/luno/luno-go/streaming"
	"golang.org/x/sync/errgroup"
	"log"
	luno_go_examples "luno-go-examples"
)

var apiKey = flag.String("api_key", "", "The Luno API key id")
var apiSecretPath = flag.String("api_secret_path", "./secret", "The Luno API secret file path")

// Just some example markets to listen to. Feel free to add/remove as you
// please but keep in mind each will create a new connection.
var markets = []string{
	// Crypto/BTC markets
	"BCHXBT",
	"ETHXBT",
	"LTCXBT",
	"XRPXBT",

	// Crypto/Stable markets
	"ETHUSDC",
	"XBTUSDC",

	"XBTIDR",
}

type TradeSetup struct {
	ordersSpread   decimal.Decimal
	ordersVolume   decimal.Decimal
	baseBalance    decimal.Decimal
	counterBalance decimal.Decimal
}

var marketSetup = map[string]TradeSetup{
	"BCHXBT": {
		ordersSpread:   decimal.NewFromFloat64(0.000035, 6),
		ordersVolume:   decimal.NewFromFloat64(15, 1),
		baseBalance:    decimal.NewFromFloat64(150, 8),
		counterBalance: decimal.NewFromFloat64(1, 8),
	},
	"ETHXBT": {
		ordersSpread:   decimal.NewFromFloat64(0.00035, 6),
		ordersVolume:   decimal.NewFromFloat64(1.5, 2),
		baseBalance:    decimal.NewFromFloat64(15, 8),
		counterBalance: decimal.NewFromFloat64(1, 8),
	},
	"LTCXBT": {
		ordersSpread:   decimal.NewFromFloat64(0.00001, 6),
		ordersVolume:   decimal.NewFromFloat64(45, 2),
		baseBalance:    decimal.NewFromFloat64(450, 8),
		counterBalance: decimal.NewFromFloat64(1, 8),
	},
	"XRPXBT": {
		ordersSpread:   decimal.NewFromFloat64(0.00000006, 8),
		ordersVolume:   decimal.NewFromFloat64(7000, 2),
		baseBalance:    decimal.NewFromFloat64(70000, 8),
		counterBalance: decimal.NewFromFloat64(1, 8),
	},

	"ETHUSDC": {
		ordersSpread:   decimal.NewFromFloat64(10, 8),
		ordersVolume:   decimal.NewFromFloat64(0.05, 2),
		baseBalance:    decimal.NewFromFloat64(1, 8),
		counterBalance: decimal.NewFromFloat64(2000, 8),
	},
	"XBTUSDC": {
		ordersSpread:   decimal.NewFromFloat64(150, 8),
		ordersVolume:   decimal.NewFromFloat64(0.1, 2),
		baseBalance:    decimal.NewFromFloat64(1, 8),
		counterBalance: decimal.NewFromFloat64(30000, 8),
	},
}

func main() {
	flag.Parse()

	var eg errgroup.Group
	for _, market := range markets {
		eg.Go(runPaperTrading(market))
	}

	if err := eg.Wait(); err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
}

// This trade automation places orders at a given spread but swings the
// mid-price and total volume depending on the base and counter ratio.
func runPaperTrading(market string) func() error {
	return func() error {
		updatesCh := make(chan streaming.Update, 100)
		conn, err := streaming.Dial(*apiKey, luno_go_examples.ReadSecret(*apiSecretPath), market,
			streaming.WithUpdateCallback(func(u streaming.Update) { updatesCh <- u }))
		if err != nil {
			return err
		}
		defer conn.Close()

		targetSpread := marketSetup[market].ordersSpread

		totalOrderVolume := marketSetup[market].ordersVolume

		baseWallet := marketSetup[market].baseBalance
		counterWallet := marketSetup[market].counterBalance

		totalBought := decimal.Zero()
		totalSold := decimal.Zero()
		totalSpent := decimal.Zero()
		totalEarned := decimal.Zero()

		ticker, err := luno.NewClient().GetTicker(context.TODO(), &luno.GetTickerRequest{
			Pair: market,
		})

		myAsk := luno.OrderBookEntry{
			Price:  ticker.Ask,
			Volume: decimal.Zero(),
		}

		myBid := luno.OrderBookEntry{
			Price:  ticker.Bid,
			Volume: decimal.Zero(),
		}

		for u := range updatesCh {
			if len(u.TradeUpdates) == 0 {
				continue
			}

			lastTrade := u.TradeUpdates[len(u.TradeUpdates)-1]
			lastPrice := lastTrade.Counter.Div(lastTrade.Base, 8)

			if lastPrice.Cmp(myAsk.Price) >= 0 {
				// Assume our ask order would have traded
				sellVol := minDec(myAsk.Volume, lastTrade.Base)
				sellPrice := minDec(myAsk.Price, lastPrice)
				sellValue := sellVol.Mul(sellPrice)

				baseWallet = baseWallet.Sub(sellVol)
				counterWallet = counterWallet.Add(sellValue)
				log.Printf("%s: sold %s@%s", market, myAsk.Volume, lastPrice)

				totalSold = totalSold.Add(sellVol)
				totalEarned = totalEarned.Add(sellValue)
			} else if lastPrice.Cmp(myBid.Price) <= 0 {
				// Assume our bid order would have traded
				buyVol := minDec(myBid.Volume, lastTrade.Base)
				buyPrice := minDec(myBid.Price, lastPrice)
				buyValue := buyVol.Mul(buyPrice)

				baseWallet = baseWallet.Add(buyVol)
				counterWallet = counterWallet.Sub(buyValue)
				log.Printf("%s: bought %s@%s", market, myBid.Volume, lastPrice)

				totalBought = totalBought.Add(buyVol)
				totalSpent = totalSpent.Add(buyValue)
			} else {
				// Assume nothing happened
				log.Printf("%s: last_price=%s", market, lastPrice)
				continue
			}

			if totalBought.Sign() != 0 && totalSold.Sign() != 0 {
				avgBuyValue := totalSpent.Div(totalBought, 8)
				avgSellValue := totalEarned.Div(totalSold, 8)
				log.Printf("%s: avg_buy_value=%s avg_sell_value=%s avg_buy_return=%s", market,
					avgBuyValue,
					avgSellValue,
					totalBought.Mul(avgSellValue.Sub(avgBuyValue)))
			}

			baseValue := baseWallet.Mul(lastPrice)
			totalValue := baseWallet.Mul(lastPrice).Add(counterWallet)
			baseRatio := baseValue.Div(totalValue, 9)
			counterRatio := counterWallet.Div(totalValue, 9)

			log.Printf("%s: current_value=%s base_ratio=%s counter_ratio=%s",
				market, totalValue.ToScale(6),
				baseRatio.ToScale(3), counterRatio.ToScale(3))

			myAsk = luno.OrderBookEntry{
				Price:  lastPrice.Add(targetSpread.Mul(counterRatio)),
				Volume: totalOrderVolume.Mul(baseRatio),
			}
			myBid = luno.OrderBookEntry{
				Price:  lastPrice.Sub(targetSpread.Mul(baseRatio)),
				Volume: totalOrderVolume.Mul(counterRatio),
			}

			log.Printf("%s: ask_price=%s bid_price=%s spread=%s%%",
				market, myAsk.Price, myBid.Price,
				myAsk.Price.Sub(myBid.Price).Div(lastPrice, 4).Mul(decimal.NewFromInt64(100)))
		}

		return nil
	}
}

func minDec(vals ...decimal.Decimal) decimal.Decimal {
	if len(vals) == 0 {
		return decimal.Decimal{}
	}
	min := vals[0]
	for _, v := range vals[1:] {
		if v.Cmp(min) < 0 {
			min = v
		}
	}
	return min
}
