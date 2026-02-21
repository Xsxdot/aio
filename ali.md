//tts-qwen3
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation' \
-H "Authorization: Bearer $DASHSCOPE_API_KEY" \
-H 'Content-Type: application/json' \
-d '{
    "model": "qwen3-tts-flash",
    "input": {
        "text": "那我来给大家推荐一款T恤，这款呢真的是超级好看，这个颜色呢很显气质，而且呢也是搭配的绝佳单品，大家可以闭眼入，真的是非常好看，对身材的包容性也很好，不管啥身材的宝宝呢，穿上去都是很好看的。推荐宝宝们下单哦。",
        "voice": "Cherry",
        "language_type": "Chinese"
    }
}'

//口型替换
提交任务
curl --location 'https://dashscope.aliyuncs.com/api/v1/services/aigc/image2video/video-synthesis/' \
--header 'X-DashScope-Async: enable' \
--header "Authorization: Bearer $DASHSCOPE_API_KEY" \
--header 'Content-Type: application/json' \
--data '{
    "model": "videoretalk",
    "input": {
        "video_url": "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20250717/pvegot/input_video_01.mp4",
        "audio_url": "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20250717/aumwir/stella2-%E6%9C%89%E5%A3%B0%E4%B9%A67.wav",
        "ref_image_url": ""
     },
    "parameters": {
        "video_extension": false
    }
  }'
响应
{
    "output": {
	"task_id": "a8532587-fa8c-4ef8-82be-0c46b17950d1", 
    	"task_status": "PENDING"
    },
    "request_id": "7574ee8f-38a3-4b1e-9280-11c33ab46e51"
}
查询状态和结果
curl -X GET 'https://dashscope.aliyuncs.com/api/v1/tasks/<YOUR_TASK_ID>' \
--header "Authorization: Bearer $DASHSCOPE_API_KEY"

响应示例
{
    "request_id": "87b9dce5-7f36-4305-a347-xxxxxx",
    "output": {
        "task_id": "3afd65eb-9604-48ea-8a91-xxxxxx",
        "task_status": "SUCCEEDED",
        "submit_time": "2025-09-11 20:15:29.887",
        "scheduled_time": "2025-09-11 20:15:36.741",
        "end_time": "2025-09-11 20:16:40.577",
        "video_url": "http://dashscope-result-sh.oss-cn-shanghai.aliyuncs.com/xxx.mp4?Expires=xxx"
    },
    "usage": {
        "video_duration": 7.16,
        "video_ratio": "standard"
    }
}
{
    "request_id": "7574ee8f-38a3-4b1e-9280-11c33ab46e51",
  	"output": {
        "task_id": "a8532587-fa8c-4ef8-82be-0c46b17950d1", 
    	"task_status": "FAILED",
    	"code": "xxx", 
    	"message": "xxxxxx"
    }  
}

//视频换人
发起任务
curl --location 'https://dashscope.aliyuncs.com/api/v1/services/aigc/image2video/video-synthesis' \
--header 'X-DashScope-Async: enable' \
--header "Authorization: Bearer $DASHSCOPE_API_KEY" \
--header 'Content-Type: application/json' \
--data '{
    "model": "wan2.2-animate-mix",
    "input": {
        "image_url": "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20250919/bhkfor/mix_input_image.jpeg",
        "video_url": "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20250919/wqefue/mix_input_video.mp4",
        "watermark": true
    },
    "parameters": {
        "mode": "wan-std"
    }
  }'
响应
{
    "output": {
        "task_status": "PENDING",
        "task_id": "0385dc79-5ff8-4d82-bcb6-xxxxxx"
    },
    "request_id": "4909100c-7b5a-9f92-bfe5-xxxxxx"
}
{
    "code": "InvalidApiKey",
    "message": "No API-key provided.",
    "request_id": "7438d53d-6eb8-4596-8835-xxxxxx"
}
查询结果
curl -X GET https://dashscope.aliyuncs.com/api/v1/tasks/0385dc79-5ff8-4d82-bcb6-xxxxxx \
--header "Authorization: Bearer $DASHSCOPE_API_KEY"
响应
{
    "request_id": "a67f8716-18ef-447c-a286-xxxxxx",
    "output": {
        "task_id": "0385dc79-5ff8-4d82-bcb6-xxxxxx",
        "task_status": "SUCCEEDED",
        "submit_time": "2025-09-18 15:32:00.105",
        "scheduled_time": "2025-09-18 15:32:15.066",
        "end_time": "2025-09-18 15:34:41.898",
        "results": {
            "video_url": "http://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxxxx.mp4?Expires=xxxxxx"
        }
    },
    "usage": {
        "video_duration": 5.2,
        "video_ratio": "standard"
    }
}
{
    "request_id": "daad9007-6acd-9fb3-a6bc-xxxxxx",
    "output": {
        "task_id": "fe8aa114-d9f1-4f76-b598-xxxxxx",
        "task_status": "FAILED",
        "code": "InternalError",
        "message": "xxxxxx"
    }
}

