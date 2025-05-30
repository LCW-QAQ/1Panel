package router

import (
	v1 "github.com/1Panel-dev/1Panel/backend/app/api/v1"
	"github.com/1Panel-dev/1Panel/backend/middleware"

	"github.com/gin-gonic/gin"
)

type HostsRouter struct{}

func (s *HostsRouter) InitRouter(Router *gin.RouterGroup) {
	hostRouter := Router.Group("mhosts").
		Use(middleware.JwtAuth()).
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired())
	baseApi := v1.ApiGroupApp.BaseApi
	{
		hostRouter.POST("/forward", baseApi.ForwardHostApi)
	}
}
