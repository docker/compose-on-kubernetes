package requestaddons

import (
	"context"
	"net/http"

	"k8s.io/apiserver/pkg/endpoints/request"
)

type skipValidationKeyType int

const skipValidationKey skipValidationKeyType = iota

// WithSkipValidationHandler adds a query parameter parsing for skipping validation
func WithSkipValidationHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		skipValidation := req.FormValue("skip-validation") == "1"
		ctx := req.Context()
		req = req.WithContext(WithSkipValidation(ctx, skipValidation))
		handler.ServeHTTP(w, req)
	})
}

// WithSkipValidation adds the skip-validation info in the request context
func WithSkipValidation(parent context.Context, skipValidation bool) context.Context {
	return request.WithValue(parent, skipValidationKey, skipValidation)
}

// SkipValidationFrom gets the skip-validation option value from the context
func SkipValidationFrom(ctx context.Context) bool {
	val := ctx.Value(skipValidationKey)
	if val == nil {
		return false
	}
	if v, ok := val.(bool); ok {
		return v
	}
	return false
}
