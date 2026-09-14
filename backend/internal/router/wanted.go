package router

import (
	"github.com/gin-gonic/gin"
	"github.com/marketpal/marketpal/internal/handler"
	"github.com/marketpal/marketpal/internal/middleware"
)

// RegisterWantedRoutes 求购需求模块路由。
func RegisterWantedRoutes(api *gin.RouterGroup, h *handler.WantedHandler, secret string) {
	public := api.Group("/wanteds")
	{
		public.GET("", h.List)
		public.GET("/:id", h.Detail)
	}
	authed := api.Group("/wanteds", middleware.Auth(secret))
	{
		authed.POST("", h.Create)
		authed.POST("/:id/close", h.Close)
		authed.GET("/mine", h.MyWanteds)
	}
}
