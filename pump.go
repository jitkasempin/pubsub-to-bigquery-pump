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
	// custom metrics dimensions
	invocationMetric = "invocation"
	messagesMetric   = "message"
	durationMetric   = "duration"
)

// PumpResult represents the result of pump process
type PumpResult struct {
	// ExecutedOn is the time when the pump job was executed
	ExecutedOn time.Time `json:"executed_on"`
	// Duration is the amount of seconds that the pump job took
	Duration int `json:"duration"`
	// Release is the version of the code that was used to
	// execute this pump job
	Release string `json:"release"`
	// Request is the job configuration that was originally submitted
	Request *PumpJob `json:"request"`
	// MessageCount is the total message count processed by this pump job
	MessageCount int `json:"message_count"`
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

	logger.Printf("creating importer[%s.%s.%s]",
		projectID, in.Target.Dataset, in.Target.Table)
	imp, err := newImportClient(ctx, &in.Target)
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

	// this will cancel the sub receive loop if max stall time has reached
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				elapsed := int(time.Now().Sub(lastMessage).Seconds())
				if elapsed > in.Source.MaxStall {
					logger.Println("max stall time reached")
					cancel()
					ticker.Stop()
					return
				}
			}
		}
	}()

	// start pulling messages from subscription
	receiveErr := sub.Receive(inCtx, func(ctx context.Context, msg *pubsub.Message) {

		lastMessage = time.Now()

		mu.Lock()
		defer mu.Unlock()

		messageCounter++
		totalCounter++

		// append message to the importer
		appendErr := imp.append(msg.Data)
		if appendErr != nil {
			logger.Printf("error on data append: %v", appendErr)
			innerError = appendErr
			return
		}

		msg.Ack() //TODO: Ack after inserts?

		// check whether time to exec the batch
		if messageCounter == in.Target.BatchSize {
			logger.Println("batch size reached")
			messageCounter = 0
			if insertErr := imp.insert(ctx); insertErr != nil {
				innerError = insertErr
				return
			}
		}

		// check if max job time has been reached
		elapsed := int(time.Now().Sub(start).Seconds())
		if elapsed > in.MaxDuration {
			logger.Println("max job exec time reached")
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
	totalDuration := time.Now().Sub(start).Seconds()
	if metricErr := submitMetrics(ctx, in.ID, totalCounter, totalDuration); metricErr != nil {
		return nil, fmt.Errorf("metrics[%s] error: %v",
			in.ID, metricErr)
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

func submitMetrics(ctx context.Context, id string, c int, d float64) error {
	m, err := metric.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("metric client[%s]: %v", projectID, err)
	}

	if err = m.Publish(ctx, id, invocationMetric, 1); err != nil {
		return fmt.Errorf("metric record[%s][%s]: %v", id, invocationMetric, err)
	}

	if err = m.Publish(ctx, id, messagesMetric, c); err != nil {
		return fmt.Errorf("metric record[%s][%s]: %v", id, messagesMetric, err)
	}

	if err = m.Publish(ctx, id, durationMetric, d); err != nil {
		return fmt.Errorf("metric record[%s][%s]: %v", id, durationMetric, err)
	}

	return nil
}
