package main

import (
	"flag"
	"github.com/gorilla/websocket"
	"log"
	luno_go_examples "luno-go-examples"
	"time"
)

var apiKey = flag.String("api_key", "", "The Luno API key id")
var apiSecretPath = flag.String("api_secret_path", "./secret", "The Luno API secret file path")

func main() {
	flag.Parse()

	url := "wss://ws.luno.com/api/1/userstream"
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = ws.Close()
	}()

	ws.SetPongHandler(func(data string) error {
		return ws.SetReadDeadline(time.Now().Add(time.Minute))
	})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			defer ws.SetWriteDeadline(time.Time{})
			err := ws.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	cred := credentials{
		APIKeyID:     *apiKey,
		APIKeySecret: luno_go_examples.ReadSecret(*apiSecretPath),
	}

	_ = ws.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := ws.WriteJSON(cred); err != nil {
		log.Fatal(err)
	}
	_ = ws.SetWriteDeadline(time.Time{})

	log.Printf("luno/streaming: Connection established key=%s", cred.APIKeyID)

	for {
		var u update
		err := ws.ReadJSON(&u)
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
			log.Printf("error: %v", err)
		} else if err != nil {
			log.Fatal(err)
		}
		if u.ErrCode != "" {
			log.Printf("Got error: %s (%s)", u.ErrMsg, u.ErrCode)
			continue
		}
		if u.FillUpdate != nil {
			log.Printf("Got fill update @%d for %s: base=%s counter=%s", u.Timestamp, u.FillUpdate.OrderID, u.FillUpdate.BaseFill, u.FillUpdate.CounterFill)
		}
		if u.StatusUpdate != nil {
			log.Printf("Got status update @%d for %s: status=%s", u.Timestamp, u.StatusUpdate.OrderID, u.StatusUpdate.Status)
		}
		if u.BalanceUpdate != nil {
			log.Printf("Got balance update @%d for %d: available_delta=%s balance_delta=%s", u.Timestamp, u.BalanceUpdate.AccountID, u.BalanceUpdate.AvailableDelta, u.BalanceUpdate.BalanceDelta)
		}
	}
}

type update struct {
	ErrCode       string             `json:"error_code"`
	ErrMsg        string             `json:"error"`
	Type          string             `json:"type"`
	Timestamp     int64              `json:"timestamp"`
	StatusUpdate  *orderStatusUpdate `json:"order_status_update"`
	FillUpdate    *orderFillUpdate   `json:"order_fill_update"`
	BalanceUpdate *balanceUpdate     `json:"balance_update"`
}

type orderStatusUpdate struct {
	OrderID  string `json:"order_id"`
	MarketID string `json:"market_id"`
	Status   string `json:"status"`
}

type orderFillUpdate struct {
	OrderID         string `json:"order_id"`
	MarketID        string `json:"market_id"`
	BaseFill        string `json:"base_fill"`
	CounterFill     string `json:"counter_fill"`
	BaseDelta       string `json:"base_delta"`
	CounterDelta    string `json:"counter_delta"`
	BaseFee         string `json:"base_fee"`
	CounterFee      string `json:"counter_fee"`
	BaseFeeDelta    string `json:"base_fee_delta"`
	CounterFeeDelta string `json:"counter_fee_delta"`
}

type balanceUpdate struct {
	AccountID      int64  `json:"account_id"`
	RowIndex       int    `json:"row_index"`
	Balance        string `json:"balance"`
	BalanceDelta   string `json:"balance_delta"`
	Available      string `json:"available"`
	AvailableDelta string `json:"available_delta"`
}

type credentials struct {
	APIKeyID     string `json:"api_key_id"`
	APIKeySecret string `json:"api_key_secret"`
}
