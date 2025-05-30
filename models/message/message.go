package message

import "fmt"

type Message struct {
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

	// Note: These are "private" fields that should never be set via the API.
	// Hence these fields have getters/setters, to make it obvious throughout
	// the code. Unfortunately the fields cannot be made truly private (lowercase)
	// as this would overcomplicate JSON marshalling/unmarshalling for transfer
	// between the API and the worker.
	//
	// Important: Whenever a message is being received from outside, the
	// ClearInternal method must be called.
	Internal struct {
		ViaSubmit bool `json:"via_submit",validate:"-"`
	} `json:"_internal",validate:"-"`
}

func (msg *Message) ToString() string {
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

func (msg *Message) ClearInternal() {
	msg.Internal.ViaSubmit = false
}

func (msg *Message) SetViaSubmit(submit bool) {
	msg.Internal.ViaSubmit = submit
}

func (msg *Message) IsViaSubmit() bool {
	return msg.Internal.ViaSubmit
}

