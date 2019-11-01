package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/mchmarny/gcputil/project"
)

var (
	projectID = project.GetIDOrFail()
)

// PumpResult represents the result of pump process
type PumpResult struct {
	ExecutedOn   time.Time    `json:"executed_on"`
	Duration     int          `json:"duration"`
	Release      string       `json:"release"`
	MessageCount int          `json:"message_count"`
	Request      *PumpRequest `json:"request"`
}

func pump(in *PumpRequest) (out *PumpResult, err error) {

	if in == nil {
		return nil, errors.New("nil PumpRequest")
	}

	ctx := context.Background()
	start := time.Now()

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub client[%s]: %v",
			projectID, err)
	}

	sub := client.Subscription(in.Subscription)
	inCtx, cancel := context.WithCancel(ctx)
	var mu sync.Mutex
	messages := make([]*pubsub.Message, 0)
	err = sub.Receive(inCtx, func(ctx context.Context, msg *pubsub.Message) {

		mu.Lock()
		defer mu.Unlock()

		// spool messages into array but don't msg.Ack()
		messages = append(messages, msg)
		msg.Ack()

		// count
		logger.Printf("messages spooled: %d\n", len(messages))
		if len(messages) == in.MaxMessages {
			cancel()
		}

		// duration
		elapsed := int(time.Now().Sub(start).Seconds())
		logger.Printf("seconds: %d\n", elapsed)
		if elapsed > in.MaxSeconds {
			cancel()
		}

	})

	logger.Println("done receiving")

	if err != nil {
		return nil, fmt.Errorf("pubsub subscription[%s]: %v",
			in.Subscription, err)
	}

	//TODO: build batch and save to BQ

	r := &PumpResult{
		ExecutedOn:   start,
		Duration:     int(time.Now().Sub(start).Seconds()),
		Request:      in,
		Release:      release,
		MessageCount: len(messages),
	}

	return r, nil
}
