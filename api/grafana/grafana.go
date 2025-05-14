package grafana

import "fmt"

// {
//     "receiver": "test",
//     "status": "firing",
//     "alerts": [
//         {
//             "status": "firing",
//             "labels": {
//                 "alertname": "TestAlert",
//                 "instance": "Grafana"
//             },
//             "annotations": {
//                 "summary": "Notification test"
//             },
//             "startsAt": "2024-09-19T02:31:42.985255201Z",
//             "endsAt": "0001-01-01T00:00:00Z",
//             "generatorURL": "",
//             "fingerprint": "57c6d9296de2ad39",
//             "silenceURL": "http://stats.0xdead10.cc:3000/alerting/silence/new?alertmanager=grafana&matcher=alertname%3DTestAlert&matcher=instance%3DGrafana",
//             "dashboardURL": "",
//             "panelURL": "",
//             "values": {"A": 1, "B": 22},
//             "valueString": "[ metric='foo' labels={instance=bar} value=10 ]"
//         }
//     ],
//     "groupLabels": {
//         "alertname": "TestAlert",
//         "instance": "Grafana"
//     },
//     "commonLabels": {
//         "alertname": "TestAlert",
//         "instance": "Grafana"
//     },
//     "commonAnnotations": {
//         "summary": "Notification test"
//     },
//     "externalURL": "http://stats.0xdead10.cc:3000/",
//     "version": "1",
//     "groupKey": "test-57c6d9296de2ad39-1726713102",
//     "truncatedAlerts": 0,
//     "orgId": 1,
//     "title": "[FIRING:1] TestAlert Grafana ",
//     "state": "alerting",
//     "message": "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = TestAlert\n - instance = Grafana\nAnnotations:\n - summary = Notification test\nSilence: http://stats.0xdead10.cc:3000/alerting/silence/new?alertmanager=grafana&matcher=alertname%3DTestAlert&matcher=instance%3DGrafana\n"
// }

type Alert struct {
	Status string `json:"status",validate:""`
	Labels struct {
		AlertName string `json:"alertname",validate:""`
		Instance  string `json:"instance",validate:""`
	} `json:"labels",validate:""`
	Annotations struct {
		Summary string `json:"summary",validate:""`
	} `json:"annotations",validate:""`
	StartsAt     string         `json:"startsAt",validate:""`
	EndsAt       string         `json:"endsAt",validate:""`
	GeneratorURL string         `json:"generatorURL",validate:"http_url"`
	Fingerprint  string         `json:"fingerprint",validate:""`
	SilenceURL   string         `json:"silenceURL",validate:"http_url"`
	DashboardURL string         `json:"dashboardURL",validate:"http_url"`
	PanelURL     string         `json:"panelURL",validate:"http_url"`
	Values       map[string]int `json:"values",validate:""`
	ValueString  string         `json:"valueString",validate:""`
}

type Request struct {
	Token string `json:"token",validate:"required,printascii"`
	User  string `json:"user",validate:"required,printascii"`

	Receiver    string  `json:"receiver",validate:""`
	Status      string  `json:"status",validate:""`
	Alerts      []Alert `json:"alerts",validate:""`
	GroupLabels struct {
		AlertName string `json:"alertname",validate:""`
		Instance  string `json:"instance",validate:""`
	} `json:"groupLabels",validate:""`
	CommonLabels struct {
		AlertName string `json:"alertname",validate:""`
		Instance  string `json:"instance",validate:""`
	} `json:"commonLabels",validate:""`
	CommonAnnotations struct {
		Summary string `json:"summary",validate:""`
	} `json:"commonAnnotations",validate:""`

	ExternalURL     string `json:"externalURL",validate:"http_url"`
	Version         string `json:"version",validate:""`
	GroupKey        string `json:"groupKey",validate:""`
	TruncatedAlerts int    `json:"truncatedAlerts",validate:""`
	OrgID           int    `json:"orgId",validate:""`
	Title           string `json:"title",validate:""`
	State           string `json:"state",validate:""`
	Message         string `json:"message",validate:"required"`
}

func (msg *Request) ToString() string {
	var s string = ""

	s = fmt.Sprintf(
		"%s\n\n%s\n",
		msg.Title,
		msg.Message,
	)

	if msg.ExternalURL != "" {
		s = fmt.Sprintf(
			"%s\n%s",
			s,
			msg.ExternalURL,
		)
	}

	return s
}
