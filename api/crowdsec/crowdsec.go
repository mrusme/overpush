package crowdsec

import "fmt"

// [
//   {
//     "capacity": 0,
//     "created_at": "2025-05-23T19:35:22Z",
//     "decisions": [
//       {
//         "duration": "4h",
//         "origin": "cscli",
//         "scenario": "test alert",
//         "scope": "Ip",
//         "type": "ban",
//         "value": "10.10.10.10"
//       }
//     ],
//     "events": [],
//     "events_count": 1,
//     "labels": null,
//     "leakspeed": "0",
//     "message": "test alert",
//     "scenario": "test alert",
//     "scenario_hash": "",
//     "scenario_version": "",
//     "simulated": false,
//     "source": { "ip": "10.10.10.10", "scope": "Ip", "value": "10.10.10.10" },
//     "start_at": "2025-05-23T19:35:22Z",
//     "stop_at": "2025-05-23T19:35:22Z"
//   }
// ]

type Decision struct {
	Duration string `json:"duration",validate:""`
	Origin   string `json:"origin",validate:""`
	Scenario string `json:"scenario",validate:""`
	Scope    string `json:"scope",validate:""`
	Type     string `json:"type",validate:""`
	Value    string `json:"value",validate:""`
}

type Alert struct {
	Token string `json:"token",validate:"required,printascii"`
	User  string `json:"user",validate:"required,printascii"`

	Capacity  int        `json:"capacity",validate:""`
	Decisions []Decision `json:"decisions",validate:""`

	// Events    []Event    `json:"events",validate:""`
	EventsCount int `json:"events_count",validate:""`

	Labels    string `json:"labels",validate:""`
	LeakSpeed string `json:"leakspeeed",validate:""`

	Message string `json:"message",validate:""`

	Scenario        string `json:"scenario",validate:""`
	ScenarioHash    string `json:"scenario_hash",validate:""`
	ScenarioVersion string `json:"scenario_version",validate:""`

	Simulated bool `json:"simulated",validate:""`
	Source    struct {
		IP    string `json:"ip",validate:""`
		Scope string `json:"scope",validate:""`
		Value string `json:"value",validate:""`
	} `json:"source",validate:""`

	CreatedAt string `json:"created_at",validate:""`
	StartAt   string `json:"start_at",validate:""`
	StopAt    string `json:"stop_at",validate:""`
}

type Request struct {
	Alerts []Alert `json:"alerts",validate:""`
}

func (msg *Request) SetToken(s string) {
	msg.Alerts[0].Token = s
}

func (msg *Request) SetUser(s string) {
	msg.Alerts[0].User = s
}

func (msg *Request) GetToken() string {
	return msg.Alerts[0].Token
}

func (msg *Request) GetUser() string {
	return msg.Alerts[0].User
}

func (msg *Request) GetTitle() string {
	return "CrowdSec"
}

func (msg *Request) GetMessage() string {
	var message string

	for _, alert := range msg.Alerts {
		// TODO: Add more info
		message += fmt.Sprintf("- %s\n", alert.Message)
	}

	return message
}

func (msg *Request) GetExternalURL() string {
	return ""
}

func (msg *Request) ToString() string {
	var s string = ""

	s = fmt.Sprintf(
		"%s\n\n%s\n",
		msg.GetTitle(),
		msg.GetMessage(),
	)

	if msg.GetExternalURL() != "" {
		s = fmt.Sprintf(
			"%s\n%s",
			s,
			msg.GetExternalURL(),
		)
	}

	return s
}
