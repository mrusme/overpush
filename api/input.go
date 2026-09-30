package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

var inputHeaders = map[string]struct{}{
	"Accept":         {},
	"Content-Length": {},
	"Content-Type":   {},
	"Host":           {},
	"User-Agent":     {},
}

func formatInput(
	headers map[string][]string,
	queries map[string]string,
	body []byte,
) string {
	var input strings.Builder

	input.WriteString("--- HEADERS --------------------------------------------------------------------\n")
	headerKeys := make([]string, 0, len(headers))
	for k := range headers {
		if _, ok := inputHeaders[http.CanonicalHeaderKey(k)]; !ok {
			continue
		}
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)
	for _, k := range headerKeys {
		input.WriteString(fmt.Sprintf("%s: %s\n", k, headers[k]))
	}
	input.WriteString("\n")

	input.WriteString("--- QUERIES --------------------------------------------------------------------\n")
	queryKeys := make([]string, 0, len(queries))
	for k := range queries {
		queryKeys = append(queryKeys, k)
	}
	sort.Strings(queryKeys)
	for _, k := range queryKeys {
		input.WriteString(fmt.Sprintf("%s: %s\n", k, queries[k]))
	}
	input.WriteString("\n")

	input.WriteString("--- BODY -----------------------------------------------------------------------\n")
	input.Write(body)

	return input.String()
}
