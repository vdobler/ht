// Copyright 2018 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// kafka.go contains checks against Kafka topics.

package ht

import (
	"fmt"
	"strings"
	"time"

	"github.com/Shopify/sarama"
)

func init() {
	RegisterCheck(&Kafka{})
}

// ----------------------------------------------------------------------------
// Kafka

// Kafka checks that a certain message was delivered to a Kafka topic.
// This check starts consuming messages from all partitions from the given
// topic the moment the check is Prepared. It will consume messaged until
// a matching one is found check timeouts.
type Kafka struct {
	// Zookeeper is the comma-separated list of Zookeeper hosts
	Zookeeper string

	// Topic to observe.
	Topic string

	// Key and Message are used to match consumed messaged
	Key, Message Condition

	// Wait that long for matching messages. A zero value means 500ms.
	Wait time.Duration

	consumer sarama.Consumer
	combined chan *sarama.ConsumerMessage
	done     chan bool
}

// Prepare implements Check's Prepare method.
func (k *Kafka) Prepare(t *Test) (err error) {
	zookeepers := strings.Split(k.Zookeeper, ",")
	for i, z := range zookeepers {
		zookeepers[i] = strings.TrimSpace(z)
	}

	k.consumer, err = sarama.NewConsumer(zookeepers, nil)
	if err != nil {
		return err
	}
	partitions, err := k.consumer.Partitions(k.Topic)
	if err != nil {
		k.consumer.Close()
		return err
	}

	// Consume each partition in its own goroutine and combine all messages
	// into one channel.
	k.combined = make(chan *sarama.ConsumerMessage, 256)
	k.done = make(chan bool)
	for _, partition := range partitions {
		pc, err := k.consumer.ConsumePartition(k.Topic, partition, sarama.OffsetNewest)
		if err != nil {
			close(k.done)
			k.consumer.Close()
			return err
		}
		go func(pc sarama.PartitionConsumer, partition int32) {
			defer func() {
				err := pc.Close()
				t.debugf("Closing partition %d: err=%v", partition, err)
			}()

			t.debugf("Collecting messages from topic %q partition %d",
				k.Topic, partition)
			for {
				select {
				case msg := <-pc.Messages():
					t.debugf("Collected message from partition %d", partition)
					k.combined <- msg
				case <-k.done:
					return
				}
			}
		}(pc, partition)
	}

	if k.Wait <= 0 {
		k.Wait = 500 * time.Millisecond
	}

	return nil
}

var _ Preparable = &Kafka{}

// Execute implements Check's Execute method.
func (k *Kafka) Execute(t *Test) error {
	defer func() {
		// TODO: check for nil
		k.consumer.Close()
	}()

	timeout := time.After(k.Wait)

	consumed, matched := 0, false
loop:
	for {
		select {
		case msg := <-k.combined:
			consumed++
			if k.matches(msg) {
				matched = true
				break loop
			}
		case <-timeout:
			break loop
		}
	}
	close(k.done)

	if !matched {
		return fmt.Errorf("fruitlessly consumed %d non-matching messages",
			consumed)
	}

	return nil
}

func (k *Kafka) matches(msg *sarama.ConsumerMessage) bool {
	return k.Key.FulfilledBytes(msg.Key) == nil &&
		k.Message.FulfilledBytes(msg.Value) == nil
}
