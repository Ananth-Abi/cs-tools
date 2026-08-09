// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package eventbus

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// handleAttempts is how many times a single record's Handle func is retried
// before giving up on it. There is no dead-letter topic yet (see the package
// doc and the service's CLAUDE.md) — a record that still fails after this
// many attempts is logged at ERROR and its offset is committed anyway, so one
// permanently-failing record (e.g. a downstream outage) cannot block every
// later record on its partition forever.
const handleAttempts = 3

// handleRetryDelay is the fixed pause between attempts. This is deliberately
// simple (no exponential backoff) since handleAttempts is small and this is
// covering transient blips (a downstream timeout), not sustained outages.
const handleRetryDelay = 2 * time.Second

// Record is the eventbus-agnostic view of a consumed message that Handle
// receives — deliberately not the underlying Kafka client's own message
// type, so dispatch (and any future caller) never needs to import
// github.com/segmentio/kafka-go directly.
type Record struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
}

// Consumer reads records from a topic as a member of a named consumer group,
// so multiple running instances of this service split the topic's partitions
// between them instead of each seeing every record.
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer constructs a Consumer that joins groupID and consumes
// cfg.Topic. Auto-commit is not used: offsets are committed explicitly by
// Run, only after a record has been handled (or exhausted its retries) —
// never before, so a crash mid-processing redelivers the record on restart
// instead of silently skipping it.
func NewConsumer(cfg Config, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{cfg.Broker},
			GroupID: groupID,
			Topic:   cfg.Topic,
			Dialer: &kafka.Dialer{
				TLS:           &tls.Config{MinVersion: tls.VersionTLS12},
				SASLMechanism: cfg.saslMechanism(),
			},
			// Only applies to a partition with no committed offset yet (e.g.
			// the very first time this consumer group ever runs) — this is
			// kafka-go's own default, set explicitly here for clarity and to
			// document the reason: a notification service should process
			// backlog, not silently start from the tail. The Kafka client
			// used before this one defaulted the other way and needed this
			// set explicitly to avoid dropping events published just before
			// its first join — confirmed against the real namespace.
			StartOffset: kafka.FirstOffset,
			Logger:      kafka.LoggerFunc(readerLogWarn),
			ErrorLogger: kafka.LoggerFunc(logError),
			// kafka-go's consumer-group rebalancing only offers Range and
			// RoundRobin balancers (its default, left unset here) — there is
			// no cooperative/incremental strategy like the Kafka client used
			// before this one had. In practice this only matters once this
			// service scales beyond one instance: a rebalance briefly pauses
			// every partition in the group instead of only the ones actually
			// moving. Not a concern for a single running instance.
		}),
	}
}

// Handle processes a single record. A non-nil error causes Run to retry (see
// handleAttempts).
type Handle func(context.Context, Record) error

// Run polls for records and calls handle for each one, committing its offset
// once handle succeeds or its retries are exhausted. Run blocks until ctx is
// canceled or the Consumer is closed; call it from its own goroutine.
func (c *Consumer) Run(ctx context.Context, handle Handle) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				return
			}
			slog.ErrorContext(ctx, "eventbus: fetch error", "err", err)
			continue
		}
		c.handleAndCommit(ctx, msg, handle)
	}
}

func (c *Consumer) handleAndCommit(ctx context.Context, msg kafka.Message, handle Handle) {
	record := Record{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       msg.Key,
		Value:     msg.Value,
	}

	var err error
	for attempt := 1; attempt <= handleAttempts; attempt++ {
		if err = handle(ctx, record); err == nil {
			break
		}
		slog.ErrorContext(ctx, "eventbus: handler failed",
			"topic", record.Topic, "partition", record.Partition, "offset", record.Offset,
			"attempt", attempt, "maxAttempts", handleAttempts, "err", err)
		if attempt < handleAttempts {
			select {
			case <-ctx.Done():
				return
			case <-time.After(handleRetryDelay):
			}
		}
	}
	if err != nil {
		// TODO: publish (record, err) to a dead-letter topic once one exists
		// (see the package doc) instead of only logging — for now this
		// record's content is only recoverable from Event Hub's own
		// retention window.
		slog.ErrorContext(ctx, "eventbus: handler exhausted retries, dropping record",
			"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", err)
	}
	if cerr := c.reader.CommitMessages(ctx, msg); cerr != nil {
		slog.ErrorContext(ctx, "eventbus: commit failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", cerr)
	}
}

// Close leaves the consumer group and closes the underlying connection.
func (c *Consumer) Close() {
	_ = c.reader.Close()
}
