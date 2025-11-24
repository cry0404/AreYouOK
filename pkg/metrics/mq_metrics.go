package metrics

// var (
//     // 消息发送失败次数
//     MessagePublishFailures = prometheus.NewCounterVec(
//         prometheus.CounterOpts{
//             Name: "mq_message_publish_failures_total",
//             Help: "Total number of message publish failures",
//         },
//         []string{"exchange", "routing_key"},
//     )

//     // 消息消费失败次数
//     MessageConsumeFailures = prometheus.NewCounterVec(
//         prometheus.CounterOpts{
//             Name: "mq_message_consume_failures_total",
//             Help: "Total number of message consume failures",
//         },
//         []string{"queue", "retry_count"},
//     )

//     // 死信队列消息数量
//     DLQMessageCount = prometheus.NewGaugeVec(
//         prometheus.GaugeOpts{
//             Name: "mq_dlq_message_count",
//             Help: "Number of messages in dead letter queue",
//         },
//         []string{"queue"},
//     )

//     // 待补偿消息数量
//     OutboxPendingCount = prometheus.NewGauge(
//         prometheus.GaugeOpts{
//             Name: "outbox_pending_messages_count",
//             Help: "Number of pending messages in outbox table",
//         },
//     )
// )
