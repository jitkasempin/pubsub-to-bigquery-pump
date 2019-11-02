package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
)

// PumpResult represents the result of pump process
type PumpResult struct {
	ExecutedOn   time.Time `json:"executed_on"`
	Duration     int       `json:"duration"`
	Release      string    `json:"release"`
	Request      *PumpJob  `json:"request"`
	MessageCount int       `json:"message_count"`
}

func pump(in *PumpJob) (out *PumpResult, err error) {

	if in == nil {
		return nil, errors.New("nil PumpRequest")
	}

	ctx := context.Background()
	start := time.Now()

	logger.Printf("creating pubsub client[%s]", projectID)
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub client[%s]: %v",
			projectID, err)
	}

	logger.Printf("creating importer client[%s.%s.%s]",
		projectID, in.Target.Dataset, in.Target.Table)
	imp, err := getImportClient(ctx, projectID, in.Target.Dataset, in.Target.Table)
	if err != nil {
		return nil, fmt.Errorf("bigquery client[%s.%s]: %v",
			in.Target.Dataset, in.Target.Table, err)
	}

	logger.Printf("creating pubsub subscription[%s]", in.Source.Subscription)
	sub := client.Subscription(in.Source.Subscription)
	inCtx, cancel := context.WithCancel(ctx)
	var mu sync.Mutex
	messageCounter := 0
	var receiveErr error
	err = sub.Receive(inCtx, func(ctx context.Context, msg *pubsub.Message) {

		mu.Lock()
		defer mu.Unlock()

		logger.Printf("appending: %q\n", msg.Data)
		appendErr := imp.append(msg.Data)
		if appendErr != nil {
			logger.Printf("error on data append: %v", appendErr)
			receiveErr = appendErr
			return
		}

		logger.Printf("acknowledging: %s\n", msg.ID)
		msg.Ack()

		// count
		messageCounter++
		if messageCounter == in.MaxMessages {
			logger.Println("message count exceeded")
			cancel()
		}

		// duration
		elapsed := int(time.Now().Sub(start).Seconds())
		if elapsed > in.MaxSeconds {
			logger.Println("time elapsed")
			cancel()
		}

	})

	if receiveErr != nil {
		return nil, fmt.Errorf("pubsub receive[%s] process error: %v",
			in.ID, receiveErr)
	}

	if err != nil {
		return nil, fmt.Errorf("pubsub subscription[%s] receive: %v",
			in.ID, err)
	}

	logger.Println("inserting...")
	if insertErr := imp.insert(ctx); insertErr != nil {
		return nil, fmt.Errorf("bigquery insert[%s] error: %v",
			in.ID, insertErr)
	}

	r := &PumpResult{
		ExecutedOn:   start,
		Duration:     int(time.Now().Sub(start).Seconds()),
		Request:      in,
		Release:      release,
		MessageCount: messageCounter,
	}

	return r, nil
}
