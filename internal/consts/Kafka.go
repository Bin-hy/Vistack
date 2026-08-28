package consts

type KafkaTopic string

const (
	KafkaTopicTranscode         KafkaTopic = "transcode"          // 转码任务
	KafkaTopicDeleteFile        KafkaTopic = "delete_file"        // 删除文件任务
	KafkaTopicDanmaku           KafkaTopic = "danmaku"            // 弹幕落库
	KafkaTopicCommentModeration KafkaTopic = "comment_moderation" // 评论图片审核
)
