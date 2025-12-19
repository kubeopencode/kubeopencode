// Copyright Contributors to the KubeTask project

package webhook

import (
	"bytes"
	"encoding/json"
	"text/template"
)

// RenderTemplate renders a Go template string with the given data.
// The data is typically the webhook payload.
func RenderTemplate(tmplStr string, data interface{}) (string, error) {
	// Create template with useful functions
	tmpl, err := template.New("webhook").Funcs(templateFuncs()).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// templateFuncs returns a map of useful template functions.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// toJson converts a value to JSON string
		"toJson": func(v interface{}) string {
			b, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return string(b)
		},
		// toPrettyJson converts a value to pretty-printed JSON string
		"toPrettyJson": func(v interface{}) string {
			b, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return ""
			}
			return string(b)
		},
		// default returns the default value if the given value is empty
		"default": func(defaultVal, val interface{}) interface{} {
			if val == nil || val == "" {
				return defaultVal
			}
			return val
		},
		// quote wraps the string in double quotes
		"quote": func(s string) string {
			return `"` + s + `"`
		},
	}
}
