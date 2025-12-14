package logger

//import (
//	"context"
//	"github.com/sirupsen/logrus"
//	"zilai.fun/core/util"
//)
//
//type AlertHook struct {
//	levels    []logrus.Level
//	alertFunc func(ctx context.Context, message string) error
//}
//
//func NewAlertHook() *AlertHook {
//	return &AlertHook{
//		levels: []logrus.Level{
//			logrus.PanicLevel,
//			logrus.FatalLevel,
//			logrus.ErrorLevel,
//			logrus.WarnLevel,
//			logrus.InfoLevel,
//			logrus.DebugLevel,
//		},
//		alertFunc: util.SendOpsMessage,
//	}
//}
//
//func (s *AlertHook) Fire(entry *logrus.Entry) error {
//	if _, ok := entry.Data["Alert"]; ok {
//		s2, err := entry.String()
//		if err != nil {
//			s2 = entry.Message
//		}
//		err = s.alertFunc(context.Background(), s2)
//		entry.WithField("AlertErr", err)
//	}
//	return nil
//}
//
//func (s *AlertHook) Levels() []logrus.Level {
//	return s.levels
//}
