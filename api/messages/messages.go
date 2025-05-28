package messages

import "fmt"

type Request struct {
	Token   string `json:"token",validate:"required,printascii"`
	User    string `json:"user",validate:"required,printascii"`
	Message string `json:"message",validate:"required"`

	Attachment       string `json:"attachment",validate:""`
	AttachmentBase64 string `json:"attachment",validate:"base64"`
	AttachmentType   string `json:"attachment_type",validate:""`
	Device           string `json:"device",validate:""`
	HTML             int    `json:"html",validate:"min=0,max=1"`
	Priority         int    `json:"priority",validate:"min=-2,max=2"`
	Timestamp        int64  `json:"timestamp",validate:""`
	Title            string `json:"title",validate:""`
	TTL              int    `json:"ttl",validate:""`
	URL              string `json:"url",validate:"http_url"`
	URLTitle         string `json:"url_title",validate:""`
}

func (msg *Request) ToString() string {
	var s string = ""

	s = fmt.Sprintf(
		"%s\n\n%s\n",
		msg.Title,
		msg.Message,
	)

	if msg.URLTitle != "" {
		s = fmt.Sprintf(
			"%s\n%s",
			s,
			msg.URLTitle,
		)
	}

	if msg.URL != "" {
		s = fmt.Sprintf(
			"%s\n%s",
			s,
			msg.URL,
		)
	}

	return s
}
