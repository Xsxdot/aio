package service

import (
	"fmt"
	"strconv"
	"strings"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/systemd/internal/model/dto"
)

// UnitGeneratorService unit 文件生成服务
// 负责根据结构化参数生成 systemd service unit 内容
// 只做参数校验与文本渲染，不做文件写入或 systemctl 调用
type UnitGeneratorService struct {
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewUnitGeneratorService 创建 unit 生成服务
func NewUnitGeneratorService(log *logger.Log) *UnitGeneratorService {
	return &UnitGeneratorService{
		log: log.WithEntryName("UnitGeneratorService"),
		err: errorc.NewErrorBuilder("UnitGeneratorService"),
	}
}

// Generate 根据参数生成 systemd service unit 内容
// 输出顺序固定为 [Unit] → [Service] → [Install]，段落间空行，文件末尾保留换行
func (s *UnitGeneratorService) Generate(params *dto.ServiceUnitParams) (string, error) {
	// 参数校验
	if err := s.validate(params); err != nil {
		return "", err
	}

	var sb strings.Builder

	// [Unit] 段
	sb.WriteString("[Unit]\n")
	if params.Description != "" {
		sb.WriteString(fmt.Sprintf("Description=%s\n", params.Description))
	}
	if params.Documentation != "" {
		sb.WriteString(fmt.Sprintf("Documentation=%s\n", params.Documentation))
	}
	for _, v := range params.After {
		sb.WriteString(fmt.Sprintf("After=%s\n", v))
	}
	for _, v := range params.Wants {
		sb.WriteString(fmt.Sprintf("Wants=%s\n", v))
	}
	for _, v := range params.Requires {
		sb.WriteString(fmt.Sprintf("Requires=%s\n", v))
	}
	for _, line := range params.ExtraUnitLines {
		sb.WriteString(line + "\n")
	}

	// 空行分隔
	sb.WriteString("\n")

	// [Service] 段
	sb.WriteString("[Service]\n")

	// Type 默认 simple
	svcType := params.Type
	if svcType == "" {
		svcType = "simple"
	}
	sb.WriteString(fmt.Sprintf("Type=%s\n", svcType))

	// ExecStartPre
	for _, v := range params.ExecStartPre {
		sb.WriteString(fmt.Sprintf("ExecStartPre=%s\n", v))
	}

	// ExecStart（必填，已在 validate 中检查）
	sb.WriteString(fmt.Sprintf("ExecStart=%s\n", params.ExecStart))

	// ExecStartPost
	for _, v := range params.ExecStartPost {
		sb.WriteString(fmt.Sprintf("ExecStartPost=%s\n", v))
	}

	// ExecStop
	if params.ExecStop != "" {
		sb.WriteString(fmt.Sprintf("ExecStop=%s\n", params.ExecStop))
	}

	// ExecReload
	if params.ExecReload != "" {
		sb.WriteString(fmt.Sprintf("ExecReload=%s\n", params.ExecReload))
	}

	// WorkingDirectory
	if params.WorkingDirectory != "" {
		sb.WriteString(fmt.Sprintf("WorkingDirectory=%s\n", params.WorkingDirectory))
	}

	// User
	if params.User != "" {
		sb.WriteString(fmt.Sprintf("User=%s\n", params.User))
	}

	// Group
	if params.Group != "" {
		sb.WriteString(fmt.Sprintf("Group=%s\n", params.Group))
	}

	// Environment
	for _, v := range params.Environment {
		sb.WriteString(fmt.Sprintf("Environment=%s\n", v))
	}

	// EnvironmentFile
	if params.EnvironmentFile != "" {
		sb.WriteString(fmt.Sprintf("EnvironmentFile=%s\n", params.EnvironmentFile))
	}

	// Restart 默认 always
	restart := params.Restart
	if restart == "" {
		restart = "always"
	}
	sb.WriteString(fmt.Sprintf("Restart=%s\n", restart))

	// RestartSec
	if params.RestartSec > 0 {
		sb.WriteString(fmt.Sprintf("RestartSec=%s\n", strconv.Itoa(params.RestartSec)))
	}

	// TimeoutStartSec
	if params.TimeoutStartSec > 0 {
		sb.WriteString(fmt.Sprintf("TimeoutStartSec=%s\n", strconv.Itoa(params.TimeoutStartSec)))
	}

	// TimeoutStopSec
	if params.TimeoutStopSec > 0 {
		sb.WriteString(fmt.Sprintf("TimeoutStopSec=%s\n", strconv.Itoa(params.TimeoutStopSec)))
	}

	// LimitNOFILE
	if params.LimitNOFILE > 0 {
		sb.WriteString(fmt.Sprintf("LimitNOFILE=%s\n", strconv.Itoa(params.LimitNOFILE)))
	}

	// LimitNPROC
	if params.LimitNPROC > 0 {
		sb.WriteString(fmt.Sprintf("LimitNPROC=%s\n", strconv.Itoa(params.LimitNPROC)))
	}

	// Extra Service Lines
	for _, line := range params.ExtraServiceLines {
		sb.WriteString(line + "\n")
	}

	// 空行分隔
	sb.WriteString("\n")

	// [Install] 段
	sb.WriteString("[Install]\n")

	// WantedBy 默认 multi-user.target
	wantedBy := params.WantedBy
	if len(wantedBy) == 0 {
		wantedBy = []string{"multi-user.target"}
	}
	for _, v := range wantedBy {
		sb.WriteString(fmt.Sprintf("WantedBy=%s\n", v))
	}

	// RequiredBy
	for _, v := range params.RequiredBy {
		sb.WriteString(fmt.Sprintf("RequiredBy=%s\n", v))
	}

	// Alias
	for _, v := range params.Alias {
		sb.WriteString(fmt.Sprintf("Alias=%s\n", v))
	}

	// Extra Install Lines
	for _, line := range params.ExtraInstallLines {
		sb.WriteString(line + "\n")
	}

	return sb.String(), nil
}

// validate 校验参数
func (s *UnitGeneratorService) validate(params *dto.ServiceUnitParams) error {
	if params == nil {
		return s.err.New("参数不能为空", nil).ValidWithCtx()
	}

	// ExecStart 必填
	if strings.TrimSpace(params.ExecStart) == "" {
		return s.err.New("ExecStart 不能为空", nil).ValidWithCtx()
	}

	// 安全校验：所有字段值禁止包含换行符（避免注入多行导致 unit 结构被破坏）
	if err := s.checkNoNewlines("Description", params.Description); err != nil {
		return err
	}
	if err := s.checkNoNewlines("Documentation", params.Documentation); err != nil {
		return err
	}
	if err := s.checkNoNewlines("Type", params.Type); err != nil {
		return err
	}
	if err := s.checkNoNewlines("ExecStart", params.ExecStart); err != nil {
		return err
	}
	if err := s.checkNoNewlines("ExecStop", params.ExecStop); err != nil {
		return err
	}
	if err := s.checkNoNewlines("ExecReload", params.ExecReload); err != nil {
		return err
	}
	if err := s.checkNoNewlines("WorkingDirectory", params.WorkingDirectory); err != nil {
		return err
	}
	if err := s.checkNoNewlines("User", params.User); err != nil {
		return err
	}
	if err := s.checkNoNewlines("Group", params.Group); err != nil {
		return err
	}
	if err := s.checkNoNewlines("EnvironmentFile", params.EnvironmentFile); err != nil {
		return err
	}
	if err := s.checkNoNewlines("Restart", params.Restart); err != nil {
		return err
	}

	// 校验数组类型字段
	for i, v := range params.After {
		if err := s.checkNoNewlines(fmt.Sprintf("After[%d]", i), v); err != nil {
			return err
		}
	}
	for i, v := range params.Wants {
		if err := s.checkNoNewlines(fmt.Sprintf("Wants[%d]", i), v); err != nil {
			return err
		}
	}
	for i, v := range params.Requires {
		if err := s.checkNoNewlines(fmt.Sprintf("Requires[%d]", i), v); err != nil {
			return err
		}
	}
	for i, v := range params.ExecStartPre {
		if err := s.checkNoNewlines(fmt.Sprintf("ExecStartPre[%d]", i), v); err != nil {
			return err
		}
	}
	for i, v := range params.ExecStartPost {
		if err := s.checkNoNewlines(fmt.Sprintf("ExecStartPost[%d]", i), v); err != nil {
			return err
		}
	}
	for i, v := range params.Environment {
		if err := s.checkNoNewlines(fmt.Sprintf("Environment[%d]", i), v); err != nil {
			return err
		}
	}
	for i, v := range params.WantedBy {
		if err := s.checkNoNewlines(fmt.Sprintf("WantedBy[%d]", i), v); err != nil {
			return err
		}
	}
	for i, v := range params.RequiredBy {
		if err := s.checkNoNewlines(fmt.Sprintf("RequiredBy[%d]", i), v); err != nil {
			return err
		}
	}
	for i, v := range params.Alias {
		if err := s.checkNoNewlines(fmt.Sprintf("Alias[%d]", i), v); err != nil {
			return err
		}
	}

	// 校验扩展行
	for i, v := range params.ExtraUnitLines {
		if err := s.checkNoNewlines(fmt.Sprintf("ExtraUnitLines[%d]", i), v); err != nil {
			return err
		}
	}
	for i, v := range params.ExtraServiceLines {
		if err := s.checkNoNewlines(fmt.Sprintf("ExtraServiceLines[%d]", i), v); err != nil {
			return err
		}
	}
	for i, v := range params.ExtraInstallLines {
		if err := s.checkNoNewlines(fmt.Sprintf("ExtraInstallLines[%d]", i), v); err != nil {
			return err
		}
	}

	return nil
}

// checkNoNewlines 检查字符串是否包含换行符
func (s *UnitGeneratorService) checkNoNewlines(fieldName, value string) error {
	if strings.ContainsAny(value, "\n\r") {
		return s.err.New(fmt.Sprintf("%s 不能包含换行符", fieldName), nil).ValidWithCtx()
	}
	return nil
}

