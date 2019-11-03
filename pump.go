package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/mchmarny/gcputil/metric"
)

const (
	invocationMetric = "invocation"
	messagesMetric   = "message"
	durationMetric   = "duration"
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
	imp, err := newImportClient(ctx, in.Target)
	if err != nil {
		return nil, fmt.Errorf("bigquery client[%s.%s]: %v",
			in.Target.Dataset, in.Target.Table, err)
	}

	logger.Printf("creating pubsub subscription[%s]", in.Source.Subscription)
	sub := client.Subscription(in.Source.Subscription)
	inCtx, cancel := context.WithCancel(ctx)
	var mu sync.Mutex
	messageCounter := 0
	totalCounter := 0
	var innerError error
	lastMessage := time.Now()

	// this will cancel the sub Receive loop if max stall time has reached
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				elapsed := int(time.Now().Sub(lastMessage).Seconds())
				if elapsed > in.Source.MaxStall {
					logger.Println("max stall time elapsed")
					cancel()
					ticker.Stop()
					return
				}
			}
		}
	}()

	// start pulling messages
	receiveErr := sub.Receive(inCtx, func(ctx context.Context, msg *pubsub.Message) {

		lastMessage = time.Now()

		mu.Lock()
		defer mu.Unlock()

		messageCounter++
		totalCounter++

		//logger.Printf("appending %d", messageCounter)
		appendErr := imp.append(msg.Data)
		if appendErr != nil {
			logger.Printf("error on data append: %v", appendErr)
			innerError = appendErr
			return
		}

		msg.Ack() //TODO: Ack after inserts?

		// count
		if messageCounter == in.Target.BatchSize {
			logger.Println("batch size reached")
			messageCounter = 0
			if insertErr := imp.insert(ctx); insertErr != nil {
				innerError = insertErr
				return
			}
		}

		// duration
		elapsed := int(time.Now().Sub(start).Seconds())
		if elapsed > in.MaxDuration {
			logger.Println("time elapsed")
			cancel()
		}

	}) // end revive

	// ticker times no longer needed
	ticker.Stop()

	// receive error
	if receiveErr != nil {
		return nil, fmt.Errorf("pubsub subscription[%s] receive: %v",
			in.ID, receiveErr)
	}

	// error inside of receive handler
	if innerError != nil {
		return nil, fmt.Errorf("pubsub receive[%s] process error: %v",
			in.ID, innerError)
	}

	// insert leftovers
	if insertErr := imp.insert(ctx); insertErr != nil {
		return nil, fmt.Errorf("bigquery insert[%s] error: %v",
			in.ID, insertErr)
	}

	// metrics
	m, metricErr := metric.NewClient(ctx)
	if metricErr != nil {
		return nil, fmt.Errorf("metric client[%s]: %v",
			projectID, metricErr)
	}

	if metricErr = m.Publish(ctx, in.ID, invocationMetric, 1); err != nil {
		return nil, fmt.Errorf("metric record[%s][%s]: %v",
			in.ID, invocationMetric, metricErr)
	}

	if metricErr = m.Publish(ctx, in.ID, messagesMetric, totalCounter); err != nil {
		return nil, fmt.Errorf("metric record[%s][%s]: %v",
			in.ID, messagesMetric, metricErr)
	}

	totalDuration := time.Now().Sub(start).Seconds()
	if metricErr = m.Publish(ctx, in.ID, durationMetric, totalDuration); err != nil {
		return nil, fmt.Errorf("metric record[%s][%s]: %v",
			in.ID, durationMetric, metricErr)
	}

	// response
	r := &PumpResult{
		ExecutedOn:   start,
		Duration:     int(totalDuration),
		MessageCount: totalCounter,
		Request:      in,
		Release:      release,
	}

	return r, nil
}
