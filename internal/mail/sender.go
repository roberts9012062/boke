// internal/mail/sender.go
// 邮件发送器（M2 找回密码）：
//   - SMTP 配置齐全（.env SMTP_*）时用 net/smtp 真实发送
//   - 未配置时降级为日志输出（开发模式：重置链接写入 logs/，文档说明）
// 说明：连接器类（外部系统接口），纯函数式调用封装。
package mail

import (
	"fmt"
	"net/smtp"

	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/config"
)

// Sender 邮件发送器（连接器类）。
type Sender struct {
	cfg    config.MailConfig // SMTP 配置
	logger *zap.Logger       // 日志（降级输出与发送留痕）
}

// NewSender 创建邮件发送器。
func NewSender(cfg config.MailConfig, logger *zap.Logger) *Sender {
	return &Sender{cfg: cfg, logger: logger}
}

// Enabled 是否配置了 SMTP（未配置时降级日志输出）。
func (s *Sender) Enabled() bool {
	return s.cfg.Host != "" && s.cfg.Port != "" && s.cfg.From != ""
}

// Send 发送邮件。
// 参数：to 收件人；subject 主题；body 正文（纯文本）。
// 返回：错误（降级模式下返回 nil，链接已写日志）。
func (s *Sender) Send(to string, subject string, body string) error {
	// ---------- 降级模式：日志输出（开发环境可验证，正式环境配置 SMTP 后自动启用） ----------
	if !s.Enabled() {
		s.logger.Warn("邮件发送降级为日志（未配置 SMTP）",
			zap.String("to", to), zap.String("subject", subject), zap.String("body", body))
		return nil
	}

	// ---------- SMTP 真实发送 ----------
	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n", s.cfg.From, to, subject, body)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("发送邮件失败：%w", err)
	}
	s.logger.Info("邮件已发送", zap.String("to", to), zap.String("subject", subject))
	return nil
}
