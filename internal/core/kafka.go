package core

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/binhy/vistack/internal/config"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaWriter 全局 Kafka 生产者
var KafkaWriter *kafka.Writer

// KafkaConfig 保存 Kafka 配置以便复用（例如消费者需要 Brokers 和 GroupID）
var KafkaConfig config.AppConfig

// consumerWG 跟踪所有消费者 goroutine，供优雅停机排空
var consumerWG sync.WaitGroup

// InitKafka 初始化 Kafka 生产者
func InitKafka(cfg *config.AppConfig) {
	if len(cfg.Kafka.Brokers) == 0 {
		if Logger != nil {
			Logger.Warn("Kafka brokers are empty, skip Kafka initialization")
		}
		return
	}

	KafkaConfig = *cfg

	// 初始化 Writer
	KafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(cfg.Kafka.Brokers...),
		Balancer: &kafka.LeastBytes{}, // 负载均衡策略
		// 生产环境建议配置超时
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		Async:        false, // 同步发送，写失败可由调用方捕获处理
	}

	if Logger != nil {
		Logger.Info("Kafka producer initialized", zap.Strings("brokers", cfg.Kafka.Brokers))
	}
}

// SendKafkaMessage 发送消息到指定 Topic
func SendKafkaMessage(ctx context.Context, topic, key string, value []byte) error {
	if KafkaWriter == nil {
		if Logger != nil {
			Logger.Warn("Kafka writer is not initialized")
		}
		return nil
	}

	// 动态设置 Topic，因为 Writer 可以是全局的，但 Topic 可以不同
	// 注意：segmentio/kafka-go 的 Writer 如果指定了 Topic，则只能发往该 Topic。
	// 如果未指定 Topic（如我们这里初始化时），则每条消息必须指定 Topic。
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	}

	err := KafkaWriter.WriteMessages(ctx, msg)
	if err != nil {
		if Logger != nil {
			Logger.Error("Failed to write kafka message",
				zap.String("topic", topic),
				zap.String("key", key),
				zap.Error(err),
			)
		}
		return err
	}
	return nil
}

// CloseKafka 关闭 Kafka 生产者连接
func CloseKafka() {
	if KafkaWriter != nil {
		if err := KafkaWriter.Close(); err != nil {
			if Logger != nil {
				Logger.Error("Failed to close kafka writer", zap.Error(err))
			}
		} else {
			if Logger != nil {
				Logger.Info("Kafka writer closed")
			}
		}
	}
}

// EnsureTopic 确保 Kafka topic 存在（不存在则创建，1 分区 1 副本）。
// 用于避免消费者在 topic 尚未创建时订阅、生产者首次写入失败。
func EnsureTopic(topic string) error {
	if len(KafkaConfig.Kafka.Brokers) == 0 {
		return nil
	}
	conn, err := kafka.Dial("tcp", KafkaConfig.Kafka.Brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()

	err = conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}

// StartKafkaConsumer 启动一个 Kafka 消费者：按配置并发度启动多个同 group reader。
// 同一 partition 只会被组内一个 reader 持有，因此同 partition 消息仍顺序处理、顺序提交
// （at-least-once 语义），并发上限 = min(concurrency, topic 分区数)。
// handler 是处理消息的回调函数，返回 error 会记录日志（实际生产中可能需要重试机制）。
func StartKafkaConsumer(ctx context.Context, topic string, handler func(ctx context.Context, key, value []byte) error) {
	if len(KafkaConfig.Kafka.Brokers) == 0 {
		if Logger != nil {
			Logger.Warn("Kafka brokers not configured, cannot start consumer", zap.String("topic", topic))
		}
		return
	}

	concurrency := KafkaConfig.Kafka.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	for i := 0; i < concurrency; i++ {
		consumerWG.Add(1)
		go runConsumer(ctx, topic, handler)
	}
}

// runConsumer 单个 reader 的消费循环
func runConsumer(ctx context.Context, topic string, handler func(ctx context.Context, key, value []byte) error) {
	defer consumerWG.Done()

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        KafkaConfig.Kafka.Brokers,
		Topic:          topic,
		GroupID:        KafkaConfig.Kafka.GroupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: 0,
	})
	defer func() {
		if err := r.Close(); err != nil {
			if Logger != nil {
				Logger.Error("Failed to close kafka reader", zap.String("topic", topic), zap.Error(err))
			}
		}
	}()

	if Logger != nil {
		Logger.Info("Kafka consumer started",
			zap.String("topic", topic),
			zap.String("group_id", KafkaConfig.Kafka.GroupID),
		)
	}

	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			// ctx 已取消是正常退出路径，直接返回；其他错误短暂等待后重试
			if ctx.Err() != nil {
				return
			}
			if Logger != nil {
				Logger.Error("Failed to read kafka message", zap.String("topic", topic), zap.Error(err))
			}
			time.Sleep(time.Second)
			continue
		}

		if Logger != nil {
			Logger.Debug("Received kafka message",
				zap.String("topic", m.Topic),
				zap.String("key", string(m.Key)),
				zap.Int64("offset", m.Offset),
			)
		}

		// 调用业务处理逻辑
		if err := handler(ctx, m.Key, m.Value); err != nil {
			if Logger != nil {
				Logger.Error("Failed to handle kafka message",
					zap.String("topic", topic),
					zap.ByteString("key", m.Key),
					zap.Error(err),
				)
			}
		} else {
			if err := r.CommitMessages(ctx, m); err != nil {
				if Logger != nil {
					Logger.Error("Failed to commit kafka message",
						zap.String("topic", topic),
						zap.ByteString("key", m.Key),
						zap.Error(err),
					)
				}
			}
		}
	}
}

// WaitKafkaConsumers 等待全部消费者 goroutine 退出（在途消息处理完成后才退出）；
// 超时返回 false，供优雅停机判断是否强制退出。
func WaitKafkaConsumers(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		consumerWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
