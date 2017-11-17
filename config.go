package main

import "time"

const (
	connectTimeout     = time.Second * 5
	readTimeout        = time.Second * 10
	removeAfter        = time.Second * 60
	clearTopicInterval = time.Second * 10
	// maxSubscribeTopics = 100 // 一个请求最多订阅几个 topic
)
