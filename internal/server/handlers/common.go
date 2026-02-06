// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"encoding/json"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, err string, message string) {
	writeJSON(w, status, types.ErrorResponse{
		Error:   err,
		Message: message,
		Code:    status,
	})
}

// writeResourceOutput writes a Kubernetes resource as JSON or YAML depending on the output query parameter
func writeResourceOutput(w http.ResponseWriter, r *http.Request, statusCode int, obj client.Object, jsonResponse interface{}) {
	output := r.URL.Query().Get("output")
	if output == "yaml" {
		data, err := json.Marshal(obj)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to marshal resource", err.Error())
			return
		}
		yamlData, err := yaml.JSONToYAML(data)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to convert to YAML", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.WriteHeader(statusCode)
		_, _ = w.Write(yamlData)
		return
	}
	writeJSON(w, statusCode, jsonResponse)
}
