// Package dococd calls the Doco-CD REST API (https://doco.cd/latest/Endpoints/REST-API/).
package dococd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Recreating stops and starts containers and may pull images, so it can take minutes.
var httpClient = &http.Client{Timeout: 5 * time.Minute}

// Recreate asks Doco-CD to force-recreate service in project, or the whole project when service is "".
// Doco-CD reloads the Compose project first, so env_file changes reach the new containers.
func Recreate(ctx context.Context, baseURL, apiKey, project, service string) error {
	target := strings.TrimRight(baseURL, "/") + "/v1/api/project/" + url.PathEscape(project) + "/recreate"
	if service != "" {
		target += "?service=" + url.QueryEscape(service)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, nil)
	if err != nil {
		return err
	}
	request.Header.Set("x-api-key", apiKey)
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode/100 == 2 {
		return nil
	}
	return fmt.Errorf("doco-cd answered %d: %s", response.StatusCode, errorMessage(response.Body))
}

// errorMessage reads Doco-CD's {"error": "..."} body, or the raw text from a proxy in between.
func errorMessage(body io.Reader) string {
	content, _ := io.ReadAll(io.LimitReader(body, 4<<10))
	var errorBody struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(content, &errorBody) == nil && errorBody.Error != "" {
		return errorBody.Error
	}
	return strings.TrimSpace(string(content))
}
