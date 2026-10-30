package provider

import (
	"errors"
	"net/http"

	"google.golang.org/api/googleapi"
)

func isNotFound(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound
}
