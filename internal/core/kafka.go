package core

import (
	"context"
	"time"

	"github.com/binhy/vistack/internal/config"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaWriter 全局 Kafka 生产者
var KafkaWriter *kafka.Writer

// KafkaConfig 保存 Kafka 配置以便复用（例如消费者需要 Brokers 和 GroupID）
var KafkaConfig config.AppConfig

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
		Async:        true, // 异步发送，提高吞吐量
		Completion: func(messages []kafka.Message, err error) {
			if err != nil && Logger != nil {
				Logger.Error("Kafka async write failed", zap.Error(err))
			}
		},
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

// StartKafkaConsumer 启动一个 Kafka 消费者
// handler 是处理消息的回调函数，返回 error 会记录日志（实际生产中可能需要重试机制）
func StartKafkaConsumer(ctx context.Context, topic string, handler func(ctx context.Context, key, value []byte) error) {
	if len(KafkaConfig.Kafka.Brokers) == 0 {
		if Logger != nil {
			Logger.Warn("Kafka brokers not configured, cannot start consumer", zap.String("topic", topic))
		}
		return
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  KafkaConfig.Kafka.Brokers,
		Topic:    topic,
		GroupID:  KafkaConfig.Kafka.GroupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	if Logger != nil {
		Logger.Info("Kafka consumer started",
			zap.String("topic", topic),
			zap.String("group_id", KafkaConfig.Kafka.GroupID),
		)
	}

	go func() {
		defer func() {
			if err := r.Close(); err != nil {
				if Logger != nil {
					Logger.Error("Failed to close kafka reader", zap.String("topic", topic), zap.Error(err))
				}
			}
		}()

		for {
			select {
			case <-ctx.Done():
				if Logger != nil {
					Logger.Info("Stopping kafka consumer", zap.String("topic", topic))
				}
				return
			default:
				m, err := r.ReadMessage(ctx)
				if err != nil {
					// 只有非 context canceled 错误才记录
					if ctx.Err() == nil && Logger != nil {
						Logger.Error("Failed to read kafka message", zap.String("topic", topic), zap.Error(err))
					}
					// 遇到错误稍微等待一下，避免死循环刷日志
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
				}
			}
		}
	}()
}
