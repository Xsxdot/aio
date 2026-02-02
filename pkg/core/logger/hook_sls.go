package logger

import (
	"fmt"
	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/gogo/protobuf/proto"
	"github.com/sirupsen/logrus"
	"github.com/xsxdot/aio/pkg/core/config"
	"time"
)

var (
	endpoint = "cn-qingdao-intranet.log.aliyuncs.com"
	logstore = "prod"
)

type SlsHook struct {
	levels []logrus.Level
	//producer *producer.Producer
	client   sls.ClientInterface
	appName  string
	host     string
	logstore string
}

func NewSlsHook(appName, host string, config config.LogConfig) *SlsHook {
	provider := sls.NewStaticCredentialsProvider(config.AccessKey, config.AccessSecret, "")
	client := sls.CreateNormalInterfaceV2(config.Endpoint, provider)

	return &SlsHook{
		levels: []logrus.Level{
			logrus.PanicLevel,
			logrus.FatalLevel,
			logrus.ErrorLevel,
			logrus.WarnLevel,
			logrus.InfoLevel,
			logrus.DebugLevel,
		},
		client:   client,
		appName:  appName,
		host:     host,
		logstore: logstore,
	}
}

func (s *SlsHook) Fire(entry *logrus.Entry) error {
	var content []*sls.LogContent
	for k, v := range entry.Data {
		content = append(content, &sls.LogContent{
			Key:   proto.String(k),
			Value: proto.String(fmt.Sprintf("%v", v)),
		})
	}

	content = append(content, &sls.LogContent{
		Key:   proto.String("message"),
		Value: proto.String(entry.Message),
	})

	logGroup := &sls.LogGroup{
		Topic:  proto.String(s.appName),
		Source: proto.String(s.host),
		Logs: []*sls.Log{{
			Time:     proto.Uint32(uint32(time.Now().Unix())),
			Contents: content,
		}},
	}

	return s.client.PutLogs("szl", s.logstore, logGroup)
}

func (s *SlsHook) Levels() []logrus.Level {
	return s.levels
}
