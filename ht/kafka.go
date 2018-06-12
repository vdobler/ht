// Copyright 2018 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// kafka.go contains checks against Kafka topics.

package ht

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/samuel/go-zookeeper/zk"
)

func init() {
	RegisterCheck(&Kafka{})
}

// ----------------------------------------------------------------------------
// Kafka

// Kafka checks that a certain message was delivered to a Kafka topic.
// This check starts consuming messages from all partitions from the given
// topic the moment the check is Prepared. It will consume messaged until
// a matching one is found or the check times out.
type Kafka struct {
	// Zookeeper is the comma-separated list of Zookeeper hosts to
	// query for brookers. If empty only the directly given Brookers
	// are used.
	Zookeeper string

	// Brookers is a comma-separated list of Kafka brookers. If empty
	// the brookers are read from the given Zookeeper.
	Brookers string

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
	if k.Zookeeper == "" && k.Brookers == "" {
		return errors.New("at least one of Zookeeper or Brookers must be specified")
	}

	brookers := splitAndTrim(k.Brookers)
	zookeepers := splitAndTrim(k.Zookeeper)
	if len(zookeepers) > 0 {
		zkbrookers, err := brookersFromZK(zookeepers)
		if len(zkbrookers) > 0 {
			brookers = append(zkbrookers, brookers...)
		} else {
			if len(brookers) == 0 {
				return fmt.Errorf("could not read brooker from Zookeper: %v", err)
			}
			t.infof("problems reading Kafka brookers from Zookeeper: %v", err)
		}
	}

	k.consumer, err = sarama.NewConsumer(brookers, nil)
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
	k.combined = make(chan *sarama.ConsumerMessage, 1024)
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

func splitAndTrim(list string) []string {
	r := []string{}
	for _, s := range strings.Split(list, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			r = append(r, s)
		}
	}
	return r
}

func brookersFromZK(zookeepers []string) ([]string, error) {
	conn, _, err := zk.Connect(zookeepers, time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	ids, _, err := conn.Children("/brokers/ids")
	if err != nil {
		return nil, err
	}

	hosts := []string{}
	for _, bid := range ids {
		data, _, err := conn.Get("/brokers/ids/" + bid)
		if err != nil {
			return nil, err
		}

		var x struct {
			Host string `json:"host"`
			Port int    `json:"port"`
		}
		err = json.Unmarshal([]byte(data), &x)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, fmt.Sprintf("%s:%d", x.Host, x.Port))
	}

	return hosts, nil
}

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
