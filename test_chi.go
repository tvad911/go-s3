package main
import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
)
func main() {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("key", "value")
	ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
	fmt.Println(chi.URLParamFromCtx(ctx, "key"))
}