//ocr
curl -X POST https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions \
-H "Authorization: Bearer $DASHSCOPE_API_KEY" \
-H "Content-Type: application/json" \
-d '{
  "model": "qwen-vl-ocr-latest",
  "messages": [
        {
            "role": "user",
            "content": [
                {
                    "type": "image_url",
                    "image_url": {"url":"https://img.alicdn.com/imgextra/i2/O1CN01ktT8451iQutqReELT_!!6000000004408-0-tps-689-487.jpg"},
                    "min_pixels": 3072,
                    "max_pixels": 8388608
                },
                {"type": "text", "text": "请提取车票图像中的发票号码、车次、起始站、终点站、发车日期和时间点、座位号、席别类型、票价、身份证号码、购票人姓名。要求准确无误的提取上述关键信息、不要遗漏和捏造虚假信息，模糊或者强光遮挡的单个文字可以用英文问号?代替。返回数据格式以json方式输出，格式为：{'发票号码': 'xxx', '起始站': 'xxx', '终点站': 'xxx', '发车日期和时间点':'xxx', '座位号': 'xxx','票价':'xxx', '身份证号码': 'xxx', '购票人姓名': 'xxx'"}
            ]
        }
    ]
}'

//tts CosyVoice
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	wsURL      = "wss://dashscope.aliyuncs.com/api-ws/v1/inference/"
	outputFile = "output.mp3"
)

func main() {
	// 若没有将API Key配置到环境变量，可将下行替换为：apiKey := "your_api_key"。不建议在生产环境中直接将API Key硬编码到代码中，以减少API Key泄露风险。
	apiKey := os.Getenv("DASHSCOPE_API_KEY")

	// 清空输出文件
	os.Remove(outputFile)
	os.Create(outputFile)

	// 连接WebSocket
	header := make(http.Header)
	header.Add("X-DashScope-DataInspection", "enable")
	header.Add("Authorization", fmt.Sprintf("bearer %s", apiKey))

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			fmt.Printf("连接失败 HTTP状态码: %d\n", resp.StatusCode)
		}
		fmt.Println("连接失败:", err)
		return
	}
	defer conn.Close()

	// 生成任务ID
	taskID := uuid.New().String()
	fmt.Printf("生成任务ID: %s\n", taskID)

	// 发送run-task指令
	runTaskCmd := map[string]interface{}{
		"header": map[string]interface{}{
			"action":    "run-task",
			"task_id":   taskID,
			"streaming": "duplex",
		},
		"payload": map[string]interface{}{
			"task_group": "audio",
			"task":       "tts",
			"function":   "SpeechSynthesizer",
			"model":      "cosyvoice-v3-flash",
			"parameters": map[string]interface{}{
				"text_type":   "PlainText",
				"voice":       "longanyang",
				"format":      "mp3",
				"sample_rate": 22050,
				"volume":      50,
				"rate":        1,
				"pitch":       1,
				// 如果enable_ssml设为true，只允许发送一次continue-task指令，否则会报错“Text request limit violated, expected 1.”
				"enable_ssml": false,
			},
			"input": map[string]interface{}{},
		},
	}

	runTaskJSON, _ := json.Marshal(runTaskCmd)
	fmt.Printf("发送run-task指令: %s\n", string(runTaskJSON))

	err = conn.WriteMessage(websocket.TextMessage, runTaskJSON)
	if err != nil {
		fmt.Println("发送run-task失败:", err)
		return
	}

	textSent := false

	// 处理消息
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("读取消息失败:", err)
			break
		}

		// 处理二进制消息
		if messageType == websocket.BinaryMessage {
			fmt.Printf("收到二进制消息，长度: %d\n", len(message))
			file, _ := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
			file.Write(message)
			file.Close()
			continue
		}

		// 处理文本消息
		messageStr := string(message)
		fmt.Printf("收到文本消息: %s\n", strings.ReplaceAll(messageStr, "\n", ""))

		// 简单解析JSON获取event类型
		var msgMap map[string]interface{}
		if json.Unmarshal(message, &msgMap) == nil {
			if header, ok := msgMap["header"].(map[string]interface{}); ok {
				if event, ok := header["event"].(string); ok {
					fmt.Printf("事件类型: %s\n", event)

					switch event {
					case "task-started":
						fmt.Println("=== 收到task-started事件 ===")

						if !textSent {
							// 发送continue-task指令

							texts := []string{"床前明月光，疑是地上霜。", "举头望明月，低头思故乡。"}

							for _, text := range texts {
								continueTaskCmd := map[string]interface{}{
									"header": map[string]interface{}{
										"action":    "continue-task",
										"task_id":   taskID,
										"streaming": "duplex",
									},
									"payload": map[string]interface{}{
										"input": map[string]interface{}{
											"text": text,
										},
									},
								}

								continueTaskJSON, _ := json.Marshal(continueTaskCmd)
								fmt.Printf("发送continue-task指令: %s\n", string(continueTaskJSON))

								err = conn.WriteMessage(websocket.TextMessage, continueTaskJSON)
								if err != nil {
									fmt.Println("发送continue-task失败:", err)
									return
								}
							}

							textSent = true

							// 延迟发送finish-task
							time.Sleep(500 * time.Millisecond)

							// 发送finish-task指令
							finishTaskCmd := map[string]interface{}{
								"header": map[string]interface{}{
									"action":    "finish-task",
									"task_id":   taskID,
									"streaming": "duplex",
								},
								"payload": map[string]interface{}{
									"input": map[string]interface{}{},
								},
							}

							finishTaskJSON, _ := json.Marshal(finishTaskCmd)
							fmt.Printf("发送finish-task指令: %s\n", string(finishTaskJSON))

							err = conn.WriteMessage(websocket.TextMessage, finishTaskJSON)
							if err != nil {
								fmt.Println("发送finish-task失败:", err)
								return
							}
						}

					case "task-finished":
						fmt.Println("=== 任务完成 ===")
						return

					case "task-failed":
						fmt.Println("=== 任务失败 ===")
						if header["error_message"] != nil {
							fmt.Printf("错误信息: %s\n", header["error_message"])
						}
						return

					case "result-generated":
						fmt.Println("收到result-generated事件")
					}
				}
			}
		}
	}
}

