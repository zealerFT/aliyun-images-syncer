package middleware

import (
	"fmt"

	"gopkg.in/gomail.v2"
)

type MailClient struct {
	Dialer                     *gomail.Dialer
	UserName, MailTo, SendName string
	Subject, Body              string
}

func NewMailClient(host, userName, authCode, mailTo string) *MailClient {
	return &MailClient{
		Dialer:   gomail.NewDialer(host, 465, userName, authCode),
		Subject:  "主从镜像自动同步服务挂了！",
		SendName: "gitlab-bot",
		MailTo:   mailTo,
		UserName: userName,
	}
}

// SendMail 防止程序挂掉，而无人知晓的情况
func (m *MailClient) SendMail(body string) {
	message := gomail.NewMessage()
	message.SetHeader("From", message.FormatAddress(m.UserName, m.SendName))
	message.SetHeader("To", m.MailTo)
	message.SetHeader("Subject", m.Subject)
	message.SetBody("text/html", body)
	err := m.Dialer.DialAndSend(message)
	if err != nil {
		fmt.Println("send email fail:", err)
	}
}
