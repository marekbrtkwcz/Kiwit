package xmlrpc

import "context"

type ctxKey string

const tokenKey ctxKey = "xmlrpc_token"

func contextWithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}