//asr
curl -X POST 'https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions' \
-H "Authorization: Bearer $DASHSCOPE_API_KEY" \
-H "Content-Type: application/json" \
-d '{
    "model": "qwen3-asr-flash",
    "messages": [
        {
            "content": [
                {
                    "type": "input_audio",
                    "input_audio": {
                        "data": "https://dashscope.oss-cn-beijing.aliyuncs.com/audios/welcome.mp3"
                    }
                }
            ],
            "role": "user"
        }
    ],
    "stream":false,
    "asr_options": {
        "enable_itn": false,
        "language": "en"
    }
}'

//图片理解-模型：qwen3-vl-plus、qwen3-vl-flash、qwen-vl-max、qwen-vl-plus，千问VL 模型支持在单次请求中传入多张图片，可用于商品对比、多页文档处理等任务。实现时只需在user message 的content数组中包含多个图片对象即可。
curl --location 'https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions' \
--header "Authorization: Bearer $DASHSCOPE_API_KEY" \
--header 'Content-Type: application/json' \
--data '{
  "model": "qwen3-vl-plus",
  "messages": [
  {
    "role": "user",
    "content": [
      {"type": "image_url", "image_url": {"url": "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20241022/emyrja/dog_and_girl.jpeg"}},
      {"type": "text", "text": "图中描绘的是什么景象?"}
    ]
  }]
}'
响应
{
  "choices": [
    {
      "message": {
        "content": "这是一张在海滩上拍摄的照片。照片中，一个人和一只狗坐在沙滩上，背景是大海和天空。人和狗似乎在互动，狗的前爪搭在人的手上。阳光从画面的右侧照射过来，给整个场景增添了一种温暖的氛围。",
        "role": "assistant"
      },
      "finish_reason": "stop",
      "index": 0,
      "logprobs": null
    }
  ],
  "object": "chat.completion",
  "usage": {
    "prompt_tokens": 1270,
    "completion_tokens": 54,
    "total_tokens": 1324
  },
  "created": 1725948561,
  "system_fingerprint": null,
  "model": "qwen3-vl-plus",
  "id": "chatcmpl-0fd66f46-b09e-9164-a84f-3ebbbedbac15"
}

//视频理解-视频版：千问VL模型通过从视频中提取帧序列进行内容分析。您可以通过以下两个参数控制抽帧策略：

fps：控制抽帧频率，每隔fps分之1秒秒抽取一帧。取值范围为 [0.1, 10]，默认值为 2.0。

高速运动场景：建议设置较高的 fps 值，以捕捉更多细节

静态或长视频：建议设置较低的 fps 值，以提高处理效率

curl -X POST https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions \
  -H "Authorization: Bearer $DASHSCOPE_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "qwen3-vl-plus",
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "video_url",
            "video_url": {
              "url": "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20241115/cqqkru/1.mp4"
            },
            "fps":2
          },
          {
            "type": "text",
            "text": "这段视频的内容是什么?"
          }
        ]
      }
    ]
  }'

//视频理解：多图版
当视频以图像列表（即预先抽取的视频帧）传入时，可通过fps参数告知模型视频帧之间的时间间隔，这能帮助模型更准确地理解事件的顺序、持续时间和动态变化。模型支持通过 fps 参数指定原始视频的抽帧率，表示视频帧是每隔fps分之1秒从原始视频中抽取的。该参数支持 Qwen2.5-VL、Qwen3-VL模型。

curl -X POST https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions \
-H "Authorization: Bearer $DASHSCOPE_API_KEY" \
-H 'Content-Type: application/json' \
-d '{
    "model": "qwen3-vl-plus",
    "messages": [{"role": "user","content": [{"type": "video","video": [
                  "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20241108/xzsgiz/football1.jpg",
                  "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20241108/tdescd/football2.jpg",
                  "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20241108/zefdja/football3.jpg",
                  "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20241108/aedbqh/football4.jpg"],
                  "fps":2},
                {"type": "text","text": "描述这个视频的具体过程"}]}]
}'