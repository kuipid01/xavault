package mail

import (
	"bytes"
	"html/template"
	"path/filepath"
)

func RenderEmailTemplate(templateName string, data interface{}) (string, error) {
	templatePath := filepath.Join(
		"internal",
		"platform",
		"mail",
		"templates",
		"emails",
		templateName,
	)

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer

	err = tmpl.Execute(&body, data)
	if err != nil {
		return "", err
	}

	return body.String(), nil
}
