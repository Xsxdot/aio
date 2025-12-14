package util

import (
	"context"
	"errors"
)

var env = "正式版，"

func SendMessage(ctx context.Context, webhook string, message *DingMessage) error {
	result, err := HttpPost(webhook, message)
	if err != nil {
		return err
	}

	if errMsg, ok := result.Get("errmsg").Value().(string); ok && len(errMsg) > 0 {
		if errMsg == "ok" {
			return nil
		}
	}
	return errors.New(result.Get("msg").String())
}

func SendOpsMessage(ctx context.Context, message string) error {
	return SendMessage(ctx, "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=f0d453bb-9589-4dcd-b643-be668914776d", &DingMessage{
		MsgType: "text",
		Text: TextContent{
			Content: env + message,
		},
	})
}

type DingMessage struct {
	MsgType  DingMessageType `json:"msgtype"`
	Text     TextContent     `json:"text"`
	Markdown MarkDownContent `json:"markdown"`
	At       At              `json:"at"`
}

type MessageContent map[string]interface{}

type At struct {
	AtUserIds []string `json:"atUserIds"`
	AtMobiles []string `json:"atMobiles"`
	IsAtAll   bool     `json:"isAtAll"`
}

type TextContent struct {
	Content string `json:"content"`
}

type MarkDownContent struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type DingMessageType string

const (
	DingMessageTypeText     DingMessageType = "text"
	DingMessageTypeMarkdown DingMessageType = "markdown"
)
